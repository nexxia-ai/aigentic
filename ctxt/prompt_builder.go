package ctxt

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

const defaultSystemIntro = `You are an autonomous agent working to complete a task.
You have to consider all the information you were given and reason about the next step to take.
`

const DefaultUserTemplate = `
{{if .TurnTags}}
{{range .TurnTags}}<{{.Key}}>{{.Value}}</{{.Key}}>
{{end}}
{{end}}

{{if .UserData}}
{{.UserData}}

{{end}}
{{if .UserMessage}}
{{.UserMessage}}
{{end}}

{{if .FileRefs}}
File references for this turn:
{{range .FileRefs}}  {{.}}
{{end}}
{{end}}
`

const promptHistoryTurnLimit = 100

func createSystemMsg(ac *AgentContext, tools []ai.Tool) (ai.Message, error) {
	var b bytes.Buffer
	b.WriteString(defaultSystemIntro)

	for _, p := range ac.SystemParts() {
		if p.Value == "" {
			continue
		}
		b.WriteString("\n<")
		b.WriteString(p.Key)
		b.WriteString(">\n")
		b.WriteString(p.Value)
		b.WriteString("\n</")
		b.WriteString(p.Key)
		b.WriteString(">\n")
	}

	if len(tools) > 0 {
		b.WriteString("\nYou have access to the following tools:\n<tools>\n")
		for _, t := range tools {
			b.WriteString("<tool>\n")
			b.WriteString(t.Name)
			b.WriteString("\n")
			b.WriteString(t.Description)
			b.WriteString("\n</tool>\n")
		}
		b.WriteString("</tools>\n")
	}

	ws := ac.Workspace()
	if ws != nil {
		docs, err := ws.MemoryFiles()
		if err != nil {
			slog.Error("failed to load memory files", "error", err)
		} else if len(docs) > 0 {
			for _, doc := range docs {
				relPath := doc.FilePath
				if ws.MemoryDir != "" && ws.LLMDir != "" {
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
				b.WriteString("\n<document name=\"")
				b.WriteString(relPath)
				b.WriteString("\">\n")
				b.WriteString(doc.Text())
				b.WriteString("\n</document>\n")
			}
		}
	}

	if t := ac.Turn(); t != nil && len(t.systemTags) > 0 {
		b.WriteString("\n")
		for _, tag := range t.systemTags {
			b.WriteString("<")
			b.WriteString(tag.Name)
			b.WriteString(">")
			b.WriteString(tag.Content)
			b.WriteString("</")
			b.WriteString(tag.Name)
			b.WriteString(">\n")
		}
	}

	return ai.SystemMessage{Role: ai.SystemRole, Content: b.String()}, nil
}

func createDocsMsg(ac *AgentContext) (ai.Message, error) {
	turn := ac.Turn()
	if turn == nil || len(turn.Files) == 0 {
		return nil, nil
	}

	seenPath := make(map[string]bool)
	norm := func(p string) string { return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/")) }

	var promptBuf bytes.Buffer
	promptBuf.WriteString("The following documents are available in this session:\n")
	for _, ref := range turn.Files {
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

	if promptBuf.Len() == 0 {
		return nil, nil
	}

	userMsg := ai.UserMessage{Role: ai.UserRole, Content: promptBuf.String()}
	return userMsg, nil
}

func createUserMsgForTurn(ac *AgentContext, turn *Turn) (ai.Message, error) {
	var turnTags []ai.KeyValue
	var fileRefPaths []string
	norm := func(p string) string { return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/")) }
	seenPath := make(map[string]bool)
	userMessage := ""
	userData := ""
	if turn != nil {
		turnTags = turn.TurnTags()
		for _, ref := range turn.Files {
			p := norm(ref.Path)
			if p != "" && !seenPath[p] {
				seenPath[p] = true
				fileRefPaths = append(fileRefPaths, ref.Path)
			}
		}
		userMessage = turn.UserMessage
		userData = turn.UserData
	}
	userVars := map[string]any{
		"UserMessage": userMessage,
		"UserData":    userData,
		"TurnTags":    turnTags,
		"FileRefs":    fileRefPaths,
	}
	var userBuf bytes.Buffer
	if err := ac.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user template: %w", err)
	}
	return ai.UserMessage{Role: ai.UserRole, Content: userBuf.String()}, nil
}

func createUserMsg(ac *AgentContext) (ai.Message, error) {
	return createUserMsgForTurn(ac, ac.Turn())
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
		historyMessages := r.conversationHistory.getMessages(promptHistoryTurnLimit, r)
		msgs = append(msgs, historyMessages...)
	}

	docsMsg, _ := createDocsMsg(r)
	if docsMsg != nil {
		msgs = append(msgs, docsMsg)
	}

	// Add user message before documents
	userMsg, err := createUserMsg(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create user message: %w", err)
	}
	if userMsg != nil {
		msgs = append(msgs, userMsg)
	}

	// Add document content for files with IncludeInPrompt
	for _, ref := range r.currentTurn.PromptFiles() {
		doc, err := OpenFileRef(ref)
		if err != nil {
			slog.Warn("failed to open file for prompt", "path", ref.Path, "error", err)
			continue
		}
		if ref.MimeType != "" {
			doc.MimeType = ref.MimeType
		}
		msgs = append(msgs, r.insertDocuments([]*document.Document{doc})...)
	}

	// tool messages are last
	msgs = append(msgs, r.currentTurn.messages...) // tool messages

	return msgs, nil
}

func OpenFileRef(ref FileRef) (*document.Document, error) {
	docID := strings.TrimSpace(ref.Path)
	resolvedPath := docID
	if strings.TrimSpace(ref.BasePath) != "" && !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(ref.BasePath, filepath.FromSlash(resolvedPath))
	}
	if resolvedPath == "" {
		return nil, fmt.Errorf("file path not set")
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(resolvedPath)
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = filepath.Base(resolvedPath)
	}
	if docID == "" {
		docID = resolvedPath
	}
	doc := document.NewInMemoryDocument(docID, name, data, nil)
	doc.FilePath = resolvedPath
	if ref.MimeType != "" {
		doc.MimeType = ref.MimeType
	}
	return doc, nil
}
