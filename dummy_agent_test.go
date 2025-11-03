package aigentic

import (
	"context"
	"fmt"
	"strings"
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
	type LookupCompanyInput struct {
		CompanyNumber string `json:"company_number" description:"The company number to lookup"`
	}

	tool := NewTool(
		"lookup_company_name",
		"A tool that looks up the name of a company based on a company number",
		func(run *AgentRun, input LookupCompanyInput) (string, error) {
			return "Nexxia", nil
		},
	)

	// Test agent with LLM call limit
	agent := Agent{
		Model:        model,
		Name:         "limited_agent",
		Description:  "You are a helpful assistant that looks up company information.",
		Instructions: "Always use the lookup_company_name tool to get information.",
		MaxLLMCalls:  3, // Limit to 3 LLM calls
		AgentTools:   []AgentTool{tool},
		Tracer:       NewTracer(),
		// LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Start("Look up company information")
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

func getApprovalTool() AgentTool {
	type ApprovalToolInput struct {
		Action string `json:"action" description:"The action to perform"`
	}

	approvalTool := NewTool(
		"test_approval_tool",
		"A test tool that requires approval",
		func(run *AgentRun, input ApprovalToolInput) (string, error) {
			return "Tool executed: " + input.Action, nil
		},
	)
	approvalTool.RequireApproval = true
	return approvalTool
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
		AgentTools:   []AgentTool{approvalTool},
		Tracer:       NewTracer(),
		// LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Start("Please execute the test tool with action 'test_action'")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var approvalEvent *ApprovalEvent
	var toolEvent *ToolEvent

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			finalContent = e.Content
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
	assert.Contains(t, approvalEvent.ValidationResult.Message, "", "message should be empty")
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
		AgentTools:   []AgentTool{approvalTool},
		Tracer:       NewTracer(),
		// LogLevel:     slog.LevelDebug,
	}

	run, err := agent.Start("Please execute the test tool with action 'test_action'")
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
		assert.Contains(t, approvalEvent.ValidationResult.Message, "", "message should be empty")
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
		AgentTools:   []AgentTool{approvalTool},
		Tracer:       NewTracer(),
		// LogLevel:     slog.LevelDebug,
	}

	approvalTimeout = time.Millisecond * 300
	tickerInterval = time.Millisecond * 100

	run, err := agent.Start("Please execute the test tool with action 'test_action'")
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
		assert.Contains(t, approvalEvent.ValidationResult.Message, "", "message should be empty")
	}
	assert.True(t, toolRequested, "Tool should have been requested")
	assert.Equal(t, 0, len(run.pendingApprovals), "Should have no pending approvals")
	t.Logf("Tool response received: %v", toolResponseReceived)
}

func TestDummyTeamCoordination(t *testing.T) {
	// Create separate test data for each agent in the team coordination

	// Coordinator responses - handles orchestration and final responses
	coordinatorData := []ai.RecordedResponse{
		// Test 1: Existing company "Nexxia"
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "lookup", Args: `{"input": "Nexxia"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "save_memory", Args: `{"content": "Company found: COMP-001 Nexxia"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "invoice_creator", Args: `{"input": "company_id: COMP-001, amount: 100"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "COMPANY_ID: COMP-001; NAME: Nexxia; INVOICE_ID: INV-1001; AMOUNT: 100",
			},
		},

		// Test 2: Non-existing company "Contoso"
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "lookup", Args: `{"input": "Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "save_memory", Args: `{"content": "Company not found: Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "company_creator", Args: `{"input": "Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "save_memory", Args: `{"content": "Company created: COMP-CONTOSO-001 Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "invoice_creator", Args: `{"input": "company_id: COMP-CONTOSO-001, amount: 250"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "COMPANY_ID: COMP-CONTOSO-001; NAME: Contoso; INVOICE_ID: INV-1001; AMOUNT: 250",
			},
		},
	}

	// Lookup agent responses - handles company lookups
	lookupData := []ai.RecordedResponse{
		// Test 1: Lookup "Nexxia" - found
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "lookup_company_id", Args: `{"name": "Nexxia"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "COMPANY_ID: COMP-001; NAME: Nexxia",
			},
		},

		// Test 2: Lookup "Contoso" - not found
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "lookup_company_id", Args: `{"name": "Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "NOT_FOUND",
			},
		},
	}

	// Company creator responses - handles company creation
	companyCreatorData := []ai.RecordedResponse{
		// Only called in Test 2: Create "Contoso"
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "create_company", Args: `{"name": "Contoso"}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "COMPANY_ID: COMP-CONTOSO-001; NAME: Contoso",
			},
		},
	}

	// Invoice creator responses - handles invoice creation
	invoiceCreatorData := []ai.RecordedResponse{
		// Test 1: Create invoice for COMP-001
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "create_invoice", Args: `{"company_id": "COMP-001", "amount": 100}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "INVOICE_ID: INV-1001; AMOUNT: 100",
			},
		},

		// Test 2: Create invoice for COMP-CONTOSO-001
		{
			AIMessage: ai.AIMessage{
				Role: ai.AssistantRole,
				ToolCalls: []ai.ToolCall{
					{Name: "create_invoice", Args: `{"company_id": "COMP-CONTOSO-001", "amount": 250}`},
				},
			},
		},
		{
			AIMessage: ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "INVOICE_ID: INV-1001; AMOUNT: 250",
			},
		},
	}

	// Create separate models for each agent
	coordinatorReplayFunc, err := ai.ReplayFunctionFromData(coordinatorData)
	if err != nil {
		t.Fatalf("Failed to create coordinator replay function: %v", err)
	}
	coordinatorModel := ai.NewDummyModel(coordinatorReplayFunc)

	lookupReplayFunc, err := ai.ReplayFunctionFromData(lookupData)
	if err != nil {
		t.Fatalf("Failed to create lookup replay function: %v", err)
	}
	lookupModel := ai.NewDummyModel(lookupReplayFunc)

	companyCreatorReplayFunc, err := ai.ReplayFunctionFromData(companyCreatorData)
	if err != nil {
		t.Fatalf("Failed to create company creator replay function: %v", err)
	}
	companyCreatorModel := ai.NewDummyModel(companyCreatorReplayFunc)

	invoiceCreatorReplayFunc, err := ai.ReplayFunctionFromData(invoiceCreatorData)
	if err != nil {
		t.Fatalf("Failed to create invoice creator replay function: %v", err)
	}
	invoiceCreatorModel := ai.NewDummyModel(invoiceCreatorReplayFunc)

	// Create a custom test that mimics TestTeamCoordination but uses our separate models
	session := NewSession(context.Background())
	// Sessions no longer have Trace field

	// Subagents with their own models
	lookup := Agent{
		Model:        lookupModel,
		Name:         "lookup",
		Description:  "Lookup company details by name. Return either 'COMPANY_ID: <id>; NAME: <name>' or 'NOT_FOUND' only.",
		Instructions: "Use tools to perform the lookup and return the canonical format only.",
		AgentTools:   []AgentTool{NewLookupCompanyByNameTool()},
	}

	companyCreator := Agent{
		Model:        companyCreatorModel,
		Name:         "company_creator",
		Description:  "Create a new company by name and return 'COMPANY_ID: <id>; NAME: <name>' only.",
		Instructions: "Use tools to create the company and return the canonical format only.",
		AgentTools:   []AgentTool{NewCreateCompanyTool()},
	}

	invoiceCreator := Agent{
		Model:        invoiceCreatorModel,
		Name:         "invoice_creator",
		Description:  "Create an invoice for a given company_id and amount. Return 'INVOICE_ID: <id>; AMOUNT: <amount>' only.",
		Instructions: "Use tools to create the invoice and return the canonical format only.",
		AgentTools:   []AgentTool{NewCreateInvoiceTool()},
	}

	coordinator := Agent{
		Session: session,
		Model:   coordinatorModel,
		Name:    "coordinator",
		Description: "Coordinate a workflow to ensure an invoice exists for the requested company name and amount. " +
			"Steps: 1) Call 'lookup' subagent with the company name. 2) If NOT_FOUND, call 'company_creator' to create it. " +
			"3) Call 'invoice_creator' with the resolved company_id and the requested amount. " +
			"Finally, return exactly: 'COMPANY_ID: <id>; NAME: <name>; INVOICE_ID: <invoice>; AMOUNT: <amount>'.",
		Instructions: "Call exactly one tool at a time and wait for the response before the next call. " +
			"Use the save_memory tool to persist important context between tool calls, especially after getting company information and getting invoice information. " +
			"Do not add commentary.",
		Agents: []Agent{lookup, companyCreator, invoiceCreator},
		Tracer: NewTracer(),
	}

	// Now run the same test logic as TestTeamCoordination
	type testCase struct {
		name              string
		companyName       string
		amount            string
		expectCompanyID   string
		expectCompanyName string
		expectInvoiceID   string
		expectCallsCreate bool
	}

	tests := []testCase{
		{
			name:              "existing company",
			companyName:       "Nexxia",
			amount:            "100",
			expectCompanyID:   "COMP-001",
			expectCompanyName: "Nexxia",
			expectInvoiceID:   "INV-1001",
			expectCallsCreate: false,
		},
		{
			name:              "non-existing company",
			companyName:       "Contoso",
			amount:            "250",
			expectCompanyID:   "COMP-CONTOSO-001",
			expectCompanyName: "Contoso",
			expectInvoiceID:   "INV-1001",
			expectCallsCreate: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toolOrder := []string{}

			msg := fmt.Sprintf("Create an invoice for company '%s' for the amount %s. Return the final canonical line only.", tc.companyName, tc.amount)
			run, err := coordinator.Start(msg)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}

			var chunks []string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					chunks = append(chunks, e.Content)
				case *ToolEvent:
					toolOrder = append(toolOrder, e.ToolName)
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			finalContent := strings.Join(chunks, "")

			// Validate final content
			assert.NotEmpty(t, finalContent)
			assert.Contains(t, finalContent, "COMPANY_ID:")
			assert.Contains(t, finalContent, "NAME:")
			assert.Contains(t, finalContent, "INVOICE_ID:")
			assert.Contains(t, finalContent, "AMOUNT:")
			assert.Contains(t, finalContent, tc.expectCompanyID)
			assert.Contains(t, finalContent, tc.expectCompanyName)
			assert.Contains(t, finalContent, tc.expectInvoiceID)
			assert.Contains(t, finalContent, tc.amount)

			// Ensure orchestration used tools and subagents in expected order
			indexOf := func(name string) int {
				for i, n := range toolOrder {
					if n == name {
						return i
					}
				}
				return -1
			}

			// Coordinator should call lookup subagent first
			lookupIdx := indexOf("lookup")
			assert.NotEqual(t, -1, lookupIdx, "lookup subagent should be called")

			// It should create the company only in the non-existing path
			createCompanyIdx := indexOf("company_creator")
			if tc.expectCallsCreate {
				assert.NotEqual(t, -1, createCompanyIdx, "company_creator should be called when company is not found")
				assert.Greater(t, createCompanyIdx, lookupIdx, "company_creator should be called after lookup")
			} else {
				assert.Equal(t, -1, createCompanyIdx, "company_creator should not be called for existing company")
			}

			invoiceIdx := indexOf("invoice_creator")
			assert.NotEqual(t, -1, invoiceIdx, "invoice_creator should be called")
			if tc.expectCallsCreate {
				assert.Greater(t, invoiceIdx, createCompanyIdx, "invoice should be created after company creation")
			} else {
				assert.Greater(t, invoiceIdx, lookupIdx, "invoice should be created after lookup result")
			}

			// Check for save_memory usage
			saveIdx := indexOf("save_memory")
			if saveIdx == -1 {
				t.Log("Warning: save_memory was not called during the workflow. This may indicate the coordinator is not persisting context between steps.")
			} else {
				t.Logf("save_memory was called at position %d in the tool call sequence", saveIdx)
			}
		})
	}
}
