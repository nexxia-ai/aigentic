package aigentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type ContextManager interface {
	BuildPrompt(context.Context, []ai.Message, []ai.Tool) ([]ai.Message, error)
}

type BasicContextManager struct {
	agent          Agent // copy of agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template
}

var _ ContextManager = &BasicContextManager{}

const DefaultSystemTemplate = `
You are an autonomous agent working to complete a task.
You have to consider all the information you were given and reason about the next step to take.
Analyse the tools you have already used to ensure you are not repeating yourself.
{{if .HasTools}}
You have access to one or more tools to complete the task. Use these tools as required to complete the task.
{{end}}

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
{{if .HasMemory}}

<memory>
{{.Memory}}
</memory>

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
{{if .HasToolHistory}}
You have already used the following tools:

<tool_call_history>
{{.ToolHistory}}
</tool_call_history>

{{end}}`

const DefaultUserTemplate = `{{if .HasMemory}}This is the content of the run memory:
<run_memory>
{{.MemoryContent}}
</run_memory>

{{end}}{{if .HasMessage}}Please answer the following request or task:
{{.Message}} 

{{end}}`

func NewBasicContextManager(agent Agent, userMsg string) *BasicContextManager {
	cm := &BasicContextManager{agent: agent, userMsg: userMsg}
	cm.SetDefaultTemplates()
	return cm
}

// SetDefaultTemplates sets the default system and user templates
func (r *BasicContextManager) SetDefaultTemplates() {
	r.SystemTemplate = template.Must(template.New("system").Parse(DefaultSystemTemplate))
	r.UserTemplate = template.Must(template.New("user").Parse(DefaultUserTemplate))
}

// ParseSystemTemplate parses and sets a custom system template
func (r *BasicContextManager) ParseSystemTemplate(templateStr string) error {
	tmpl, err := template.New("system").Parse(templateStr)
	if err != nil {
		return err
	}
	r.SystemTemplate = tmpl
	return nil
}

// ParseUserTemplate parses and sets a custom user template
func (r *BasicContextManager) ParseUserTemplate(templateStr string) error {
	tmpl, err := template.New("user").Parse(templateStr)
	if err != nil {
		return err
	}
	r.UserTemplate = tmpl
	return nil
}

func (r *BasicContextManager) BuildPrompt(ctx context.Context, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	// Generate system prompt using template
	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	// Generate user prompt using template
	userVars := r.createUserVariables(r.userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}

	// Create user message
	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	// Add documents using shared function
	msgs = append(msgs, addDocuments(r.agent)...)

	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

func (r *BasicContextManager) createToolHistory() string {
	return createToolHistory(r.msgHistory, r.currentMsg)
}

func (r *BasicContextManager) createToolHistoryVariables() string {
	return r.createToolHistory()
}

func (r *BasicContextManager) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	toolHistory := r.createToolHistoryVariables()
	return createSystemVariables(r.agent, tools, toolHistory)
}

func (r *BasicContextManager) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r.agent, message)
}

// Package-level utility functions for reuse across context managers
func createSystemVariables(agent Agent, tools []ai.Tool, toolHistory string) map[string]interface{} {
	var memorySystemPrompt string
	var hasMemory bool
	if agent.Memory != nil {
		memorySystemPrompt = agent.Memory.SystemPrompt()
		hasMemory = memorySystemPrompt != ""
	}

	hasToolHistory := strings.TrimSpace(toolHistory) != ""

	return map[string]interface{}{
		"HasTools":        len(tools) > 0,
		"Role":            agent.Description,
		"Instructions":    agent.Instructions,
		"Memory":          memorySystemPrompt,
		"Tools":           tools,
		"ToolHistory":     toolHistory,
		"HasRole":         agent.Description != "",
		"HasInstructions": agent.Instructions != "",
		"HasMemory":       hasMemory,
		"HasToolHistory":  hasToolHistory,
	}
}

func createUserVariables(agent Agent, message string) map[string]interface{} {
	var memoryContent string
	var hasMemory bool
	if agent.Memory != nil {
		memoryContent = agent.Memory.GetRunMemoryContent()
		hasMemory = memoryContent != ""
	}

	return map[string]interface{}{
		"Message":            message,
		"MemoryContent":      memoryContent,
		"HasMemory":          hasMemory,
		"HasMessage":         message != "",
		"Documents":          agent.Documents,
		"DocumentReferences": agent.DocumentReferences,
	}
}

func createToolHistory(msgHistory []ai.Message, currentMsg int) string {
	msg := ""

	// Create a map to store tool response messages by tool call ID
	toolResponses := make(map[string]ai.ToolMessage)

	// First pass: collect all tool response messages
	for _, history := range msgHistory[0:currentMsg] {
		if toolMsg, ok := history.(ai.ToolMessage); ok && toolMsg.Role == ai.ToolRole {
			toolResponses[toolMsg.ToolCallID] = toolMsg
		}
	}

	// Second pass: process AI messages with tool calls
	for _, history := range msgHistory[0:currentMsg] {
		if aiMsg, ok := history.(ai.AIMessage); ok && aiMsg.Role == ai.AssistantRole {
			// Process each tool call in this AI message
			for _, toolCall := range aiMsg.ToolCalls {
				// Find the corresponding tool response
				if toolResponse, found := toolResponses[toolCall.ID]; found {
					// Create JSON strings for parameters and result
					toolParams := toolCall.Args
					if toolParams == "" {
						toolParams = "{}"
					}

					var toolResult string
					if resultBytes, err := json.Marshal(toolResponse.Content); err == nil {
						toolResult = string(resultBytes)
					} else {
						toolResult = fmt.Sprintf("\"%s\"", toolResponse.Content)
					}

					msg += fmt.Sprintf("<tool_called>\ntool_name: %s\ntool_call_id: %s\ntool_parameters: %s\ntool_result: %s\n</tool_called>\n",
						toolCall.Name, toolCall.ID, toolParams, toolResult)
				}
			}
		}
	}

	return msg
}

func addDocuments(agent Agent) []ai.Message {
	var msgs []ai.Message

	// Add document attachments as separate Resource messages
	for _, doc := range agent.Documents {
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
