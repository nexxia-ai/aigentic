package ctxt

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

type conversationFile struct {
	TurnRefs []string `json:"turn_refs"`
}

type ConversationHistory struct {
	turnRefs         []string
	conversationPath string
	ledger           *Ledger
	mutex            sync.RWMutex
}

func NewConversationHistory(ledger *Ledger, conversationPath string) *ConversationHistory {
	h := &ConversationHistory{
		turnRefs:         make([]string, 0),
		conversationPath: conversationPath,
		ledger:           ledger,
	}
	if ledger != nil && conversationPath != "" {
		if refs, _ := LoadConversationRefs(conversationPath); refs != nil {
			h.turnRefs = refs
		}
	}
	return h
}

// LoadConversationRefs reads turn_refs from a conversation.json file.
func LoadConversationRefs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cf conversationFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	if cf.TurnRefs == nil {
		return nil, nil
	}
	return cf.TurnRefs, nil
}

func (h *ConversationHistory) saveConversation() {
	if h.ledger == nil || h.conversationPath == "" {
		return
	}
	dir := filepath.Dir(h.conversationPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create conversation dir", "dir", dir, "error", err)
		return
	}
	h.mutex.RLock()
	refs := make([]string, len(h.turnRefs))
	copy(refs, h.turnRefs)
	h.mutex.RUnlock()

	cf := conversationFile{TurnRefs: refs}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		slog.Error("failed to marshal conversation", "error", err)
		return
	}
	if err := os.WriteFile(h.conversationPath, data, 0644); err != nil {
		slog.Error("failed to write conversation", "path", h.conversationPath, "error", err)
	}
}

func (h *ConversationHistory) Ledger() *Ledger {
	return h.ledger
}

func (h *ConversationHistory) SetTurnLimit(limit int) {
	_ = limit
}

func (h *ConversationHistory) resolveTurns(limit int) []Turn {
	h.mutex.RLock()
	refs := h.turnRefs
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
		t, err := h.ledger.Get(refs[i])
		if err != nil {
			slog.Warn("failed to resolve turn", "turnID", refs[i], "error", err)
			continue
		}
		turns = append(turns, *t)
	}
	return turns
}

func (h *ConversationHistory) getMessages(limit int, ac *AgentContext) []ai.Message {
	turns := h.resolveTurns(limit)
	var messages []ai.Message
	for _, turn := range turns {
		if turn.Hidden {
			continue
		}
		if turn.Request != nil {
			messages = append(messages, turn.Request)
		} else if turn.UserMessage != "" || turn.UserData != "" {
			if ac != nil {
				userMsg, err := createUserMsgForTurn(ac, &turn)
				if err == nil {
					messages = append(messages, userMsg)
				}
			} else {
				messages = append(messages, ai.UserMessage{Role: ai.UserRole, Content: turn.UserMessage})
			}
		}
		if turn.Reply != nil {
			messages = append(messages, turn.Reply)
		}
	}
	return messages
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
	h.saveConversation()
}

func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	h.turnRefs = make([]string, 0)
	h.mutex.Unlock()
	h.saveConversation()
}

func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.turnRefs)
}

func (h *ConversationHistory) FindByTraceFile(traceFile string) []Turn {
	turns := h.resolveTurns(0)
	var result []Turn
	for _, turn := range turns {
		if turn.TraceFile == traceFile {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) GetTurns() []Turn {
	return h.resolveTurns(0)
}

func (h *ConversationHistory) Last(n int) []Turn {
	turns := h.resolveTurns(0)
	if n > 0 && len(turns) > n {
		turns = turns[len(turns)-n:]
	}
	return turns
}

func (h *ConversationHistory) FilterByAgent(name string) []Turn {
	turns := h.resolveTurns(0)
	var result []Turn
	for _, turn := range turns {
		if turn.AgentName == name {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) ExcludeHidden() []Turn {
	turns := h.resolveTurns(0)
	var result []Turn
	for _, turn := range turns {
		if !turn.Hidden {
			result = append(result, turn)
		}
	}
	return result
}
