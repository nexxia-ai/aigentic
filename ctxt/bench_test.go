package ctxt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run: go test ./ctxt -bench=. -benchmem -count=3

func BenchmarkListSessions(b *testing.B) {
	base := buildWorkspace(b, 20, 5, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ListSessions(base)
	}
}

func BenchmarkListSessions_ThousandRuns(b *testing.B) {
	base := buildWorkspace(b, 1000, 5, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ListSessions(base)
	}
}

func BenchmarkListSessions_FiveThousandRuns(b *testing.B) {
	base := buildWorkspace(b, 5000, 2, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ListSessions(base)
	}
}

func BenchmarkListSessions_FiveThousandRuns_Cached(b *testing.B) {
	base := buildWorkspace(b, 5000, 2, 64)
	_, _ = ListSessions(base)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ListSessions(base)
	}
}

func BenchmarkListSessions_LegacyLayout(b *testing.B) {
	base := buildLegacyWorkspace(b, 20, 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = listSessionsLegacy(base, false)
	}
}

func BenchmarkListSessions_LegacyLayout_ThousandRuns(b *testing.B) {
	base := buildLegacyWorkspace(b, 1000, 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = listSessionsLegacy(base, false)
	}
}

func listSessionsLegacy(baseDir string, includeArchived bool) ([]Session, error) {
	runDirs, err := sessionRunDirs(baseDir)
	if err != nil {
		return nil, err
	}
	var sessions []Session
	for _, runDir := range runDirs {
		privateDir := filepath.Join(runDir, aigenticDirName)
		if !includeArchived && sessionRunMetaIndicatesArchived(privateDir) {
			continue
		}
		session, err := loadSession(runDir)
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}
	return sessions, nil
}

func BenchmarkFindSession(b *testing.B) {
	base := buildWorkspace(b, 5000, 2, 64)
	id := NewRunID(time.Now().UTC())
	ctx, _ := New(id, "d", "i", base)
	ctx.SetName("find")
	runID := ctx.ID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindSession(base, runID)
	}
}

func BenchmarkGetTurns(b *testing.B) {
	ctx, _ := buildSingleRunWithTurns(b, 100, 6000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.GetHistory().GetTurns()
	}
}

func BenchmarkGetTurns_LegacyLayout(b *testing.B) {
	base := benchmarkBaseDir(b)
	now := time.Now().UTC()
	id := NewRunID(now)
	runDir := RunDir(base, id)
	private := filepath.Join(runDir, aigenticDirName)
	_ = os.MkdirAll(private, 0755)
	ledger := NewLedger(base)
	var refs []string
	for j := 0; j < 100; j++ {
		turnID, dir, _ := ledger.PrepareTurn(now)
		turn := testTurnFixture(turnID, "a", nil)
		turn.Reply = heavyAIMessage(6000)
		payload, _ := json.Marshal(turn)
		_ = os.WriteFile(filepath.Join(dir, "turn.json"), payload, 0644)
		refs = append(refs, turnID)
	}
	convPath := filepath.Join(private, "conversation.json")
	cf := map[string][]string{"turn_refs": refs}
	cdata, _ := json.Marshal(cf)
	_ = os.WriteFile(convPath, cdata, 0644)
	h := NewConversationHistory(ledger, convPath)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.GetTurns()
	}
}

func BenchmarkGetMessages(b *testing.B) {
	ctx, _ := buildSingleRunWithTurns(b, 100, 6000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.GetHistory().GetMessages(ctx)
	}
}

func BenchmarkLedgerHead(b *testing.B) {
	ctx, _ := buildSingleRunWithTurns(b, 100, 6000)
	ledger := ctx.Ledger()
	refs := ctx.GetHistory().Len()
	h := ctx.GetHistory()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		turns := h.Last(refs)
		for _, t := range turns {
			_, _ = ledger.Head(t.TurnID)
		}
	}
}

func BenchmarkEndTurn(b *testing.B) {
	ctx, _ := buildSingleRunWithTurns(b, 30, 512)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.StartTurn("q", "")
		ctx.EndTurn(heavyAIMessage(512))
	}
}

func BenchmarkEndTurn_ThousandRunCatalog(b *testing.B) {
	base := buildWorkspace(b, 1000, 2, 64)
	ctx, _ := New(NewRunID(time.Now().UTC()), "d", "i", base)
	for i := 0; i < 99; i++ {
		ctx.StartTurn("q", "")
		ctx.EndTurn(heavyAIMessage(128))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.StartTurn("q", "")
		ctx.EndTurn(heavyAIMessage(128))
	}
}

func BenchmarkLoadContext(b *testing.B) {
	ctx, _ := buildSingleRunWithTurns(b, 100, 1024)
	runDir := ctx.Workspace().RootDir
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadContext(runDir)
	}
}

func BenchmarkListTurnArtifactsWithFeedback(b *testing.B) {
	base := benchmarkBaseDir(b)
	day := time.Date(2025, 6, 4, 0, 0, 0, 0, time.UTC)
	shard := filepath.Join(base, ledgerDir, day.Format("20060102"))
	_ = os.MkdirAll(shard, 0755)
	for i := 0; i < 1000; i++ {
		turnID := day.Format("20060102") + "-" + string(rune('a'+(i%26))) + "000000"
		meta := map[string]string{}
		if i < 50 {
			meta["feedback"] = "up"
		}
		mkTurnBench(b, shard, turnID, meta)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ListTurnArtifactsWithFeedback(base, day, 100)
	}
}

func mkTurnBench(b *testing.B, shard, turnID string, meta map[string]string) {
	b.Helper()
	turn := testTurnFixture(turnID, "a", nil)
	turn.SetMeta(meta)
	if err := saveTurn(filepath.Join(shard, turnID), turn); err != nil {
		b.Fatal(err)
	}
}
