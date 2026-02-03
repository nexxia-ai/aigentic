//go:build integration

// run this with: go test -v -tags=integration -run ^TestOpenAI_AgentSuite

package aigentic

import (
	"os"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ai/openai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/utils"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.LoadEnvFile("../.env")
}

func TestOpenAI_AgentSuite(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return openai.NewModel("gpt-5-mini", os.Getenv("OPENAI_API_KEY"))
		},
		Name: "OpenAI",
		SkipTests: []string{
			"TeamCoordination",
			"FileAttachments",
		},
	})
}

func TestOpenAI_OpenRouter(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return openai.NewModel("qwen/qwen3-30b-a3b-instruct-2507", os.Getenv("OPENROUTER_API_KEY"), openai.OpenRouterBaseURL)
		},
		Name: "OpenRouter",
		SkipTests: []string{
			"TeamCoordination",
		},
	})
}
func TestOpenAI_Helicone(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return openai.NewModel("gpt-5-mini", os.Getenv("HELICONE_API_KEY"), openai.HeliconeBaseURL)
		},
		Name: "OpenRouter",
		SkipTests: []string{
			"TeamCoordination",
		},
	})
}

func TestOpenAI_BasicAgent(t *testing.T) {
	model := openai.NewModel("gpt-5-mini", os.Getenv("OPENAI_API_KEY"))
	TestBasicAgent(t, model)
}

func TestOpenAI_TeamCoordination(t *testing.T) {
	model := openai.NewModel("gpt-5-mini", os.Getenv("OPENAI_API_KEY"))
	TestTeamCoordination(t, model)
}

// TestAgent_Run_WithFileID tests the agent with OpenAI Files API integration
func TestOpenAI_Agent_WithFileID(t *testing.T) {
	// Skip if no OpenAI API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("Skipping OpenAI integration test: OPENAI_API_KEY not set")
	}

	model := openai.NewModel("gpt-5-mini", "")

	// Create a document reference for the file ID
	fileDoc := document.NewInMemoryDocument("file-WjBr55R67mVmhXCsvKZ6Zs", "document.pdf", []byte("test document content for analysis"), nil)

	agent := Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		EnableTrace:  true,
		Documents:    []*document.Document{fileDoc},
	}

	// Test the agent with file ID
	_, err := agent.Execute("Please analyze the attached file and tell me what it contains. If you can access it, start your response with 'SUCCESS:' followed by the analysis.")
	assert.NoError(t, err)
}
