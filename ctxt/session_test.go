package ctxt

import (
	"path/filepath"
	"testing"

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

	sessionPath := ctx.ExecutionEnvironment().RootDir
	loadedCtx, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if loadedCtx.ID() != ctx.id {
		t.Errorf("expected ID %s, got %s", ctx.id, loadedCtx.ID())
	}
	if loadedCtx.Name() != "Test Session" {
		t.Errorf("expected Name 'Test Session', got '%s'", loadedCtx.Name())
	}
	if loadedCtx.Summary() != "Test summary" {
		t.Errorf("expected Summary 'Test summary', got '%s'", loadedCtx.Summary())
	}
	if loadedCtx.description != "test description" {
		t.Errorf("expected description 'test description', got '%s'", loadedCtx.description)
	}
	if loadedCtx.instructions != "test instructions" {
		t.Errorf("expected instructions 'test instructions', got '%s'", loadedCtx.instructions)
	}
}

func TestContextAutoSave(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	sessionPath := ctx.ExecutionEnvironment().RootDir

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

	ctx1, err := New("id1", "desc1", "inst1", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx1.SetName("Session 1")
	ctx1.SetSummary("Summary 1")

	ctx2, err := New("id2", "desc2", "inst2", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx2.SetName("Session 2")
	ctx2.SetSummary("Summary 2")

	ctx3, err := New("id3", "desc3", "inst3", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx3.SetName("Session 3")
	ctx3.SetSummary("Summary 3")

	loadedCtx1, err := LoadContext(ctx1.ExecutionEnvironment().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 1: %v", err)
	}
	if loadedCtx1.Name() != "Session 1" {
		t.Errorf("expected Name 'Session 1', got '%s'", loadedCtx1.Name())
	}
	if loadedCtx1.Summary() != "Summary 1" {
		t.Errorf("expected Summary 'Summary 1', got '%s'", loadedCtx1.Summary())
	}

	loadedCtx2, err := LoadContext(ctx2.ExecutionEnvironment().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 2: %v", err)
	}
	if loadedCtx2.Name() != "Session 2" {
		t.Errorf("expected Name 'Session 2', got '%s'", loadedCtx2.Name())
	}

	loadedCtx3, err := LoadContext(ctx3.ExecutionEnvironment().RootDir)
	if err != nil {
		t.Fatalf("failed to load context 3: %v", err)
	}
	if loadedCtx3.Name() != "Session 3" {
		t.Errorf("expected Name 'Session 3', got '%s'", loadedCtx3.Name())
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

	ctx.StartTurn("Hello")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Hi there"})

	ctx.StartTurn("How are you?")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "I'm fine"})

	sessionPath := ctx.ExecutionEnvironment().RootDir
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

func TestReloadedContextMatchesOriginal(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("parity-test", "desc", "instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}
	ctx.SetName("Parity Test")
	ctx.SetSummary("Test summary")

	ctx.StartTurn("First question")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "First answer"})

	ctx.StartTurn("Second question")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Second answer"})

	origTurns := ctx.GetHistory().GetTurns()
	if len(origTurns) != 2 {
		t.Fatalf("expected 2 original turns, got %d", len(origTurns))
	}

	sessionPath := ctx.ExecutionEnvironment().RootDir
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

	msgs := loadedCtx.GetHistory().GetMessages()
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

	ctx.StartTurn("Q1")
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

	ctx.SetMeta("executive-assistant", "executive-assistant-1.0.0")
	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	sessionPath := ctx.ExecutionEnvironment().RootDir
	loaded, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if got := loaded.RunAgentName(); got != "executive-assistant" {
		t.Errorf("RunAgentName: expected %q, got %q", "executive-assistant", got)
	}
	if got := loaded.RunPackageID(); got != "executive-assistant-1.0.0" {
		t.Errorf("RunPackageID: expected %q, got %q", "executive-assistant-1.0.0", got)
	}
	if loaded.RunStartedAt().IsZero() {
		t.Error("RunStartedAt: expected non-zero timestamp")
	}
}

func TestLoadContextMetadataWithHistory(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("meta-history", "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.SetMeta("support-agent", "support-agent-1.0.0")
	ctx.StartTurn("Hello")
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Hi"})

	sessionPath := ctx.ExecutionEnvironment().RootDir
	loaded, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if got := loaded.RunAgentName(); got != "support-agent" {
		t.Errorf("RunAgentName after reload: expected %q, got %q", "support-agent", got)
	}
	if got := loaded.RunPackageID(); got != "support-agent-1.0.0" {
		t.Errorf("RunPackageID after reload: expected %q, got %q", "support-agent-1.0.0", got)
	}
	if loaded.GetHistory().Len() != 1 {
		t.Errorf("expected 1 turn in history, got %d", loaded.GetHistory().Len())
	}
}

func TestLoadContextNoRunMeta(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("legacy-no-meta", "desc", "inst", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.runMeta = nil
	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	sessionPath := ctx.ExecutionEnvironment().RootDir
	loaded, err := LoadContext(sessionPath)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	if got := loaded.RunAgentName(); got != "" {
		t.Errorf("RunAgentName (legacy): expected empty, got %q", got)
	}
	if got := loaded.RunPackageID(); got != "" {
		t.Errorf("RunPackageID (legacy): expected empty, got %q", got)
	}
}
