package ctxt

import (
	"bytes"
	"fmt"
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
	SystemTemplate *template.Template
	UserTemplate   *template.Template

	mutex                   sync.RWMutex
	memories                []MemoryEntry
	documents               []*document.Document
	documentReferences      []*document.Document
	conversationHistory     *ConversationHistory
	outputInstructions      string
	currentConversationTurn *ConversationTurn
}

func New(id, description, instructions string) *AgentContext {
	ctx := &AgentContext{
		id:                 id,
		description:        description,
		instructions:       instructions,
		memories:           make([]MemoryEntry, 0),
		documents:          make([]*document.Document, 0),
		documentReferences: make([]*document.Document, 0),
	}
	ctx.conversationHistory = NewConversationHistory()
	ctx.UpdateSystemTemplate(DefaultSystemTemplate)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	return ctx
}

func NewAgentContext(id, description, instructions string) *AgentContext {
	return New(id, description, instructions)
}

func (r *AgentContext) SetOutputInstructions(instructions string) *AgentContext {
	r.outputInstructions = instructions
	return r
}

const DefaultSystemTemplate = `
You are an autonomous agent working to complete a task.
You have to consider all the information you were given and reason about the next step to take.

{{if .HasRole}}
The user provided the following description of your role:
<role>
{{.Role}}
</role>
{{end}}

{{if .HasInstructions}}
 <instructions>
{{.Instructions}}
</instructions>
{{end}}

{{if .HasOutputInstructions}}
<output_instructions>
{{.OutputInstructions}}
</output_instructions>
{{end}}

{{if .HasTools}}
You have access to the following tools:
<tools>
{{range .Tools}}<tool>
{{.Name}}
{{.Description}}
</tool>
{{end}}
</tools>
{{end}}

{{if .HasMemories}}
<memories>
{{range .Memories}}
<memory id="{{.ID}}" description="{{.Description}}">
{{.Content}}
</memory>
{{end}}
</memories>
{{end}}`

const DefaultUserTemplate = `
{{if .HasMessage}}Please answer the following request or task:
{{.Message}} 
{{end}}`

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
	return r
}

func (r *AgentContext) SetInstructions(instructions string) *AgentContext {
	r.instructions = instructions
	return r
}

func (r *AgentContext) BuildPrompt(messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	userMsg := ""
	if r.currentConversationTurn != nil {
		userMsg = r.currentConversationTurn.UserMessage
	}

	userVars := r.createUserVariables(userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}

	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	msgs = append(msgs, r.insertDocuments(r.documents, r.documentReferences)...)

	msgs = append(msgs, messages...)
	return msgs, nil
}

func (r *AgentContext) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	return createSystemVariables(r, tools)
}

func (r *AgentContext) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r, message)
}

func createSystemVariables(ac *AgentContext, tools []ai.Tool) map[string]interface{} {
	memories := ac.GetMemories()
	var filteredMemories []MemoryEntry
	filteredMemories = append(filteredMemories, memories...)
	hasMemories := len(filteredMemories) > 0

	return map[string]interface{}{
		"HasTools":              len(tools) > 0,
		"Role":                  ac.description,
		"Instructions":          ac.instructions,
		"Tools":                 tools,
		"HasRole":               ac.description != "",
		"HasInstructions":       ac.instructions != "",
		"Memories":              filteredMemories,
		"HasMemories":           hasMemories,
		"HasOutputInstructions": ac.outputInstructions != "",
		"OutputInstructions":    ac.outputInstructions,
	}
}

func createUserVariables(ac *AgentContext, message string) map[string]interface{} {

	return map[string]interface{}{
		"Message":            message,
		"HasMessage":         message != "",
		"Documents":          ac.documents,
		"DocumentReferences": ac.documentReferences,
	}
}

func (r *AgentContext) insertDocuments(docs []*document.Document, docRefs []*document.Document) []ai.Message {
	var msgs []ai.Message

	for _, doc := range docs {
		content, err := doc.Bytes()
		if err != nil {
			continue
		}

		attachmentMsg := ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  "",
			Name: doc.Filename,
			Body: content,
			Type: document.DeriveTypeFromMime(doc.MimeType),
		}
		msgs = append(msgs, attachmentMsg)
	}

	for _, docRef := range docRefs {
		fileID := docRef.ID()

		refMsg := ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  fmt.Sprintf("file://%s", fileID),
			Name: docRef.Filename,
			Body: nil,
			Type: document.DeriveTypeFromMime(docRef.MimeType),
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

func (r *AgentContext) AddMemory(id, description, content, scope, runID string) *AgentContext {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	for i := range r.memories {
		if r.memories[i].ID == id {
			r.memories[i].Description = description
			r.memories[i].Content = content
			r.memories[i].Scope = scope
			r.memories[i].RunID = runID
			r.memories[i].Timestamp = now
			return r
		}
	}

	r.memories = append(r.memories, MemoryEntry{
		ID:          id,
		Description: description,
		Content:     content,
		Scope:       scope,
		RunID:       runID,
		Timestamp:   now,
	})
	return r
}

func (r *AgentContext) RemoveMemory(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i := range r.memories {
		if r.memories[i].ID == id {
			r.memories = append(r.memories[:i], r.memories[i+1:]...)
			return nil
		}
	}
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
		history = NewConversationHistory()
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

func (r *AgentContext) StartTurn(userMessage string) *AgentContext {
	turn := NewConversationTurn(userMessage, r.id, r.description, r.instructions)
	r.currentConversationTurn = turn
	return r
}

func (r *AgentContext) EndTurn(msg ai.Message) *AgentContext {
	r.currentConversationTurn.AddMessage(msg)
	r.currentConversationTurn.Reply = msg
	r.currentConversationTurn.Compact()
	r.conversationHistory.appendTurn(*r.currentConversationTurn)
	return r
}

func (r *AgentContext) ConversationTurn() *ConversationTurn {
	return r.currentConversationTurn
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
