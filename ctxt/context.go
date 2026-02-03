package ctxt

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type runMetaData struct {
	AgentName string    `json:"agent_name"`
	PackageID string    `json:"package_id"`
	StartedAt time.Time `json:"started_at"`
}

type AgentContext struct {
	id             string
	description    string
	instructions   string
	name           string
	summary        string
	runMeta        *runMetaData
	SystemTemplate *template.Template
	UserTemplate   *template.Template

	mutex               sync.RWMutex
	memories            []MemoryEntry
	pendingRefs         []FileRefEntry
	conversationHistory *ConversationHistory
	outputInstructions  string
	currentTurn         *Turn
	execEnv             *ExecutionEnvironment
	turnCounter         int
	enableTrace         bool
}

func New(id, description, instructions string, basePath string) (*AgentContext, error) {
	m := &runMetaData{AgentName: id, PackageID: "", StartedAt: time.Now()}
	ctx := &AgentContext{
		id:           id,
		description:  description,
		instructions: instructions,
		memories:     make([]MemoryEntry, 0),
		runMeta:      m,
	}

	if basePath == "" {
		return nil, fmt.Errorf("context base path is required")
	}
	ee, err := NewExecutionEnvironment(basePath, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution environment: %s: %w", basePath, err)
	}
	ctx.execEnv = ee

	ctx.conversationHistory = NewConversationHistory(ctx.execEnv)
	ctx.UpdateSystemTemplate(DefaultSystemTemplate)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

func (r *AgentContext) ExecutionEnvironment() *ExecutionEnvironment {
	return r.execEnv
}

func (r *AgentContext) SetOutputInstructions(instructions string) *AgentContext {
	r.outputInstructions = instructions
	r.save()
	return r
}

func (r *AgentContext) UpdateSystemTemplate(templateStr string) error {
	tmpl, err := template.New("system").Parse(templateStr)
	if err != nil {
		return err
	}
	r.SystemTemplate = tmpl
	return nil
}

func (r *AgentContext) UpdateUserTemplate(templateStr string) error {
	tmpl, err := template.New("user").Parse(templateStr)
	if err != nil {
		return err
	}
	r.UserTemplate = tmpl
	return nil
}

func (r *AgentContext) SetDescription(description string) *AgentContext {
	r.description = description
	r.save()
	return r
}

func (r *AgentContext) SetInstructions(instructions string) *AgentContext {
	r.instructions = instructions
	r.save()
	return r
}

func (r *AgentContext) insertDocuments(docs []*document.Document) []ai.Message {
	var msgs []ai.Message

	for _, doc := range docs {
		content, err := doc.Bytes()
		if err != nil {
			continue
		}

		var partType ai.ContentPartType
		var part ai.ContentPart

		switch {
		case strings.HasPrefix(doc.MimeType, "image/"):
			partType = ai.ContentPartImage
			part = ai.ContentPart{
				Type:     partType,
				MimeType: doc.MimeType,
				Data:     content,
				Name:     doc.Filename,
			}
		case strings.HasPrefix(doc.MimeType, "audio/"):
			partType = ai.ContentPartAudio
			part = ai.ContentPart{
				Type:     partType,
				MimeType: doc.MimeType,
				Data:     content,
				Name:     doc.Filename,
			}
		case strings.HasPrefix(doc.MimeType, "video/"):
			partType = ai.ContentPartVideo
			part = ai.ContentPart{
				Type:     partType,
				MimeType: doc.MimeType,
				Data:     content,
				Name:     doc.Filename,
			}
		case strings.HasPrefix(doc.MimeType, "text/"),
			strings.HasSuffix(doc.MimeType, "application/json"),
			strings.HasSuffix(doc.MimeType, "application/yaml"),
			strings.HasSuffix(doc.MimeType, "application/xml"),
			strings.HasSuffix(doc.MimeType, "application/csv"),
			strings.HasSuffix(doc.MimeType, "application/xml"),
			strings.HasSuffix(doc.MimeType, "application/xml"):
			partType = ai.ContentPartText
			part = ai.ContentPart{
				Type:     partType,
				MimeType: doc.MimeType,
				Data:     content,
				Name:     doc.Filename,
			}

		case strings.HasPrefix(doc.MimeType, "application/"):
			partType = ai.ContentPartFile
			part = ai.ContentPart{
				Type:     partType,
				MimeType: doc.MimeType,
				Data:     content,
				Name:     doc.Filename,
			}

		default:
			partType = ai.ContentPartText
			part = ai.ContentPart{
				Type:     partType,
				Text:     string(content),
				MimeType: doc.MimeType,
				Name:     doc.Filename,
			}
		}

		attachmentMsg := ai.UserMessage{
			Role:  ai.UserRole,
			Parts: []ai.ContentPart{part},
		}
		msgs = append(msgs, attachmentMsg)
	}

	return msgs
}

func (r *AgentContext) uploadStore() (*document.LocalStore, error) {
	if r.execEnv == nil {
		return nil, fmt.Errorf("execution environment not set")
	}
	store := document.NewLocalStore(r.execEnv.UploadDir)
	storeID := store.ID()
	if _, exists := document.GetStore(storeID); !exists {
		if err := document.RegisterStore(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (r *AgentContext) llmStore() (*document.LocalStore, error) {
	if r.execEnv == nil {
		return nil, fmt.Errorf("execution environment not set")
	}
	store := document.NewLocalStore(r.execEnv.LLMDir)
	storeID := store.ID()
	if _, exists := document.GetStore(storeID); !exists {
		if err := document.RegisterStore(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func normalizePath(llmDir, path string) (string, error) {
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

func (r *AgentContext) UploadDocument(path string, content []byte, includeInNextTurn ...bool) error {
	if r.execEnv == nil {
		return fmt.Errorf("execution environment not set")
	}
	normPath, err := normalizePath(r.execEnv.LLMDir, path)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(r.execEnv.LLMDir, normPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	inc := false
	if len(includeInNextTurn) > 0 {
		inc = includeInNextTurn[0]
	}
	r.pendingRefs = append(r.pendingRefs, FileRefEntry{Path: normPath, IncludeInPrompt: inc})
	return nil
}

func (r *AgentContext) RemoveDocument(path string) error {
	if r.execEnv == nil {
		return fmt.Errorf("execution environment not set")
	}
	normPath, err := normalizePath(r.execEnv.LLMDir, path)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(r.execEnv.LLMDir, normPath)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("document not found: %s", normPath)
		}
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

func (r *AgentContext) GetDocument(path string) *document.Document {
	if path == "" || r.execEnv == nil {
		return nil
	}
	normPath, err := normalizePath(r.execEnv.LLMDir, path)
	if err != nil {
		return nil
	}
	store, err := r.llmStore()
	if err != nil {
		return nil
	}
	doc, err := document.Open(context.Background(), store.ID(), normPath)
	if err != nil {
		return nil
	}
	return doc
}

func (r *AgentContext) SetConversationHistory(history *ConversationHistory) *AgentContext {
	if history == nil {
		history = NewConversationHistory(r.execEnv)
	}
	r.conversationHistory = history
	return r
}

func (r *AgentContext) GetDocuments() []*document.Document {
	if r.execEnv == nil {
		return []*document.Document{}
	}
	store, err := r.llmStore()
	if err != nil {
		return []*document.Document{}
	}
	var paths []string
	_ = filepath.WalkDir(r.execEnv.LLMDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.execEnv.LLMDir, path)
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

func (r *AgentContext) GetHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) ID() string {
	return r.id
}

func (r *AgentContext) Name() string {
	return r.name
}

func (r *AgentContext) SetMeta(agentName, packageID string) {
	r.runMeta.AgentName = agentName
	r.runMeta.PackageID = packageID
	r.saveRunMeta()
}

func (r *AgentContext) SetRunMeta(agentName, packageID string) {
	if r.runMeta == nil {
		r.runMeta = &runMetaData{StartedAt: time.Now()}
	}
	r.SetMeta(agentName, packageID)
}

func (r *AgentContext) RunAgentName() string {
	if r.runMeta != nil {
		return r.runMeta.AgentName
	}
	return ""
}

func (r *AgentContext) RunPackageID() string {
	if r.runMeta != nil {
		return r.runMeta.PackageID
	}
	return ""
}

func (r *AgentContext) RunStartedAt() time.Time {
	if r.runMeta != nil {
		return r.runMeta.StartedAt
	}
	return time.Time{}
}

func (r *AgentContext) SetEnableTrace(enable bool) *AgentContext {
	r.enableTrace = enable
	r.save()
	return r
}

func (r *AgentContext) EnableTrace() bool {
	return r.enableTrace
}

func (r *AgentContext) saveRunMeta() {
	if r.execEnv == nil || r.runMeta == nil {
		return
	}
	path := filepath.Join(r.execEnv.PrivateDir, "run_meta.json")
	data, err := json.MarshalIndent(r.runMeta, "", "  ")
	if err != nil {
		slog.Error("failed to marshal run metadata", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("failed to write run metadata", "path", path, "error", err)
	}
}

func loadRunMeta(ctx *AgentContext, privateDir string) {
	path := filepath.Join(privateDir, "run_meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		slog.Warn("failed to read run metadata", "path", path, "error", err)
		return
	}
	var meta runMetaData
	if err := json.Unmarshal(data, &meta); err != nil {
		slog.Warn("failed to parse run metadata", "path", path, "error", err)
		return
	}
	ctx.runMeta = &meta
}

func (r *AgentContext) Summary() string {
	return r.summary
}

func (r *AgentContext) SetName(name string) *AgentContext {
	r.name = name
	r.save()
	return r
}

func (r *AgentContext) SetSummary(summary string) *AgentContext {
	r.summary = summary
	r.save()
	return r
}

func (r *AgentContext) newTurn() *Turn {
	r.turnCounter++
	turnID := fmt.Sprintf("turn-%03d", r.turnCounter)
	turn := NewTurn(r, "", "", turnID)

	if r.execEnv != nil {
		turnDir := filepath.Join(r.execEnv.TurnDir, turn.TurnID)
		if err := os.MkdirAll(turnDir, 0755); err != nil {
			slog.Error("failed to create turn directory", "error", err)
		}
	}

	return turn
}

func (r *AgentContext) StartTurn(userMessage string) *Turn {
	r.currentTurn = r.newTurn()
	r.currentTurn.UserMessage = userMessage
	r.currentTurn.Request = ai.UserMessage{Role: ai.UserRole, Content: userMessage}
	r.currentTurn.FileRefs = append(r.currentTurn.FileRefs, r.pendingRefs...)
	r.pendingRefs = nil
	return r.currentTurn
}

func (r *AgentContext) EndTurn(msg ai.Message) *AgentContext {
	r.currentTurn.AddMessage(msg)
	r.currentTurn.Reply = msg
	r.conversationHistory.appendTurn(*r.currentTurn)
	r.save()
	return r
}

func (r *AgentContext) Turn() *Turn {
	return r.currentTurn
}

func (r *AgentContext) ClearDocuments() *AgentContext {
	store, err := r.uploadStore()
	if err != nil {
		return r
	}
	ids, err := store.List(context.Background())
	if err != nil {
		return r
	}
	for _, id := range ids {
		_ = document.Delete(context.Background(), store.ID(), id)
	}
	return r
}

func (r *AgentContext) ClearMemories() *AgentContext {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.memories = make([]MemoryEntry, 0)
	return r
}

func (r *AgentContext) ClearHistory() *AgentContext {
	r.conversationHistory.Clear()
	return r
}

func (r *AgentContext) ClearAll() *AgentContext {
	ctx := r.ClearDocuments().
		ClearMemories().
		ClearHistory().
		SetOutputInstructions("")
	ctx.UpdateSystemTemplate(DefaultSystemTemplate)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx
}

type contextData struct {
	ID                 string        `json:"id"`
	Description        string        `json:"description"`
	Instructions       string        `json:"instructions"`
	Name               string        `json:"name"`
	Summary            string        `json:"summary"`
	OutputInstructions string        `json:"output_instructions"`
	Memories           []MemoryEntry `json:"memories"`
	TurnCounter        int           `json:"turn_counter"`
	MemoryDir          string        `json:"memory_dir"`
	EnableTrace        bool          `json:"enable_trace"`
}

func (r *AgentContext) save() error {
	if r.execEnv == nil {
		return nil
	}

	r.mutex.RLock()
	memories := make([]MemoryEntry, len(r.memories))
	copy(memories, r.memories)
	r.mutex.RUnlock()

	data := contextData{
		ID:                 r.id,
		Description:        r.description,
		Instructions:       r.instructions,
		Name:               r.name,
		Summary:            r.summary,
		OutputInstructions: r.outputInstructions,
		Memories:           memories,
		TurnCounter:        r.turnCounter,
		MemoryDir:          r.execEnv.MemoryDir,
		EnableTrace:        r.enableTrace,
	}

	contextFile := filepath.Join(r.execEnv.PrivateDir, "context.json")
	file, err := os.Create(contextFile)
	if err != nil {
		slog.Error("failed to save context", "error", err, "context_file", contextFile)
		return fmt.Errorf("failed to create context file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		slog.Error("failed to save context", "error", err, "context_file", contextFile)
		return fmt.Errorf("failed to encode context: %w", err)
	}

	r.saveRunMeta()
	return nil
}
