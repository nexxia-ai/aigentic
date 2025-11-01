package aigentic

import (
	"os"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
)

// createTestAgentRun creates a minimal AgentRun for testing
func createTestAgentRun(agentName, modelName string) *AgentRun {
	return &AgentRun{
		agent: Agent{
			Name: agentName,
		},
		model: &ai.Model{
			ModelName: modelName,
		},
	}
}

func TestTrace_LLMCall_ResourceMessage(t *testing.T) {
	// Create a temporary trace file
	tempDir := t.TempDir()

	// Create tracer with custom directory
	tracer := NewTracer(TraceConfig{Directory: tempDir})
	trace := tracer.NewTraceRun("test-run-id")
	defer trace.Close()

	// Create test AgentRun
	run := createTestAgentRun("test-agent", "test-model")

	// Create test messages including a ResourceMessage
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

	// Call BeforeCall (implements Interceptor)
	_, _, err := trace.BeforeCall(run, messages, nil)
	if err != nil {
		t.Fatalf("BeforeCall failed: %v", err)
	}

	// Read the trace file content
	content, err := os.ReadFile(trace.Filepath())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	contentStr := string(content)

	// Check that ResourceMessage was logged correctly
	expectedResourceLog := "resource: test-document.pdf"
	if !strings.Contains(contentStr, expectedResourceLog) {
		t.Errorf("Expected trace to contain '%s', but got: %s", expectedResourceLog, contentStr)
	}

	// Check that other messages were logged normally
	if !strings.Contains(contentStr, "Hello, world!") {
		t.Error("Expected trace to contain user message content")
	}

	if !strings.Contains(contentStr, "Response content") {
		t.Error("Expected trace to contain AI message content")
	}

	t.Logf("Trace content:\n%s", contentStr)
}

func TestTrace_LLMCall_ResourceMessageWithContent(t *testing.T) {
	// Create a temporary trace file
	tempDir := t.TempDir()

	// Create tracer with custom directory
	tracer := NewTracer(TraceConfig{Directory: tempDir})
	trace := tracer.NewTraceRun("test-run-id")
	defer trace.Close()

	// Create test AgentRun
	run := createTestAgentRun("test-agent", "test-model")

	// Create a ResourceMessage with content
	messages := []ai.Message{
		ai.ResourceMessage{
			Role:     ai.UserRole,
			Name:     "large-image.png",
			MIMEType: "image/png",
			Body:     []byte("fake-image-data-that-is-quite-long"),
		},
	}

	// Call BeforeCall (implements Interceptor)
	_, _, err := trace.BeforeCall(run, messages, nil)
	if err != nil {
		t.Fatalf("BeforeCall failed: %v", err)
	}

	// Read the trace file content
	content, err := os.ReadFile(trace.Filepath())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	contentStr := string(content)

	// Check that ResourceMessage was logged with correct content length
	expectedResourceLog := "resource: large-image.png (content length: 34)"
	if !strings.Contains(contentStr, expectedResourceLog) {
		t.Errorf("Expected trace to contain '%s', but got: %s", expectedResourceLog, contentStr)
	}

	t.Logf("Trace content:\n%s", contentStr)
}
