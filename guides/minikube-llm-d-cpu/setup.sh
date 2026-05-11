#!/bin/bash
set -e

# --- Paths ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LLM_D_DIR="${SCRIPT_DIR}/llm-d"
VLLM_DIR="${SCRIPT_DIR}/vllm"

# --- Configuration ---
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"
NAMESPACE="llm-d-cpu"
GUIDE_NAME="minikube-llm-d-cpu"
LLM_D_REPO="https://github.com/llm-d/llm-d.git"
LLM_D_COMMIT="83718227b22db83ed8e77d1c20dcade088153f33"
GAIE_VERSION="v1.5.0"
ISTIO_VERSION="1.29.0"
VLLM_CPU_IMAGE_X86="ghcr.io/llm-d/llm-d-cpu:v0.6.0"
VLLM_CPU_IMAGE_ARM="vllm-cpu:local"
VLLM_VERSION="v0.20.2"

# --- Architecture Detection ---
ARCH=$(uname -m)
case "$ARCH" in
    arm64|aarch64) PLATFORM="arm64" ;;
    x86_64)        PLATFORM="amd64" ;;
    *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# --- Colors for Output ---
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log "Detected platform: ${PLATFORM}"

# --- Prerequisites Check ---
log "Checking prerequisites..."
command -v minikube >/dev/null 2>&1 || { echo >&2 "minikube is required but not installed. Aborting."; exit 1; }
command -v helm >/dev/null 2>&1 || { echo >&2 "helm is required but not installed. Aborting."; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo >&2 "kubectl is required but not installed. Aborting."; exit 1; }
command -v git >/dev/null 2>&1 || { echo >&2 "git is required but not installed. Aborting."; exit 1; }
command -v go >/dev/null 2>&1 || { echo >&2 "go is required but not installed. Aborting."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo >&2 "docker is required but not installed. Aborting."; exit 1; }

# --- Prepare vLLM CPU Image ---
if [ "${PLATFORM}" = "amd64" ]; then
    VLLM_CPU_IMAGE="${VLLM_CPU_IMAGE_X86}"
    log "Using pre-built vLLM CPU image: ${VLLM_CPU_IMAGE}"
else
    VLLM_CPU_IMAGE="${VLLM_CPU_IMAGE_ARM}"
    if docker image inspect "${VLLM_CPU_IMAGE}" >/dev/null 2>&1; then
        log "vLLM CPU image '${VLLM_CPU_IMAGE}' already exists, skipping build."
    else
        log "Building vLLM CPU image for ARM64 (this takes 30-60 minutes, only needed once)..."

        if [ ! -d "${VLLM_DIR}" ]; then
            log "Cloning vLLM ${VLLM_VERSION}..."
            git clone --depth 1 --branch "${VLLM_VERSION}" https://github.com/vllm-project/vllm.git "${VLLM_DIR}"
        fi

        log "Building Docker image (platform: arm64, max_jobs=2)..."
        ( cd "${VLLM_DIR}" && \
            docker build -f docker/Dockerfile.cpu \
                --platform=linux/arm64 \
                --build-arg VLLM_CPU_ARM_BF16=true \
                --build-arg max_jobs=2 \
                --tag "${VLLM_CPU_IMAGE}" \
                --target vllm-openai . )

        success "vLLM CPU image built successfully."
    fi
fi

# --- Clone llm-d Repository ---
log "Setting up workspace in ${LLM_D_DIR}..."

if [ -d "${LLM_D_DIR}" ]; then
    log "llm-d already exists, checking out commit ${LLM_D_COMMIT}..."
    ( cd "${LLM_D_DIR}" && git fetch && git checkout ${LLM_D_COMMIT} )
else
    log "Cloning llm-d and checking out commit ${LLM_D_COMMIT}..."
    git clone "${LLM_D_REPO}" "${LLM_D_DIR}"
    ( cd "${LLM_D_DIR}" && git checkout ${LLM_D_COMMIT} )
fi

# --- Install Client Tools ---
log "Installing client tools (kustomize, helmfile, etc.)..."
"${LLM_D_DIR}/helpers/client-setup/install-deps.sh"

# --- Minikube Setup ---
log "Starting Minikube (profile: ${MINIKUBE_PROFILE})..."
if minikube status -p "${MINIKUBE_PROFILE}" 2>/dev/null | grep -q "Running"; then
    log "Minikube is already running."
else
    minikube start -p "${MINIKUBE_PROFILE}" --driver docker --memory 10g --cpus 6
fi

log "Creating namespace ${NAMESPACE}..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# --- Load vLLM CPU Image into Minikube ---
if [ "${PLATFORM}" = "arm64" ]; then
    log "Loading locally built vLLM CPU image into minikube..."
    minikube image load "${VLLM_CPU_IMAGE}" -p "${MINIKUBE_PROFILE}"
fi

# --- Install Gateway API and Inference Extension CRDs ---
log "Installing Gateway API CRDs..."
GATEWAY_API_VERSION="v1.5.1"
kubectl apply -k "https://github.com/kubernetes-sigs/gateway-api/config/crd?ref=${GATEWAY_API_VERSION}"

log "Installing Gateway API Inference Extension CRDs (${GAIE_VERSION})..."
kubectl apply -k "https://github.com/kubernetes-sigs/gateway-api-inference-extension/config/crd?ref=${GAIE_VERSION}"

# --- Install Istio ---
log "Installing Istio ${ISTIO_VERSION}..."
ISTIO_DIR="${SCRIPT_DIR}/istio-${ISTIO_VERSION}"
if [ ! -d "${ISTIO_DIR}" ]; then
    log "Downloading istioctl ${ISTIO_VERSION}..."
    ( cd "${SCRIPT_DIR}" && curl -sL https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh - )
fi
ISTIOCTL="${ISTIO_DIR}/bin/istioctl"

${ISTIOCTL} install -y \
    --set values.pilot.env.ENABLE_GATEWAY_API_INFERENCE_EXTENSION=true

log "Waiting for Istio to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/istiod -n istio-system

# --- Static TLS Certificate ---
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

# --- Deploy Gateway ---
log "Deploying HTTPS Gateway..."
kubectl apply -k "${SCRIPT_DIR}/kustomize/gateway" -n ${NAMESPACE}

# --- Deploy llm-d Scheduler (InferencePool) ---
log "Installing llm-d scheduler via OCI helm chart..."
helm install ${GUIDE_NAME} \
    oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool \
    -f "${LLM_D_DIR}/guides/recipes/scheduler/base.values.yaml" \
    -f "${SCRIPT_DIR}/kustomize/scheduler/values.yaml" \
    --set provider.name=istio \
    --set experimentalHttpRoute.enabled=true \
    --set experimentalHttpRoute.inferenceGatewayName=llm-d-cpu-inference-gateway \
    -n ${NAMESPACE} --version ${GAIE_VERSION}

# --- Deploy Model Server ---
log "Deploying model server (vLLM CPU with LoRA adapters)..."
log "Using image: ${VLLM_CPU_IMAGE}"

# Create HF token secret if HF_TOKEN env var is set
if [ -n "${HF_TOKEN}" ]; then
    log "Creating HuggingFace token secret..."
    kubectl create secret generic hf-token \
        --from-literal=token="${HF_TOKEN}" \
        -n "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
else
    warn "HF_TOKEN not set. Set HF_TOKEN if model download fails."
fi

kubectl kustomize "${SCRIPT_DIR}/kustomize/modelserver" | \
    sed "s|image: VLLM_CPU_IMAGE|image: ${VLLM_CPU_IMAGE}|g" | \
    kubectl apply -n ${NAMESPACE} -f -

# --- Wait for Model Server to be Ready ---
log "Waiting for model server to be ready (this may take 5-10 minutes)..."

MAX_RETRIES=60
RETRY_INTERVAL=15

for i in $(seq 1 ${MAX_RETRIES}); do
    if kubectl wait --for=condition=available --timeout=30s deployment/decode -n ${NAMESPACE} >/dev/null 2>&1; then
        log "Model server deployment is available."
        break
    fi
    if [ "$i" -eq "${MAX_RETRIES}" ]; then
        echo >&2 "ERROR: Model server not ready after $((MAX_RETRIES * RETRY_INTERVAL))s. Aborting."
        echo >&2 "Check pod logs: kubectl logs -n ${NAMESPACE} -l llm-d.ai/role=decode --all-containers"
        exit 1
    fi
    log "Model server not yet ready, retrying in ${RETRY_INTERVAL}s... (attempt $i/${MAX_RETRIES})"
    sleep ${RETRY_INTERVAL}
done

success "llm-d model server is ready in namespace ${NAMESPACE}."

# --- secure-inference Deployment ---
log "Building secure-inference images..."
( cd "${PROJECT_ROOT}" && \
    make build-all && \
    make image-build SIDECARS=adapter-selection-fastembed=adapter-selection-fastembed:latest )

log "Loading images to minikube..."
( cd "${PROJECT_ROOT}" && \
    make load-images-minikube \
        SIDECARS=adapter-selection-fastembed=adapter-selection-fastembed:latest \
        MINIKUBE_PROFILE="${MINIKUBE_PROFILE}" )

log "Deploying secure-inference access control..."
export K8S_NAMESPACE=${NAMESPACE}
export GATEWAY_TYPE=istio

( cd "${PROJECT_ROOT}" && make deploy HELM_VALUES="${SCRIPT_DIR}/values.yaml" )

log "Applying sample policies..."
kubectl apply -f "${SCRIPT_DIR}/sample_policies.yaml" -n ${NAMESPACE}

success "Setup complete!"
echo ""
log "To test, run:"
log "  kubectl port-forward svc/llm-d-cpu-inference-gateway-istio 8443:443 -n ${NAMESPACE} &"
log "  export JWT=\$(cd ${PROJECT_ROOT} && ./bin/llmd-admin create --name alice)"
log "  curl -vik --connect-to llm-d.com:443:localhost:8443 https://llm-d.com:443/v1/completions \\"
log "    --header \"Authorization: Bearer \$JWT\" \\"
log "    --header \"Content-Type: application/json\" \\"
log "    --data '{\"model\": \"meta-llama/Llama-3.2-1B-Instruct\", \"prompt\": \"What is Kubernetes?\", \"max_tokens\": 100}'"
