package integration

import (
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
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
