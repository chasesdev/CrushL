package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ModelDetector handles auto-detection of loaded models in LM Studio
type ModelDetector struct {
	baseURL         string
	modelPreference []string // Ordered list of preferred models
	fallbackModel   string
	httpClient      *http.Client
}

// NewModelDetector creates a new model detector
func NewModelDetector(baseURL string, preferences []string, fallback string) *ModelDetector {
	return &ModelDetector{
		baseURL:         baseURL,
		modelPreference: preferences,
		fallbackModel:   fallback,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ModelsResponse represents the response from /v1/models endpoint
type ModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// DetectBestModel queries LM Studio and returns the best available model
func (d *ModelDetector) DetectBestModel(ctx context.Context) (string, error) {
	models, err := d.queryLoadedModels(ctx)
	if err != nil {
		slog.Warn("Failed to query loaded models, using fallback", "error", err, "fallback", d.fallbackModel)
		return d.fallbackModel, nil
	}

	if len(models) == 0 {
		slog.Warn("No models loaded in LM Studio, using fallback", "fallback", d.fallbackModel)
		return d.fallbackModel, nil
	}

	// If preferences are specified, try to match in order
	if len(d.modelPreference) > 0 {
		for _, preferred := range d.modelPreference {
			for _, model := range models {
				if d.matchesModel(model, preferred) {
					slog.Info("Selected preferred model", "model", model)
					return model, nil
				}
			}
		}
	}

	// Auto-detect based on model name patterns (best to worst)
	// Priority: glm > qwen3-next > qwen3-8b
	for _, model := range models {
		if d.isGLMModel(model) {
			slog.Info("Auto-detected GLM model (best available)", "model", model)
			return model, nil
		}
	}

	for _, model := range models {
		if d.isQwen3Next(model) {
			slog.Info("Auto-detected Qwen3-Next model (high-end)", "model", model)
			return model, nil
		}
	}

	for _, model := range models {
		if d.isQwen38B(model) {
			slog.Info("Auto-detected Qwen3-8B model (standard)", "model", model)
			return model, nil
		}
	}

	// Use first available model if no known patterns match
	if len(models) > 0 {
		slog.Info("Using first available model", "model", models[0])
		return models[0], nil
	}

	return d.fallbackModel, nil
}

// queryLoadedModels queries the LM Studio API for loaded models
func (d *ModelDetector) queryLoadedModels(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/v1/models", strings.TrimSuffix(d.baseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var modelIDs []string
	for _, model := range modelsResp.Data {
		modelIDs = append(modelIDs, model.ID)
	}

	return modelIDs, nil
}

// matchesModel checks if a model ID matches a preference string
func (d *ModelDetector) matchesModel(modelID, preference string) bool {
	return strings.Contains(strings.ToLower(modelID), strings.ToLower(preference))
}

// isGLMModel checks if this is a GLM model
func (d *ModelDetector) isGLMModel(modelID string) bool {
	lower := strings.ToLower(modelID)
	return strings.Contains(lower, "glm") &&
		(strings.Contains(lower, "4.6") || strings.Contains(lower, "46"))
}

// isQwen3Next checks if this is a Qwen3-Next model
func (d *ModelDetector) isQwen3Next(modelID string) bool {
	lower := strings.ToLower(modelID)
	return strings.Contains(lower, "qwen") &&
		(strings.Contains(lower, "next") || strings.Contains(lower, "80b"))
}

// isQwen38B checks if this is a Qwen3-8B model
func (d *ModelDetector) isQwen38B(modelID string) bool {
	lower := strings.ToLower(modelID)
	return strings.Contains(lower, "qwen") &&
		strings.Contains(lower, "8b") &&
		!strings.Contains(lower, "80b")
}
