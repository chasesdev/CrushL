package reasoning

import (
	"regexp"
	"strings"
)

// FastParser provides regex-based task extraction for common patterns
type FastParser struct{}

// NewFastParser creates a new fast parser
func NewFastParser() *FastParser {
	return &FastParser{}
}

var (
	// Markdown task list patterns
	markdownTaskRegex = regexp.MustCompile(`(?m)^[\s]*[-*]\s*\[([ xX])\]\s*(.+)$`)

	// Numbered list patterns (1. Do this, 2. Do that)
	numberedListRegex = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+(.+)$`)

	// TODO/FIXME patterns
	todoRegex = regexp.MustCompile(`(?mi)^\s*(TODO|FIXME|HACK|NOTE|XXX):\s*(.+)$`)

	// Imperative sentences that likely indicate tasks
	// Common action verbs at start of sentence
	imperativeRegex = regexp.MustCompile(`(?mi)^[\s]*(?:please\s+)?(?:can you\s+)?(add|create|implement|fix|update|remove|delete|refactor|test|write|make|build|setup|configure|install|run|execute|check|verify|validate|ensure|generate|modify|change|improve|optimize|debug|review|document|explain|describe|analyze|research|investigate|explore|find|search|look|fetch|get|set|enable|disable|toggle|switch|move|copy|rename|migrate|upgrade|downgrade|restore|backup|deploy|release|publish|merge|rebase|commit|push|pull|clone|branch|tag|stash)\s+(.+)$`)

	// Explicit task markers
	taskMarkerRegex = regexp.MustCompile(`(?i)^[\s]*(?:task|step|action|item)[\s:]+(.+)$`)

	// Ordinal patterns (first X, second Y, third Z)
	ordinalRegex = regexp.MustCompile(`(?i)(first|second|third|fourth|fifth|1st|2nd|3rd|4th|5th|sixth|seventh|eighth|ninth|tenth)\s+(.+?)(?:\.|,|;|\s+(?:first|second|third|fourth|fifth|1st|2nd|3rd|4th|5th|sixth|seventh|eighth|ninth|tenth)|$)`)
)

// ParsedTask represents a task extracted by regex
type ParsedTask struct {
	Description string
	Priority    string
	Tags        []string
	IsCompleted bool
}

// ExtractTasks attempts to extract tasks using fast regex patterns
// Returns extracted tasks and a boolean indicating if LLM analysis is still needed
func (p *FastParser) ExtractTasks(text string) ([]ParsedTask, bool) {
	tasks := []ParsedTask{}

	// Try markdown task lists first
	markdownTasks := p.extractMarkdownTasks(text)
	tasks = append(tasks, markdownTasks...)

	// Try numbered lists
	numberedTasks := p.extractNumberedList(text)
	tasks = append(tasks, numberedTasks...)

	// Try TODO comments
	todoTasks := p.extractTODOs(text)
	tasks = append(tasks, todoTasks...)

	// Try task markers
	markerTasks := p.extractTaskMarkers(text)
	tasks = append(tasks, markerTasks...)

	// Try ordinal patterns (first X, second Y, third Z)
	ordinalTasks := p.extractOrdinals(text)
	tasks = append(tasks, ordinalTasks...)

	// Try imperative sentences (only if no other patterns found)
	if len(tasks) == 0 {
		imperativeTasks := p.extractImperatives(text)
		tasks = append(tasks, imperativeTasks...)
	}

	// Deduplicate tasks with similar descriptions
	tasks = p.deduplicateTasks(tasks)

	// If we found ANY tasks, we don't need LLM
	// Only use LLM if we found absolutely nothing
	needsLLM := len(tasks) == 0

	return tasks, needsLLM
}

// extractMarkdownTasks extracts tasks from markdown checkbox lists
func (p *FastParser) extractMarkdownTasks(text string) []ParsedTask {
	matches := markdownTaskRegex.FindAllStringSubmatch(text, -1)
	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			isCompleted := strings.ToLower(match[1]) == "x"
			description := strings.TrimSpace(match[2])

			// Determine priority from description markers
			priority := "medium"
			if strings.Contains(strings.ToLower(description), "urgent") ||
			   strings.Contains(strings.ToLower(description), "critical") ||
			   strings.HasPrefix(description, "!!!") {
				priority = "high"
			} else if strings.Contains(strings.ToLower(description), "optional") ||
			          strings.Contains(strings.ToLower(description), "nice to have") {
				priority = "low"
			}

			tasks = append(tasks, ParsedTask{
				Description: description,
				Priority:    priority,
				Tags:        []string{"markdown"},
				IsCompleted: isCompleted,
			})
		}
	}

	return tasks
}

// extractNumberedList extracts tasks from numbered lists
func (p *FastParser) extractNumberedList(text string) []ParsedTask {
	matches := numberedListRegex.FindAllStringSubmatch(text, -1)

	// Only consider it a task list if there are multiple items
	if len(matches) < 2 {
		return nil
	}

	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 2 {
			description := strings.TrimSpace(match[1])

			// Skip if it looks like regular prose (contains common non-task words)
			if !p.looksLikeTask(description) {
				continue
			}

			tasks = append(tasks, ParsedTask{
				Description: description,
				Priority:    "medium",
				Tags:        []string{"numbered-list"},
				IsCompleted: false,
			})
		}
	}

	return tasks
}

// extractTODOs extracts tasks from TODO/FIXME comments
func (p *FastParser) extractTODOs(text string) []ParsedTask {
	matches := todoRegex.FindAllStringSubmatch(text, -1)
	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			marker := strings.ToUpper(match[1])
			description := strings.TrimSpace(match[2])

			priority := "medium"
			tags := []string{strings.ToLower(marker)}

			// FIXME and HACK are higher priority
			if marker == "FIXME" || marker == "HACK" {
				priority = "high"
			}

			tasks = append(tasks, ParsedTask{
				Description: description,
				Priority:    priority,
				Tags:        tags,
				IsCompleted: false,
			})
		}
	}

	return tasks
}

// extractTaskMarkers extracts tasks with explicit "Task:" or "Step:" markers
func (p *FastParser) extractTaskMarkers(text string) []ParsedTask {
	matches := taskMarkerRegex.FindAllStringSubmatch(text, -1)
	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 2 {
			description := strings.TrimSpace(match[1])

			tasks = append(tasks, ParsedTask{
				Description: description,
				Priority:    "medium",
				Tags:        []string{"explicit-marker"},
				IsCompleted: false,
			})
		}
	}

	return tasks
}

// extractOrdinals extracts tasks from ordinal patterns (first X, second Y, third Z)
func (p *FastParser) extractOrdinals(text string) []ParsedTask {
	matches := ordinalRegex.FindAllStringSubmatch(text, -1)

	// Only consider it a task list if there are multiple ordinals
	if len(matches) < 2 {
		return nil
	}

	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			description := strings.TrimSpace(match[2])

			// Clean up trailing punctuation
			description = strings.TrimRight(description, ".,;")
			description = strings.TrimSpace(description)

			tasks = append(tasks, ParsedTask{
				Description: description,
				Priority:    "medium",
				Tags:        []string{"ordinal-list"},
				IsCompleted: false,
			})
		}
	}

	return tasks
}

// extractImperatives extracts potential tasks from imperative sentences
func (p *FastParser) extractImperatives(text string) []ParsedTask {
	matches := imperativeRegex.FindAllStringSubmatch(text, -1)
	tasks := make([]ParsedTask, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			verb := strings.ToLower(match[1])
			description := strings.TrimSpace(match[2])

			// Reconstruct full description with verb
			fullDesc := verb + " " + description

			// Determine priority based on verb
			priority := "medium"
			if verb == "fix" || verb == "debug" || verb == "verify" || verb == "check" {
				priority = "high"
			} else if verb == "improve" || verb == "optimize" || verb == "document" {
				priority = "low"
			}

			tasks = append(tasks, ParsedTask{
				Description: fullDesc,
				Priority:    priority,
				Tags:        []string{"imperative"},
				IsCompleted: false,
			})
		}
	}

	return tasks
}

// looksLikeTask checks if a sentence looks like a task vs regular prose
func (p *FastParser) looksLikeTask(text string) bool {
	lower := strings.ToLower(text)

	// If it starts with a common task verb, it's likely a task
	taskVerbs := []string{
		"add", "create", "implement", "fix", "update", "remove", "delete",
		"refactor", "test", "write", "make", "build", "setup", "configure",
		"install", "run", "check", "verify", "ensure", "generate", "modify",
	}

	for _, verb := range taskVerbs {
		if strings.HasPrefix(lower, verb+" ") {
			return true
		}
	}

	// If it contains task-like words
	if strings.Contains(lower, "should") ||
	   strings.Contains(lower, "must") ||
	   strings.Contains(lower, "need to") ||
	   strings.Contains(lower, "have to") {
		return true
	}

	// If it's too long, it's probably prose
	if len(text) > 150 {
		return false
	}

	return false
}

// deduplicateTasks removes duplicate tasks based on similarity
func (p *FastParser) deduplicateTasks(tasks []ParsedTask) []ParsedTask {
	if len(tasks) <= 1 {
		return tasks
	}

	unique := []ParsedTask{}
	seen := make(map[string]bool)

	for _, task := range tasks {
		// Normalize description for comparison
		normalized := strings.ToLower(strings.TrimSpace(task.Description))
		normalized = strings.Join(strings.Fields(normalized), " ") // collapse whitespace

		if !seen[normalized] {
			seen[normalized] = true
			unique = append(unique, task)
		}
	}

	return unique
}
