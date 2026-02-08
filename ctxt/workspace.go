package ctxt

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic/document"
)

const memoryStoreName = "memory_files"

// Workspace manages the filesystem layout and I/O operations for an agent's execution directory.
// It owns llm/, _private/, turns/, and provides document CRUD and memory file access.
type Workspace struct {
	RootDir    string
	LLMDir     string
	PrivateDir string
	MemoryDir  string
	UploadDir  string
	OutputDir  string
	TurnDir    string
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

func newWorkspaceAt(rootDir string) (*Workspace, error) {
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private")

	w := &Workspace{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
		TurnDir:    filepath.Join(privateDir, "turns"),
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
		w.TurnDir,
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
		"AGENT_TURN_DIR":    w.TurnDir,
	}
	if w.MemoryDir != "" {
		m["AGENT_MEMORY_DIR"] = w.MemoryDir
	}
	return m
}

func (w *Workspace) MemoryStoreName() string {
	return memoryStoreName
}

func loadWorkspace(sessionDir string) (*Workspace, error) {
	sessionDir, err := filepath.Abs(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	rootDir := sessionDir
	llmDir := filepath.Join(rootDir, "llm")
	privateDir := filepath.Join(rootDir, "_private")

	ws := &Workspace{
		RootDir:    rootDir,
		LLMDir:     llmDir,
		PrivateDir: privateDir,
		MemoryDir:  "",
		UploadDir:  filepath.Join(llmDir, "uploads"),
		OutputDir:  filepath.Join(llmDir, "output"),
		TurnDir:    filepath.Join(privateDir, "turns"),
	}

	return ws, nil
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

func workspaceNormalizePath(llmDir, path string) (string, error) {
	path = filepath.ToSlash(strings.TrimPrefix(path, "/"))
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return "", fmt.Errorf("invalid path: %s", path)
	}
	if strings.HasPrefix(path, "..") || strings.Contains(path, "..") {
		return "", fmt.Errorf("path must not contain ..: %s", path)
	}
	absLLM, err := filepath.Abs(llmDir)
	if err != nil {
		return "", fmt.Errorf("llm dir: %w", err)
	}
	fullPath := filepath.Join(absLLM, path)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("path resolve: %w", err)
	}
	rel, err := filepath.Rel(absLLM, absFull)
	if err != nil {
		return "", fmt.Errorf("path not under LLMDir: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path resolves outside LLMDir: %s", path)
	}
	return filepath.ToSlash(rel), nil
}

func (w *Workspace) UploadDocument(path string, content []byte, mimeType string) (string, error) {
	if w == nil {
		return "", fmt.Errorf("workspace not set")
	}
	normPath, err := workspaceNormalizePath(w.LLMDir, path)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(w.LLMDir, normPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	if mimeType == "" {
		mimeType = document.DetectMimeTypeFromPath(normPath)
	}
	return normPath, nil
}

func (w *Workspace) RemoveDocument(path string) error {
	if w == nil {
		return fmt.Errorf("workspace not set")
	}
	normPath, err := workspaceNormalizePath(w.LLMDir, path)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(w.LLMDir, normPath)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("document not found: %s", normPath)
		}
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

func (w *Workspace) GetDocument(path string) *document.Document {
	if path == "" || w == nil {
		return nil
	}
	normPath, err := workspaceNormalizePath(w.LLMDir, path)
	if err != nil {
		return nil
	}
	store, err := w.llmStore()
	if err != nil {
		return nil
	}
	doc, err := document.Open(context.Background(), store.ID(), normPath)
	if err != nil {
		return nil
	}
	return doc
}

func (w *Workspace) GetDocuments() []*document.Document {
	if w == nil {
		return []*document.Document{}
	}
	store, err := w.llmStore()
	if err != nil {
		return []*document.Document{}
	}
	var paths []string
	_ = filepath.WalkDir(w.LLMDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(w.LLMDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	sort.Strings(paths)
	docs := make([]*document.Document, 0, len(paths))
	for _, rel := range paths {
		doc, err := document.Open(context.Background(), store.ID(), rel)
		if err != nil {
			continue
		}
		doc.SetID(rel)
		docs = append(docs, doc)
	}
	return docs
}

// GetUploadDocuments returns only documents in the uploads directory (user-uploaded session files).
func (w *Workspace) GetUploadDocuments() []*document.Document {
	if w == nil || w.UploadDir == "" {
		return []*document.Document{}
	}
	if _, err := w.uploadStore(); err != nil {
		return []*document.Document{}
	}
	var paths []string
	_ = filepath.WalkDir(w.UploadDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(w.UploadDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		paths = append(paths, "uploads/"+rel)
		return nil
	})
	sort.Strings(paths)
	docs := make([]*document.Document, 0, len(paths))
	llmStore, err := w.llmStore()
	if err != nil {
		return docs
	}
	for _, rel := range paths {
		doc, err := document.Open(context.Background(), llmStore.ID(), rel)
		if err != nil {
			continue
		}
		doc.SetID(rel)
		docs = append(docs, doc)
	}
	return docs
}

// ArchiveDir returns the path to the archive directory (_private/archive/).
func (w *Workspace) ArchiveDir() string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.PrivateDir, "archive")
}

// ArchiveTurns moves turn directories to archive/YYYY-MM/YYYY-MM-DD/.
// Uses atomic temp-dir + rename for safety.
func (w *Workspace) ArchiveTurns(turns []Turn, date time.Time) error {
	if w == nil || len(turns) == 0 {
		return nil
	}
	month := date.Format("2006-01")
	day := date.Format("2006-01-02")
	destDir := filepath.Join(w.ArchiveDir(), month, day)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}
	tmpDir := filepath.Join(w.PrivateDir, ".archive-tmp-"+day)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, turn := range turns {
		src := filepath.Join(w.TurnDir, turn.TurnID)
		dst := filepath.Join(tmpDir, turn.TurnID)
		if err := os.Rename(src, dst); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("move turn %s: %w", turn.TurnID, err)
			}
		}
	}

	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if e.IsDir() {
			src := filepath.Join(tmpDir, e.Name())
			dst := filepath.Join(destDir, e.Name())
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("move %s to archive: %w", e.Name(), err)
			}
		}
	}
	return nil
}

// SaveSummaries persists compaction summaries to _private/summaries.json.
func (w *Workspace) SaveSummaries(summaries []CompactionSummary) error {
	if w == nil {
		return nil
	}
	path := filepath.Join(w.PrivateDir, "summaries.json")
	out, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summaries: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

// LoadSummaries loads compaction summaries from _private/summaries.json.
func (w *Workspace) LoadSummaries() ([]CompactionSummary, error) {
	if w == nil {
		return nil, nil
	}
	path := filepath.Join(w.PrivateDir, "summaries.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read summaries: %w", err)
	}
	var summaries []CompactionSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		return nil, fmt.Errorf("parse summaries: %w", err)
	}
	return summaries, nil
}

// SaveArchiveIndex writes the per-month search index to archive/YYYY-MM/index.json.
func (w *Workspace) SaveArchiveIndex(month string, entries []ArchiveIndexEntry) error {
	if w == nil {
		return nil
	}
	dir := filepath.Join(w.ArchiveDir(), month)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create archive month dir: %w", err)
	}
	path := filepath.Join(dir, "index.json")
	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

// LoadArchiveIndex reads the per-month search index from archive/YYYY-MM/index.json.
func (w *Workspace) LoadArchiveIndex(month string) ([]ArchiveIndexEntry, error) {
	if w == nil {
		return nil, nil
	}
	path := filepath.Join(w.ArchiveDir(), month, "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read index: %w", err)
	}
	var entries []ArchiveIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	return entries, nil
}
