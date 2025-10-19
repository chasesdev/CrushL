package reasoning

import (
	"testing"
)

func TestFastParser_MarkdownTasks(t *testing.T) {
	parser := NewFastParser()

	text := `
Here are the tasks for today:

- [ ] Fix authentication bug
- [x] Update documentation
- [ ] Add unit tests
- [ ] Deploy to staging
`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected fast parser to handle markdown tasks without LLM")
	}

	// Should extract all 4 tasks (including completed ones)
	if len(tasks) != 4 {
		t.Errorf("Expected 4 tasks (3 pending + 1 completed), got %d", len(tasks))
	}

	// Count uncompleted tasks
	uncompleted := 0
	for _, task := range tasks {
		if !task.IsCompleted {
			uncompleted++
		}
	}

	if uncompleted != 3 {
		t.Errorf("Expected 3 uncompleted tasks, got %d", uncompleted)
	}

	// Verify first task
	if tasks[0].Description != "Fix authentication bug" {
		t.Errorf("Expected 'Fix authentication bug', got '%s'", tasks[0].Description)
	}

	if tasks[0].IsCompleted {
		t.Error("Expected task to be uncompleted")
	}
}

func TestFastParser_NumberedList(t *testing.T) {
	parser := NewFastParser()

	text := `
Please complete these steps:

1. Create new database schema
2. Update API endpoints
3. Add validation logic
4. Write integration tests
`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected fast parser to handle numbered lists without LLM")
	}

	if len(tasks) < 4 {
		t.Errorf("Expected at least 4 tasks, got %d", len(tasks))
	}
}

func TestFastParser_TODOComments(t *testing.T) {
	parser := NewFastParser()

	text := `
TODO: Refactor error handling
FIXME: Memory leak in connection pool
HACK: Temporary workaround for API issue
`

	tasks, _ := parser.ExtractTasks(text)

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// FIXME should be high priority
	for _, task := range tasks {
		if task.Description == "Memory leak in connection pool" {
			if task.Priority != "high" {
				t.Errorf("Expected FIXME to have high priority, got %s", task.Priority)
			}
		}
	}
}

func TestFastParser_Imperatives(t *testing.T) {
	parser := NewFastParser()

	text := `
Add support for OAuth authentication
Fix the rendering bug in the sidebar
Update the README with new examples
`

	tasks, needsLLM := parser.ExtractTasks(text)

	// Imperatives should NOT trigger LLM - fast parser handles them instantly
	if needsLLM {
		t.Error("Expected imperatives to be handled without LLM")
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestFastParser_MixedContent(t *testing.T) {
	parser := NewFastParser()

	text := `
I need help with the following:

- [ ] Implement user registration
- [ ] Add email verification

Also, please fix the CSS bug on the homepage.
`

	tasks, needsLLM := parser.ExtractTasks(text)

	// Should find markdown tasks and not need LLM
	if needsLLM {
		t.Error("Expected markdown tasks to be sufficient")
	}

	if len(tasks) < 2 {
		t.Errorf("Expected at least 2 tasks, got %d", len(tasks))
	}
}

func TestFastParser_NoTasks(t *testing.T) {
	parser := NewFastParser()

	text := `
This is just a regular message explaining some concepts.
There are no actionable tasks here, just information.
`

	tasks, needsLLM := parser.ExtractTasks(text)

	// Should request LLM to verify no tasks
	if !needsLLM {
		t.Error("Expected to request LLM for ambiguous content")
	}

	// Fast parser might extract imperatives, but should be minimal
	if len(tasks) > 2 {
		t.Errorf("Expected few or no tasks, got %d", len(tasks))
	}
}

func TestFastParser_Ordinals(t *testing.T) {
	parser := NewFastParser()

	text := `3 tasks. first list files in directory. second tell me about this app, what programming
languages. third tell me about my system.`

	tasks, needsLLM := parser.ExtractTasks(text)

	// Should extract ordinals without LLM
	if needsLLM {
		t.Error("Expected ordinals to be handled without LLM")
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("  Task %d: %s", i, task.Description)
		}
	}

	// Verify all tasks are tagged as ordinal-list
	for i, task := range tasks {
		if len(task.Tags) == 0 || task.Tags[0] != "ordinal-list" {
			t.Errorf("Task %d should be tagged as ordinal-list, got %v", i, task.Tags)
		}
	}
}

func TestFastParser_Deduplication(t *testing.T) {
	parser := NewFastParser()

	text := `
- [ ] Fix bug in authentication
TODO: Fix bug in authentication
1. Fix bug in authentication
`

	tasks, _ := parser.ExtractTasks(text)

	// Should deduplicate similar tasks
	if len(tasks) > 1 {
		t.Errorf("Expected deduplication, got %d tasks", len(tasks))
	}
}

// Benchmark tests
func BenchmarkFastParser_Markdown(b *testing.B) {
	parser := NewFastParser()
	text := `
- [ ] Task 1
- [ ] Task 2
- [ ] Task 3
- [ ] Task 4
- [ ] Task 5
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ExtractTasks(text)
	}
}

func BenchmarkFastParser_Ordinals(b *testing.B) {
	parser := NewFastParser()
	text := `3 tasks. first list files in directory. second tell me about this app, what programming languages. third tell me about my system.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ExtractTasks(text)
	}
}

func BenchmarkFastParser_Mixed(b *testing.B) {
	parser := NewFastParser()
	text := `
Here's what we need to do:

1. Update the authentication system
2. Fix the database migration
3. Add new API endpoints

TODO: Review the security implications
FIXME: Performance issue in query

- [ ] Write documentation
- [ ] Add tests
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ExtractTasks(text)
	}
}
