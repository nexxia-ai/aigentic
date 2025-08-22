package core

import (
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func RunConcurrentRuns(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	var agent = aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that can perform various tasks.",
		Instructions: "use tools when requested.",
		AgentTools:   []aigentic.AgentTool{NewCompanyNameTool()},
		Trace:        aigentic.NewTrace(),
	}

	// Define multiple sequential runs
	runs := []struct {
		name        string
		message     string
		expectsTool bool
	}{
		{
			name:        "tool call request",
			message:     "What is the name of the company with the number 150? Use tools.",
			expectsTool: true,
		},
		{
			name:        "simple question",
			message:     "What is the capital of France? respond with the name of the city only",
			expectsTool: false,
		},
		{
			name:        "another simple question",
			message:     "What is 2 + 2? respond with the answer only",
			expectsTool: false,
		},
	}

	// Start all runs first (parallel execution)
	var agentRuns []*aigentic.AgentRun
	for _, run := range runs {
		agentRun, err := agent.Start(run.message)
		if err != nil {
			result := CreateBenchResult("ConcurrentRuns", model, start, "", err)
			result.ErrorMessage = "Failed to start run: " + err.Error()
			return result, err
		}
		agentRuns = append(agentRuns, agentRun)
	}

	// Now wait for all runs to complete (parallel waiting)
	responses := make([]string, len(agentRuns))
	for i, agentRun := range agentRuns {
		response, err := agentRun.Wait(0)
		if err != nil {
			result := CreateBenchResult("ConcurrentRuns", model, start, "", err)
			result.ErrorMessage = "Wait for run failed: " + err.Error()
			return result, err
		}
		responses[i] = response
	}

	// Verify all responses
	if len(responses) != len(runs) {
		result := CreateBenchResult("ConcurrentRuns", model, start, "", nil)
		result.Success = false
		result.ErrorMessage = "Should have responses for all runs"
		return result, nil
	}

	// Check that tool calls were made when expected
	foundToolCall := false
	for _, response := range responses {
		if strings.Contains(response, "Nexxia") {
			foundToolCall = true
			break
		}
	}

	if !foundToolCall {
		result := CreateBenchResult("ConcurrentRuns", model, start, "", nil)
		result.Success = false
		result.ErrorMessage = "Should have found a response with tool call result"
		return result, nil
	}

	// Verify no errors occurred
	for _, response := range responses {
		if strings.Contains(response, "Error:") {
			result := CreateBenchResult("ConcurrentRuns", model, start, "", nil)
			result.Success = false
			result.ErrorMessage = "Run should not contain error"
			return result, nil
		}
		if response == "" {
			result := CreateBenchResult("ConcurrentRuns", model, start, "", nil)
			result.Success = false
			result.ErrorMessage = "Run should have non-empty response"
			return result, nil
		}
	}

	allResponses := strings.Join(responses, " | ")
	result := CreateBenchResult("ConcurrentRuns", model, start, allResponses, nil)

	result.Metadata["num_runs"] = len(runs)
	result.Metadata["tool_call_found"] = foundToolCall
	result.Metadata["response_preview"] = TruncateString(allResponses, 150)

	return result, nil
}
