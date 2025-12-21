package run

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/conversation"
	"github.com/nexxia-ai/aigentic/document"
)

type AgentContext struct {
	description    string
	instructions   string
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template

	mutex               sync.RWMutex
	memories            []MemoryEntry
	documents           []*document.Document
	documentReferences  []*document.Document
	conversationHistory *conversation.ConversationHistory
}

var _ ContextManager = &AgentContext{}

func NewAgentContext(description, instructions, userMsg string) *AgentContext {
	cm := &AgentContext{description: description, instructions: instructions, userMsg: userMsg}

	cm.conversationHistory = conversation.NewConversationHistory()
	cm.SetDefaultTemplates()
	return cm
}

func collectContextFunctions(run *AgentRun) string {
	var parts []string

	for _, fn := range run.ContextFunctions {
		output, err := fn(run)
		if err != nil {
			parts = append(parts, fmt.Sprintf("Error in context function: %v", err))
		} else if output != "" {
			parts = append(parts, output)
		}
	}

	for _, tool := range run.tools {
		for _, fn := range tool.ContextFunctions {
			output, err := fn(run)
			if err != nil {
				parts = append(parts, fmt.Sprintf("Error in tool context function: %v", err))
			} else if output != "" {
				parts = append(parts, output)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
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
{{if .HasSessionContext}}
<session_context>
{{.SessionContext}}
</session_context>
{{end}}

{{if .HasMessage}}Please answer the following request or task:
{{.Message}} 
{{end}}`

func (r *AgentContext) SetDefaultTemplates() {
	r.SystemTemplate = template.Must(template.New("system").Parse(DefaultSystemTemplate))
	r.UserTemplate = template.Must(template.New("user").Parse(DefaultUserTemplate))
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

func (r *AgentContext) BuildPrompt(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	systemVars := r.createSystemVariables(tools, run)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	userVars := r.createUserVariables(r.userMsg, run)
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

func (r *AgentContext) createSystemVariables(tools []ai.Tool, run *AgentRun) map[string]interface{} {
	return createSystemVariables(r, tools, run)
}

func (r *AgentContext) createUserVariables(message string, run *AgentRun) map[string]interface{} {
	return createUserVariables(r, message, run)
}

func createSystemVariables(ac *AgentContext, tools []ai.Tool, run *AgentRun) map[string]interface{} {
	memories := run.GetMemories()
	var filteredMemories []MemoryEntry
	for _, mem := range memories {
		if mem.Scope == "session" {
			filteredMemories = append(filteredMemories, mem)
		} else if mem.Scope == "run" && mem.RunID == run.ID() {
			filteredMemories = append(filteredMemories, mem)
		}
	}
	hasMemories := len(filteredMemories) > 0

	return map[string]interface{}{
		"HasTools":        len(tools) > 0,
		"Role":            ac.description,
		"Instructions":    ac.instructions,
		"Tools":           tools,
		"HasRole":         ac.description != "",
		"HasInstructions": ac.instructions != "",
		"Memories":        filteredMemories,
		"HasMemories":     hasMemories,
	}
}

func createUserVariables(ac *AgentContext, message string, run *AgentRun) map[string]interface{} {
	sessionContext := collectContextFunctions(run)
	hasSessionContext := sessionContext != ""

	return map[string]interface{}{
		"Message":            message,
		"HasMessage":         message != "",
		"Documents":          ac.documents,
		"DocumentReferences": ac.documentReferences,
		"SessionContext":     sessionContext,
		"HasSessionContext":  hasSessionContext,
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

func (r *AgentContext) SetConversationHistory(history *conversation.ConversationHistory) {
	r.conversationHistory = history
}
