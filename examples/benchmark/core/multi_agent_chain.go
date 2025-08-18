package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// NewMultiAgentChainAgent creates a coordinator with expert agents
func NewMultiAgentChainAgent(model *ai.Model) aigentic.Agent {
	const numExperts = 3

	experts := make([]*aigentic.Agent, numExperts)
	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		experts[i] = &aigentic.Agent{
			Name:        expertName,
			Description: "You are an expert in a group of experts. Your role is to respond with your name",
			Instructions: `
			Remember:
			return your name only
			do not add any additional information` +
				fmt.Sprintf("My name is %s.", expertName),
			Model:      model,
			AgentTools: nil,
		}
	}

	coordinator := aigentic.Agent{
		Name:        "coordinator",
		Description: "You are the coordinator to collect signature from experts. Your role is to call each expert one by one in order to get their names",
		Instructions: `
		Create a plan for what you have to do and save the plan to memory. 
		Update the plan as you proceed to reflect tasks already completed.
		Call each expert one by one in order to request their name - what is your name?
		Save each expert name to memory.
		You must call all the experts in order.
		Do no make up information. Use only the names provided by the agents.
		Return the final names as received from the last expert. do not add any additional text or commentary.`,
		Model:  model,
		Agents: experts,
		Trace:  aigentic.NewTrace(),
	}

	return coordinator
}

// RunMultiAgentChain executes the multi agent chain example and returns benchmark results
func RunMultiAgentChain(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())
	session.Trace = aigentic.NewTrace()

	coordinator := NewMultiAgentChainAgent(model)
	coordinator.Session = session

	run, err := coordinator.Start("call the names of expert1, expert2 and expert3 and return them in order, do not add any additional text or commentary.")
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

	// Check ordering - expert1 should appear before expert2, expert2 before expert3
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
