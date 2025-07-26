//go:build integration

package integration

import (
	"testing"

	ollama "github.com/nexxia-ai/aigentic-ollama"
	"github.com/nexxia-ai/aigentic/ai"
)

func TestOllamaIntegration(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return ollama.NewModel("qwen3:1.7b", "")
		},
		Name:      "Ollama",
		SkipTests: []string{}, // Ollama supports all test types
	})
}
