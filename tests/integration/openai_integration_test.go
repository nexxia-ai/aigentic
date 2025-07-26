//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/nexxia-ai/aigentic"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

func TestOpenAIIntegration(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return openai.NewModel("gpt-4o-mini", "")
		},
		Name:      "OpenAI",
		SkipTests: []string{}, // OpenAI supports all test types
	})
}

// TestAgent_Run_WithFileID tests the agent with OpenAI Files API integration
func TestAgent_Run_WithFileID(t *testing.T) {
	// Skip if no OpenAI API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	model := openai.NewModel("o4-mini", "")

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		Trace:        aigentic.NewTrace(),
		Attachments: []aigentic.Attachment{
			{
				Type:     "file",
				MimeType: "application/pdf",
				Name:     "file-WjBr55R67mVmhXCsvKZ6Zs",
			},
		},
	}

	// Test the agent with file ID
	_, err := agent.RunAndWait("Please analyze the attached file and tell me what it contains. If you can access it, start your response with 'SUCCESS:' followed by the analysis.")
	assert.NoError(t, err)
}
