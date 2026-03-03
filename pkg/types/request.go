package types

import (
	"encoding/json"
	"fmt"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

// RequestType classifies the incoming inference request.
type RequestType string

const (
	RequestTypeCompletion     RequestType = "completion"
	RequestTypeChatCompletion RequestType = "chat_completion"
	RequestTypeListModels     RequestType = "list_models"
	RequestTypeUnknown        RequestType = "unknown"
)

// CompletionBody is the parsed body for /v1/completions requests.
type CompletionBody struct {
	Model  string
	Prompt string
}

// ChatMessage represents a single message in a chat completion request.
type ChatMessage struct {
	Role    string
	Content string
}

// ChatCompletionBody is the parsed body for /v1/chat/completions requests.
type ChatCompletionBody struct {
	Model    string
	Messages []ChatMessage
}

// InferenceRequest is the internal representation of a parsed gRPC check request.
// Fields are exported but treated as read-only after parsing.
type InferenceRequest struct {
	// Path is the HTTP path from the :path header.
	Path string

	// Type classifies the request based on its path.
	Type RequestType

	// ModelID is extracted from the request body's "model" field.
	ModelID string

	// Headers contains the original request headers (lowercase keys).
	Headers map[string]string

	// Completion is populated when Type is RequestTypeCompletion.
	Completion *CompletionBody

	// ChatCompletion is populated when Type is RequestTypeChatCompletion.
	ChatCompletion *ChatCompletionBody
}

// PromptText extracts the user's prompt text from the request,
// regardless of whether it's a completion or chat completion.
func (ir *InferenceRequest) PromptText() string {
	switch ir.Type {
	case RequestTypeCompletion:
		if ir.Completion != nil {
			return ir.Completion.Prompt
		}
	case RequestTypeChatCompletion:
		if ir.ChatCompletion != nil {
			for i := len(ir.ChatCompletion.Messages) - 1; i >= 0; i-- {
				if ir.ChatCompletion.Messages[i].Role == "user" {
					return ir.ChatCompletion.Messages[i].Content
				}
			}
		}
	}
	return ""
}

const (
	pathCompletions     = "/v1/completions"
	pathChatCompletions = "/v1/chat/completions"
	pathModels          = "/v1/models"
	headerPath          = ":path"
)

// ParseCheckRequest parses an Envoy CheckRequest into an InferenceRequest.
func ParseCheckRequest(request *authv3.CheckRequest) (*InferenceRequest, error) {
	attrs := request.GetAttributes()
	headers := attrs.GetRequest().GetHttp().GetHeaders()

	path, ok := headers[headerPath]
	if !ok {
		return nil, fmt.Errorf("missing %s header", headerPath)
	}

	ir := &InferenceRequest{
		Path:    path,
		Headers: headers,
	}

	switch path {
	case pathCompletions:
		ir.Type = RequestTypeCompletion
	case pathChatCompletions:
		ir.Type = RequestTypeChatCompletion
	case pathModels:
		ir.Type = RequestTypeListModels
	default:
		ir.Type = RequestTypeUnknown
		return ir, nil
	}

	// Parse body for completion/chat paths
	if ir.Type == RequestTypeCompletion || ir.Type == RequestTypeChatCompletion {
		bodyStr := attrs.GetRequest().GetHttp().GetBody()
		var bodyMap map[string]any
		if err := json.Unmarshal([]byte(bodyStr), &bodyMap); err != nil {
			return nil, fmt.Errorf("invalid request body: %w", err)
		}

		if model, ok := bodyMap["model"].(string); ok {
			ir.ModelID = model
		}

		if ir.Type == RequestTypeCompletion {
			prompt, _ := bodyMap["prompt"].(string) //nolint:errcheck // zero value on missing key is fine
			ir.Completion = &CompletionBody{
				Model:  ir.ModelID,
				Prompt: prompt,
			}
		} else {
			ir.ChatCompletion = &ChatCompletionBody{Model: ir.ModelID}
			if msgs, ok := bodyMap["messages"].([]any); ok {
				for _, m := range msgs {
					if msg, ok := m.(map[string]any); ok {
						role, _ := msg["role"].(string)       //nolint:errcheck // zero value on missing key is fine
						content, _ := msg["content"].(string) //nolint:errcheck // zero value on missing key is fine
						ir.ChatCompletion.Messages = append(ir.ChatCompletion.Messages, ChatMessage{
							Role:    role,
							Content: content,
						})
					}
				}
			}
		}
	}

	return ir, nil
}
