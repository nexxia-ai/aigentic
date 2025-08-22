package core

import (
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func RunStreaming(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
		Trace:        aigentic.NewTrace(),
	}

	run, err := agent.Start("What is the capital of France and give me a brief summary of the city")
	if err != nil {
		result := CreateBenchResult("Streaming", model, start, "", err)
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
			result := CreateBenchResult("Streaming", model, start, "", e.Err)
			return result, e.Err
		}
	}

	finalContent := strings.Join(chunks, "")
	result := CreateBenchResult("Streaming", model, start, finalContent, nil)

	if err := ValidateResponse(finalContent, "paris"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	if len(chunks) < 2 {
		result.Success = false
		result.ErrorMessage = "Should have received streaming chunks"
		return result, nil
	}

	result.Metadata["chunk_count"] = len(chunks)
	result.Metadata["expected_content"] = "paris"
	result.Metadata["response_preview"] = TruncateString(finalContent, 100)

	return result, nil
}

func RunStreamingWithTools(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
		AgentTools:   []aigentic.AgentTool{NewCompanyNameTool()},
		Trace:        aigentic.NewTrace(),
	}

	run, err := agent.Start("tell me the name of the company with the number 150. Use tools.")
	if err != nil {
		result := CreateBenchResult("StreamingWithTools", model, start, "", err)
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
			result := CreateBenchResult("StreamingWithTools", model, start, "", e.Err)
			return result, e.Err
		}
	}

	finalContent := strings.Join(chunks, "")
	result := CreateBenchResult("StreamingWithTools", model, start, finalContent, nil)

	if err := ValidateResponse(finalContent, "Nexxia"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	if len(chunks) < 2 {
		result.Success = false
		result.ErrorMessage = "Should have received streaming chunks"
		return result, nil
	}

	result.Metadata["chunk_count"] = len(chunks)
	result.Metadata["expected_content"] = "Nexxia"
	result.Metadata["response_preview"] = TruncateString(finalContent, 100)

	return result, nil
}
