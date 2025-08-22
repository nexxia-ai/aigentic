package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func RunSimpleAgent(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	agent := aigentic.Agent{
		Model:        model,
		Description:  "A basic conversational agent that provides clear and helpful responses",
		Instructions: "Answer questions clearly and concisely. For geography questions, provide accurate information.",
		Trace:        aigentic.NewTrace(),
	}
	response, err := agent.Execute("What is the capital of Australia?")

	duration := time.Since(start)

	result := BenchResult{
		TestCase:     "SimpleAgent",
		ModelName:    model.ModelName,
		Duration:     duration,
		ResponseSize: len(response),
	}

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	if !strings.Contains(strings.ToLower(response), "canberra") {
		err = fmt.Errorf("expected response to contain 'Canberra', got: %s", response)
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.Success = true
	result.Metadata = map[string]interface{}{
		"expected_content": "canberra",
		"response_preview": TruncateString(response, 100),
	}

	return result, nil
}
