package aigentic

import (
	"fmt"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

// historyInterceptor captures conversation history across agent runs
type historyInterceptor struct {
	history *ConversationHistory
}

// newHistoryInterceptor creates a new history interceptor
func newHistoryInterceptor(history *ConversationHistory) *historyInterceptor {
	return &historyInterceptor{
		history: history,
	}
}

// BeforeCall appends conversation history just before the new user message
func (h *historyInterceptor) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	if len(messages) == 0 {
		return messages, tools, nil
	}

	historyMessages := h.history.GetMessages()
	if len(historyMessages) == 0 {
		return messages, tools, nil
	}

	// Find the first user message (the new user message from the template)
	userMessageIndex := -1
	for i, msg := range messages {
		role, _ := msg.Value()
		if role == ai.UserRole {
			// Check if this is a UserMessage (not ResourceMessage or ToolMessage)
			if _, ok := msg.(ai.UserMessage); ok {
				userMessageIndex = i
				break
			}
		}
	}

	// If no user message found, append history at the end (shouldn't happen in normal flow)
	if userMessageIndex == -1 {
		result := make([]ai.Message, 0, len(messages)+len(historyMessages))
		result = append(result, messages...)
		result = append(result, historyMessages...)
		return result, tools, nil
	}

	// Insert history right before the user message (as second-to-last before user message)
	result := make([]ai.Message, 0, len(messages)+len(historyMessages))
	result = append(result, messages[:userMessageIndex]...) // All messages before user message
	result = append(result, historyMessages...)              // History messages (chronological order)
	result = append(result, messages[userMessageIndex:]...) // User message and everything after
	return result, tools, nil
}

// AfterCall appends the completed conversation turn to history when conversation completes
func (h *historyInterceptor) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	// Only append to history when conversation turn completes (no tool calls)
	if len(response.ToolCalls) == 0 {
		// Set the Reply before appending to history
		run.currentConversationTurn.Reply = response
		h.history.AppendTurn(*run.currentConversationTurn)
	}

	return response, nil
}

// BeforeToolCall passes through without modification
func (h *historyInterceptor) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error) {
	return validationResult, nil
}

// AfterToolCall passes through without modification
func (h *historyInterceptor) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
	return result, nil
}

// ConversationHistory stores conversation history with metadata for trace correlation
type ConversationHistory struct {
	turns []ConversationTurn
	mutex sync.RWMutex
}

// NewConversationHistory creates a new conversation history object
func NewConversationHistory() *ConversationHistory {
	return &ConversationHistory{
		turns: make([]ConversationTurn, 0),
	}
}

// GetTurns returns a copy of all conversation turns
func (h *ConversationHistory) GetTurns() []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]ConversationTurn, len(h.turns))
	copy(result, h.turns)
	return result
}

// GetEntries returns a copy of all conversation turns (for backward compatibility)
func (h *ConversationHistory) GetEntries() []ConversationTurn {
	return h.GetTurns()
}

// GetMessages returns all messages flattened for LLM context (user, reply in order)
// Hidden entries are excluded from the result
// Messages are returned in chronological order (oldest first)
func (h *ConversationHistory) GetMessages() []ai.Message {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var messages []ai.Message
	for i := 0; i < len(h.turns); i++ {
		turn := h.turns[i]
		if turn.Hidden {
			continue
		}
		messages = append(messages, turn.Request)
		if turn.Reply != nil {
			messages = append(messages, turn.Reply)
		}
	}
	return messages
}

// AppendTurn adds a turn to the history
func (h *ConversationHistory) AppendTurn(turn ConversationTurn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = append(h.turns, turn)
}

// AppendEntry adds a turn to the history (for backward compatibility)
func (h *ConversationHistory) AppendEntry(turn ConversationTurn) {
	h.AppendTurn(turn)
}

// Clear removes all turns from the history
func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]ConversationTurn, 0)
}

// RemoveAt removes the turn at the specified index
func (h *ConversationHistory) RemoveAt(index int) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if index < 0 || index >= len(h.turns) {
		return &errIndexOutOfRange{index: index, length: len(h.turns)}
	}

	h.turns = append(h.turns[:index], h.turns[index+1:]...)
	return nil
}

// SetTurns replaces the entire history with new turns
func (h *ConversationHistory) SetTurns(turns []ConversationTurn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]ConversationTurn, len(turns))
	copy(h.turns, turns)
}

// SetEntries replaces the entire history with new turns (for backward compatibility)
func (h *ConversationHistory) SetEntries(turns []ConversationTurn) {
	h.SetTurns(turns)
}

// Len returns the number of turns in the history
func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.turns)
}

// FindByTraceFile finds all turns matching the given trace file
func (h *ConversationHistory) FindByTraceFile(traceFile string) []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []ConversationTurn
	for _, turn := range h.turns {
		if turn.TraceFile == traceFile {
			result = append(result, turn)
		}
	}
	return result
}

// FindByRunID finds all turns matching the given run ID
func (h *ConversationHistory) FindByRunID(runID string) []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []ConversationTurn
	for _, turn := range h.turns {
		if turn.RunID == runID {
			result = append(result, turn)
		}
	}
	return result
}

// GetByRunID returns the first turn matching the given run ID and an error if not found
func (h *ConversationHistory) GetByRunID(runID string) (*ConversationTurn, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for i := range h.turns {
		if h.turns[i].RunID == runID {
			result := h.turns[i]
			return &result, nil
		}
	}
	return nil, &errEntryNotFound{runID: runID}
}

// HideByRunID marks all turns with the given run ID as hidden
func (h *ConversationHistory) HideByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	for i := range h.turns {
		if h.turns[i].RunID == runID {
			h.turns[i].Hidden = true
			found = true
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	return nil
}

// UnhideByRunID marks all turns with the given run ID as visible
func (h *ConversationHistory) UnhideByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	for i := range h.turns {
		if h.turns[i].RunID == runID {
			h.turns[i].Hidden = false
			found = true
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	return nil
}

// DeleteByRunID removes all turns with the given run ID from the history
func (h *ConversationHistory) DeleteByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	filtered := make([]ConversationTurn, 0, len(h.turns))
	for _, turn := range h.turns {
		if turn.RunID == runID {
			found = true
		} else {
			filtered = append(filtered, turn)
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	h.turns = filtered
	return nil
}

type errEntryNotFound struct {
	runID string
}

func (e *errEntryNotFound) Error() string {
	return fmt.Sprintf("entry with runID %s not found", e.runID)
}

type errIndexOutOfRange struct {
	index  int
	length int
}

func (e *errIndexOutOfRange) Error() string {
	return fmt.Sprintf("index %d out of range for length %d", e.index, e.length)
}
