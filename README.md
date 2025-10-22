Kyverno Artifact Watcher
========================

[![Tests](https://github.com/OctoKode/kyverno-artifact-watcher/actions/workflows/test.yml/badge.svg)](https://github.com/OctoKode/kyverno-artifact-watcher/actions/workflows/test.yml)
[![Release](https://github.com/OctoKode/kyverno-artifact-watcher/actions/workflows/release.yml/badge.svg)](https://github.com/OctoKode/kyverno-artifact-watcher/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/OctoKode/kyverno-artifact-watcher)](https://goreportcard.com/report/github.com/OctoKode/kyverno-artifact-watcher)

A Go-based watcher for kyverno policies

## Installation

### Binary Release

Download the latest release for your platform from the [releases page](https://github.com/OctoKode/kyverno-artifact-watcher/releases):

### Docker

```bash
$ docker run -it -e GITHUB_TOKEN=ghp_changeme -e IMAGE_BASE=ghcr.io/myoung34/kyverno-test/policies ghcr.io/octokode/kyverno-artifact-watcher:latest
```

### From Source

```bash
$ git clone https://github.com/OctoKode/kyverno-artifact-watcher.git
$ cd kyverno-artifact-watcher
$ make build
```

## Configuration

The application is configured via environment variables:

### Required
- `IMAGE_BASE` - Full OCI image reference (e.g., "ghcr.io/myoung34/kyverno-test/policies" or "ghcr.io/myoung34/kyverno-test/policies:v0.0.1")

#### For GitHub Container Registry (default)
- `GITHUB_TOKEN` - GitHub token with read:packages (and repo visibility access if needed)

#### For Artifactory
- `PROVIDER` - Set to "artifactory" to use Artifactory instead of GitHub
- `ARTIFACTORY_USERNAME` - Artifactory username
- `ARTIFACTORY_PASSWORD` - Artifactory password/token

### Optional
- `PROVIDER` - Registry provider: "github" (default) or "artifactory"
- `POLL_INTERVAL` - Seconds between polls (default: 30)
- `GITHUB_API_OWNER_TYPE` - "users" or "orgs" (default: users, only used for GitHub provider)

## Testing

```bash
# Run tests
$ make test

# Run tests with coverage
$ make test-coverage

# Run linters
$ make lint
```

## Running

### GitHub Container Registry

```bash
$ export GITHUB_TOKEN=your_token_here
$ export IMAGE_BASE=ghcr.io/myoung34/kyverno-test/policies
$ ./kyverno-watcher
```

### Artifactory

```bash
$ export PROVIDER=artifactory
$ export ARTIFACTORY_USERNAME=your_username
$ export ARTIFACTORY_PASSWORD=your_password
$ export IMAGE_BASE=your-registry.jfrog.io/repo/image:tag
$ ./kyverno-watcher
```
