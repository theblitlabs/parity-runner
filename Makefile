# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BINARY_NAME=parity
MAIN_PATH=cmd/main.go
AIR_VERSION=v1.49.0
GOPATH=$(shell go env GOPATH)
AIR=$(GOPATH)/bin/air

# Docker metrics configuration
DOCKER_COMPOSE=docker-compose
METRICS_COMPOSE_FILE=docker-compose.metrics.yml
OBSERVABILITY_COMPOSE_FILE=docker-compose.observability.yml
PROMETHEUS_CONTAINER=prometheus
GRAFANA_CONTAINER=grafana

# Test related variables
COVERAGE_DIR=coverage
COVERAGE_PROFILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html
TEST_FLAGS=-race -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic
TEST_PACKAGES=./...  # This will test all packages
TEST_PATH=./test/...

# Build flags
BUILD_FLAGS=-v

# Add these lines after the existing parameters
INSTALL_PATH=/usr/local/bin

.PHONY: all build run test clean deps fmt help docker-up docker-down docker-logs docker-build docker-clean install-air watch migrate-up migrate-down tools install uninstall install-lint-tools lint metrics-up metrics-down metrics-logs metrics-restart prometheus-logs grafana-logs

all: clean build

build: ## Build the application
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd
	chmod +x $(BINARY_NAME)

run:  ## Run the application
	$(GOCMD) run $(MAIN_PATH) --help

server:  ## Start the parity server
	$(GOCMD) run $(MAIN_PATH) server

runner:  ## Start the task runner
	$(GOCMD) run $(MAIN_PATH) runner

chain:  ## Start the chain proxy server
	$(GOCMD) run $(MAIN_PATH) chain

stake:  ## Stake tokens in the network
	$(GOCMD) run $(MAIN_PATH) stake --amount 10

balance:  ## Check token balances
	$(GOCMD) run $(MAIN_PATH) balance

auth:  ## Authenticate with the network
	$(GOCMD) run $(MAIN_PATH) auth

test: setup-coverage ## Run tests with coverage
	$(GOTEST) $(TEST_FLAGS) -v $(TEST_PACKAGES)
	@go tool cover -func=$(COVERAGE_PROFILE)
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)


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

install-air: ## Install air for hot reloading
	@if ! command -v air > /dev/null; then \
		echo "Installing air..." && \
		go install github.com/air-verse/air@latest; \
	fi

watch: install-air ## Run the application with hot reload
	$(AIR)

migrate-up: ## Run database migrations up
	$(GOCMD) run $(MAIN_PATH) migrate

migrate-down: ## Run database migrations down
	$(GOCMD) run $(MAIN_PATH) migrate --down

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install: build ## Install parity command globally
	@echo "Installing parity to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete"

uninstall: ## Remove parity command from system
	@echo "Uninstalling parity from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete"

lint: ## Run linting
	golangci-lint run --timeout=5m

install-lint-tools: ## Install linting tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

metrics-up: ## Start metrics containers (Prometheus & Grafana)
	$(DOCKER_COMPOSE) -f $(METRICS_COMPOSE_FILE) up -d
	$(DOCKER_COMPOSE) -f $(OBSERVABILITY_COMPOSE_FILE) up -d

metrics-down: ## Stop metrics containers
	$(DOCKER_COMPOSE) -f $(METRICS_COMPOSE_FILE) down
	$(DOCKER_COMPOSE) -f $(OBSERVABILITY_COMPOSE_FILE) down

metrics-logs: ## View metrics containers logs
	$(DOCKER_COMPOSE) -f $(METRICS_COMPOSE_FILE) logs -f

metrics-delete: ## Delete metrics containers
	$(DOCKER_COMPOSE) -f $(METRICS_COMPOSE_FILE) down -v
	$(DOCKER_COMPOSE) -f $(OBSERVABILITY_COMPOSE_FILE) down -v

metrics-restart: ## Restart metrics containers
	$(DOCKER_COMPOSE) -f $(METRICS_COMPOSE_FILE) restart
	$(DOCKER_COMPOSE) -f $(OBSERVABILITY_COMPOSE_FILE) restart

prometheus-logs: ## View Prometheus logs
	docker logs -f $(PROMETHEUS_CONTAINER)

grafana-logs: ## View Grafana logs
	docker logs -f $(GRAFANA_CONTAINER)

.DEFAULT_GOAL := help
