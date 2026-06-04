package ctxt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func testTurnFixture(turnID, agentName string, files []FileRef) *Turn {
	if files == nil {
		files = []FileRef{}
	}
	turn := &Turn{
		TurnID:      turnID,
		RunID:       "20250318-runid123",
		UserMessage: "test",
		AgentName:   agentName,
		Timestamp:   time.Date(2025, 3, 18, 12, 0, 0, 0, time.UTC),
		Files:       files,
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "ok"},
		systemTags:  make([]TagEntry, 0),
		turnTags:    make([]ai.KeyValue, 0),
	}
	turn.AddMessage(turn.Reply)
	return turn
}

func benchmarkBaseDir(b *testing.B) string {
	if dir := os.Getenv("AIGENTIC_BENCH_DIR"); dir != "" {
		return dir
	}
	return b.TempDir()
}

func buildWorkspace(b *testing.B, numRuns, turnsPerRun int, msgSize int) string {
	b.Helper()
	base := benchmarkBaseDir(b)
	now := time.Now().UTC()
	for i := 0; i < numRuns; i++ {
		id := NewRunID(now.Add(time.Duration(i) * time.Second))
		ctx, err := New(id, "desc", "inst", base)
		if err != nil {
			b.Fatal(err)
		}
		ctx.SetName("run-" + id)
		for j := 0; j < turnsPerRun; j++ {
			ctx.StartTurn("user", "")
			reply := heavyAIMessage(msgSize)
			ctx.EndTurn(reply)
		}
	}
	return base
}

func heavyAIMessage(size int) ai.AIMessage {
	content := make([]byte, size)
	for i := range content {
		content[i] = 'x'
	}
	return ai.AIMessage{
		Role:    ai.AssistantRole,
		Content: string(content),
		Response: ai.Response{
			Usage: ai.Usage{PromptTokens: 10, CompletionTokens: 10},
		},
	}
}

func buildLegacyWorkspace(b *testing.B, numRuns, turnsPerRun int) string {
	b.Helper()
	base := benchmarkBaseDir(b)
	now := time.Now().UTC()
	for i := 0; i < numRuns; i++ {
		id := NewRunID(now.Add(time.Duration(i) * time.Second))
		runDir := RunDir(base, id)
		private := filepath.Join(runDir, aigenticDirName)
		_ = os.MkdirAll(private, 0755)
		_ = os.MkdirAll(filepath.Join(runDir, "llm"), 0755)
		ctxData := map[string]string{"id": id, "name": "legacy", "summary": "s"}
		data, _ := json.Marshal(ctxData)
		_ = os.WriteFile(filepath.Join(private, "context.json"), data, 0644)
		var refs []string
		ledger := NewLedger(base)
		for j := 0; j < turnsPerRun; j++ {
			turnID, dir, _ := ledger.PrepareTurn(now)
			turn := testTurnFixture(turnID, "a", nil)
			turn.Reply = heavyAIMessage(256)
			payload, _ := json.Marshal(turn)
			_ = os.WriteFile(filepath.Join(dir, "turn.json"), payload, 0644)
			refs = append(refs, turnID)
		}
		cf := map[string][]string{"turn_refs": refs}
		cdata, _ := json.Marshal(cf)
		_ = os.WriteFile(filepath.Join(private, "conversation.json"), cdata, 0644)
	}
	return base
}

func buildSingleRunWithTurns(b *testing.B, turns, msgSize int) (*AgentContext, string) {
	b.Helper()
	base := benchmarkBaseDir(b)
	id := NewRunID(time.Now().UTC())
	ctx, err := New(id, "d", "i", base)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < turns; i++ {
		ctx.StartTurn("q", "")
		ctx.EndTurn(heavyAIMessage(msgSize))
	}
	return ctx, base
}
