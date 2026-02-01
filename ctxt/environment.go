package ctxt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic/document"
)

const memoryStoreName = "memory_files"

type ExecutionEnvironment struct {
	RootDir    string
	LLMDir     string
	PrivateDir string
	MemoryDir  string
	UploadDir  string
	OutputDir  string
	TurnDir    string
}

func NewExecutionEnvironment(baseDir, agentID string) (*ExecutionEnvironment, error) {
	baseDir, _ = filepath.Abs(baseDir)

	timestamp := time.Now().Format("060102150405")
	rootDir := filepath.Join(baseDir, fmt.Sprintf("%s-%s", timestamp, agentID))
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private")

	e := &ExecutionEnvironment{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
		TurnDir:    filepath.Join(privateDir, "turns"),
	}
	if err := e.init(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *ExecutionEnvironment) SetMemoryDir(dir string) error {
	if dir == "" {
		e.MemoryDir = ""
		return nil
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("memory dir: %w", err)
	}
	absLLM, err := filepath.Abs(e.LLMDir)
	if err != nil {
		return fmt.Errorf("llm dir: %w", err)
	}
	rel, err := filepath.Rel(absLLM, absDir)
	if err != nil {
		return fmt.Errorf("memory dir not under LLM dir: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("memory dir must be under LLM dir: %s", dir)
	}
	e.MemoryDir = absDir
	if err := os.MkdirAll(e.MemoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory dir %s: %w", e.MemoryDir, err)
	}
	return nil
}

func (e *ExecutionEnvironment) init() error {
	dirs := []string{
		e.RootDir,
		e.LLMDir,
		e.PrivateDir,
		e.UploadDir,
		e.OutputDir,
		e.TurnDir,
	}
	if e.MemoryDir != "" {
		dirs = append(dirs, e.MemoryDir)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (e *ExecutionEnvironment) MemoryFiles() ([]*document.Document, error) {
	if e.MemoryDir == "" {
		return nil, nil
	}
	s := document.NewLocalStore(e.MemoryDir)
	storeID := s.ID()

	_, exists := document.GetStore(storeID)
	if !exists {
		if err := document.RegisterStore(s); err != nil {
			return nil, fmt.Errorf("failed to register memory store: %w", err)
		}
	}

	ctx := context.Background()
	return document.List(ctx, storeID)
}

func (e *ExecutionEnvironment) EnvVars() map[string]string {
	if e == nil {
		return make(map[string]string)
	}
	m := map[string]string{
		"AGENT_ROOT_DIR":    e.RootDir,
		"AGENT_LLM_DIR":     e.LLMDir,
		"AGENT_PRIVATE_DIR": e.PrivateDir,
		"AGENT_UPLOAD_DIR":  e.UploadDir,
		"AGENT_OUTPUT_DIR":  e.OutputDir,
		"AGENT_TURN_DIR":    e.TurnDir,
	}
	if e.MemoryDir != "" {
		m["AGENT_MEMORY_DIR"] = e.MemoryDir
	}
	return m
}

func (e *ExecutionEnvironment) MemoryStoreName() string {
	return memoryStoreName
}

func loadExecutionEnvironment(sessionDir string) (*ExecutionEnvironment, error) {
	sessionDir, err := filepath.Abs(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	rootDir := sessionDir
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private")

	e := &ExecutionEnvironment{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
		TurnDir:    filepath.Join(privateDir, "turns"),
	}

	return e, nil
}
