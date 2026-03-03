#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LLM_D_DIR="${SCRIPT_DIR}/llm-d"

# --- Configuration ---
NAMESPACE="llm-d-sim"
LLM_D_REPO="https://github.com/llm-d/llm-d.git"
LLM_D_COMMIT="1d59107"
GATEWAY="istio"

# --- Colors for Output ---
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# --- Prerequisites Check ---
log "Checking prerequisites..."
command -v minikube >/dev/null 2>&1 || { echo >&2 "minikube is required but not installed. Aborting."; exit 1; }
command -v helm >/dev/null 2>&1 || { echo >&2 "helm is required but not installed. Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "kubectl is required but not installed. Aborting."; exit 1; }
command -v git >/dev/null 2>&1 || { echo >&2 "git is required but not installed. Aborting."; exit 1; }
command -v go >/dev/null 2>&1 || { echo >&2 "go is required but not installed. Aborting."; exit 1; }

# --- Cloning Repository ---
log "Setting up workspace in ${LLM_D_DIR}..."

if [ -d "${LLM_D_DIR}" ]; then
    log "llm-d already exists, checking out commit ${LLM_D_COMMIT}..."
    ( cd "${LLM_D_DIR}" && git fetch && git checkout ${LLM_D_COMMIT} )
else
    log "Cloning llm-d and checking out commit ${LLM_D_COMMIT}..."
    git clone "${LLM_D_REPO}" "${LLM_D_DIR}"
    ( cd "${LLM_D_DIR}" && git checkout ${LLM_D_COMMIT} )
fi

# --- Customize llm-d Configuration ---
log "Customizing llm-d configuration..."

ISTIO_VALUES="${LLM_D_DIR}/guides/prereq/gateway-provider/common-configurations/istio.yaml"
MS_VALUES="${LLM_D_DIR}/guides/simulated-accelerators/ms-sim/values.yaml"

log "Building deployment-customizer..."
( cd "${PROJECT_ROOT}" && make build-deployment-customizer )

log "Customizing ${ISTIO_VALUES} for HTTPS support..."
"${PROJECT_ROOT}/bin/deployment-customizer" gateway "${ISTIO_VALUES}"

log "Customizing ${MS_VALUES} for LoRA support..."
"${PROJECT_ROOT}/bin/deployment-customizer" model-service "${MS_VALUES}"

# --- Minikube Setup ---
log "Starting Minikube..."
if minikube status | grep -q "Running"; then
    log "Minikube is already running."
else
    minikube start --driver docker --container-runtime docker --memory no-limit --cpus no-limit
fi

log "Creating namespace ${NAMESPACE}..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# --- Installation Steps ---
log "Installing dependencies"
"${LLM_D_DIR}/guides/prereq/client-setup/install-deps.sh"

log "Applying gateway"
( cd "${LLM_D_DIR}/guides/prereq/gateway-provider" && \
    ./install-gateway-provider-dependencies.sh && \
    helmfile apply -f ${GATEWAY}.helmfile.yaml )

log "Installing Prometheus CRDs (required by llm-d charts, no full monitoring stack)..."
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_podmonitors.yaml

# --- Static TLS Certificate (replaces cert-manager) ---
log "Generating static TLS certificate..."
( cd "${PROJECT_ROOT}" && \
    make build-cli && \
    ./bin/llmd-admin init && \
    ./bin/llmd-admin tls-cert --dns-names "llm-d.com" )

log "Creating TLS secret in Kubernetes..."
kubectl create secret tls llm-d-gateway-https-cert-secret \
    --cert="${PROJECT_ROOT}/certs/tls-cert.pem" \
    --key="${PROJECT_ROOT}/certs/tls-key.pem" \
    -n "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# --- Deploy llm-d ---
log "Installing llm-d simulated-accelerators helmfile"
( cd "${LLM_D_DIR}/guides/simulated-accelerators" && \
    helmfile apply -e ${GATEWAY} -n ${NAMESPACE} )

log "Applying http route"
kubectl apply -f "${LLM_D_DIR}/guides/simulated-accelerators/httproute.yaml" -n ${NAMESPACE}

# --- Wait for llm-d to be ready ---
log "Waiting for cluster to stabilize and llm-d pods to be ready..."

MAX_RETRIES=30
RETRY_INTERVAL=10

# First, wait until the API server is reachable again
for i in $(seq 1 ${MAX_RETRIES}); do
    if kubectl get nodes >/dev/null 2>&1; then
        log "Cluster API server is reachable."
        break
    fi
    if [ "$i" -eq "${MAX_RETRIES}" ]; then
        echo >&2 "ERROR: Cluster API server not reachable after $((MAX_RETRIES * RETRY_INTERVAL))s. Aborting."
        exit 1
    fi
    log "Cluster API server not reachable, retrying in ${RETRY_INTERVAL}s... (attempt $i/${MAX_RETRIES})"
    sleep ${RETRY_INTERVAL}
done

# Wait for all deployments in the namespace to be available
for i in $(seq 1 ${MAX_RETRIES}); do
    if kubectl wait --for=condition=available --timeout=30s deployment --all -n ${NAMESPACE} >/dev/null 2>&1; then
        log "All deployments in ${NAMESPACE} are available."
        break
    fi
    if [ "$i" -eq "${MAX_RETRIES}" ]; then
        echo >&2 "ERROR: Deployments in ${NAMESPACE} not ready after $((MAX_RETRIES * RETRY_INTERVAL))s. Aborting."
        exit 1
    fi
    log "Waiting for deployments to be ready... (attempt $i/${MAX_RETRIES})"
    sleep ${RETRY_INTERVAL}
done

# Wait for all pods to be in Running/Completed state
for i in $(seq 1 ${MAX_RETRIES}); do
    PENDING_PODS=$(kubectl get pods -n ${NAMESPACE} --no-headers 2>/dev/null | grep -v -E 'Running|Completed' | wc -l | tr -d ' ')
    if [ "${PENDING_PODS}" -eq 0 ]; then
        log "All pods in ${NAMESPACE} are running."
        break
    fi
    if [ "$i" -eq "${MAX_RETRIES}" ]; then
        echo >&2 "ERROR: ${PENDING_PODS} pod(s) still not running after $((MAX_RETRIES * RETRY_INTERVAL))s. Aborting."
        kubectl get pods -n ${NAMESPACE} --no-headers | grep -v -E 'Running|Completed'
        exit 1
    fi
    log "${PENDING_PODS} pod(s) not yet running, retrying in ${RETRY_INTERVAL}s... (attempt $i/${MAX_RETRIES})"
    sleep ${RETRY_INTERVAL}
done

success "llm-d is fully ready in namespace ${NAMESPACE}."

# --- secure-inference Deployment ---
log "Building images..."
( cd "${PROJECT_ROOT}" && \
    make build-all && \
    make image-build SIDECARS=adapter-selection-fastembed=adapter-selection-fastembed:latest )

log "Loading images to minikube..."
( cd "${PROJECT_ROOT}" && \
    make load-images-minikube \
        SIDECARS=adapter-selection-fastembed=adapter-selection-fastembed:latest \
        MINIKUBE_PROFILE=minikube )

log "Deploying secure-inference access control..."
export K8S_NAMESPACE=${NAMESPACE}
export GATEWAY_TYPE=${GATEWAY}

( cd "${PROJECT_ROOT}" && make deploy HELM_VALUES="${SCRIPT_DIR}/values.yaml" )

log "Applying sample policies..."
kubectl apply -f "${SCRIPT_DIR}/sample_policies.yaml" -n ${NAMESPACE}

success "Setup complete!"
