package aigentic

import (
	"strings"
	"testing"

	"github.com/irai/rag/ai"
	"github.com/irai/rag/loader"
	"github.com/stretchr/testify/assert"
)

func init() {
	loader.LoadEnvFile("../.env")
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
	loader.LoadEnvFile("../.env")
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
	loader.LoadEnvFile("../.env")
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
	loader.LoadEnvFile("../.env")
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
