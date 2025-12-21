package run

import (
	"fmt"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

type ConversationHistory struct {
	turns []ConversationTurn
	mutex sync.RWMutex
}

func NewConversationHistory() *ConversationHistory {
	return &ConversationHistory{
		turns: make([]ConversationTurn, 0),
	}
}

func (h *ConversationHistory) GetTurns() []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]ConversationTurn, len(h.turns))
	copy(result, h.turns)
	return result
}

func (h *ConversationHistory) GetEntries() []ConversationTurn {
	return h.GetTurns()
}

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

func (h *ConversationHistory) AppendTurn(turn ConversationTurn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = append(h.turns, turn)
}

func (h *ConversationHistory) AppendEntry(turn ConversationTurn) {
	h.AppendTurn(turn)
}

func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]ConversationTurn, 0)
}

func (h *ConversationHistory) RemoveAt(index int) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if index < 0 || index >= len(h.turns) {
		return &errIndexOutOfRange{index: index, length: len(h.turns)}
	}

	h.turns = append(h.turns[:index], h.turns[index+1:]...)
	return nil
}

func (h *ConversationHistory) SetTurns(turns []ConversationTurn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]ConversationTurn, len(turns))
	copy(h.turns, turns)
}

func (h *ConversationHistory) SetEntries(turns []ConversationTurn) {
	h.SetTurns(turns)
}

func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.turns)
}

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
