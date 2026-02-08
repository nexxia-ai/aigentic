package run

import (
	"context"
	"fmt"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
)

// CompactHistory compacts old conversation turns into daily summaries.
// It uses the agent's model to generate summaries and delegates data
// management to ConversationHistory.
func (r *AgentRun) CompactHistory(ctx context.Context, config ctxt.CompactionConfig) (int, error) {
	days := r.agentContext.ConversationHistory().DaysToCompact(config)
	if len(days) == 0 {
		return 0, nil
	}

	archived := 0
	for _, day := range days {
		prompt := ctxt.DefaultSummaryPrompt(day.Turns, day.Date)
		reply, err := r.model.Call(ctx, []ai.Message{ai.UserMessage{Role: ai.UserRole, Content: prompt}}, nil)
		if err != nil {
			return archived, fmt.Errorf("summarize %s: %w", day.Date.Format("2006-01-02"), err)
		}

		content := reply.Content
		if content == "" && len(reply.Parts) > 0 {
			for _, p := range reply.Parts {
				if p.Type == ai.ContentPartText && p.Text != "" {
					content = p.Text
					break
				}
			}
		}

		summary := ctxt.CompactionSummary{
			Date:      day.Date,
			Summary:   content,
			TurnCount: len(day.Turns),
		}

		if err := r.agentContext.ConversationHistory().ArchiveDay(day, summary); err != nil {
			return archived, fmt.Errorf("archive %s: %w", day.Date.Format("2006-01-02"), err)
		}
		archived += len(day.Turns)
	}
	return archived, nil
}

// ShouldCompact checks if there are turns eligible for compaction.
func (r *AgentRun) ShouldCompact(config ctxt.CompactionConfig) bool {
	return r.agentContext.ShouldCompact(config)
}
