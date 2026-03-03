#!/bin/bash

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LLM_D_DIR="${SCRIPT_DIR}/llm-d"

# --- Colors for Output ---
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo -e "${RED}[WARNING]${NC} $1"
}

# --- Cleanup ---
log "Starting cleanup..."

# Delete minikube cluster
if command -v minikube >/dev/null 2>&1; then
    log "Deleting minikube cluster..."
    minikube delete
    success "Minikube cluster deleted"
else
    warn "minikube command not found, skipping cluster deletion"
fi

# Remove llm-d workspace directory
if [ -d "${LLM_D_DIR}" ]; then
    log "Removing ${LLM_D_DIR} directory..."
    rm -rf "${LLM_D_DIR}"
    success "llm-d directory removed"
else
    warn "llm-d directory not found at ${LLM_D_DIR}, skipping"
fi

success "Cleanup complete!"
