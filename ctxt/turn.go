package ctxt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type documentEntry struct {
	Document *document.Document `json:"-"`
	ToolID   string             `json:"tool_id"`
}

type FileRefEntry struct {
	Path            string `json:"path"`
	FileID          string `json:"file_id,omitempty"`
	IncludeInPrompt bool   `json:"include_in_prompt"`
	MimeType        string `json:"mime_type,omitempty"`
	Ephemeral       bool   `json:"ephemeral"`
	UserUpload      bool   `json:"user_upload"`
}

type tag struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type TagEntry struct {
	Name    string
	Content string
}

type Turn struct {
	TurnID       string          `json:"turn_id"`
	agentContext *AgentContext   `json:"-"`
	Request      ai.Message      `json:"-"`
	UserMessage  string          `json:"user_message"`
	messages     []ai.Message    `json:"-"`
	Reply        ai.Message      `json:"-"`
	Documents    []documentEntry `json:"-"`
	FileRefs     []FileRefEntry  `json:"file_refs"`
	TraceFile    string          `json:"trace_file"`
	Timestamp    time.Time       `json:"timestamp"`
	AgentName    string          `json:"agent_name"`
	Hidden       bool            `json:"hidden"`
	Usage        ai.Usage        `json:"usage,omitempty"`
	systemTags   []tag
	userTags     []tag
}

func NewTurn(agentContext *AgentContext, userMessage, agentName, turnID string) *Turn {
	return &Turn{
		TurnID:       turnID,
		agentContext: agentContext,
		Request:      ai.UserMessage{Role: ai.UserRole, Content: userMessage},
		UserMessage:  userMessage,
		messages:     make([]ai.Message, 0),
		Documents:    make([]documentEntry, 0),
		FileRefs:     make([]FileRefEntry, 0),
		TraceFile:    "",
		Timestamp:    time.Now(),
		AgentName:    agentName,
		systemTags:   make([]tag, 0),
		userTags:     make([]tag, 0),
	}
}

func (t *Turn) AddMessage(msg ai.Message) {
	t.messages = append(t.messages, msg)
}

func (t *Turn) AddDocument(toolID string, doc *document.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	entry := documentEntry{
		Document: doc,
		ToolID:   toolID,
	}

	t.Documents = append(t.Documents, entry)

	return nil
}

func (t *Turn) Dir() string {
	return filepath.Join(t.agentContext.Workspace().TurnDir, t.TurnID)
}

func (t *Turn) saveToFile() {
	if t.agentContext == nil {
		return
	}
	dir := t.Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create turn directory", "dir", dir, "error", err)
		return
	}
	path := filepath.Join(dir, "turn.json")
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		slog.Error("failed to marshal turn", "turnId", t.TurnID, "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("failed to write turn file", "path", path, "error", err)
	}
}

func (t *Turn) loadFromFile(turnJSONPath string) error {
	data, err := os.ReadFile(turnJSONPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, t); err != nil {
		return err
	}
	turnDir := filepath.Dir(turnJSONPath)
	t.TraceFile = filepath.Join(turnDir, "trace.txt")
	return nil
}

func (t *Turn) DeleteDocument(doc *document.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}
	for i := range t.Documents {
		if t.Documents[i].Document.ID() == doc.ID() {
			t.Documents = append(t.Documents[:i], t.Documents[i+1:]...)
			return nil
		}
	}
	return nil
}

func (t *Turn) InjectSystemTag(tagName string, content string) {
	t.systemTags = append(t.systemTags, tag{
		Name:    tagName,
		Content: content,
	})
}

func (t *Turn) SystemTags() []TagEntry {
	result := make([]TagEntry, len(t.systemTags))
	for i, tag := range t.systemTags {
		result[i] = TagEntry{Name: tag.Name, Content: tag.Content}
	}
	return result
}

func (t *Turn) InjectUserTag(tagName string, content string) {
	t.userTags = append(t.userTags, tag{
		Name:    tagName,
		Content: content,
	})
}

func (t *Turn) GetDocuments() []*document.Document {
	docs := make([]*document.Document, len(t.Documents))
	for i, docEntry := range t.Documents {
		docs[i] = docEntry.Document
	}
	return docs
}

func (t *Turn) MarshalJSON() ([]byte, error) {
	type Alias Turn

	type messageJSON struct {
		Type          string            `json:"type"`
		UserMessage   *ai.UserMessage   `json:"user_message,omitempty"`
		AIMessage     *ai.AIMessage     `json:"ai_message,omitempty"`
		ToolMessage   *ai.ToolMessage   `json:"tool_message,omitempty"`
		SystemMessage *ai.SystemMessage `json:"system_message,omitempty"`
	}

	messageToJSON := func(msg ai.Message) *messageJSON {
		if msg == nil {
			return nil
		}
		mj := &messageJSON{}
		switch m := msg.(type) {
		case ai.UserMessage:
			mj.Type = "user_message"
			mj.UserMessage = &m
		case ai.AIMessage:
			mj.Type = "ai_message"
			mj.AIMessage = &m
		case ai.ToolMessage:
			mj.Type = "tool_message"
			mj.ToolMessage = &m
		case ai.SystemMessage:
			mj.Type = "system_message"
			mj.SystemMessage = &m
		default:
			return nil
		}
		return mj
	}

	type documentJSON struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
		FilePath string `json:"file_path"`
		MimeType string `json:"mime_type"`
		ToolID   string `json:"tool_id"`
	}

	var request *messageJSON
	if t.Request != nil {
		request = messageToJSON(t.Request)
	}

	var reply *messageJSON
	if t.Reply != nil {
		reply = messageToJSON(t.Reply)
	}

	messages := make([]*messageJSON, len(t.messages))
	for i, msg := range t.messages {
		messages[i] = messageToJSON(msg)
	}

	documents := make([]documentJSON, len(t.Documents))
	for i, docEntry := range t.Documents {
		if docEntry.Document != nil {
			documents[i] = documentJSON{
				ID:       docEntry.Document.ID(),
				Filename: docEntry.Document.Filename,
				FilePath: docEntry.Document.FilePath,
				MimeType: docEntry.Document.MimeType,
				ToolID:   docEntry.ToolID,
			}
		} else {
			documents[i] = documentJSON{
				ToolID: docEntry.ToolID,
			}
		}
	}

	return json.Marshal(&struct {
		*Alias
		Request    *messageJSON   `json:"request"`
		Messages   []*messageJSON `json:"messages"`
		Reply      *messageJSON   `json:"reply,omitempty"`
		Documents  []documentJSON `json:"documents"`
		SystemTags []tag          `json:"system_tags"`
		UserTags   []tag          `json:"user_tags"`
	}{
		Alias:      (*Alias)(t),
		Request:    request,
		Messages:   messages,
		Reply:      reply,
		Documents:  documents,
		SystemTags: t.systemTags,
		UserTags:   t.userTags,
	})
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	type Alias Turn

	type messageJSON struct {
		Type          string            `json:"type"`
		UserMessage   *ai.UserMessage   `json:"user_message,omitempty"`
		AIMessage     *ai.AIMessage     `json:"ai_message,omitempty"`
		ToolMessage   *ai.ToolMessage   `json:"tool_message,omitempty"`
		SystemMessage *ai.SystemMessage `json:"system_message,omitempty"`
	}

	jsonToMessage := func(mj *messageJSON) ai.Message {
		if mj == nil {
			return nil
		}
		switch mj.Type {
		case "user_message":
			if mj.UserMessage != nil {
				return *mj.UserMessage
			}
		case "ai_message":
			if mj.AIMessage != nil {
				return *mj.AIMessage
			}
		case "tool_message":
			if mj.ToolMessage != nil {
				return *mj.ToolMessage
			}
		case "system_message":
			if mj.SystemMessage != nil {
				return *mj.SystemMessage
			}
		}
		return nil
	}

	type documentJSON struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
		FilePath string `json:"file_path"`
		MimeType string `json:"mime_type"`
		ToolID   string `json:"tool_id"`
	}

	aux := &struct {
		*Alias
		Request    *messageJSON   `json:"request"`
		Messages   []*messageJSON `json:"messages"`
		Reply      *messageJSON   `json:"reply,omitempty"`
		Documents  []documentJSON `json:"documents"`
		SystemTags []tag          `json:"system_tags"`
		UserTags   []tag          `json:"user_tags"`
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if aux.Request != nil {
		t.Request = jsonToMessage(aux.Request)
	}

	if aux.Reply != nil {
		t.Reply = jsonToMessage(aux.Reply)
	}

	t.messages = make([]ai.Message, len(aux.Messages))
	for i, mj := range aux.Messages {
		t.messages[i] = jsonToMessage(mj)
	}

	t.Documents = make([]documentEntry, len(aux.Documents))
	for i, dj := range aux.Documents {
		t.Documents[i] = documentEntry{
			ToolID: dj.ToolID,
		}
		if dj.FilePath != "" {
			doc := &document.Document{
				Filename: dj.Filename,
				FilePath: dj.FilePath,
				MimeType: dj.MimeType,
			}
			t.Documents[i].Document = doc
		}
	}

	if aux.SystemTags != nil {
		t.systemTags = aux.SystemTags
	} else {
		t.systemTags = make([]tag, 0)
	}

	if aux.UserTags != nil {
		t.userTags = aux.UserTags
	} else {
		t.userTags = make([]tag, 0)
	}

	if aux.FileRefs != nil {
		t.FileRefs = aux.FileRefs
	} else {
		t.FileRefs = make([]FileRefEntry, 0)
	}

	return nil
}
