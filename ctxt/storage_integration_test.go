package ctxt

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestStorageIntegrationMultiTurnReload(t *testing.T) {
	base := t.TempDir()
	runID := NewRunID(time.Now().UTC())
	ctx, err := New(runID, "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetName("integration")
	ctx.StartTurn("q1", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "a1"})
	ctx.StartTurn("q2", `{"k":"v"}`)
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "a2"})

	loaded, err := LoadContext(ctx.Workspace().RootDir)
	if err != nil {
		t.Fatal(err)
	}
	turns := loaded.GetHistory().GetTurns()
	if len(turns) != 2 {
		t.Fatalf("turns=%d", len(turns))
	}
	if turns[1].UserData != `{"k":"v"}` {
		t.Fatalf("userdata=%q", turns[1].UserData)
	}
	sessions, err := ListSessions(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Turns != 2 {
		t.Fatalf("sessions=%+v", sessions)
	}
}

func TestStorageIntegrationClearHistory(t *testing.T) {
	base := t.TempDir()
	ctx, err := New(NewRunID(time.Now().UTC()), "d", "i", base)
	if err != nil {
		t.Fatal(err)
	}
	ctx.StartTurn("q", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "a"})
	ctx.ClearHistory()
	if ctx.GetHistory().Len() != 0 {
		t.Fatal("expected empty history")
	}
	logPath := conversationPathForPrivateDir(filepath.Join(ctx.Workspace().RootDir, aigenticDirName))
	refs, _ := LoadConversationRefs(logPath)
	if len(refs) != 0 {
		t.Fatalf("refs=%v", refs)
	}
}
