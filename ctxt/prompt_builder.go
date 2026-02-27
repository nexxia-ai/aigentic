package ctxt

import (
	"bytes"
	"context"
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

{{if .Skills}}
You have access to the following skills:
<available_skills>
{{range .Skills}}<skill>
<name>{{.Name}}</name>
<description>{{.Description}}</description>
<location>{{.Location}}</location>
</skill>
{{end}}</available_skills>
Skills are executable instructions, not references.
When a user request matches a skill's name or description, you must load that skill before taking other actions.

Skill loading policy:
1. Identify the most relevant skill(s) from <available_skills>.
2. Use read_file with the skill <location> to load the full SKILL content.
3. Read selected skills before planning, tool calls, or substantive responses.
4. Do not assume skill behavior from name/description alone.
5. If multiple skills apply, load all relevant ones; resolve conflicts by priority: system/developer/user instructions, then skill instructions.
6. If no skill applies, continue normally.
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

const (
	maxSkillsInSystemPrompt       = 50
	maxSkillDescriptionPromptChar = 200
)

type skillSummary struct {
	Name        string
	Description string
	Location    string
}

const DefaultUserTemplate = `
{{if .HasMessage}}
{{.Message}} 
{{end}}

{{if .UserTags}}
{{range .UserTags}}<{{.Name}}>{{.Content}}</{{.Name}}>
{{end}}
{{end}}

{{if .FileRefs}}
File references for this turn:
{{range .FileRefs}}  {{.}}
{{end}}
{{end}}
`

func createSystemMsg(ac *AgentContext, tools []ai.Tool) (ai.Message, error) {

	ws := ac.Workspace()
	memoryDocs := make([]*document.Document, 0)
	if ws != nil {
		docs, err := ws.MemoryFiles()
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
		if ws != nil && ws.MemoryDir != "" && ws.LLMDir != "" {
			absLLM, errLLM := filepath.Abs(ws.LLMDir)
			absMem, errMem := filepath.Abs(ws.MemoryDir)
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

	systemTags := []tag{}
	if t := ac.Turn(); t != nil {
		systemTags = t.systemTags
	}
	skills := ac.Skills()
	summaries := make([]skillSummary, 0, len(skills))
	for i, skill := range skills {
		if i >= maxSkillsInSystemPrompt {
			break
		}
		desc := strings.TrimSpace(skill.Description)
		if len(desc) > maxSkillDescriptionPromptChar {
			desc = desc[:maxSkillDescriptionPromptChar] + "..."
		}
		name := strings.TrimSpace(skill.Name)
		if name == "" {
			name = strings.TrimSpace(skill.ID)
		}
		summaries = append(summaries, skillSummary{
			Name:        name,
			Description: desc,
			Location:    strings.TrimSpace(skill.Source),
		})
	}
	systemVars := map[string]any{
		"Role":               ac.description,
		"Instructions":       ac.instructions,
		"Tools":              tools,
		"Skills":             summaries,
		"Documents":          docsForTemplate,
		"OutputInstructions": ac.outputInstructions,
		"SystemTags":         systemTags,
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
	turn := ac.Turn()

	if len(docs) == 0 && (turn == nil || len(turn.FileRefs) == 0) {
		return nil, nil
	}

	seenPath := make(map[string]bool)
	norm := func(p string) string { return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/")) }

	var allDocs []*document.Document
	for _, doc := range docs {
		if doc != nil && doc.Filename != "" {
			id := norm(doc.ID())
			if id != "" && !seenPath[id] {
				seenPath[id] = true
				allDocs = append(allDocs, doc)
			}
		}
	}

	var promptBuf bytes.Buffer
	promptBuf.WriteString("The following documents are available in this session:\n")
	for _, doc := range allDocs {
		mimeType := doc.MimeType
		if mimeType == "" {
			mimeType = "unknown"
		}
		promptBuf.WriteString(fmt.Sprintf("  %s, Filename: %s, Type: %s\n", doc.ID(), doc.Filename, mimeType))
	}

	// Show FileRefs with MimeType when available
	if turn != nil {
		for _, ref := range turn.FileRefs {
			p := norm(ref.Path)
			if p == "" || seenPath[p] {
				continue
			}
			seenPath[p] = true
			if ref.MimeType != "" {
				promptBuf.WriteString(fmt.Sprintf("  %s, Type: %s\n", ref.Path, ref.MimeType))
			} else {
				promptBuf.WriteString(fmt.Sprintf("  %s\n", ref.Path))
			}
		}
	}

	if promptBuf.Len() == 0 {
		return nil, nil
	}

	userMsg := ai.UserMessage{Role: ai.UserRole, Content: promptBuf.String()}
	return userMsg, nil
}

func createUserMsg(ac *AgentContext, message string) (ai.Message, error) {
	userTags := []tag{}
	var fileRefPaths []string
	norm := func(p string) string { return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/")) }
	seenPath := make(map[string]bool)
	if t := ac.Turn(); t != nil {
		userTags = t.userTags
		for _, ref := range t.FileRefs {
			p := norm(ref.Path)
			if p != "" && !seenPath[p] {
				seenPath[p] = true
				fileRefPaths = append(fileRefPaths, ref.Path)
			}
		}
	}
	userVars := map[string]any{
		"Message":    message,
		"HasMessage": message != "",
		"UserTags":   userTags,
		"FileRefs":   fileRefPaths,
	}
	var userBuf bytes.Buffer
	if err := ac.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}
	return ai.UserMessage{Role: ai.UserRole, Content: userBuf.String()}, nil
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

	// Add compaction summaries (if any) between system message and history
	if r.conversationHistory != nil {
		for _, s := range r.conversationHistory.GetSummaries() {
			msgs = append(msgs, ai.UserMessage{
				Role:    ai.UserRole,
				Content: fmt.Sprintf("[Summary for %s]: %s", s.Date.Format("2006-01-02"), s.Summary),
			})
		}
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
	msgs = append(msgs, r.insertDocuments(r.Turn().GetDocuments())...)

	// Add ephemeral document content for FileRefs with IncludeInPrompt
	for _, ref := range r.currentTurn.FileRefs {
		if !ref.IncludeInPrompt {
			continue
		}
		doc, err := r.openDocumentByPath(ref.Path)
		if err != nil {
			slog.Warn("failed to open file for prompt", "path", ref.Path, "error", err)
			continue
		}
		// Use Python-provided MIME type if available
		if ref.MimeType != "" {
			doc.MimeType = ref.MimeType
		}
		msgs = append(msgs, r.insertDocuments([]*document.Document{doc})...)
	}

	// tool messages are last
	msgs = append(msgs, r.currentTurn.messages...) // tool messages

	return msgs, nil
}

func (r *AgentContext) openDocumentByPath(path string) (*document.Document, error) {
	if r.workspace == nil {
		return nil, fmt.Errorf("workspace not set")
	}
	store, err := r.workspace.llmStore()
	if err != nil {
		return nil, err
	}
	return document.Open(context.Background(), store.ID(), path)
}
