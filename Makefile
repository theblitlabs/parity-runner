# Project configuration
BINARY_NAME := parity-runner
MAIN_PATH := cmd/main.go
INSTALL_PATH := /usr/local/bin

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

# Tool paths
GOPATH := $(shell go env GOPATH)
GOFUMPT_PATH := $(GOPATH)/bin/gofumpt
GOIMPORTS_PATH := $(GOPATH)/bin/goimports
GOLANGCI_LINT := $(shell which golangci-lint)

# Build configuration
BUILD_FLAGS := -v

# Lint configuration
LINT_FLAGS := --timeout=5m
LINT_CONFIG := .golangci.yml
LINT_OUTPUT_FORMAT := colored-line-number

# Define phony targets
.PHONY: all build clean deps fmt imports format lint format-lint check-format help \
        run stake balance auth install uninstall install-lint-tools install-hooks

# Default target
.DEFAULT_GOAL := help

# Primary targets
all: clean build ## Clean and build the project

build: ## Build the application
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd
	chmod +x $(BINARY_NAME)

clean: ## Clean build files and test artifacts
	rm -f $(BINARY_NAME)
	find . -type f -name '*.test' -delete
	find . -type f -name '*.out' -delete
	rm -rf tmp/ $(COVERAGE_DIR)

# Development targets
fmt: ## Format code using gofumpt (preferred) or gofmt
	@echo "Formatting code..."
	@if [ -x "$(GOFUMPT_PATH)" ]; then \
		echo "Using gofumpt for better formatting..."; \
		$(GOFUMPT_PATH) -l -w .; \
	else \
		echo "gofumpt not found, using standard gofmt..."; \
		$(GOFMT) ./...; \
		echo "Consider installing gofumpt for better formatting: go install mvdan.cc/gofumpt@latest"; \
	fi

imports: ## Fix imports formatting and add missing imports
	@echo "Organizing imports..."
	@if [ -x "$(GOIMPORTS_PATH)" ]; then \
		$(GOIMPORTS_PATH) -w -local github.com/theblitlabs/parity-runner .; \
	else \
		echo "goimports not found. Installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
		$(GOIMPORTS_PATH) -w -local github.com/theblitlabs/parity-runner .; \
	fi

format: fmt imports ## Run all formatters (gofumpt + goimports)
	@echo "All formatting completed."

lint: ## Run linting with options (make lint VERBOSE=true CONFIG=custom.yml OUTPUT=json)
	@echo "Running linters..."
	$(eval FINAL_LINT_FLAGS := $(LINT_FLAGS))
	@if [ "$(VERBOSE)" = "true" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) -v"; \
	fi
	@if [ -n "$(CONFIG)" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --config=$(CONFIG)"; \
	else \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS)"; \
	fi
	@if [ -n "$(OUTPUT)" ]; then \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --out-format=$(OUTPUT)"; \
	else \
		FINAL_LINT_FLAGS="$(FINAL_LINT_FLAGS) --out-format=$(LINT_OUTPUT_FORMAT)"; \
	fi
	golangci-lint run $(FINAL_LINT_FLAGS)

format-lint: format lint ## Format code and run linters in one step

check-format: ## Check code formatting without applying changes (useful for CI)
	@echo "Checking code formatting..."
	@./scripts/check_format.sh

run: ## Start the task runner
	$(GORUN) $(MAIN_PATH)


stake: ## Stake tokens in the network
	$(GORUN) $(MAIN_PATH) stake --amount 10

balance: ## Check token balances
	$(GORUN) $(MAIN_PATH) balance

auth: ## Authenticate with the network
	$(GORUN) $(MAIN_PATH) auth

# Installation targets
deps: ## Download dependencies
	git submodule update --init --recursive
	$(GOMOD) download
	$(GOMOD) tidy

install: build ## Install parity command globally
	@echo "Installing parity to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installation complete"

uninstall: ## Remove parity command from system
	@echo "Uninstalling parity from $(INSTALL_PATH)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Uninstallation complete"

install-lint-tools: ## Install formatting and linting tools
	@echo "Installing linting and formatting tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installation complete."

install-hooks: ## Install git hooks
	@echo "Installing git hooks..."
	@./scripts/hooks/install-hooks.sh


help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
