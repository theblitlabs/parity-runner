#!/bin/bash

# Ollama Docker Management Script for Parity Runner
# This script helps manage the Ollama Docker container

set -e

CONTAINER_NAME="ollama-runner"
DOCKER_IMAGE="ollama/ollama:latest"
PORT="11434"
MODEL_VOLUME="$HOME/.ollama"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi
}

pull_image() {
    log_info "Pulling Ollama Docker image..."
    docker pull "$DOCKER_IMAGE"
    log_success "Ollama image pulled successfully"
}

start_container() {
    # Stop and remove existing container if it exists
    if docker ps -a --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Stopping existing container..."
        docker stop "$CONTAINER_NAME" 2>/dev/null || true
        log_info "Removing existing container..."
        docker rm "$CONTAINER_NAME" 2>/dev/null || true
    fi
    
    # Create models directory
    mkdir -p "$MODEL_VOLUME"
    
    log_info "Starting Ollama container..."
    
    # Check for NVIDIA GPU support
    GPU_ARGS=""
    if docker info --format '{{json .Runtimes}}' 2>/dev/null | grep -q nvidia; then
        GPU_ARGS="--gpus all"
        log_info "NVIDIA GPU support detected and enabled"
    fi
    
    # Start the container
    CONTAINER_ID=$(docker run -d \
        --name "$CONTAINER_NAME" \
        -p "$PORT:11434" \
        -v "$MODEL_VOLUME:/root/.ollama" \
        --restart unless-stopped \
        $GPU_ARGS \
        "$DOCKER_IMAGE")
    
    log_success "Ollama container started with ID: ${CONTAINER_ID:0:12}"
    
    # Wait for container to be ready
    log_info "Waiting for Ollama to be ready..."
    for i in {1..30}; do
        if curl -s "http://localhost:$PORT/api/version" > /dev/null 2>&1; then
            log_success "Ollama is ready!"
            return 0
        fi
        sleep 2
        echo -n "."
    done
    
    log_error "Ollama failed to start within 60 seconds"
    show_logs
    return 1
}

stop_container() {
    if docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Stopping Ollama container..."
        docker stop "$CONTAINER_NAME"
        log_success "Ollama container stopped"
    else
        log_warning "Container is not running"
    fi
}

remove_container() {
    if docker ps -a --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Removing Ollama container..."
        docker rm "$CONTAINER_NAME"
        log_success "Ollama container removed"
    else
        log_warning "Container does not exist"
    fi
}

show_status() {
    echo "=== Ollama Docker Status ==="
    
    # Check if image exists
    if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^$DOCKER_IMAGE$"; then
        log_success "Docker image: $DOCKER_IMAGE (available)"
    else
        log_warning "Docker image: $DOCKER_IMAGE (not found)"
    fi
    
    # Check container status
    if docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_success "Container: $CONTAINER_NAME (running)"
        
        # Check if API is responding
        if curl -s "http://localhost:$PORT/api/version" > /dev/null 2>&1; then
            log_success "API: http://localhost:$PORT (responding)"
        else
            log_warning "API: http://localhost:$PORT (not responding)"
        fi
    elif docker ps -a --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_warning "Container: $CONTAINER_NAME (stopped)"
    else
        log_warning "Container: $CONTAINER_NAME (not found)"
    fi
    
    # Show model volume
    log_info "Model volume: $MODEL_VOLUME"
    if [ -d "$MODEL_VOLUME" ]; then
        MODEL_COUNT=$(find "$MODEL_VOLUME" -name "*.bin" 2>/dev/null | wc -l || echo "0")
        log_info "Models in volume: $MODEL_COUNT files"
    fi
}

show_logs() {
    if docker ps -a --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Showing container logs (last 50 lines)..."
        docker logs --tail 50 "$CONTAINER_NAME"
    else
        log_error "Container not found"
    fi
}

list_models() {
    if docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Listing models in container..."
        docker exec "$CONTAINER_NAME" ollama list
    else
        log_error "Container is not running"
    fi
}

pull_model() {
    local model_name="$1"
    if [ -z "$model_name" ]; then
        log_error "Please specify a model name"
        exit 1
    fi
    
    if docker ps --filter "name=$CONTAINER_NAME" --format "{{.Names}}" | grep -q "^$CONTAINER_NAME$"; then
        log_info "Pulling model: $model_name"
        docker exec -it "$CONTAINER_NAME" ollama pull "$model_name"
    else
        log_error "Container is not running"
    fi
}

show_usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start      Start Ollama container"
    echo "  stop       Stop Ollama container"
    echo "  restart    Restart Ollama container"
    echo "  remove     Remove Ollama container"
    echo "  status     Show container status"
    echo "  logs       Show container logs"
    echo "  pull       Pull Ollama Docker image"
    echo "  models     List installed models"
    echo "  pull-model MODEL_NAME   Pull a specific model"
    echo "  cleanup    Stop and remove container"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 pull-model llama2:7b"
    echo "  $0 status"
}

# Main command handling
case "${1:-}" in
    start)
        check_docker
        start_container
        ;;
    stop)
        check_docker
        stop_container
        ;;
    restart)
        check_docker
        stop_container
        start_container
        ;;
    remove)
        check_docker
        remove_container
        ;;
    status)
        check_docker
        show_status
        ;;
    logs)
        check_docker
        show_logs
        ;;
    pull)
        check_docker
        pull_image
        ;;
    models)
        check_docker
        list_models
        ;;
    pull-model)
        check_docker
        pull_model "$2"
        ;;
    cleanup)
        check_docker
        stop_container
        remove_container
        ;;
    *)
        show_usage
        exit 1
        ;;
esac 