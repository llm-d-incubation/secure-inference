# Tests

## Directory Structure

```
test/
└── e2e/          # End-to-end tests (gRPC-level)
```

## Running Tests

### Unit Tests

```bash
make test
```

Runs all unit tests in `pkg/` with envtest for controller tests.

### E2E Tests

```bash
make test-e2e
```

Starts a real gRPC ext-auth server per test, connects a client, and sends CheckRequests over the wire. Tests the full auth + store + policy engine + adapter selection pipeline.

### All Tests

```bash
make test-all
```

## Test Categories

### Unit Tests (`pkg/`)

- Test individual components in isolation
- Controller tests use envtest (real etcd + kube-apiserver)
- Run via `make test`

### E2E Tests (`test/e2e/`)

- Start a real gRPC server on a random port
- Use real components (memory store, OPA engine, JWT authenticator)
- Connect via gRPC client and send CheckRequests over the wire
- Cover: auth, access control, adapter selection, policy updates, concurrency
- Run via `make test-e2e`
