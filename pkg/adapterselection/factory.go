package adapterselection

import (
	"context"
	"fmt"

	"github.com/llm-d-incubation/secure-inference/pkg/adapterselection/semantic"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// NewAdapterSelectionFromConfig creates a Selector from an AdapterSelectionConfig.
// Returns (nil, nil) when adapter selection is disabled (type is empty).
func NewAdapterSelectionFromConfig(ctx context.Context, cfg config.AdapterSelectionConfig) (Selector, error) {
	switch cfg.Type {
	case "semantic":
		return semantic.New(ctx, cfg.ComponentConfig)
	case "":
		return nil, nil //nolint:nilnil // nil selector is the documented way to disable adapter selection
	default:
		return nil, fmt.Errorf("unknown adapter selection type: %s", cfg.Type)
	}
}
