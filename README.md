# secure-inference

[![CI](https://github.com/llm-d-incubation/secure-inference/actions/workflows/ci-pr-checks.yaml/badge.svg)](https://github.com/llm-d-incubation/secure-inference/actions/workflows/ci-pr-checks.yaml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

secure-inference is a gateway level access control system for [LLM-D](https://github.com/llm-d). It provides JWT-based authentication and attribute-based access control (ABAC) for LLM inference requests, operating independently of LLM-D internals.

## About

This provides an Envoy [ext-auth] compatible gRPC server that sits in front of LLM-D inference pools. It validates JWT tokens, looks up users and models from Kubernetes CRDs, and evaluates access policies using OPA — all in a single binary.

For details on the internal structure, component patterns, and dependency layers, see the [Architecture Documentation].

### How It Works

Admins define `User` and `Model` CRDs. Kubernetes controllers sync these
into an in-memory store. When a request arrives, the ext-auth server
validates the JWT, looks up the user and model, and asks the OPA policy
engine whether access is allowed. Optionally, for base model requests,
a Python sidecar selects the best LoRA adapter via semantic similarity.

See [User and Model CRDs](./docs/crds.md) for the full CRD reference and access policy details.

## Prerequisites

- Go 1.24+
- Docker (for container builds)
- [pre-commit](https://pre-commit.com/) (for local development)

## Quick Start

```bash
# Clone the repo
git clone https://github.com/llm-d-incubation/secure-inference.git
cd secure-inference

# Install pre-commit hooks
pre-commit install

# Build
make build

# Run tests
make test

# Run linters
make lint
```

### Common Commands

```bash
make help           # Show all available targets
make build          # Build secure-inference binary
make build-all      # Build all binaries (main + CLI + deployment-customizer)
make test           # Run unit tests
make test-e2e       # Run e2e tests
make lint           # Run Go and Python linters
make fmt            # Format Go and Python code
make image-build    # Build Docker images
make pre-commit     # Run pre-commit hooks
make deploy         # Deploy all components to cluster
```

## Getting Started

For local development and deployment, see the [Minikube Guide].

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines, coding standards, and how to submit changes.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

All commits must be signed off (DCO). See [PR_SIGNOFF.md](PR_SIGNOFF.md) for instructions.

For large changes please [create an issue] first describing the change so the maintainers can do an assessment.

## Security

To report a security vulnerability, please see [SECURITY.md](SECURITY.md).

## License

This project is licensed under the Apache License 2.0 - see [LICENSE](LICENSE) for details.

[Architecture Documentation]:docs/architecture.md
[Minikube Guide]:guides/minikube-llm-d-sim/readme.md
[ext-auth]:https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter
[create an issue]:https://github.com/llm-d-incubation/secure-inference/issues/new
