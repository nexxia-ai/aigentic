package aigentic

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type ContextManager interface {
	BuildPrompt(*AgentRun, []ai.Message, []ai.Tool) ([]ai.Message, error)
}

// MemoryEntry represents a single memory entry
type MemoryEntry struct {
	ID          string
	Description string
	Content     string
	Scope       string
	RunID       string
	Timestamp   time.Time
}

type AgentContext struct {
	agent          Agent // copy of agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template

	// Thread-safe memory storage
	mutex     sync.RWMutex
	memories  []MemoryEntry
	documents []*document.Document
}

var _ ContextManager = &AgentContext{}

func collectContextFunctions(agent Agent, run *AgentRun) string {
	var parts []string

	for _, fn := range agent.ContextFunctions {
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

func NewAgentContext(agent Agent, userMsg string) *AgentContext {
	cm := &AgentContext{agent: agent, userMsg: userMsg}
	cm.SetDefaultTemplates()
	return cm
}

// SetDefaultTemplates sets the default system and user templates
func (r *AgentContext) SetDefaultTemplates() {
	r.SystemTemplate = template.Must(template.New("system").Parse(DefaultSystemTemplate))
	r.UserTemplate = template.Must(template.New("user").Parse(DefaultUserTemplate))
}

// ParseSystemTemplate parses and sets a custom system template
func (r *AgentContext) ParseSystemTemplate(templateStr string) error {
	tmpl, err := template.New("system").Parse(templateStr)
	if err != nil {
		return err
	}
	r.SystemTemplate = tmpl
	return nil
}

// ParseUserTemplate parses and sets a custom user template
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

	// Generate system prompt using template
	systemVars := r.createSystemVariables(tools, run)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	// Generate user prompt using template
	userVars := r.createUserVariables(r.userMsg, run)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}

	// Create user message
	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	// Add documents to the prompt
	msgs = append(msgs, r.addDocuments(r.agent)...)

	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

func (r *AgentContext) createSystemVariables(tools []ai.Tool, run *AgentRun) map[string]interface{} {
	return createSystemVariables(r.agent, tools, run)
}

func (r *AgentContext) createUserVariables(message string, run *AgentRun) map[string]interface{} {
	return createUserVariables(r.agent, message, run)
}

// Package-level utility functions for reuse across context managers
func createSystemVariables(agent Agent, tools []ai.Tool, run *AgentRun) map[string]interface{} {
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
		"Role":            agent.Description,
		"Instructions":    agent.Instructions,
		"Tools":           tools,
		"HasRole":         agent.Description != "",
		"HasInstructions": agent.Instructions != "",
		"Memories":        filteredMemories,
		"HasMemories":     hasMemories,
	}
}

func createUserVariables(agent Agent, message string, run *AgentRun) map[string]interface{} {
	sessionContext := collectContextFunctions(agent, run)
	hasSessionContext := sessionContext != ""

	return map[string]interface{}{
		"Message":            message,
		"HasMessage":         message != "",
		"Documents":          agent.Documents,
		"DocumentReferences": agent.DocumentReferences,
		"SessionContext":     sessionContext,
		"HasSessionContext":  hasSessionContext,
	}
}

func (r *AgentContext) addDocuments(agent Agent) []ai.Message {
	var msgs []ai.Message

	// Add document attachments as separate Resource messages
	for _, doc := range r.documents {
		content, err := doc.Bytes()
		if err != nil {
			continue // skip
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

	// Add attachment references as Resource messages with file:// URI
	for _, docRef := range agent.DocumentReferences {
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

// AddMemory adds a new memory entry or updates an existing one by ID
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

// DeleteMemory removes a memory entry by ID
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

// GetMemories returns all memories in insertion order
func (r *AgentContext) GetMemories() []MemoryEntry {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make([]MemoryEntry, len(r.memories))
	copy(result, r.memories)
	return result
}
