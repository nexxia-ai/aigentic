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
	msgHistory     []ai.Message
	currentMsg     int
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

func NewAgentContext(id, description, instructions string) *AgentContext {
	cm := &AgentContext{id: id, description: description, instructions: instructions}

	cm.conversationHistory = NewConversationHistory()
	cm.SetTemplates(DefaultSystemTemplate, DefaultUserTemplate)
	return cm
}

func (r *AgentContext) SetOutputInstructions(instructions string) {
	r.outputInstructions = instructions
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

func (r *AgentContext) SetTemplates(systemTemplate, userTemplate string) {
	r.SystemTemplate = template.Must(template.New("system").Parse(systemTemplate))
	r.UserTemplate = template.Must(template.New("user").Parse(userTemplate))
}

func (r *AgentContext) ParseSystemTemplate(templateStr string) error {
	tmpl, err := template.New("system").Parse(templateStr)
	if err != nil {
		return err
	}
	r.SystemTemplate = tmpl
	return nil
}

func (r *AgentContext) ParseUserTemplate(templateStr string) error {
	tmpl, err := template.New("user").Parse(templateStr)
	if err != nil {
		return err
	}
	r.UserTemplate = tmpl
	return nil
}

func (r *AgentContext) BuildPrompt(messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

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

	msgs = append(msgs, r.msgHistory...)
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

func (r *AgentContext) AddDocument(doc *document.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}
	r.documents = append(r.documents, doc)
	return nil
}

func (r *AgentContext) AddMemory(id, description, content, scope, runID string) error {
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
			return nil
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
	return nil
}

func (r *AgentContext) DeleteMemory(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i := range r.memories {
		if r.memories[i].ID == id {
			r.memories = append(r.memories[:i], r.memories[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *AgentContext) GetMemories() []MemoryEntry {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]MemoryEntry, len(r.memories))
	copy(result, r.memories)
	return result
}

func (r *AgentContext) SetDocuments(docs []*document.Document) {
	r.documents = docs
}

func (r *AgentContext) SetDocumentReferences(docRefs []*document.Document) {
	r.documentReferences = docRefs
}

func (r *AgentContext) SetConversationHistory(history *ConversationHistory) {
	if history == nil { // reset conversation history
		history = NewConversationHistory()
	}
	r.conversationHistory = history
}

func (r *AgentContext) ConversationHistory() *ConversationHistory {
	return r.conversationHistory
}

func (r *AgentContext) ConversationTurn() *ConversationTurn {
	return r.currentConversationTurn
}

func (r *AgentContext) SetConversationTurn(turn *ConversationTurn) {
	r.currentConversationTurn = turn
}
