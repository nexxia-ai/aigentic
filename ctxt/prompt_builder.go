package ctxt

import (
	"bytes"
	"fmt"

	"github.com/nexxia-ai/aigentic/ai"
)

const DefaultSystemTemplate = `
You are an autonomous agent working to complete a task.
You have to consider all the information you were given and reason about the next step to take.

{{if .Role}}
The user provided the following description of your role:
<role>
{{.Role}}
</role>
{{end}}

{{if .Instructions}}
 <instructions>
{{.Instructions}}
</instructions>
{{end}}

{{if .OutputInstructions}}
<output_instructions>
{{.OutputInstructions}}
</output_instructions>
{{end}}

{{if .Tools}}
You have access to the following tools:
<tools>
{{range .Tools}}<tool>
{{.Name}}
{{.Description}}
</tool>
{{end}}
</tools>
{{end}}

{{if .Memories}}
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
{{end}}

{{if .Documents}}
<documents_attached>
{{range .Documents}}
<document id="{{.ID}}" name="{{.Filename}}" type="{{.MimeType}}">
</document>
{{end}}
{{range .DocumentReferences}}
<document_reference id="{{.ID}}" name="{{.Filename}}" type="{{.MimeType}}">
</document_reference>
{{end}}
</documents_attached>
{{end}}`

func createSystemMsg(ac *AgentContext, tools []ai.Tool) (ai.Message, error) {
	memories := ac.GetMemories()
	var filteredMemories []MemoryEntry
	filteredMemories = append(filteredMemories, memories...)

	systemVars := map[string]any{
		"Role":               ac.description,
		"Instructions":       ac.instructions,
		"Tools":              tools,
		"Memories":           filteredMemories,
		"OutputInstructions": ac.outputInstructions,
	}

	var systemBuf bytes.Buffer
	if err := ac.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	sysMsg := ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()}
	return sysMsg, nil

}

func createUserMsg(ac *AgentContext, message string) (ai.Message, error) {

	userVars := map[string]any{
		"Message":            message,
		"HasMessage":         message != "",
		"Documents":          ac.documents,
		"DocumentReferences": ac.documentReferences,
	}
	var userBuf bytes.Buffer
	if err := ac.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}

	userMsg := ai.UserMessage{Role: ai.UserRole, Content: userBuf.String()}
	return userMsg, nil
}

func (r *AgentContext) BuildPrompt(tools []ai.Tool, includeHistory bool) ([]ai.Message, error) {

	// Add system message first
	sysMsg, err := createSystemMsg(r, tools)
	if err != nil {
		return nil, fmt.Errorf("failed to create system message: %w", err)
	}

	msgs := []ai.Message{}
	if sysMsg != nil {
		msgs = append(msgs, sysMsg)
	}

	// Add history messages before user message
	if includeHistory && r.conversationHistory != nil {
		historyMessages := r.conversationHistory.GetMessages()
		msgs = append(msgs, historyMessages...)
	}

	// Add user message before documents
	userMsg, err := createUserMsg(r, r.currentConversationTurn.UserMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to create user message: %w", err)
	}
	if userMsg != nil {
		msgs = append(msgs, userMsg)
	}

	// Add documents second
	msgs = append(msgs, r.insertDocuments(r.documents, r.documentReferences)...)

	// tool messages are last
	msgs = append(msgs, r.currentConversationTurn.messages...) // tool messages

	return msgs, nil
}
