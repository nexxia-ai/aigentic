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
	ctx.AddMemory("mem1", "memory 1", "content 1")

	if err := ctx.save(); err != nil {
		t.Fatalf("failed to save context: %v", err)
	}

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	loadedCtx, err := LoadContext(sessions[0].Path)
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

	memories := loadedCtx.GetMemories()
	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}
	if len(memories) > 0 && memories[0].ID != "mem1" {
		t.Errorf("expected memory ID 'mem1', got '%s'", memories[0].ID)
	}
}

func TestContextAutoSave(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.SetName("Test Name")
	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	loadedCtx1, err := LoadContext(sessions[0].Path)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	if loadedCtx1.Name() != "Test Name" {
		t.Errorf("expected Name 'Test Name', got '%s'", loadedCtx1.Name())
	}

	ctx.SetSummary("Test Summary")
	sessions, err = ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	loadedCtx2, err := LoadContext(sessions[0].Path)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	if loadedCtx2.Summary() != "Test Summary" {
		t.Errorf("expected Summary 'Test Summary', got '%s'", loadedCtx2.Summary())
	}

	ctx.AddMemory("mem1", "memory 1", "content 1")
	sessions, err = ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	loadedCtx3, err := LoadContext(sessions[0].Path)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}
	memories := loadedCtx3.GetMemories()
	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
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

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	sessionMap := make(map[string]Session)
	for _, s := range sessions {
		sessionMap[s.ID] = s
	}

	if s1, ok := sessionMap["id1"]; ok {
		if s1.Name != "Session 1" {
			t.Errorf("expected Name 'Session 1', got '%s'", s1.Name)
		}
		if s1.Summary != "Summary 1" {
			t.Errorf("expected Summary 'Summary 1', got '%s'", s1.Summary)
		}
	} else {
		t.Error("session id1 not found")
	}

	if s2, ok := sessionMap["id2"]; ok {
		if s2.Name != "Session 2" {
			t.Errorf("expected Name 'Session 2', got '%s'", s2.Name)
		}
	} else {
		t.Error("session id2 not found")
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

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	loadedCtx, err := LoadContext(sessions[0].Path)
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
}

func TestLoadContextWithMemories(t *testing.T) {
	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	ctx.AddMemory("mem1", "memory 1", "content 1")
	ctx.AddMemory("mem2", "memory 2", "content 2")
	ctx.AddMemory("mem3", "memory 3", "content 3")

	sessions, err := ListSessions(baseDir)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	loadedCtx, err := LoadContext(sessions[0].Path)
	if err != nil {
		t.Fatalf("failed to load context: %v", err)
	}

	memories := loadedCtx.GetMemories()
	if len(memories) != 3 {
		t.Errorf("expected 3 memories, got %d", len(memories))
	}

	memoryMap := make(map[string]MemoryEntry)
	for _, m := range memories {
		memoryMap[m.ID] = m
	}

	if mem1, ok := memoryMap["mem1"]; ok {
		if mem1.Description != "memory 1" {
			t.Errorf("expected description 'memory 1', got '%s'", mem1.Description)
		}
		if mem1.Content != "content 1" {
			t.Errorf("expected content 'content 1', got '%s'", mem1.Content)
		}
	} else {
		t.Error("memory mem1 not found")
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
