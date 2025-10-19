// Package reasoning provides intelligent task management and reasoning capabilities
// that analyze user requests and LLM responses to automatically manage tasks.
package reasoning

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/message"
)

// Layer provides reasoning capabilities for intelligent task management
type Layer struct {
	detector       *ModelDetector
	analyzer       *Analyzer
	stores         map[string]*TaskStore // sessionID -> TaskStore
	storesMu       sync.RWMutex
	dataDir        string
	autoCreate     bool
	autoUpdate     bool
	enabled        bool
	currentModel   string
}

// Config holds configuration for the reasoning layer
type Config struct {
	Provider        provider.Provider
	BaseURL         string
	ModelPreference []string
	FallbackModel   string
	DataDir         string
	AutoCreate      bool
	AutoUpdate      bool
	Enabled         bool
}

// New creates a new reasoning layer
func New(ctx context.Context, cfg Config) (*Layer, error) {
	if !cfg.Enabled {
		return &Layer{enabled: false}, nil
	}

	detector := NewModelDetector(cfg.BaseURL, cfg.ModelPreference, cfg.FallbackModel)

	// Auto-detect best model
	model, err := detector.DetectBestModel(ctx)
	if err != nil {
		slog.Warn("Failed to detect model, using fallback", "error", err, "fallback", cfg.FallbackModel)
		model = cfg.FallbackModel
	}

	analyzer := NewAnalyzer(cfg.Provider, model)

	layer := &Layer{
		detector:     detector,
		analyzer:     analyzer,
		stores:       make(map[string]*TaskStore),
		dataDir:      cfg.DataDir,
		autoCreate:   cfg.AutoCreate,
		autoUpdate:   cfg.AutoUpdate,
		enabled:      true,
		currentModel: model,
	}

	slog.Info("Reasoning layer initialized",
		"model", model,
		"auto_create", cfg.AutoCreate,
		"auto_update", cfg.AutoUpdate)

	return layer, nil
}

// AnalyzeUserRequest processes a user message and creates tasks if auto-create is enabled
func (l *Layer) AnalyzeUserRequest(ctx context.Context, sessionID string, msg message.Message) error {
	if !l.enabled || !l.autoCreate {
		return nil
	}

	// Get or create task store for this session
	store, err := l.getOrCreateStore(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get task store: %w", err)
	}

	// Extract text content from message
	var textContent string
	for _, part := range msg.Parts {
		if text, ok := part.(message.TextContent); ok {
			textContent = text.Text
			break
		}
	}

	if textContent == "" {
		return nil // No text to analyze
	}

	// Extract tasks using analyzer
	existingTasks := store.GetAllTasks()
	extractedTasks, err := l.analyzer.ExtractTasks(ctx, textContent, existingTasks)
	if err != nil {
		slog.Warn("Failed to extract tasks", "error", err)
		return nil // Don't fail the whole request
	}

	// Add extracted tasks to store
	for _, task := range extractedTasks {
		if _, err := store.AddTask(task.Description, task.Priority, task.Tags); err != nil {
			slog.Warn("Failed to add task", "error", err, "task", task.Description)
		} else {
			slog.Info("Created task", "description", task.Description, "priority", task.Priority)
		}
	}

	return nil
}

// UpdateTasksFromResponse processes an LLM response and updates tasks if auto-update is enabled
func (l *Layer) UpdateTasksFromResponse(ctx context.Context, sessionID string, msg message.Message) error {
	if !l.enabled || !l.autoUpdate {
		return nil
	}

	// Get task store for this session
	store, err := l.getStore(sessionID)
	if err != nil {
		// No store means no tasks to update
		return nil
	}

	// Extract text content from message
	var textContent string
	for _, part := range msg.Parts {
		if text, ok := part.(message.TextContent); ok {
			textContent += text.Text
		}
	}

	if textContent == "" {
		return nil // No text to analyze
	}

	// Analyze response using analyzer
	currentTasks := store.GetAllTasks()
	analysis, err := l.analyzer.AnalyzeResponse(ctx, textContent, currentTasks)
	if err != nil {
		slog.Warn("Failed to analyze response", "error", err)
		return nil // Don't fail the whole request
	}

	// Mark completed tasks
	for _, taskID := range analysis.Completed {
		if err := store.UpdateTaskStatus(taskID, "completed"); err != nil {
			slog.Warn("Failed to complete task", "error", err, "task_id", taskID)
		} else {
			slog.Info("Completed task", "task_id", taskID)
		}
	}

	// Create new tasks
	for _, newTask := range analysis.Created {
		if _, err := store.AddTask(newTask.Description, newTask.Priority, newTask.Tags); err != nil {
			slog.Warn("Failed to create task", "error", err, "description", newTask.Description)
		} else {
			slog.Info("Created follow-up task", "description", newTask.Description)
		}
	}

	// Update task statuses
	for _, update := range analysis.Updated {
		if err := store.UpdateTaskStatus(update.ID, update.Status); err != nil {
			slog.Warn("Failed to update task status", "error", err, "task_id", update.ID)
		} else {
			slog.Info("Updated task status", "task_id", update.ID, "status", update.Status)
		}
	}

	return nil
}

// GetTasks returns all tasks for a session
func (l *Layer) GetTasks(sessionID string) ([]Task, error) {
	if !l.enabled {
		return nil, nil
	}

	store, err := l.getStore(sessionID)
	if err != nil {
		return nil, err
	}

	return store.GetAllTasks(), nil
}

// getOrCreateStore gets or creates a task store for a session
func (l *Layer) getOrCreateStore(sessionID string) (*TaskStore, error) {
	l.storesMu.RLock()
	store, exists := l.stores[sessionID]
	l.storesMu.RUnlock()

	if exists {
		return store, nil
	}

	// Create new store
	l.storesMu.Lock()
	defer l.storesMu.Unlock()

	// Check again in case another goroutine created it
	if store, exists := l.stores[sessionID]; exists {
		return store, nil
	}

	newStore, err := NewTaskStore(l.dataDir, sessionID)
	if err != nil {
		return nil, err
	}

	newStore.Model = l.currentModel
	l.stores[sessionID] = newStore

	return newStore, nil
}

// getStore gets an existing task store for a session
func (l *Layer) getStore(sessionID string) (*TaskStore, error) {
	l.storesMu.RLock()
	defer l.storesMu.RUnlock()

	store, exists := l.stores[sessionID]
	if !exists {
		return nil, fmt.Errorf("no task store for session %s", sessionID)
	}

	return store, nil
}

// IsEnabled returns whether the reasoning layer is enabled
func (l *Layer) IsEnabled() bool {
	return l.enabled
}

// GetModel returns the current model being used for reasoning
func (l *Layer) GetModel() string {
	return l.currentModel
}
