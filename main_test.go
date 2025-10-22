package main

import (
	"fmt"
	"os"
	"testing"
)

func TestParseImageBase(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantOwner   string
		wantPackage string
		wantErr     bool
	}{
		{
			name:        "simple package",
			input:       "ghcr.io/myoung34/policies",
			wantOwner:   "myoung34",
			wantPackage: "policies",
			wantErr:     false,
		},
		{
			name:        "nested package",
			input:       "ghcr.io/myoung34/kyverno-test/policies",
			wantOwner:   "myoung34",
			wantPackage: "kyverno-test/policies",
			wantErr:     false,
		},
		{
			name:        "package with tag",
			input:       "ghcr.io/myoung34/kyverno-test/policies:v0.0.1",
			wantOwner:   "myoung34",
			wantPackage: "kyverno-test/policies",
			wantErr:     false,
		},
		{
			name:        "deeply nested package with tag",
			input:       "ghcr.io/foo/bar/baz/qux:latest",
			wantOwner:   "foo",
			wantPackage: "bar/baz/qux",
			wantErr:     false,
		},
		{
			name:        "package with digest",
			input:       "ghcr.io/owner/package:sha256-abcd1234",
			wantOwner:   "owner",
			wantPackage: "package",
			wantErr:     false,
		},
		{
			name:        "invalid format - no slashes",
			input:       "invalid",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "invalid format - missing package",
			input:       "ghcr.io/owner",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "invalid format - only one slash",
			input:       "ghcr.io/owner/",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
		{
			name:        "empty string",
			input:       "",
			wantOwner:   "",
			wantPackage: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, pkg, err := parseImageBase(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseImageBase(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}

			if err == nil {
				if owner != tt.wantOwner {
					t.Errorf("parseImageBase(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
				}
				if pkg != tt.wantPackage {
					t.Errorf("parseImageBase(%q) package = %q, want %q", tt.input, pkg, tt.wantPackage)
				}
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "replace colons",
			input: "v0.0.1:latest",
			want:  "v0.0.1_latest",
		},
		{
			name:  "replace slashes",
			input: "owner/package",
			want:  "owner_package",
		},
		{
			name:  "replace both",
			input: "owner/package:v0.0.1",
			want:  "owner_package_v0.0.1",
		},
		{
			name:  "no special chars",
			input: "simple",
			want:  "simple",
		},
		{
			name:  "multiple colons and slashes",
			input: "a/b/c:d:e",
			want:  "a_b_c_d_e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		setEnv       bool
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR_1",
			defaultValue: "default",
			envValue:     "custom",
			setEnv:       true,
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR_2",
			defaultValue: "default",
			envValue:     "",
			setEnv:       false,
			want:         "default",
		},
		{
			name:         "env var set to empty string",
			key:          "TEST_VAR_3",
			defaultValue: "default",
			envValue:     "",
			setEnv:       true,
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvOrDefault(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvAsIntOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		setEnv       bool
		want         int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT_1",
			defaultValue: 10,
			envValue:     "42",
			setEnv:       true,
			want:         42,
		},
		{
			name:         "env var not set",
			key:          "TEST_INT_2",
			defaultValue: 10,
			envValue:     "",
			setEnv:       false,
			want:         10,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT_3",
			defaultValue: 10,
			envValue:     "not-a-number",
			setEnv:       true,
			want:         10,
		},
		{
			name:         "zero value",
			key:          "TEST_INT_4",
			defaultValue: 10,
			envValue:     "0",
			setEnv:       true,
			want:         0,
		},
		{
			name:         "negative integer",
			key:          "TEST_INT_5",
			defaultValue: 10,
			envValue:     "-5",
			setEnv:       true,
			want:         -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			}

			got := getEnvAsIntOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvAsIntOrDefault(%q, %d) = %d, want %d", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestLoadConfigProvider(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		wantErr      bool
		errContains  string
		wantProvider string
	}{
		{
			name: "github provider - default",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_test123",
				"IMAGE_BASE":   "ghcr.io/owner/package",
			},
			wantErr:      false,
			wantProvider: "github",
		},
		{
			name: "github provider - explicit",
			envVars: map[string]string{
				"PROVIDER":     "github",
				"GITHUB_TOKEN": "ghp_test123",
				"IMAGE_BASE":   "ghcr.io/owner/package",
			},
			wantErr:      false,
			wantProvider: "github",
		},
		{
			name: "github provider - uppercase",
			envVars: map[string]string{
				"PROVIDER":     "GITHUB",
				"GITHUB_TOKEN": "ghp_test123",
				"IMAGE_BASE":   "ghcr.io/owner/package",
			},
			wantErr:      false,
			wantProvider: "github",
		},
		{
			name: "artifactory provider - valid",
			envVars: map[string]string{
				"PROVIDER":             "artifactory",
				"ARTIFACTORY_USERNAME": "user@example.com",
				"ARTIFACTORY_PASSWORD": "password123",
				"IMAGE_BASE":           "registry.example.com/repo/image:tag",
			},
			wantErr:      false,
			wantProvider: "artifactory",
		},
		{
			name: "artifactory provider - uppercase",
			envVars: map[string]string{
				"PROVIDER":             "ARTIFACTORY",
				"ARTIFACTORY_USERNAME": "user@example.com",
				"ARTIFACTORY_PASSWORD": "password123",
				"IMAGE_BASE":           "registry.example.com/repo/image:tag",
			},
			wantErr:      false,
			wantProvider: "artifactory",
		},
		{
			name: "invalid provider",
			envVars: map[string]string{
				"PROVIDER":     "invalid",
				"GITHUB_TOKEN": "ghp_test123",
				"IMAGE_BASE":   "ghcr.io/owner/package",
			},
			wantErr:     true,
			errContains: "Unsupported PROVIDER: invalid",
		},
		{
			name: "github provider - missing token",
			envVars: map[string]string{
				"PROVIDER":   "github",
				"IMAGE_BASE": "ghcr.io/owner/package",
			},
			wantErr:     true,
			errContains: "GITHUB_TOKEN environment variable must be set",
		},
		{
			name: "artifactory provider - missing username",
			envVars: map[string]string{
				"PROVIDER":             "artifactory",
				"ARTIFACTORY_PASSWORD": "password123",
				"IMAGE_BASE":           "registry.example.com/repo/image:tag",
			},
			wantErr:     true,
			errContains: "ARTIFACTORY_USERNAME and ARTIFACTORY_PASSWORD environment variables must be set",
		},
		{
			name: "artifactory provider - missing password",
			envVars: map[string]string{
				"PROVIDER":             "artifactory",
				"ARTIFACTORY_USERNAME": "user@example.com",
				"IMAGE_BASE":           "registry.example.com/repo/image:tag",
			},
			wantErr:     true,
			errContains: "ARTIFACTORY_USERNAME and ARTIFACTORY_PASSWORD environment variables must be set",
		},
		{
			name: "missing image base",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_test123",
			},
			wantErr:     true,
			errContains: "IMAGE_BASE environment variable must be set",
		},
		{
			name: "github provider - invalid image base",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_test123",
				"IMAGE_BASE":   "invalid",
			},
			wantErr:     true,
			errContains: "Failed to parse IMAGE_BASE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory to avoid creating /tmp/kyverno-watcher
			originalStateDirBase := stateDirBase
			stateDirBase = t.TempDir()
			defer func() {
				stateDirBase = originalStateDirBase
			}()

			// Mock getEnv to return test values
			originalGetEnvFunc := getEnvFunc
			getEnvFunc = func(key string) string {
				if val, ok := tt.envVars[key]; ok {
					return val
				}
				return ""
			}
			defer func() {
				getEnvFunc = originalGetEnvFunc
			}()

			// Capture fatal calls
			var fatalErr string
			defer func() {
				if r := recover(); r != nil {
					if !tt.wantErr {
						t.Errorf("loadConfig() panicked unexpectedly: %v", r)
						return
					}
					// Check error message if specified
					if errStr, ok := r.(string); ok {
						fatalErr = errStr
					} else {
						fatalErr = fmt.Sprint(r)
					}
					if tt.errContains != "" && !contains(fatalErr, tt.errContains) {
						t.Errorf("loadConfig() error = %q, want to contain %q", fatalErr, tt.errContains)
					}
				} else if tt.wantErr {
					t.Errorf("loadConfig() should have failed but didn't")
				}
			}()

			// Override log.Fatal for testing
			originalLogFatal := logFatal
			logFatal = func(v ...interface{}) {
				panic(v[0])
			}
			defer func() {
				logFatal = originalLogFatal
			}()

			// If we get here without panicking and wantErr is true, the defer will catch it
			if tt.wantErr {
				// The function will panic and defer will handle it
				config := loadConfig()
				// This line should not be reached if test is correct
				t.Errorf("loadConfig() = %+v, should have failed", config)
				return
			}

			config := loadConfig()

			if config.Provider != tt.wantProvider {
				t.Errorf("loadConfig() Provider = %q, want %q", config.Provider, tt.wantProvider)
			}

			// Verify provider-specific fields are set correctly
			if config.Provider == "github" && config.GithubToken == "" {
				t.Error("loadConfig() GithubToken should be set for github provider")
			}
			if config.Provider == "artifactory" {
				if config.Username == "" {
					t.Error("loadConfig() Username should be set for artifactory provider")
				}
				if config.Password == "" {
					t.Error("loadConfig() Password should be set for artifactory provider")
				}
			}
		})
	}
}

func TestWatchLoopProviderBehavior(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		imageBase   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "artifactory - image base without tag",
			provider:    "artifactory",
			imageBase:   "registry.example.com/repo/image",
			wantErr:     true,
			errContains: "IMAGE_BASE for artifactory must include a tag",
		},
		{
			name:      "artifactory - image base with tag",
			provider:  "artifactory",
			imageBase: "registry.example.com/repo/image:1.0.0",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for test state
			testTempDir := t.TempDir()

			// Mock pullImageToDir to avoid creating /tmp/image-* directories
			originalPullImageToDirFunc := pullImageToDirFunc
			pullImageToDirCalled := false
			pullImageToDirFunc = func(config *Config, tag, destDir string) error {
				pullImageToDirCalled = true
				// Create files in test temp dir instead of /tmp
				testDestDir := testTempDir + "/image-" + sanitizePath(tag)
				if err := os.MkdirAll(testDestDir, 0755); err != nil {
					return err
				}
				mockFile := testDestDir + "/test-policy.yaml"
				if err := os.WriteFile(mockFile, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0644); err != nil {
					return err
				}
				// Call applyManifests with the test dir
				return applyManifestsFunc(config, testDestDir)
			}
			defer func() {
				pullImageToDirFunc = originalPullImageToDirFunc
			}()

			// Mock kubectl apply to avoid actual execution
			originalApplyManifestsFunc := applyManifestsFunc
			applyManifestsCalled := false
			applyManifestsFunc = func(config *Config, dir string) error {
				applyManifestsCalled = true
				return nil
			}
			defer func() {
				applyManifestsFunc = originalApplyManifestsFunc
			}()

			config := &Config{
				Provider:  tt.provider,
				ImageBase: tt.imageBase,
				StateDir:  testTempDir,
			}
			config.LastFile = config.StateDir + "/last_seen"

			err := watchLoop(config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("watchLoop() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("watchLoop() error = %q, want to contain %q", err.Error(), tt.errContains)
				}
				// Should not have called pullImageToDir for validation errors
				if pullImageToDirCalled {
					t.Error("watchLoop() should not have called pullImageToDir for validation error")
				}
				if applyManifestsCalled {
					t.Error("watchLoop() should not have called applyManifests for validation error")
				}
			} else {
				// For successful validation, functions should have been called
				if !pullImageToDirCalled && err == nil {
					t.Error("watchLoop() should have called pullImageToDir")
				}
				if !applyManifestsCalled && err == nil {
					t.Error("watchLoop() should have called applyManifests")
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
