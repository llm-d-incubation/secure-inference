package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
	"github.com/llm-d-incubation/secure-inference/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("adapter-selector")

// pickRequest is the JSON body sent to the sidecar /pick endpoint.
type pickRequest struct {
	Prompt     string      `json:"prompt"`
	Candidates []candidate `json:"candidates"`
}

// candidate is a LoRA model candidate for selection.
type candidate struct {
	ModelID      string   `json:"model_id"`
	Descriptions []string `json:"descriptions"`
}

// pickResponse is the JSON body returned from the sidecar /pick endpoint.
// The sidecar returns the best candidate and its score; thresholding is done Go-side.
type pickResponse struct {
	ModelID string  `json:"model_id"`
	Score   float64 `json:"score"`
}

const pickEndpoint = "/pick"

// Selector implements adapterselection.Selector via HTTP calls to a Python sidecar
// that uses sentence-transformer embeddings for semantic similarity matching.
type Selector struct {
	url       string
	threshold float64
	client    *http.Client
}

// New creates a new semantic adapter selector from a ComponentConfig.
// Required parameters: "url". Optional: "similarityThreshold" (default 0.7).
func New(ctx context.Context, cfg config.ComponentConfig) (*Selector, error) {
	url := cfg.Parameters["url"]
	if url == "" {
		return nil, fmt.Errorf("adapter selection semantic: 'url' parameter is required")
	}

	threshold := 0.7
	if t, ok := cfg.Parameters["similarityThreshold"]; ok {
		parsed, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return nil, fmt.Errorf("adapter selection semantic: invalid similarityThreshold %q: %w", t, err)
		}
		threshold = parsed
	}

	return &Selector{
		url:       url,
		threshold: threshold,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

// Pick selects the best LoRA adapter by calling the sidecar /pick endpoint.
// Returns the selected ModelSpec and true if a match above the threshold was found.
func (s *Selector) Pick(
	ctx context.Context, req *types.InferenceRequest, allowedModels []*v1alpha1.ModelSpec,
) (*v1alpha1.ModelSpec, bool) {
	promptText := req.PromptText()
	if promptText == "" {
		return nil, false
	}

	// Build a lookup map and candidates from allowed LoRA models that match the base model
	modelByID := make(map[string]*v1alpha1.ModelSpec, len(allowedModels))
	var candidates []candidate
	for _, m := range allowedModels {
		if m.Type != v1alpha1.ModelTypeLora || m.BaseModelId != req.ModelID {
			continue
		}
		if len(m.SelectionPolicy.Descriptions) == 0 {
			continue
		}
		modelByID[m.Id] = m
		candidates = append(candidates, candidate{
			ModelID:      m.Id,
			Descriptions: m.SelectionPolicy.Descriptions,
		})
	}
	if len(candidates) == 0 {
		return nil, false
	}

	// Build and send request
	body, err := json.Marshal(pickRequest{
		Prompt:     promptText,
		Candidates: candidates,
	})
	if err != nil {
		logger.Error(err, "Failed to marshal request")
		return nil, false
	}

	sidecarURL := s.url + pickEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, sidecarURL, bytes.NewReader(body))
	if err != nil {
		logger.Error(err, "Failed to create request")
		return nil, false
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		logger.Error(err, "Sidecar request failed")
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.V(1).Info("Sidecar returned non-OK status", "status", resp.StatusCode)
		return nil, false
	}

	var result pickResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error(err, "Failed to decode response")
		return nil, false
	}

	if result.Score < s.threshold {
		logger.V(1).Info("Score below threshold", "score", result.Score, "threshold", s.threshold, "model", result.ModelID)
		return nil, false
	}

	selected, ok := modelByID[result.ModelID]
	if !ok {
		logger.V(1).Info("Unknown model ID from sidecar", "model", result.ModelID)
		return nil, false
	}

	logger.Info("Adapter selected", "model", result.ModelID, "score", result.Score, "threshold", s.threshold)
	return selected, true
}

// String returns a description of the selector for logging.
func (s *Selector) String() string {
	return fmt.Sprintf("SemanticSelector{url=%s, threshold=%.2f}", s.url, s.threshold)
}
