package evals

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// EvalCheck is a simple function that evaluates an EvalEvent
type EvalCheck func(event aigentic.EvalEvent) (passed bool, score float64, message string)

// ToolCheck is a function that evaluates tool parameters
type ToolCheck func(toolName string, toolArgs string) (passed bool, score float64, message string)

// EvalResult contains the result of evaluating an EvalEvent
type EvalResult struct {
	EventID    string
	EventIndex int // Index of the event in the sequence
	Passed     bool
	Score      float64
	Message    string
	CheckName  string
}

// EvalSuite is a collection of checks for an agent
type EvalSuite struct {
	Name            string
	ToolChecks      map[string]ToolCheck // Tool-specific checks for tool parameters
	FinalChecks     map[string]EvalCheck // Final result checks
	RequiredTools   map[string]int       // Tools that MUST be called (count: -1 means 1 or more)
	UniversalChecks map[string]EvalCheck // Always applied (legacy support)
}

// NewEvalSuite creates a new evaluation suite
func NewEvalSuite(name string) *EvalSuite {
	return &EvalSuite{
		Name:            name,
		ToolChecks:      make(map[string]ToolCheck),
		FinalChecks:     make(map[string]EvalCheck),
		RequiredTools:   make(map[string]int),
		UniversalChecks: make(map[string]EvalCheck),
	}
}

// NewProcessor creates a new evaluation processor for this suite
func (es *EvalSuite) NewProcessor() *EvalProcessor {
	return &EvalProcessor{
		Suite:     es,
		AgentRuns: make(map[string]*AgentRunData),
	}
}

// AddToolCheck adds a tool-specific check for tool parameter validation
func (es *EvalSuite) AddToolCheck(toolName string, check ToolCheck) {
	if check != nil {
		es.ToolChecks[toolName] = check
	}
}

// AddFinalToolCheck adds a requirement for a tool to be called a specific number of times
func (es *EvalSuite) AddFinalToolCheck(toolName string, requiredCount int) {
	if requiredCount != 0 {
		es.RequiredTools[toolName] = requiredCount
	}
}

// AddFinalCheck adds a final result check
func (es *EvalSuite) AddFinalCheck(name string, check EvalCheck) {
	es.FinalChecks[name] = check
}

// AddCheck adds a universal check (applies to all responses - legacy support)
func (es *EvalSuite) AddCheck(name string, check EvalCheck) {
	es.UniversalChecks[name] = check
}

// Evaluate runs all checks against an EvalEvent
func (es *EvalSuite) Evaluate(event aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	// Apply universal checks (always)
	for checkName, check := range es.UniversalChecks {
		passed, score, message := check(event)
		results = append(results, EvalResult{
			EventID:    event.RunID,
			EventIndex: 0, // Single event evaluation
			Passed:     passed,
			Score:      score,
			Message:    message,
			CheckName:  checkName,
		})
	}

	// Check if this is a tool response or final response
	isToolResponse := len(event.Response.ToolCalls) > 0

	if isToolResponse {
		// Apply only the specific tool checks that match the tools being called
		results = append(results, es.evaluateToolResponse(event)...)
	} else {
		// Apply final result checks
		for checkName, check := range es.FinalChecks {
			passed, score, message := check(event)
			results = append(results, EvalResult{
				EventID:   event.RunID,
				Passed:    passed,
				Score:     score,
				Message:   message,
				CheckName: checkName,
			})
		}
	}

	return results
}

// evaluateToolResponse evaluates tool-specific checks for a tool response
func (es *EvalSuite) evaluateToolResponse(event aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	for _, toolCall := range event.Response.ToolCalls {
		// Apply tool-specific quality checks
		if check, exists := es.ToolChecks[toolCall.Name]; exists {
			passed, score, message := check(toolCall.Name, toolCall.Args)
			results = append(results, EvalResult{
				EventID:    event.RunID,
				EventIndex: 0, // Single event evaluation
				Passed:     passed,
				Score:      score,
				Message:    message,
				CheckName:  fmt.Sprintf("tool_%s_quality", toolCall.Name),
			})
		}
	}

	return results
}

// EvaluateWithHistory runs evaluation with run history for efficiency metrics
func (es *EvalSuite) EvaluateWithHistory(event aigentic.EvalEvent, runHistory []aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	// Apply universal checks (always)
	for checkName, check := range es.UniversalChecks {
		passed, score, message := check(event)
		results = append(results, EvalResult{
			EventID:    event.RunID,
			EventIndex: es.getEventIndex(event, runHistory),
			Passed:     passed,
			Score:      score,
			Message:    message,
			CheckName:  checkName,
		})
	}

	// Check if this is a tool response or final response
	isToolResponse := len(event.Response.ToolCalls) > 0

	if isToolResponse {
		// Apply only the specific tool checks that match the tools being called
		results = append(results, es.evaluateToolResponseWithHistory(event, runHistory)...)
	} else {
		// Apply final result checks and required tool validation
		results = append(results, es.evaluateFinalResultWithHistory(event, runHistory)...)
	}

	return results
}

// getEventIndex returns the index of an event in the run history
func (es *EvalSuite) getEventIndex(event aigentic.EvalEvent, runHistory []aigentic.EvalEvent) int {
	for i, histEvent := range runHistory {
		if histEvent.RunID == event.RunID && histEvent.Sequence == event.Sequence {
			return i
		}
	}
	return 0 // Default to 0 if not found
}

// evaluateToolResponseWithHistory evaluates tool responses with efficiency metrics
func (es *EvalSuite) evaluateToolResponseWithHistory(event aigentic.EvalEvent, runHistory []aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	for _, toolCall := range event.Response.ToolCalls {
		// Apply tool-specific quality checks (only if defined for this tool)
		if check, exists := es.ToolChecks[toolCall.Name]; exists {
			passed, score, message := check(toolCall.Name, toolCall.Args)
			results = append(results, EvalResult{
				EventID:    event.RunID,
				EventIndex: es.getEventIndex(event, runHistory),
				Passed:     passed,
				Score:      score,
				Message:    message,
				CheckName:  fmt.Sprintf("tool_%s_quality", toolCall.Name),
			})
		}

		// Check for duplicate tool calls (loop detection)
		duplicateScore := es.checkDuplicateToolCalls(toolCall, runHistory)
		results = append(results, EvalResult{
			EventID:    event.RunID,
			EventIndex: es.getEventIndex(event, runHistory),
			Passed:     duplicateScore > 0.5,
			Score:      duplicateScore,
			Message:    fmt.Sprintf("tool %s efficiency", toolCall.Name),
			CheckName:  fmt.Sprintf("tool_%s_efficiency", toolCall.Name),
		})
	}

	return results
}

// evaluateFinalResultWithHistory evaluates final results with required tool validation
func (es *EvalSuite) evaluateFinalResultWithHistory(event aigentic.EvalEvent, runHistory []aigentic.EvalEvent) []EvalResult {
	var results []EvalResult

	// Apply final result checks
	for checkName, check := range es.FinalChecks {
		passed, score, message := check(event)
		results = append(results, EvalResult{
			EventID:    event.RunID,
			EventIndex: 0, // Single event evaluation
			Passed:     passed,
			Score:      score,
			Message:    message,
			CheckName:  checkName,
		})
	}

	// Check if all required tools were called
	requiredToolScore := es.validateRequiredTools(runHistory)
	results = append(results, EvalResult{
		EventID:    event.RunID,
		EventIndex: es.getEventIndex(event, runHistory),
		Passed:     requiredToolScore > 0.8,
		Score:      requiredToolScore,
		Message:    "required tools validation",
		CheckName:  "required_tools_complete",
	})

	// Overall efficiency score (fewer tool calls = higher score)
	efficiencyScore := es.calculateEfficiencyScore(runHistory)
	results = append(results, EvalResult{
		EventID:    event.RunID,
		EventIndex: es.getEventIndex(event, runHistory),
		Passed:     efficiencyScore > 0.7,
		Score:      efficiencyScore,
		Message:    "tool usage efficiency",
		CheckName:  "overall_efficiency",
	})

	return results
}

// checkDuplicateToolCalls detects and scores duplicate tool calls
func (es *EvalSuite) checkDuplicateToolCalls(currentTool ai.ToolCall, runHistory []aigentic.EvalEvent) float64 {
	duplicateCount := 0
	totalCalls := 0

	for _, event := range runHistory {
		for _, toolCall := range event.Response.ToolCalls {
			if toolCall.Name == currentTool.Name {
				totalCalls++
				// Check if parameters are similar (simplified comparison)
				if areToolParamsSimilar(toolCall.Args, currentTool.Args) {
					duplicateCount++
				}
			}
		}
	}

	if totalCalls == 0 {
		return 1.0 // First call
	}

	// Score decreases with duplicates
	duplicateRatio := float64(duplicateCount) / float64(totalCalls)
	return 1.0 - duplicateRatio
}

// validateRequiredTools checks if all required tools were called the required number of times
func (es *EvalSuite) validateRequiredTools(runHistory []aigentic.EvalEvent) float64 {
	calledRequiredTools := make(map[string]int)

	// Track how many times each required tool was called
	for _, event := range runHistory {
		for _, toolCall := range event.Response.ToolCalls {
			if _, exists := es.RequiredTools[toolCall.Name]; exists {
				calledRequiredTools[toolCall.Name]++
			}
		}
	}

	// Calculate completion percentage
	totalRequired := 0
	totalCalled := 0
	for toolName, requiredCount := range es.RequiredTools {
		if requiredCount == -1 {
			// -1 means 1 or more calls
			if calledRequiredTools[toolName] >= 1 {
				totalCalled++
			}
			totalRequired++
		} else {
			// Specific count required
			totalRequired += requiredCount
			totalCalled += calledRequiredTools[toolName]
		}
	}

	if totalRequired == 0 {
		return 1.0
	}

	// Cap totalCalled at totalRequired to avoid over 100%
	if totalCalled > totalRequired {
		totalCalled = totalRequired
	}

	return float64(totalCalled) / float64(totalRequired)
}

// calculateEfficiencyScore calculates overall tool usage efficiency
func (es *EvalSuite) calculateEfficiencyScore(runHistory []aigentic.EvalEvent) float64 {
	totalToolCalls := 0
	uniqueToolCalls := make(map[string]bool)

	for _, event := range runHistory {
		for _, toolCall := range event.Response.ToolCalls {
			totalToolCalls++
			uniqueToolCalls[toolCall.Name] = true
		}
	}

	if totalToolCalls == 0 {
		return 1.0
	}

	// Efficiency = unique tools / total calls (higher is better)
	uniqueCount := len(uniqueToolCalls)
	efficiency := float64(uniqueCount) / float64(totalToolCalls)

	// Bonus for achieving results with minimal tool usage
	if totalToolCalls <= 3 && len(es.RequiredTools) > 0 {
		efficiency += 0.2 // 20% bonus for efficiency
	}

	if efficiency > 1.0 {
		efficiency = 1.0
	}

	return efficiency
}

// areToolParamsSimilar checks if two tool call arguments are similar
func areToolParamsSimilar(args1, args2 string) bool {
	// Simple similarity check for string arguments
	return args1 == args2
}

// EvalProcessor processes multiple EvalEvents for a complete agent run
type EvalProcessor struct {
	Suite     *EvalSuite
	Events    []aigentic.EvalEvent
	Results   []EvalResult
	AgentRuns map[string]*AgentRunData // Group by agent name
}

// AgentRunData groups runs by agent name
type AgentRunData struct {
	Name string
	Runs map[string]*RunData // Group by runID
}

// RunData contains data for a single run
type RunData struct {
	RunID   string
	Events  []aigentic.EvalEvent
	Results []EvalResult
	Summary EvalSummary
}

// ProcessEvent stores an EvalEvent without processing (deferred evaluation)
func (ep *EvalProcessor) ProcessEvent(event aigentic.EvalEvent) {
	ep.Events = append(ep.Events, event)
	// Results will be calculated in GetSummary()
}

// ProcessEventWithHistory stores an EvalEvent without processing (deferred evaluation)
func (ep *EvalProcessor) ProcessEventWithHistory(event aigentic.EvalEvent) {
	ep.Events = append(ep.Events, event)
	// Results will be calculated in GetSummary() with full history
}

// AddResult streams events as they come in, grouping by agent and run
func (ep *EvalProcessor) AddResult(agentName, runID string, event *aigentic.EvalEvent) {
	// Initialize agent if not exists
	if ep.AgentRuns[agentName] == nil {
		ep.AgentRuns[agentName] = &AgentRunData{
			Name: agentName,
			Runs: make(map[string]*RunData),
		}
	}

	// Initialize run if not exists
	if ep.AgentRuns[agentName].Runs[runID] == nil {
		ep.AgentRuns[agentName].Runs[runID] = &RunData{
			RunID:   runID,
			Events:  []aigentic.EvalEvent{},
			Results: []EvalResult{},
		}
	}

	// Add event without evaluation (deferred until GetSummary)
	runData := ep.AgentRuns[agentName].Runs[runID]
	runData.Events = append(runData.Events, *event)
	// Results and summary will be calculated in GetSummary()
}

// calculateRunSummary calculates summary for a single run
func (ep *EvalProcessor) calculateRunSummary(results []EvalResult, events []aigentic.EvalEvent) EvalSummary {
	if len(results) == 0 {
		return EvalSummary{}
	}

	totalChecks := len(results)
	passed := 0
	totalScore := 0.0
	var totalDuration time.Duration

	for _, result := range results {
		if result.Passed {
			passed++
		}
		totalScore += result.Score
	}

	for _, event := range events {
		totalDuration += event.Duration
	}

	return EvalSummary{
		TotalCalls:    len(events),
		TotalChecks:   totalChecks,
		PassedChecks:  passed,
		PassRate:      float64(passed) / float64(totalChecks) * 100,
		AverageScore:  totalScore / float64(totalChecks),
		TotalDuration: totalDuration,
		Results:       results,
	}
}

// GetSummary performs all evaluations and returns a summary of all events
func (ep *EvalProcessor) GetSummary() EvalSummary {
	if len(ep.Events) == 0 {
		return EvalSummary{}
	}

	// Clear previous results and recalculate everything
	ep.Results = nil

	// Process all events with full history for efficiency metrics
	for _, event := range ep.Events {
		results := ep.Suite.EvaluateWithHistory(event, ep.Events)
		ep.Results = append(ep.Results, results...)
	}

	// Calculate summary from all results
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

// GetCallResults returns evaluation results broken down by individual calls
func (ep *EvalProcessor) GetCallResults() []CallResult {
	if len(ep.Events) == 0 {
		return nil
	}

	// Ensure results are calculated
	if len(ep.Results) == 0 {
		ep.GetSummary()
	}

	var callResults []CallResult

	// Group results by event (call)
	for i, event := range ep.Events {
		// Find results for this specific event
		var eventResults []EvalResult
		for _, result := range ep.Results {
			if result.EventIndex == i {
				eventResults = append(eventResults, result)
			}
		}

		// Calculate metrics for this call
		totalChecks := len(eventResults)
		passed := 0
		totalScore := 0.0

		for _, result := range eventResults {
			if result.Passed {
				passed++
			}
			totalScore += result.Score
		}

		passRate := 0.0
		avgScore := 0.0
		if totalChecks > 0 {
			passRate = float64(passed) / float64(totalChecks) * 100
			avgScore = totalScore / float64(totalChecks)
		}

		callResult := CallResult{
			CallNumber: i + 1,
			Timestamp:  event.Timestamp,
			PassRate:   passRate,
			AvgScore:   avgScore,
			Results:    eventResults,
			Duration:   event.Duration,
		}

		callResults = append(callResults, callResult)
	}

	return callResults
}

// CallResult contains evaluation results for a single call
type CallResult struct {
	CallNumber int
	Timestamp  time.Time
	PassRate   float64
	AvgScore   float64
	Results    []EvalResult
	Duration   time.Duration
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

// NoToolCalls checks if the response has no tool calls (final response)
func NoToolCalls() EvalCheck {
	return func(event aigentic.EvalEvent) (bool, float64, string) {
		passed := len(event.Response.ToolCalls) == 0
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := "no tool calls"
		if len(event.Response.ToolCalls) > 0 {
			message = fmt.Sprintf("has %d tool calls", len(event.Response.ToolCalls))
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
	accuracy, relevance := CalculateAccuracyRelevance(summary.Results)
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

// CalculateAccuracyRelevance calculates accuracy and relevance scores from results
func CalculateAccuracyRelevance(results []EvalResult) (accuracy, relevance float64) {
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

// GetComparisonTable returns comparison data across all agents
func (ep *EvalProcessor) GetComparisonTable() []ComparisonResult {
	var results []ComparisonResult

	for agentName, agentData := range ep.AgentRuns {
		// Aggregate across all runs for this agent
		var allResults []EvalResult
		var allEvents []aigentic.EvalEvent

		for _, runData := range agentData.Runs {
			allResults = append(allResults, runData.Results...)
			allEvents = append(allEvents, runData.Events...)
		}

		if len(allResults) > 0 {
			summary := ep.calculateRunSummary(allResults, allEvents)
			accuracy, relevance := CalculateAccuracyRelevance(allResults)

			results = append(results, ComparisonResult{
				Name:           agentName,
				PassRate:       summary.PassRate,
				AverageScore:   summary.AverageScore,
				AccuracyScore:  accuracy,
				RelevanceScore: relevance,
				Duration:       summary.TotalDuration,
				ErrorCount:     len(allResults) - summary.PassedChecks,
			})
		}
	}

	return results
}

// ComparisonResult contains comparison data for one agent
type ComparisonResult struct {
	Name           string
	PassRate       float64
	AverageScore   float64
	AccuracyScore  float64
	RelevanceScore float64
	Duration       time.Duration
	ErrorCount     int
}

// PrintComparisonTable prints formatted comparison across all agents
func (ep *EvalProcessor) PrintComparisonTable() {
	results := ep.GetComparisonTable()
	if len(results) == 0 {
		fmt.Println("No results to compare")
		return
	}

	fmt.Printf("\n=== Agent Performance Comparison ===\n")
	fmt.Printf("%-20s | %-8s | %-8s | %-8s | %-8s | %-8s | %-6s\n",
		"Agent", "Pass%", "AvgScore", "Accuracy", "Relevance", "Duration", "Errors")
	fmt.Printf("%s\n", strings.Repeat("-", 85))

	// Sort by average score (best first)
	sortedResults := make([]ComparisonResult, len(results))
	copy(sortedResults, results)

	// Simple bubble sort by AverageScore
	for i := 0; i < len(sortedResults)-1; i++ {
		for j := 0; j < len(sortedResults)-i-1; j++ {
			if sortedResults[j].AverageScore < sortedResults[j+1].AverageScore {
				sortedResults[j], sortedResults[j+1] = sortedResults[j+1], sortedResults[j]
			}
		}
	}

	for i, result := range sortedResults {
		rank := ""
		if i == 0 {
			rank = "🏆 "
		} else if i == 1 {
			rank = "🥈 "
		} else if i == 2 {
			rank = "🥉 "
		}

		fmt.Printf("%s%-18s | %7.1f%% | %8.2f | %8.2f | %8.2f | %8s | %6d\n",
			rank, result.Name, result.PassRate, result.AverageScore,
			result.AccuracyScore, result.RelevanceScore,
			FormatDuration(result.Duration), result.ErrorCount)
	}

	// Show winner
	if len(sortedResults) > 0 {
		winner := sortedResults[0]
		fmt.Printf("\n🎯 Best Performer: %s (%.2f avg score, %.1f%% pass rate)\n",
			winner.Name, winner.AverageScore, winner.PassRate)
	}
}

// FormatDuration formats duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// HasToolArgs checks if the tool has arguments with minimum length
func HasToolArgs(minLength int) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		argsLen := len(strings.TrimSpace(toolArgs))
		passed := argsLen >= minLength

		score := 1.0
		if argsLen < minLength {
			score = float64(argsLen) / float64(minLength)
		}

		return passed, score, fmt.Sprintf("tool %s args length: %d chars (min %d)", toolName, argsLen, minLength)
	}
}

// HasToolKeywords checks if the tool arguments contain expected keywords
func HasToolKeywords(keywords ...string) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		if len(keywords) == 0 {
			return true, 1.0, "no keywords to check"
		}

		args := strings.ToLower(toolArgs)
		matches := 0

		for _, keyword := range keywords {
			if strings.Contains(args, strings.ToLower(keyword)) {
				matches++
			}
		}

		score := float64(matches) / float64(len(keywords))
		passed := score >= 0.5 // At least half the keywords

		return passed, score, fmt.Sprintf("tool %s found %d/%d keywords: %s", toolName, matches, len(keywords), toolArgs)
	}
}

// ToolArgsNotEmpty checks if the tool has non-empty arguments
func ToolArgsNotEmpty() ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		passed := len(strings.TrimSpace(toolArgs)) > 0
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := fmt.Sprintf("tool %s has arguments: %s", toolName, toolArgs)
		if !passed {
			message = fmt.Sprintf("tool %s has no arguments", toolName)
		}

		return passed, score, message
	}
}

// ToolArgsFormat checks if tool arguments match expected format (simple regex-like check)
func ToolArgsFormat(expectedPattern string) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		// Simple pattern matching - in practice you might want more sophisticated regex
		passed := strings.Contains(toolArgs, expectedPattern)
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := fmt.Sprintf("tool %s args match pattern '%s': %s", toolName, expectedPattern, toolArgs)
		if !passed {
			message = fmt.Sprintf("tool %s args don't match pattern '%s': %s", toolName, expectedPattern, toolArgs)
		}

		return passed, score, message
	}
}

// ToolArgsContains checks if tool arguments contain specific text
func ToolArgsContains(expectedText string) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		passed := strings.Contains(strings.ToLower(toolArgs), strings.ToLower(expectedText))
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := fmt.Sprintf("tool %s args contain '%s': %s", toolName, expectedText, toolArgs)
		if !passed {
			message = fmt.Sprintf("tool %s args don't contain '%s': %s", toolName, expectedText, toolArgs)
		}

		return passed, score, message
	}
}

// ToolArgsStartsWith checks if tool arguments start with specific text
func ToolArgsStartsWith(prefix string) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		passed := strings.HasPrefix(strings.ToLower(toolArgs), strings.ToLower(prefix))
		score := 1.0
		if !passed {
			score = 0.0
		}

		message := fmt.Sprintf("tool %s args start with '%s': %s", toolName, prefix, toolArgs)
		if !passed {
			message = fmt.Sprintf("tool %s args don't start with '%s': %s", toolName, prefix, toolArgs)
		}

		return passed, score, message
	}
}

// ToolArgsEndsWith checks if tool arguments end with specific text
func ToolArgsEndsWith(suffix string) ToolCheck {
	return func(toolName, toolArgs string) (bool, float64, string) {
		passed := strings.HasSuffix(strings.ToLower(toolArgs), strings.ToLower(suffix))
		score := 1.0
		if !passed {
			score = 0.0
		}

		return passed, score, fmt.Sprintf("tool %s args end with '%s': %s", toolName, suffix, toolArgs)
	}
}

// HasContent checks if the response has meaningful content
