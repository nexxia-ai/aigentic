package ctxt

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestRepairWorkspaceRebuildsCatalog(t *testing.T) {
	base := t.TempDir()
	ctx, err := New(NewRunID(time.Now().UTC()), "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetName("session")
	ctx.StartTurn("hi", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "ok"})

	_ = os.Remove(catalogSnapshotPath(base))
	_ = os.Remove(catalogLogPath(base))

	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
	sessions, err := ListSessions(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions=%d", len(sessions))
	}
}

func TestRepairWorkspaceIdempotent(t *testing.T) {
	base := t.TempDir()
	ctx, err := New(NewRunID(time.Now().UTC()), "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetName("s")
	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
}

func TestRepairWorkspaceDoesNotDeleteLLMFiles(t *testing.T) {
	base := t.TempDir()
	ctx, err := New(NewRunID(time.Now().UTC()), "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	upload := filepath.Join(ctx.Workspace().UploadDir, "keep.txt")
	if err := os.WriteFile(upload, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(upload); err != nil {
		t.Fatal(err)
	}
}

func TestRepairTruncatedCatalogSnapshot(t *testing.T) {
	base := t.TempDir()
	ctx, err := New(NewRunID(time.Now().UTC()), "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetName("s")
	_ = os.WriteFile(catalogSnapshotPath(base), []byte("{"), 0644)
	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
	if _, err := ListSessions(base); err != nil {
		t.Fatal(err)
	}
}

func TestPersistOrderCrashSafe(t *testing.T) {
	base := t.TempDir()
	runID := NewRunID(time.Now().UTC())
	ctx, err := New(runID, "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.StartTurn("one", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "a1"})
	ctx.StartTurn("two", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "a2"})

	privateDir := filepath.Join(ctx.Workspace().RootDir, aigenticDirName)
	logPath := conversationPathForPrivateDir(privateDir)
	if err := os.WriteFile(logPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	if err := RepairWorkspace(base); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadContext(ctx.Workspace().RootDir)
	if err != nil {
		t.Fatal(err)
	}
	turns := loaded.GetHistory().GetTurns()
	if len(turns) != 2 {
		t.Fatalf("turns=%d", len(turns))
	}
	if turns[0].UserMessage != "one" || turns[1].UserMessage != "two" {
		t.Fatalf("order=%q %q", turns[0].UserMessage, turns[1].UserMessage)
	}
}
