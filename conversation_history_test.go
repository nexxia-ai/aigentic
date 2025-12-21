package aigentic

import (
	"context"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/stretchr/testify/assert"
)

func TestToolReturnsDocumentInToolResult(t *testing.T) {
	var receivedEvents []*event.ToolResponseEvent
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("PDF content"), nil)
	doc.FilePath = "/path/to/test.pdf"
	doc.MimeType = "application/pdf"

	testTool := run.AgentTool{
		Name:        "create_pdf",
		Description: "Creates a PDF document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "PDF created successfully at /path/to/test.pdf"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:        "test-document-agent",
		Description: "Agent that creates documents",
		AgentTools:  []run.AgentTool{testTool},
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

	ag, err := agent.Start("Please create a PDF")
	assert.NoError(t, err)

	for evt := range ag.Next() {
		switch ev := evt.(type) {
		case *event.ToolResponseEvent:
			receivedEvents = append(receivedEvents, ev)
		case *event.ErrorEvent:
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

func TestDocumentsAddedToConversationTurn(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "report.pdf", []byte("Report content"), nil)
	doc.FilePath = "/path/to/report.pdf"
	doc.MimeType = "application/pdf"

	testTool := run.AgentTool{
		Name:        "create_report",
		Description: "Creates a report document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Report created successfully"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-history-agent",
		Description:         "Agent that creates documents in history",
		AgentTools:          []run.AgentTool{testTool},
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry, "ConversationTurn should not be nil")
	assert.Equal(t, 1, len(entry.Documents), "Should have one document in ConversationTurn")
	assert.Equal(t, "report.pdf", entry.Documents[0].Document.Filename)
	assert.Equal(t, "application/pdf", entry.Documents[0].Document.MimeType)

	conversationTurn, err := history.GetByRunID(run.ID())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(conversationTurn.Documents), "Document should persist in ConversationHistory")
}

func TestMultipleDocumentsFromSingleTool(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc1 := document.NewInMemoryDocument("doc1", "file1.pdf", []byte("Content 1"), nil)
	doc1.MimeType = "application/pdf"
	doc2 := document.NewInMemoryDocument("doc2", "file2.txt", []byte("Content 2"), nil)
	doc2.MimeType = "text/plain"

	testTool := run.AgentTool{
		Name:        "create_files",
		Description: "Creates multiple files",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc1, "model")
			run.AddDocument("", doc2, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Multiple files created"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-multiple-docs-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Documents), "Should have two documents")
	assert.Equal(t, "file1.pdf", entry.Documents[0].Document.Filename)
	assert.Equal(t, "file2.txt", entry.Documents[1].Document.Filename)
}

func TestMultipleToolsWithDocuments(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc1 := document.NewInMemoryDocument("doc1", "pdf1.pdf", []byte("PDF 1"), nil)
	doc1.MimeType = "application/pdf"
	doc2 := document.NewInMemoryDocument("doc2", "pdf2.pdf", []byte("PDF 2"), nil)
	doc2.MimeType = "application/pdf"

	tool1 := run.AgentTool{
		Name:        "create_pdf1",
		Description: "Creates first PDF",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc1, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "PDF1 ready"}},
			}, nil
		},
	}

	tool2 := run.AgentTool{
		Name:        "create_pdf2",
		Description: "Creates second PDF",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc2, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "PDF2 ready"}},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-multi-tool-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{tool1, tool2},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Documents), "Should have documents from both tools")
	assert.Equal(t, "pdf1.pdf", entry.Documents[0].Document.Filename)
	assert.Equal(t, "pdf2.pdf", entry.Documents[1].Document.Filename)
}

func TestToolWithNoDocuments(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	testTool := run.AgentTool{
		Name:        "simple_tool",
		Description: "A simple tool with no documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Simple response"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-no-docs-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestDocumentsPersistInConversationHistory(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "persistent.pdf", []byte("Persistent content"), nil)
	doc.MimeType = "application/pdf"

	testTool := run.AgentTool{
		Name:        "create_persistent",
		Description: "Creates a persistent document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-persist-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	conversationTurn, err := history.GetByRunID(runID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(conversationTurn.Documents), "Document should persist in ConversationHistory")
	assert.Equal(t, "persistent.pdf", conversationTurn.Documents[0].Document.Filename)
}

func TestDocumentMetadataPreserved(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "metadata.pdf", []byte("Metadata test"), nil)
	doc.FilePath = "/custom/path/metadata.pdf"
	doc.MimeType = "application/pdf"
	doc.FileSize = 12345
	doc.URL = "https://example.com/metadata.pdf"

	testTool := run.AgentTool{
		Name:        "create_with_metadata",
		Description: "Creates document with metadata",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-metadata-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 1, len(entry.Documents), "Should have one document")

	savedDoc := entry.Documents[0].Document
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
	history := ctxt.NewConversationHistory()
	callCount := 0

	testTool := run.AgentTool{
		Name:        "empty_docs_tool",
		Description: "Tool with empty documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Response"}},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-empty-docs-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestAgentRunConversationTurnAccess(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	doc := document.NewInMemoryDocument("doc1", "accessible.pdf", []byte("Accessible content"), nil)
	doc.MimeType = "application/pdf"

	testTool := run.AgentTool{
		Name:        "create_accessible",
		Description: "Creates accessible document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AddDocument("", doc, "model")
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-accessible-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
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

	entry := run.ConversationTurn()
	assert.NotNil(t, entry, "ConversationTurn should be accessible after completion")
	assert.Equal(t, run.ID(), entry.RunID, "Turn should have correct RunID")
	assert.Equal(t, 1, len(entry.Documents), "Should have document in turn")
	assert.Equal(t, "accessible.pdf", entry.Documents[0].Document.Filename)

	entry2 := run.ConversationTurn()
	assert.NotNil(t, entry2, "ConversationTurn should still be accessible on second call")
	assert.Equal(t, entry, entry2, "Should return same turn")
}

func TestMessageOrderWithMultipleStartCalls(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	testTool := run.AgentTool{
		Name:        "echo",
		Description: "Echoes back the input",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"text"},
		},
		NewExecute: func(run *run.AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "echoed: test"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-agent",
		ConversationHistory: history,
		AgentTools:          []run.AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Name: "echo",
							Args: `{"text": "test"}`,
						},
					},
				}, nil
			}

			if callCount == 2 {
				verifyMessageOrder(t, messages)
				verifyToolCallIDsMatch(t, messages)
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Second call completed",
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Final response",
			}, nil
		}),
	}

	run1, err := agent.Start("First message - use echo tool")
	assert.NoError(t, err)
	_, err = run1.Wait(0)
	assert.NoError(t, err)

	run2, err := agent.Start("Second message")
	assert.NoError(t, err)
	_, err = run2.Wait(0)
	assert.NoError(t, err)
}

func verifyToolCallIDsMatch(t *testing.T, messages []ai.Message) {
	var currentAssistantMessage ai.AIMessage
	var assistantHasToolCalls bool

	for _, msg := range messages {
		role, _ := msg.Value()

		if role == ai.AssistantRole {
			if aiMsg, ok := msg.(ai.AIMessage); ok {
				currentAssistantMessage = aiMsg
				assistantHasToolCalls = len(aiMsg.ToolCalls) > 0
			}
		}

		if role == ai.ToolRole {
			if toolMsg, ok := msg.(ai.ToolMessage); ok {
				if assistantHasToolCalls {
					found := false
					for _, tc := range currentAssistantMessage.ToolCalls {
						if tc.ID == toolMsg.ToolCallID {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Tool message with tool_call_id '%s' does not match any tool_call ID in the preceding assistant message", toolMsg.ToolCallID)
					}
				}
			}
		}
	}
}

func verifyMessageOrder(t *testing.T, messages []ai.Message) {
	assistantWithToolCallsFound := false
	toolMessageFound := false
	lastAssistantIndex := -1

	for i, msg := range messages {
		role, _ := msg.Value()

		if role == ai.AssistantRole {
			if aiMsg, ok := msg.(ai.AIMessage); ok && len(aiMsg.ToolCalls) > 0 {
				assistantWithToolCallsFound = true
				lastAssistantIndex = i
			}
		}

		if role == ai.ToolRole {
			if !assistantWithToolCallsFound {
				t.Errorf("Tool message found at index %d without preceding assistant message with tool_calls. Last assistant was at index %d", i, lastAssistantIndex)
			}
			if lastAssistantIndex >= i {
				t.Errorf("Tool message found at index %d but assistant with tool_calls is at index %d (should come before)", i, lastAssistantIndex)
			}
			toolMessageFound = true
		}
	}

	assert.True(t, assistantWithToolCallsFound, "Should have assistant message with tool_calls")
	assert.True(t, toolMessageFound, "Should have tool message")
}

func TestConversationHistoryIncludedInFutureConversations(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0
	var receivedMessages []ai.Message

	agent := Agent{
		Name:                "test-history-agent",
		Description:         "Agent that remembers conversation history",
		ConversationHistory: history,
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			receivedMessages = append(receivedMessages, messages...)

			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Hello, I'm the assistant. First response.",
				}, nil
			}

			if callCount == 2 {
				hasPreviousMessage := false
				for _, msg := range messages {
					if userMsg, ok := msg.(ai.UserMessage); ok {
						if userMsg.Content == "First message" {
							hasPreviousMessage = true
							break
						}
					}
					if aiMsg, ok := msg.(ai.AIMessage); ok {
						if aiMsg.Content == "Hello, I'm the assistant. First response." {
							hasPreviousMessage = true
							break
						}
					}
				}
				if !hasPreviousMessage {
					t.Errorf("Previous conversation messages not found in second conversation. Received %d messages", len(messages))
				}
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Hello again, I remember our previous conversation.",
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Unexpected call",
			}, nil
		}),
	}

	run1, err := agent.Start("First message")
	assert.NoError(t, err)
	result1, err := run1.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result1, "First response")

	assert.Equal(t, 1, history.Len(), "History should have one turn after first conversation")

	receivedMessages = nil

	run2, err := agent.Start("Second message")
	assert.NoError(t, err)
	result2, err := run2.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result2, "remember our previous conversation")

	assert.Equal(t, 2, history.Len(), "History should have two turns after second conversation")

	foundFirstMessage := false
	foundFirstResponse := false
	for _, msg := range receivedMessages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			if userMsg.Content == "First message" {
				foundFirstMessage = true
			}
		}
		if aiMsg, ok := msg.(ai.AIMessage); ok {
			if aiMsg.Content == "Hello, I'm the assistant. First response." {
				foundFirstResponse = true
			}
		}
	}

	assert.True(t, foundFirstMessage, "Previous user message should be included in second conversation")
	assert.True(t, foundFirstResponse, "Previous assistant response should be included in second conversation")
}
