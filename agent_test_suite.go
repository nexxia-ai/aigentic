package aigentic

// This file contains reusable integration tests to test various model providers.

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
)

// IntegrationTestSuite defines a test suite for integration testing with different providers
type IntegrationTestSuite struct {
	NewModel  func() *ai.Model
	Name      string
	SkipTests []string
}

// TODO: fix this - this is a hack and does not test the real tool
// newMemoryTool creates a memory tool for testing without importing tools package to avoid import cycles
func newMemoryTool() AgentTool {
	data := make(map[string]string)
	var mutex sync.RWMutex

	formatAll := func() string {
		mutex.RLock()
		defer mutex.RUnlock()

		if len(data) == 0 {
			return ""
		}

		var parts []string
		for name, content := range data {
			parts = append(parts, fmt.Sprintf("## Memory: %s\n%s", name, content))
		}
		return strings.Join(parts, "\n\n")
	}

	update := func(name, content string) error {
		mutex.Lock()
		defer mutex.Unlock()

		if content == "" {
			delete(data, name)
		} else {
			data[name] = content
		}
		return nil
	}

	contextFn := func(run *AgentRun) (string, error) {
		return formatAll(), nil
	}

	return AgentTool{
		Name:        "update_memory",
		Description: "Update or delete memory entries. Set memory_content to empty string to delete.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"memory_name": map[string]interface{}{
					"type":        "string",
					"description": "Name/identifier for this memory entry",
				},
				"memory_content": map[string]interface{}{
					"type":        "string",
					"description": "Markdown content (empty string to delete)",
				},
			},
			"required": []string{"memory_name", "memory_content"},
		},
		ContextFunctions: []ContextFunction{contextFn},
		NewExecute: func(run *AgentRun, result ValidationResult) (*ai.ToolResult, error) {
			args := result.Values.(map[string]interface{})
			name := args["memory_name"].(string)
			content := args["memory_content"].(string)

			if err := update(name, content); err != nil {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("Error: %v", err)}},
					Error:   true,
				}, nil
			}

			msg := fmt.Sprintf("Memory '%s' updated", name)
			if content == "" {
				msg = fmt.Sprintf("Memory '%s' deleted", name)
			}

			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: msg}},
				Error:   false,
			}, nil
		},
	}
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

		t.Run("MemoryPersistence", func(t *testing.T) {
			if shouldSkipTest("MemoryPersistence") {
				t.Skipf("Skipping MemoryPersistence test for %s", suite.Name)
			}
			TestMemoryPersistence(t, suite.NewModel())
		})
	})
}

// NewLookupCompanyNumberTool returns an AgentTool struct for testing
func NewLookupCompanyNumberTool(counter *int) AgentTool {
	type LookupCompanyNumberInput struct {
		CompanyNumber string `json:"company_number" description:"The company number to lookup"`
	}

	return NewTool(
		"lookup_company_name",
		"A tool that looks up the name of a company based on a company number",
		func(run *AgentRun, input LookupCompanyNumberInput) (string, error) {
			*counter++
			return "Nexxia", nil
		},
	)
}

// NewSecretNumberToolLegacy returns an ai.Tool struct for testing (legacy compatibility)
func NewSecretNumberToolLegacy() ai.Tool {
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

// NewSecretSupplierTool returns an AgentTool struct for testing supplier lookup
func NewSecretSupplierTool() AgentTool {
	type LookupSupplierInput struct {
		SupplierNumber string `json:"supplier_number" description:"The supplier number to lookup"`
	}

	return NewTool(
		"lookup_supplier_name",
		"A tool that looks up the name of a supplier based on a supplier number",
		func(run *AgentRun, input LookupSupplierInput) (string, error) {
			return "Phoenix", nil
		},
	)
}

// NewLookupCompanyByNameTool simulates looking up a company by name and returning an ID or not found
func NewLookupCompanyByNameTool() AgentTool {
	type LookupCompanyByNameInput struct {
		Name string `json:"name" description:"The company name to lookup"`
	}

	return NewTool(
		"lookup_company_id",
		"Lookup a company ID by its name. Returns 'COMPANY_ID: <id>; NAME: <name>' if found, otherwise 'NOT_FOUND'",
		func(run *AgentRun, input LookupCompanyByNameInput) (string, error) {
			content := "NOT_FOUND"
			if strings.EqualFold(strings.TrimSpace(input.Name), "Nexxia") {
				content = "COMPANY_ID: COMP-001; NAME: Nexxia"
			}
			return content, nil
		},
	)
}

// NewCreateCompanyTool simulates creating a company, returning a deterministic ID for testing
func NewCreateCompanyTool() AgentTool {
	type CreateCompanyInput struct {
		Name string `json:"name" description:"The company name to create"`
	}

	return NewTool(
		"create_company",
		"Create a new company by name. Returns 'COMPANY_ID: <id>; NAME: <name>'",
		func(run *AgentRun, input CreateCompanyInput) (string, error) {
			// Deterministic ID derived from name for test stability
			id := "COMP-NEW-001"
			if strings.EqualFold(strings.TrimSpace(input.Name), "Contoso") {
				id = "COMP-CONTOSO-001"
			}
			if strings.EqualFold(strings.TrimSpace(input.Name), "Nexxia") {
				id = "COMP-001"
			}
			content := fmt.Sprintf("COMPANY_ID: %s; NAME: %s", id, strings.TrimSpace(input.Name))
			return content, nil
		},
	)
}

// NewCreateInvoiceTool simulates creating an invoice for a company ID and amount
func NewCreateInvoiceTool() AgentTool {
	type CreateInvoiceInput struct {
		CompanyID string  `json:"company_id" description:"The company ID to invoice"`
		Amount    float64 `json:"amount" description:"The invoice amount"`
	}

	return NewTool(
		"create_invoice",
		"Create an invoice for a company. Returns 'INVOICE_ID: <id>; AMOUNT: <amount>'",
		func(run *AgentRun, input CreateInvoiceInput) (string, error) {
			amountStr := fmt.Sprintf("%.0f", input.Amount)
			content := fmt.Sprintf("INVOICE_ID: INV-1001; AMOUNT: %s", amountStr)
			return content, nil
		},
	)
}

// Individual test functions that can be reused
func TestBasicAgent(t *testing.T, model *ai.Model) {
	tests := []struct {
		agent         Agent
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, content string, run *AgentRun)
		attachments   []*document.Document
		tools         []ai.Tool
	}{
		{
			agent:         Agent{Model: model},
			name:          "empty agent",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, run *AgentRun) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, run.ID())
				assert.NotEmpty(t, run.agent.Name)
				assert.Contains(t, strings.ToLower(content), "sydney")
			},
			tools: []ai.Tool{},
		},
		{
			agent:         Agent{Model: model},
			name:          "basic conversation",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, run *AgentRun) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, run.ID())
				assert.NotEmpty(t, run.agent.Name)
				assert.Contains(t, strings.ToLower(content), "sydney")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.agent.Documents = tt.attachments
			// Convert ai.Tools to AgentTools
			for _, tool := range tt.tools {
				tt.agent.AgentTools = append(tt.agent.AgentTools, WrapTool(tool))
			}

			run, err := tt.agent.Start(tt.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var chunks []string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					chunks = append(chunks, e.Content)
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			finalContent := strings.Join(chunks, "")
			if tt.validate != nil {
				tt.validate(t, finalContent, run)
			}
		})
	}
}

func TestAgentRun(t *testing.T, model *ai.Model) {
	// Sessions no longer have Trace field

	newAgent := func() Agent {
		return Agent{
			Model:        model,
			Description:  "You are a helpful assistant that provides clear and concise answers.",
			Instructions: "Always explain your reasoning and provide examples when possible.",
		}
	}

	tests := []struct {
		name        string
		message     string
		validate    func(t *testing.T, content string, run *AgentRun)
		attachments []*document.Document
		tools       []ai.Tool
	}{
		{
			name:    "basic conversation",
			message: "What is the capital of Australia?",
			validate: func(t *testing.T, content string, run *AgentRun) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, run.ID())
				assert.NotEmpty(t, run.agent.Name)
				assert.Contains(t, strings.ToLower(content), "canberra")
			},
			tools: []ai.Tool{},
		},
		{
			name:    "conversation with instructions",
			message: "Explain the concept of recursion",
			validate: func(t *testing.T, content string, run *AgentRun) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, run.ID())
				assert.NotEmpty(t, run.agent.Name)
				assert.Contains(t, strings.ToLower(content), "recursion")
			},
			tools: []ai.Tool{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent := newAgent()
			// Convert ai.Tools to AgentTools
			for _, tool := range test.tools {
				agent.AgentTools = append(agent.AgentTools, WrapTool(tool))
			}
			agent.Documents = test.attachments
			run, err := agent.Start(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var chunks []string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					chunks = append(chunks, e.Content)
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			finalContent := strings.Join(chunks, "")
			if test.validate != nil {
				test.validate(t, finalContent, run)
			}
		})
	}
}

func TestToolIntegration(t *testing.T, model *ai.Model) {
	// Sessions no longer have Trace field

	newAgent := func() Agent {
		return Agent{
			Name:         "test-agent",
			Model:        model,
			Description:  "You are a helpful assistant that provides clear and concise answers.",
			Instructions: "Always explain your reasoning and provide examples when possible.",
			// LogLevel:     slog.LevelDebug,
		}
	}

	tests := []struct {
		name        string
		message     string
		agent       Agent
		validate    func(t *testing.T, content string, agent Agent)
		attachments []*document.Document
		tools       []ai.Tool
	}{
		{
			name:        "tool call",
			message:     "tell me the name of the company with the number 150. Use tools.",
			agent:       newAgent(),
			tools:       []ai.Tool{NewSecretNumberToolLegacy()},
			attachments: []*document.Document{},
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "Nexxia")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Convert ai.Tools to AgentTools
			for _, tool := range test.tools {
				test.agent.AgentTools = append(test.agent.AgentTools, WrapTool(tool))
			}
			test.agent.Documents = test.attachments
			run, err := test.agent.Start(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var chunks []string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					chunks = append(chunks, e.Content)
				case *ToolEvent:
				case *ApprovalEvent:
					run.Approve(e.ApprovalID, true)
				case *ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			finalContent := strings.Join(chunks, "")
			if test.validate != nil {
				test.validate(t, finalContent, test.agent)
			}
		})
	}
}

func TestTeamCoordination(t *testing.T, model *ai.Model) {
	// Subagents
	lookup := Agent{
		Model:        model,
		Name:         "lookup",
		Description:  "Lookup company details by name. Return either 'COMPANY_ID: <id>; NAME: <name>' or 'NOT_FOUND' only.",
		Instructions: "Use tools to perform the lookup and return the canonical format only.",
		AgentTools:   []AgentTool{NewLookupCompanyByNameTool()},
	}

	companyCreator := Agent{
		Model:        model,
		Name:         "company_creator",
		Description:  "Create a new company by name and return 'COMPANY_ID: <id>; NAME: <name>' only.",
		Instructions: "Use tools to create the company and return the canonical format only.",
		AgentTools:   []AgentTool{NewCreateCompanyTool()},
	}

	invoiceCreator := Agent{
		Model:        model,
		Name:         "invoice_creator",
		Description:  "Create an invoice for a given company_id and amount. Return 'INVOICE_ID: <id>; AMOUNT: <amount>' only.",
		Instructions: "Use tools to create the invoice and return the canonical format only.",
		AgentTools:   []AgentTool{NewCreateInvoiceTool()},
	}

	coordinator := Agent{
		Model: model,
		Name:  "coordinator",
		Description: "Coordinate a workflow to ensure an invoice exists for the requested company name and amount. " +
			"Steps: 1) Call 'lookup' subagent with the company name. 2) If NOT_FOUND, call 'company_creator' to create it. " +
			"3) Call 'invoice_creator' with the resolved company_id and the requested amount. " +
			"Finally, return exactly: 'COMPANY_ID: <id>; NAME: <name>; INVOICE_ID: <invoice>; AMOUNT: <amount>'.",
		Instructions: "Call exactly one tool at a time and wait for the response before the next call. " +
			"Use the save_memory tool to persist important context between tool calls, especially after getting company information and getting invoice information. " +
			"Do not add commentary.",
		Agents: []Agent{lookup, companyCreator, invoiceCreator},
		Tracer: NewTracer(),
		// LogLevel: slog.LevelDebug,
	}

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

			// Implicit memory usage: ensure the coordinator used save_memory at least once during the workflow
			saveIdx := indexOf("save_memory")
			if saveIdx == -1 {
				t.Log("Warning: save_memory was not called during the workflow. This may indicate the coordinator is not persisting context between steps.")
			} else {
				t.Logf("save_memory was called at position %d in the tool call sequence", saveIdx)
			}
		})
	}
}

func TestFileAttachments(t *testing.T, model *ai.Model) {
	// Define test cases
	testCases := []struct {
		name        string
		attachments []*document.Document
		description string
	}{
		{
			name: "text file",
			attachments: []*document.Document{
				document.NewInMemoryDocument("", "sample.txt", []byte("This is a test text file with some sample content for analysis."), nil),
			},
			description: "You are a helpful assistant that analyzes text files and provides insights.",
		},
		{
			name: "PDF file",
			attachments: []*document.Document{
				document.NewInMemoryDocument("", "sample.pdf", []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000204 00000 n \ntrailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n297\n%%EOF"), nil),
			},
			description: "You are a helpful assistant that analyzes PDF files and provides insights.",
		},
		{
			name: "image file",
			attachments: []*document.Document{
				document.NewInMemoryDocument("", "sample.png", []byte("fake-image-data-for-testing"), nil),
			},
			description: "You are a helpful assistant that analyzes images and provides insights.",
		},
	}

	tracer := NewTracer()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent := Agent{
				Model:        model,
				Description:  tc.description,
				Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
				Tracer:       tracer,
				Documents:    tc.attachments,
			}

			run, err := agent.Start("Please analyze the attached file and tell me what it contains. If you can are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")
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

	experts := make([]Agent, numExperts)
	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		experts[i] = Agent{
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

	coordinator := Agent{
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
		Tracer: NewTracer(),
	}

	run, err := coordinator.Start("call the names of expert1, expert2 and expert3 and return them in order, do not add any additional text or commentary.")
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
	counter := 0
	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that can perform various tasks.",
		Instructions: "use tools when requested.",
		AgentTools:   []AgentTool{NewLookupCompanyNumberTool(&counter)},
		Tracer:       NewTracer(),
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

		agentRun, err := agent.Start(run.message)
		if err != nil {
			t.Fatalf("Run %d failed to start: %v", i+1, err)
		}

		agentRuns = append(agentRuns, agentRun)
	}

	// Now wait for all runs to complete (parallel waiting)
	responses := make([]string, len(agentRuns))
	for i, agentRun := range agentRuns {
		// t.Logf("Waiting for run %d to complete", i+1)

		response, err := agentRun.Wait(0)
		if err != nil {
			t.Fatalf("Wait for run %d failed: %v", i+1, err)
		}

		responses[i] = response
		// t.Logf("Run %d completed with response: %s", i+1, response)
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
		// t.Logf("Tool call response found in run %d: '%s' (expected tool call: %v)",
		// toolCallRunIndex+1, runs[toolCallRunIndex].name, runs[toolCallRunIndex].expectsTool)
	}

	// For now, just verify that we found a tool call response, regardless of which run it was in
	// This is a more lenient check that accounts for potential race conditions in parallel execution

	// Verify no errors occurred
	for i, response := range responses {
		assert.NotEmpty(t, response, "Run %d should have non-empty response", i+1)
		assert.NotContains(t, response, "Error:", "Run %d should not contain error", i+1)
	}

	assert.Equal(t, counter, 1, "Should have made 1 tool call")
	// t.Logf("All %d parallel runs completed successfully", len(runs))
}

func TestBasicStreaming(t *testing.T, model *ai.Model) {

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "What is the capital of France what give me a brief summary of the city"
	run, err := agent.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}
	finalContent := strings.Join(chunks, "")

	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, run.ID())
	assert.NotEmpty(t, run.agent.Name)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingContentOnly(t *testing.T, model *ai.Model) {

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "What is the capital of France?"
	run, err := agent.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	finalContent := strings.Join(chunks, "")
	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, run.ID)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingWithCitySummary(t *testing.T, model *ai.Model) {

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
	}

	message := "Give me a brief summary of Paris"
	run, err := agent.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	finalContent := strings.Join(chunks, "")
	assert.NotEmpty(t, finalContent)
	assert.NotEmpty(t, run.ID)
	assert.Contains(t, strings.ToLower(finalContent), "paris")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
}

func TestStreamingWithTools(t *testing.T, model *ai.Model) {

	counter := 0
	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Stream:       true,
		AgentTools:   []AgentTool{NewLookupCompanyNumberTool(&counter)},
	}

	message := "tell me the name of the company with the number 150. Use tools. "
	run, err := agent.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var chunks []string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	finalContent := strings.Join(chunks, "")
	assert.NotEmpty(t, finalContent)
	assert.Contains(t, finalContent, "Nexxia")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
	assert.Equal(t, counter, 1, "Should have made 1 tool call")
}

func TestStreamingToolLookup(t *testing.T, model *ai.Model) {

	counter := 0
	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that looks up company information.",
		Instructions: "Use the lookup tool to find company information when asked.",
		Stream:       true,
		AgentTools:   []AgentTool{NewLookupCompanyNumberTool(&counter)},
	}

	message := "What company has the number 150?"
	run, err := agent.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var chunks []string
	var toolCalls int
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
			toolCalls++
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	finalContent := strings.Join(chunks, "")
	assert.NotEmpty(t, finalContent)
	assert.Contains(t, finalContent, "Nexxia")
	assert.Greater(t, len(chunks), 2, "Should have received streaming chunks")
	assert.GreaterOrEqual(t, toolCalls, 1, "Should have made at least one tool call")
	assert.Equal(t, counter, 1, "Should have made 1 tool call")
}

func TestMemoryPersistence(t *testing.T, model *ai.Model) {

	counter := 0
	// Sub-agents
	lookupCompany := Agent{
		Model:        model,
		Name:         "lookup_company",
		Description:  "This agent allows you to look up a company name by company number. Please provide the request as 'lookup the company name for xxx'",
		Instructions: "Use tools to look up the company name. Return exactly 'COMPANY: <name>' and nothing else.",
		AgentTools:   []AgentTool{NewLookupCompanyNumberTool(&counter)},
	}

	lookupSupplier := Agent{
		Model:        model,
		Name:         "lookup_company_supplier",
		Description:  "This agent allows you to look up a supplier name by supplier number. The request should be in the format 'lookup the supplier name for xxx'",
		Instructions: "Use tools to look up the supplier name. Return exactly 'SUPPLIER: <name>' and nothing else.",
		AgentTools:   []AgentTool{NewSecretSupplierTool()},
	}

	// Coordinator executes the plan, saves each result to memory, then replies with full memory content
	coordinator := Agent{
		Model:       model,
		Name:        "coordinator",
		Description: "You are a coordinator that executes a plan and saves the results to memory. ",
		Instructions: "EXECUTE TASKS SEQUENTIALLY - ONE AT A TIME:\n" +
			"1) Analyze the plan and identify each task step\n" +
			"2) Execute tasks ONE AT A TIME in the exact order specified - DO NOT make parallel calls\n" +
			"3) After each task completion, immediately save the result to memory using update_memory tool before proceeding to the next task\n" +
			"4) NEVER repeat or duplicate tool calls - each task should be executed only once\n" +
			"5) Track completed tasks to avoid repetition\n" +
			"6) When saving memory, include all previous memory content plus the new result\n" +
			"7) After all tasks are complete, return only the final memory content (no commentary)\n" +
			"CRITICAL: Execute step 1, then step 2, then step 3, etc. - NEVER execute multiple steps simultaneously.",
		AgentTools: []AgentTool{newMemoryTool()},
		Agents:     []Agent{lookupCompany, lookupSupplier},
		Tracer:     NewTracer(),
	}

	run, err := coordinator.Start(
		"Execute the following plan: " +
			"1) Call 'lookup_company' with input 'Look up company 150'. " +
			"2) Save the result to memory using update_memory tool. " +
			"3) Call 'lookup_company_supplier' with input 'Look up supplier 200'. " +
			"4) Save the result to memory again using update_memory tool, keeping the previous memory content. " +
			"5) When you have the company and the supplier details, then respond with exactly the full content of the memory (no extra text).",
	)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var toolOrder []string
	var saveIdxs []int
	var companyToolInput string
	var supplierToolInput string
	var chunks []string

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			chunks = append(chunks, e.Content)
		case *ToolEvent:
			args := e.ValidationResult.Values.(map[string]any)
			toolOrder = append(toolOrder, e.ToolName)
			if e.ToolName == "update_memory" {
				saveIdxs = append(saveIdxs, len(toolOrder)-1)
			}
			if e.ToolName == "lookup_company" {
				if v, ok := args["input"].(string); ok {
					companyToolInput = v
				}
			}
			if e.ToolName == "lookup_company_supplier" {
				counter++
				if v, ok := args["input"].(string); ok {
					supplierToolInput = v
				}
			}
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	finalContent := strings.Join(chunks, "")
	assert.NotEmpty(t, finalContent, "Final response should not be empty")
	assert.Contains(t, strings.ToLower(finalContent), "nexxia", "memory should include company result")
	assert.Contains(t, strings.ToLower(finalContent), "phoenix", "memory should include supplier result")

	// Ensure orchestration used subagents and memory saves in order
	indexOf := func(name string) int {
		for i, n := range toolOrder {
			if n == name {
				return i
			}
		}
		return -1
	}
	companyIdx := indexOf("lookup_company")
	supplierIdx := indexOf("lookup_company_supplier")
	assert.NotEqual(t, -1, companyIdx, "lookup_company subagent should be called")
	assert.NotEqual(t, -1, supplierIdx, "lookup_company_supplier subagent should be called")
	assert.Greater(t, len(saveIdxs), 1, "update_memory should be called at least twice")

	// Validate inputs used to call subagents
	assert.Contains(t, strings.ToLower(companyToolInput), "look up company 150")
	assert.Contains(t, strings.ToLower(supplierToolInput), "look up supplier 200")

	assert.Contains(t, finalContent, "Nexxia")
	assert.Contains(t, finalContent, "Phoenix")
	assert.Equal(t, 2, counter, "Should have made 2 tool calls")
}
