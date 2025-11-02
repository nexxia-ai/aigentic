package aigentic

import (
	"fmt"
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

// HistoryEntry represents a complete conversation turn
type HistoryEntry struct {
	UserMessage      ai.Message
	AssistantMessage ai.Message
	ToolMessages     []ai.Message
	TraceFile        string
	RunID            string
	Timestamp        time.Time
	AgentName        string
	Hidden           bool
}

// ConversationHistory stores conversation history with metadata for trace correlation
type ConversationHistory struct {
	entries []HistoryEntry
	mutex   sync.RWMutex
}

// NewConversationHistory creates a new conversation history object
func NewConversationHistory() *ConversationHistory {
	return &ConversationHistory{
		entries: make([]HistoryEntry, 0),
	}
}

// GetEntries returns a copy of all history entries
func (h *ConversationHistory) GetEntries() []HistoryEntry {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]HistoryEntry, len(h.entries))
	copy(result, h.entries)
	return result
}

// GetMessages returns all messages flattened for LLM context (user, assistant, tools in order)
// Hidden entries are excluded from the result
func (h *ConversationHistory) GetMessages() []ai.Message {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var messages []ai.Message
	for _, entry := range h.entries {
		if entry.Hidden {
			continue
		}
		messages = append(messages, entry.UserMessage)
		messages = append(messages, entry.ToolMessages...)
		messages = append(messages, entry.AssistantMessage)
	}
	return messages
}

// AppendEntry adds an entry to the history
func (h *ConversationHistory) AppendEntry(entry HistoryEntry) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.entries = append(h.entries, entry)
}

// Clear removes all entries from the history
func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.entries = make([]HistoryEntry, 0)
}

// RemoveAt removes the entry at the specified index
func (h *ConversationHistory) RemoveAt(index int) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if index < 0 || index >= len(h.entries) {
		return &errIndexOutOfRange{index: index, length: len(h.entries)}
	}

	h.entries = append(h.entries[:index], h.entries[index+1:]...)
	return nil
}

// SetEntries replaces the entire history with new entries
func (h *ConversationHistory) SetEntries(entries []HistoryEntry) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.entries = make([]HistoryEntry, len(entries))
	copy(h.entries, entries)
}

// Len returns the number of entries in the history
func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.entries)
}

// FindByTraceFile finds all entries matching the given trace file
func (h *ConversationHistory) FindByTraceFile(traceFile string) []HistoryEntry {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []HistoryEntry
	for _, entry := range h.entries {
		if entry.TraceFile == traceFile {
			result = append(result, entry)
		}
	}
	return result
}

// FindByRunID finds all entries matching the given run ID
func (h *ConversationHistory) FindByRunID(runID string) []HistoryEntry {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []HistoryEntry
	for _, entry := range h.entries {
		if entry.RunID == runID {
			result = append(result, entry)
		}
	}
	return result
}

// GetByRunID returns the first entry matching the given run ID and an error if not found
func (h *ConversationHistory) GetByRunID(runID string) (*HistoryEntry, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for i := range h.entries {
		if h.entries[i].RunID == runID {
			result := h.entries[i]
			return &result, nil
		}
	}
	return nil, &errEntryNotFound{runID: runID}
}

// HideByRunID marks all entries with the given run ID as hidden
func (h *ConversationHistory) HideByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	for i := range h.entries {
		if h.entries[i].RunID == runID {
			h.entries[i].Hidden = true
			found = true
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	return nil
}

// UnhideByRunID marks all entries with the given run ID as visible
func (h *ConversationHistory) UnhideByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	for i := range h.entries {
		if h.entries[i].RunID == runID {
			h.entries[i].Hidden = false
			found = true
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	return nil
}

// DeleteByRunID removes all entries with the given run ID from the history
func (h *ConversationHistory) DeleteByRunID(runID string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	found := false
	filtered := make([]HistoryEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		if entry.RunID == runID {
			found = true
		} else {
			filtered = append(filtered, entry)
		}
	}
	if !found {
		return &errEntryNotFound{runID: runID}
	}
	h.entries = filtered
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
