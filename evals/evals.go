package evals

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
)

// EvalCheck is a simple function that evaluates an EvalEvent
type EvalCheck func(event aigentic.EvalEvent) (passed bool, score float64, message string)

// EvalResult contains the result of evaluating an EvalEvent
type EvalResult struct {
	EventID   string
	Passed    bool
	Score     float64
	Message   string
	CheckName string
}

// EvalSuite is a collection of checks for an agent
type EvalSuite struct {
	Name   string
	Checks map[string]EvalCheck
}

// NewEvalSuite creates a new evaluation suite
func NewEvalSuite(name string) *EvalSuite {
	return &EvalSuite{
		Name:   name,
		Checks: make(map[string]EvalCheck),
	}
}

// AddCheck adds a check to the suite
func (es *EvalSuite) AddCheck(name string, check EvalCheck) {
	es.Checks[name] = check
}

// Evaluate runs all checks against an EvalEvent
func (es *EvalSuite) Evaluate(event aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	for checkName, check := range es.Checks {
		passed, score, message := check(event)
		results = append(results, EvalResult{
			EventID:   event.EventID,
			Passed:    passed,
			Score:     score,
			Message:   message,
			CheckName: checkName,
		})
	}

	return results
}

// EvalProcessor processes multiple EvalEvents for a complete agent run
type EvalProcessor struct {
	Suite   *EvalSuite
	Events  []aigentic.EvalEvent
	Results []EvalResult
}

// NewEvalProcessor creates a new evaluation processor
func NewEvalProcessor(suite *EvalSuite) *EvalProcessor {
	return &EvalProcessor{
		Suite: suite,
	}
}

// ProcessEvent processes a single EvalEvent and stores results
func (ep *EvalProcessor) ProcessEvent(event aigentic.EvalEvent) {
	ep.Events = append(ep.Events, event)
	results := ep.Suite.Evaluate(event)
	ep.Results = append(ep.Results, results...)
}

// GetSummary returns a summary of all processed events
func (ep *EvalProcessor) GetSummary() EvalSummary {
	if len(ep.Results) == 0 {
		return EvalSummary{}
	}

	totalChecks := len(ep.Results)
	passed := 0
	totalScore := 0.0
	var totalDuration time.Duration

	for _, result := range ep.Results {
		if result.Passed {
			passed++
		}
		totalScore += result.Score
	}

	for _, event := range ep.Events {
		totalDuration += event.Duration
	}

	return EvalSummary{
		TotalCalls:    len(ep.Events),
		TotalChecks:   totalChecks,
		PassedChecks:  passed,
		PassRate:      float64(passed) / float64(totalChecks) * 100,
		AverageScore:  totalScore / float64(totalChecks),
		TotalDuration: totalDuration,
		Results:       ep.Results,
	}
}

// EvalSummary contains the summary of evaluation results
type EvalSummary struct {
	TotalCalls    int
	TotalChecks   int
	PassedChecks  int
	PassRate      float64
	AverageScore  float64
	TotalDuration time.Duration
	Results       []EvalResult
}

// CollectEvalEvents collects evaluation events from an agent run
func CollectEvalEvents(run *aigentic.AgentRun) []aigentic.EvalEvent {
	var events []aigentic.EvalEvent

	for event := range run.Next() {
		if evalEvent, ok := event.(*aigentic.EvalEvent); ok {
			events = append(events, *evalEvent)
		}
	}

	return events
}

// HasKeywords checks if the response contains expected keywords
func HasKeywords(keywords ...string) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		if len(keywords) == 0 {
			return true, 1.0, "no keywords to check"
		}

		content := strings.ToLower(event.Response.Content)
		matches := 0

		for _, keyword := range keywords {
			if strings.Contains(content, strings.ToLower(keyword)) {
				matches++
			}
		}

		score := float64(matches) / float64(len(keywords))
		passed := score >= 0.5 // At least half the keywords

		return passed, score, fmt.Sprintf("found %d/%d keywords", matches, len(keywords))
	}
}

// CallsTools checks if the response calls specific tools
func CallsTools(tools ...string) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		if len(tools) == 0 {
			return true, 1.0, "no tools to check"
		}

		matches := 0
		for _, expectedTool := range tools {
			for _, toolCall := range event.Response.ToolCalls {
				if toolCall.Name == expectedTool {
					matches++
					break
				}
			}
		}

		score := float64(matches) / float64(len(tools))
		passed := score >= 0.5

		return passed, score, fmt.Sprintf("called %d/%d expected tools", matches, len(tools))
	}
}

// LatencyUnder checks if the call completed within expected time
func LatencyUnder(maxDuration time.Duration) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		passed := event.Duration <= maxDuration
		score := 1.0
		if !passed {
			score = float64(maxDuration) / float64(event.Duration)
		}

		return passed, score, fmt.Sprintf("took %v (max %v)", event.Duration, maxDuration)
	}
}

// NoErrors checks if the call completed without errors
func NoErrors() EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		passed := event.Error == nil
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := "no errors"
		if event.Error != nil {
			message = fmt.Sprintf("error: %v", event.Error)
		}

		return passed, score, message
	}
}

// HasContent checks if the response has meaningful content
func HasContent(minLength int) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		contentLen := len(strings.TrimSpace(event.Response.Content))
		passed := contentLen >= minLength

		score := 1.0
		if contentLen < minLength {
			score = float64(contentLen) / float64(minLength)
		}

		return passed, score, fmt.Sprintf("content length: %d chars (min %d)", contentLen, minLength)
	}
}

// SequenceCheck checks if calls happen in expected order
func SequenceCheck(expectedSequence int) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		passed := event.Sequence == expectedSequence
		score := 1.0
		if !passed {
			score = 0.0
		}

		return passed, score, fmt.Sprintf("sequence %d (expected %d)", event.Sequence, expectedSequence)
	}
}

// ContentRelevance provides detailed relevance scoring based on content analysis
func ContentRelevance(expectedElements []string, weight float64) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		if len(expectedElements) == 0 {
			return true, 1.0, "no elements to check"
		}

		content := strings.ToLower(event.Response.Content)
		matches := 0
		var foundElements []string

		for _, element := range expectedElements {
			if strings.Contains(content, strings.ToLower(element)) {
				matches++
				foundElements = append(foundElements, element)
			}
		}

		score := float64(matches) / float64(len(expectedElements))
		weighted_score := score * weight
		passed := score >= 0.6 // 60% relevance threshold

		return passed, weighted_score, fmt.Sprintf("relevance %.2f (%d/%d elements: %v)",
			score, matches, len(expectedElements), foundElements)
	}
}

// BehavioralAccuracy checks if the agent follows expected behavioral patterns
func BehavioralAccuracy(expectedBehaviors []string, weight float64) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		if len(expectedBehaviors) == 0 {
			return true, 1.0, "no behaviors to check"
		}

		// Analyze both the messages (input) and response for behavioral patterns
		allText := ""
		for _, msg := range event.Messages {
			// Extract content from message (simplified approach)
			if content := extractMessageContent(msg); content != "" {
				allText += strings.ToLower(content) + " "
			}
		}
		allText += strings.ToLower(event.Response.Content)

		matches := 0
		var foundBehaviors []string

		for _, behavior := range expectedBehaviors {
			if strings.Contains(allText, strings.ToLower(behavior)) {
				matches++
				foundBehaviors = append(foundBehaviors, behavior)
			}
		}

		score := float64(matches) / float64(len(expectedBehaviors))
		weighted_score := score * weight
		passed := score >= 0.7 // 70% accuracy threshold

		return passed, weighted_score, fmt.Sprintf("accuracy %.2f (%d/%d behaviors: %v)",
			score, matches, len(expectedBehaviors), foundBehaviors)
	}
}

// ToolAccuracy checks if the right tools are called in the right context
func ToolAccuracy(expectedToolContext map[string][]string) EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		if len(expectedToolContext) == 0 {
			return true, 1.0, "no tool context to check"
		}

		var issues []string
		totalChecks := 0
		passedChecks := 0

		for _, toolCall := range event.Response.ToolCalls {
			if expectedKeywords, exists := expectedToolContext[toolCall.Name]; exists {
				totalChecks++

				// Check if the context (messages) contains expected keywords for this tool
				contextText := ""
				for _, msg := range event.Messages {
					if content := extractMessageContent(msg); content != "" {
						contextText += strings.ToLower(content) + " "
					}
				}

				hasContext := false
				for _, keyword := range expectedKeywords {
					if strings.Contains(contextText, strings.ToLower(keyword)) {
						hasContext = true
						break
					}
				}

				if hasContext {
					passedChecks++
				} else {
					issues = append(issues, fmt.Sprintf("%s called without proper context", toolCall.Name))
				}
			}
		}

		score := 1.0
		if totalChecks > 0 {
			score = float64(passedChecks) / float64(totalChecks)
		}

		passed := score >= 0.8 // 80% tool accuracy threshold

		message := fmt.Sprintf("tool accuracy %.2f (%d/%d)", score, passedChecks, totalChecks)
		if len(issues) > 0 {
			message += fmt.Sprintf(" issues: %v", issues)
		}

		return passed, score, message
	}
}

// PrintResults prints evaluation results in a readable format
func PrintResults(results []EvalResult) {
	fmt.Printf("\n=== Evaluation Results ===\n")

	totalChecks := len(results)
	passed := 0
	totalScore := 0.0

	for _, result := range results {
		status := "❌"
		if result.Passed {
			status = "✅"
			passed++
		}

		totalScore += result.Score

		fmt.Printf("%s %s: %.2f - %s\n",
			status, result.CheckName, result.Score, result.Message)
	}

	avgScore := totalScore / float64(totalChecks)
	passRate := float64(passed) / float64(totalChecks) * 100

	fmt.Printf("\nSummary: %.1f%% passed (%d/%d), avg score: %.2f\n",
		passRate, passed, totalChecks, avgScore)
}

// PrintSummary prints a summary of evaluation results
func PrintSummary(summary EvalSummary) {
	fmt.Printf("\n=== Evaluation Summary ===\n")
	fmt.Printf("Total LLM Calls: %d\n", summary.TotalCalls)
	fmt.Printf("Total Checks: %d\n", summary.TotalChecks)
	fmt.Printf("Pass Rate: %.1f%% (%d/%d)\n", summary.PassRate, summary.PassedChecks, summary.TotalChecks)
	fmt.Printf("Average Score: %.2f\n", summary.AverageScore)
	fmt.Printf("Total Duration: %v\n", summary.TotalDuration)

	if summary.TotalCalls > 0 {
		avgLatency := summary.TotalDuration / time.Duration(summary.TotalCalls)
		fmt.Printf("Average Latency: %v per call\n", avgLatency)
	}

	// Enhanced accuracy and relevance reporting
	accuracy, relevance := calculateAccuracyRelevance(summary.Results)
	fmt.Printf("\n=== Quality Metrics ===\n")
	fmt.Printf("📊 Accuracy Score: %.2f/1.00\n", accuracy)
	fmt.Printf("🎯 Relevance Score: %.2f/1.00\n", relevance)

	// Quality rating
	overallQuality := (accuracy + relevance) / 2.0
	var qualityRating string
	var emoji string
	switch {
	case overallQuality >= 0.9:
		qualityRating = "Excellent"
		emoji = "🌟"
	case overallQuality >= 0.8:
		qualityRating = "Good"
		emoji = "✅"
	case overallQuality >= 0.7:
		qualityRating = "Fair"
		emoji = "⚠️"
	default:
		qualityRating = "Needs Improvement"
		emoji = "❌"
	}
	fmt.Printf("%s Overall Quality: %s (%.2f)\n", emoji, qualityRating, overallQuality)
}

// calculateAccuracyRelevance calculates accuracy and relevance scores from results
func calculateAccuracyRelevance(results []EvalResult) (accuracy, relevance float64) {
	if len(results) == 0 {
		return 0.0, 0.0
	}

	var accuracySum, relevanceSum float64
	var accuracyCount, relevanceCount int

	for _, result := range results {
		// Classify checks as accuracy or relevance based on check name
		if isAccuracyCheck(result.CheckName) {
			accuracySum += result.Score
			accuracyCount++
		} else if isRelevanceCheck(result.CheckName) {
			relevanceSum += result.Score
			relevanceCount++
		}
	}

	if accuracyCount > 0 {
		accuracy = accuracySum / float64(accuracyCount)
	}
	if relevanceCount > 0 {
		relevance = relevanceSum / float64(relevanceCount)
	}

	return accuracy, relevance
}

// isAccuracyCheck determines if a check measures accuracy
func isAccuracyCheck(checkName string) bool {
	accuracyChecks := []string{
		"calls tools", "calls save memory", "calls", "tool",
		"no errors", "error", "sequence", "order",
		"has content", "content", "structure", "format",
	}

	checkLower := strings.ToLower(checkName)
	for _, pattern := range accuracyChecks {
		if strings.Contains(checkLower, pattern) {
			return true
		}
	}
	return false
}

// isRelevanceCheck determines if a check measures relevance
func isRelevanceCheck(checkName string) bool {
	relevanceChecks := []string{
		"keywords", "expert", "names", "table",
		"responds", "relevant", "appropriate", "correct",
	}

	checkLower := strings.ToLower(checkName)
	for _, pattern := range relevanceChecks {
		if strings.Contains(checkLower, pattern) {
			return true
		}
	}
	return false
}

// extractMessageContent extracts content from any message type (helper function)
func extractMessageContent(msg interface{}) string {
	// Use reflection or type assertion to extract content
	// This is a simplified version - in practice you'd handle all message types
	if msgWithContent, ok := msg.(interface{ GetContent() string }); ok {
		return msgWithContent.GetContent()
	}

	// Fallback: try to access Content field via interface{}
	if msgMap, ok := msg.(map[string]interface{}); ok {
		if content, exists := msgMap["content"]; exists {
			if contentStr, ok := content.(string); ok {
				return contentStr
			}
		}
	}

	return ""
}
