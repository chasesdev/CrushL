package reasoning

import (
	"fmt"
	"strings"
)

// PendingTaskApproval represents tasks waiting for user approval
type PendingTaskApproval struct {
	SessionID string
	Tasks     []ParsedTask
	Source    string // "fast-parser" or "llm-analyzer"
}

// TaskApprovalDecision represents the user's decision on pending tasks
type TaskApprovalDecision struct {
	SessionID string
	Approved  bool
}

// FormatApprovalPrompt creates a user-friendly prompt for task approval
func FormatApprovalPrompt(tasks []ParsedTask, source string) string {
	if len(tasks) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n📋 I found %d task", len(tasks)))
	if len(tasks) > 1 {
		sb.WriteString("s")
	}
	sb.WriteString(" in your message:\n\n")

	for i, task := range tasks {
		icon := "☐"
		if task.IsCompleted {
			icon = "✓"
		}

		priority := ""
		if task.Priority == "high" {
			priority = " [high priority]"
		} else if task.Priority == "low" {
			priority = " [low priority]"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s%s\n", i+1, icon, task.Description, priority))
	}

	sb.WriteString("\n")

	if source == "fast-parser" {
		sb.WriteString("🚀 Auto-detected from structured format (markdown/numbered list)\n")
	} else if source == "prose-nlp" {
		sb.WriteString("🧠 Extracted using NLP analysis (natural language processing)\n")
	} else {
		sb.WriteString("🤖 Extracted using AI analysis\n")
	}

	sb.WriteString("\nWould you like me to track these tasks? (y/n): ")

	return sb.String()
}

// FormatTasksSummary creates a summary after tasks are created
func FormatTasksSummary(count int) string {
	if count == 0 {
		return "No tasks created."
	}

	return fmt.Sprintf("✅ Created %d task", count) +
		map[bool]string{true: "s", false: ""}[count > 1] +
		". Use Ctrl+T to view task list."
}
