package ctxt

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestContextSaveAndLoad(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx.SetName("Test Session")
	ctx.SetSummary("Test summary")

	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save context: %v", err)
	}

	sessionPath := ctx.Workspace().RootDir
	loadedCtx, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if loadedCtx.ID() != ctx.ID() {
		t.Errorf("expected ID %s, got %s", ctx.ID(), loadedCtx.ID())
	}
	if loadedCtx.Name() != "Test Session" {
		t.Errorf("expected Name 'Test Session', got '%s'", loadedCtx.Name())
	}
	if loadedCtx.Summary() != "Test summary" {
		t.Errorf("expected Summary 'Test summary', got '%s'", loadedCtx.Summary())
	}
	if desc, ok := loadedCtx.PromptPart(SystemPartKeyDescription); !ok || desc != "test description" {
		t.Errorf("expected description 'test description', got %q (ok=%v)", desc, ok)
	}
	if inst, ok := loadedCtx.PromptPart(SystemPartKeyInstructions); !ok || inst != "test instructions" {
		t.Errorf("expected instructions 'test instructions', got %q (ok=%v)", inst, ok)
	}
}

func TestContextSaveAndLoadPreservesGoal(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "d", "i", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx.SetSystemPart(SystemPartKeyGoal, "Help the user achieve the outcome")
	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save context: %v", err)
	}
	loaded, err := LoadContext(ctx.Workspace().RootDir)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	g, ok := loaded.PromptPart(SystemPartKeyGoal)
	if !ok || g != "Help the user achieve the outcome" {
		t.Errorf("expected goal preserved, got %q (ok=%v)", g, ok)
	}
}

func TestContextAutoSave(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	sessionPath := ctx.Workspace().RootDir

	ctx.SetName("Test Name")
	loadedCtx1, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	if loadedCtx1.Name() != "Test Name" {
		t.Errorf("expected Name 'Test Name', got '%s'", loadedCtx1.Name())
	}

	ctx.SetSummary("Test Summary")
	loadedCtx2, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	if loadedCtx2.Summary() != "Test Summary" {
		t.Errorf("expected Summary 'Test Summary', got '%s'", loadedCtx2.Summary())
	}
}

func TestListSessions(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Now()

	ctx1, err := New(NewRunID(now), "desc1", "inst1", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx1.SetName("Session 1")
	ctx1.SetSummary("Summary 1")

	ctx2, err := New(NewRunID(now.Add(time.Second)), "desc2", "inst2", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx2.SetName("Session 2")
	ctx2.SetSummary("Summary 2")

	ctx3, err := New(NewRunID(now.Add(2*time.Second)), "desc3", "inst3", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx3.SetName("Session 3")
	ctx3.SetSummary("Summary 3")

	loadedCtx1, err := LoadContext(ctx1.Workspace().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 1: %v", err)
	}
	if loadedCtx1.Name() != "Session 1" {
		t.Errorf("expected Name 'Session 1', got '%s'", loadedCtx1.Name())
	}
	if loadedCtx1.Summary() != "Summary 1" {
		t.Errorf("expected Summary 'Summary 1', got '%s'", loadedCtx1.Summary())
	}

	loadedCtx2, err := LoadContext(ctx2.Workspace().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 2: %v", err)
	}
	if loadedCtx2.Name() != "Session 2" {
		t.Errorf("expected Name 'Session 2', got '%s'", loadedCtx2.Name())
	}

	loadedCtx3, err := LoadContext(ctx3.Workspace().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 3: %v", err)
	}
	if loadedCtx3.Name() != "Session 3" {
		t.Errorf("expected Name 'Session 3', got '%s'", loadedCtx3.Name())
	}

	inactiveRunID := NewRunID(now.Add(3 * time.Second))
	ctxInactive, err := New(inactiveRunID, "ina", "ina", baseDir)
	if err != nil {
		t.Fatalf("inactive context: %v", err)
	}
	ctxInactive.SetMeta("run_state", "inactive")
	if err := ctxInactive.save(); err != nil {
		t.Fatalf("save inactive: %v", err)
	}

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 active sessions, got %d", len(sessions))
	}
	for _, s := range sessions {
		if s.ID == inactiveRunID {
			t.Fatal("inactive session should be excluded from default ListSessions")
		}
	}

	allSessions, err := ListSessions(baseDir, ListSessionsOptions{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListSessions include archived: %v", err)
	}
	if len(allSessions) != 4 {
		t.Fatalf("expected 4 sessions when including archived, got %d", len(allSessions))
	}
	var foundInactive bool
	for _, s := range allSessions {
		if s.ID == inactiveRunID {
			foundInactive = true
			break
		}
	}
	if !foundInactive {
		t.Fatal("inactive session missing when IncludeArchived is true")
	}
}

func TestListSessionsEmpty(t *testing.T) {
	baseDir := t.TempDir()

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestLoadContextWithHistory(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.StartTurn("Hello", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Hi there"})

	ctx.StartTurn("How are you?", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "I'm fine"})

	sessionPath := ctx.Workspace().RootDir
	loadedCtx, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	history := loadedCtx.GetHistory()
	if history.Len() != 2 {
		t.Errorf("expected 2 turns in history, got %d", history.Len())
	}

	turns := history.GetTurns()
	if len(turns) != 2 {
		t.Errorf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].UserMessage != "Hello" {
		t.Errorf("expected first turn UserMessage 'Hello', got %q", turns[0].UserMessage)
	}
	if turns[1].UserMessage != "How are you?" {
		t.Errorf("expected second turn UserMessage 'How are you?', got %q", turns[1].UserMessage)
	}
	if turns[0].Reply == nil {
		t.Error("expected first turn Reply to be set")
	} else if aiMsg, ok := turns[0].Reply.(ai.AIMessage); ok && aiMsg.Content != "Hi there" {
		t.Errorf("expected first turn Reply content 'Hi there', got %q", aiMsg.Content)
	}
	if turns[1].Reply == nil {
		t.Error("expected second turn Reply to be set")
	} else if aiMsg, ok := turns[1].Reply.(ai.AIMessage); ok && aiMsg.Content != "I'm fine" {
		t.Errorf("expected second turn Reply content 'I'm fine', got %q", aiMsg.Content)
	}
}

func TestLoadContextWithHistory_UserMessageAndUserDataReloaded(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.StartTurn("From: alice@example.com | Subject: Meeting tomorrow", `{"type":"mail.received","from":"alice@example.com","subject":"Meeting tomorrow"}`)
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "I'll add it to your calendar."})

	ctx.StartTurn("Second message", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Got it."})

	sessionPath := ctx.Workspace().RootDir
	loadedCtx, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	turns := loadedCtx.GetHistory().GetTurns()
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].UserMessage != "From: alice@example.com | Subject: Meeting tomorrow" {
		t.Errorf("turn 0 UserMessage = %q, want display message", turns[0].UserMessage)
	}
	wantData := `{"type":"mail.received","from":"alice@example.com","subject":"Meeting tomorrow"}`
	if turns[0].UserData != wantData {
		t.Errorf("turn 0 UserData = %q, want %q", turns[0].UserData, wantData)
	}
	if turns[1].UserMessage != "Second message" {
		t.Errorf("turn 1 UserMessage = %q, want 'Second message'", turns[1].UserMessage)
	}
	if turns[1].UserData != "" {
		t.Errorf("turn 1 UserData = %q, want empty", turns[1].UserData)
	}
}

func TestReloadedContextMatchesOriginal(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("parity-test", "desc", "instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx.SetName("Parity Test")
	ctx.SetSummary("Test summary")

	ctx.StartTurn("First question", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "First answer"})

	ctx.StartTurn("Second question", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Second answer"})

	origTurns := ctx.GetHistory().GetTurns()
	if len(origTurns) != 2 {
		t.Fatalf("expected 2 original turns, got %d", len(origTurns))
	}

	sessionPath := ctx.Workspace().RootDir
	loadedCtx, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if loadedCtx.ID() != ctx.id {
		t.Errorf("expected ID %s, got %s", ctx.id, loadedCtx.ID())
	}
	if loadedCtx.Name() != ctx.Name() {
		t.Errorf("expected Name %q, got %q", ctx.Name(), loadedCtx.Name())
	}
	if loadedCtx.Summary() != ctx.Summary() {
		t.Errorf("expected Summary %q, got %q", ctx.Summary(), loadedCtx.Summary())
	}

	loadedTurns := loadedCtx.GetHistory().GetTurns()
	if len(loadedTurns) != len(origTurns) {
		t.Errorf("expected %d turns after reload, got %d", len(origTurns), len(loadedTurns))
	}
	for i := range origTurns {
		if loadedTurns[i].TurnID != origTurns[i].TurnID {
			t.Errorf("turn %d: expected TurnID %q, got %q", i, origTurns[i].TurnID, loadedTurns[i].TurnID)
		}
		if loadedTurns[i].UserMessage != origTurns[i].UserMessage {
			t.Errorf("turn %d: expected UserMessage %q, got %q", i, origTurns[i].UserMessage, loadedTurns[i].UserMessage)
		}
	}

	msgs := loadedCtx.GetHistory().GetMessages(loadedCtx)
	for _, m := range msgs {
		if m == nil {
			t.Error("GetMessages returned nil message")
		}
	}

	_, err = loadedCtx.BuildPrompt(nil, true)
	if err != nil {
		t.Errorf("BuildPrompt with history failed: %v", err)
	}
}

func TestGetTurnsReturnsRehydratedHistory(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("reload-test", "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.StartTurn("Q1", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "A1"})

	turns := ctx.GetHistory().GetTurns()
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].TurnID == "" {
		t.Error("expected TurnID to be set")
	}
	if turns[0].UserMessage != "Q1" {
		t.Errorf("expected UserMessage 'Q1', got %q", turns[0].UserMessage)
	}
	if turns[0].Reply == nil {
		t.Error("expected Reply to be set")
	} else if aiMsg, ok := turns[0].Reply.(ai.AIMessage); !ok || aiMsg.Content != "A1" {
		t.Errorf("expected Reply content 'A1', got %q", turns[0].Reply)
	}
}

func TestLoadContextNonExistent(t *testing.T) {
	baseDir := t.TempDir()
	nonExistentDir := filepath.Join(baseDir, "agent-nonexistent")

	_, err := LoadContext(nonExistentDir)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestLoadContextRestoresMetadata(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("meta-test", "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.SetMeta("agent_name", "executive-assistant")
	ctx.SetMeta("package_id", "executive-assistant-1.0.0")
	ctx.SetMeta("started_at", time.Now().Format(time.RFC3339))
	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	sessionPath := ctx.Workspace().RootDir
	loaded, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if v, ok := loaded.GetMeta("agent_name"); !ok || v.(string) != "executive-assistant" {
		t.Errorf("agent_name: expected executive-assistant, got %v", v)
	}
	if v, ok := loaded.GetMeta("package_id"); !ok || v.(string) != "executive-assistant-1.0.0" {
		t.Errorf("package_id: expected executive-assistant-1.0.0, got %v", v)
	}
	if v, ok := loaded.GetMeta("started_at"); !ok {
		t.Error("started_at: expected non-empty")
	} else if s, ok := v.(string); !ok || s == "" {
		t.Error("started_at: expected non-empty RFC3339 string")
	}
}

func TestLoadContextMetadataWithHistory(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("meta-history", "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.SetMeta("agent_name", "support-agent")
	ctx.SetMeta("package_id", "support-agent-1.0.0")
	ctx.StartTurn("Hello", "")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Hi"})

	sessionPath := ctx.Workspace().RootDir
	loaded, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if v, ok := loaded.GetMeta("agent_name"); !ok || v.(string) != "support-agent" {
		t.Errorf("agent_name after reload: expected support-agent, got %v", v)
	}
	if v, ok := loaded.GetMeta("package_id"); !ok || v.(string) != "support-agent-1.0.0" {
		t.Errorf("package_id after reload: expected support-agent-1.0.0, got %v", v)
	}
	if loaded.GetHistory().Len() != 1 {
		t.Errorf("expected 1 turn in history, got %d", loaded.GetHistory().Len())
	}
}

func TestFindSessionRequiresShardedRunID(t *testing.T) {
	baseDir := t.TempDir()
	if _, err := FindSession(baseDir, "invalid-run-id"); err == nil {
		t.Fatal("expected error for invalid run id")
	}
}

func TestNewRunIDUsesZeroShardForZeroTime(t *testing.T) {
	runID := NewRunID(time.Time{})
	if got := RunIDShard(runID); got != "00000000" {
		t.Fatalf("expected zero-time shard %q, got %q", "00000000", got)
	}
}

func TestFindSessionAllowsMissingRunMeta(t *testing.T) {
	baseDir := t.TempDir()
	runID := NewRunID(time.Now())
	ctx, err := New(runID, "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.runMeta = nil
	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	session, err := FindSession(baseDir, ctx.ID())
	if err != nil {
		t.Fatalf("expected session lookup to succeed without metadata: %v", err)
	}
	if session.ID != ctx.ID() {
		t.Fatalf("expected session ID %q, got %q", ctx.ID(), session.ID)
	}
	if len(session.Meta) > 0 {
		t.Fatalf("expected empty metadata when run_meta.json is missing, got %v", session.Meta)
	}
}
