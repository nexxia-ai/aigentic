package core

import (
	"context"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// NewStreamingAgent creates an agent configured for streaming
func NewStreamingAgent(model *ai.Model) aigentic.Agent {
	return aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}
}

// NewStreamingWithToolsAgent creates a streaming agent with tools
func NewStreamingWithToolsAgent(model *ai.Model) aigentic.Agent {
	return aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
		AgentTools:   []aigentic.AgentTool{NewSecretNumberTool()},
	}
}

// RunStreaming executes the streaming example and returns benchmark results
func RunStreaming(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())
	session.Trace = aigentic.NewTrace()

	agent := NewStreamingAgent(model)
	agent.Session = session

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

	// Validate response contains expected content
	if err := ValidateResponse(finalContent, "paris"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Should have received streaming chunks
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

// RunStreamingWithTools executes streaming with tools and returns benchmark results
func RunStreamingWithTools(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())
	session.Trace = aigentic.NewTrace()

	agent := NewStreamingWithToolsAgent(model)
	agent.Session = session

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

	// Validate response contains tool result
	if err := ValidateResponse(finalContent, "Nexxia"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Should have received streaming chunks
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
