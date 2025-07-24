package aigentic

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/utils"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.LoadEnvFile("./.env")
}

// NewMagicNumberTool returns a SimpleTool struct for testing
func NewMagicNumberTool() ai.Tool {
	return ai.Tool{
		Name:        "magic_number",
		Description: "A tool that generates a magic number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "The input seed to randomize the number",
				},
			},
			"required": []string{"input"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "150"}},
				Error:   false,
			}, nil
		},
	}
}

func TestAgent_Basic(t *testing.T) {
	model := ai.NewOllamaModel("qwen3:1.7b", "")
	session := NewSession()
	session.Trace = NewTrace()

	tests := []struct {
		agent         Agent
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, content string, agent Agent)
		attachments   []Attachment
		tools         []ai.Tool
	}{
		{
			agent:         Agent{}, // there won't be any tracing in this case
			name:          "empty agent",
			message:       "What is the capital of Australia?",
			expectedError: false,
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, content, "Canberra")
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
				assert.Contains(t, content, "Sydney")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		fmt.Printf("Running test: %s\n", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			tt.agent.Attachments = tt.attachments
			tt.agent.Tools = tt.tools

			tt.agent.Run(tt.message)
			var finalContent string
			for ev := range tt.agent.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *ToolEvent:
					if e.RequireApproval {
						tt.agent.Approve(e.ID())
					}
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

func TestAgent_Run(t *testing.T) {
	model := ai.NewOllamaModel("qwen3:1.7b", "")

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
		attachments []Attachment
		tools       []ai.Tool
	}{
		{
			name:    "basic conversation",
			message: "What is the capital of Australia?",
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, content, "Canberra")
			},
			tools: []ai.Tool{},
		},
		{
			name:    "conversation with instructions",
			message: "Explain the concept of recursion",
			validate: func(t *testing.T, content string, agent Agent) {
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
			agent.Run(test.message)
			var finalContent string
			for ev := range agent.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *ToolEvent:
					if e.RequireApproval {
						agent.Approve(e.ID())
					}
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

func TestAgent_Run_WithTools(t *testing.T) {
	session := NewSession()
	session.Trace = NewTrace()

	newAgent := func(model *ai.Model) Agent {
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
		agent       Agent
		validate    func(t *testing.T, content string, agent Agent)
		attachments []Attachment
		tools       []ai.Tool
	}{
		{
			name:        "Ollama tool call",
			message:     "Generate a magic number and tell me the number. Use tools.",
			agent:       newAgent(ai.NewOllamaModel("qwen3:1.7b", "")),
			tools:       []ai.Tool{NewMagicNumberTool()},
			attachments: []Attachment{},
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "150")
			},
		},
		{
			name:        "OpenAI tool call",
			message:     "Generate a magic number and tell me the number. Use tools.",
			agent:       newAgent(ai.NewOpenAIModel("gpt-4o-mini", "")),
			tools:       []ai.Tool{NewMagicNumberTool()},
			attachments: []Attachment{},
			validate: func(t *testing.T, content string, agent Agent) {
				assert.NotEmpty(t, content)
				assert.Contains(t, content, "150")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.agent.Tools = test.tools
			test.agent.Attachments = test.attachments
			test.agent.Run(test.message)
			var finalContent string
			for ev := range test.agent.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					if e.IsFinal {
						finalContent = e.Content
					}
				case *ToolEvent:
					if e.RequireApproval {
						test.agent.Approve(e.ID())
					}
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

func TestTeam(t *testing.T) {
	// set log level to debug
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	model := ai.NewOllamaModel("qwen3:1.7b", "")

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
		// Model:        ai.NewOllamaModel("qwen3:14b", ""),
		Model: ai.NewOpenAIModel("gpt-4o-mini", ""),
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
		Trace:  NewTrace(),
		Agents: []*Agent{&calculator, &explainer},
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
			team.Run(test.message)
			var finalContent string
			for ev := range team.Next() {
				switch e := ev.(type) {
				case *ContentEvent:
					fmt.Printf("ContentEvent: %+v\n\n", e)
					if e.IsFinal {
						finalContent = e.Content
					}
				case *ToolEvent:
					fmt.Printf("ToolEvent: %+v\n\n", e)
					if e.RequireApproval {
						team.Approve(e.ID())
					}
				case *ErrorEvent:
					fmt.Printf("ErrorEvent: %+v\n\n", e)
					t.Fatalf("Agent error: %v", e.Err)
				case *ThinkingEvent:
					fmt.Printf("ThinkingEvent: %+v\n\n", e)
				case *LLMCallEvent:
					fmt.Printf("LLMCallEvent: %+v\n\n", e)
				default:
					fmt.Printf("Event: %+v (Type: %T)\n", e, e)
				}
			}
			if test.validate != nil {
				test.validate(t, finalContent)
			}
		})
	}
}

// TestCreateUserMsg tests the new createUserMsg2 function
func TestCreateUserMsg(t *testing.T) {
	agent := Agent{
		Attachments: []Attachment{
			{
				Type:     "file",
				Content:  []byte("test content"),
				MimeType: "text/plain",
				Name:     "file-abc123",
			},
			{
				Type:     "image",
				Content:  []byte("image data"),
				MimeType: "image/png",
				Name:     "test.png",
			},
		},
	}

	// Test with message and attachments (no FileID)
	messages := agent.createUserMsg("Hello, please analyze these files")

	assert.Len(t, messages, 3) // 1 main message + 2 attachments

	// Check main message
	mainMsg, ok := messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "Hello, please analyze these files", mainMsg.Content)

	// Check first attachment message (should include content)
	att1Msg, ok := messages[1].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att1Msg.Name, "file-abc123")
	assert.Contains(t, string(att1Msg.Body.([]byte)), "test content")

	// Check second attachment message (should include content)
	att2Msg, ok := messages[2].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att2Msg.Name, "test.png")
	assert.Contains(t, string(att2Msg.Body.([]byte)), "image data")

}

// TestAgent_Run_WithFileID tests the agent with OpenAI Files API integration
func TestAgent_Run_WithFileID(t *testing.T) {
	// Skip if no OpenAI API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	// model := ai.NewOpenAIModel("gpt-4o", "")
	model := ai.NewOpenAIModel("o4-mini", "")
	// model := ai.NewOpenAIModel("gpt-4.1", "")
	// model := ai.NewOllkamaModel("qwen3:1.7b", "")

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		Trace:        NewTrace(),
		Attachments: []Attachment{
			{
				Type: "file",
				// Content:  []byte("This is test content for the file."),
				MimeType: "application/pdf",
				Name:     "file-WjBr55R67mVmhXCsvKZ6Zs",
			},
		},
	}

	// Test the agent with file ID
	response, err := agent.RunAndWait("Please analyze the attached file and tell me what it contains. If you can access it, start your response with 'SUCCESS:' followed by the analysis.")

	if err != nil {
		t.Logf("Agent run completed with error: %v", err)
		// Even if there's an error, we should get some response
		assert.NotEmpty(t, response)
	} else {
		assert.NoError(t, err)
		assert.NotEmpty(t, response)

		// The response should mention the file reference
		assert.Contains(t, response, "file-Rro2oxubCRkrbpWsdSypWL")

		// Log the response for debugging
		t.Logf("Agent response: %s", response)
	}
}

func TestAgent_Run_Attachments(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	// Define test cases
	testCases := []struct {
		name        string
		model       *ai.Model
		attachments []Attachment
		description string
	}{
		{
			name:  "GPT-4o-mini with text file",
			model: ai.NewOpenAIModel("gpt-4o-mini", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o-mini", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o-mini-2024-07-18", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o-mini", ""),
			attachments: []Attachment{
				{
					Type: "file",
					Name: "file-Rro2oxubCRkrbpWsdSypWL",
				},
			},
			description: "You are a helpful assistant that analyzes files using file IDs and provides insights.",
		},
		{
			name:  "GPT-4o with text file",
			model: ai.NewOpenAIModel("gpt-4o", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o", ""),
			attachments: []Attachment{
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
			model: ai.NewOpenAIModel("gpt-4o", ""),
			attachments: []Attachment{
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
			model: ai.NewOllamaModel("qwen2.5:7b", ""),
			attachments: []Attachment{
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
			model: ai.NewOllamaModel("qwen3:1.7b", ""),
			attachments: []Attachment{
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
			model: ai.NewOllamaModel("qwen3:1.7b", ""),
			attachments: []Attachment{
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

	tracer := NewTrace()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agent := Agent{
				Model:        tc.model,
				Description:  tc.description,
				Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
				Trace:        tracer,
				Attachments:  tc.attachments,
			}

			// Test the agent with attachments
			err := agent.Run("Please analyze the attached file and tell me what it contains. If you can are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")
			if err != nil {
				t.Fatalf("Agent run failed: %v", err)
			}
			response, err := agent.Wait(10 * time.Second)
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
