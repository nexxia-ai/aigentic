package ctxt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/nexxia-ai/aigentic/ai"
)

type ConversationHistory struct {
	turns   []ConversationTurn
	mutex   sync.RWMutex
	execEnv *ExecutionEnvironment
}

func NewConversationHistory(execEnv *ExecutionEnvironment) *ConversationHistory {
	h := &ConversationHistory{
		turns:   make([]ConversationTurn, 0),
		execEnv: execEnv,
	}
	if execEnv != nil {
		if err := h.LoadFromFile(); err != nil {
			slog.Warn("failed to load history from file", "error", err)
		}
	}
	return h
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

func (h *ConversationHistory) appendTurn(turn ConversationTurn) {
	h.mutex.Lock()
	h.turns = append(h.turns, turn)
	h.mutex.Unlock()

	if h.execEnv != nil {
		h.appendTurnToFile(turn)
	}
}

func (h *ConversationHistory) appendTurnToFile(turn ConversationTurn) {

	historyFile := filepath.Join(h.execEnv.HistoryDir, "history.json")

	if err := os.MkdirAll(h.execEnv.HistoryDir, 0755); err != nil {
		slog.Error("failed to ensure history directory exists", "error", err)
		return
	}

	file, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("failed to open history file for appending", "error", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(turn); err != nil {
		slog.Error("failed to encode turn to history file", "error", err)
		return
	}

	if err := file.Sync(); err != nil {
		slog.Error("failed to sync history file", "error", err)
	}
}

func (h *ConversationHistory) LoadFromFile() error {
	if h.execEnv == nil {
		return nil
	}

	historyFile := filepath.Join(h.execEnv.HistoryDir, "history.json")
	file, err := os.Open(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var turn ConversationTurn
		if err := json.Unmarshal(line, &turn); err != nil {
			slog.Warn("failed to parse turn from history file", "error", err, "line", string(line))
			continue
		}

		h.turns = append(h.turns, turn)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read history file: %w", err)
	}

	return nil
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

func (h *ConversationHistory) GetTurns() []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]ConversationTurn, len(h.turns))
	copy(result, h.turns)
	return result
}

func (h *ConversationHistory) Last(n int) []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	turns := h.turns
	if n > 0 && len(turns) > n {
		turns = turns[len(turns)-n:]
	}

	result := make([]ConversationTurn, len(turns))
	copy(result, turns)
	return result
}

func (h *ConversationHistory) FilterByAgent(name string) []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []ConversationTurn
	for _, turn := range h.turns {
		if turn.AgentName == name {
			result = append(result, turn)
		}
	}
	return result
}

func (h *ConversationHistory) ExcludeHidden() []ConversationTurn {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var result []ConversationTurn
	for _, turn := range h.turns {
		if !turn.Hidden {
			result = append(result, turn)
		}
	}
	return result
}
