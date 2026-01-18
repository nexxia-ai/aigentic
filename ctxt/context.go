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
	documents           []*document.Document
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
		documents:          make([]*document.Document, 0),
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

func (r *AgentContext) AddDocument(doc *document.Document) *AgentContext {
	if doc == nil {
		return r
	}
	r.documents = append(r.documents, doc)
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
	for i := range r.documents {
		if r.documents[i].ID() == doc.ID() {
			r.documents = append(r.documents[:i], r.documents[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("document not found: %s", doc.ID())
}

func (r *AgentContext) RemoveDocumentByID(id string) error {
	for i := range r.documents {
		if r.documents[i].ID() == id {
			r.documents = append(r.documents[:i], r.documents[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("document not found: %s", id)
}

func (r *AgentContext) GetDocumentByID(id string) *document.Document {
	for _, doc := range r.documents {
		if doc.ID() == id {
			return doc
		}
	}
	return nil
}

func (r *AgentContext) AddMemory(id, description, content string) *AgentContext {
	r.mutex.Lock()

	now := time.Now()
	for i := range r.memories {
		if r.memories[i].ID == id {
			r.memories[i].Description = description
			r.memories[i].Content = content
			r.memories[i].Timestamp = now
			r.mutex.Unlock()
			r.save()
			return r
		}
	}

	r.memories = append(r.memories, MemoryEntry{
		ID:          id,
		Description: description,
		Content:     content,
		Timestamp:   now,
	})
	r.mutex.Unlock()
	r.save()
	return r
}

func (r *AgentContext) RemoveMemory(id string) error {
	r.mutex.Lock()

	for i := range r.memories {
		if r.memories[i].ID == id {
			r.memories = append(r.memories[:i], r.memories[i+1:]...)
			r.mutex.Unlock()
			r.save()
			return nil
		}
	}
	r.mutex.Unlock()
	return fmt.Errorf("memory not found: %s", id)
}

func (r *AgentContext) GetMemories() []MemoryEntry {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]MemoryEntry, len(r.memories))
	copy(result, r.memories)
	return result
}

func (r *AgentContext) SetDocuments(docs []*document.Document) *AgentContext {
	r.documents = docs
	return r
}

func (r *AgentContext) SetDocumentReferences(docRefs []*document.Document) *AgentContext {
	r.documentReferences = docRefs
	return r
}

func (r *AgentContext) SetConversationHistory(history *ConversationHistory) *AgentContext {
	if history == nil {
		history = NewConversationHistory(r.execEnv)
	}
	r.conversationHistory = history
	return r
}

func (r *AgentContext) GetDocuments() []*document.Document {
	return r.documents
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
	r.documents = make([]*document.Document, 0)
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
