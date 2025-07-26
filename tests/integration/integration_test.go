//go:build integration

// To run integration tests, run this:
// go test -v -tags=integration ./tests/integration/

package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic"
	ollama "github.com/nexxia-ai/aigentic-ollama"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/utils"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.LoadEnvFile("../../.env")
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

func TestAgent_Basic(t *testing.T) {
	model := ollama.NewModel("qwen3:1.7b", "")
	session := aigentic.NewSession()
	session.Trace = aigentic.NewTrace()

	tests := []struct {
		agent         aigentic.Agent
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, content string, agent aigentic.Agent)
		attachments   []aigentic.Attachment
		tools         []ai.Tool
	}{
		{
			agent:         aigentic.Agent{Model: model}, // there won't be any tracing in this case
			name:          "empty agent",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, content, "sydney")
			},
			tools: []ai.Tool{},
		},
		{
			agent:         aigentic.Agent{Session: session, Model: model},
			name:          "basic conversation",
			message:       "What is the capital of New South Wales, Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, content, "sydney")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		fmt.Printf("Running test: %s\n", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			tt.agent.Attachments = tt.attachments
			tt.agent.Tools = tt.tools

			run, err := tt.agent.Run(tt.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *aigentic.ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *aigentic.ToolEvent:
					if e.RequireApproval {
						run.Approve(e.ID())
					}
				case *aigentic.ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if tt.validate != nil {
				tt.validate(t, finalContent, tt.agent)
			}
		})
	}
}

func TestAgent_Run(t *testing.T) {
	model := ollama.NewModel("qwen3:1.7b", "")

	session := aigentic.NewSession()
	session.Trace = aigentic.NewTrace()

	newAgent := func() aigentic.Agent {
		return aigentic.Agent{
			Session:      session,
			Model:        model,
			Description:  "You are a helpful assistant that provides clear and concise answers.",
			Instructions: "Always explain your reasoning and provide examples when possible.",
		}
	}

	tests := []struct {
		name        string
		message     string
		validate    func(t *testing.T, content string, agent aigentic.Agent)
		attachments []aigentic.Attachment
		tools       []ai.Tool
	}{
		{
			name:    "basic conversation",
			message: "What is the capital of Australia?",
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, content, "Canberra")
			},
			tools: []ai.Tool{},
		},
		{
			name:    "conversation with instructions",
			message: "Explain the concept of recursion",
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "recursion")
			},
			tools: []ai.Tool{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent := newAgent()
			agent.Tools = test.tools
			agent.Attachments = test.attachments
			run, err := agent.Run(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *aigentic.ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *aigentic.ToolEvent:
					if e.RequireApproval {
						run.Approve(e.ID())
					}
				case *aigentic.ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent, agent)
			}
		})
	}
}

func TestAgent_Run_WithTools(t *testing.T) {
	session := aigentic.NewSession()
	session.Trace = aigentic.NewTrace()

	newAgent := func(model *ai.Model) aigentic.Agent {
		return aigentic.Agent{
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
		agent       aigentic.Agent
		validate    func(t *testing.T, content string, agent aigentic.Agent)
		attachments []aigentic.Attachment
		tools       []ai.Tool
	}{
		{
			name:        "Ollama tool call",
			message:     "tell me the name of the company with the number 150. Use tools.",
			agent:       newAgent(ollama.NewModel("qwen3:1.7b", "")),
			tools:       []ai.Tool{NewSecretNumberTool()},
			attachments: []aigentic.Attachment{},
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "Nexxia")
			},
		},
		{
			name:        "OpenAI tool call",
			message:     "tell me the name of the company with the number 150. Use tools.",
			agent:       newAgent(openai.NewModel("gpt-4o-mini", "")),
			tools:       []ai.Tool{NewSecretNumberTool()},
			attachments: []aigentic.Attachment{},
			validate: func(t *testing.T, content string, agent aigentic.Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "Nexxia")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.agent.Tools = test.tools
			test.agent.Attachments = test.attachments
			run, err := test.agent.Run(test.message)
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			var finalContent string
			for ev := range run.Next() {
				switch e := ev.(type) {
				case *aigentic.ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *aigentic.ToolEvent:
					if e.RequireApproval {
						run.Approve(e.ID())
					}
				case *aigentic.ErrorEvent:
					t.Fatalf("Agent error: %v", e.Err)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent, test.agent)
			}
		})
	}
}

func TestTeam(t *testing.T) {

	model := ollama.NewModel("qwen3:1.7b", "")

	// Add agents with different roles
	calculator := aigentic.Agent{
		Model:        model,
		Name:         "calculator",
		Description:  "You are a calculator. When given a math problem, solve it and return only the numerical result.",
		Instructions: "Solve the math problem and return only the number. Do not add any explanation or text.",
	}
	explainer := aigentic.Agent{
		Model:        model,
		Name:         "explainer",
		Description:  "You are a math teacher. When given a calculation, explain what it means in simple terms in terms of the office oranges that you have.",
		Instructions: "Explain the calculation in simple terms. Start your response with 'EXPLANATION: ' followed by your explanation.",
	}

	team := aigentic.Agent{
		Model: ollama.NewModel("qwen3:14b", ""),
		// Model: ai.ollama("gpt-4o-mini", ""),
		Name: "coordinator",
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
		Trace:    aigentic.NewTrace(),
		LogLevel: slog.LevelDebug,
		Agents:   []*aigentic.Agent{&calculator, &explainer},
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
				case *aigentic.ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *aigentic.ToolEvent:
					if e.RequireApproval {
						run.Approve(e.ID())
					}
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent)
			}
		})
	}
}

// TestAgent_Run_WithFileID tests the agent with OpenAI Files API integration
func TestAgent_Run_WithFileID(t *testing.T) {
	// Skip if no OpenAI API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	// model := openai.NewOpenAIModel("gpt-4o", "")
	model := openai.NewModel("o4-mini", "")
	// model := openai.NewOpenAIModel("gpt-4.1", "")

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		Trace:        aigentic.NewTrace(),
		Attachments: []aigentic.Attachment{
			{
				Type: "file",
				// Content:  []byte("This is test content for the file."),
				MimeType: "application/pdf",
				Name:     "file-WjBr55R67mVmhXCsvKZ6Zs",
			},
		},
	}

	// Test the agent with file ID
	_, err := agent.RunAndWait("Please analyze the attached file and tell me what it contains. If you can access it, start your response with 'SUCCESS:' followed by the analysis.")
	assert.NoError(t, err)
}

func TestAgent_Run_Attachments(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	// Define test cases
	testCases := []struct {
		name        string
		model       *ai.Model
		attachments []aigentic.Attachment
		description string
	}{
		{
			name:  "GPT-4o-mini with text file",
			model: openai.NewModel("gpt-4o-mini", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "text",
					Content:  []byte("This is a test text file with some sample content for analysis."),
					MimeType: "text/plain",
					Name:     "sample.txt",
				},
			},
			description: "You are a helpful assistant that analyzes text files and provides insights.",
		},
		{
			name:  "GPT-4o-mini with PDF file",
			model: openai.NewModel("gpt-4o-mini", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "pdf",
					Content:  []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000204 00000 n \ntrailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n297\n%%EOF"),
					MimeType: "application/pdf",
					Name:     "sample.pdf",
				},
			},
			description: "You are a helpful assistant that analyzes PDF files and provides insights.",
		},
		{
			name:  "GPT-4o-mini with image file",
			model: openai.NewModel("gpt-4o-mini-2024-07-18", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "image",
					Content:  []byte("fake-image-data-for-testing"),
					MimeType: "image/png",
					Name:     "sample.png",
				},
			},
			description: "You are a helpful assistant that analyzes images and provides insights.",
		},
		{
			name:  "GPT-4o-mini with file ID",
			model: openai.NewModel("gpt-4o-mini", ""),
			attachments: []aigentic.Attachment{
				{
					Type: "file",
					Name: "file-Rro2oxubCRkrbpWsdSypWL",
				},
			},
			description: "You are a helpful assistant that analyzes files using file IDs and provides insights.",
		},
		{
			name:  "GPT-4o with text file",
			model: openai.NewModel("gpt-4o", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "text",
					Content:  []byte("This is a test text file with some sample content for analysis."),
					MimeType: "text/plain",
					Name:     "sample.txt",
				},
			},
			description: "You are a helpful assistant that analyzes text files and provides insights.",
		},
		{
			name:  "GPT-4o with PDF file",
			model: openai.NewModel("gpt-4o", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "pdf",
					Content:  []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000204 00000 n \ntrailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n297\n%%EOF"),
					MimeType: "application/pdf",
					Name:     "sample.pdf",
				},
			},
			description: "You are a helpful assistant that analyzes PDF files and provides insights.",
		},
		{
			name:  "GPT-4o with image file",
			model: openai.NewModel("gpt-4o", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "image",
					Content:  []byte("fake-image-data-for-testing"),
					MimeType: "image/png",
					Name:     "sample.png",
				},
			},
			description: "You are a helpful assistant that analyzes images and provides insights.",
		},
		{
			name:  "GPT-4o with file ID",
			model: openai.NewModel("gpt-4o", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "file",
					Content:  []byte("This is test content for the file."),
					MimeType: "text/plain",
					Name:     "file-Rro2oxubCRkrbpWsdSypWL",
				},
			},
			description: "You are a helpful assistant that analyzes files using file IDs and provides insights.",
		},
		{
			name:  "Qwen with text file",
			model: ollama.NewModel("qwen2.5:7b", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "text",
					Content:  []byte("This is a test text file with some sample content for analysis."),
					MimeType: "text/plain",
					Name:     "sample.txt",
				},
			},
			description: "You are a helpful assistant that analyzes text files and provides insights.",
		},
		{
			name:  "Qwen with PDF file",
			model: ollama.NewModel("qwen3:1.7b", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "pdf",
					Content:  []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Contents 4 0 R\n>>\nendobj\n4 0 obj\n<<\n/Length 44\n>>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Test PDF Content) Tj\nET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000204 00000 n \ntrailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n297\n%%EOF"),
					MimeType: "application/pdf",
					Name:     "sample.pdf",
				},
			},
			description: "You are a helpful assistant that analyzes PDF files and provides insights.",
		},
		{
			name:  "Qwen with image file",
			model: ollama.NewModel("qwen3:1.7b", ""),
			attachments: []aigentic.Attachment{
				{
					Type:     "image",
					Content:  []byte("fake-image-data-for-testing"),
					MimeType: "image/png",
					Name:     "sample.png",
				},
			},
			description: "You are a helpful assistant that analyzes images and provides insights.",
		},
	}

	tracer := aigentic.NewTrace()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent := aigentic.Agent{
				Model:        tc.model,
				Description:  tc.description,
				Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
				Trace:        tracer,
				Attachments:  tc.attachments,
			}

			// Test the agent with attachments
			run, err := agent.Run("Please analyze the attached file and tell me what it contains. If you can are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			response, err := run.Wait(10 * time.Second)
			if err != nil {
				t.Fatalf("Agent wait failed: %v", err)
			}

			if err != nil {
				t.Logf("Agent run completed with error: %v", err)
				// Even if there's an error, we should get some response
				assert.NotEmpty(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, response)

				// Log the response for debugging
				t.Logf("Agent response: %s", response)

				// For file ID tests, check if the response mentions the file ID (only for OpenAI models)
				if len(tc.attachments) > 0 && tc.attachments[0].Type == "file" && strings.Contains(tc.model.ModelName, "gpt") {
					assert.Contains(t, response, tc.attachments[0].Name)
				}
			}
		})
	}
}

func TestMultiAgent_ChainExperts(t *testing.T) {
	const numExperts = 3
	input := "start"

	experts := make([]*aigentic.Agent, numExperts)
	for i := 0; i < numExperts; i++ {
		expertName := fmt.Sprintf("expert%d", i+1)
		experts[i] = &aigentic.Agent{
			Name:        expertName,
			Description: "You are an expert in a group of experts. Your role is to sign the input with your name by appending your name at the end of the input.",
			Instructions: `
			Remember:
			return your name signed at the end of the input
			do not change the input text
			do not add any additional information` +
				fmt.Sprintf("Your name is %s. Append your name to the input and return that as the output.", expertName),
			Model: ollama.NewModel("qwen3:1.7b", ""),
			Tools: nil,
		}
	}

	coordinator := aigentic.Agent{
		Name:        "coordinator",
		Description: "You are the coordinator to collect signature from experts. Your role is to call each expert one by one in order, passing the previous signature to the next expert so he/she can sign the input. Return all the signatures as received by the experts.",
		Instructions: `
		Call each expert one by one in order, passing the previous expert's signature. 
		You must call all the experts in order.
		Return the final signatures as received from the last expert. do not add any additional text or commentary.`,
		Model:  openai.NewModel("gpt-4o-mini", ""),
		Agents: experts,
		Trace:  aigentic.NewTrace(),
	}

	run, err := coordinator.Run("call the first expert with the input: " + input)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}
	response, err := run.Wait(0)
	if err != nil {
		t.Fatalf("Agent wait failed: %v", err)
	}

	assert.Equal(t, "start expert1 expert2 expert3", strings.TrimSpace(response))
}

func TestAgent_MultipleSequentialRuns(t *testing.T) {
	agent := aigentic.Agent{
		Model:        ollama.NewModel("qwen3:1.7b", ""),
		Description:  "You are a helpful assistant that can perform various tasks.",
		Instructions: "use tools when requested.",
		Tools:        []ai.Tool{NewSecretNumberTool()},
		Trace:        aigentic.NewTrace(),
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
	var agentRuns []*aigentic.AgentRun
	for i, run := range runs {
		t.Logf("Starting run %d: %s", i+1, run.name)

		agentRun, err := agent.Run(run.message)
		if err != nil {
			t.Fatalf("Run %d failed to start: %v", i+1, err)
		}

		agentRuns = append(agentRuns, agentRun)
	}

	// Now wait for all runs to complete (parallel waiting)
	var responses []string
	for i, agentRun := range agentRuns {
		t.Logf("Waiting for run %d to complete", i+1)

		response, err := agentRun.Wait(0)
		if err != nil {
			t.Fatalf("Wait for run %d failed: %v", i+1, err)
		}

		responses = append(responses, response)
		t.Logf("Run %d completed with response: %s", i+1, response)
	}

	// Verify all responses
	assert.Len(t, responses, len(runs), "Should have responses for all runs")

	// Check that tool calls were made when expected
	for i, run := range runs {
		if run.expectsTool {
			assert.Contains(t, responses[i], "Nexxia", "Run %d should contain the company name", i+1)
		}
	}

	// Verify no errors occurred
	for i, response := range responses {
		assert.NotEmpty(t, response, "Run %d should have non-empty response", i+1)
		assert.NotContains(t, response, "Error:", "Run %d should not contain error", i+1)
	}

	t.Logf("All %d parallel runs completed successfully", len(runs))
}
