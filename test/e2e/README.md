# End-to-End Tests

This directory contains end-to-end tests that verify the complete system deployment and functionality.

## Test Scenarios

### 1. Full System Deployment

- Deploy all components to test cluster
- Verify all pods are running
- Verify services are accessible

### 2. CRD Synchronization

- Create User CRD
- Verify sync to Decision Point
- Create Model CRD
- Verify sync to Decision Point

### 3. JWT Authentication Flow

- Generate JWT token
- Make authenticated request
- Verify token validation
- Test invalid token rejection

### 4. Access Control Enforcement

- Test allowed access
- Test denied access
- Test model filtering in /v1/models

### 5. LoRA Adapter Selection

- Request with base model + ext-proc-enable header
- Verify correct LoRA selection based on query
- Verify request modification

## Running E2E Tests

### Prerequisites

- Kind or Minikube cluster
- kubectl configured
- Helm installed
- Docker images built

### Run All E2E Tests

```bash
make test-e2e
```

### Run Individual Test

```bash
go test ./test/e2e -run TestFullSystemDeployment -v
```

## Manual E2E Testing

### 1. Deploy System

```bash
cd guides/minikube-llm-d-sim
./setup.sh
```

### 2. Apply Test Policies

```bash
kubectl apply -f config/samples/
```

### 3. Generate Tokens

```bash
./bin/llmd-auth init
export ALICE_TOKEN=$(./bin/llmd-auth create --name alice)
export BOB_TOKEN=$(./bin/llmd-auth create --name bob)
```

### 4. Test Authentication

```bash
# Should succeed
curl -H "Authorization: Bearer $ALICE_TOKEN" \
     https://gateway/v1/models

# Should fail
curl -H "Authorization: Bearer invalid" \
     https://gateway/v1/models
```

### 5. Test Access Control

```bash
# Alice can access z17 LoRA
curl -H "Authorization: Bearer $ALICE_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"model":"ibm_z17_technical_technical_introduction","prompt":"test"}' \
     https://gateway/v1/completions

# Bob cannot access z17 LoRA (should get 403)
curl -H "Authorization: Bearer $BOB_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"model":"ibm_z17_technical_technical_introduction","prompt":"test"}' \
     https://gateway/v1/completions
```

### 6. Test Adapter Selection

```bash
# Request base model with query about z17
curl -H "Authorization: Bearer $ALICE_TOKEN" \
     -H "Content-Type: application/json" \
     -H "ext-proc-enable: allow" \
     -d '{"model":"meta-llama/Llama-3.2-1B-Instruct","prompt":"Tell me about IBM z17"}' \
     https://gateway/v1/completions

# Check logs to verify z17 LoRA was selected
kubectl logs -l app=llmd-proc -n llm-d-sim
```

## Test Coverage

E2E tests should cover:

- [ ] System deployment and initialization
- [ ] User CRD creation and synchronization
- [ ] Model CRD creation and synchronization
- [ ] JWT token generation and validation
- [ ] Successful authentication
- [ ] Failed authentication (invalid token)
- [ ] Access control allow scenario
- [ ] Access control deny scenario
- [ ] Model list filtering
- [ ] LoRA adapter selection
- [ ] Policy updates (add/modify/delete)
- [ ] Concurrent request handling
- [ ] Component failure recovery
- [ ] Rolling updates

## Automated E2E Test Implementation

### Test Structure

```go
func TestE2E_FullFlow(t *testing.T) {
    // 1. Setup cluster
    // 2. Deploy components
    // 3. Apply policies
    // 4. Generate tokens
    // 5. Test requests
    // 6. Verify responses
    // 7. Cleanup
}
```

### Future Work

- Implement automated E2E tests using Ginkgo
- Add performance benchmarks
- Add chaos testing scenarios
- Add upgrade/downgrade tests
- Add backup/restore tests
