.PHONY: build build-dev build-prod run run-dev run-prod test clean docker-build docker-run help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_NAME=quiz-forge
BINARY_DEV=$(BINARY_NAME)-dev
BINARY_PROD=$(BINARY_NAME)

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Version info
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

## Build Commands

build: build-prod ## Build production binary

build-dev: ## Build development binary with debug endpoints
	$(GOBUILD) $(LDFLAGS) -tags "dev" -o $(BINARY_DEV) ./cmd/server

build-prod: ## Build production binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PROD) ./cmd/server

## Run Commands

run: run-dev ## Run the application (default: dev mode)

run-dev: build-dev ## Build and run in development mode
	APP_ENV=development LOG_LEVEL=debug ./$(BINARY_DEV)

run-prod: build-prod ## Build and run in production mode
	APP_ENV=production LOG_LEVEL=info ./$(BINARY_PROD)

## Docker Commands

docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 $(BINARY_NAME):latest

docker-run-prod: ## Run Docker container in production mode
	docker run -p 8080:8080 \
		-e APP_ENV=production \
		-e SESSION_SECRET=change-me-in-production \
		$(BINARY_NAME):latest

## Development Commands

test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

fmt: ## Format code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

mod-tidy: ## Tidy modules
	$(GOMOD) tidy

## Cleanup

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME) $(BINARY_DEV)
	rm -f coverage.out coverage.html
	docker rmi $(BINARY_NAME):latest 2>/dev/null || true

## Install (for development)

install: build-dev ## Install development binary to GOPATH/bin
	cp $(BINARY_DEV) $(shell go env GOPATH)/bin/$(BINARY_DEV)
