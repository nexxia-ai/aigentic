package ctxt

import (
	"bytes"
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

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type AgentContext struct {
	id             string
	description    string
	instructions   string
	name           string
	summary        string
	SystemTemplate *template.Template
	UserTemplate   *template.Template

	mutex               sync.RWMutex
	memories            []MemoryEntry
	documentReferences  []*document.Document
	conversationHistory *ConversationHistory
	outputInstructions  string
	currentTurn         *Turn
	execEnv             *ExecutionEnvironment
	turnCounter         int
}

func New(id, description, instructions string, basePath string) (*AgentContext, error) {
	ctx := &AgentContext{
		id:                 id,
		description:        description,
		instructions:       instructions,
		memories:           make([]MemoryEntry, 0),
		documentReferences: make([]*document.Document, 0),
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
	ctx.currentTurn = ctx.newTurn() // create the first turn so it is available for the first prompt
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

func (r *AgentContext) insertDocuments(docs []*document.Document, docRefs []*document.Document) []ai.Message {
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

	for _, docRef := range docRefs {
		fileID := docRef.ID()
		var partType ai.ContentPartType
		var part ai.ContentPart

		switch {
		case strings.HasPrefix(docRef.MimeType, "image/"):
			partType = ai.ContentPartImageURL
		case strings.HasPrefix(docRef.MimeType, "audio/"):
			partType = ai.ContentPartAudio
		case strings.HasPrefix(docRef.MimeType, "video/"):
			partType = ai.ContentPartVideo
		default:
			partType = ai.ContentPartInputFile
		}

		if partType == ai.ContentPartInputFile {
			part = ai.ContentPart{
				Type:   partType,
				FileID: fileID,
				Name:   docRef.Filename,
			}
		} else {
			part = ai.ContentPart{
				Type:     partType,
				URI:      fmt.Sprintf("file://%s", fileID),
				Name:     docRef.Filename,
				MimeType: docRef.MimeType,
			}
		}

		refMsg := ai.UserMessage{
			Role:  ai.UserRole,
			Parts: []ai.ContentPart{part},
		}
		msgs = append(msgs, refMsg)
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

func (r *AgentContext) AddDocument(doc *document.Document) *AgentContext {
	if doc == nil {
		return r
	}
	store, err := r.uploadStore()
	if err != nil {
		slog.Error("failed to resolve upload store", "error", err)
		return r
	}
	content, err := doc.Bytes()
	if err != nil {
		slog.Error("failed to read document content", "error", err)
		return r
	}
	filename := doc.Filename
	if filename == "" {
		filename = filepath.Base(doc.FilePath)
	}
	if filename == "" || filename == "." || filename == string(os.PathSeparator) || filename == "/" {
		filename = doc.ID()
	}
	if filename == "" {
		return r
	}
	_, err = document.Create(context.Background(), store.ID(), filename, bytes.NewReader(content))
	if err != nil {
		slog.Error("failed to store document", "error", err, "filename", filename)
	}
	return r
}

func (r *AgentContext) AddDocumentReference(doc *document.Document) *AgentContext {
	if doc == nil {
		return r
	}
	r.documentReferences = append(r.documentReferences, doc)
	return r
}

func (r *AgentContext) RemoveDocument(doc *document.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}
	candidates := []string{}
	if doc.ID() != "" {
		candidates = append(candidates, doc.ID())
	}
	if doc.Filename != "" {
		candidates = append(candidates, doc.Filename)
	}
	if doc.FilePath != "" {
		base := filepath.Base(doc.FilePath)
		if base != "" {
			candidates = append(candidates, base)
		}
	}
	for _, id := range candidates {
		if err := r.RemoveDocumentByID(id); err == nil {
			return nil
		}
	}
	return fmt.Errorf("document not found: %s", doc.ID())
}

func (r *AgentContext) RemoveDocumentByID(id string) error {
	if id == "" {
		return fmt.Errorf("document id cannot be empty")
	}
	id = filepath.ToSlash(id)
	id = strings.TrimPrefix(id, "uploads/")
	store, err := r.uploadStore()
	if err != nil {
		return err
	}
	ids, err := store.List(context.Background())
	if err != nil {
		return err
	}
	found := false
	for _, existingID := range ids {
		if existingID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("document not found: %s", id)
	}
	if err := document.Delete(context.Background(), store.ID(), id); err != nil {
		return err
	}
	return nil
}

func (r *AgentContext) GetDocumentByID(id string) *document.Document {
	if id == "" {
		return nil
	}
	id = filepath.ToSlash(id)
	store, err := r.llmStore()
	if err != nil {
		return nil
	}
	doc, err := document.Open(context.Background(), store.ID(), id)
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

func (r *AgentContext) GetDocumentReferences() []*document.Document {
	return r.documentReferences
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
	r.currentTurn.UserMessage = userMessage
	r.currentTurn.Request = ai.UserMessage{Role: ai.UserRole, Content: userMessage}
	return r.currentTurn
}

func (r *AgentContext) EndTurn(msg ai.Message) *AgentContext {
	r.currentTurn.AddMessage(msg)
	r.currentTurn.Reply = msg
	r.conversationHistory.appendTurn(*r.currentTurn)

	// create the next turn so it is available for callers
	r.currentTurn = r.newTurn()
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

func (r *AgentContext) ClearDocumentReferences() *AgentContext {
	r.documentReferences = make([]*document.Document, 0)
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
		ClearDocumentReferences().
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

	return nil
}
