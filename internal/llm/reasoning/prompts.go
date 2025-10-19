package reasoning

import "fmt"

// GetTaskExtractionPrompt returns a prompt for extracting tasks from user requests
func GetTaskExtractionPrompt(userMessage string, existingTasks []Task) string {
	existingTasksStr := formatTasksForPrompt(existingTasks)

	return fmt.Sprintf(`You are a task analysis assistant. Analyze the user's request and identify discrete, actionable tasks.

USER REQUEST:
%s

EXISTING TASKS:
%s

INSTRUCTIONS:
1. Identify specific, actionable tasks from the user's request
2. Don't duplicate existing tasks
3. Break down complex requests into concrete steps
4. Assign appropriate priority (high/medium/low)
5. Keep task descriptions clear and concise

Respond ONLY with valid JSON in this exact format:
{
  "tasks": [
    {
      "description": "task description here",
      "priority": "high|medium|low",
      "tags": ["tag1", "tag2"]
    }
  ]
}

If no new tasks are needed, respond with: {"tasks": []}`, userMessage, existingTasksStr)
}

// GetResponseAnalysisPrompt returns a prompt for analyzing LLM responses
func GetResponseAnalysisPrompt(llmResponse string, currentTasks []Task) string {
	currentTasksStr := formatTasksForPrompt(currentTasks)

	return fmt.Sprintf(`You are a task tracker. Analyze what the assistant accomplished and update task statuses.

ASSISTANT'S RESPONSE:
%s

CURRENT TASKS:
%s

INSTRUCTIONS:
1. Identify which tasks were completed based on the response
2. Identify new tasks that emerged from the work
3. Identify tasks that need status updates (to in_progress or blocked)
4. Be conservative - only mark tasks complete if clearly finished

Respond ONLY with valid JSON in this exact format:
{
  "completed": [1, 3],
  "created": [
    {
      "description": "new task",
      "priority": "medium",
      "tags": []
    }
  ],
  "updated": [
    {
      "id": 2,
      "status": "in_progress"
    }
  ]
}

If no updates needed, respond with: {"completed": [], "created": [], "updated": []}`, llmResponse, currentTasksStr)
}

// formatTasksForPrompt formats tasks for display in prompts
func formatTasksForPrompt(tasks []Task) string {
	if len(tasks) == 0 {
		return "(no existing tasks)"
	}

	result := ""
	for _, task := range tasks {
		result += fmt.Sprintf("- [%d] %s (status: %s, priority: %s)\n",
			task.ID, task.Description, task.Status, task.Priority)
	}
	return result
}

// TaskExtractionResponse represents the expected JSON response from task extraction
type TaskExtractionResponse struct {
	Tasks []struct {
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		Tags        []string `json:"tags"`
	} `json:"tasks"`
}

// ResponseAnalysisResponse represents the expected JSON response from response analysis
type ResponseAnalysisResponse struct {
	Completed []int `json:"completed"`
	Created   []struct {
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		Tags        []string `json:"tags"`
	} `json:"created"`
	Updated []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	} `json:"updated"`
}
