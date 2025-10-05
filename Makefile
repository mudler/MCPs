# Makefile for MCPS project
# Inspired by .github/workflows/image.yml

# Variables
MCP_SERVER ?= duckduckgo
DOCKER_REGISTRY ?= ghcr.io
DOCKER_REPOSITORY ?= mudler/mcps
DOCKER_TAG ?= latest
GO_VERSION ?= 1.25.1

# Docker image name
IMAGE_NAME = $(DOCKER_REGISTRY)/$(DOCKER_REPOSITORY)/$(MCP_SERVER)

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the Docker image locally
	@echo "Building Docker image: $(IMAGE_NAME):$(DOCKER_TAG)"
	docker build \
		--build-arg MCP_SERVER=$(MCP_SERVER) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(IMAGE_NAME):$(DOCKER_TAG) \
		-f Dockerfile \
		.

.PHONY: build-multiarch
build-multiarch: ## Build multi-architecture Docker image (requires buildx)
	@echo "Building multi-architecture Docker image: $(IMAGE_NAME):$(DOCKER_TAG)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg MCP_SERVER=$(MCP_SERVER) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(IMAGE_NAME):$(DOCKER_TAG) \
		-f Dockerfile \
		.

.PHONY: run
run: build ## Build and run the container locally
	@echo "Running container: $(IMAGE_NAME):$(DOCKER_TAG)"
	docker run --rm -it $(IMAGE_NAME):$(DOCKER_TAG)

.PHONY: test-build
test-build: ## Test build without pushing (similar to PR build in CI)
	@echo "Testing build (PR mode): $(IMAGE_NAME):$(DOCKER_TAG)"
	docker build \
		--build-arg MCP_SERVER=$(MCP_SERVER) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(IMAGE_NAME):$(DOCKER_TAG) \
		-f Dockerfile \
		.

.PHONY: push
push: build ## Build and push to registry (requires authentication)
	@echo "Pushing to registry: $(IMAGE_NAME):$(DOCKER_TAG)"
	docker push $(IMAGE_NAME):$(DOCKER_TAG)

.PHONY: clean
clean: ## Remove local Docker images
	@echo "Cleaning up Docker images..."
	docker rmi $(IMAGE_NAME):$(DOCKER_TAG) 2>/dev/null || true

.PHONY: clean-all
clean-all: ## Remove all Docker images and containers
	@echo "Cleaning up all Docker resources..."
	docker system prune -f

.PHONY: go-build
go-build: ## Build Go binary locally (without Docker)
	@echo "Building Go binary for $(MCP_SERVER)..."
	cd $(MCP_SERVER) && go build -o ../bin/$(MCP_SERVER) .

.PHONY: go-run
go-run: go-build ## Build and run Go binary locally
	@echo "Running Go binary: $(MCP_SERVER)"
	./bin/$(MCP_SERVER)

.PHONY: go-test
go-test: ## Run Go tests
	@echo "Running Go tests..."
	go test ./...

.PHONY: go-mod
go-mod: ## Download Go dependencies
	@echo "Downloading Go dependencies..."
	go mod download
	go mod tidy

.PHONY: setup-buildx
setup-buildx: ## Set up Docker Buildx for multi-architecture builds
	@echo "Setting up Docker Buildx..."
	docker buildx create --name mcps-builder --use || true
	docker buildx inspect --bootstrap

.PHONY: lint
lint: ## Run linting checks
	@echo "Running linting checks..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install it first."; exit 1; }
	golangci-lint run

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting Go code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: check
check: fmt vet lint go-test ## Run all checks (format, vet, lint, test)

# Development targets
.PHONY: dev
dev: go-mod go-build go-run ## Development workflow: mod download, build, and run

.PHONY: ci-local
ci-local: check test-build ## Run local CI checks (format, vet, lint, test, build)

# Docker registry targets
.PHONY: login
login: ## Login to Docker registry
	@echo "Logging in to $(DOCKER_REGISTRY)..."
	docker login $(DOCKER_REGISTRY)

.PHONY: tags
tags: ## List available tags for the image
	@echo "Available tags for $(IMAGE_NAME):"
	docker images $(IMAGE_NAME) --format "table {{.Tag}}\t{{.CreatedAt}}\t{{.Size}}"

# Utility targets
.PHONY: version
version: ## Show version information
	@echo "MCP: $(MCP_SERVER)"
	@echo "Docker Image: $(IMAGE_NAME):$(DOCKER_TAG)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Docker Registry: $(DOCKER_REGISTRY)"

.PHONY: info
info: ## Show build information
	@echo "Build Information:"
	@echo "=================="
	@echo "MCP Name: $(MCP_SERVER)"
	@echo "Image Name: $(IMAGE_NAME)"
	@echo "Tag: $(DOCKER_TAG)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Docker Registry: $(DOCKER_REGISTRY)"
	@echo "Repository: $(DOCKER_REPOSITORY)"
