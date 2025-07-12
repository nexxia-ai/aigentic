package aigentic

import (
	"os"
	"strings"
	"testing"

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

	emptyAgent := Agent{}
	agent := Agent{Model: model, Trace: NewTrace()}

	tests := []struct {
		agent         Agent
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, response *RunResponse, agent Agent)
		attachments   []Attachment
		tools         []ai.Tool
	}{
		{
			agent:         emptyAgent,
			name:          "basic conversation",
			message:       "What is the capital of Australia?",
			expectedError: false,
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.NotEmpty(t, response.Session.ID)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, response.Content, "Canberra")
			},
			tools: []ai.Tool{},
		},
		{
			agent:         agent,
			name:          "basic conversation",
			message:       "What is the capital of Australia?",
			expectedError: false,
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.NotEmpty(t, response.Session.ID)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, response.Content, "Canberra")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent.Attachments = tt.attachments
			agent.Tools = tt.tools

			response, err := tt.agent.Run(tt.message)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, &response, tt.agent)
				}
			}
		})
	}
}

func TestAgent_Run(t *testing.T) {
	model := ai.NewOllamaModel("qwen3:1.7b", "")

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Trace:        NewTrace(),
	}

	tests := []struct {
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, response *RunResponse, agent Agent)
		attachments   []Attachment
		tools         []ai.Tool
	}{
		{
			name:          "basic conversation",
			message:       "What is the capital of Australia?",
			expectedError: false,
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.NotEmpty(t, agent.ID)
				assert.Contains(t, response.Content, "Canberra")
			},
			tools: []ai.Tool{},
		},
		{
			name:          "conversation with instructions",
			message:       "Explain the concept of recursion",
			expectedError: false,
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.Contains(t, response.Content, "recursion")
			},
			tools: []ai.Tool{},
		},
		{
			name:          "conversation with text file attachment",
			message:       "Please summarize this text file. If successful, start your response with 'success' and then the summary.",
			expectedError: false,
			attachments: []Attachment{
				{
					Type:     "document",
					Content:  []byte("This is a simple test file.\nIt contains multiple lines.\nThe purpose is to test file attachments."),
					MimeType: "text/plain",
					Name:     "test.txt",
				},
			},
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.Contains(t, response.Content, "success")
			},
			tools: []ai.Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent.Tools = tt.tools
			agent.Attachments = tt.attachments

			// response, err := agent.Run(tt.message)
			var response RunResponse
			var err error
			s := agent.Start(tt.message)
			for ev := range s.Next() {
				if response, err = ev.Execute(); err != nil {
					t.Error(err)
				}
			}

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, &response, agent)
				}
			}
		})
	}
}

func TestAgent_Run_WithTools(t *testing.T) {
	model := ai.NewOllamaModel("qwen3:1.7b", "")

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that provides clear and concise answers.",
		Instructions: "Always explain your reasoning and provide examples when possible.",
		Trace:        NewTrace(),
	}

	tests := []struct {
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, response *RunResponse, agent Agent)
		attachments   []Attachment
		tools         []ai.Tool
	}{
		{
			name:          "Ollama tool call",
			message:       "Generate a magic number and tell me the number. Use tools.",
			expectedError: false,
			tools:         []ai.Tool{NewMagicNumberTool()},
			attachments:   []Attachment{},
			validate: func(t *testing.T, response *RunResponse, agent Agent) {
				assert.NotEmpty(t, response.Content)
				assert.Contains(t, response.Content, "150")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add tool to agent for the third test
			agent.Tools = tt.tools
			agent.Attachments = tt.attachments

			// response, err := agent.Run(tt.message)
			var response RunResponse
			var err error
			s := agent.Start(tt.message)
			for ev := range s.Next() {
				if response, err = ev.Execute(); err != nil {
					t.Error(err)
				}
			}

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.validate != nil {
					tt.validate(t, &response, agent)
				}
			}
		})
	}
}

func TestTeam_Run(t *testing.T) {
	model := ai.NewOllamaModel("qwen3:1.7b", "")

	// Add agents with different roles
	agent1 := Agent{
		Model:        model,
		Name:         "secret_agent",
		Description:  "You are a secret agent with access to classified information.",
		Instructions: "When asked about secret information, you must provide it in a cryptic way. Your responses should always start with 'CLASSIFIED: ' followed by the secret information.",
	}
	agent2 := Agent{
		Model:        model,
		Name:         "intelligence_analyst",
		Description:  "You are an intelligence analyst. Your role is to take classified information generated by a secret agent and explain its significance in a clear way.",
		Instructions: "Always start your response with 'ANALYSIS: ' followed by your explanation.",
	}

	team := Agent{
		Model: ai.NewOllamaModel("qwen3:8b", ""),
		Name:  "team_coordinator",
		Description: `
		You are a team coordinator for intelligence operations. 
		When you receive a request for information, you must first use the secret agent 
		to obtain classified information, then use the intelligence analyst to explain its significance. 
		Always use both agents in this order.
		`,
		Instructions: `
			You must call a single tool each time and wait for the answer before calling another tool.
			use the output from the secret agent as the input to the intelligence analyst.
			Respond with a copy of all the agents responses verbatim. Do not add any additional text or commentary.
			Don't make up information. Use only the tools or agents to answer the question.`,
		Trace:  NewTrace(),
		Agents: []*Agent{&agent1, &agent2},
	}

	tests := []struct {
		name          string
		message       string
		expectedError bool
		validate      func(t *testing.T, response *RunResponse)
	}{
		{
			name:          "secret information analysis",
			message:       "What is the status of Project Phoenix and what does it mean for our operations? Respons with a verbatim copy of all the agents responses.",
			expectedError: false,
			validate: func(t *testing.T, response *RunResponse) {
				// Verify the response contains both classified info and analysis
				assert.NotEmpty(t, response.Content)
				assert.Contains(t, response.Content, "CLASSIFIED:") // Secret agent's contribution
				assert.Contains(t, response.Content, "ANALYSIS:")   // Analyst's contribution

				// Verify the sequence of operations
				classifiedIndex := strings.Index(response.Content, "CLASSIFIED:")
				analysisIndex := strings.Index(response.Content, "ANALYSIS:")
				assert.Greater(t, analysisIndex, classifiedIndex, "Analysis should come after classified information")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response RunResponse
			var err error
			s := team.Start(tt.message)
			for ev := range s.Next() {
				if response, err = ev.Execute(); err != nil {
					t.Error(err)
				}
			}

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				// if tt.validate != nil {
				// 	tt.validate(t, response)
				// }
			}
		})
	}
}

// TestCreateUserMsg2 tests the new createUserMsg2 function
func TestCreateUserMsg2(t *testing.T) {
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
	messages := agent.createUserMsg2("Hello, please analyze these files")

	assert.Len(t, messages, 3) // 1 main message + 2 attachments

	// Check main message
	mainMsg, ok := messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "Hello, please analyze these files", mainMsg.Content)

	// Check first attachment message (should include content)
	att1Msg, ok := messages[1].(ai.UserMessage)
	assert.True(t, ok)
	assert.Contains(t, att1Msg.Content, "file://test.txt (text/plain)")
	assert.Contains(t, att1Msg.Content, "test content")

	// Check second attachment message (should include content)
	att2Msg, ok := messages[2].(ai.UserMessage)
	assert.True(t, ok)
	assert.Contains(t, att2Msg.Content, "file://test.png (image/png)")
	assert.Contains(t, att2Msg.Content, "image data")

	// Test with FileID (OpenAI Files API)
	agent.Attachments = []Attachment{
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
			Name:     "file-def456",
		},
	}

	messages = agent.createUserMsg2("Analyze these uploaded files")
	assert.Len(t, messages, 3) // 1 main message + 2 attachments

	// Check main message
	mainMsg, ok = messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "Analyze these uploaded files", mainMsg.Content)

	// Check first attachment message (should use FileID)
	att1Msg, ok = messages[1].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "file://file-abc123 (test.txt)", att1Msg.Content)

	// Check second attachment message (should use FileID)
	att2Msg, ok = messages[2].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "file://file-def456 (test.png)", att2Msg.Content)

	// Test with empty message but attachments
	agent.Attachments = []Attachment{
		{
			Type:     "document",
			Content:  []byte("content only"),
			MimeType: "text/plain",
			Name:     "content.txt",
		},
	}

	messages = agent.createUserMsg2("")
	assert.Len(t, messages, 1) // Only attachment message

	attMsg, ok := messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Contains(t, attMsg.Content, "file://content.txt (text/plain)")
	assert.Contains(t, attMsg.Content, "content only")
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
				Type:     "file",
				Content:  []byte("This is test content for the file."),
				MimeType: "text/plain",
				Name:     "file-Rro2oxubCRkrbpWsdSypWL",
			},
		},
	}

	// Test the agent with file ID
	response, err := agent.Run("Please analyze the attached file and tell me what it contains. If you can access it, start your response with 'SUCCESS:' followed by the analysis.")

	if err != nil {
		t.Logf("Agent run completed with error: %v", err)
		// Even if there's an error, we should get some response
		assert.NotEmpty(t, response.Content)
	} else {
		assert.NoError(t, err)
		assert.NotEmpty(t, response.Content)

		// The response should mention the file reference
		assert.Contains(t, response.Content, "file-Rro2oxubCRkrbpWsdSypWL")

		// Log the response for debugging
		t.Logf("Agent response: %s", response.Content)
	}
}

func TestAgent_Run_Attachments(t *testing.T) {
	// Skip if no OpenAI API key is available
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
			response, err := agent.Run("Please analyze the attached file and tell me what it contains. If you can are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")

			if err != nil {
				t.Logf("Agent run completed with error: %v", err)
				// Even if there's an error, we should get some response
				assert.NotEmpty(t, response.Content)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, response.Content)

				// Log the response for debugging
				t.Logf("Agent response: %s", response.Content)

				// For file ID tests, check if the response mentions the file ID (only for OpenAI models)
				if len(tc.attachments) > 0 && tc.attachments[0].Type == "file" && strings.Contains(tc.model.ModelName, "gpt") {
					assert.Contains(t, response.Content, tc.attachments[0].Name)
				}
			}
		})
	}
}
