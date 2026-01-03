package ctxt

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nexxia-ai/aigentic/document"
)

type ExecutionEnvironment struct {
	RootDir    string
	SessionDir string
	FilesDir   string
	OutputDir  string
	HistoryDir string
}

func NewExecutionEnvironment(baseDir, agentID string) (*ExecutionEnvironment, error) {
	baseDir, _ = filepath.Abs(baseDir)

	rootDir := filepath.Join(baseDir, fmt.Sprintf("agent-%s", agentID))
	e := &ExecutionEnvironment{
		RootDir:    rootDir,
		SessionDir: filepath.Join(rootDir, "session"),
		FilesDir:   filepath.Join(rootDir, "files"),
		OutputDir:  filepath.Join(rootDir, "output"),
		HistoryDir: filepath.Join(rootDir, "history"),
	}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ExecutionEnvironment) init() error {
	dirs := []string{e.RootDir, e.SessionDir, e.FilesDir, e.OutputDir, e.HistoryDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (e *ExecutionEnvironment) SessionFiles() ([]*document.Document, error) {

	docs := make([]*document.Document, 0)
	s := document.NewLocalStore(e.SessionDir)
	list, err := s.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list session files: %w", err)
	}

	for _, entry := range list {
		doc, err := s.Open(context.Background(), entry.FilePath)
		if err != nil {
			slog.Error("failed to open session file", "error", err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (e *ExecutionEnvironment) EnvVars() map[string]string {
	if e == nil {
		return make(map[string]string)
	}
	return map[string]string{
		"AGENT_ROOT_DIR":    e.RootDir,
		"AGENT_SESSION_DIR": e.SessionDir,
		"AGENT_FILES_DIR":   e.FilesDir,
		"AGENT_OUTPUT_DIR":  e.OutputDir,
		"AGENT_HISTORY_DIR": e.HistoryDir,
	}
}
