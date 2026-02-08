package ctxt

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type ConversationHistory struct {
	turns     []Turn
	summaries []CompactionSummary
	mutex     sync.RWMutex
	workspace *Workspace
}

func NewConversationHistory(workspace *Workspace) *ConversationHistory {
	h := &ConversationHistory{
		turns:     make([]Turn, 0),
		summaries: make([]CompactionSummary, 0),
		workspace: workspace,
	}
	if workspace != nil {
		turns := loadTurnsFromDir(workspace.TurnDir)
		summaries, _ := workspace.LoadSummaries()
		h.mutex.Lock()
		h.turns = turns
		if summaries != nil {
			h.summaries = summaries
		}
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

	if h.workspace != nil {
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

func (h *ConversationHistory) GetSummaries() []CompactionSummary {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make([]CompactionSummary, len(h.summaries))
	copy(result, h.summaries)
	return result
}

func (h *ConversationHistory) DaysToCompact(config CompactionConfig) []DayGroup {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -config.KeepRecentDays)
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, cutoff.Location())

	byDate := make(map[string]DayGroup)
	for _, turn := range h.turns {
		if turn.Hidden {
			continue
		}
		day := time.Date(turn.Timestamp.Year(), turn.Timestamp.Month(), turn.Timestamp.Day(), 0, 0, 0, 0, turn.Timestamp.Location())
		if day.Before(cutoff) {
			key := day.Format("2006-01-02")
			g := byDate[key]
			g.Date = day
			g.Turns = append(g.Turns, turn)
			byDate[key] = g
		}
	}

	var result []DayGroup
	for _, g := range byDate {
		result = append(result, g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})
	return result
}

func (h *ConversationHistory) ArchiveDay(day DayGroup, summary CompactionSummary) error {
	if h.workspace == nil {
		return nil
	}

	h.mutex.Lock()
	turnIDs := make(map[string]bool)
	for _, t := range day.Turns {
		turnIDs[t.TurnID] = true
	}
	var kept []Turn
	for _, t := range h.turns {
		if !turnIDs[t.TurnID] {
			kept = append(kept, t)
		}
	}
	h.turns = kept
	h.summaries = append(h.summaries, summary)
	h.mutex.Unlock()

	if err := h.workspace.ArchiveTurns(day.Turns, day.Date); err != nil {
		return err
	}

	summaries := h.GetSummaries()
	if err := h.workspace.SaveSummaries(summaries); err != nil {
		return err
	}

	month := day.Date.Format("2006-01")
	existing, _ := h.workspace.LoadArchiveIndex(month)
	entries := make([]ArchiveIndexEntry, len(existing))
	copy(entries, existing)
	for _, t := range day.Turns {
		content := ""
		if t.Reply != nil {
			_, content = t.Reply.Value()
			if len(content) > 200 {
				content = content[:200] + "..."
			}
		}
		entries = append(entries, ArchiveIndexEntry{
			TurnID:      t.TurnID,
			Date:       day.Date,
			UserMessage: t.UserMessage,
			Summary:    content,
		})
	}
	return h.workspace.SaveArchiveIndex(month, entries)
}
