package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"google.golang.org/grpc/codes"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/adapterselection"
	mocksel "github.com/llm-d-incubation/secure-inference/pkg/adapterselection/mock"
	authjwt "github.com/llm-d-incubation/secure-inference/pkg/auth/jwt"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine"
	"github.com/llm-d-incubation/secure-inference/pkg/policyengine/opa"
	"github.com/llm-d-incubation/secure-inference/pkg/store/memory"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
)

// testKeyPair holds RSA keys generated for tests.
type testKeyPair struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func generateTestKeyPair(t *testing.T) *testKeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	return &testKeyPair{privateKey: key, publicKey: &key.PublicKey}
}

func signToken(t *testing.T, kp *testKeyPair, claims *authjwt.UserClaims) string {
	t.Helper()
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(kp.privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return signed
}

func newTestClaims(username, role, org string) *authjwt.UserClaims {
	return &authjwt.UserClaims{
		Username:     username,
		Role:         role,
		Organization: org,
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    config.ServerName,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}
}

// jwtAuthenticator implements auth.Authenticator using a pre-loaded public key.
// Used in tests to avoid needing a key file on disk.
type jwtAuthenticator struct {
	publicKey *rsa.PublicKey
}

func (a *jwtAuthenticator) Authenticate(_ context.Context, req *types.InferenceRequest) (*types.AuthResult, error) {
	authString, ok := req.Headers["authorization"]
	if !ok {
		return nil, fmt.Errorf("missing authorization header")
	}
	// Expect "Bearer <token>"
	const prefix = "Bearer "
	if len(authString) <= len(prefix) || authString[:len(prefix)] != prefix {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	token := authString[len(prefix):]
	claims, err := authjwt.ValidateJWTWithKey(token, a.publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}
	return &types.AuthResult{UserID: claims.Username}, nil
}

// testSetup creates an engine + store for server tests.
type testSetup struct {
	engine policyengine.PolicyEngine
	store  *memory.Store
}

func newTestSetup(t *testing.T) *testSetup {
	t.Helper()
	s := memory.New()
	engine, err := opa.New(context.Background(), config.ComponentConfig{Type: "opa"})
	if err != nil {
		t.Fatalf("Failed to create OPA engine: %v", err)
	}
	return &testSetup{
		engine: engine,
		store:  s,
	}
}

func (ts *testSetup) seedUser(t *testing.T, user *v1alpha1.UserSpec) {
	t.Helper()
	if err := ts.store.SyncUser(context.Background(), user); err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}
}

func (ts *testSetup) seedModel(t *testing.T, model *v1alpha1.ModelSpec) {
	t.Helper()
	if err := ts.store.SyncModel(context.Background(), model); err != nil {
		t.Fatalf("Failed to seed model: %v", err)
	}
}

func (ts *testSetup) server(authenticator *jwtAuthenticator) *extAuthzServerV3 {
	return &extAuthzServerV3{
		engine:        ts.engine,
		store:         ts.store,
		authenticator: authenticator,
	}
}

func (ts *testSetup) serverWithSelector(
	authenticator *jwtAuthenticator, selector adapterselection.Selector, alwaysActive bool,
) *extAuthzServerV3 {
	return &extAuthzServerV3{
		engine:                       ts.engine,
		store:                        ts.store,
		authenticator:                authenticator,
		selector:                     selector,
		alwaysActiveAdapterSelection: alwaysActive,
	}
}

func makeCheckRequest(path, authHeader, body string) *authv3.CheckRequest {
	headers := map[string]string{
		":path": path,
	}
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

func getResponseCode(resp *authv3.CheckResponse) int32 {
	return resp.GetStatus().GetCode()
}

func getResultHeader(resp *authv3.CheckResponse) string {
	if ok := resp.GetOkResponse(); ok != nil {
		for _, h := range ok.GetHeaders() {
			if h.GetHeader().GetKey() == resultHeader {
				return h.GetHeader().GetValue()
			}
		}
	}
	if denied := resp.GetDeniedResponse(); denied != nil {
		for _, h := range denied.GetHeaders() {
			if h.GetHeader().GetKey() == resultHeader {
				return h.GetHeader().GetValue()
			}
		}
	}
	return ""
}

// --- ext-auth Check() Tests ---

func TestCheck_UnprotectedPath_AllowedWithoutClaims(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	req := makeCheckRequest("/healthz", "", "")
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if getResultHeader(resp) != notApplicable {
		t.Errorf("Expected result header %s, got %s", notApplicable, getResultHeader(resp))
	}
}

func TestCheck_MissingPathHeader_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	req := &authv3.CheckRequest{
		Attributes: &authv3.AttributeContext{
			Request: &authv3.AttributeContext_Request{
				Http: &authv3.AttributeContext_HttpRequest{
					Headers: map[string]string{},
				},
			},
		},
	}
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_MissingAuthHeader_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	req := makeCheckRequest("/v1/completions", "", "")
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_InvalidBearerFormat_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	req := makeCheckRequest("/v1/completions", "NotBearer token", "")
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_InvalidJWT_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	req := makeCheckRequest("/v1/completions", "Bearer invalid.jwt.token", "")
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_UserNotInStore_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})

	token := signToken(t, kp, newTestClaims("ghost", "admin", "acme"))
	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"test-model"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_ModelsEndpoint_AllowedForValidUser(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)

	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "admin"},
	})

	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})
	token := signToken(t, kp, newTestClaims("alice", "admin", "acme"))

	req := makeCheckRequest("/v1/models", "Bearer "+token, "")
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if getResultHeader(resp) != resultAllowed {
		t.Errorf("Expected result header %s, got %s", resultAllowed, getResultHeader(resp))
	}
}

func TestCheck_CompletionsEndpoint_AccessGranted(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)

	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "test-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"systems_role"},
			},
		},
	})

	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})
	token := signToken(t, kp, newTestClaims("alice", "systems_role", "acme"))

	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"test-model"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if getResultHeader(resp) != resultAllowed {
		t.Errorf("Expected result header %s, got %s", resultAllowed, getResultHeader(resp))
	}
}

func TestCheck_CompletionsEndpoint_AccessDenied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)

	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "bob",
		Attributes: map[string]string{"role": "guest"},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "restricted-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"admin"},
			},
		},
	})

	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})
	token := signToken(t, kp, newTestClaims("bob", "guest", "acme"))

	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"restricted-model"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_MissingModelInBody_Denied(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)

	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "admin"},
	})

	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})
	token := signToken(t, kp, newTestClaims("alice", "admin", "acme"))

	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"prompt":"hello"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.PermissionDenied) {
		t.Errorf("Expected PermissionDenied, got %d", getResponseCode(resp))
	}
}

func TestCheck_ChatCompletionsPath_Works(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)

	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "chat-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"systems_role"},
			},
		},
	})

	srv := ts.server(&jwtAuthenticator{publicKey: kp.publicKey})
	token := signToken(t, kp, newTestClaims("alice", "systems_role", "acme"))

	req := makeCheckRequest("/v1/chat/completions", "Bearer "+token, `{"model":"chat-model"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
}

// makeCheckRequestWithHeaders creates a CheckRequest with extra headers.
func makeCheckRequestWithHeaders(path, authHeader, body string, extraHeaders map[string]string) *authv3.CheckRequest {
	headers := map[string]string{
		":path": path,
	}
	if authHeader != "" {
		headers["authorization"] = authHeader
	}
	for k, v := range extraHeaders {
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

func getModelRewriteHeader(resp *authv3.CheckResponse) string {
	if ok := resp.GetOkResponse(); ok != nil {
		for _, h := range ok.GetHeaders() {
			if h.GetHeader().GetKey() == modelRewriteHeader {
				return h.GetHeader().GetValue()
			}
		}
	}
	return ""
}

// seedAdapterSelectionTestData seeds a base model and a matching LoRA for adapter selection tests.
func seedAdapterSelectionTestData(t *testing.T, ts *testSetup) {
	t.Helper()
	ts.seedUser(t, &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "systems_role"},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:   "base-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"systems_role"},
			},
		},
	})
	ts.seedModel(t, &v1alpha1.ModelSpec{
		Id:          "lora-adapter-1",
		Type:        v1alpha1.ModelTypeLora,
		BaseModelId: "base-model",
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"systems_role"},
			},
		},
		SelectionPolicy: v1alpha1.ModelSelectionPolicy{
			Descriptions: []string{"A LoRA for code generation"},
		},
	})
}

func TestCheck_AdapterSelection_HeaderGated_NoHeader(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	seedAdapterSelectionTestData(t, ts)

	mockSelector := &mocksel.Selector{
		Model:   &v1alpha1.ModelSpec{Id: "lora-adapter-1"},
		Matched: true,
	}
	srv := ts.serverWithSelector(&jwtAuthenticator{publicKey: kp.publicKey}, mockSelector, false)
	token := signToken(t, kp, newTestClaims("alice", "systems_role", "acme"))

	// No x-adapter-selection header — selector should NOT be called, no rewrite
	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"base-model","prompt":"write code"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if rewrite := getModelRewriteHeader(resp); rewrite != "" {
		t.Errorf("Expected no model rewrite header, got %q", rewrite)
	}
}

func TestCheck_AdapterSelection_HeaderGated_WithHeader(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	seedAdapterSelectionTestData(t, ts)

	mockSelector := &mocksel.Selector{
		Model:   &v1alpha1.ModelSpec{Id: "lora-adapter-1"},
		Matched: true,
	}
	srv := ts.serverWithSelector(&jwtAuthenticator{publicKey: kp.publicKey}, mockSelector, false)
	token := signToken(t, kp, newTestClaims("alice", "systems_role", "acme"))

	// With x-adapter-selection: true header — selector SHOULD be called, rewrite expected
	body := `{"model":"base-model","prompt":"write code"}`
	req := makeCheckRequestWithHeaders(
		"/v1/completions", "Bearer "+token, body,
		map[string]string{"x-adapter-selection": "true"},
	)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if rewrite := getModelRewriteHeader(resp); rewrite != "lora-adapter-1" {
		t.Errorf("Expected model rewrite to lora-adapter-1, got %q", rewrite)
	}
}

func TestCheck_AdapterSelection_AlwaysActive(t *testing.T) {
	kp := generateTestKeyPair(t)
	ts := newTestSetup(t)
	seedAdapterSelectionTestData(t, ts)

	mockSelector := &mocksel.Selector{
		Model:   &v1alpha1.ModelSpec{Id: "lora-adapter-1"},
		Matched: true,
	}
	srv := ts.serverWithSelector(&jwtAuthenticator{publicKey: kp.publicKey}, mockSelector, true)
	token := signToken(t, kp, newTestClaims("alice", "systems_role", "acme"))

	// No header but alwaysActive=true — selector SHOULD be called, rewrite expected
	req := makeCheckRequest("/v1/completions", "Bearer "+token, `{"model":"base-model","prompt":"write code"}`)
	resp, err := srv.Check(context.Background(), req)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if getResponseCode(resp) != int32(codes.OK) {
		t.Errorf("Expected OK, got %d", getResponseCode(resp))
	}
	if rewrite := getModelRewriteHeader(resp); rewrite != "lora-adapter-1" {
		t.Errorf("Expected model rewrite to lora-adapter-1, got %q", rewrite)
	}
}
