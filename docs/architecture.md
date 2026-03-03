# Architecture

secure-inference is a gateway-level access control system for
[LLM-D](https://github.com/llm-d). It combines a policy engine,
Kubernetes controllers, and an Envoy ext-auth gRPC server in a single
binary. An optional Python sidecar handles LoRA adapter selection via
semantic similarity.

## Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/server/` | Envoy ext-auth gRPC `Check()` handler |
| `pkg/auth/` | Authenticator interface + JWT implementation |
| `pkg/policyengine/` | PolicyEngine interface + OPA implementation |
| `pkg/store/` | ReadStore/Store interfaces + in-memory implementation |
| `pkg/adapterselection/` | Optional LoRA adapter selector + semantic implementation |
| `pkg/controller/` | Kubernetes reconcilers for User and Model CRDs |
| `pkg/runnable/` | controller-runtime lifecycle adapters for gRPC |
| `pkg/types/` | Shared leaf types (InferenceRequest, AuthResult) |
| `pkg/config/` | YAML configuration loading and validation |
| `api/v1alpha1/` | Kubernetes CRD type definitions |

## Component Pattern

The pluggable components (`store`, `policyengine`, `auth`,
`adapterselection`) each follow the same structure:

- `interface.go` — defines the component interface
- `factory.go` — `NewComponentFromConfig(ctx, cfg)` constructor
- `<implementation>/` — sub-package with concrete implementation

Factories read from `ComponentConfig` (type + parameters map).
Sub-packages implement interfaces implicitly and never import
their parent, preventing import cycles.

## Layers

Each layer depends only on the layers below it:

| Layer | Packages | Role |
|-------|----------|------|
| Entry | `cmd/secure-inference/` | Wires everything together |
| Orchestration | `pkg/server/`, `pkg/controller/` | Uses all interfaces |
| Features | `pkg/auth/`, `pkg/adapterselection/` | Used by the server |
| Core | `pkg/policyengine/`, `pkg/store/` | Logic + data storage |
| Leaf | `pkg/types/`, `api/v1alpha1/` | No internal dependencies |

## Request Flow

```text
Client -> Gateway (Istio/KGW) -> ext-auth gRPC :9000
                                   |
                                   +-- Parse request -> InferenceRequest
                                   +-- Authenticate (JWT) -> AuthResult
                                   +-- Check user exists in store
                                   +-- Look up model in store
                                   +-- PolicyEngine.CheckAccess(user, model)
                                   +-- (optional) Adapter selection via sidecar
                                   |
                                   +-- ALLOW / DENY -> Gateway -> LLM-D Pool
```

Kubernetes controllers run concurrently, watching User and Model CRDs
and keeping the in-memory store populated. The store is repopulated
from CRDs on every restart — no external storage required.
