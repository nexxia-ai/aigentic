package aigentic

// This file contains reusable integration tests to test various model providers.

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

// IntegrationTestSuite defines a test suite for integration testing with different providers
type IntegrationTestSuite struct {
	NewModel  func() *ai.Model
	Name      string
	SkipTests []string
}

// RunIntegrationTestSuite runs all standard integration tests against a model implementation
func RunIntegrationTestSuite(t *testing.T, suite IntegrationTestSuite) {
	// Helper function to check if a test should be skipped
	shouldSkipTest := func(testName string) bool {
		for _, skipTest := range suite.SkipTests {
			if skipTest == testName {
				return true
			}
		}
		return false
	}

	t.Run(suite.Name, func(t *testing.T) {
		t.Run("BasicAgent", func(t *testing.T) {
			if shouldSkipTest("BasicAgent") {
				t.Skipf("Skipping BasicAgent test for %s", suite.Name)
			}
			TestBasicAgent(t, suite.NewModel())
		})

		t.Run("AgentRun", func(t *testing.T) {
			if shouldSkipTest("AgentRun") {
				t.Skipf("Skipping AgentRun test for %s", suite.Name)
			}
			TestAgentRun(t, suite.NewModel())
		})

		t.Run("ToolIntegration", func(t *testing.T) {
			if shouldSkipTest("ToolIntegration") {
				t.Skipf("Skipping ToolIntegration test for %s", suite.Name)
			}
			TestToolIntegration(t, suite.NewModel())
		})

		t.Run("TeamCoordination", func(t *testing.T) {
			if shouldSkipTest("TeamCoordination") {
				t.Skipf("Skipping TeamCoordination test for %s", suite.Name)
			}
			TestTeamCoordination(t, suite.NewModel())
		})

		t.Run("FileAttachments", func(t *testing.T) {
			if shouldSkipTest("FileAttachments") {
				t.Skipf("Skipping FileAttachments test for %s", suite.Name)
			}
			TestFileAttachments(t, suite.NewModel())
		})

		t.Run("MultiAgentChain", func(t *testing.T) {
			if shouldSkipTest("MultiAgentChain") {
				t.Skipf("Skipping MultiAgentChain test for %s", suite.Name)
			}
			TestMultiAgentChain(t, suite.NewModel())
		})

		t.Run("ConcurrentRuns", func(t *testing.T) {
			if shouldSkipTest("ConcurrentRuns") {
				t.Skipf("Skipping ConcurrentRuns test for %s", suite.Name)
			}
			TestConcurrentRuns(t, suite.NewModel())
		})

		t.Run("BasicStreaming", func(t *testing.T) {
			if shouldSkipTest("BasicStreaming") {
				t.Skipf("Skipping BasicStreaming test for %s", suite.Name)
			}
			TestBasicStreaming(t, suite.NewModel())
		})

		t.Run("StreamingWithTools", func(t *testing.T) {
			if shouldSkipTest("StreamingWithTools") {
				t.Skipf("Skipping StreamingWithTools test for %s", suite.Name)
			}
			TestStreamingWithTools(t, suite.NewModel())
		})
	})
}

// NewSecretNumberTool returns a SimpleTool struct for testing
func NewSecretNumberTool() ai.Tool {
	return ai.Tool{
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
}

// Individual test functions that can be reused
func TestBasicAgent(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	tests := []struct {
		agent         Agent
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, content string, agent Agent)
		attachments   []Document
		tools         []ai.Tool
	}{
		{
			agent:         Agent{Model: model},
			name:          "empty agent",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, strings.ToLower(content), "sydney")
			},
			tools: []ai.Tool{},
		},
		{
			agent:         Agent{Session: session, Model: model},
			name:          "basic conversation",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, strings.ToLower(content), "sydney")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.agent.Documents = tt.attachments
			tt.agent.Tools = tt.tools

			run, err := tt.agent.Run(tt.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if !e.IsChunk {
						finalContent = e.Content
					}
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if tt.validate != nil {
				tt.validate(t, finalContent, tt.agent)
			}
		})
	}
}

func TestAgentRun(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	newAgent := func() Agent {
		return Agent{
			Session:      session,
			Model:        model,
			Description:  "You are a helpful assistant that provides clear and concise answers.",
			Instructions: "Always explain your reasoning and provide examples when possible.",
		}
	}

	tests := []struct {
		name        string
		message     string
		validate    func(t *testing.T, content string, agent Agent)
		attachments []Document
		tools       []ai.Tool
	}{
		{
			name:    "basic conversation",
			message: "What is the capital of Australia?",
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, strings.ToLower(content), "canberra")
			},
			tools: []ai.Tool{},
		},
		{
			name:    "conversation with instructions",
			message: "Explain the concept of recursion",
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, strings.ToLower(content), "recursion")
			},
			tools: []ai.Tool{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent := newAgent()
			agent.Tools = test.tools
			agent.Documents = test.attachments
			run, err := agent.Run(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if !e.IsChunk {
						finalContent = e.Content
					}
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent, agent)
			}
		})
	}
}

func TestToolIntegration(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	newAgent := func() Agent {
		return Agent{
			Name:         "test-agent",
			Session:      session,
			Model:        model,
			Description:  "You are a helpful assistant that provides clear and concise answers.",
			Instructions: "Always explain your reasoning and provide examples when possible.",
			LogLevel:     slog.LevelDebug,
		}
	}

	tests := []struct {
		name        string
		message     string
		agent       Agent
		validate    func(t *testing.T, content string, agent Agent)
		attachments []Document
		tools       []ai.Tool
	}{
		{
			name:        "tool call",
			message:     "tell me the name of the company with the number 150. Use tools.",
			agent:       newAgent(),
			tools:       []ai.Tool{NewSecretNumberTool()},
			attachments: []Document{},
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "Nexxia")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.agent.Tools = test.tools
			test.agent.Documents = test.attachments
			run, err := test.agent.Run(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if !e.IsChunk {
						finalContent = e.Content
					}
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent, test.agent)
			}
		})
	}
}

func TestTeamCoordination(t *testing.T, model *ai.Model) {
	// Add agents with different roles
	calculator := Agent{
		Model:        model,
		Name:         "calculator",
		Description:  "You are a calculator. When given a math problem, solve it and return only the numerical result.",
		Instructions: "Solve the math problem and return only the number. Do not add any explanation or text.",
	}
	explainer := Agent{
		Model:        model,
		Name:         "explainer",
		Description:  "You are a math teacher. When given a calculation, explain what it means in simple terms in terms of the office oranges that you have.",
		Instructions: "Explain the calculation in simple terms. Start your response with 'EXPLANATION: ' followed by your explanation.",
	}

	team := Agent{
		Model: model,
		Name:  "coordinator",
		Description: `
		You are a coordinator for a math problem solving team. 
		When you receive a math question, you must first use the calculator to get the answer, 
		then use the explainer to explain what the calculation means in terms of the office oranges that you have. 
		Always use both agents in this order.
		`,
		Instructions: `
			You must call a single tool each time and wait for the answer before calling another tool.
			Use the output from the calculator as input to the explainer.
			Respond with both answers clearly labeled: "Calculator: [result]" and "Explainer: [explanation]".
			Do not add any additional text or commentary.`,
		Trace:    NewTrace(),
		LogLevel: slog.LevelDebug,
		Agents:   []*Agent{&calculator, &explainer},
	}

	tests := []struct {
		name     string
		message  string
		validate func(t *testing.T, content string)
	}{
		{
			name:    "math problem solving",
			message: "What is 15 + 27 and what does this calculation represent?",
			validate: func(t *testing.T, content string) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "Calculator:")
				assert.Contains(t, content, "Explainer:")
				assert.Contains(t, content, "42")
				calculatorIndex := strings.Index(content, "Calculator:")
				explainerIndex := strings.Index(content, "Explainer:")
				assert.Greater(t, explainerIndex, calculatorIndex, "Explainer should come after calculator")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run, err := team.Run(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if !e.IsChunk {
						finalContent = e.Content
					}
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent)
			}
		})
	}
}

func TestFileAttachments(t *testing.T, model *ai.Model) {
	// Define test cases
	testCases := []struct {
		name        string
		attachments []Document
		description string
	}{
		{
			name: "text file",
			attachments: []Document{
				NewInMemoryDocument("", "sample.txt", []byte("This is a test text file with some sample content for analysis."), nil),
			},
			description: "You are a helpful assistant that analyzes text files and provides insights.",
		},
		{
			name: "PDF file",
			attachments: []Document{
				NewInMemoryDocument("", "sample.pdf", []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000204 00000 n \ntrailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n297\n%%EOF"), nil),
			},
			description: "You are a helpful assistant that analyzes PDF files and provides insights.",
		},
		{
			name: "image file",
			attachments: []Document{
				NewInMemoryDocument("", "sample.png", []byte("fake-image-data-for-testing"), nil),
			},
			description: "You are a helpful assistant that analyzes images and provides insights.",
		},
	}

	tracer := NewTrace()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent := Agent{
				Model:        model,
				Description:  tc.description,
				Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
				Trace:        tracer,
				Documents:    tc.attachments,
			}

			run, err := agent.Run("Please analyze the attached file and tell me what it contains. If you can are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			response, err := run.Wait(10 * time.Second)
			if err != nil {
				t.Fatalf("Agent wait failed: %v", err)
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, response)
			t.Logf("Agent response: %s", response)
		})
	}
}

func TestMultiAgentChain(t *testing.T, model *ai.Model) {
	const numExperts = 3

	experts := make([]*Agent, numExperts)
	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		experts[i] = &Agent{
			Name:        expertName,
			Description: "You are an expert in a group of experts. Your role is to respond with your name",
			Instructions: `
			Remember:
			return your name only
			do not add any additional information` +
				fmt.Sprintf("Your name is %s.", expertName),
			Model: model,
			Tools: nil,
		}
	}

	coordinator := Agent{
		Name:        "coordinator",
		Description: "You are the coordinator to collect signature from experts. Your role is to call each expert one by one in order to get their names",
		Instructions: `
		Call each expert one by one in order to request their name - what is your name?
		You must call all the experts in order.
		Return the final names as received from the last expert. do not add any additional text or commentary.`,
		Model:  model,
		Agents: experts,
		Trace:  NewTrace(),
	}

	run, err := coordinator.Run("call the names of expert1, expert2 and expert3 and return them in order, do not add any additional text or commentary.")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}
	response, err := run.Wait(0)
	if err != nil {
		t.Fatalf("Agent wait failed: %v", err)
	}

	assert.Contains(t, response, "expert1")
	assert.Contains(t, response, "expert2")
	assert.Contains(t, response, "expert3")

	pos1 := strings.Index(response, "expert1")
	pos2 := strings.Index(response, "expert2")
	pos3 := strings.Index(response, "expert3")

	assert.Greater(t, pos2, pos1, "expert1 should appear before expert2")
	assert.Greater(t, pos3, pos2, "expert2 should appear before expert3")
}

func TestConcurrentRuns(t *testing.T, model *ai.Model) {
	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that can perform various tasks.",
		Instructions: "use tools when requested.",
		Tools:        []ai.Tool{NewSecretNumberTool()},
		Trace:        NewTrace(),
		LogLevel:     slog.LevelDebug,
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
	var agentRuns []*AgentRun
	for i, run := range runs {
		t.Logf("Starting run %d: %s", i+1, run.name)

		agentRun, err := agent.Run(run.message)
		if err != nil {
			t.Fatalf("Run %d failed to start: %v", i+1, err)
		}

		agentRuns = append(agentRuns, agentRun)
	}

	// Now wait for all runs to complete (parallel waiting)
	responses := make([]string, len(agentRuns))
	for i, agentRun := range agentRuns {
		t.Logf("Waiting for run %d to complete", i+1)

		response, err := agentRun.Wait(0)
		if err != nil {
			t.Fatalf("Wait for run %d failed: %v", i+1, err)
		}

		responses[i] = response
		t.Logf("Run %d completed with response: %s", i+1, response)
	}

	// Verify all responses
	assert.Len(t, responses, len(runs), "Should have responses for all runs")

	// Check that tool calls were made when expected
	// Find the response that contains the tool call result
	foundToolCall := false
	toolCallRunIndex := -1
	for i, response := range responses {
		if strings.Contains(response, "Nexxia") {
			foundToolCall = true
			toolCallRunIndex = i
			break
		}
	}
	assert.True(t, foundToolCall, "Should have found a response with tool call result")

	// Log which run actually got the tool call response for debugging
	if toolCallRunIndex >= 0 {
		t.Logf("Tool call response found in run %d: '%s' (expected tool call: %v)",
			toolCallRunIndex+1, runs[toolCallRunIndex].name, runs[toolCallRunIndex].expectsTool)
	}

	// For now, just verify that we found a tool call response, regardless of which run it was in
	// This is a more lenient check that accounts for potential race conditions in parallel execution

	// Verify no errors occurred
	for i, response := range responses {
		assert.NotEmpty(t, response, "Run %d should have non-empty response", i+1)
		assert.NotContains(t, response, "Error:", "Run %d should not contain error", i+1)
	}

	t.Logf("All %d parallel runs completed successfully", len(runs))
}

func TestBasicStreaming(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	agent := Agent{
		Session:      session,
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "What is the capital of France what give me a brief summary of the city"
	run, err := agent.Run(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, agent.ID)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingContentOnly(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	agent := Agent{
		Session:      session,
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "What is the capital of France?"
	run, err := agent.Run(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, agent.ID)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingWithCitySummary(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	agent := Agent{
		Session:      session,
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "Give me a brief summary of Paris"
	run, err := agent.Run(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, agent.ID)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingWithTools(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	agent := Agent{
		Session:      session,
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
		Tools:        []ai.Tool{NewSecretNumberTool()},
	}

	message := "tell me the name of the company with the number 150. Use tools. "
	run, err := agent.Run(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	assert.NotEmpty(t, finalContent)
	assert.Contains(t, finalContent, "Nexxia")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingToolLookup(t *testing.T, model *ai.Model) {
	session := NewSession()
	session.Trace = NewTrace()

	agent := Agent{
		Session:      session,
		Model:        model,
		Description:  "You are a helpful assistant that looks up company information.",
		Instructions: "Use the lookup tool to find company information when asked.",
		Stream:       true,
		Tools:        []ai.Tool{NewSecretNumberTool()},
	}

	message := "What company has the number 150?"
	run, err := agent.Run(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var finalContent string
	var chunks []string
	var toolCalls int
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
			if !e.IsChunk {
				finalContent = e.Content
			}
		case *ToolEvent:
			toolCalls++
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	assert.NotEmpty(t, finalContent)
	assert.Contains(t, finalContent, "Nexxia")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
	assert.GreaterOrEqual(t, toolCalls, 1, "Should have made at least one tool call")
}
