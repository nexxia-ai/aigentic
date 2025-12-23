package ctxt

import (
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

func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]ConversationTurn, 0)
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
