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

# Test related variables
COVERAGE_DIR=coverage
COVERAGE_PROFILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html
TEST_FLAGS=-race -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic
TEST_PATH=./test/...

# Docker parameters
DOCKER_COMPOSE=docker-compose
DOCKER_IMAGE_NAME=parity
DOCKER_IMAGE_TAG=latest

# Build flags
BUILD_FLAGS=-v

# Add these lines after the existing parameters
INSTALL_PATH=/usr/local/bin

.PHONY: all build run test clean deps fmt lint help docker-up docker-down docker-logs docker-build docker-clean install-air watch migrate-up migrate-down tools install uninstall

all: clean build

build: ## Build the application
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) cmd/server/main.go

run: ## Run the application
	$(GORUN) cmd/server/main.go daemon

test: setup-coverage ## Run tests without logs
	@$(GOTEST) $(TEST_FLAGS) $(TEST_PATH) 

test-verbose: setup-coverage ## Run tests with verbose output and logs
	$(GOTEST) $(TEST_FLAGS) -v $(TEST_PATH)


setup-coverage: ## Create coverage directory
	@mkdir -p $(COVERAGE_DIR)

clean: ## Clean build files
	rm -f $(BINARY_NAME)
	rm -rf $(COVERAGE_DIR)
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

install: build ## Install parity command globally
	@echo "Installing parity to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete. Run 'parity auth --private-key YOUR_KEY' to authenticate"

uninstall: ## Remove parity command from system
	@echo "Uninstalling parity from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete"


.DEFAULT_GOAL := help
