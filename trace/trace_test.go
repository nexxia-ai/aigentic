package trace

import (
	"os"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
)

func createTestAgentRun(agentName, modelName string) *run.AgentRun {
	a := run.NewAgentRun("testAgent", "", "")
	a.SetModel(&ai.Model{ModelName: modelName})
	return a
}

func TestTrace_LLMCall_ResourceMessage(t *testing.T) {
	tempDir := t.TempDir()

	trace := NewTracer(TraceConfig{Directory: tempDir})
	defer trace.Close()

	run := createTestAgentRun("test-agent", "test-model")

	messages := []ai.Message{
		ai.UserMessage{
			Role:    ai.UserRole,
			Content: "Hello, world!",
		},
		ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  "file://file-abc123",
			Name: "test-document.pdf",
		},
		ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Response content",
		},
	}

	_, _, err := trace.BeforeCall(run, messages, nil)
	if err != nil {
		t.Fatalf("BeforeCall failed: %v", err)
	}

	traceRun := trace.(*TraceRun)
	content, err := os.ReadFile(traceRun.Filepath())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	contentStr := string(content)

	expectedResourceLog := "resource: test-document.pdf"
	if !strings.Contains(contentStr, expectedResourceLog) {
		t.Errorf("Expected trace to contain '%s', but got: %s", expectedResourceLog, contentStr)
	}

	if !strings.Contains(contentStr, "Hello, world!") {
		t.Error("Expected trace to contain user message content")
	}

	if !strings.Contains(contentStr, "Response content") {
		t.Error("Expected trace to contain AI message content")
	}

	t.Logf("Trace content:\n%s", contentStr)
}

func TestTrace_LLMCall_ResourceMessageWithContent(t *testing.T) {
	tempDir := t.TempDir()

	trace := NewTracer(TraceConfig{Directory: tempDir})
	defer trace.Close()

	run := createTestAgentRun("test-agent", "test-model")

	messages := []ai.Message{
		ai.ResourceMessage{
			Role:     ai.UserRole,
			Name:     "large-image.png",
			MIMEType: "image/png",
			Body:     []byte("fake-image-data-that-is-quite-long"),
		},
	}

	_, _, err := trace.BeforeCall(run, messages, nil)
	if err != nil {
		t.Fatalf("BeforeCall failed: %v", err)
	}

	traceRun := trace.(*TraceRun)
	content, err := os.ReadFile(traceRun.Filepath())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	contentStr := string(content)

	expectedResourceLog := "resource: large-image.png (content length: 34)"
	if !strings.Contains(contentStr, expectedResourceLog) {
		t.Errorf("Expected trace to contain '%s', but got: %s", expectedResourceLog, contentStr)
	}

	t.Logf("Trace content:\n%s", contentStr)
}
