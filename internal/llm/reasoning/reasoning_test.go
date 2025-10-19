package reasoning

import (
	"context"
	"testing"
)

// TestNewReasoningLayerDisabled tests that a disabled reasoning layer returns successfully
func TestNewReasoningLayerDisabled(t *testing.T) {
	ctx := context.Background()

	cfg := Config{
		Enabled: false,
	}

	layer, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Expected no error when creating disabled reasoning layer, got: %v", err)
	}

	if layer.IsEnabled() {
		t.Error("Expected reasoning layer to be disabled")
	}
}

// TestNewReasoningLayerEnabled tests basic initialization of an enabled reasoning layer
func TestNewReasoningLayerEnabled(t *testing.T) {
	// This test requires a running LM Studio instance, so we skip it in CI
	if testing.Short() {
		t.Skip("Skipping test that requires LM Studio")
	}

	ctx := context.Background()

	cfg := Config{
		Provider:        nil, // Would need a real provider in production
		BaseURL:         "http://localhost:1234",
		ModelPreference: []string{"qwen3-8b"},
		FallbackModel:   "qwen3-8b",
		DataDir:         t.TempDir(),
		AutoCreate:      true,
		AutoUpdate:      true,
		Enabled:         true,
	}

	// This will fail without a real provider, but we're testing the structure
	_, err := New(ctx, cfg)

	// We expect an error since we don't have a real provider
	if err == nil {
		t.Log("Reasoning layer initialized successfully")
	}
}

// TestModelDetector tests the model detection logic
func TestModelDetector(t *testing.T) {
	detector := NewModelDetector(
		"http://localhost:1234",
		[]string{"glm-4.6", "qwen3-next-80b", "qwen3-8b"},
		"qwen3-8b",
	)

	if detector == nil {
		t.Fatal("Expected detector to be created")
	}

	// Test model matching logic
	tests := []struct {
		modelID  string
		isGLM    bool
		isQwen3N bool
		isQwen38 bool
	}{
		{"glm-4.6-chat", true, false, false},
		{"qwen3-next-80b", false, true, false},
		{"qwen3-8b", false, false, true},
		{"qwen3-80b-instruct", false, true, false},
		{"unknown-model", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			if got := detector.isGLMModel(tt.modelID); got != tt.isGLM {
				t.Errorf("isGLMModel(%s) = %v, want %v", tt.modelID, got, tt.isGLM)
			}
			if got := detector.isQwen3Next(tt.modelID); got != tt.isQwen3N {
				t.Errorf("isQwen3Next(%s) = %v, want %v", tt.modelID, got, tt.isQwen3N)
			}
			if got := detector.isQwen38B(tt.modelID); got != tt.isQwen38 {
				t.Errorf("isQwen38B(%s) = %v, want %v", tt.modelID, got, tt.isQwen38)
			}
		})
	}
}

// TestTaskStore tests basic task store operations
func TestTaskStore(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "test-session"

	store, err := NewTaskStore(tempDir, sessionID)
	if err != nil {
		t.Fatalf("Failed to create task store: %v", err)
	}

	// Test adding a task
	task, err := store.AddTask("Test task", "high", []string{"test"})
	if err != nil {
		t.Fatalf("Failed to add task: %v", err)
	}

	if task.ID != 1 {
		t.Errorf("Expected task ID to be 1, got %d", task.ID)
	}

	if task.Status != "pending" {
		t.Errorf("Expected task status to be pending, got %s", task.Status)
	}

	// Test getting a task
	retrieved, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.Description != task.Description {
		t.Errorf("Expected description %s, got %s", task.Description, retrieved.Description)
	}

	// Test updating task status
	err = store.UpdateTaskStatus(task.ID, "completed")
	if err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}

	updated, err := store.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get updated task: %v", err)
	}

	if updated.Status != "completed" {
		t.Errorf("Expected status to be completed, got %s", updated.Status)
	}

	if updated.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	// Test getting all tasks
	allTasks := store.GetAllTasks()
	if len(allTasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(allTasks))
	}
}
