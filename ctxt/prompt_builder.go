package ctxt

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
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

{{if .Documents}}
{{range .Documents}}
<document name="{{.Filename}}">
{{.Text}}
</document>
{{end}}
{{end}}

{{if .SystemTags}}
{{range .SystemTags}}<{{.Name}}>{{.Content}}</{{.Name}}>
{{end}}
{{end}}`

const DefaultUserTemplate = `
{{if .HasMessage}}
{{.Message}} 
{{end}}

{{if .UserTags}}
{{range .UserTags}}<{{.Name}}>{{.Content}}</{{.Name}}>
{{end}}
{{end}}

`

func createSystemMsg(ac *AgentContext, tools []ai.Tool) (ai.Message, error) {

	ee := ac.ExecutionEnvironment()
	memoryDocs := make([]*document.Document, 0)
	if ee != nil {
		docs, err := ee.MemoryFiles()
		if err != nil {
			slog.Error("failed to load memory files", "error", err)
		} else {
			memoryDocs = docs
		}
	}

	docsForTemplate := make([]struct {
		Filename string
		Text     string
	}, 0, len(memoryDocs))
	for _, doc := range memoryDocs {
		relPath := doc.FilePath
		if ee != nil && ee.MemoryDir != "" && ee.LLMDir != "" {
			absLLM, errLLM := filepath.Abs(ee.LLMDir)
			absMem, errMem := filepath.Abs(ee.MemoryDir)
			if errLLM == nil && errMem == nil {
				fullPath := filepath.Join(absMem, doc.FilePath)
				if r, err := filepath.Rel(absLLM, fullPath); err == nil {
					joined := filepath.Join(".", r)
					prefix := "." + string(filepath.Separator)
					if !strings.HasPrefix(joined, prefix) {
						joined = prefix + joined
					}
					relPath = joined
				}
			}
		}
		docsForTemplate = append(docsForTemplate, struct {
			Filename string
			Text     string
		}{
			Filename: relPath,
			Text:     doc.Text(),
		})
	}

	systemVars := map[string]any{
		"Role":               ac.description,
		"Instructions":       ac.instructions,
		"Tools":              tools,
		"Documents":          docsForTemplate,
		"OutputInstructions": ac.outputInstructions,
		"SystemTags":         ac.Turn().systemTags,
	}

	var systemBuf bytes.Buffer
	if err := ac.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute system template: %w", err)
	}

	sysMsg := ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()}
	return sysMsg, nil

}

func createDocsMsg(ac *AgentContext) (ai.Message, error) {
	docs := ac.GetDocuments()
	docRefs := ac.GetDocumentReferences()

	if len(docs) == 0 && len(docRefs) == 0 {
		return nil, nil
	}

	var allDocs []*document.Document
	for _, doc := range docs {
		if doc != nil && doc.Filename != "" {
			allDocs = append(allDocs, doc)
		}
	}
	for _, doc := range docRefs {
		if doc != nil && doc.Filename != "" {
			allDocs = append(allDocs, doc)
		}
	}

	if len(allDocs) == 0 {
		return nil, nil
	}

	var promptBuf bytes.Buffer
	promptBuf.WriteString("The following documents are available in this session:\n")
	for _, doc := range allDocs {
		mimeType := doc.MimeType
		if mimeType == "" {
			mimeType = "unknown"
		}
		promptBuf.WriteString(fmt.Sprintf("- FQN: %s, Filename: %s, Type: %s\n", doc.ID(), doc.Filename, mimeType))
	}

	userMsg := ai.UserMessage{Role: ai.UserRole, Content: promptBuf.String()}
	return userMsg, nil
}

func createUserMsg(ac *AgentContext, message string) (ai.Message, error) {

	userVars := map[string]any{
		"Message":    message,
		"HasMessage": message != "",
		"UserTags":   ac.Turn().userTags,
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

	docsMsg, _ := createDocsMsg(r)
	if docsMsg != nil {
		msgs = append(msgs, docsMsg)
	}

	// Add user message before documents
	userMsg, err := createUserMsg(r, r.currentTurn.UserMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to create user message: %w", err)
	}
	if userMsg != nil {
		msgs = append(msgs, userMsg)
	}

	// Add documents second (including memory files)
	msgs = append(msgs, r.insertDocuments(r.Turn().GetDocuments(), r.documentReferences)...)

	// tool messages are last
	msgs = append(msgs, r.currentTurn.messages...) // tool messages

	return msgs, nil
}
