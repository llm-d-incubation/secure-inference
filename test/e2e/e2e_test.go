package e2e_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	mocksel "github.com/llm-d-incubation/secure-inference/pkg/adapterselection/mock"
	authjwt "github.com/llm-d-incubation/secure-inference/pkg/auth/jwt"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine/opa"
	"github.com/llm-d-incubation/secure-inference/pkg/server"
	"github.com/llm-d-incubation/secure-inference/pkg/store/memory"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
)

// --- Test harness ---

type testServer struct {
	store   *memory.Store
	engine  policyengine.PolicyEngine
	grpcSrv *grpc.Server
	client  authv3.AuthorizationClient
	conn    *grpc.ClientConn
	privKey *rsa.PrivateKey
	pubKey  *rsa.PublicKey
}

func newTestServer(t *testing.T, selector *mocksel.Selector, alwaysActive bool) *testServer {
	t.Helper()

	// Generate RSA key pair for JWT
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create real components
	st := memory.New()
	eng, err := opa.New(context.Background(), config.ComponentConfig{Type: "opa"})
	if err != nil {
		t.Fatalf("Failed to create OPA engine: %v", err)
	}

	// Create authenticator that uses the in-memory public key
	authenticator := &inMemoryAuthenticator{publicKey: &key.PublicKey}

	// Create ext-auth server
	extAuth := server.NewExtAuthzServer(eng, st, authenticator, selector, alwaysActive)

	// Start gRPC server on random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	authv3.RegisterAuthorizationServer(grpcSrv, extAuth.V3())

	go func() {
		_ = grpcSrv.Serve(lis) // error expected on graceful shutdown
	}()

	// Connect gRPC client
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		grpcSrv.GracefulStop()
		t.Fatalf("Failed to connect gRPC client: %v", err)
	}

	return &testServer{
		store:   st,
		engine:  eng,
		grpcSrv: grpcSrv,
		client:  authv3.NewAuthorizationClient(conn),
		conn:    conn,
		privKey: key,
		pubKey:  &key.PublicKey,
	}
}

func (ts *testServer) close(t *testing.T) {
	t.Helper()
	ts.conn.Close()
	ts.grpcSrv.GracefulStop()
}

func (ts *testServer) signToken(t *testing.T, username, role string) string {
	t.Helper()
	claims := &authjwt.UserClaims{
		Username:     username,
		Role:         role,
		Organization: "test-org",
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    config.ServerName,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(ts.privKey)
	if err != nil {
		t.Fatalf("Failed to sign JWT: %v", err)
	}
	return signed
}

func (ts *testServer) seedUser(t *testing.T, id string, attrs map[string]string) {
	t.Helper()
	if err := ts.store.SyncUser(context.Background(), &v1alpha1.UserSpec{
		Id:         id,
		Attributes: attrs,
	}); err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}
}

func (ts *testServer) seedModel(t *testing.T, model *v1alpha1.ModelSpec) {
	t.Helper()
	if err := ts.store.SyncModel(context.Background(), model); err != nil {
		t.Fatalf("Failed to seed model: %v", err)
	}
}

func (ts *testServer) check(t *testing.T, req *authv3.CheckRequest) *authv3.CheckResponse {
	t.Helper()
	resp, err := ts.client.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("gRPC Check failed: %v", err)
	}
	return resp
}

// inMemoryAuthenticator implements auth.Authenticator using a pre-loaded public key.
type inMemoryAuthenticator struct {
	publicKey *rsa.PublicKey
}

func (a *inMemoryAuthenticator) Authenticate(_ context.Context, req *types.InferenceRequest) (*types.AuthResult, error) {
	authString, ok := req.Headers["authorization"]
	if !ok {
		return nil, fmt.Errorf("missing authorization header")
	}
	const prefix = "Bearer "
	if len(authString) <= len(prefix) || authString[:len(prefix)] != prefix {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	claims, err := authjwt.ValidateJWTWithKey(authString[len(prefix):], a.publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}
	return &types.AuthResult{UserID: claims.Username}, nil
}

// --- Request builders ---

func makeCheckRequest(path, authHeader, body string) *authv3.CheckRequest {
	headers := map[string]string{":path": path}
	if authHeader != "" {
		headers["authorization"] = authHeader
	}
	return &authv3.CheckRequest{
		Attributes: &authv3.AttributeContext{
			Request: &authv3.AttributeContext_Request{
				Http: &authv3.AttributeContext_HttpRequest{
					Headers: headers,
					Body:    body,
				},
			},
		},
	}
}

func makeCheckRequestWithHeaders(path, authHeader, body string, extra map[string]string) *authv3.CheckRequest {
	headers := map[string]string{":path": path}
	if authHeader != "" {
		headers["authorization"] = authHeader
	}
	for k, v := range extra {
		headers[k] = v
	}
	return &authv3.CheckRequest{
		Attributes: &authv3.AttributeContext{
			Request: &authv3.AttributeContext_Request{
				Http: &authv3.AttributeContext_HttpRequest{
					Headers: headers,
					Body:    body,
				},
			},
		},
	}
}

// --- Response helpers ---

func responseCode(resp *authv3.CheckResponse) int32 {
	return resp.GetStatus().GetCode()
}

func resultHeader(resp *authv3.CheckResponse) string {
	if ok := resp.GetOkResponse(); ok != nil {
		for _, h := range ok.GetHeaders() {
			if h.GetHeader().GetKey() == "x-ext-authz-check-result" {
				return h.GetHeader().GetValue()
			}
		}
	}
	if denied := resp.GetDeniedResponse(); denied != nil {
		for _, h := range denied.GetHeaders() {
			if h.GetHeader().GetKey() == "x-ext-authz-check-result" {
				return h.GetHeader().GetValue()
			}
		}
	}
	return ""
}

func modelRewriteHeader(resp *authv3.CheckResponse) string {
	if ok := resp.GetOkResponse(); ok != nil {
		for _, h := range ok.GetHeaders() {
			if h.GetHeader().GetKey() == "x-gateway-model-name-rewrite" {
				return h.GetHeader().GetValue()
			}
		}
	}
	return ""
}

// --- E2E Tests ---

func TestE2E_UnprotectedPath(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	resp := ts.check(t, makeCheckRequest("/healthz", "", ""))
	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", got)
	}
	if got := resultHeader(resp); got != "not-applicable" {
		t.Errorf("Expected not-applicable, got %q", got)
	}
}

func TestE2E_InvalidJWT_Denied(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer invalid.jwt.token", ""))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_MissingAuth_Denied(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	resp := ts.check(t, makeCheckRequest("/v1/completions", "", `{"model":"test"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_UserNotRegistered_Denied(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	token := ts.signToken(t, "ghost", "admin")
	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"test"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_ListModels_AllowedForValidUser(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "systems_role"})
	token := ts.signToken(t, "alice", "systems_role")

	resp := ts.check(t, makeCheckRequest("/v1/models", "Bearer "+token, ""))
	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", got)
	}
	if got := resultHeader(resp); got != "allowed" {
		t.Errorf("Expected allowed, got %q", got)
	}
}

func TestE2E_AccessControl_Allowed(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "systems_role"})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "llama-base",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
	})
	token := ts.signToken(t, "alice", "systems_role")

	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"llama-base","prompt":"hello"}`))
	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", got)
	}
	if got := resultHeader(resp); got != "allowed" {
		t.Errorf("Expected allowed, got %q", got)
	}
}

func TestE2E_AccessControl_Denied(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "bob", map[string]string{"role": "guest"})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "restricted-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
	})
	token := ts.signToken(t, "bob", "guest")

	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"restricted-model","prompt":"hello"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_ChatCompletions(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "systems_role"})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "chat-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
	})
	token := ts.signToken(t, "alice", "systems_role")

	body := `{"model":"chat-model","messages":[{"role":"user","content":"hello"}]}`
	resp := ts.check(t, makeCheckRequest("/v1/chat/completions", "Bearer "+token, body))
	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", got)
	}
}

func TestE2E_MissingModelField_Denied(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "admin"})
	token := ts.signToken(t, "alice", "admin")

	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"prompt":"hello"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_AdapterSelection_WithHeader(t *testing.T) {
	mockSelector := &mocksel.Selector{
		Model:   &v1alpha1.ModelSpec{Id: "lora-z17"},
		Matched: true,
	}
	ts := newTestServer(t, mockSelector, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "systems_role"})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "base-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:          "lora-z17",
		Type:        v1alpha1.ModelTypeLora,
		BaseModelId: "base-model",
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
		SelectionPolicy: v1alpha1.ModelSelectionPolicy{
			Descriptions: []string{"IBM z17 technical documentation"},
		},
	})

	token := ts.signToken(t, "alice", "systems_role")
	req := makeCheckRequestWithHeaders(
		"/v1/completions", "Bearer "+token,
		`{"model":"base-model","prompt":"Tell me about IBM z17"}`,
		map[string]string{"x-adapter-selection": "true"},
	)
	resp := ts.check(t, req)

	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", got)
	}
	if got := modelRewriteHeader(resp); got != "lora-z17" {
		t.Errorf("Expected model rewrite to lora-z17, got %q", got)
	}
}

func TestE2E_PolicyUpdate_DynamicAccess(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "premium-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"premium"}},
		},
	})

	// Step 1: Add user with guest role — access denied
	ts.seedUser(t, "charlie", map[string]string{"role": "guest"})
	token := ts.signToken(t, "charlie", "guest")
	resp := ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"premium-model","prompt":"hi"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Step 1: Expected PermissionDenied, got %d", got)
	}

	// Step 2: Update user to premium role — access granted
	ts.seedUser(t, "charlie", map[string]string{"role": "premium"})
	token = ts.signToken(t, "charlie", "premium")
	resp = ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"premium-model","prompt":"hi"}`))
	if got := responseCode(resp); got != int32(codes.OK) {
		t.Errorf("Step 2: Expected OK, got %d", got)
	}

	// Step 3: Delete user — access denied
	if err := ts.store.DeleteUser(context.Background(), "charlie"); err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	resp = ts.check(t, makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"premium-model","prompt":"hi"}`))
	if got := responseCode(resp); got != int32(codes.PermissionDenied) {
		t.Errorf("Step 3: Expected PermissionDenied, got %d", got)
	}
}

func TestE2E_ConcurrentRequests(t *testing.T) {
	ts := newTestServer(t, nil, false)
	defer ts.close(t)

	ts.seedUser(t, "alice", map[string]string{"role": "systems_role"})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "concurrent-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{"role": {"systems_role"}},
		},
	})
	token := ts.signToken(t, "alice", "systems_role")

	const numRequests = 50
	var wg sync.WaitGroup
	errs := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"concurrent-model","prompt":"hello"}`)
			resp, err := ts.client.Check(context.Background(), req)
			if err != nil {
				errs <- fmt.Errorf("gRPC error: %w", err)
				return
			}
			if code := responseCode(resp); code != int32(codes.OK) {
				errs <- fmt.Errorf("expected OK, got %d", code)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
