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
	MemoryDir  string
	FilesDir   string
	OutputDir  string
	HistoryDir string
	TurnDir    string
}

func NewExecutionEnvironment(baseDir, agentID string) (*ExecutionEnvironment, error) {
	baseDir, _ = filepath.Abs(baseDir)

	rootDir := filepath.Join(baseDir, fmt.Sprintf("agent-%s", agentID))
	e := &ExecutionEnvironment{
		RootDir:    rootDir,
		MemoryDir:  filepath.Join(rootDir, "memory"),
		FilesDir:   filepath.Join(rootDir, "files"),
		OutputDir:  filepath.Join(rootDir, "output"),
		HistoryDir: filepath.Join(rootDir, "history"),
		TurnDir:    filepath.Join(rootDir, "turns"),
	}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ExecutionEnvironment) init() error {
	dirs := []string{e.RootDir, e.MemoryDir, e.FilesDir, e.OutputDir, e.HistoryDir, e.TurnDir}
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
		"AGENT_MEMORY_DIR":  e.MemoryDir,
		"AGENT_FILES_DIR":   e.FilesDir,
		"AGENT_OUTPUT_DIR":  e.OutputDir,
		"AGENT_HISTORY_DIR": e.HistoryDir,
		"AGENT_TURN_DIR":    e.TurnDir,
	}
}
