package ctxt

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

type ConversationHistory struct {
	turnRefs         []string
	conversationPath string
	ledger           *Ledger
	turnLimit        int
	byteBudget       int
	mutex            sync.RWMutex
}

func NewConversationHistory(ledger *Ledger, conversationPath string) *ConversationHistory {
	h := &ConversationHistory{
		turnRefs:         make([]string, 0),
		conversationPath: conversationPath,
		ledger:           ledger,
		turnLimit:        promptHistoryTurnLimit,
		byteBudget:       0,
	}
	if ledger != nil && conversationPath != "" {
		if refs, _ := LoadConversationRefs(conversationPath); refs != nil {
			h.turnRefs = refs
		}
	}
	return h
}

func (h *ConversationHistory) Ledger() *Ledger {
	return h.ledger
}

func (h *ConversationHistory) SetTurnLimit(limit int) {
	h.mutex.Lock()
	h.turnLimit = limit
	h.mutex.Unlock()
}

func (h *ConversationHistory) SetBudget(limit int, byteBudget int) {
	h.mutex.Lock()
	h.turnLimit = limit
	h.byteBudget = byteBudget
	h.mutex.Unlock()
}

func (h *ConversationHistory) resolveTurnsFull() []Turn {
	h.mutex.RLock()
	refs := make([]string, len(h.turnRefs))
	copy(refs, h.turnRefs)
	h.mutex.RUnlock()

	if h.ledger == nil || len(refs) == 0 {
		return nil
	}
	var turns []Turn
	for _, ref := range refs {
		t, err := h.ledger.Get(ref)
		if err != nil {
			slog.Warn("failed to resolve turn", "turnID", ref, "error", err)
			continue
		}
		turns = append(turns, *t)
	}
	return turns
}

func (h *ConversationHistory) resolveTurnsForPrompt(limit int) []Turn {
	h.mutex.RLock()
	refs := make([]string, len(h.turnRefs))
	copy(refs, h.turnRefs)
	h.mutex.RUnlock()

	if h.ledger == nil || len(refs) == 0 {
		return nil
	}
	start := 0
	if limit > 0 && len(refs) > limit {
		start = len(refs) - limit
	}
	var turns []Turn
	for i := start; i < len(refs); i++ {
		t, err := h.ledger.Head(refs[i])
		if err != nil {
			slog.Warn("failed to resolve turn head", "turnID", refs[i], "error", err)
			continue
		}
		turns = append(turns, *t)
	}
	return turns
}

func (h *ConversationHistory) getMessages(limit int, ac *AgentContext) []ai.Message {
	h.mutex.RLock()
	if limit <= 0 {
		limit = h.turnLimit
	}
	byteBudget := h.byteBudget
	h.mutex.RUnlock()

	turns := h.resolveTurnsForPrompt(limit)
	var selected [][]ai.Message
	usedBytes := 0
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn.Hidden {
			continue
		}
		var turnMessages []ai.Message
		if turn.Request != nil {
			turnMessages = append(turnMessages, turn.Request)
		} else if turn.RequestSnapshot != nil {
			turnMessages = append(turnMessages, turn.RequestSnapshot)
		} else if turn.UserMessage != "" || turn.UserData != "" {
			if ac != nil {
				userMsg, err := createUserMsgForTurn(ac, &turn)
				if err == nil {
					turnMessages = append(turnMessages, userMsg)
				}
			} else {
				turnMessages = append(turnMessages, ai.UserMessage{Role: ai.UserRole, Content: turn.UserMessage})
			}
		}
		if turn.Reply != nil {
			turnMessages = append(turnMessages, turn.Reply)
		}
		if len(turnMessages) == 0 {
			continue
		}
		turnBytes := messagesByteSize(turnMessages)
		if byteBudget > 0 && len(selected) > 0 && usedBytes+turnBytes > byteBudget {
			continue
		}
		usedBytes += turnBytes
		selected = append(selected, turnMessages)
	}

	var messages []ai.Message
	for i := len(selected) - 1; i >= 0; i-- {
		messages = append(messages, selected[i]...)
	}
	return messages
}

func messagesByteSize(messages []ai.Message) int {
	total := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		_, content := msg.Value()
		total += len(content)
	}
	return total
}

func (h *ConversationHistory) GetMessages(ac *AgentContext) []ai.Message {
	return h.getMessages(0, ac)
}

func (h *ConversationHistory) appendTurn(turn Turn) {
	if h.ledger == nil {
		return
	}
	if err := h.ledger.Append(&turn); err != nil {
		slog.Error("failed to append turn to ledger", "turnID", turn.TurnID, "error", err)
		return
	}
	h.mutex.Lock()
	h.turnRefs = append(h.turnRefs, turn.TurnID)
	h.mutex.Unlock()
	if h.conversationPath != "" {
		if err := appendConversationRef(h.conversationPath, turn.TurnID); err != nil {
			slog.Error("failed to append conversation ref", "turnID", turn.TurnID, "error", err)
		}
	}
}

func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	h.turnRefs = make([]string, 0)
	h.mutex.Unlock()
	if h.conversationPath != "" {
		if err := clearConversation(h.conversationPath); err != nil {
			slog.Error("failed to clear conversation", "path", h.conversationPath, "error", err)
		}
	}
}

func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.turnRefs)
}

func (h *ConversationHistory) FindByTraceFile(traceFile string) []Turn {
	turns := h.resolveTurnsFull()
	var result []Turn
	for _, turn := range turns {
		if turn.TraceFile == traceFile {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) GetTurns() []Turn {
	return h.resolveTurnsFull()
}

func (h *ConversationHistory) Last(n int) []Turn {
	h.mutex.RLock()
	refs := make([]string, len(h.turnRefs))
	copy(refs, h.turnRefs)
	h.mutex.RUnlock()
	if h.ledger == nil || len(refs) == 0 {
		return nil
	}
	start := 0
	if n > 0 && len(refs) > n {
		start = len(refs) - n
	}
	var turns []Turn
	for i := start; i < len(refs); i++ {
		t, err := h.ledger.Get(refs[i])
		if err != nil {
			continue
		}
		turns = append(turns, *t)
	}
	return turns
}

func (h *ConversationHistory) FilterByAgent(name string) []Turn {
	turns := h.resolveTurnsFull()
	var result []Turn
	for _, turn := range turns {
		if turn.AgentName == name {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) ExcludeHidden() []Turn {
	turns := h.resolveTurnsFull()
	var result []Turn
	for _, turn := range turns {
		if !turn.Hidden {
			result = append(result, turn)
		}
	}
	return result
}

func conversationPathForPrivateDir(privateDir string) string {
	return filepath.Join(privateDir, ConversationLogName)
}
