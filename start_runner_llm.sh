#!/bin/bash

set -e

# Configuration
DEFAULT_MODELS="llama2,mistral"
DEFAULT_OLLAMA_URL="http://localhost:11434"
AUTO_INSTALL="true"

# Parse command line arguments
MODELS="$DEFAULT_MODELS"
OLLAMA_URL="$DEFAULT_OLLAMA_URL"
AUTO_INSTALL_FLAG="--auto-install"

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -m, --models MODEL_LIST     Comma-separated list of models (default: $DEFAULT_MODELS)"
    echo "  -u, --ollama-url URL        Ollama server URL (default: $DEFAULT_OLLAMA_URL)"
    echo "  --no-auto-install          Disable automatic Ollama installation"
    echo "  -h, --help                  Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                          # Start with default models"
    echo "  $0 -m llama2,codellama      # Start with specific models"
    echo "  $0 -m llama2 -u http://remote:11434  # Use remote Ollama server"
    echo "  $0 --no-auto-install       # Don't auto-install Ollama"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--models)
            MODELS="$2"
            shift 2
            ;;
        -u|--ollama-url)
            OLLAMA_URL="$2"
            shift 2
            ;;
        --no-auto-install)
            AUTO_INSTALL_FLAG=""
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

echo "🚀 Starting Parity Runner with LLM capabilities..."
echo "Models: $MODELS"
echo "Ollama URL: $OLLAMA_URL"
echo "Auto-install: $([ -n "$AUTO_INSTALL_FLAG" ] && echo "enabled" || echo "disabled")"
echo ""

# Check if binary exists
if [ ! -f "./parity-runner" ]; then
    echo "❌ parity-runner binary not found. Please build it first:"
    echo "   go build -o parity-runner cmd/main.go"
    exit 1
fi

# Convert comma-separated models to repeated --models flags
MODEL_FLAGS=""
IFS=',' read -ra MODEL_ARRAY <<< "$MODELS"
for model in "${MODEL_ARRAY[@]}"; do
    model=$(echo "$model" | xargs)  # Trim whitespace
    MODEL_FLAGS="$MODEL_FLAGS$model,"
done
MODEL_FLAGS=${MODEL_FLAGS%,}  # Remove trailing comma

# Start the runner
echo "🔄 Starting runner..."
exec ./parity-runner runner \
    --models "$MODEL_FLAGS" \
    --ollama-url "$OLLAMA_URL" \
    $AUTO_INSTALL_FLAG 