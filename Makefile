.PHONY: help build run test clean docker-build docker-run tidy fmt lint server web

# Variables
BINARY_NAME=research-helper
SERVER_BINARY=research-server
DOCKER_IMAGE=research-helper
VERSION?=latest

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI application
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) cmd/research-helper/main.go
	@echo "Build complete: bin/$(BINARY_NAME)"

build-server: ## Build the API Server
	@echo "Building $(SERVER_BINARY)..."
	@go build -o bin/$(SERVER_BINARY) cmd/server/main.go
	@echo "Build complete: bin/$(SERVER_BINARY)"

run: ## Run the CLI application
	@echo "Running CLI..."
	@go run cmd/research-helper/main.go

server: ## Run the API Server
	@echo "Running API Server..."
	@go run cmd/server/main.go

web: ## Run the Web UI (Frontend)
	@echo "Starting Web UI..."
	@cd web && npm run dev

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.txt ./...
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.txt coverage.html
	@echo "Clean complete"

tidy: ## Tidy and download dependencies
	@echo "Tidying dependencies..."
	@go mod tidy
	@go mod download

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w .

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -f Dockerfile.go -t $(DOCKER_IMAGE):$(VERSION) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(VERSION)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run --rm -p 3000:3000 --env-file .env $(DOCKER_IMAGE):$(VERSION)

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/air-verse/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed"

dev: ## Run with hot reload using air
	@echo "Starting development server with hot reload..."
	@air
