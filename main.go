package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"sigs.k8s.io/yaml"
)

const (
	PolicyLayerMediaType = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
)

var (
	// Version is set via ldflags during build
	Version = "dev"
)

type Manifest struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   ManifestMetadata       `yaml:"metadata" json:"metadata"`
	Spec       map[string]interface{} `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type ManifestMetadata struct {
	Name      string            `yaml:"name" json:"name"`
	Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type Config struct {
	GithubToken        string
	ImageBase          string
	Owner              string
	Package            string
	PackageNormalized  string
	PollInterval       int
	GithubAPIOwnerType string
	StateDir           string
	LastFile           string
}

type GitHubPackageVersion struct {
	ID        int64     `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

func main() {
	// Print version
	log.Printf("Kyverno Artifact Watcher version %s\n", Version)

	config := loadConfig()

	log.Printf("Starting GHCR watcher for %s (owner=%s, package=%s)\n",
		config.ImageBase, config.Owner, config.Package)

	for {
		if err := watchLoop(config); err != nil {
			log.Printf("Error in watch loop: %v\n", err)
		}
		time.Sleep(time.Duration(config.PollInterval) * time.Second)
	}
}

func loadConfig() *Config {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable must be set")
	}

	imageBase := os.Getenv("IMAGE_BASE")
	if imageBase == "" {
		log.Fatal("IMAGE_BASE environment variable must be set (e.g., ghcr.io/owner/package)")
	}

	// Parse IMAGE_BASE to extract owner and package
	// Expected format: ghcr.io/owner/package or ghcr.io/owner/package:tag
	owner, packageName, err := parseImageBase(imageBase)
	if err != nil {
		log.Fatalf("Failed to parse IMAGE_BASE: %v", err)
	}

	pollInterval := getEnvAsIntOrDefault("POLL_INTERVAL", 30)
	githubAPIOwnerType := getEnvOrDefault("GITHUB_API_OWNER_TYPE", "users")

	// Normalize package name for API path
	packageNormalized := strings.ReplaceAll(packageName, "/", "%2F")

	stateDir := "/tmp/ghcr-watcher"
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}
	lastFile := filepath.Join(stateDir, "last_seen")

	return &Config{
		GithubToken:        githubToken,
		ImageBase:          imageBase,
		Owner:              owner,
		Package:            packageName,
		PackageNormalized:  packageNormalized,
		PollInterval:       pollInterval,
		GithubAPIOwnerType: githubAPIOwnerType,
		StateDir:           stateDir,
		LastFile:           lastFile,
	}
}

func parseImageBase(imageBase string) (owner, packageName string, err error) {
	// Remove tag if present (e.g., ghcr.io/owner/package:v0.0.1 -> ghcr.io/owner/package)
	imageBase = strings.Split(imageBase, ":")[0]

	// Expected format: ghcr.io/owner/package[/subpackage/...]
	parts := strings.Split(imageBase, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("IMAGE_BASE must be in format ghcr.io/owner/package, got: %s", imageBase)
	}

	// parts[0] = ghcr.io
	// parts[1] = owner
	// parts[2:] = package parts
	owner = parts[1]
	packageName = strings.Join(parts[2:], "/")

	if owner == "" || packageName == "" {
		return "", "", fmt.Errorf("could not extract owner and package from IMAGE_BASE: %s", imageBase)
	}

	return owner, packageName, nil
}

func watchLoop(config *Config) error {
	latest, err := getLatestTagOrDigest(config)
	if err != nil {
		return fmt.Errorf("could not determine latest tag/digest: %w", err)
	}

	if latest == "" {
		log.Println("No versions found for package")
		return nil
	}

	prev, _ := os.ReadFile(config.LastFile)
	prevTag := strings.TrimSpace(string(prev))

	if latest != prevTag {
		log.Printf("Detected change: previous='%s' new='%s'\n", prevTag, latest)

		destDir := fmt.Sprintf("/tmp/image-%s", sanitizePath(latest))

		if err := pullImageToDir(config, latest, destDir); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}

		if err := applyManifests(config, destDir); err != nil {
			return fmt.Errorf("apply manifests failed: %w", err)
		}

		if err := os.WriteFile(config.LastFile, []byte(latest), 0644); err != nil {
			return fmt.Errorf("failed to write last file: %w", err)
		}
	} else {
		log.Printf("No change (latest=%s)\n", latest)
	}

	return nil
}

func getLatestTagOrDigest(config *Config) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/%s/%s/packages/container/%s/versions",
		config.GithubAPIOwnerType, config.Owner, config.PackageNormalized)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+config.GithubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		var errMsg struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errMsg)

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return "", fmt.Errorf("authentication failed (401): invalid or expired GITHUB_TOKEN")
		case http.StatusForbidden:
			return "", fmt.Errorf("access forbidden (403): token may lack required permissions (read:packages). Message: %s", errMsg.Message)
		case http.StatusNotFound:
			return "", fmt.Errorf("package not found (404): owner=%s, package=%s (owner type: %s). Verify package exists and token has access",
				config.Owner, config.Package, config.GithubAPIOwnerType)
		default:
			return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, errMsg.Message)
		}
	}

	var versions []GitHubPackageVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w. Response body: %s", err, string(body))
	}

	if len(versions) == 0 {
		return "", nil
	}

	// Find the most recently updated version
	latest := versions[0]
	for _, v := range versions {
		if v.UpdatedAt.After(latest.UpdatedAt) {
			latest = v
		}
	}

	// Prefer tag names if present
	if len(latest.Metadata.Container.Tags) > 0 {
		return latest.Metadata.Container.Tags[0], nil
	}

	// Fallback to version ID
	return fmt.Sprintf("version-id-%d", latest.ID), nil
}

func pullImageToDir(config *Config, tag, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		log.Printf("Warning: failed to remove directory %s: %v", destDir, err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	log.Printf("Pulling image %s:%s into %s ...\n", config.ImageBase, tag, destDir)

	// Pull using OCI library
	imageRef := fmt.Sprintf("%s:%s", config.ImageBase, tag)
	ctx := context.Background()

	if err := pullOCI(ctx, imageRef, destDir); err != nil {
		return fmt.Errorf("OCI pull failed: %w", err)
	}

	// Add labels to manifests
	files, err := filepath.Glob(filepath.Join(destDir, "*"))
	if err != nil {
		return err
	}

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		if err := addLabelsToManifest(file, tag); err != nil {
			log.Printf("Warning: failed to add labels to %s: %v\n", file, err)
			// Don't fail - continue with other files
			continue
		}
	}

	return nil
}

func addLabelsToManifest(filePath, tag string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Add labels to the YAML content
	updatedData, err := addLabelsToYAML(data, tag)
	if err != nil {
		return fmt.Errorf("adding labels: %w", err)
	}

	// Write back to the same file
	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

func addLabelsToYAML(yamlData []byte, tag string) ([]byte, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(yamlData, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshaling YAML: %w", err)
	}

	// Initialize labels map if it doesn't exist
	if manifest.Metadata.Labels == nil {
		manifest.Metadata.Labels = make(map[string]string)
	}

	// Add our labels
	manifest.Metadata.Labels["managed-by"] = "kyverno-watcher"
	manifest.Metadata.Labels["policy-version"] = tag

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(&manifest)
	if err != nil {
		return nil, fmt.Errorf("marshaling YAML: %w", err)
	}

	return updatedData, nil
}

func pullOCI(ctx context.Context, imageRef, outputDir string) error {
	// Parse the image reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %w", err)
	}

	log.Printf("Pulling files from OCI image: %s\n", ref.Name())

	// Pull the image using default keychain (uses Docker credentials if available)
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("getting remote image: %w", err)
	}

	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("converting to image: %w", err)
	}

	// Get image layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting image layers: %w", err)
	}

	log.Printf("Found %d layers\n", len(layers))

	// Process each layer
	fileCount := 0
	for i, layer := range layers {
		if err := processLayer(layer, outputDir, i, &fileCount); err != nil {
			return fmt.Errorf("processing layer %d: %w", i, err)
		}
	}

	if fileCount == 0 {
		log.Println("Warning: No files were extracted from the image")
	} else {
		log.Printf("Successfully pulled %d file(s)\n", fileCount)
	}

	return nil
}

func processLayer(layer v1.Layer, outputDir string, layerIndex int, fileCount *int) error {
	// Get layer media type
	mediaType, err := layer.MediaType()
	if err != nil {
		return fmt.Errorf("getting media type: %w", err)
	}

	log.Printf("Layer %d media type: %s\n", layerIndex, mediaType)

	// Get layer content
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("getting compressed layer: %w", err)
	}
	defer func() {
		if cerr := blob.Close(); cerr != nil {
			log.Printf("Warning: failed to close blob for layer %d: %v\n", layerIndex, cerr)
		}
	}()

	// Read the layer content
	content, err := io.ReadAll(blob)
	if err != nil {
		return fmt.Errorf("reading layer content: %w", err)
	}

	if len(content) == 0 {
		log.Printf("  Layer %d is empty, skipping\n", layerIndex)
		return nil
	}

	// Save layer content to file
	filename := filepath.Join(outputDir, fmt.Sprintf("layer-%d.yaml", layerIndex))

	// If it's a policy layer, try to give it a better name
	if mediaType == PolicyLayerMediaType {
		filename = filepath.Join(outputDir, fmt.Sprintf("policy-%d.yaml", layerIndex))
	}

	if err := os.WriteFile(filename, content, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	log.Printf("  Saved to: %s (%d bytes)\n", filepath.Base(filename), len(content))
	*fileCount++

	return nil
}

func applyManifests(config *Config, dir string) error {
	// Find YAML files
	files, err := findYAMLFiles(dir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		log.Printf("No YAML manifests found in %s\n", dir)
		return nil
	}

	log.Printf("Applying manifests in %s ...\n", dir)

	for _, file := range files {
		log.Printf("kubectl apply -f %s\n", file)

		p := script.Exec(fmt.Sprintf("kubectl apply -f %s", file)).
			WithStdout(os.Stdout).
			WithStderr(os.Stderr)

		exitCode := p.ExitStatus()
		if exitCode != 0 {
			log.Printf("kubectl apply failed for %s with exit code %d\n", file, exitCode)
		}
	}

	return nil
}

func findYAMLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yaml" || ext == ".yml" {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

func sanitizePath(s string) string {
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}
