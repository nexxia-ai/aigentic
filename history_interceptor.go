package aigentic

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

// historyInterceptor captures conversation history across agent runs
type historyInterceptor struct {
	history      *ConversationHistory
	currentEntry *HistoryEntry
}

// newHistoryInterceptor creates a new history interceptor
func newHistoryInterceptor(history *ConversationHistory) *historyInterceptor {
	return &historyInterceptor{
		history: history,
	}
}

// BeforeCall injects previous conversation history into the messages
func (h *historyInterceptor) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	// Initialize current entry for this conversation turn
	h.initializeCurrentEntry(run, messages)

	// Inject previous conversation history into messages
	historyMessages := h.history.GetMessages()
	if len(historyMessages) == 0 {
		return messages, tools, nil
	}

	// Find where to inject history (after ALL system messages, before user messages/documents)
	injectionPoint := 0
	for i, msg := range messages {
		role, _ := msg.Value()
		if role == ai.SystemRole {
			injectionPoint = i + 1
		} else if role != ai.SystemRole {
			break
		}
	}

	// Insert history messages at the injection point
	result := make([]ai.Message, 0, len(messages)+len(historyMessages))
	result = append(result, messages[:injectionPoint]...)
	result = append(result, historyMessages...)
	result = append(result, messages[injectionPoint:]...)

	return result, tools, nil
}

// AfterCall captures the AI response and finalizes the conversation turn if no tool calls
func (h *historyInterceptor) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	if h.currentEntry == nil {
		return response, nil
	}

	// Set the assistant message
	h.currentEntry.AssistantMessage = response
	// Ensure run's reference is up to date
	run.currentHistoryEntry = h.currentEntry

	// If no tool calls, finalize this conversation turn
	if len(response.ToolCalls) == 0 {
		h.finalizeEntry()
	}

	return response, nil
}

// BeforeToolCall passes through without modification
func (h *historyInterceptor) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error) {
	return validationResult, nil
}

// AfterToolCall captures tool responses and finalizes if this was the last tool call
func (h *historyInterceptor) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
	if h.currentEntry == nil {
		return result, nil
	}

	// Create a ToolMessage from the result
	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    formatToolResponse(result),
		ToolCallID: toolCallID,
		ToolName:   toolName,
	}

	// Append to tool messages
	h.currentEntry.ToolMessages = append(h.currentEntry.ToolMessages, toolMsg)

	// Extract and append documents from ToolResult
	if result != nil && len(result.Documents) > 0 {
		h.currentEntry.Documents = append(h.currentEntry.Documents, result.Documents...)
	}

	return result, nil
}

// initializeCurrentEntry creates a new history entry for the current conversation turn
func (h *historyInterceptor) initializeCurrentEntry(run *AgentRun, messages []ai.Message) {
	if h.currentEntry != nil {
		return
	}

	// Use the original user message from the run, not the templated prompt
	if run.userMessage == "" {
		return
	}

	traceFile := ""
	if run.trace != nil {
		traceFile = run.trace.Filepath()
	}

	h.currentEntry = &HistoryEntry{
		UserMessage:  ai.UserMessage{Role: ai.UserRole, Content: run.userMessage},
		ToolMessages: make([]ai.Message, 0),
		Documents:    make([]*document.Document, 0),
		TraceFile:    traceFile,
		RunID:        run.ID(),
		Timestamp:    time.Now(),
		AgentName:    run.agent.Name,
	}
	run.currentHistoryEntry = h.currentEntry
}

// finalizeEntry appends the current entry to history and resets it
func (h *historyInterceptor) finalizeEntry() {
	if h.currentEntry == nil {
		return
	}

	// Finalize the entry - make a copy for history, but keep the original reference
	// in run.currentHistoryEntry so callers can access it after completion
	h.history.AppendEntry(*h.currentEntry)
	// Note: run.currentHistoryEntry already points to h.currentEntry, so it remains accessible
	// The interceptor's currentEntry can be cleared
	h.currentEntry = nil
}
