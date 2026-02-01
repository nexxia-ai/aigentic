package ctxt

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

type ConversationHistory struct {
	turns   []Turn
	mutex   sync.RWMutex
	execEnv *ExecutionEnvironment
}

func NewConversationHistory(execEnv *ExecutionEnvironment) *ConversationHistory {
	h := &ConversationHistory{
		turns:   make([]Turn, 0),
		execEnv: execEnv,
	}
	if execEnv != nil {
		turns := loadTurnsFromDir(execEnv.TurnDir)
		h.mutex.Lock()
		h.turns = turns
		h.mutex.Unlock()
	}
	return h
}

func loadTurnsFromDir(turnDir string) []Turn {
	entries, err := os.ReadDir(turnDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Warn("failed to read turns directory", "dir", turnDir, "error", err)
		return nil
	}
	var turnDirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "turn-") {
			turnDirs = append(turnDirs, e.Name())
		}
	}
	sort.Strings(turnDirs)

	var turns []Turn
	for _, name := range turnDirs {
		path := filepath.Join(turnDir, name, "turn.json")
		var turn Turn
		if err := turn.loadFromFile(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			slog.Warn("failed to load turn file", "path", path, "error", err)
			continue
		}
		turns = append(turns, turn)
	}
	return turns
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
		if turn.Request != nil {
			messages = append(messages, turn.Request)
		} else if turn.UserMessage != "" {
			messages = append(messages, ai.UserMessage{Role: ai.UserRole, Content: turn.UserMessage})
		}
		if turn.Reply != nil {
			messages = append(messages, turn.Reply)
		}
	}
	return messages
}

func (h *ConversationHistory) appendTurn(turn Turn) {
	h.mutex.Lock()
	h.turns = append(h.turns, turn)
	h.mutex.Unlock()

	if h.execEnv != nil {
		turn.saveToFile()
	}
}

func (h *ConversationHistory) Clear() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.turns = make([]Turn, 0)
}

func (h *ConversationHistory) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.turns)
}

func (h *ConversationHistory) FindByTraceFile(traceFile string) []Turn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []Turn
	for _, turn := range h.turns {
		if turn.TraceFile == traceFile {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) GetTurns() []Turn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]Turn, len(h.turns))
	copy(result, h.turns)
	return result
}

func (h *ConversationHistory) Last(n int) []Turn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	turns := h.turns
	if n > 0 && len(turns) > n {
		turns = turns[len(turns)-n:]
	}

	result := make([]Turn, len(turns))
	copy(result, turns)
	return result
}

func (h *ConversationHistory) FilterByAgent(name string) []Turn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []Turn
	for _, turn := range h.turns {
		if turn.AgentName == name {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) ExcludeHidden() []Turn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []Turn
	for _, turn := range h.turns {
		if !turn.Hidden {
			result = append(result, turn)
		}
	}
	return result
}
