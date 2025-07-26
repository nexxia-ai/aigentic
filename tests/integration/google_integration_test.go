//go:build integration

package integration

import (
	"testing"

	google "github.com/nexxia-ai/aigentic-google"
	"github.com/nexxia-ai/aigentic/ai"
)

func TestGoogleIntegration(t *testing.T) {
	RunIntegrationTestSuite(t, IntegrationTestSuite{
		NewModel: func() *ai.Model {
			return google.NewModel("gemini-1.5-flash", "")
		},
		Name:      "Google",
		SkipTests: []string{}, // Google supports all test types
	})
}
