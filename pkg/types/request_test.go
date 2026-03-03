package types

import (
	"testing"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

func makeCheckRequest(path, body string) *authv3.CheckRequest {
	return &authv3.CheckRequest{
		Attributes: &authv3.AttributeContext{
			Request: &authv3.AttributeContext_Request{
				Http: &authv3.AttributeContext_HttpRequest{
					Headers: map[string]string{
						":path": path,
					},
					Body: body,
				},
			},
		},
	}
}

func TestParseCheckRequest_Completion(t *testing.T) {
	req := makeCheckRequest("/v1/completions", `{"model":"llama-3","prompt":"hello"}`)
	ir, err := ParseCheckRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir.Type != RequestTypeCompletion {
		t.Errorf("expected type %q, got %q", RequestTypeCompletion, ir.Type)
	}
	if ir.ModelID != "llama-3" {
		t.Errorf("expected model llama-3, got %q", ir.ModelID)
	}
	if ir.Completion == nil {
		t.Fatal("expected Completion to be populated")
	}
	if ir.Completion.Prompt != "hello" {
		t.Errorf("expected prompt hello, got %q", ir.Completion.Prompt)
	}
}

func TestParseCheckRequest_ChatCompletion(t *testing.T) {
	req := makeCheckRequest("/v1/chat/completions", `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	ir, err := ParseCheckRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir.Type != RequestTypeChatCompletion {
		t.Errorf("expected type %q, got %q", RequestTypeChatCompletion, ir.Type)
	}
	if ir.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", ir.ModelID)
	}
	if ir.ChatCompletion == nil {
		t.Fatal("expected ChatCompletion to be populated")
	}
	if len(ir.ChatCompletion.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(ir.ChatCompletion.Messages))
	}
	if ir.ChatCompletion.Messages[0].Content != "hi" {
		t.Errorf("expected message content hi, got %q", ir.ChatCompletion.Messages[0].Content)
	}
}

func TestParseCheckRequest_ListModels(t *testing.T) {
	req := makeCheckRequest("/v1/models", "")
	ir, err := ParseCheckRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir.Type != RequestTypeListModels {
		t.Errorf("expected type %q, got %q", RequestTypeListModels, ir.Type)
	}
}

func TestParseCheckRequest_UnknownPath(t *testing.T) {
	req := makeCheckRequest("/v1/embeddings", "")
	ir, err := ParseCheckRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir.Type != RequestTypeUnknown {
		t.Errorf("expected type %q, got %q", RequestTypeUnknown, ir.Type)
	}
}

func TestParseCheckRequest_MissingPathHeader(t *testing.T) {
	req := &authv3.CheckRequest{
		Attributes: &authv3.AttributeContext{
			Request: &authv3.AttributeContext_Request{
				Http: &authv3.AttributeContext_HttpRequest{
					Headers: map[string]string{},
				},
			},
		},
	}
	_, err := ParseCheckRequest(req)
	if err == nil {
		t.Fatal("expected error for missing :path header")
	}
}

func TestParseCheckRequest_InvalidBody(t *testing.T) {
	req := makeCheckRequest("/v1/completions", "not json")
	_, err := ParseCheckRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
}

func TestPromptText_Completion(t *testing.T) {
	ir := &InferenceRequest{
		Type:       RequestTypeCompletion,
		Completion: &CompletionBody{Prompt: "test prompt"},
	}
	if got := ir.PromptText(); got != "test prompt" {
		t.Errorf("expected 'test prompt', got %q", got)
	}
}

func TestPromptText_ChatCompletion(t *testing.T) {
	ir := &InferenceRequest{
		Type: RequestTypeChatCompletion,
		ChatCompletion: &ChatCompletionBody{
			Messages: []ChatMessage{
				{Role: "system", Content: "you are helpful"},
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi there"},
				{Role: "user", Content: "latest message"},
			},
		},
	}
	if got := ir.PromptText(); got != "latest message" {
		t.Errorf("expected 'latest message', got %q", got)
	}
}

func TestPromptText_Empty(t *testing.T) {
	ir := &InferenceRequest{Type: RequestTypeListModels}
	if got := ir.PromptText(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
