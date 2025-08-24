package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/evals"
)

// Agent definitions as variables at the top
var basicCoordinatorAgent = aigentic.Agent{
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
	AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
	Memory:           aigentic.NewMemory(),
	Trace:            aigentic.NewTrace(),
	EnableEvaluation: true,
}

var enhancedCoordinatorAgent = aigentic.Agent{
	Name:        "enhanced_coordinator",
	Description: `You are a coordinator that systematically retrieves information from experts and organizes the results.`,
	Instructions: `
EXECUTION STEPS:
1. Save plan to memory
2. Call expert1 tool → save response  
3. Call expert2 tool → save response
4. Call expert3 tool → save response
5. Call lookup_company_name for each expert
6. Create table: | Expert | Company | ID |
7. Present table and finish

RULES:
- Execute steps in order
- Save progress to memory
- Use actual expert responses
- Present clear final table`,
	AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
	Memory:           aigentic.NewMemory(),
	Trace:            aigentic.NewTrace(),
	EnableEvaluation: true,
}

var stepByStepCoordinatorAgent = aigentic.Agent{
	Name:        "step_by_step_coordinator",
	Description: `You are a methodical coordinator that follows explicit steps to complete tasks.`,
	Instructions: `
Execute these steps in exact order:

1. Save plan to memory
2. Call expert1 → save result
3. Call expert2 → save result  
4. Call expert3 → save result
5. Call lookup_company_name for each expert
6. Create table: | Expert | Company | ID |
7. Present table

Execute directly without overanalyzing.`,
	AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
	Memory:           aigentic.NewMemory(),
	Trace:            aigentic.NewTrace(),
	EnableEvaluation: true,
}

var sequentialCoordinatorAgent = aigentic.Agent{
	Name:        "sequential_coordinator",
	Description: `You are a coordinator that processes tasks in strict sequential order.`,
	Instructions: `
SEQUENTIAL PROTOCOL:

1. Save plan to memory
2. expert1 → save response
3. expert2 → save response  
4. expert3 → save response
5. lookup_company_name for each
6. Build final table
7. Present and stop

Execute one step at a time in strict order.`,
	AgentTools:       []aigentic.AgentTool{NewCompanyNameTool()},
	Memory:           aigentic.NewMemory(),
	Trace:            aigentic.NewTrace(),
	EnableEvaluation: true,
}

func RunMultiAgentChain(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	// Create coordinator agent
	coordinator := basicCoordinatorAgent
	coordinator.Model = model
	coordinator.Agents = createExpertAgents(model)

	agentResult := testAgentVariation(coordinator, "MultiAgentChain")

	// Convert AgentTestResult to BenchResult
	result := CreateBenchResult("MultiAgentChain", model, start, agentResult.Content, nil)
	result.Success = agentResult.Success
	if !agentResult.Success {
		result.ErrorMessage = fmt.Sprintf("Test failed with %.1f%% pass rate", agentResult.PassRate)
	}

	// Add metadata from the test result
	result.Metadata["pass_rate"] = agentResult.PassRate
	result.Metadata["avg_score"] = agentResult.AvgScore
	result.Metadata["duration"] = agentResult.Duration.String()
	result.Metadata["error_count"] = agentResult.ErrorCount

	return result, nil
}

// AgentVariation defines a test variation for the coordinator agents
type AgentVariation struct {
	Name        string
	Description string
	Agent       aigentic.Agent
}

// RunMultiAgentVariations tests all 4 coordinator agent variations
func RunMultiAgentVariations(model *ai.Model) {
	fmt.Println("=== Testing MultiAgent Chain Variations ===")

	// Create expert agents
	experts := createExpertAgents(model)
	createFunc := func(coordinator aigentic.Agent, model *ai.Model, experts []aigentic.Agent) aigentic.Agent {
		agent := coordinator
		agent.Model = model
		agent.Agents = experts
		return agent
	}

	// Define agent variations
	variations := []AgentVariation{
		{
			Name:        "Basic",
			Description: "Original coordinator with detailed instructions",
			Agent:       createFunc(basicCoordinatorAgent, model, experts),
		},
		{
			Name:        "Enhanced",
			Description: "Systematic coordinator with clear execution steps",
			Agent:       createFunc(enhancedCoordinatorAgent, model, experts),
		},
		{
			Name:        "Step-by-Step",
			Description: "Methodical coordinator with explicit steps",
			Agent:       createFunc(stepByStepCoordinatorAgent, model, experts),
		},
		{
			Name:        "Sequential",
			Description: "Strict sequential processing coordinator",
			Agent:       createFunc(sequentialCoordinatorAgent, model, experts),
		},
	}

	// Test each variation
	for _, variation := range variations {
		fmt.Printf("\n--- Testing %s ---\n", variation.Name)
		fmt.Printf("Description: %s\n", variation.Description)

		result := testAgentVariation(variation.Agent, variation.Name)

		if result.Success {
			fmt.Printf("✅ PASS: %.1f%% pass rate, %.2f avg score (%v)\n",
				result.PassRate, result.AvgScore, evals.FormatDuration(result.Duration))
		} else {
			fmt.Printf("❌ FAIL: %.1f%% pass rate, %.2f avg score (%v)\n",
				result.PassRate, result.AvgScore, evals.FormatDuration(result.Duration))
		}

		// Show detailed evaluation results
		fmt.Printf("   📊 Evaluation Details:\n")
		if len(result.Failed) > 0 {
			fmt.Printf("      ❌ Failed: %s\n", strings.Join(result.Failed, ", "))
		}
		if result.PassRate > 0 {
			fmt.Printf("      ✅ Pass Rate: %.1f%%\n", result.PassRate)
		}
		if result.AvgScore > 0 {
			fmt.Printf("      📈 Score: %.2f\n", result.AvgScore)
		}
		if result.AccuracyScore > 0 {
			fmt.Printf("      🎯 Accuracy: %.2f\n", result.AccuracyScore)
		}
		if result.RelevanceScore > 0 {
			fmt.Printf("      🔗 Relevance: %.2f\n", result.RelevanceScore)
		}
	}

	fmt.Println("\n=== MultiAgent Variations Testing Complete ===")
}

// createExpertAgents creates the shared expert agents
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

// testAgentVariation tests a single agent variation
func testAgentVariation(agent aigentic.Agent, name string) AgentTestResult {
	result := AgentTestResult{
		Name:   name,
		Failed: []string{},
	}

	// Create evaluation suite for this test
	evalSuite := evals.NewEvalSuite(fmt.Sprintf("%s Evaluation", name))

	// Add universal checks (run on every event)
	evalSuite.AddCheck("no errors", evals.NoErrors())
	evalSuite.AddCheck("responds quickly", evals.LatencyUnder(60*time.Second)) // More lenient timing

	// Add tool-specific checks for tool parameter validation (run once per tool call)
	evalSuite.AddToolCheck("expert1", evals.HasToolKeywords("what is your name?"))
	evalSuite.AddToolCheck("expert2", evals.HasToolKeywords("what is your name?"))
	evalSuite.AddToolCheck("expert3", evals.HasToolKeywords("what is your name?"))
	evalSuite.AddToolCheck("lookup_company_name", evals.HasToolKeywords("what is your company name?"))

	// Add final tool checks for usage counting (run once per result)
	evalSuite.AddFinalToolCheck("expert1", 1)
	evalSuite.AddFinalToolCheck("expert2", 1)
	evalSuite.AddFinalToolCheck("expert3", 1)
	evalSuite.AddFinalToolCheck("lookup_company_name", 3)
	evalSuite.AddFinalToolCheck("save_memory", -1) // called 1 or more times

	// Add final result checks (run only on final result)
	evalSuite.AddFinalCheck("has table", evals.HasKeywords("table", "Expert", "Company"))
	evalSuite.AddFinalCheck("complete response", evals.HasContent(30)) // Lower content requirement
	evalSuite.AddFinalCheck("mentions experts", evals.HasKeywords("expert1", "expert2", "expert3"))
	evalSuite.AddFinalCheck("mentions company", evals.HasKeywords("company", "corp", "inc", "ltd"))

	// Create expert agents and set them on the agent
	experts := createExpertAgents(agent.Model)
	agent.Agents = experts

	// Define the standard prompt
	userMessage := `get the names of expert1, expert2 and expert3 then retrieve their company names.
respond with a table of the experts, their company names and their id numbers in the order`

	// Start the run
	run, err := agent.Start(userMessage)
	if err != nil {
		result.ErrorCount = 1
		result.Failed = append(result.Failed, fmt.Sprintf("Start error: %v", err))
		return result
	}

	// Process evaluation events (deferred evaluation)
	processor := evalSuite.NewProcessor()
	content := ""
	errorCount := 0

	// Process events until completion (no evaluation during loop)
	for event := range run.Next() {
		switch ev := event.(type) {
		case *aigentic.ContentEvent:
			content = ev.Content
		case *aigentic.EvalEvent:
			fmt.Println("EvalEvent", ev.AgentName, ev.Sequence)
			if ev.AgentName == "coordinator" {
				processor.ProcessEventWithHistory(*ev)
			}
		case *aigentic.ErrorEvent:
			errorCount++
		}
	}

	// Get final evaluation summary (all calculations happen here)
	summary := processor.GetSummary()

	// Calculate metrics
	result.PassRate = summary.PassRate
	result.AvgScore = summary.AverageScore
	result.Duration = summary.TotalDuration
	result.ErrorCount = errorCount
	result.Content = content
	result.Success = summary.PassRate >= 60.0 // Consider 60%+ as success

	// Calculate accuracy and relevance scores
	result.AccuracyScore, result.RelevanceScore = evals.CalculateAccuracyRelevance(summary.Results)

	// Show call-by-call breakdown using the new deferred evaluation system
	fmt.Printf("      📋 Call-by-Call Evaluation Results:\n")
	callResults := processor.GetCallResults()
	for _, callResult := range callResults {
		fmt.Printf("         📞 Call #%d (%s) - Pass: %.1f%%, Score: %.2f\n",
			callResult.CallNumber, callResult.Timestamp.Format("15:04:05"),
			callResult.PassRate, callResult.AvgScore)

		// Show individual check results for this call
		for _, evalResult := range callResult.Results {
			if evalResult.Passed {
				fmt.Printf("            ✅ %s: PASSED\n", evalResult.CheckName)
			} else {
				fmt.Printf("            ❌ %s: FAILED - %s\n", evalResult.CheckName, evalResult.Message)
			}
		}
	}

	// Collect failed checks for overall summary
	for _, evalResult := range summary.Results {
		if !evalResult.Passed {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", evalResult.CheckName, evalResult.Message))
		}
	}

	return result
}

// RunMultiAgentVariationsWrapper is a wrapper that matches the RunFunction signature
func RunMultiAgentVariationsWrapper(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	// Run the variations test
	RunMultiAgentVariations(model)

	// Return a simple result indicating completion
	result := CreateBenchResult("MultiAgentVariations", model, start, "Variations test completed", nil)
	result.Success = true
	result.Metadata["test_type"] = "variations"

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
	Success        bool
	ErrorMessage   string
}
