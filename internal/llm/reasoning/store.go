package reasoning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Task represents a single task in the reasoning layer
type Task struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // pending, in_progress, completed, blocked
	Priority    string    `json:"priority"` // high, medium, low
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Subtasks    []Task    `json:"subtasks,omitempty"`
}

// TaskStore holds all tasks for a session
type TaskStore struct {
	SessionID string    `json:"session_id"`
	Model     string    `json:"model,omitempty"`
	Tasks     []Task    `json:"tasks"`
	NextID    int       `json:"next_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	filePath string
	mu       sync.RWMutex
}

// NewTaskStore creates a new task store for a session
func NewTaskStore(dataDir, sessionID string) (*TaskStore, error) {
	sessionDir := filepath.Join(dataDir, "sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	filePath := filepath.Join(sessionDir, "tasks.json")
	store := &TaskStore{
		SessionID: sessionID,
		Tasks:     []Task{},
		NextID:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		filePath:  filePath,
	}

	// Try to load existing tasks
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load existing tasks: %w", err)
		}
	}

	return store, nil
}

// AddTask adds a new task to the store
func (s *TaskStore) AddTask(description, priority string, tags []string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := Task{
		ID:          s.NextID,
		Description: description,
		Status:      "pending",
		Priority:    priority,
		Tags:        tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Subtasks:    []Task{},
	}

	s.Tasks = append(s.Tasks, task)
	s.NextID++
	s.UpdatedAt = time.Now()

	if err := s.save(); err != nil {
		return nil, err
	}

	return &task, nil
}

// GetTask retrieves a task by ID
func (s *TaskStore) GetTask(id int) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i], nil
		}
	}

	return nil, fmt.Errorf("task %d not found", id)
}

// UpdateTaskStatus updates the status of a task
func (s *TaskStore) UpdateTaskStatus(id int, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks[i].Status = status
			s.Tasks[i].UpdatedAt = time.Now()

			if status == "completed" {
				now := time.Now()
				s.Tasks[i].CompletedAt = &now
			}

			s.UpdatedAt = time.Now()
			return s.save()
		}
	}

	return fmt.Errorf("task %d not found", id)
}

// GetAllTasks returns all tasks
func (s *TaskStore) GetAllTasks() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	tasks := make([]Task, len(s.Tasks))
	copy(tasks, s.Tasks)
	return tasks
}

// GetTasksByStatus returns tasks with a specific status
func (s *TaskStore) GetTasksByStatus(status string) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []Task
	for _, task := range s.Tasks {
		if task.Status == status {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// save writes the task store to disk
func (s *TaskStore) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// load reads the task store from disk
func (s *TaskStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read tasks file: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	s.filePath = s.filePath // Restore filePath after unmarshal
	return nil
}
