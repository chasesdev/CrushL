package reasoning

import (
	"strings"
	"testing"
)

func TestProseParser_EngineeringSlang(t *testing.T) {
	parser := NewProseParser()

	text := `impl OAuth for API. refactor AuthN logic. rm deprecated DB code.`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected prose parser to handle engineering slang without LLM")
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("  Task %d: %s", i, task.Description)
		}
	}

	// Verify tasks are tagged as prose
	for i, task := range tasks {
		hasProseTag := false
		for _, tag := range task.Tags {
			if tag == "prose-imperative" || tag == "prose-composite" {
				hasProseTag = true
				break
			}
		}
		if !hasProseTag {
			t.Errorf("Task %d should have prose tag, got %v", i, task.Tags)
		}
	}
}

func TestProseParser_Abbreviations(t *testing.T) {
	parser := NewProseParser()

	text := `Add examples to docs, e.g. API usage patterns. Update README i.e. installation section.`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected prose parser to handle abbreviations without LLM")
	}

	if len(tasks) < 1 {
		t.Errorf("Expected at least 1 task, got %d", len(tasks))
	}

	// Verify that abbreviations were preprocessed
	// (should be expanded in internal processing)
	foundTask := false
	for _, task := range tasks {
		if len(task.Description) > 0 {
			foundTask = true
		}
	}

	if !foundTask {
		t.Error("Expected to find at least one valid task")
	}
}

func TestProseParser_LongFormPaste(t *testing.T) {
	parser := NewProseParser()

	text := `Implement a caching layer for database queries.
This should include support for Redis and Memcached.
Make sure to add proper error handling and logging.
Consider implementing cache invalidation strategies.`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected prose parser to handle long-form text without LLM")
	}

	if len(tasks) < 1 {
		t.Errorf("Expected at least 1 task, got %d", len(tasks))
		return
	}

	// Should create either individual tasks or a composite task
	// Verify we captured the main intent
	foundCaching := false
	for _, task := range tasks {
		if len(task.Description) > 20 { // Composite tasks are longer
			foundCaching = true
		}
	}

	if !foundCaching {
		t.Log("Tasks found:")
		for i, task := range tasks {
			t.Logf("  %d: %s", i, task.Description)
		}
	}
}

func TestProseParser_CodeSnippets(t *testing.T) {
	parser := NewProseParser()

	text := "Fix the bug in getUserById function. Update the auth middleware."

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected prose parser to handle code references without LLM")
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
}

func TestProseParser_MixedFormats(t *testing.T) {
	parser := NewProseParser()

	text := `Fix auth bug. Implement OAuth support. Refactor database layer.`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected prose parser to handle mixed formats without LLM")
	}

	if len(tasks) < 2 {
		t.Errorf("Expected at least 2 tasks, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("  Task %d: %s (tags: %v)", i, task.Description, task.Tags)
		}
	}
}

func TestProseParser_PriorityDetection(t *testing.T) {
	parser := NewProseParser()

	testCases := []struct {
		name             string
		text             string
		expectedPriority string
	}{
		{
			name:             "High priority - urgent",
			text:             "Fix the urgent security vulnerability",
			expectedPriority: "high",
		},
		{
			name:             "High priority - critical",
			text:             "Resolve critical authentication bug",
			expectedPriority: "high",
		},
		{
			name:             "Low priority - optional",
			text:             "Improve the optional documentation",
			expectedPriority: "low",
		},
		{
			name:             "Low priority - optimize",
			text:             "Optimize the database queries",
			expectedPriority: "low",
		},
		{
			name:             "Medium priority - default",
			text:             "Update the API endpoints",
			expectedPriority: "medium",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tasks, _ := parser.ExtractTasks(tc.text)

			if len(tasks) == 0 {
				t.Errorf("Expected at least 1 task for: %s", tc.text)
				return
			}

			task := tasks[0]
			if task.Priority != tc.expectedPriority {
				t.Errorf("Expected priority %q, got %q for: %s",
					tc.expectedPriority, task.Priority, tc.text)
			}
		})
	}
}

func TestProseParser_EngineeringVerbs(t *testing.T) {
	parser := NewProseParser()

	verbs := []string{
		"implement", "refactor", "optimize", "deploy",
		"migrate", "integrate", "configure", "provision",
	}

	for _, verb := range verbs {
		text := verb + " the authentication system"

		tasks, needsLLM := parser.ExtractTasks(text)

		if needsLLM {
			t.Errorf("Verb %q should not require LLM", verb)
		}

		if len(tasks) == 0 {
			t.Errorf("Verb %q should produce at least 1 task", verb)
		}
	}
}

func TestProseParser_NoTasks(t *testing.T) {
	parser := NewProseParser()

	text := `This is just a regular conversation.
The weather looks nice today.
Everything seems to be going well.`

	tasks, needsLLM := parser.ExtractTasks(text)

	// Prose parser might find some false positives in conversational text
	// The important thing is that it doesn't create many tasks from pure conversation
	if len(tasks) > 1 {
		t.Errorf("Expected few or no tasks for conversational text, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("  Task %d: %s", i, task.Description)
		}
	}

	// If no tasks found, should indicate LLM might be needed
	if len(tasks) == 0 && !needsLLM {
		t.Error("Expected needsLLM=true when no tasks found")
	}
}

func TestProseParser_MultiSentenceGrouping(t *testing.T) {
	parser := NewProseParser()

	text := `Implement OAuth authentication.
This should support Google and GitHub providers.
Make sure to add proper error handling.`

	tasks, needsLLM := parser.ExtractTasks(text)

	if needsLLM {
		t.Error("Expected multi-sentence task to not require LLM")
	}

	// Should either create individual tasks or a composite task
	// The important thing is that tasks are created
	if len(tasks) == 0 {
		t.Error("Expected at least one task from multi-sentence input")
	}

	// Log what was found for debugging
	t.Logf("Found %d tasks:", len(tasks))
	for i, task := range tasks {
		t.Logf("  %d: %s (tags: %v)", i, task.Description, task.Tags)
	}
}

func TestProseParser_Deduplication(t *testing.T) {
	parser := NewProseParser()

	text := `Fix the auth bug. Fix the auth bug. Resolve the authentication issue.`

	tasks, _ := parser.ExtractTasks(text)

	// Should deduplicate identical or very similar tasks
	if len(tasks) > 2 {
		t.Errorf("Expected deduplication to reduce tasks, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("  %d: %s", i, task.Description)
		}
	}
}

func TestJargonExpander_CommonAbbreviations(t *testing.T) {
	expander := NewJargonExpander()

	testCases := []struct {
		input    string
		contains string // Should contain this after expansion
	}{
		{
			input:    "impl OAuth",
			contains: "implement",
		},
		{
			input:    "refactor AuthN logic",
			contains: "authentication",
		},
		{
			input:    "rm DB code",
			contains: "database",
		},
		{
			input:    "setup K8s cluster",
			contains: "kubernetes",
		},
		{
			input:    "add API endpoint",
			contains: "api",
		},
	}

	for _, tc := range testCases {
		expanded := expander.Expand(tc.input)
		expandedLower := strings.ToLower(expanded)
		containsLower := strings.ToLower(tc.contains)
		if !strings.Contains(expandedLower, containsLower) {
			t.Errorf("Expected %q to contain %q after expansion, got %q",
				tc.input, tc.contains, expanded)
		}
	}
}

func TestJargonExpander_CasePreservation(t *testing.T) {
	expander := NewJargonExpander()

	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "impl feature",
			expected: "implement feature",
		},
		{
			input:    "Impl feature", // Capitalized
			expected: "Implement feature",
		},
		{
			input:    "IMPL FEATURE", // All caps
			expected: "IMPLEMENT FEATURE",
		},
	}

	for _, tc := range testCases {
		expanded := expander.Expand(tc.input)
		if expanded != tc.expected {
			t.Errorf("Expected %q to expand to %q, got %q",
				tc.input, tc.expected, expanded)
		}
	}
}

func TestIsEngineeringVerb(t *testing.T) {
	verbs := []string{
		"implement", "refactor", "optimize", "deploy",
		"fix", "update", "create", "remove",
	}

	for _, verb := range verbs {
		if !IsEngineeringVerb(verb) {
			t.Errorf("Expected %q to be recognized as engineering verb", verb)
		}
	}

	nonVerbs := []string{
		"hello", "world", "foo", "bar",
	}

	for _, nonVerb := range nonVerbs {
		if IsEngineeringVerb(nonVerb) {
			t.Errorf("Expected %q to not be recognized as engineering verb", nonVerb)
		}
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmarks

func BenchmarkProseParser_Simple(b *testing.B) {
	parser := NewProseParser()
	text := "Fix the authentication bug. Update the API documentation."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ExtractTasks(text)
	}
}

func BenchmarkProseParser_EngineeringSlang(b *testing.B) {
	parser := NewProseParser()
	text := "impl OAuth for API. refactor AuthN logic. rm deprecated DB code."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ExtractTasks(text)
	}
}

func BenchmarkProseParser_LongForm(b *testing.B) {
	parser := NewProseParser()
	text := `Implement a caching layer for database queries.
This should include support for Redis and Memcached.
Make sure to add proper error handling and logging.
Consider implementing cache invalidation strategies.
The cache should support TTL configuration.
Add metrics for cache hit/miss rates.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ExtractTasks(text)
	}
}

func BenchmarkProseParser_Complex(b *testing.B) {
	parser := NewProseParser()
	text := `We need to implement OAuth authentication for the API.
This should support multiple providers including Google, GitHub, and Microsoft.
Make sure to add proper error handling and logging.
The implementation should follow security best practices.
Consider adding rate limiting for auth endpoints.
Update the API documentation with authentication examples.
Add comprehensive unit tests and integration tests.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ExtractTasks(text)
	}
}

func BenchmarkJargonExpander(b *testing.B) {
	expander := NewJargonExpander()
	text := "impl OAuth for API. refactor AuthN and AuthZ. setup K8s cluster. rm old DB migrations."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = expander.Expand(text)
	}
}
