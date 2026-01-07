package ctxt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexxia-ai/aigentic/document"
)

type ExecutionEnvironment struct {
	RootDir    string
	LLMDir     string
	PrivateDir string
	MemoryDir  string
	FilesDir   string
	OutputDir  string
	HistoryDir string
	TurnDir    string
}

func NewExecutionEnvironment(baseDir, agentID string) (*ExecutionEnvironment, error) {
	baseDir, _ = filepath.Abs(baseDir)

	rootDir := filepath.Join(baseDir, fmt.Sprintf("agent-%s", agentID))
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private")

	e := &ExecutionEnvironment{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  filepath.Join(privateDir, "memory"),
		FilesDir:   filepath.Join(llmDir, "files"),
		OutputDir:  filepath.Join(llmDir, "output"),
		HistoryDir: filepath.Join(privateDir, "history"),
		TurnDir:    filepath.Join(privateDir, "turns"),
	}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ExecutionEnvironment) init() error {
	dirs := []string{
		e.RootDir,
		e.LLMDir,
		e.PrivateDir,
		e.MemoryDir,
		e.FilesDir,
		e.OutputDir,
		e.HistoryDir,
		e.TurnDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (e *ExecutionEnvironment) MemoryFiles() ([]*document.Document, error) {
	s := document.NewLocalStore(e.MemoryDir)
	return s.List(context.Background())
}

func (e *ExecutionEnvironment) EnvVars() map[string]string {
	if e == nil {
		return make(map[string]string)
	}
	return map[string]string{
		"AGENT_ROOT_DIR":    e.RootDir,
		"AGENT_LLM_DIR":     e.LLMDir,
		"AGENT_PRIVATE_DIR": e.PrivateDir,
		"AGENT_MEMORY_DIR":  e.MemoryDir,
		"AGENT_FILES_DIR":   e.FilesDir,
		"AGENT_OUTPUT_DIR":  e.OutputDir,
		"AGENT_HISTORY_DIR": e.HistoryDir,
		"AGENT_TURN_DIR":    e.TurnDir,
	}
}
