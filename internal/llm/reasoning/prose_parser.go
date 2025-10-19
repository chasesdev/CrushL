package reasoning

import (
	"log/slog"
	"strings"

	"github.com/jdkato/prose/v2"
)

// ProseParser uses NLP to extract tasks from natural language text
type ProseParser struct {
	expander *JargonExpander
}

// NewProseParser creates a new prose-based task parser
func NewProseParser() *ProseParser {
	return &ProseParser{
		expander: NewJargonExpander(),
	}
}

// ExtractTasks extracts tasks using NLP analysis
// Returns extracted tasks and whether LLM is still needed
func (p *ProseParser) ExtractTasks(text string) ([]ParsedTask, bool) {
	// Preprocess text
	preprocessed := p.preprocess(text)

	// Create prose document
	doc, err := prose.NewDocument(preprocessed)
	if err != nil {
		slog.Warn("Failed to create prose document", "error", err)
		return nil, true // Needs LLM fallback
	}

	tasks := []ParsedTask{}

	// Extract tasks from imperative sentences
	imperativeTasks := p.extractImperatives(doc)
	tasks = append(tasks, imperativeTasks...)

	// Extract tasks from multi-sentence groups
	groupedTasks := p.extractMultiSentenceTasks(doc)
	tasks = append(tasks, groupedTasks...)

	// Deduplicate
	tasks = p.deduplicateTasks(tasks)

	// If we found tasks using NLP, we don't need LLM
	needsLLM := len(tasks) == 0

	return tasks, needsLLM
}

// preprocess prepares text for NLP analysis
func (p *ProseParser) preprocess(text string) string {
	// Expand engineering abbreviations and jargon
	expanded := p.expander.Expand(text)

	// Normalize whitespace
	normalized := normalizeWhitespace(expanded)

	// Handle common patterns that confuse sentence segmentation
	normalized = p.handleAbbreviations(normalized)

	return normalized
}

// handleAbbreviations preprocesses abbreviations that can confuse sentence segmentation
func (p *ProseParser) handleAbbreviations(text string) string {
	// Replace common abbreviations that end with periods
	replacements := map[string]string{
		"e.g.": "for example",
		"i.e.": "that is",
		"etc.": "and so on",
		"vs.":  "versus",
		"e.t.c.": "and so on",
	}

	result := text
	for abbrev, replacement := range replacements {
		result = strings.ReplaceAll(result, abbrev, replacement)
		result = strings.ReplaceAll(result, strings.ToUpper(abbrev), strings.ToUpper(replacement))
	}

	return result
}

// extractImperatives extracts tasks from imperative sentences using POS tagging
func (p *ProseParser) extractImperatives(doc *prose.Document) []ParsedTask {
	tasks := []ParsedTask{}
	sentences := doc.Sentences()

	for _, sent := range sentences {
		sentText := sent.Text

		// Skip very short sentences (likely fragments)
		if len(strings.Fields(sentText)) < 2 {
			continue
		}

		// Skip questions (conversational)
		if strings.HasSuffix(strings.TrimSpace(sentText), "?") {
			continue
		}

		// Get tokens for this sentence
		sentDoc, err := prose.NewDocument(sentText)
		if err != nil {
			continue
		}

		tokens := sentDoc.Tokens()
		if len(tokens) == 0 {
			continue
		}

		// Check if sentence starts with imperative verb
		// Look at first few tokens (skip common prefixes like "please", "can you")
		isImperative := false
		verbIndex := -1
		foundEngineeringVerb := false

		for i := 0; i < min(3, len(tokens)); i++ {
			tok := tokens[i]

			// Check if it's a known engineering verb FIRST
			// This gives us higher confidence it's a task
			if IsEngineeringVerb(tok.Text) {
				isImperative = true
				verbIndex = i
				foundEngineeringVerb = true
				break
			}

			// VB = verb base form (typical for imperatives)
			// VBP = verb non-3rd person singular present
			// Only accept these if we haven't found an engineering verb
			if tok.Tag == "VB" || tok.Tag == "VBP" {
				// Skip common conversational verbs that aren't task-related
				if !isConversationalVerb(tok.Text) {
					isImperative = true
					verbIndex = i
				}
			}
		}

		// Only create task if we found an engineering verb or strong imperative
		if isImperative && verbIndex >= 0 && (foundEngineeringVerb || len(tokens) > 3) {
			// Extract the task
			priority := p.determinePriority(sentText, tokens[verbIndex].Text)

			task := ParsedTask{
				Description: strings.TrimSpace(sentText),
				Priority:    priority,
				Tags:        []string{"prose-imperative"},
				IsCompleted: false,
			}
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// extractMultiSentenceTasks groups related sentences into composite tasks
func (p *ProseParser) extractMultiSentenceTasks(doc *prose.Document) []ParsedTask {
	tasks := []ParsedTask{}
	sentences := doc.Sentences()

	if len(sentences) < 2 {
		return tasks
	}

	// Look for patterns like:
	// "We need to implement X. This should include Y. Make sure to Z."
	// These should be grouped as a single task

	var currentGroup []string

	for _, sent := range sentences {
		sentText := sent.Text

		// Check if sentence indicates continuation
		// (starts with "This", "It", "That", connectors, etc.)
		isContinuation := p.isContinuationSentence(sentText)

		if isContinuation && len(currentGroup) > 0 {
			// Add to current group
			currentGroup = append(currentGroup, sentText)
		} else {
			// Check if previous group should become a task
			if len(currentGroup) >= 2 {
				// Create composite task from group
				compositeDesc := strings.Join(currentGroup, " ")
				task := ParsedTask{
					Description: compositeDesc,
					Priority:    "medium",
					Tags:        []string{"prose-composite"},
					IsCompleted: false,
				}
				tasks = append(tasks, task)
			}

			// Start new group
			currentGroup = []string{sentText}
		}
	}

	// Handle last group
	if len(currentGroup) >= 2 {
		compositeDesc := strings.Join(currentGroup, " ")
		task := ParsedTask{
			Description: compositeDesc,
			Priority:    "medium",
			Tags:        []string{"prose-composite"},
			IsCompleted: false,
		}
		tasks = append(tasks, task)
	}

	return tasks
}

// isContinuationSentence checks if a sentence continues a previous topic
func (p *ProseParser) isContinuationSentence(sent string) bool {
	sent = strings.TrimSpace(sent)
	lower := strings.ToLower(sent)

	// Check for continuation words at start
	continuations := []string{
		"this should", "this will", "this could", "this would",
		"it should", "it will", "it could", "it would",
		"that should", "that will", "that could", "that would",
		"also,", "additionally,", "furthermore,", "moreover,",
		"make sure", "ensure that", "be sure",
		"don't forget", "remember to",
	}

	for _, cont := range continuations {
		if strings.HasPrefix(lower, cont) {
			return true
		}
	}

	return false
}

// extractTopic extracts the main topic/subject from a sentence
func (p *ProseParser) extractTopic(sent string) string {
	// Simple heuristic: first noun phrase
	// This could be enhanced with more sophisticated NLP
	words := strings.Fields(sent)
	if len(words) > 0 {
		return words[0]
	}
	return ""
}

// determinePriority determines task priority based on text content
func (p *ProseParser) determinePriority(text string, verb string) string {
	lower := strings.ToLower(text)

	// High priority indicators
	highPriority := []string{
		"urgent", "critical", "asap", "immediately", "emergency",
		"blocking", "blocker", "broken", "down", "failing",
		"security", "vulnerability", "exploit", "breach",
	}

	for _, indicator := range highPriority {
		if strings.Contains(lower, indicator) {
			return "high"
		}
	}

	// High priority verbs
	if verb == "fix" || verb == "debug" || verb == "resolve" || verb == "repair" {
		return "high"
	}

	// Low priority indicators
	lowPriority := []string{
		"optional", "nice to have", "consider", "maybe",
		"eventually", "future", "someday", "when possible",
		"improve", "optimize", "enhance", "refine",
	}

	for _, indicator := range lowPriority {
		if strings.Contains(lower, indicator) {
			return "low"
		}
	}

	// Low priority verbs
	if verb == "improve" || verb == "optimize" || verb == "document" || verb == "refactor" {
		return "low"
	}

	return "medium"
}

// deduplicateTasks removes duplicate tasks
func (p *ProseParser) deduplicateTasks(tasks []ParsedTask) []ParsedTask {
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

// normalizeWhitespace normalizes whitespace in text
func normalizeWhitespace(text string) string {
	// Replace multiple spaces with single space
	text = strings.Join(strings.Fields(text), " ")

	// Normalize line breaks
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Replace multiple line breaks with double line break
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isConversationalVerb checks if a verb is likely conversational rather than task-oriented
func isConversationalVerb(verb string) bool {
	conversational := []string{
		"am", "is", "are", "was", "were", "be", "been",
		"have", "has", "had",
		"do", "does", "did",
		"will", "would", "should", "could", "might", "may",
		"know", "think", "believe", "feel", "want", "need",
		"say", "tell", "ask", "answer",
		"go", "come", "see", "look", "hear", "listen",
		"like", "love", "hate", "prefer",
	}

	lower := strings.ToLower(verb)
	for _, conv := range conversational {
		if lower == conv {
			return true
		}
	}
	return false
}
