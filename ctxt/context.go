package ctxt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// PromptPart is a key-value segment of the system prompt, rendered in order.
type PromptPart struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Well-known system prompt part keys. Use these with SetSystemPart / PromptPart for consistency.
const (
	SystemPartKeyDescription        = "description"
	SystemPartKeyInstructions       = "instructions"
	SystemPartKeyOutputInstructions = "output_instructions"
)

type AgentContext struct {
	id             string
	name           string
	summary        string
	runMeta        *runMetaData
	UserTemplate   *template.Template

	systemParts []PromptPart

	mutex               sync.RWMutex
	pendingRefs         []FileRefEntry
	conversationHistory *ConversationHistory
	currentTurn         *Turn
	workspace           *Workspace
	turnCounter         int
	enableTrace         bool
}

func New(id, description, instructions string, basePath string) (*AgentContext, error) {
	m := &runMetaData{AgentName: id, PackageID: "", StartedAt: time.Now()}
	ctx := &AgentContext{
		id:      id,
		runMeta: m,
		systemParts: []PromptPart{
			{Key: SystemPartKeyDescription, Value: description},
			{Key: SystemPartKeyInstructions, Value: instructions},
		},
	}

	if basePath == "" {
		return nil, fmt.Errorf("context base path is required")
	}
	ws, err := NewWorkspace(basePath, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %s: %w", basePath, err)
	}
	ctx.workspace = ws

	ctx.conversationHistory = NewConversationHistory(ctx.workspace)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

// NewAtPath creates an AgentContext at an exact path without timestamp prefix.
// Used by the orchestrator for single-instance agents.
func NewAtPath(id, description, instructions, exactPath string) (*AgentContext, error) {
	m := &runMetaData{AgentName: id, PackageID: "", StartedAt: time.Now()}
	ctx := &AgentContext{
		id:      id,
		runMeta: m,
		systemParts: []PromptPart{
			{Key: SystemPartKeyDescription, Value: description},
			{Key: SystemPartKeyInstructions, Value: instructions},
		},
	}
	if exactPath == "" {
		return nil, fmt.Errorf("exact path is required")
	}
	ws, err := NewWorkspaceAtPath(exactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace at path: %w", err)
	}
	ctx.workspace = ws
	ctx.conversationHistory = NewConversationHistory(ctx.workspace)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

// NewChild creates a child AgentContext with its own _private/ directory but sharing the
// given llm/ directory. Used by batch and plan tools to give each child its own turns
// while sharing uploads, output, and memory with the parent and siblings.
func NewChild(id, description, instructions, privateDir, sharedLLMDir string) (*AgentContext, error) {
	if privateDir == "" {
		return nil, fmt.Errorf("child private dir is required")
	}
	if sharedLLMDir == "" {
		return nil, fmt.Errorf("shared LLM dir is required")
	}

	ws, err := newChildWorkspace(privateDir, sharedLLMDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create child workspace: %w", err)
	}

	m := &runMetaData{AgentName: id, PackageID: "", StartedAt: time.Now()}
	ctx := &AgentContext{
		id:      id,
		runMeta: m,
		workspace: ws,
		systemParts: []PromptPart{
			{Key: SystemPartKeyDescription, Value: description},
			{Key: SystemPartKeyInstructions, Value: instructions},
		},
	}
	ctx.conversationHistory = NewConversationHistory(ctx.workspace)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

func (r *AgentContext) Workspace() *Workspace {
	return r.workspace
}

func (r *AgentContext) SetSystemPart(key, value string) *AgentContext {
	if key == "" {
		return r
	}
	r.mutex.Lock()
	found := -1
	for i := range r.systemParts {
		if r.systemParts[i].Key == key {
			found = i
			break
		}
	}
	if value == "" {
		if found >= 0 {
			r.systemParts = append(r.systemParts[:found], r.systemParts[found+1:]...)
		}
	} else if found >= 0 {
		r.systemParts[found].Value = value
	} else {
		r.systemParts = append(r.systemParts, PromptPart{Key: key, Value: value})
	}
	r.mutex.Unlock()
	r.save()
	return r
}

// PromptPart returns the value for the given system part key and whether it was present.
func (r *AgentContext) PromptPart(key string) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, p := range r.systemParts {
		if p.Key == key {
			return p.Value, true
		}
	}
	return "", false
}

// SystemParts returns a copy of the ordered system prompt parts.
func (r *AgentContext) SystemParts() []PromptPart {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	out := make([]PromptPart, len(r.systemParts))
	copy(out, r.systemParts)
	return out
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
	return r.SetSystemPart(SystemPartKeyDescription, description)
}

func (r *AgentContext) SetInstructions(instructions string) *AgentContext {
	return r.SetSystemPart(SystemPartKeyInstructions, instructions)
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

func (r *AgentContext) UploadDocument(path string, content []byte, mimeType string, includeInNextTurn ...bool) error {
	if r.workspace == nil {
		return fmt.Errorf("workspace not set")
	}
	normPath, err := r.workspace.UploadDocument(path, content, mimeType)
	if err != nil {
		return err
	}
	inc := false
	if len(includeInNextTurn) > 0 {
		inc = includeInNextTurn[0]
	}
	mime := mimeType
	if mime == "" {
		mime = document.DetectMimeTypeFromPath(normPath)
	}
	for i := range r.pendingRefs {
		if r.pendingRefs[i].Path != normPath {
			continue
		}
		slog.Error("duplicated file ref in upload", "path", normPath)
		return nil
	}
	r.pendingRefs = append(r.pendingRefs, FileRefEntry{
		Path:            normPath,
		IncludeInPrompt: inc,
		MimeType:        mime,
		UserUpload:      true,
	})
	return nil
}

func (r *AgentContext) RemoveDocument(path string) error {
	if r.workspace == nil {
		return fmt.Errorf("workspace not set")
	}
	return r.workspace.RemoveDocument(path)
}

func (r *AgentContext) GetDocument(path string) *document.Document {
	if r.workspace == nil {
		return nil
	}
	return r.workspace.GetDocument(path)
}

func (r *AgentContext) SetConversationHistory(history *ConversationHistory) *AgentContext {
	if history == nil {
		history = NewConversationHistory(r.workspace)
	}
	r.conversationHistory = history
	return r
}

func (r *AgentContext) GetDocuments() []*document.Document {
	if r.workspace == nil {
		return []*document.Document{}
	}
	return r.workspace.GetDocuments()
}

func (r *AgentContext) GetUploadDocuments() []*document.Document {
	if r.workspace == nil {
		return []*document.Document{}
	}
	return r.workspace.GetUploadDocuments()
}

func (r *AgentContext) GetHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) ConversationHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) ShouldCompact(config CompactionConfig) bool {
	return len(r.conversationHistory.DaysToCompact(config)) > 0
}

func (r *AgentContext) ID() string {
	return r.id
}

func (r *AgentContext) Name() string {
	return r.name
}

func (r *AgentContext) SetMeta(agentName, packageID string) {
	if r.runMeta == nil {
		r.runMeta = &runMetaData{StartedAt: time.Now()}
	}
	r.runMeta.AgentName = agentName
	r.runMeta.PackageID = packageID
	r.saveRunMeta()
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
	if r.workspace == nil || r.runMeta == nil {
		return
	}
	path := filepath.Join(r.workspace.PrivateDir, "run_meta.json")
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
	turnID := fmt.Sprintf("turn-%06d", r.turnCounter)
	turn := NewTurn(r, "", "", turnID)

	if r.workspace != nil {
		turnDir := filepath.Join(r.workspace.TurnDir, turn.TurnID)
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

	if aiMsg, ok := msg.(ai.AIMessage); ok {
		r.currentTurn.Usage = aiMsg.Response.Usage
	}

	if !r.currentTurn.Hidden {
		r.conversationHistory.appendTurn(*r.currentTurn)
	}
	r.save()
	return r
}

func (r *AgentContext) Turn() *Turn {
	return r.currentTurn
}

func (r *AgentContext) ClearDocuments() *AgentContext {
	store, err := r.workspace.uploadStore()
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

func (r *AgentContext) ClearHistory() *AgentContext {
	r.conversationHistory.Clear()
	return r
}

func (r *AgentContext) ClearAll() *AgentContext {
	ctx := r.ClearDocuments().
		ClearHistory().
		SetSystemPart(SystemPartKeyOutputInstructions, "")
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx
}

type contextData struct {
	ID          string       `json:"id"`
	SystemParts []PromptPart `json:"system_parts"`
	Name        string       `json:"name"`
	Summary     string       `json:"summary"`
	TurnCounter int          `json:"turn_counter"`
	MemoryDir   string       `json:"memory_dir"`
	EnableTrace bool         `json:"enable_trace"`
}

func (r *AgentContext) save() error {
	if r.workspace == nil {
		return nil
	}

	r.mutex.RLock()
	partsCopy := make([]PromptPart, len(r.systemParts))
	copy(partsCopy, r.systemParts)
	r.mutex.RUnlock()

	data := contextData{
		ID:          r.id,
		SystemParts: partsCopy,
		Name:        r.name,
		Summary:     r.summary,
		TurnCounter: r.turnCounter,
		MemoryDir:   r.workspace.MemoryDir,
		EnableTrace: r.enableTrace,
	}

	contextFile := filepath.Join(r.workspace.PrivateDir, "context.json")
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
