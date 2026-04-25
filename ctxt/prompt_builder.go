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

// systemPartPromptOrder is the canonical order of well-known keys in the system prompt (goal before instructions).
var systemPartPromptOrder = []string{
	SystemPartKeyDescription,
	SystemPartKeyGoal,
	SystemPartKeyInstructions,
	SystemPartKeyOutputInstructions,
	SystemPartKeySkills,
}

func orderedSystemPartsForPrompt(parts []PromptPart) []PromptPart {
	known := make(map[string]struct{}, len(systemPartPromptOrder))
	for _, k := range systemPartPromptOrder {
		known[k] = struct{}{}
	}
	byKey := make(map[string]string)
	for _, p := range parts {
		byKey[p.Key] = p.Value
	}
	var out []PromptPart
	for _, k := range systemPartPromptOrder {
		v := strings.TrimSpace(byKey[k])
		if v == "" {
			continue
		}
		out = append(out, PromptPart{Key: k, Value: byKey[k]})
	}
	seenUnknown := make(map[string]struct{})
	for _, p := range parts {
		if _, isKnown := known[p.Key]; isKnown {
			continue
		}
		if _, dup := seenUnknown[p.Key]; dup {
			continue
		}
		if strings.TrimSpace(p.Value) == "" {
			continue
		}
		seenUnknown[p.Key] = struct{}{}
		out = append(out, PromptPart{Key: p.Key, Value: byKey[p.Key]})
	}
	return out
}

func createSystemMsg(ac *AgentContext, tools []ai.Tool) (ai.Message, error) {
	var b bytes.Buffer
	b.WriteString(defaultSystemIntro)

	for _, p := range orderedSystemPartsForPrompt(ac.SystemParts()) {
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

	norm := func(p string) string { return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "/")) }
	renderRef := func(buf *bytes.Buffer, ref FileRef) {
		if ref.MimeType != "" {
			buf.WriteString(fmt.Sprintf("  %s, Type: %s\n", ref.Path, ref.MimeType))
			return
		}
		buf.WriteString(fmt.Sprintf("  %s\n", ref.Path))
	}

	var promptBuf bytes.Buffer
	promptFiles := turn.PromptFiles()
	includedByPath := make(map[string]struct{}, len(promptFiles))
	seenIncluded := make(map[string]bool)
	for _, ref := range promptFiles {
		p := norm(ref.Path)
		if p == "" || seenIncluded[p] {
			continue
		}
		seenIncluded[p] = true
		includedByPath[p] = struct{}{}
	}

	seenOnDisk := make(map[string]bool)
	var onDiskRefs []FileRef
	for _, ref := range turn.Files {
		p := norm(ref.Path)
		if p == "" || seenOnDisk[p] {
			continue
		}
		seenOnDisk[p] = true
		if _, ok := includedByPath[p]; ok {
			continue
		}
		onDiskRefs = append(onDiskRefs, ref)
	}

	if len(onDiskRefs) > 0 {
		promptBuf.WriteString("Files available on disk (use filesystem.read_text to load):\n")
		for _, ref := range onDiskRefs {
			renderRef(&promptBuf, ref)
		}
	}

	if len(seenIncluded) > 0 {
		if promptBuf.Len() > 0 {
			promptBuf.WriteString("\n")
		}
		promptBuf.WriteString("Files included below in this turn:\n")
		for _, ref := range promptFiles {
			p := norm(ref.Path)
			if p == "" {
				continue
			}
			if !seenIncluded[p] {
				continue
			}
			renderRef(&promptBuf, ref)
			delete(seenIncluded, p)
		}
	}

	if promptBuf.Len() == 0 {
		return nil, nil
	}

	userMsg := ai.UserMessage{Role: ai.UserRole, Content: promptBuf.String()}
	return userMsg, nil
}

const contextMapBucketLimit = 50

func createContextMapMsg(ac *AgentContext) (ai.Message, error) {
	turn := ac.Turn()
	if turn == nil {
		return nil, nil
	}
	byPath := make(map[string]struct{})
	var injected []FileRef
	var onDisk []FileRef
	var generated []FileRef
	for _, ref := range turn.Files {
		path := strings.TrimSpace(ref.Path)
		if path == "" {
			continue
		}
		key := filepath.ToSlash(strings.TrimPrefix(path, "/"))
		if _, seen := byPath[key]; seen {
			continue
		}
		byPath[key] = struct{}{}
		if ref.IncludeInPrompt {
			injected = append(injected, ref)
		} else if ref.IsUserUpload() || ref.IsReference() || ref.IsToolArtifact() {
			onDisk = append(onDisk, ref)
		}
		if !ref.AddedAt.IsZero() && !turn.StartFileCutoff.IsZero() && ref.AddedAt.After(turn.StartFileCutoff) {
			generated = append(generated, ref)
		}
	}
	if ws := ac.Workspace(); ws != nil {
		docs, err := ws.MemoryFiles()
		if err != nil {
			slog.Error("failed to load memory files", "error", err)
		} else {
			for _, doc := range docs {
				path := memoryPromptPath(ws, doc.FilePath)
				if strings.TrimSpace(path) == "" {
					continue
				}
				key := filepath.ToSlash(strings.TrimPrefix(path, "/"))
				if _, seen := byPath[key]; seen {
					continue
				}
				byPath[key] = struct{}{}
				onDisk = append(onDisk, FileRef{
					Path:      path,
					MimeType:  doc.MimeType,
					Role:      FileRoleReference,
					SizeBytes: doc.FileSize,
				})
			}
		}
	}
	stateBlock := strings.TrimSpace(ac.StateBlock())
	if len(injected) == 0 && len(onDisk) == 0 && len(generated) == 0 && stateBlock == "" {
		return nil, nil
	}

	writeBucket := func(buf *bytes.Buffer, name string, refs []FileRef) {
		buf.WriteString(name)
		buf.WriteString(":\n")
		if len(refs) == 0 {
			buf.WriteString("  (none)\n")
			return
		}
		limit := len(refs)
		if limit > contextMapBucketLimit {
			limit = contextMapBucketLimit
		}
		for i := 0; i < limit; i++ {
			ref := refs[i]
			buf.WriteString("  - path: ")
			buf.WriteString(ref.Path)
			buf.WriteString("\n")
			if ref.SizeBytes > 0 {
				buf.WriteString(fmt.Sprintf("    size_bytes: %d\n", ref.SizeBytes))
			}
			if ref.MimeType != "" {
				buf.WriteString("    mime_type: ")
				buf.WriteString(ref.MimeType)
				buf.WriteString("\n")
			}
		}
		if len(refs) > limit {
			buf.WriteString(fmt.Sprintf("  ... (%d more)\n", len(refs)-limit))
		}
	}

	var buf bytes.Buffer
	buf.WriteString("Context map for this turn:\n<context_map>\n")
	writeBucket(&buf, "injected", injected)
	writeBucket(&buf, "on_disk", onDisk)
	writeBucket(&buf, "generated_this_turn", generated)
	buf.WriteString("package_state:\n")
	if stateBlock == "" {
		buf.WriteString("  (none)\n")
	} else {
		for _, line := range strings.Split(stateBlock, "\n") {
			buf.WriteString("  ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	buf.WriteString("</context_map>")
	return ai.UserMessage{Role: ai.UserRole, Content: buf.String()}, nil
}

func memoryPromptPath(ws *Workspace, path string) string {
	if ws == nil {
		return path
	}
	relPath := path
	if ws.MemoryDir != "" && ws.LLMDir != "" {
		absLLM, errLLM := filepath.Abs(ws.LLMDir)
		absMem, errMem := filepath.Abs(ws.MemoryDir)
		if errLLM == nil && errMem == nil {
			fullPath := filepath.Join(absMem, path)
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
	return filepath.ToSlash(relPath)
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
		historyMessages := r.conversationHistory.getMessages(0, r)
		msgs = append(msgs, historyMessages...)
	}

	docsMsg, _ := createDocsMsg(r)
	if docsMsg != nil {
		msgs = append(msgs, docsMsg)
	}

	contextMapMsg, _ := createContextMapMsg(r)
	if contextMapMsg != nil {
		msgs = append(msgs, contextMapMsg)
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
	policy := DefaultInjectionPolicy()
	usedBytes := 0
	for _, ref := range r.currentTurn.PromptFiles() {
		// Tool artifacts are injected through their tool response so the next LLM call sees them once.
		if ref.ToolID != "" {
			continue
		}
		doc, err := OpenFileRef(ref)
		if err != nil {
			slog.Warn("failed to open file for prompt", "path", ref.Path, "error", err)
			continue
		}
		if ref.MimeType != "" {
			doc.MimeType = ref.MimeType
		}
		data, err := doc.Bytes()
		if err != nil {
			slog.Warn("failed to read file for prompt", "path", ref.Path, "error", err)
			continue
		}
		rendered := RenderInjectedText(ref.Path, data, policy, usedBytes)
		if rendered.Omitted {
			continue
		}
		usedBytes += len(rendered.Text)
		if rendered.Truncated {
			msgs = append(msgs, ai.UserMessage{
				Role:    ai.UserRole,
				Content: fmt.Sprintf("Content of %s:\n\n%s", ref.Path, rendered.Text),
			})
			continue
		}
		msgs = append(msgs, r.insertDocuments([]*document.Document{doc})...)
	}

	// tool messages are last
	msgs = append(msgs, r.currentTurn.messages...) // tool messages

	return msgs, nil
}

func OpenFileRef(ref FileRef) (*document.Document, error) {
	docID := strings.TrimSpace(ref.Path)
	resolvedPath, err := resolveFileRefPath(ref)
	if err != nil {
		return nil, err
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

func resolveFileRefPath(ref FileRef) (string, error) {
	docID := strings.TrimSpace(ref.Path)
	if docID == "" {
		return "", fmt.Errorf("file path not set")
	}
	resolvedPath := filepath.FromSlash(docID)
	basePath := strings.TrimSpace(ref.BasePath)
	if basePath == "" {
		return resolvedPath, nil
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("file base path: %w", err)
	}
	if evalBase, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = evalBase
	}

	if filepath.IsAbs(resolvedPath) {
		resolvedPath, err = filepath.Abs(resolvedPath)
	} else {
		resolvedPath, err = filepath.Abs(filepath.Join(absBase, resolvedPath))
	}
	if err != nil {
		return "", fmt.Errorf("file path: %w", err)
	}
	if evalPath, err := filepath.EvalSymlinks(resolvedPath); err == nil {
		resolvedPath = evalPath
	}

	rel, err := filepath.Rel(absBase, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("file path relative to base: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file path escapes base path: %s", ref.Path)
	}
	return resolvedPath, nil
}
