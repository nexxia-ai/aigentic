package ctxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetMemoryDir_UnderLLMDir_Success(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	memDir := filepath.Join(e.LLMDir, "memory")
	if err := e.SetMemoryDir(memDir); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	if e.MemoryDir != memDir {
		t.Errorf("MemoryDir = %q, want %q", e.MemoryDir, memDir)
	}
	if _, err := os.Stat(memDir); err != nil {
		t.Errorf("memory dir should exist: %v", err)
	}
}

func TestSetMemoryDir_OutsideLLMDir_Error(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	otherDir := t.TempDir()
	if err := e.SetMemoryDir(otherDir); err == nil {
		t.Error("SetMemoryDir outside LLMDir should return error")
	}
	if e.MemoryDir != "" {
		t.Errorf("MemoryDir should remain empty, got %q", e.MemoryDir)
	}
}

func TestSetMemoryDir_Empty_Clears(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	if err := e.SetMemoryDir(filepath.Join(e.LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	if err := e.SetMemoryDir(""); err != nil {
		t.Fatalf("SetMemoryDir empty: %v", err)
	}
	if e.MemoryDir != "" {
		t.Errorf("MemoryDir = %q, want empty", e.MemoryDir)
	}
}

func TestMemoryFiles_EmptyMemoryDir_ReturnsNil(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	docs, err := e.MemoryFiles()
	if err != nil {
		t.Fatalf("MemoryFiles: %v", err)
	}
	if docs != nil {
		t.Errorf("MemoryFiles() = %v, want nil", docs)
	}
}

func TestEnvVars_EmptyMemoryDir_OmitsAGENT_MEMORY_DIR(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	m := e.EnvVars()
	if _, ok := m["AGENT_MEMORY_DIR"]; ok {
		t.Error("EnvVars should not include AGENT_MEMORY_DIR when MemoryDir is empty")
	}
}

func TestEnvVars_NonEmptyMemoryDir_IncludesAGENT_MEMORY_DIR(t *testing.T) {
	e, err := NewExecutionEnvironment(t.TempDir(), "agent1")
	if err != nil {
		t.Fatalf("NewExecutionEnvironment: %v", err)
	}
	if err := e.SetMemoryDir(filepath.Join(e.LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}
	m := e.EnvVars()
	if v, ok := m["AGENT_MEMORY_DIR"]; !ok || v == "" {
		t.Errorf("EnvVars should include AGENT_MEMORY_DIR, got %q", m["AGENT_MEMORY_DIR"])
	}
}
