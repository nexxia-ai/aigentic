package ctxt

import (
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

// PromptPart is a key-value segment of the system prompt, rendered in order.
type PromptPart struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Well-known system prompt part keys. Use these with SetSystemPart / PromptPart for consistency.
const (
	SystemPartKeyDescription        = "description"
	SystemPartKeyGoal               = "goal"
	SystemPartKeyInstructions       = "instructions"
	SystemPartKeyOutputInstructions = "output_instructions"
	SystemPartKeySkills             = "skills"
)

type AgentContext struct {
	id           string
	name         string
	summary      string
	runMeta      map[string]interface{}
	UserTemplate *template.Template

	systemParts []PromptPart
	stateBlock  string

	mutex               sync.RWMutex
	pendingRefs         []FileRef
	conversationHistory *ConversationHistory
	currentTurn         *Turn
	workspace           *Workspace
	basePath            string
	ledger              *Ledger
	enableTrace         bool
}

func New(id, description, instructions string, basePath string) (*AgentContext, error) {
	ctx := &AgentContext{
		id:      id,
		runMeta: make(map[string]interface{}),
		systemParts: []PromptPart{
			{Key: SystemPartKeyDescription, Value: description},
			{Key: SystemPartKeyInstructions, Value: instructions},
		},
	}

	if basePath == "" {
		return nil, fmt.Errorf("context base path is required")
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("base path: %w", err)
	}
	ctx.basePath = absBase
	runDir := RunDir(absBase, id)
	ws, err := newWorkspaceAtRunDir(runDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	ctx.workspace = ws
	ctx.ledger = NewLedger(absBase)
	conversationPath := filepath.Join(runDir, aigenticDirName, "conversation.json")
	ctx.conversationHistory = NewConversationHistory(ctx.ledger, conversationPath)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

// NewAtPath creates an AgentContext at an exact run path.
// runDir is the run root (basePath/runs/{id}/). BasePath is derived for ledger access.
func NewAtPath(id, description, instructions, runDir string) (*AgentContext, error) {
	ctx := &AgentContext{
		id:      id,
		runMeta: make(map[string]interface{}),
		systemParts: []PromptPart{
			{Key: SystemPartKeyDescription, Value: description},
			{Key: SystemPartKeyInstructions, Value: instructions},
		},
	}
	if runDir == "" {
		return nil, fmt.Errorf("run path is required")
	}
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return nil, fmt.Errorf("run path: %w", err)
	}
	parentDir := filepath.Dir(absRunDir)
	grandparentDir := filepath.Dir(parentDir)
	var basePath string
	if filepath.Base(grandparentDir) == "runs" {
		basePath = filepath.Dir(grandparentDir)
	} else {
		basePath = grandparentDir
	}
	ctx.basePath = basePath
	ws, err := newWorkspaceAtRunDir(absRunDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	ctx.workspace = ws
	ctx.ledger = NewLedger(basePath)
	conversationPath := filepath.Join(absRunDir, aigenticDirName, "conversation.json")
	ctx.conversationHistory = NewConversationHistory(ctx.ledger, conversationPath)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

// NewChild creates a child AgentContext with its own directory but sharing the
// given llm/ directory. Used by batch and plan tools. basePath is the ledger base (same as parent).
func NewChild(id, description, instructions, privateDir, sharedLLMDir, basePath string, inheritedParts ...PromptPart) (*AgentContext, error) {
	if privateDir == "" {
		return nil, fmt.Errorf("child private dir is required")
	}
	if sharedLLMDir == "" {
		return nil, fmt.Errorf("shared LLM dir is required")
	}
	if basePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("base path: %w", err)
	}
	ws, err := newChildWorkspace(privateDir, sharedLLMDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create child workspace: %w", err)
	}

	ctx := &AgentContext{
		id:          id,
		runMeta:     make(map[string]interface{}),
		workspace:   ws,
		basePath:    absBase,
		ledger:      NewLedger(absBase),
		systemParts: childSystemParts(description, instructions, inheritedParts),
	}
	conversationPath := filepath.Join(privateDir, "conversation.json")
	ctx.conversationHistory = NewConversationHistory(ctx.ledger, conversationPath)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx, nil
}

func childSystemParts(description, instructions string, inheritedParts []PromptPart) []PromptPart {
	parts := []PromptPart{
		{Key: SystemPartKeyDescription, Value: description},
		{Key: SystemPartKeyInstructions, Value: instructions},
	}
	for _, part := range inheritedParts {
		if part.Key == "" || part.Value == "" {
			continue
		}
		if part.Key == SystemPartKeyDescription || part.Key == SystemPartKeyInstructions {
			continue
		}
		replaced := false
		for i := range parts {
			if parts[i].Key == part.Key {
				parts[i].Value = part.Value
				replaced = true
				break
			}
		}
		if !replaced {
			parts = append(parts, part)
		}
	}
	return parts
}

func (r *AgentContext) Workspace() *Workspace {
	return r.workspace
}

func (r *AgentContext) BasePath() string {
	return r.basePath
}

func (r *AgentContext) Ledger() *Ledger {
	return r.ledger
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

func (r *AgentContext) SetStateBlock(state string) *AgentContext {
	r.mutex.Lock()
	r.stateBlock = state
	r.mutex.Unlock()
	return r
}

func (r *AgentContext) StateBlock() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.stateBlock
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

// AddFile registers a FileRef for the next turn.
func (r *AgentContext) AddFile(ref FileRef) error {
	if ref.MimeType == "" {
		ref.MimeType = document.DetectMimeTypeFromPath(ref.Path)
	}
	r.upsertPendingRef(ref)
	return nil
}

// AddFileRef registers a file reference for the next turn.
func (r *AgentContext) AddFileRef(path string, includeInPrompt bool, mimeType string) error {
	mime := mimeType
	if mime == "" {
		mime = document.DetectMimeTypeFromPath(path)
	}
	r.upsertPendingRef(FileRef{
		Path:            path,
		IncludeInPrompt: includeInPrompt,
		MimeType:        mime,
	})
	return nil
}

func (r *AgentContext) upsertPendingRef(ref FileRef) {
	for i := range r.pendingRefs {
		if r.pendingRefs[i].Path != ref.Path || r.pendingRefs[i].BasePath != ref.BasePath {
			continue
		}
		if ref.BasePath != "" {
			r.pendingRefs[i].BasePath = ref.BasePath
		}
		if ref.MimeType != "" {
			r.pendingRefs[i].MimeType = ref.MimeType
		}
		if ref.ToolID != "" {
			r.pendingRefs[i].ToolID = ref.ToolID
		}
		if ref.Role != "" {
			r.pendingRefs[i].Role = ref.Role
		}
		if ref.SizeBytes > 0 {
			r.pendingRefs[i].SizeBytes = ref.SizeBytes
		}
		if !ref.AddedAt.IsZero() {
			r.pendingRefs[i].AddedAt = ref.AddedAt
		}
		r.pendingRefs[i].IncludeInPrompt = r.pendingRefs[i].IncludeInPrompt || ref.IncludeInPrompt
		r.pendingRefs[i].Ephemeral = r.pendingRefs[i].Ephemeral || ref.Ephemeral
		if meta := ref.Meta(); len(meta) > 0 {
			r.pendingRefs[i].SetMeta(meta)
		}
		return
	}
	r.pendingRefs = append(r.pendingRefs, ref)
}

func (r *AgentContext) SetConversationHistory(history *ConversationHistory) *AgentContext {
	if history == nil && r.ledger != nil && r.workspace != nil {
		conversationPath := filepath.Join(r.workspace.PrivateDir, "conversation.json")
		history = NewConversationHistory(r.ledger, conversationPath)
	}
	r.conversationHistory = history
	return r
}

func (r *AgentContext) GetHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) SetHistoryBudget(turns int, bytes int) *AgentContext {
	if r.conversationHistory != nil {
		r.conversationHistory.SetBudget(turns, bytes)
	}
	return r
}

func (r *AgentContext) ConversationHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) ID() string {
	return r.id
}

func (r *AgentContext) Name() string {
	return r.name
}

func (r *AgentContext) SetMeta(key string, value interface{}) {
	if r.runMeta == nil {
		r.runMeta = make(map[string]interface{})
	}
	r.runMeta[key] = value
	r.saveRunMeta()
}

func (r *AgentContext) GetMeta(key string) (interface{}, bool) {
	if r.runMeta == nil {
		return nil, false
	}
	v, ok := r.runMeta[key]
	return v, ok
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
	if r.workspace == nil || r.runMeta == nil || len(r.runMeta) == 0 {
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
	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		slog.Warn("failed to parse run metadata", "path", path, "error", err)
		return
	}
	ctx.runMeta = meta
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

func (r *AgentContext) StartTurn(userMessage string, userData string) *Turn {
	turn := NewTurn(r, "", "", "", "")
	if r.ledger != nil {
		turnID, dirPath, err := r.ledger.PrepareTurn(time.Now())
		if err != nil {
			slog.Error("failed to prepare turn", "error", err)
		} else {
			turn.TurnID = turnID
			turn.SetLedgerDir(dirPath)
			turn.RunID = r.id
		}
	}
	r.currentTurn = turn
	r.currentTurn.UserMessage = userMessage
	r.currentTurn.UserData = userData
	for _, f := range r.pendingRefs {
		r.currentTurn.AddFile(f)
	}
	r.pendingRefs = nil
	r.currentTurn.StartFileCutoff = time.Now()
	if userMsg, err := createUserMsgForTurn(r, r.currentTurn); err == nil {
		r.currentTurn.RequestSnapshot = userMsg
	}
	return r.currentTurn
}

func (r *AgentContext) EndTurn(msg ai.Message) *AgentContext {
	r.currentTurn.AddMessage(msg)
	r.currentTurn.Reply = msg

	if aiMsg, ok := msg.(ai.AIMessage); ok {
		r.currentTurn.Usage = aiMsg.Response.Usage
	}

	if !r.currentTurn.Hidden {
		if r.currentTurn.RequestSnapshot != nil {
			r.currentTurn.Request = r.currentTurn.RequestSnapshot
		} else if userMsg, err := createUserMsgForTurn(r, r.currentTurn); err == nil {
			r.currentTurn.Request = userMsg
		}
		r.conversationHistory.appendTurn(*r.currentTurn)
	}
	r.save()
	return r
}

func (r *AgentContext) Turn() *Turn {
	return r.currentTurn
}

func (r *AgentContext) ClearHistory() *AgentContext {
	r.conversationHistory.Clear()
	return r
}

func (r *AgentContext) ClearAll() *AgentContext {
	ctx := r.ClearHistory().
		SetSystemPart(SystemPartKeyOutputInstructions, "")
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx
}

type contextData struct {
	ID          string       `json:"id"`
	SystemParts []PromptPart `json:"system_parts"`
	Name        string       `json:"name"`
	Summary     string       `json:"summary"`
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
