package aigentic

import (
	"log/slog"
	"testing"
	"time"

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
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
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

func getApprovalTool() ai.Tool {
	return ai.Tool{
		Name:        "test_approval_tool",
		Description: "A test tool that requires approval",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "The action to perform",
				},
			},
			"required": []string{"action"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			action, _ := args["action"].(string)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Tool executed: " + action}},
				Error:   false,
			}, nil
		},
		RequireApproval: true,
	}
}

func getApprovalTestData() []ai.RecordedResponse {
	return []ai.RecordedResponse{
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{
						ID:   "call_1",
						Name: "test_approval_tool",
						Args: `{"action": "test_action"}`,
					},
				},
			},
			Error: "",
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Tool execution completed successfully.",
			},
			Error: "",
		},
	}
}

func TestToolApprovalGiven(t *testing.T) {
	approvalTool := getApprovalTool()
	testData := getApprovalTestData()

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}
	model := ai.NewDummyModel(replayFunc)

	agent := Agent{
		Model:        model,
		Name:         "approval_test_agent",
		Description:  "Test agent for approval functionality",
		Instructions: "Use the test_approval_tool when requested.",
		Tools:        []ai.Tool{approvalTool},
		Trace:        NewTrace(),
		LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Run("Please execute the test tool with action 'test_action'")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var approvalEvent *ApprovalEvent
	var toolEvent *ToolEvent

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ApprovalEvent:
			approvalEvent = e
			run.Approve(e.ApprovalID, true)
		case *ToolEvent:
			toolEvent = e
		case *ErrorEvent:
			t.Fatalf("Unexpected error: %v", e.Err)
		}
	}

	assert.NotNil(t, approvalEvent, "Should have received an ApprovalEvent")
	assert.NotEmpty(t, approvalEvent.ApprovalID, "ApprovalEvent should have an approval ID")
	assert.Contains(t, approvalEvent.Content, "Approval required for tool: test_approval_tool", "Content should mention the tool")
	assert.NotNil(t, toolEvent, "Should have received a ToolEvent")
	assert.Contains(t, finalContent, "Tool execution completed successfully", "Should have final content")
	assert.Equal(t, 0, len(run.pendingApprovals), "Should have no pending approvals")
}

func TestToolApprovalRejected(t *testing.T) {
	approvalTool := getApprovalTool()
	testData := getApprovalTestData()

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}
	model := ai.NewDummyModel(replayFunc)

	agent := Agent{
		Model:        model,
		Name:         "approval_test_agent",
		Description:  "Test agent for approval functionality",
		Instructions: "Use the test_approval_tool when requested.",
		Tools:        []ai.Tool{approvalTool},
		Trace:        NewTrace(),
		LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Run("Please execute the test tool with action 'test_action'")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var approvalEvent *ApprovalEvent
	var toolRequested bool
	var toolResponseReceived bool

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ApprovalEvent:
			approvalEvent = e
			toolRequested = true
			run.Approve(e.ApprovalID, false)
		case *ToolResponseEvent:
			toolResponseReceived = true
			assert.Contains(t, e.Content, "approval denied", "Tool response should indicate approval was denied")
		case *ErrorEvent:
			// Error might occur if the approval system times out
			t.Logf("Error occurred: %v", e.Err)
		}
	}

	assert.NotNil(t, approvalEvent, "Should have received an ApprovalEvent")
	if approvalEvent != nil {
		assert.NotEmpty(t, approvalEvent.ApprovalID, "ApprovalEvent should have an approval ID")
		assert.Contains(t, approvalEvent.Content, "Approval required for tool: test_approval_tool", "Content should mention the tool")
	}
	assert.True(t, toolRequested, "Tool should have been requested")
	assert.True(t, toolResponseReceived, "Should have received a tool response indicating denial")
	assert.Equal(t, 0, len(run.pendingApprovals), "Should have no pending approvals")
}

func TestToolApprovalTimeout(t *testing.T) {
	approvalTool := getApprovalTool()
	testData := getApprovalTestData()

	replayFunc, err := ai.ReplayFunctionFromData(testData)
	if err != nil {
		t.Fatalf("Failed to create replay function: %v", err)
	}
	model := ai.NewDummyModel(replayFunc)

	agent := Agent{
		Model:        model,
		Name:         "timeout_test_agent",
		Description:  "Test agent for approval timeout functionality",
		Instructions: "Use the test_approval_tool when requested.",
		Tools:        []ai.Tool{approvalTool},
		Trace:        NewTrace(),
		LogLevel:     slog.LevelDebug,
	}

	approvalTimeout = time.Millisecond * 300
	tickerInterval = time.Millisecond * 100

	run, err := agent.Run("Please execute the test tool with action 'test_action'")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var approvalEvent *ApprovalEvent
	var toolRequested bool
	var toolResponseReceived bool

	timeout := time.After(500 * time.Millisecond)
	done := make(chan bool)

	go func() {
		defer func() { done <- true }()
		for ev := range run.Next() {
			switch e := ev.(type) {
			case *ApprovalEvent:
				approvalEvent = e
				toolRequested = true
				// Don't approve, let it timeout
			case *ToolResponseEvent:
				toolResponseReceived = true
			case *ErrorEvent:
				t.Logf("Error occurred: %v", e.Err)
			}
		}
	}()

	select {
	case <-timeout:
		// Force timeout to occur
	case <-done:
		// Test completed normally
	}

	assert.NotNil(t, approvalEvent, "Should have received an ApprovalEvent")
	if approvalEvent != nil {
		assert.NotEmpty(t, approvalEvent.ApprovalID, "ApprovalEvent should have an approval ID")
		assert.Contains(t, approvalEvent.Content, "Approval required for tool: test_approval_tool", "Content should mention the tool")
	}
	assert.True(t, toolRequested, "Tool should have been requested")
	assert.Equal(t, 0, len(run.pendingApprovals), "Should have no pending approvals")
	t.Logf("Tool response received: %v", toolResponseReceived)
}
