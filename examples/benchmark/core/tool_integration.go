package core

import (
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func RunToolIntegration(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	agent := aigentic.Agent{
		Model:        model,
		Name:         "test-agent",
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible. Use tools when requested.",
		AgentTools:   []aigentic.AgentTool{NewCompanyNameTool()},
		Trace:        aigentic.NewTrace(),
	}

	run, err := agent.Start("tell me the name of the company with the number 150. Use tools.")
	if err != nil {
		result := CreateBenchResult("ToolIntegration", model, start, "", err)
		return result, err
	}

	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *aigentic.ContentEvent:
			chunks = append(chunks, e.Content)
		case *aigentic.ToolEvent:
		case *aigentic.ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *aigentic.ErrorEvent:
			result := CreateBenchResult("ToolIntegration", model, start, "", e.Err)
			return result, e.Err
		}
	}

	response := ""
	for _, chunk := range chunks {
		response += chunk
	}

	result := CreateBenchResult("ToolIntegration", model, start, response, nil)

	if err := ValidateResponse(response, "Nexxia"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.Metadata["expected_content"] = "Nexxia"
	result.Metadata["response_preview"] = TruncateString(response, 100)

	return result, nil
}
