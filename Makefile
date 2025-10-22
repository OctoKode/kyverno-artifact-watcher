.PHONY: help build test run clean install lint fmt vet docker-build docker-run deps

# Variables
BINARY_NAME=kyverno-watcher
DOCKER_IMAGE=kyverno-watcher
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building ${BINARY_NAME}..."
	@go build ${LDFLAGS} -o ${BINARY_NAME} .
	@echo "Build complete: ${BINARY_NAME}"

test: fmt ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

test-coverage: test ## Run tests with coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

run: ## Run the application (requires GITHUB_TOKEN and IMAGE_BASE env vars)
	@echo "Running ${BINARY_NAME}..."
	@go run .

install: ## Install the binary to GOPATH/bin
	@echo "Installing ${BINARY_NAME}..."
	@go install ${LDFLAGS} .
	@echo "Installed to $(shell go env GOPATH)/bin/${BINARY_NAME}"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f ${BINARY_NAME}
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet complete"

lint: fmt vet ## Run linters (fmt and vet)
	@echo "Linting complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

docker-build: ## Build Docker image
	@echo "Building Docker image: ${DOCKER_IMAGE}:${VERSION}..."
	@docker build -t ${DOCKER_IMAGE}:${VERSION} -t ${DOCKER_IMAGE}:latest .
	@echo "Docker image built: ${DOCKER_IMAGE}:${VERSION}"

docker-run: ## Run Docker container (requires GITHUB_TOKEN and IMAGE_BASE env vars)
	@echo "Running Docker container..."
	@docker run --rm \
		-e GITHUB_TOKEN=${GITHUB_TOKEN} \
		-e IMAGE_BASE=${IMAGE_BASE} \
		-e POLL_INTERVAL=${POLL_INTERVAL} \
		-e GITHUB_API_OWNER_TYPE=${GITHUB_API_OWNER_TYPE} \
		${DOCKER_IMAGE}:latest

all: clean lint test build ## Run clean, lint, test, and build

dev: fmt test ## Run format and test for development

.DEFAULT_GOAL := help
