#!/bin/bash

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LLM_D_DIR="${SCRIPT_DIR}/llm-d"
VLLM_DIR="${SCRIPT_DIR}/vllm"

# --- Configuration ---
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"
NAMESPACE="llm-d-cpu"
GUIDE_NAME="minikube-llm-d-cpu"

# --- Colors for Output ---
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

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

# Uninstall secure-inference helm release
if helm list -n ${NAMESPACE} 2>/dev/null | grep -q "secure-inference"; then
    log "Uninstalling secure-inference helm release..."
    helm uninstall secure-inference -n ${NAMESPACE}
    success "secure-inference uninstalled"
fi

# Uninstall scheduler helm release
if helm list -n ${NAMESPACE} 2>/dev/null | grep -q "${GUIDE_NAME}"; then
    log "Uninstalling ${GUIDE_NAME} scheduler helm release..."
    helm uninstall ${GUIDE_NAME} -n ${NAMESPACE}
    success "Scheduler uninstalled"
fi

# Delete namespace (removes model server, gateway, secrets, etc.)
if kubectl get namespace ${NAMESPACE} >/dev/null 2>&1; then
    log "Deleting namespace ${NAMESPACE}..."
    kubectl delete namespace ${NAMESPACE} --timeout=60s
    success "Namespace deleted"
fi

# Uninstall Istio
ISTIOCTL=$(find "${SCRIPT_DIR}" -name "istioctl" -path "*/bin/*" 2>/dev/null | head -1)
if [ -n "${ISTIOCTL}" ]; then
    log "Uninstalling Istio..."
    ${ISTIOCTL} uninstall --purge -y 2>/dev/null || true
    kubectl delete namespace istio-system 2>/dev/null || true
    success "Istio uninstalled"
elif command -v istioctl >/dev/null 2>&1; then
    log "Uninstalling Istio..."
    istioctl uninstall --purge -y 2>/dev/null || true
    kubectl delete namespace istio-system 2>/dev/null || true
    success "Istio uninstalled"
else
    warn "istioctl not found, skipping Istio uninstall"
fi

# Delete minikube cluster
if command -v minikube >/dev/null 2>&1; then
    log "Deleting minikube cluster (profile: ${MINIKUBE_PROFILE})..."
    minikube delete -p "${MINIKUBE_PROFILE}"
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

# Remove vllm source directory (image is cached in Docker)
if [ -d "${VLLM_DIR}" ]; then
    log "Removing ${VLLM_DIR} directory..."
    rm -rf "${VLLM_DIR}"
    success "vLLM source removed (Docker image still cached)"
fi

# Remove istioctl download if present
for dir in "${SCRIPT_DIR}"/istio-*; do
    if [ -d "$dir" ]; then
        log "Removing istio download..."
        rm -rf "${SCRIPT_DIR}"/istio-*
        success "Istio download removed"
        break
    fi
done

success "Cleanup complete!"
log "Note: The vllm-cpu:local Docker image is preserved. To remove it: docker rmi vllm-cpu:local"
