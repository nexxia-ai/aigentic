package ctxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetMemoryDir_UnderLLMDir_Success(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	memDir := filepath.Join(w.LLMDir, "memory")
	if err := w.SetMemoryDir(memDir); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	if w.MemoryDir != memDir {
		t.Errorf("MemoryDir = %q, want %q", w.MemoryDir, memDir)
	}
	if _, err := os.Stat(memDir); err != nil {
		t.Errorf("memory dir should exist: %v", err)
	}
}

func TestSetMemoryDir_OutsideLLMDir_Error(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	otherDir := t.TempDir()
	if err := w.SetMemoryDir(otherDir); err == nil {
		t.Error("SetMemoryDir outside LLMDir should return error")
	}
	if w.MemoryDir != "" {
		t.Errorf("MemoryDir should remain empty, got %q", w.MemoryDir)
	}
}

func TestSetMemoryDir_Empty_Clears(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if err := w.SetMemoryDir(filepath.Join(w.LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	if err := w.SetMemoryDir(""); err != nil {
		t.Fatalf("SetMemoryDir empty: %v", err)
	}
	if w.MemoryDir != "" {
		t.Errorf("MemoryDir = %q, want empty", w.MemoryDir)
	}
}

func TestMemoryFiles_EmptyMemoryDir_ReturnsNil(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	docs, err := w.MemoryFiles()
	if err != nil {
		t.Fatalf("MemoryFiles: %v", err)
	}
	if docs != nil {
		t.Errorf("MemoryFiles() = %v, want nil", docs)
	}
}

func TestEnvVars_EmptyMemoryDir_OmitsAGENT_MEMORY_DIR(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	m := w.EnvVars()
	if _, ok := m["AGENT_MEMORY_DIR"]; ok {
		t.Error("EnvVars should not include AGENT_MEMORY_DIR when MemoryDir is empty")
	}
}

func TestEnvVars_NonEmptyMemoryDir_IncludesAGENT_MEMORY_DIR(t *testing.T) {
	w, err := NewWorkspace(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if err := w.SetMemoryDir(filepath.Join(w.LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	m := w.EnvVars()
	if v, ok := m["AGENT_MEMORY_DIR"]; !ok || v == "" {
		t.Errorf("EnvVars should include AGENT_MEMORY_DIR, got %q", m["AGENT_MEMORY_DIR"])
	}
}
