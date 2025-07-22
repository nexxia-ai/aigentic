package aigentic

import (
	"os"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestTrace_LLMCall_ResourceMessage(t *testing.T) {
	// Create a temporary trace file
	tempDir := t.TempDir()

	// Create trace with custom directory
	trace := NewTrace(TraceConfig{Directory: tempDir})
	defer trace.Close()

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

	// Call LLMCall
	err := trace.LLMCall("test-model", "test-agent", messages)
	if err != nil {
		t.Fatalf("LLMCall failed: %v", err)
	}

	// Read the trace file content
	content, err := os.ReadFile(trace.filename)
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

	// Create trace with custom directory
	trace := NewTrace(TraceConfig{Directory: tempDir})
	defer trace.Close()

	// Create a ResourceMessage with content
	messages := []ai.Message{
		ai.ResourceMessage{
			Role:     ai.UserRole,
			Name:     "large-image.png",
			MIMEType: "image/png",
			Body:     []byte("fake-image-data-that-is-quite-long"),
		},
	}

	// Call LLMCall
	err := trace.LLMCall("test-model", "test-agent", messages)
	if err != nil {
		t.Fatalf("LLMCall failed: %v", err)
	}

	// Read the trace file content
	content, err := os.ReadFile(trace.filename)
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
