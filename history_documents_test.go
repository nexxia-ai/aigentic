package aigentic

import (
	"context"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
)

func TestToolReturnsDocumentInToolResult(t *testing.T) {
	var receivedEvents []*ToolResponseEvent
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("PDF content"), nil)
	doc.FilePath = "/path/to/test.pdf"
	doc.MimeType = "application/pdf"

	testTool := AgentTool{
		Name:        "create_pdf",
		Description: "Creates a PDF document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "PDF created successfully at /path/to/test.pdf"},
				},
				Documents: []*document.Document{doc},
			}, nil
		},
	}

	agent := Agent{
		Name:        "test-document-agent",
		Description: "Agent that creates documents",
		AgentTools:  []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll create a PDF for you.",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Name: "create_pdf",
							Args: "{}",
						},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "PDF has been created successfully.",
			}, nil
		}),
	}

	run, err := agent.Start("Please create a PDF")
	assert.NoError(t, err)

	for event := range run.Next() {
		switch ev := event.(type) {
		case *ToolResponseEvent:
			receivedEvents = append(receivedEvents, ev)
		case *ErrorEvent:
			t.Fatalf("Unexpected error: %v", ev.Err)
		}
	}

	assert.GreaterOrEqual(t, len(receivedEvents), 1, "Should receive at least one ToolResponseEvent")
	
	hasDocument := false
	for _, event := range receivedEvents {
		if event.ToolName == "create_pdf" {
			assert.NotNil(t, event.Documents, "Documents should not be nil")
			if len(event.Documents) > 0 {
				assert.Equal(t, 1, len(event.Documents), "Should have one document")
				assert.Equal(t, "test.pdf", event.Documents[0].Filename)
				assert.Equal(t, "application/pdf", event.Documents[0].MimeType)
				hasDocument = true
			}
		}
	}
	assert.True(t, hasDocument, "Should receive document in ToolResponseEvent")
}

func TestDocumentsAddedToHistoryEntry(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "report.pdf", []byte("Report content"), nil)
	doc.FilePath = "/path/to/report.pdf"
	doc.MimeType = "application/pdf"

	testTool := AgentTool{
		Name:        "create_report",
		Description: "Creates a report document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Report created successfully"},
				},
				Documents: []*document.Document{doc},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-history-agent",
		Description:       "Agent that creates documents in history",
		AgentTools:        []AgentTool{testTool},
		ConversationHistory: history,
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Creating report...",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_456",
							Type: "function",
							Name: "create_report",
							Args: "{}",
						},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Report is ready.",
			}, nil
		}),
	}

	run, err := agent.Start("Create a report")
	assert.NoError(t, err)

	result, err := run.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result, "Report is ready")

	entry := run.HistoryEntry()
	assert.NotNil(t, entry, "HistoryEntry should not be nil")
	assert.Equal(t, 1, len(entry.Documents), "Should have one document in HistoryEntry")
	assert.Equal(t, "report.pdf", entry.Documents[0].Filename)
	assert.Equal(t, "application/pdf", entry.Documents[0].MimeType)

	historyEntry, err := history.GetByRunID(run.ID())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(historyEntry.Documents), "Document should persist in ConversationHistory")
}

func TestMultipleDocumentsFromSingleTool(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc1 := document.NewInMemoryDocument("doc1", "file1.pdf", []byte("Content 1"), nil)
	doc1.MimeType = "application/pdf"
	doc2 := document.NewInMemoryDocument("doc2", "file2.txt", []byte("Content 2"), nil)
	doc2.MimeType = "text/plain"

	testTool := AgentTool{
		Name:        "create_files",
		Description: "Creates multiple files",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Multiple files created"},
				},
				Documents: []*document.Document{doc1, doc2},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-multiple-docs-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_789",
							Type: "function",
							Name: "create_files",
							Args: "{}",
						},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Files ready",
			}, nil
		}),
	}

	run, err := agent.Start("Create files")
	assert.NoError(t, err)

	_, err = run.Wait(0)
	assert.NoError(t, err)

	entry := run.HistoryEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Documents), "Should have two documents")
	assert.Equal(t, "file1.pdf", entry.Documents[0].Filename)
	assert.Equal(t, "file2.txt", entry.Documents[1].Filename)
}

func TestMultipleToolsWithDocuments(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc1 := document.NewInMemoryDocument("doc1", "pdf1.pdf", []byte("PDF 1"), nil)
	doc1.MimeType = "application/pdf"
	doc2 := document.NewInMemoryDocument("doc2", "pdf2.pdf", []byte("PDF 2"), nil)
	doc2.MimeType = "application/pdf"

	tool1 := AgentTool{
		Name:        "create_pdf1",
		Description: "Creates first PDF",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "PDF1 ready"}},
				Documents: []*document.Document{doc1},
			}, nil
		},
	}

	tool2 := AgentTool{
		Name:        "create_pdf2",
		Description: "Creates second PDF",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "PDF2 ready"}},
				Documents: []*document.Document{doc2},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-multi-tool-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{tool1, tool2},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_1", Type: "function", Name: "create_pdf1", Args: "{}"},
						{ID: "call_2", Type: "function", Name: "create_pdf2", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Both PDFs ready",
			}, nil
		}),
	}

	run, err := agent.Start("Create both PDFs")
	assert.NoError(t, err)

	_, err = run.Wait(0)
	assert.NoError(t, err)

	entry := run.HistoryEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Documents), "Should have documents from both tools")
	assert.Equal(t, "pdf1.pdf", entry.Documents[0].Filename)
	assert.Equal(t, "pdf2.pdf", entry.Documents[1].Filename)
}

func TestToolWithNoDocuments(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	testTool := AgentTool{
		Name:        "simple_tool",
		Description: "A simple tool with no documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Simple response"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-no-docs-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_simple", Type: "function", Name: "simple_tool", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Done",
			}, nil
		}),
	}

	run, err := agent.Start("Use simple tool")
	assert.NoError(t, err)

	_, err = run.Wait(0)
	assert.NoError(t, err)

	entry := run.HistoryEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestDocumentsPersistInConversationHistory(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "persistent.pdf", []byte("Persistent content"), nil)
	doc.MimeType = "application/pdf"

	testTool := AgentTool{
		Name:        "create_persistent",
		Description: "Creates a persistent document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "Document created"}},
				Documents: []*document.Document{doc},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-persist-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_persist", Type: "function", Name: "create_persistent", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Ready",
			}, nil
		}),
	}

	run, err := agent.Start("Create persistent document")
	assert.NoError(t, err)

	runID := run.ID()
	_, err = run.Wait(0)
	assert.NoError(t, err)

	historyEntry, err := history.GetByRunID(runID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(historyEntry.Documents), "Document should persist in ConversationHistory")
	assert.Equal(t, "persistent.pdf", historyEntry.Documents[0].Filename)
}

func TestDocumentMetadataPreserved(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "metadata.pdf", []byte("Metadata test"), nil)
	doc.FilePath = "/custom/path/metadata.pdf"
	doc.MimeType = "application/pdf"
	doc.FileSize = 12345
	doc.URL = "https://example.com/metadata.pdf"

	testTool := AgentTool{
		Name:        "create_with_metadata",
		Description: "Creates document with metadata",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "Document created"}},
				Documents: []*document.Document{doc},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-metadata-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_meta", Type: "function", Name: "create_with_metadata", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Ready",
			}, nil
		}),
	}

	run, err := agent.Start("Create document with metadata")
	assert.NoError(t, err)

	_, err = run.Wait(0)
	assert.NoError(t, err)

	entry := run.HistoryEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, 1, len(entry.Documents), "Should have one document")

	savedDoc := entry.Documents[0]
	assert.Equal(t, "metadata.pdf", savedDoc.Filename)
	assert.Equal(t, "/custom/path/metadata.pdf", savedDoc.FilePath)
	assert.Equal(t, "application/pdf", savedDoc.MimeType)
	assert.Equal(t, int64(12345), savedDoc.FileSize)
	assert.Equal(t, "https://example.com/metadata.pdf", savedDoc.URL)

	bytes, err := savedDoc.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, []byte("Metadata test"), bytes)
}

func TestEmptyToolResultDocuments(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	testTool := AgentTool{
		Name:        "empty_docs_tool",
		Description: "Tool with empty documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "Response"}},
				Documents: []*document.Document{},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-empty-docs-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_empty", Type: "function", Name: "empty_docs_tool", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Done",
			}, nil
		}),
	}

	run, err := agent.Start("Use empty docs tool")
	assert.NoError(t, err)

	_, err = run.Wait(0)
	assert.NoError(t, err)

	entry := run.HistoryEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestAgentRunHistoryEntryAccess(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "accessible.pdf", []byte("Accessible content"), nil)
	doc.MimeType = "application/pdf"

	testTool := AgentTool{
		Name:        "create_accessible",
		Description: "Creates accessible document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content:   []ai.ToolContent{{Type: "text", Content: "Document created"}},
				Documents: []*document.Document{doc},
			}, nil
		},
	}

	agent := Agent{
		Name:              "test-accessible-agent",
		ConversationHistory: history,
		AgentTools:        []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{ID: "call_acc", Type: "function", Name: "create_accessible", Args: "{}"},
					},
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Ready",
			}, nil
		}),
	}

	run, err := agent.Start("Create accessible document")
	assert.NoError(t, err)

	result, err := run.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result, "Ready")

	entry := run.HistoryEntry()
	assert.NotNil(t, entry, "HistoryEntry should be accessible after completion")
	assert.Equal(t, run.ID(), entry.RunID, "Entry should have correct RunID")
	assert.Equal(t, 1, len(entry.Documents), "Should have document in entry")
	assert.Equal(t, "accessible.pdf", entry.Documents[0].Filename)

	entry2 := run.HistoryEntry()
	assert.NotNil(t, entry2, "HistoryEntry should still be accessible on second call")
	assert.Equal(t, entry, entry2, "Should return same entry")
}

