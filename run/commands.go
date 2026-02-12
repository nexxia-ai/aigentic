package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/event"
)

const cmdContext = "context"

type childTurnStats struct {
	TurnID    string    `json:"turn_id"`
	Timestamp time.Time `json:"timestamp"`
	AgentName string    `json:"agent_name"`
	Usage     ai.Usage  `json:"usage,omitempty"`
}

type runMetaStats struct {
	AgentName string `json:"agent_name"`
}

func discoverChildTurns(ws *ctxt.Workspace) []childTurnStats {
	if ws == nil {
		return nil
	}
	root := ws.RootDir
	batchBase := filepath.Join(root, "_private", "batch")
	planBase := filepath.Join(root, "_private", "plan")
	var out []childTurnStats
	for _, base := range []string{batchBase, planBase} {
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}
		for _, e1 := range entries {
			if !e1.IsDir() {
				continue
			}
			subDir := filepath.Join(base, e1.Name())
			subEntries, err := os.ReadDir(subDir)
			if err != nil {
				continue
			}
			for _, e2 := range subEntries {
				if !e2.IsDir() {
					continue
				}
				childPrivateDir := filepath.Join(subDir, e2.Name())
				turnDir := filepath.Join(childPrivateDir, "turn")
				agentName := loadRunMetaAgentName(childPrivateDir)
				turns := loadTurnsFromTurnDir(turnDir)
				for i := range turns {
					if turns[i].AgentName == "" {
						turns[i].AgentName = agentName
					}
					out = append(out, turns[i])
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

func loadRunMetaAgentName(privateDir string) string {
	data, err := os.ReadFile(filepath.Join(privateDir, "run_meta.json"))
	if err != nil {
		return ""
	}
	var m runMetaStats
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return m.AgentName
}

func loadTurnsFromTurnDir(turnDir string) []childTurnStats {
	entries, err := os.ReadDir(turnDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return nil
	}
	var turnDirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "turn-") {
			turnDirs = append(turnDirs, e.Name())
		}
	}
	sort.Strings(turnDirs)
	var turns []childTurnStats
	for _, name := range turnDirs {
		path := filepath.Join(turnDir, name, "turn.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}
		var t childTurnStats
		if json.Unmarshal(data, &t) != nil {
			continue
		}
		turns = append(turns, t)
	}
	return turns
}

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
	childTurns := discoverChildTurns(r.agentContext.Workspace())

	var totalIn, totalOut, totalCached, totalReasoning int
	var lastUsage ai.Usage

	for i := range turns {
		u := turns[i].Usage
		totalIn += u.PromptTokens
		totalOut += u.CompletionTokens
		totalCached += u.PromptTokensDetails.CachedTokens
		totalReasoning += u.CompletionTokensDetails.ReasoningTokens
	}
	for _, c := range childTurns {
		totalIn += c.Usage.PromptTokens
		totalOut += c.Usage.CompletionTokens
		totalCached += c.Usage.PromptTokensDetails.CachedTokens
		totalReasoning += c.Usage.CompletionTokensDetails.ReasoningTokens
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
	childByParent := assignChildTurnsToParents(turns, lastTurns, childTurns)

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
	for _, children := range childByParent {
		for _, c := range children {
			if c.Usage.PromptTokensDetails.CachedTokens > 0 {
				showCached = true
			}
			if c.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
				showReasoning = true
			}
		}
	}

	wTime := 10
	wTurn := 26
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
		for _, c := range childByParent[i] {
			ctm := c.Timestamp.Format("3:04 PM")
			turnLabel := "  " + c.AgentName + "/" + c.TurnID
			cline := fmt.Sprintf("%-*s %-*s  %*d %*d", wTime, ctm, wTurn, turnLabel, wNum, c.Usage.PromptTokens, wNum, c.Usage.CompletionTokens)
			if showCached {
				cline += fmt.Sprintf(" %*d", wNum, c.Usage.PromptTokensDetails.CachedTokens)
			}
			if showReasoning {
				cline += fmt.Sprintf(" %*d", wNum+2, c.Usage.CompletionTokensDetails.ReasoningTokens)
			}
			b.WriteString(cline + "\n")
		}
	}
	b.WriteString("```\n")

	return b.String()
}

func assignChildTurnsToParents(turns []ctxt.Turn, lastTurns []ctxt.Turn, childTurns []childTurnStats) [][]childTurnStats {
	n := len(lastTurns)
	if n == 0 {
		return nil
	}
	childByParent := make([][]childTurnStats, n)
	base := len(turns) - n
	for _, c := range childTurns {
		parentIdx := -1
		for i := 0; i < len(turns); i++ {
			if !c.Timestamp.Before(turns[i].Timestamp) {
				if i+1 >= len(turns) || c.Timestamp.Before(turns[i+1].Timestamp) {
					parentIdx = i
					break
				}
			}
		}
		if parentIdx < 0 {
			continue
		}
		lastIdx := parentIdx - base
		if lastIdx >= 0 && lastIdx < n {
			childByParent[lastIdx] = append(childByParent[lastIdx], c)
		}
	}
	return childByParent
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
