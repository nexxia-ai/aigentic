package ctxt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type conversationLogLine struct {
	TurnID string `json:"turn_id"`
}

type conversationState struct {
	Version    int    `json:"version"`
	Count      int    `json:"count"`
	LastTurnID string `json:"last_turn_id,omitempty"`
}

func conversationLogPath(privateDir string) string {
	return filepath.Join(privateDir, ConversationLogName)
}

func conversationStatePath(privateDir string) string {
	return filepath.Join(privateDir, ConversationStateName)
}

func appendConversationRef(logPath, turnID string) error {
	if turnID == "" {
		return nil
	}
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := appendJSONL(logPath, conversationLogLine{TurnID: turnID}); err != nil {
		return err
	}
	statePath := conversationStatePath(dir)
	count := 0
	var last string
	if refs, err := loadConversationRefsFromLog(logPath); err == nil {
		count = len(refs)
		if count > 0 {
			last = refs[count-1]
		}
	}
	st := conversationState{Version: storageVersion, Count: count, LastTurnID: last}
	return writeAtomicJSON(statePath, st)
}

func clearConversation(logPath string) error {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := writeAtomic(logPath, nil, 0644); err != nil {
		return err
	}
	st := conversationState{Version: storageVersion, Count: 0}
	return writeAtomicJSON(conversationStatePath(dir), st)
}

func loadConversationRefsFromLog(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	seen := make(map[string]int)
	var refs []string
	var corrupt bool
	err := readJSONLLines(path, func(line []byte, lineNum int, isLast bool) error {
		var row conversationLogLine
		if uerr := json.Unmarshal(line, &row); uerr != nil || row.TurnID == "" {
			if isLast {
				slog.Warn("skip corrupt final conversation log line", "path", path, "line", lineNum)
				return nil
			}
			corrupt = true
			if uerr != nil {
				return fmt.Errorf("conversation log line %d: %w", lineNum, uerr)
			}
			return fmt.Errorf("conversation log line %d: empty turn_id", lineNum)
		}
		if idx, ok := seen[row.TurnID]; ok {
			refs[idx] = row.TurnID
		} else {
			seen[row.TurnID] = len(refs)
			refs = append(refs, row.TurnID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = corrupt
	return refs, nil
}

// LoadConversationRefs reads turn IDs from conversation.log (append-only format).
func LoadConversationRefs(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return loadConversationRefsFromLog(path)
}

func conversationTurnCount(privateDir string) int {
	statePath := conversationStatePath(privateDir)
	data, err := os.ReadFile(statePath)
	if err == nil {
		var st conversationState
		if json.Unmarshal(data, &st) == nil {
			return st.Count
		}
	}
	refs, err := LoadConversationRefs(conversationLogPath(privateDir))
	if err != nil {
		return 0
	}
	return len(refs)
}
