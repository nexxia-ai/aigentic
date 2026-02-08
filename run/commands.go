package run

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
)

const cmdContext = "context"

func parseAigenticCommand(message string) (rest string, cmd string, ok bool) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "/"+cmdContext {
		return "", cmdContext, true
	}
	return message, "", false
}

func (r *AgentRun) handleAigenticCommand(ctx context.Context, cmd string) {
	content := r.formatAigenticStats()
	r.queueEvent(&event.ContentEvent{
		RunID:     r.id,
		AgentName: r.AgentName(),
		SessionID: r.sessionID,
		Content:   content,
	})
	r.queueAction(&stopAction{Error: nil})
}

func (r *AgentRun) formatAigenticStats() string {
	agentName := r.AgentName()
	if agentName == "" {
		agentName = "unknown"
	}

	modelName := "unknown"
	if r.model != nil {
		modelName = r.model.ModelName
		if r.model.ContextSize != nil && *r.model.ContextSize > 0 {
			modelName = fmt.Sprintf("%s (%dK context)", modelName, *r.model.ContextSize/1000)
		}
	}

	var b strings.Builder
	b.WriteString("aigentic stats\n\n")
	b.WriteString(fmt.Sprintf("Agent: %s  \n", agentName))
	b.WriteString(fmt.Sprintf("Model: %s\n\n", modelName))

	history := r.agentContext.GetHistory()
	if history == nil || history.Len() == 0 {
		b.WriteString("No turns completed yet.\n")
		return b.String()
	}

	turns := history.GetTurns()
	turnCount := len(turns)

	var totalIn, totalOut, totalCached, totalReasoning int
	var lastUsage ai.Usage

	for i := range turns {
		u := turns[i].Usage
		totalIn += u.PromptTokens
		totalOut += u.CompletionTokens
		totalCached += u.PromptTokensDetails.CachedTokens
		totalReasoning += u.CompletionTokensDetails.ReasoningTokens
	}
	if len(turns) > 0 {
		lastUsage = turns[len(turns)-1].Usage
	}

	b.WriteString(fmt.Sprintf("run total: %s  \n", formatSessionTotal(totalIn, totalOut, totalReasoning)))
	b.WriteString(fmt.Sprintf("last turn: %s  \n", formatUsage(lastUsage)))
	b.WriteString(fmt.Sprintf("Turns: %d\n\n", turnCount))

	b.WriteString("Context breakdown\n\n")
	sysTok, memTok := r.agentContext.EstimateSystemAndMemoryTokens()

	b.WriteString(fmt.Sprintf("system:  ~%d\n", sysTok))
	b.WriteString(fmt.Sprintf("memory:  ~%d\n\n", memTok))

	lastTurns := history.Last(20)
	showCached := false
	showReasoning := false
	for _, t := range lastTurns {
		if t.Usage.PromptTokensDetails.CachedTokens > 0 {
			showCached = true
		}
		if t.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
			showReasoning = true
		}
	}

	wTime := 10
	wTurn := 12
	wNum := 10

	b.WriteString("```\n")
	header := fmt.Sprintf("%-*s %-*s  %*s %*s", wTime, "Time", wTurn, "Turn", wNum, "in:", wNum, "out:")
	if showCached {
		header += fmt.Sprintf(" %*s", wNum, "cached:")
	}
	if showReasoning {
		header += fmt.Sprintf(" %*s", wNum+2, "reasoning:")
	}
	b.WriteString(header + "\n")

	for i := len(lastTurns) - 1; i >= 0; i-- {
		t := lastTurns[i]
		tm := t.Timestamp.Format("3:04 PM")
		line := fmt.Sprintf("%-*s %-*s  %*d %*d", wTime, tm, wTurn, t.TurnID, wNum, t.Usage.PromptTokens, wNum, t.Usage.CompletionTokens)
		if showCached {
			line += fmt.Sprintf(" %*d", wNum, t.Usage.PromptTokensDetails.CachedTokens)
		}
		if showReasoning {
			line += fmt.Sprintf(" %*d", wNum+2, t.Usage.CompletionTokensDetails.ReasoningTokens)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("```\n")

	return b.String()
}

func formatUsage(u ai.Usage) string {
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return "in: 0 out: 0"
	}
	parts := []string{fmt.Sprintf("in: %d out: %d", u.PromptTokens, u.CompletionTokens)}
	if u.CompletionTokensDetails.ReasoningTokens > 0 {
		parts = append(parts, fmt.Sprintf("reasoning: %d", u.CompletionTokensDetails.ReasoningTokens))
	}
	if u.PromptTokensDetails.CachedTokens > 0 {
		parts = append(parts, fmt.Sprintf("cached: %d", u.PromptTokensDetails.CachedTokens))
	}
	return strings.Join(parts, " ")
}

func formatSessionTotal(in, out, reasoning int) string {
	if reasoning > 0 {
		return fmt.Sprintf("in: %d out: %d reasoning: %d", in, out, reasoning)
	}
	return fmt.Sprintf("in: %d out: %d", in, out)
}
