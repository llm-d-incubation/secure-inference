# Upstream Dependency Version Tracking

> This file is the source of truth for the [upstream dependency monitor](/.github/workflows/upstream-monitor.md) workflow.
> It tracks all external dependencies pinned in this repository, their current versions,
> and where they are pinned. The `upstream-monitor` agentic workflow reads this file daily
> to detect when upstream projects release new versions that may break secure-inference.

## Docker Build Dependencies

Pinned in `Dockerfile`:

| Dependency | Current Pin | Pin Type | File Location | Upstream Repo |
|-----------|-------------|----------|---------------|---------------|
| **Go (builder)** | `1.26` | version | `Dockerfile` line 5 (`FROM golang:1.26`) | [golang/go](https://github.com/golang/go) |
| **Distroless static** | `nonroot` | tag | `Dockerfile` line 18 (`FROM gcr.io/distroless/static:nonroot`) | [GoogleContainerTools/distroless](https://github.com/GoogleContainerTools/distroless) |

## Go Module Dependencies

Pinned in `go.mod`:

| Dependency | Current Pin | Pin Type | File Location | Upstream Repo |
|-----------|-------------|----------|---------------|---------------|
| **Go toolchain** | `1.24.0` | version | `go.mod` line 3 | [golang/go](https://github.com/golang/go) |
