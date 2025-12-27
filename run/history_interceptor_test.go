package run

import (
	"context"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/event"
	"github.com/stretchr/testify/assert"
)

func TestToolReturnsDocumentInToolResult(t *testing.T) {
	var receivedEvents []*event.ToolResponseEvent
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc)
			run.AgentContext().AddDocument(doc)
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "PDF created successfully at /path/to/test.pdf"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	ag := NewAgentRun("test-document-agent", "Agent that creates documents", "")
	ag.SetModel(model)
	ag.SetTools([]AgentTool{testTool})
	ag.Run(context.Background(), "Please create a PDF")

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

	testTool := AgentTool{
		Name:        "create_report",
		Description: "Creates a report document",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc)
			run.AgentContext().AddDocument(doc)
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Report created successfully"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-history-agent", "Agent that creates documents in history", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create a report")

	result, err := agentRun.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result, "Report is ready")

	entry := agentRun.ConversationTurn()
	assert.NotNil(t, entry, "ConversationTurn should not be nil")
	assert.Equal(t, 1, len(entry.Documents), "Should have one document in ConversationTurn")
	assert.Equal(t, "report.pdf", entry.Documents[0].Document.Filename)
	assert.Equal(t, "application/pdf", entry.Documents[0].Document.MimeType)

}

func TestMultipleDocumentsFromSingleTool(t *testing.T) {
	history := ctxt.NewConversationHistory()
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc1)
			run.AgentContext().AddDocument(doc1)
			run.AgentContext().ConversationTurn().AddDocument("", doc2)
			run.AgentContext().AddDocument(doc2)
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Multiple files created"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-multiple-docs-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create files")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	entry := agentRun.ConversationTurn()
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

	tool1 := AgentTool{
		Name:        "create_pdf1",
		Description: "Creates first PDF",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc1)
			run.AgentContext().AddDocument(doc1)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "PDF1 ready"}},
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc2)
			run.AgentContext().AddDocument(doc2)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "PDF2 ready"}},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-multi-tool-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{tool1, tool2})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create both PDFs")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	entry := agentRun.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Documents), "Should have documents from both tools")
	assert.Equal(t, "pdf1.pdf", entry.Documents[0].Document.Filename)
	assert.Equal(t, "pdf2.pdf", entry.Documents[1].Document.Filename)
}

func TestToolWithNoDocuments(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	testTool := AgentTool{
		Name:        "simple_tool",
		Description: "A simple tool with no documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "Simple response"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-no-docs-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Use simple tool")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	entry := agentRun.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestDocumentsPersistInConversationHistory(t *testing.T) {
	history := ctxt.NewConversationHistory()
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc)
			run.AgentContext().AddDocument(doc)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-persist-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create persistent document")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	conversationTurn := agentRun.ConversationTurn()
	assert.NotNil(t, conversationTurn, "ConversationTurn should not be nil")
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

	testTool := AgentTool{
		Name:        "create_with_metadata",
		Description: "Creates document with metadata",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc)
			run.AgentContext().AddDocument(doc)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-metadata-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create document with metadata")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	entry := agentRun.ConversationTurn()
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

	testTool := AgentTool{
		Name:        "empty_docs_tool",
		Description: "Tool with empty documents",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Response"}},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-empty-docs-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Use empty docs tool")

	_, err := agentRun.Wait(0)
	assert.NoError(t, err)

	entry := agentRun.ConversationTurn()
	assert.NotNil(t, entry)
	assert.Equal(t, 0, len(entry.Documents), "Should have no documents")
}

func TestAgentRunConversationTurnAccess(t *testing.T) {
	history := ctxt.NewConversationHistory()
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			run.AgentContext().ConversationTurn().AddDocument("", doc)
			run.AgentContext().AddDocument(doc)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Document created"}},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	agentRun := NewAgentRun("test-accessible-agent", "", "")
	agentRun.SetModel(model)
	agentRun.SetTools([]AgentTool{testTool})
	agentRun.SetConversationHistory(history)
	agentRun.Run(context.Background(), "Create accessible document")

	result, err := agentRun.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result, "Ready")

	entry := agentRun.ConversationTurn()
	assert.NotNil(t, entry, "ConversationTurn should be accessible after completion")
	assert.Equal(t, agentRun.ID(), entry.RunID, "Turn should have correct RunID")
	assert.Equal(t, 1, len(entry.Documents), "Should have document in turn")
	assert.Equal(t, "accessible.pdf", entry.Documents[0].Document.Filename)

	entry2 := agentRun.ConversationTurn()
	assert.NotNil(t, entry2, "ConversationTurn should still be accessible on second call")
	assert.Equal(t, entry, entry2, "Should return same turn")
}

func TestMessageOrderWithMultipleStartCalls(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0

	testTool := AgentTool{
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "echoed: test"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	run1 := NewAgentRun("test-agent", "", "")
	run1.SetModel(model)
	run1.SetTools([]AgentTool{testTool})
	run1.SetConversationHistory(history)
	run1.Run(context.Background(), "First message - use echo tool")
	_, err := run1.Wait(0)
	assert.NoError(t, err)

	run2 := NewAgentRun("test-agent", "", "")
	run2.SetModel(model)
	run2.SetTools([]AgentTool{testTool})
	run2.SetConversationHistory(history)
	run2.Run(context.Background(), "Second message")
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

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
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
	})

	run1 := NewAgentRun("test-history-agent", "Agent that remembers conversation history", "")
	run1.SetModel(model)
	run1.SetConversationHistory(history)
	run1.Run(context.Background(), "First message")
	result1, err := run1.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result1, "First response")

	assert.Equal(t, 1, history.Len(), "History should have one turn after first conversation")

	receivedMessages = nil

	run2 := NewAgentRun("test-history-agent", "Agent that remembers conversation history", "")
	run2.SetModel(model)
	run2.SetConversationHistory(history)
	run2.Run(context.Background(), "Second message")
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

func TestConversationHistoryNotIncludedWhenDisabled(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0
	var receivedMessages []ai.Message

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		callCount++
		receivedMessages = append(receivedMessages, messages...)

		if callCount == 1 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "First response",
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
					if aiMsg.Content == "First response" {
						hasPreviousMessage = true
						break
					}
				}
			}
			if hasPreviousMessage {
				t.Errorf("Previous conversation messages should NOT be found in second conversation when includeHistory is false. Received %d messages", len(messages))
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Second response",
			}, nil
		}

		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Unexpected call",
		}, nil
	})

	run1 := NewAgentRun("test-history-disabled-agent", "Agent with history disabled", "")
	run1.SetModel(model)
	run1.SetConversationHistory(history)
	run1.IncludeHistory(false)
	run1.Run(context.Background(), "First message")
	result1, err := run1.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result1, "First response")

	assert.Equal(t, 1, history.Len(), "History should have one turn after first conversation (history is always captured)")

	receivedMessages = nil

	run2 := NewAgentRun("test-history-disabled-agent", "Agent with history disabled", "")
	run2.SetModel(model)
	run2.SetConversationHistory(history)
	run2.IncludeHistory(false)
	run2.Run(context.Background(), "Second message")
	result2, err := run2.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result2, "Second response")

	assert.Equal(t, 2, history.Len(), "History should have two turns after second conversation (history is always captured)")

	foundFirstMessage := false
	foundFirstResponse := false
	for _, msg := range receivedMessages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			if userMsg.Content == "First message" {
				foundFirstMessage = true
			}
		}
		if aiMsg, ok := msg.(ai.AIMessage); ok {
			if aiMsg.Content == "First response" {
				foundFirstResponse = true
			}
		}
	}

	assert.False(t, foundFirstMessage, "Previous user message should NOT be included in second conversation when includeHistory is false")
	assert.False(t, foundFirstResponse, "Previous assistant response should NOT be included in second conversation when includeHistory is false")
}

func TestConversationHistoryWithToolsNotIncludedWhenDisabled(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0
	var receivedMessages []ai.Message

	testTool := AgentTool{
		Name:        "echo_tool",
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
		NewExecute: func(run *AgentRun, validationResult event.ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "echoed: test"},
				},
			}, nil
		},
	}

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		callCount++

		if callCount == 1 {
			receivedMessages = messages
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "I'll use the echo tool",
				ToolCalls: []ai.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Name: "echo_tool",
						Args: `{"text": "test"}`,
					},
				},
			}, nil
		}

		if callCount == 2 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Tool response received",
			}, nil
		}

		if callCount == 3 {
			receivedMessages = messages
			foundPreviousToolCall := false
			foundPreviousToolResponse := false
			foundPreviousUserMessage := false
			foundPreviousAssistantMessage := false

			for _, msg := range messages {
				if userMsg, ok := msg.(ai.UserMessage); ok {
					if userMsg.Content == "First message with tools" {
						foundPreviousUserMessage = true
					}
				}
				if aiMsg, ok := msg.(ai.AIMessage); ok {
					if len(aiMsg.ToolCalls) > 0 {
						for _, tc := range aiMsg.ToolCalls {
							if tc.ID == "call_123" && tc.Name == "echo_tool" {
								foundPreviousToolCall = true
							}
						}
					}
					if aiMsg.Content == "Tool response received" {
						foundPreviousAssistantMessage = true
					}
				}
				if toolMsg, ok := msg.(ai.ToolMessage); ok {
					if toolMsg.ToolCallID == "call_123" && toolMsg.ToolName == "echo_tool" {
						foundPreviousToolResponse = true
					}
				}
			}

			if foundPreviousUserMessage || foundPreviousToolCall || foundPreviousToolResponse || foundPreviousAssistantMessage {
				t.Errorf("Previous conversation messages (including tool calls and responses) should NOT be found in second run when includeHistory is false. Found: userMsg=%v, toolCall=%v, toolResponse=%v, assistantMsg=%v. Received %d messages", foundPreviousUserMessage, foundPreviousToolCall, foundPreviousToolResponse, foundPreviousAssistantMessage, len(messages))
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Second response without history",
			}, nil
		}

		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Unexpected call",
		}, nil
	})

	run1 := NewAgentRun("test-history-tools-disabled-agent", "Agent with history disabled and tools", "")
	run1.SetModel(model)
	run1.SetTools([]AgentTool{testTool})
	run1.SetConversationHistory(history)
	run1.IncludeHistory(false)
	run1.Run(context.Background(), "First message with tools")
	result1, err := run1.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result1, "Tool response received")

	assert.Equal(t, 1, history.Len(), "History should have one turn after first conversation (history is always captured)")

	run2 := NewAgentRun("test-history-tools-disabled-agent", "Agent with history disabled and tools", "")
	run2.SetModel(model)
	run2.SetTools([]AgentTool{testTool})
	run2.SetConversationHistory(history)
	run2.IncludeHistory(false)
	run2.Run(context.Background(), "Second message without tools")
	result2, err := run2.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result2, "Second response without history")

	assert.Equal(t, 2, history.Len(), "History should have two turns after second conversation (history is always captured)")

	foundPreviousToolCall := false
	foundPreviousToolResponse := false
	foundPreviousUserMessage := false
	foundPreviousAssistantMessage := false

	if receivedMessages == nil {
		t.Fatal("receivedMessages should not be nil - the model should have been called")
	}

	for _, msg := range receivedMessages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			if userMsg.Content == "First message with tools" {
				foundPreviousUserMessage = true
			}
		}
		if aiMsg, ok := msg.(ai.AIMessage); ok {
			if len(aiMsg.ToolCalls) > 0 {
				for _, tc := range aiMsg.ToolCalls {
					if tc.ID == "call_123" && tc.Name == "echo_tool" {
						foundPreviousToolCall = true
					}
				}
			}
			if aiMsg.Content == "Tool response received" {
				foundPreviousAssistantMessage = true
			}
		}
		if toolMsg, ok := msg.(ai.ToolMessage); ok {
			if toolMsg.ToolCallID == "call_123" && toolMsg.ToolName == "echo_tool" {
				foundPreviousToolResponse = true
			}
		}
	}

	assert.False(t, foundPreviousUserMessage, "Previous user message should NOT be included in second conversation when includeHistory is false")
	assert.False(t, foundPreviousToolCall, "Previous tool call should NOT be included in second conversation when includeHistory is false")
	assert.False(t, foundPreviousToolResponse, "Previous tool response should NOT be included in second conversation when includeHistory is false")
	assert.False(t, foundPreviousAssistantMessage, "Previous assistant message should NOT be included in second conversation when includeHistory is false")
}

func TestMsgHistoryNotIncludedWhenHistoryDisabled(t *testing.T) {
	history := ctxt.NewConversationHistory()
	callCount := 0
	var secondCallMessages []ai.Message

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		callCount++

		if callCount == 1 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "First response about the document",
			}, nil
		}

		if callCount == 2 {
			secondCallMessages = make([]ai.Message, len(messages))
			copy(secondCallMessages, messages)
			foundFirstUserMessage := false
			foundFirstAssistantMessage := false

			for _, msg := range messages {
				if userMsg, ok := msg.(ai.UserMessage); ok {
					if userMsg.Content == "What is in the document?" {
						foundFirstUserMessage = true
					}
				}
				if aiMsg, ok := msg.(ai.AIMessage); ok {
					if aiMsg.Content == "First response about the document" {
						foundFirstAssistantMessage = true
					}
				}
			}

			if foundFirstUserMessage || foundFirstAssistantMessage {
				t.Errorf("Previous conversation messages should NOT be found in second call when includeHistory is false. Found: userMsg=%v, assistantMsg=%v. Received %d messages", foundFirstUserMessage, foundFirstAssistantMessage, len(messages))
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Second response without document",
			}, nil
		}

		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Unexpected call",
		}, nil
	})

	run := NewAgentRun("test-msg-history-disabled", "Agent with history disabled", "")
	run.SetModel(model)
	run.SetConversationHistory(history)
	run.IncludeHistory(false)

	run.AgentContext().AddDocument(doc)
	run.Run(context.Background(), "What is in the document?")
	result1, err := run.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result1, "First response about the document")
	assert.Equal(t, 1, callCount, "Model should have been called once")

	run.AgentContext().RemoveDocument(doc)

	run.Run(context.Background(), "What is the answer?")
	result2, err := run.Wait(0)
	assert.NoError(t, err)
	assert.Contains(t, result2, "Second response without document")
	assert.Equal(t, 2, callCount, "Model should have been called twice")

	foundFirstUserMessage := false
	foundFirstAssistantMessage := false

	if secondCallMessages == nil {
		t.Fatal("secondCallMessages should not be nil - the model should have been called")
	}

	for _, msg := range secondCallMessages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			if userMsg.Content == "What is in the document?" {
				foundFirstUserMessage = true
			}
		}
		if aiMsg, ok := msg.(ai.AIMessage); ok {
			if aiMsg.Content == "First response about the document" {
				foundFirstAssistantMessage = true
			}
		}
	}

	assert.False(t, foundFirstUserMessage, "Previous user message should NOT be included in second call when includeHistory is false")
	assert.False(t, foundFirstAssistantMessage, "Previous assistant message should NOT be included in second call when includeHistory is false")
}
