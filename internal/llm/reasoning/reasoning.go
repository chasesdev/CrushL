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
	detector         *ModelDetector
	analyzer         *Analyzer
	fastParser       *FastParser
	proseParser      *ProseParser
	stores           map[string]*TaskStore // sessionID -> TaskStore
	storesMu         sync.RWMutex
	pendingApprovals map[string]*PendingTaskApproval // sessionID -> pending tasks
	approvalsMu      sync.RWMutex
	dataDir          string
	autoCreate       bool
	autoUpdate       bool
	enabled          bool
	currentModel     string
	useFastParser    bool  // whether to use fast regex parser
	useProseParser   bool  // whether to use prose NLP parser
	requireApproval  bool  // whether to require user approval for tasks
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
	fastParser := NewFastParser()
	proseParser := NewProseParser()

	layer := &Layer{
		detector:         detector,
		analyzer:         analyzer,
		fastParser:       fastParser,
		proseParser:      proseParser,
		stores:           make(map[string]*TaskStore),
		pendingApprovals: make(map[string]*PendingTaskApproval),
		dataDir:          cfg.DataDir,
		autoCreate:       cfg.AutoCreate,
		autoUpdate:       cfg.AutoUpdate,
		enabled:          true,
		currentModel:     model,
		useFastParser:    true,  // enable fast parser by default
		useProseParser:   true,  // enable prose parser by default
		requireApproval:  true,  // require approval by default
	}

	slog.Info("Reasoning layer initialized",
		"model", model,
		"auto_create", cfg.AutoCreate,
		"auto_update", cfg.AutoUpdate)

	return layer, nil
}

// GetPendingApproval returns pending task approval for a session
func (l *Layer) GetPendingApproval(sessionID string) *PendingTaskApproval {
	l.approvalsMu.RLock()
	defer l.approvalsMu.RUnlock()
	return l.pendingApprovals[sessionID]
}

// ApproveTasks approves pending tasks and creates them
func (l *Layer) ApproveTasks(sessionID string, approved bool) error {
	l.approvalsMu.Lock()
	pending := l.pendingApprovals[sessionID]
	delete(l.pendingApprovals, sessionID)
	l.approvalsMu.Unlock()

	if pending == nil {
		return fmt.Errorf("no pending approval for session %s", sessionID)
	}

	if !approved {
		slog.Info("User rejected tasks", "session", sessionID, "count", len(pending.Tasks))
		return nil
	}

	// Get or create task store
	store, err := l.getOrCreateStore(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get task store: %w", err)
	}

	// Create approved tasks
	created := 0
	for _, task := range pending.Tasks {
		// Skip completed tasks
		if task.IsCompleted {
			continue
		}

		if _, err := store.AddTask(task.Description, task.Priority, task.Tags); err != nil {
			slog.Warn("Failed to add approved task", "error", err, "task", task.Description)
		} else {
			created++
			slog.Info("Created approved task", "description", task.Description, "priority", task.Priority)
		}
	}

	slog.Info("User approved tasks", "session", sessionID, "created", created)
	return nil
}

// AnalyzeUserRequest processes a user message and creates tasks if auto-create is enabled
func (l *Layer) AnalyzeUserRequest(ctx context.Context, sessionID string, msg message.Message) (*PendingTaskApproval, error) {
	if !l.enabled || !l.autoCreate {
		return nil, nil
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
		return nil, nil // No text to analyze
	}

	var parsedTasks []ParsedTask
	var source string

	// 3-Tier Parsing Strategy:
	// Tier 1: FastParser (regex) - ~5-13μs - structured patterns
	// Tier 2: ProseParser (NLP) - ~5-50ms - natural language
	// Tier 3: LLM Analyzer - ~500-2000ms - complex semantics

	// Tier 1: Try fast parser first if enabled
	if l.useFastParser {
		var needsMore bool
		parsedTasks, needsMore = l.fastParser.ExtractTasks(textContent)

		slog.Info("Fast parser extracted tasks", "count", len(parsedTasks), "needs_more", needsMore)

		// If fast parser found tasks, use those immediately
		if len(parsedTasks) > 0 {
			slog.Info("Using fast parser results, skipping NLP/LLM analysis")
			source = "fast-parser"
		} else if needsMore {
			// Tier 2: Try prose parser for natural language
			if l.useProseParser {
				var needsLLM bool
				parsedTasks, needsLLM = l.proseParser.ExtractTasks(textContent)

				slog.Info("Prose parser extracted tasks", "count", len(parsedTasks), "needs_llm", needsLLM)

				// If prose parser found tasks, use those
				if len(parsedTasks) > 0 {
					slog.Info("Using prose parser results, skipping LLM analysis")
					source = "prose-nlp"
				} else if needsLLM {
					// Tier 3: Fall back to LLM analyzer for complex semantics
					slog.Info("Prose parser needs LLM assistance, using analyzer")

					// Get or create store to pass existing tasks
					store, err := l.getOrCreateStore(sessionID)
					if err != nil {
						return nil, fmt.Errorf("failed to get task store: %w", err)
					}
					existingTasks := store.GetAllTasks()

					llmTasks, err := l.analyzer.ExtractTasks(ctx, textContent, existingTasks)
					if err != nil {
						slog.Warn("Failed to extract tasks via LLM", "error", err)
						return nil, nil // No tasks found
					}

					// Convert LLM tasks to ParsedTask format
					parsedTasks = []ParsedTask{}
					for _, lt := range llmTasks {
						parsedTasks = append(parsedTasks, ParsedTask{
							Description: lt.Description,
							Priority:    lt.Priority,
							Tags:        lt.Tags,
							IsCompleted: false,
						})
					}
					source = "llm-analyzer"
				}
			} else {
				// Prose disabled, go straight to LLM
				store, err := l.getOrCreateStore(sessionID)
				if err != nil {
					return nil, fmt.Errorf("failed to get task store: %w", err)
				}
				existingTasks := store.GetAllTasks()

				llmTasks, err := l.analyzer.ExtractTasks(ctx, textContent, existingTasks)
				if err != nil {
					slog.Warn("Failed to extract tasks via LLM", "error", err)
					return nil, nil
				}

				for _, lt := range llmTasks {
					parsedTasks = append(parsedTasks, ParsedTask{
						Description: lt.Description,
						Priority:    lt.Priority,
						Tags:        lt.Tags,
						IsCompleted: false,
					})
				}
				source = "llm-analyzer"
			}
		}
	} else {
		// All parsers disabled, use LLM only
		store, err := l.getOrCreateStore(sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task store: %w", err)
		}
		existingTasks := store.GetAllTasks()

		llmTasks, err := l.analyzer.ExtractTasks(ctx, textContent, existingTasks)
		if err != nil {
			slog.Warn("Failed to extract tasks", "error", err)
			return nil, nil
		}

		for _, lt := range llmTasks {
			parsedTasks = append(parsedTasks, ParsedTask{
				Description: lt.Description,
				Priority:    lt.Priority,
				Tags:        lt.Tags,
				IsCompleted: false,
			})
		}
		source = "llm-analyzer"
	}

	// If no tasks found, return nil
	if len(parsedTasks) == 0 {
		return nil, nil
	}

	// If approval is required, store pending tasks
	if l.requireApproval {
		approval := &PendingTaskApproval{
			SessionID: sessionID,
			Tasks:     parsedTasks,
			Source:    source,
		}

		l.approvalsMu.Lock()
		l.pendingApprovals[sessionID] = approval
		l.approvalsMu.Unlock()

		slog.Info("Tasks pending approval", "session", sessionID, "count", len(parsedTasks))
		return approval, nil
	}

	// Auto-approve if approval not required
	store, err := l.getOrCreateStore(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task store: %w", err)
	}

	for _, task := range parsedTasks {
		if task.IsCompleted {
			continue
		}
		if _, err := store.AddTask(task.Description, task.Priority, task.Tags); err != nil {
			slog.Warn("Failed to add task", "error", err, "task", task.Description)
		} else {
			slog.Info("Created task", "description", task.Description, "priority", task.Priority)
		}
	}

	return nil, nil
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

	// Use getOrCreateStore to load from disk if not in memory
	store, err := l.getOrCreateStore(sessionID)
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
