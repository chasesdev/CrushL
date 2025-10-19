package tasks

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/llm/reasoning"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	MaxVisibleTasks = 5
	MinHeight       = 0
	MaxHeight       = 6 // 1 line header + 5 tasks
)

// TasksCmp represents a component that displays tasks above the chat input
type TasksCmp interface {
	Init() tea.Cmd
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string
	SetSize(width, height int) tea.Cmd
	SetTasks(tasks []reasoning.Task) tea.Cmd
	GetHeight() int
}

// tasksCmp implements TasksCmp
type tasksCmp struct {
	width        int
	height       int
	tasks        []reasoning.Task
	activeTasks  []reasoning.Task // only pending and in_progress tasks
	spinner      spinner.Model
	expandedTask int  // index of expanded task, -1 if none
	visible      bool // whether tasks are visible (can be toggled)
}

// New creates a new tasks component
func New() TasksCmp {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return &tasksCmp{
		tasks:        []reasoning.Task{},
		activeTasks:  []reasoning.Task{},
		spinner:      s,
		expandedTask: -1,
		visible:      true, // visible by default
	}
}

// Init implements TasksCmp
func (t *tasksCmp) Init() tea.Cmd {
	return t.spinner.Tick
}

// SetTasksMsg is a message to update the tasks
type SetTasksMsg struct {
	Tasks []reasoning.Task
}

// ToggleVisibilityMsg is a message to toggle task visibility
type ToggleVisibilityMsg struct{}

// Update implements TasksCmp
func (t *tasksCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SetTasksMsg:
		return t, t.SetTasks(msg.Tasks)
	case ToggleVisibilityMsg:
		t.visible = !t.visible
		return t, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		t.spinner, cmd = t.spinner.Update(msg)
		return t, cmd
	case tea.MouseClickMsg:
		// Handle clicks on task items
		if msg.Button == tea.MouseLeft && len(t.activeTasks) > 0 && t.visible {
			// Calculate which task was clicked (accounting for header + border)
			// Line 0 = top border, Line 1 = header, Line 2+ = tasks
			clickedLine := msg.Y - 2
			if clickedLine >= 0 && clickedLine < len(t.activeTasks) {
				// Toggle expansion
				if t.expandedTask == clickedLine {
					t.expandedTask = -1 // collapse
				} else {
					t.expandedTask = clickedLine // expand
				}
			}
		}
	}
	return t, nil
}

// View implements TasksCmp
func (t *tasksCmp) View() string {
	// Don't show if toggled off or no active tasks
	if !t.visible || len(t.activeTasks) == 0 {
		return ""
	}

	theme := styles.CurrentTheme()

	// Build header
	taskCount := len(t.activeTasks)
	header := fmt.Sprintf("Tasks (%d active)", taskCount)

	// Build task lines
	lines := []string{}
	visibleCount := min(len(t.activeTasks), MaxVisibleTasks)

	for i := 0; i < visibleCount; i++ {
		task := t.activeTasks[i]
		line := t.renderTask(task, theme, i == t.expandedTask)
		lines = append(lines, line)

		// Add expanded details if this task is expanded
		if i == t.expandedTask {
			details := t.renderTaskDetails(task, theme)
			lines = append(lines, details)
		}
	}

	// Show indicator if there are more tasks
	if len(t.activeTasks) > MaxVisibleTasks {
		more := fmt.Sprintf("  ... and %d more", len(t.activeTasks)-MaxVisibleTasks)
		lines = append(lines, theme.S().Subtle.Render(more))
	}

	content := strings.Join(lines, "\n")

	// Style the container
	containerStyle := theme.S().Base.
		Width(t.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1)

	headerStyle := theme.S().Subtle.
		Bold(true)

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(header),
		content,
	)

	return containerStyle.Render(view)
}

// renderTask renders a single task line with icon, description, and priority
func (t *tasksCmp) renderTask(task reasoning.Task, theme *styles.Theme, isExpanded bool) string {
	// Use spinner for in-progress tasks, otherwise use static icon
	var icon string
	if task.Status == "in_progress" {
		icon = t.spinner.View()
	} else {
		icon = getStatusIcon(task.Status)
	}
	iconColor := getStatusColor(task.Status, theme)

	// Add expand/collapse indicator
	expandIndicator := "▸"
	if isExpanded {
		expandIndicator = "▾"
	}

	priorityBadge := ""
	if task.Priority != "" && task.Priority != "medium" {
		priorityBadge = fmt.Sprintf(" [%s]", task.Priority)
		priorityBadge = theme.S().Base.
			Foreground(getPriorityColor(task.Priority, theme)).
			Render(priorityBadge)
	}

	description := task.Description
	maxDescLen := t.width - 25 // Leave space for icon, priority, borders, expand indicator
	if maxDescLen > 0 && len(description) > maxDescLen {
		description = description[:maxDescLen-3] + "..."
	}

	// Apply strikethrough for completed tasks
	descStyle := theme.S().Base
	if task.Status == "completed" {
		descStyle = descStyle.Strikethrough(true).Foreground(theme.FgSubtle)
	}

	// For in-progress, use spinner directly; for others, color the icon
	iconStyled := theme.S().Base.Foreground(iconColor).Render(icon)
	expandStyled := theme.S().Subtle.Render(expandIndicator)
	descStyled := descStyle.Render(description)

	return fmt.Sprintf("%s %s %s%s", expandStyled, iconStyled, descStyled, priorityBadge)
}

// renderTaskDetails renders expanded task details
func (t *tasksCmp) renderTaskDetails(task reasoning.Task, theme *styles.Theme) string {
	details := []string{}

	// Show full description if it was truncated
	if len(task.Description) > 0 {
		details = append(details, theme.S().Subtle.Render("  Description: ")+task.Description)
	}

	// Show status
	details = append(details, theme.S().Subtle.Render("  Status: ")+task.Status)

	// Show priority
	if task.Priority != "" {
		details = append(details, theme.S().Subtle.Render("  Priority: ")+task.Priority)
	}

	// Show tags
	if len(task.Tags) > 0 {
		tagsStr := strings.Join(task.Tags, ", ")
		details = append(details, theme.S().Subtle.Render("  Tags: ")+tagsStr)
	}

	// Show timestamps
	details = append(details, theme.S().Subtle.Render("  Created: ")+task.CreatedAt.Format("15:04:05"))
	if task.CompletedAt != nil {
		details = append(details, theme.S().Subtle.Render("  Completed: ")+task.CompletedAt.Format("15:04:05"))
	}

	return strings.Join(details, "\n")
}

// getStatusIcon returns the icon for a task status
func getStatusIcon(status string) string {
	switch status {
	case "pending":
		return "☐"
	case "in_progress":
		return "→"
	case "completed":
		return "✓"
	case "blocked":
		return "⊗"
	default:
		return "·"
	}
}

// getStatusColor returns the color for a task status
func getStatusColor(status string, theme *styles.Theme) color.Color {
	switch status {
	case "pending":
		return theme.FgSubtle
	case "in_progress":
		return theme.Accent
	case "completed":
		return theme.Success
	case "blocked":
		return theme.Error
	default:
		return theme.FgBase
	}
}

// getPriorityColor returns the color for a priority level
func getPriorityColor(priority string, theme *styles.Theme) color.Color {
	switch priority {
	case "high":
		return theme.Error
	case "low":
		return theme.FgSubtle
	default:
		return theme.FgBase
	}
}

// SetSize implements TasksCmp
func (t *tasksCmp) SetSize(width, height int) tea.Cmd {
	t.width = width
	t.height = height
	return nil
}

// SetTasks updates the tasks and filters to show only active ones
func (t *tasksCmp) SetTasks(tasks []reasoning.Task) tea.Cmd {
	t.tasks = tasks

	// Filter to show only pending and in_progress tasks
	activeTasks := []reasoning.Task{}
	for _, task := range tasks {
		if task.Status == "pending" || task.Status == "in_progress" {
			activeTasks = append(activeTasks, task)
		}
	}

	t.activeTasks = activeTasks
	return nil
}

// GetHeight returns the height needed by the component
func (t *tasksCmp) GetHeight() int {
	// Return 0 height if not visible or no active tasks
	if !t.visible || len(t.activeTasks) == 0 {
		return MinHeight
	}

	// Calculate height: 1 header + tasks + borders
	visibleTasks := min(len(t.activeTasks), MaxVisibleTasks)
	height := 1 + visibleTasks + 2 // header + tasks + top/bottom borders

	// Add extra lines for expanded task details (approximately 5 lines per expanded task)
	if t.expandedTask >= 0 && t.expandedTask < visibleTasks {
		height += 5
	}

	// Add 1 if we're showing "... and X more"
	if len(t.activeTasks) > MaxVisibleTasks {
		height++
	}

	return height
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
