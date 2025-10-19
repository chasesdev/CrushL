package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/message"
)

// Analyzer handles semantic analysis of messages using an LLM
type Analyzer struct {
	provider provider.Provider
	model    string
}

// NewAnalyzer creates a new analyzer with the given provider and model
func NewAnalyzer(prov provider.Provider, model string) *Analyzer {
	return &Analyzer{
		provider: prov,
		model:    model,
	}
}

// ExtractTasks analyzes a user message and extracts actionable tasks
func (a *Analyzer) ExtractTasks(ctx context.Context, userMessage string, existingTasks []Task) ([]struct {
	Description string
	Priority    string
	Tags        []string
}, error) {
	prompt := GetTaskExtractionPrompt(userMessage, existingTasks)

	// Call LLM
	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM for task extraction: %w", err)
	}

	// Parse JSON response
	var result TaskExtractionResponse
	if err := a.parseJSONResponse(response, &result); err != nil {
		slog.Warn("Failed to parse task extraction response", "error", err, "response", response)
		return nil, fmt.Errorf("failed to parse task extraction response: %w", err)
	}

	var tasks []struct {
		Description string
		Priority    string
		Tags        []string
	}

	for _, task := range result.Tasks {
		tasks = append(tasks, struct {
			Description string
			Priority    string
			Tags        []string
		}{
			Description: task.Description,
			Priority:    task.Priority,
			Tags:        task.Tags,
		})
	}

	return tasks, nil
}

// AnalyzeResponse analyzes an LLM response to determine task updates
func (a *Analyzer) AnalyzeResponse(ctx context.Context, llmResponse string, currentTasks []Task) (*ResponseAnalysisResponse, error) {
	// Don't analyze empty responses
	if strings.TrimSpace(llmResponse) == "" {
		return &ResponseAnalysisResponse{
			Completed: []int{},
			Created:   []struct {
				Description string   `json:"description"`
				Priority    string   `json:"priority"`
				Tags        []string `json:"tags"`
			}{},
			Updated: []struct {
				ID     int    `json:"id"`
				Status string `json:"status"`
			}{},
		}, nil
	}

	prompt := GetResponseAnalysisPrompt(llmResponse, currentTasks)

	// Call LLM
	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM for response analysis: %w", err)
	}

	// Parse JSON response
	var result ResponseAnalysisResponse
	if err := a.parseJSONResponse(response, &result); err != nil {
		slog.Warn("Failed to parse response analysis", "error", err, "response", response)
		return nil, fmt.Errorf("failed to parse response analysis: %w", err)
	}

	return &result, nil
}

// callLLM calls the LLM provider with a prompt
func (a *Analyzer) callLLM(ctx context.Context, prompt string) (string, error) {
	messages := []message.Message{
		{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: prompt},
			},
		},
	}

	// Use StreamResponse and collect the full response
	responseChan := a.provider.StreamResponse(ctx, messages, nil)

	var fullResponse string
	for event := range responseChan {
		if event.Error != nil {
			return "", event.Error
		}
		if event.Response != nil {
			fullResponse = event.Response.Content
		}
	}

	return fullResponse, nil
}

// parseJSONResponse extracts and parses JSON from the LLM response
func (a *Analyzer) parseJSONResponse(response string, target interface{}) error {
	// Try to find JSON in the response
	jsonStr := a.extractJSON(response)
	if jsonStr == "" {
		return fmt.Errorf("no JSON found in response")
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// extractJSON attempts to extract JSON from a response
func (a *Analyzer) extractJSON(response string) string {
	// Look for JSON object
	start := strings.Index(response, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	braceCount := 0
	for i := start; i < len(response); i++ {
		switch response[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				return response[start : i+1]
			}
		}
	}

	return ""
}
