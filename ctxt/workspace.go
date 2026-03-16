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

// Workspace manages the filesystem layout and I/O operations for an agent's execution directory.
// It owns llm/ and _aigentic/.
type Workspace struct {
	RootDir    string
	LLMDir     string
	PrivateDir string
	MemoryDir  string
	UploadDir  string
	OutputDir  string
}

func NewWorkspace(baseDir, agentID string) (*Workspace, error) {
	baseDir, _ = filepath.Abs(baseDir)

	timestamp := time.Now().Format("060102150405")
	rootDir := filepath.Join(baseDir, fmt.Sprintf("%s-%s", timestamp, agentID))
	return newWorkspaceAt(rootDir)
}

// NewWorkspaceAtPath creates a workspace at an exact path without timestamp prefix.
// Used by the orchestrator for deterministic single-instance paths (.workspace/{userID}/{packageID}/).
func NewWorkspaceAtPath(exactPath string) (*Workspace, error) {
	absPath, err := filepath.Abs(exactPath)
	if err != nil {
		return nil, fmt.Errorf("workspace path: %w", err)
	}
	return newWorkspaceAt(absPath)
}

const aigenticDirName = "_aigentic"

// newWorkspaceAtRunDir creates a workspace for a persisted run directory.
// Run private state lives under runDir/_aigentic/.
func newWorkspaceAtRunDir(runDir string) (*Workspace, error) {
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return nil, fmt.Errorf("run dir: %w", err)
	}
	llmDir := filepath.Join(absRunDir, "llm")
	privateDir := filepath.Join(absRunDir, aigenticDirName)
	w := &Workspace{
		RootDir:    absRunDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
	}
	if err := w.init(); err != nil {
		return nil, err
	}
	return w, nil
}

// newChildWorkspace creates a workspace for a child agent that has its own _private/ directory
// but shares the parent's llm/ directory (uploads, output, memory).
func newChildWorkspace(privateDir, sharedLLMDir string) (*Workspace, error) {
	absPrivate, err := filepath.Abs(privateDir)
	if err != nil {
		return nil, fmt.Errorf("child private dir: %w", err)
	}
	absLLM, err := filepath.Abs(sharedLLMDir)
	if err != nil {
		return nil, fmt.Errorf("shared LLM dir: %w", err)
	}

	w := &Workspace{
		RootDir:    absPrivate,
		LLMDir:     absLLM,
		PrivateDir: absPrivate,
		MemoryDir:  "",
		UploadDir:  filepath.Join(absLLM, "uploads"),
		OutputDir:  filepath.Join(absLLM, "output"),
	}

	privateDirs := []string{absPrivate}
	for _, dir := range privateDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return w, nil
}

func newWorkspaceAt(rootDir string) (*Workspace, error) {
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private", "main")

	w := &Workspace{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
	}
	if err := w.init(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Workspace) SetMemoryDir(dir string) error {
	if dir == "" {
		w.MemoryDir = ""
		return nil
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("memory dir: %w", err)
	}
	absLLM, err := filepath.Abs(w.LLMDir)
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
	w.MemoryDir = absDir
	if err := os.MkdirAll(w.MemoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory dir %s: %w", w.MemoryDir, err)
	}
	return nil
}

func (w *Workspace) init() error {
	dirs := []string{
		w.RootDir,
		w.LLMDir,
		w.PrivateDir,
		w.UploadDir,
		w.OutputDir,
	}
	if w.MemoryDir != "" {
		dirs = append(dirs, w.MemoryDir)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (w *Workspace) MemoryFiles() ([]*document.Document, error) {
	if w.MemoryDir == "" {
		return nil, nil
	}
	s := document.NewLocalStore(w.MemoryDir)
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

func (w *Workspace) EnvVars() map[string]string {
	if w == nil {
		return make(map[string]string)
	}
	m := map[string]string{
		"AGENT_ROOT_DIR":    w.RootDir,
		"AGENT_LLM_DIR":     w.LLMDir,
		"AGENT_PRIVATE_DIR": w.PrivateDir,
		"AGENT_UPLOAD_DIR":  w.UploadDir,
		"AGENT_OUTPUT_DIR":  w.OutputDir,
	}
	if w.MemoryDir != "" {
		m["AGENT_MEMORY_DIR"] = w.MemoryDir
	}
	return m
}

func (w *Workspace) MemoryStoreName() string {
	return memoryStoreName
}

func loadWorkspace(runDir string) (*Workspace, error) {
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	return newWorkspaceAtRunDir(absRunDir)
}

func (w *Workspace) uploadStore() (*document.LocalStore, error) {
	if w == nil {
		return nil, fmt.Errorf("workspace not set")
	}
	store := document.NewLocalStore(w.UploadDir)
	storeID := store.ID()
	if _, exists := document.GetStore(storeID); !exists {
		if err := document.RegisterStore(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (w *Workspace) llmStore() (*document.LocalStore, error) {
	if w == nil {
		return nil, fmt.Errorf("workspace not set")
	}
	store := document.NewLocalStore(w.LLMDir)
	storeID := store.ID()
	if _, exists := document.GetStore(storeID); !exists {
		if err := document.RegisterStore(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}
