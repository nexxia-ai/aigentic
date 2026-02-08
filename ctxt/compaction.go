package ctxt

import (
	"fmt"
	"strings"
	"time"
)

// CompactionSummary represents a summary of a compacted day's turns.
type CompactionSummary struct {
	Date      time.Time `json:"date"`
	Summary   string    `json:"summary"`
	TurnCount int       `json:"turn_count"`
}

// CompactionConfig configures compaction behavior.
type CompactionConfig struct {
	KeepRecentDays  int // Days of full turns to keep (default: 7)
	KeepSummaryDays int // Days of summaries to include in prompt (default: 90)
	CompactionHour  int // Only run compaction when current hour >= this (0 = anytime, default: 4)
	ReserveTokens   int // Reserve tokens for threshold check (future use)
}

// DayGroup is a group of turns for a single calendar day, pending compaction.
type DayGroup struct {
	Date  time.Time
	Turns []Turn
}

// ArchiveIndexEntry is metadata for searchable archived history, stored in date-partitioned indexes.
type ArchiveIndexEntry struct {
	TurnID      string    `json:"turn_id"`
	Date        time.Time `json:"date"`
	UserMessage string    `json:"user_message"`
	Summary     string    `json:"summary,omitempty"`
}

// DefaultSummaryPrompt returns the built-in prompt for summarizing a day's turns.
func DefaultSummaryPrompt(turns []Turn, date time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summarize the following conversation from %s in 2-4 concise paragraphs. Preserve key facts, decisions, and action items.\n\n", date.Format("2006-01-02")))
	for _, t := range turns {
		sb.WriteString("---\n")
		sb.WriteString("User: ")
		sb.WriteString(t.UserMessage)
		sb.WriteString("\n")
		if t.Reply != nil {
			_, content := t.Reply.Value()
			if content != "" {
				sb.WriteString("Assistant: ")
				sb.WriteString(content)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}
