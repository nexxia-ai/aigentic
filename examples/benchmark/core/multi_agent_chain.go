package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/evals"
)

func NewMultiAgentChainAgent(model *ai.Model) aigentic.Agent {
	const numExperts = 3

	experts := make([]aigentic.Agent, numExperts)
	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		expertCompanyNumber := fmt.Sprintf("%d", i+1)
		idNumber := fmt.Sprintf("ID%d", i+1)
		experts[i] = aigentic.Agent{
			Name:        expertName,
			Description: "You are an expert in a group of experts. Your role is to respond with your name",
			Instructions: `
			Remember:
			return your name only
			do not add any additional information` +
				fmt.Sprintf("My name is %s and my company number is %s and my id number is %s.", expertName, expertCompanyNumber, idNumber),
			Model:      model,
			AgentTools: nil,
		}
	}

	coordinator := aigentic.Agent{
		Name:        "coordinator",
		Description: `You are the coordinator retrieve information from experts.`,
		Instructions: `
		Create a plan for what you have to do and save the plan to memory. 
		Update the plan as you proceed to reflect tasks already completed.
		Call each expert one by one in order to request their name - what is your name?
		Wait until you have received the response from the expert before calling the next expert.
		Save each expert name to memory.
		You will need to call the 
		You must call each expert in order and wait for the expert's response before calling the next expert. ie. call expert1, wait for the response, then call expert2, wait for the response, then call expert3, wait for the response.
		Do no make up information. Use only the names provided by the agents.
		Return the final names as received from the last expert. do not add any additional text or commentary.`,
		Model:      model,
		Agents:     experts,
		AgentTools: []aigentic.AgentTool{NewCompanyNameTool()},
		Memory:     aigentic.NewMemory(),
		Trace:      aigentic.NewTrace(),
	}

	return coordinator
}

func RunMultiAgentChain(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())

	coordinator := NewMultiAgentChainAgent(model)
	coordinator.Session = session

	prompt := `
	get the names of expert1, expert2 and expert3 then retrieve their company names.
	respond with a table of the experts, their company names and their id numbers in the order 
	`

	run, err := coordinator.Start(prompt)
	if err != nil {
		result := CreateBenchResult("MultiAgentChain", model, start, "", err)
		return result, err
	}

	response, err := run.Wait(0)
	if err != nil {
		result := CreateBenchResult("MultiAgentChain", model, start, "", err)
		return result, err
	}

	result := CreateBenchResult("MultiAgentChain", model, start, response, nil)

	// Validate that all experts are mentioned
	expectedExperts := []string{"expert1", "expert2", "expert3"}
	for _, expert := range expectedExperts {
		if err := ValidateResponse(response, expert); err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Missing expert '%s' in response", expert)
			return result, err
		}
	}

	pos1 := strings.Index(response, "expert1")
	pos2 := strings.Index(response, "expert2")
	pos3 := strings.Index(response, "expert3")

	if pos1 == -1 || pos2 == -1 || pos3 == -1 {
		result.Success = false
		result.ErrorMessage = "Not all experts found in response"
		return result, nil
	}

	if pos2 <= pos1 || pos3 <= pos2 {
		result.Success = false
		result.ErrorMessage = "Experts not in correct order (expert1 -> expert2 -> expert3)"
		return result, nil
	}

	result.Metadata["expected_experts"] = expectedExperts
	result.Metadata["expert_positions"] = map[string]int{"expert1": pos1, "expert2": pos2, "expert3": pos3}
	result.Metadata["response_preview"] = TruncateString(response, 100)

	return result, nil
}

// AgentTestResult holds the evaluation results for a single agent test
type AgentTestResult struct {
	Name           string
	PassRate       float64
	AvgScore       float64
	AccuracyScore  float64
	RelevanceScore float64
	Duration       time.Duration
	ErrorCount     int
	Content        string
	Failed         []string
}

// AgentVariant defines a test variant with its creation function
type AgentVariant struct {
	Name        string
	Description string
	CreateAgent func(*ai.Model, []aigentic.Agent) aigentic.Agent
}

func TestMultiAgentChainPrompts(model *ai.Model, scoreModel *ai.Model) {
	fmt.Println("=== Testing MultiAgentChain Prompt Variations ===")

	// Create evaluation suite for MultiAgentChain
	evalSuite := evals.NewEvalSuite("MultiAgentChain Evaluation")
	evalSuite.AddCheck("has expert keywords", evals.HasKeywords("expert1", "expert2", "expert3"))
	evalSuite.AddCheck("calls save memory", evals.CallsTools("save_memory"))
	evalSuite.AddCheck("has content", evals.HasContent(10))
	evalSuite.AddCheck("no errors", evals.NoErrors())
	evalSuite.AddCheck("responds quickly", evals.LatencyUnder(30*time.Second))

	// Create shared expert agents (reused across all tests)
	experts := createExpertAgents(model)

	// Define test variants in a table-driven approach
	variants := []AgentVariant{
		{
			Name:        "Original",
			Description: "Base multi-agent coordinator",
			CreateAgent: func(m *ai.Model, experts []aigentic.Agent) aigentic.Agent {
				return createOriginalAgentWithExperts(m, experts)
			},
		},
		{
			Name:        "Enhanced-Coordinator",
			Description: "Systematic coordinator with detailed steps",
			CreateAgent: func(m *ai.Model, experts []aigentic.Agent) aigentic.Agent {
				return createEnhancedCoordinatorAgentWithExperts(m, experts)
			},
		},
		{
			Name:        "Step-by-Step",
			Description: "Explicit step-by-step methodology",
			CreateAgent: func(m *ai.Model, experts []aigentic.Agent) aigentic.Agent {
				return createStepByStepAgentWithExperts(m, experts)
			},
		},
		{
			Name:        "Sequential",
			Description: "Sequential processing emphasis",
			CreateAgent: func(m *ai.Model, experts []aigentic.Agent) aigentic.Agent {
				return createSequentialAgentWithExperts(m, experts)
			},
		},
	}

	userMessage := `get the names of expert1, expert2 and expert3 then retrieve their company names.
respond with a table of the experts, their company names and their id numbers in the order`

	// Run table-driven tests
	results := runAgentVariantTests(variants, experts, model, evalSuite, userMessage)

	// Display results in table format
	printComparisonTable(results)
	printDetailedResults(results)
}

// createExpertAgents creates the shared expert agents used across all tests
func createExpertAgents(model *ai.Model) []aigentic.Agent {
	const numExperts = 3
	experts := make([]aigentic.Agent, numExperts)

	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		expertCompanyNumber := fmt.Sprintf("%d", i+1)
		idNumber := fmt.Sprintf("ID%d", i+1)
		experts[i] = aigentic.Agent{
			Name:        expertName,
			Description: "You are an expert in a group of experts. Your role is to respond with your name",
			Instructions: `
			Remember:
			return your name only
			do not add any additional information` +
				fmt.Sprintf("My name is %s and my company number is %s and my id number is %s.", expertName, expertCompanyNumber, idNumber),
			Model:            model,
			AgentTools:       nil,
			EnableEvaluation: true,
		}
	}
	return experts
}

// runAgentVariantTests executes all agent variants and collects results
func runAgentVariantTests(variants []AgentVariant, experts []aigentic.Agent, model *ai.Model, evalSuite *evals.EvalSuite, userMessage string) []AgentTestResult {
	var results []AgentTestResult

	for _, variant := range variants {
		fmt.Printf("\n--- Testing %s ---\n", variant.Name)

		// Create agent for this variant
		agent := variant.CreateAgent(model, experts)

		// Run the test
		result := runSingleAgentTest(agent, variant.Name, evalSuite, userMessage)
		results = append(results, result)

		// Show brief progress
		fmt.Printf("✅ %s: %.1f%% pass rate, %.2f avg score (duration: %v, events: %d)\n",
			variant.Name, result.PassRate, result.AvgScore,
			formatDuration(result.Duration), len(result.Failed))
	}

	return results
}

// runSingleAgentTest runs a single agent test and returns results
func runSingleAgentTest(agent aigentic.Agent, name string, evalSuite *evals.EvalSuite, userMessage string) AgentTestResult {
	result := AgentTestResult{
		Name:   name,
		Failed: []string{},
	}

	// Start the run
	run, err := agent.Start(userMessage)
	if err != nil {
		result.ErrorCount = 1
		result.Failed = append(result.Failed, fmt.Sprintf("Start error: %v", err))
		return result
	}

	// Process evaluation events
	processor := evals.NewEvalProcessor(evalSuite)
	content := ""
	errorCount := 0

	// Process events until completion (channel closes when agent finishes)
	eventCount := 0
	for event := range run.Next() {
		eventCount++
		switch ev := event.(type) {
		case *aigentic.ContentEvent:
			content = ev.Content
		case *aigentic.EvalEvent:
			processor.ProcessEvent(*ev)
		case *aigentic.ErrorEvent:
			errorCount++
		}
	}

	fmt.Printf("    [Debug] Processed %d events, %d errors\n", eventCount, errorCount)

	// Get evaluation summary
	summary := processor.GetSummary()

	// Calculate metrics
	result.PassRate = summary.PassRate
	result.AvgScore = summary.AverageScore
	result.Duration = summary.TotalDuration
	result.ErrorCount = errorCount
	result.Content = content

	// Calculate accuracy and relevance scores
	result.AccuracyScore, result.RelevanceScore = calculateAccuracyRelevance(summary.Results)

	// Collect failed checks
	for _, evalResult := range summary.Results {
		if !evalResult.Passed {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", evalResult.CheckName, evalResult.Message))
		}
	}

	return result
}

// printComparisonTable displays results in a formatted table
func printComparisonTable(results []AgentTestResult) {
	fmt.Printf("\n=== Agent Performance Comparison ===\n")
	fmt.Printf("%-20s | %-8s | %-8s | %-8s | %-8s | %-8s | %-6s\n",
		"Agent", "Pass%", "AvgScore", "Accuracy", "Relevance", "Duration", "Errors")
	fmt.Printf("%s\n", strings.Repeat("-", 85))

	// Sort by average score (best first)
	sortedResults := make([]AgentTestResult, len(results))
	copy(sortedResults, results)

	// Simple bubble sort by AvgScore
	for i := 0; i < len(sortedResults)-1; i++ {
		for j := 0; j < len(sortedResults)-i-1; j++ {
			if sortedResults[j].AvgScore < sortedResults[j+1].AvgScore {
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
			rank, result.Name, result.PassRate, result.AvgScore,
			result.AccuracyScore, result.RelevanceScore,
			formatDuration(result.Duration), result.ErrorCount)
	}

	// Show winner
	if len(sortedResults) > 0 {
		winner := sortedResults[0]
		fmt.Printf("\n🎯 Best Performer: %s (%.2f avg score, %.1f%% pass rate)\n",
			winner.Name, winner.AvgScore, winner.PassRate)
	}
}

// printDetailedResults shows detailed information for each agent
func printDetailedResults(results []AgentTestResult) {
	fmt.Printf("\n=== Detailed Results ===\n")

	for _, result := range results {
		fmt.Printf("\n--- %s ---\n", result.Name)

		if len(result.Failed) > 0 {
			fmt.Printf("❌ Failed Checks:\n")
			for _, failure := range result.Failed {
				fmt.Printf("   • %s\n", failure)
			}
		} else {
			fmt.Printf("✅ All checks passed\n")
		}

		// Show response preview
		if result.Content != "" {
			preview := result.Content
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("📄 Response: %s\n", preview)
		}
	}
}

// formatDuration formats duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// calculateAccuracyRelevance calculates accuracy and relevance scores from results
func calculateAccuracyRelevance(results []evals.EvalResult) (accuracy, relevance float64) {
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

// createEnhancedCoordinatorAgentWithExperts creates agent with more specific coordinator instructions using shared experts
func createEnhancedCoordinatorAgentWithExperts(model *ai.Model, experts []aigentic.Agent) aigentic.Agent {

	coordinator := aigentic.Agent{
		Name:        "enhanced_coordinator",
		Description: `You are a coordinator that systematically retrieves information from experts and organizes the results.`,
		Instructions: `
TASK OVERVIEW:
You must contact experts sequentially to gather their information and create a structured table.

STEP-BY-STEP PROCESS:
1. Create and save a plan to memory with clear steps
2. Contact expert1 with the question "what is your name?" and wait for response
3. Use the company name tool to get expert1's company information
4. Save expert1's complete information to memory
5. Repeat for expert2, then expert3
6. After collecting all information, create a table with columns: Expert Name | Company Name | ID Number
7. Present results in order: expert1, expert2, expert3

CRITICAL RULES:
- You MUST call experts sequentially (one at a time)
- Wait for each expert's complete response before proceeding
- Use only information provided by experts - do not make up data
- Save progress to memory after each step
- Final output must be a clear table format`,
		Model:            model,
		Agents:           experts,
		AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
		Memory:           aigentic.NewMemory(),
		Trace:            aigentic.NewTrace(),
		EnableEvaluation: true,
	}

	return coordinator
}

// createStepByStepAgentWithExperts creates agent with explicit step-by-step breakdown using shared experts
func createStepByStepAgentWithExperts(model *ai.Model, experts []aigentic.Agent) aigentic.Agent {

	coordinator := aigentic.Agent{
		Name:        "step_by_step_coordinator",
		Description: `You are a methodical coordinator that follows explicit steps to complete tasks.`,
		Instructions: `
Follow these exact steps in order:

STEP 1: Plan Creation
- Create a detailed plan and save it to memory
- Plan should list all steps you will take

STEP 2: Expert1 Information Gathering
- Call expert1 with message: "what is your name?"
- Wait for expert1's response
- Use company_name tool to get expert1's company information
- Save expert1's data to memory (name, company, ID)

STEP 3: Expert2 Information Gathering  
- Call expert2 with message: "what is your name?"
- Wait for expert2's response
- Use company_name tool to get expert2's company information
- Save expert2's data to memory (name, company, ID)

STEP 4: Expert3 Information Gathering
- Call expert3 with message: "what is your name?"
- Wait for expert3's response  
- Use company_name tool to get expert3's company information
- Save expert3's data to memory (name, company, ID)

STEP 5: Create Final Table
- Review all collected information from memory
- Create table format: | Expert | Company | ID |
- Present in order: expert1, expert2, expert3
- Return only the table, no additional commentary

IMPORTANT: Complete each step fully before moving to the next step.`,
		Model:            model,
		Agents:           experts,
		AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
		Memory:           aigentic.NewMemory(),
		Trace:            aigentic.NewTrace(),
		EnableEvaluation: true,
	}

	return coordinator
}

// createSequentialAgentWithExperts creates agent that emphasizes sequential processing using shared experts
func createSequentialAgentWithExperts(model *ai.Model, experts []aigentic.Agent) aigentic.Agent {

	coordinator := aigentic.Agent{
		Name:        "sequential_coordinator",
		Description: `You are a coordinator that processes tasks in strict sequential order.`,
		Instructions: `
SEQUENTIAL PROCESSING PROTOCOL:

PHASE 1 - Planning:
Save a plan to memory outlining the sequential steps

PHASE 2 - Sequential Expert Contact:
Execute in this exact order:
a) Contact expert1 → Wait for response → Process response → Save to memory
b) Contact expert2 → Wait for response → Process response → Save to memory  
c) Contact expert3 → Wait for response → Process response → Save to memory

PHASE 3 - Company Information Retrieval:
For each expert (in order):
a) Use company_name tool for expert1 → Save company info to memory
b) Use company_name tool for expert2 → Save company info to memory
c) Use company_name tool for expert3 → Save company info to memory

PHASE 4 - Table Generation:
Create table with format:
| Expert Name | Company Name | ID Number |
|-------------|-------------|-----------|
| expert1     | [company]   | [ID]      |
| expert2     | [company]   | [ID]      |  
| expert3     | [company]   | [ID]      |

CRITICAL RULES:
- Never process multiple experts simultaneously
- Always wait for complete response before next action
- Update memory after each completed action
- Maintain strict sequential order: expert1 → expert2 → expert3`,
		Model:            model,
		Agents:           experts,
		AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
		Memory:           aigentic.NewMemory(),
		Trace:            aigentic.NewTrace(),
		EnableEvaluation: true,
	}

	return coordinator
}

// createOriginalAgentWithExperts creates the original agent using shared experts
func createOriginalAgentWithExperts(model *ai.Model, experts []aigentic.Agent) aigentic.Agent {
	coordinator := aigentic.Agent{
		Name:        "coordinator",
		Description: `You are the coordinator retrieve information from experts.`,
		Instructions: `
		Create a plan for what you have to do and save the plan to memory. 
		Update the plan as you proceed to reflect tasks already completed.
		Call each expert one by one in order to request their name - what is your name?
		Wait until you have received the response from the expert before calling the next expert.
		Save each expert name to memory.
		Once you have all the names, retrieve the company names for each expert using the company_name tool.
		Finally, respond with a table of the experts, their company names and their id numbers in the order.`,
		Model:            model,
		Agents:           experts,
		AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
		Memory:           aigentic.NewMemory(),
		Trace:            aigentic.NewTrace(),
		EnableEvaluation: true,
	}
	return coordinator
}

// Backward compatibility functions - these create their own experts
func createEnhancedCoordinatorAgent(model *ai.Model) aigentic.Agent {
	experts := createExpertAgents(model)
	return createEnhancedCoordinatorAgentWithExperts(model, experts)
}

func createStepByStepAgent(model *ai.Model) aigentic.Agent {
	experts := createExpertAgents(model)
	return createStepByStepAgentWithExperts(model, experts)
}

func createSequentialAgent(model *ai.Model) aigentic.Agent {
	experts := createExpertAgents(model)
	return createSequentialAgentWithExperts(model, experts)
}

// RunMultiAgentEvaluation is a convenience function to run the evaluation
func RunMultiAgentEvaluation(model *ai.Model, scoreModel *ai.Model) {
	TestMultiAgentChainPrompts(model, scoreModel)
}
