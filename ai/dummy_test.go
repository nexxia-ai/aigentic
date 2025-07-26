package ai

import (
	"testing"
)

func TestDummy_StandardSuite(t *testing.T) {
	suite := ModelTestSuite{
		NewModel: func() *Model {
			replayFunc, err := ReplayFunction("testdata/ollama_test_data.json")
			if err != nil {
				t.Fatalf("Failed to create replay function: %v", err)
			}
			m := NewDummyModel(replayFunc)
			return m
		},
		Name: "Ollama",
	}
	RunModelTestSuite(t, suite)
}
