package aigentic

import (
	"log/slog"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

func TestDummyBasicAgent(t *testing.T) {
	// Test data specific to BasicAgent
	testData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The capital of New South Wales, Australia is Sydney.",
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The capital of New South Wales, Australia is Sydney.",
			},
			Error: "",
		},
	}

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}

	model := ai.NewDummyModel(replayFunc)

	// Use the integration suite
	TestBasicAgent(t, model)
}

func TestDummyAgentRun(t *testing.T) {
	// Test data specific to AgentRun
	testData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The capital of Australia is Canberra.",
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Recursion is a programming concept where a function calls itself.",
			},
			Error: "",
		},
	}

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}

	model := ai.NewDummyModel(replayFunc)

	// Use the integration suite
	TestAgentRun(t, model)
}

func TestDummyToolIntegration(t *testing.T) {
	// Test data specific to ToolIntegration
	testData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "150"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The company with number 150 is Nexxia.",
			},
			Error: "",
		},
	}

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}

	model := ai.NewDummyModel(replayFunc)

	// Use the integration suite
	TestToolIntegration(t, model)
}

func TestDummyConcurrentRuns(t *testing.T) {
	// Test data specific to ConcurrentRuns
	testData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "150"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The company with number 150 is Nexxia.",
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Paris",
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "4",
			},
			Error: "",
		},
	}

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}

	model := ai.NewDummyModel(replayFunc)

	// Use the integration suite
	TestConcurrentRuns(t, model)
}

func TestDummyLLMCallLimit(t *testing.T) {
	// Test data that will cause infinite tool calls (LLM keeps requesting tools)
	testData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "150"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "151"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "152"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "153"}`,
					},
				},
			},
			Error: "",
		},
	}

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}

	model := ai.NewDummyModel(replayFunc)

	// Create a tool that always returns a response
	tool := ai.Tool{
		Name:        "lookup_company_name",
		Description: "A tool that looks up the name of a company based on a company number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"company_number": map[string]interface{}{
					"type":        "string",
					"description": "The company number to lookup",
				},
			},
			"required": []string{"company_number"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Nexxia"}},
				Error:   false,
			}, nil
		},
	}

	// Test agent with LLM call limit
	agent := Agent{
		Model:        model,
		Name:         "limited_agent",
		Description:  "You are a helpful assistant that looks up company information.",
		Instructions: "Always use the lookup_company_name tool to get information.",
		MaxLLMCalls:  3, // Limit to 3 LLM calls
		Tools:        []ai.Tool{tool},
		Trace:        NewTrace(),
		LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Run("Look up company information")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var errorOccurred bool
	var errorMessage string

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			// Content received
		case *ToolEvent:
			if e.RequireApproval {
				run.Approve(e.ID())
			}
		case *ErrorEvent:
			errorOccurred = true
			errorMessage = e.Err.Error()
		}
	}

	// Verify that the limit was enforced
	assert.True(t, errorOccurred, "Expected an error due to LLM call limit")
	assert.Contains(t, errorMessage, "LLM call limit exceeded", "Error should mention LLM call limit")
	assert.Contains(t, errorMessage, "3 calls", "Error should mention the limit of 3 calls")
	assert.Contains(t, errorMessage, "configured limit: 3", "Error should mention the configured limit")
}

func TestDummyStreaming(t *testing.T) {
	// Test data for basic streaming (capital of France)
	basicTestData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The capital of France is Paris.",
			},
			Error: "",
		},
	}

	// Test data for content only streaming
	contentOnlyTestData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The capital of France is Paris, a beautiful city known for its culture and history.",
			},
			Error: "",
		},
	}

	// Test data for city summary streaming
	citySummaryTestData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Paris is the capital and largest city of France, known for its art, fashion, gastronomy and culture.",
			},
			Error: "",
		},
	}

	// Test data for streaming with tools
	toolsTestData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "150"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "The company with number 150 is Nexxia.",
			},
			Error: "",
		},
	}

	// Test data for tool lookup streaming
	toolLookupTestData := []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						Name: "lookup_company_name",
						Args: `{"company_number": "150"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Based on the lookup, the company with number 150 is Nexxia.",
			},
			Error: "",
		},
	}

	// Run the specific streaming tests with their own test data
	t.Run("BasicStreaming", func(t *testing.T) {
		replayFunc, err := ai.ReplayFunctionFromData(basicTestData)
		if err != nil {
			t.Fatalf("Failed to create replay function: %v", err)
		}
		model := ai.NewDummyModel(replayFunc)
		TestBasicStreaming(t, model)
	})

	t.Run("StreamingContentOnly", func(t *testing.T) {
		replayFunc, err := ai.ReplayFunctionFromData(contentOnlyTestData)
		if err != nil {
			t.Fatalf("Failed to create replay function: %v", err)
		}
		model := ai.NewDummyModel(replayFunc)
		TestStreamingContentOnly(t, model)
	})

	t.Run("StreamingWithCitySummary", func(t *testing.T) {
		replayFunc, err := ai.ReplayFunctionFromData(citySummaryTestData)
		if err != nil {
			t.Fatalf("Failed to create replay function: %v", err)
		}
		model := ai.NewDummyModel(replayFunc)
		TestStreamingWithCitySummary(t, model)
	})

	t.Run("StreamingWithTools", func(t *testing.T) {
		replayFunc, err := ai.ReplayFunctionFromData(toolsTestData)
		if err != nil {
			t.Fatalf("Failed to create replay function: %v", err)
		}
		model := ai.NewDummyModel(replayFunc)
		TestStreamingWithTools(t, model)
	})

	t.Run("StreamingToolLookup", func(t *testing.T) {
		replayFunc, err := ai.ReplayFunctionFromData(toolLookupTestData)
		if err != nil {
			t.Fatalf("Failed to create replay function: %v", err)
		}
		model := ai.NewDummyModel(replayFunc)
		TestStreamingToolLookup(t, model)
	})
}
