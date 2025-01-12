# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BINARY_NAME=parity
MAIN_PATH=cmd/server/main.go
AIR_VERSION=v1.49.0
GOPATH=$(shell go env GOPATH)
AIR=$(GOPATH)/bin/air

# Docker parameters
DOCKER_COMPOSE=docker-compose
DOCKER_IMAGE_NAME=parity
DOCKER_IMAGE_TAG=latest

# Build flags
BUILD_FLAGS=-v

# Test flags
TEST_FLAGS=-v -race -cover

.PHONY: all build run test clean deps fmt lint help docker-up docker-down docker-logs docker-build docker-clean install-air watch migrate-up migrate-down tools

all: clean build

build: ## Build the application
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) $(MAIN_PATH)

run: ## Run the application
	$(GORUN) $(MAIN_PATH)

test: ## Run tests
	$(GOTEST) $(TEST_FLAGS) ./...

clean: ## Clean build files
	rm -f $(BINARY_NAME)
	find . -type f -name '*.test' -delete
	find . -type f -name '*.out' -delete
	rm -rf tmp/

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

fmt: ## Format code
	$(GOFMT) ./...

docker-up: ## Start Docker containers
	$(DOCKER_COMPOSE) up -d --build

docker-down: ## Stop Docker containers
	$(DOCKER_COMPOSE) down

docker-logs: ## View Docker container logs
	$(DOCKER_COMPOSE) logs -f

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) .

docker-clean: ## Clean Docker resources
	$(DOCKER_COMPOSE) down -v --remove-orphans

install-air: ## Install air for hot reloading
	@if ! command -v air > /dev/null; then \
		echo "Installing air..." && \
		go install github.com/air-verse/air@latest; \
	fi

watch: install-air ## Run the application with hot reload
	$(AIR)

migrate-up: ## Run database migrations up
	$(GORUN) cmd/migrate/main.go

migrate-down: ## Run database migrations down
	$(GORUN) cmd/migrate/main.go down

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
