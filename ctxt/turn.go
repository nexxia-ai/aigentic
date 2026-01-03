package ctxt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type DocumentEntry struct {
	Document *document.Document `json:"-"`
	ToolID   string             `json:"tool_id"`
}

type ConversationTurn struct {
	Request     ai.Message      `json:"-"`
	UserMessage string          `json:"user_message"`
	messages    []ai.Message    `json:"-"`
	Reply       ai.Message      `json:"-"`
	Documents   []DocumentEntry `json:"-"`
	TraceFile   string          `json:"trace_file"`
	RunID       string          `json:"run_id"`
	Timestamp   time.Time       `json:"timestamp"`
	AgentName   string          `json:"agent_name"`
	Hidden      bool            `json:"hidden"`
}

func NewConversationTurn(userMessage, runID, agentName, traceFile string) *ConversationTurn {
	return &ConversationTurn{
		Request:     ai.UserMessage{Role: ai.UserRole, Content: userMessage},
		UserMessage: userMessage,
		messages:    make([]ai.Message, 0),
		Documents:   make([]DocumentEntry, 0),
		TraceFile:   traceFile,
		RunID:       runID,
		Timestamp:   time.Now(),
		AgentName:   agentName,
	}
}

func (t *ConversationTurn) AddMessage(msg ai.Message) {
	t.messages = append(t.messages, msg)
}

func (t *ConversationTurn) AddDocument(toolID string, doc *document.Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	entry := DocumentEntry{
		Document: doc,
		ToolID:   toolID,
	}

	t.Documents = append(t.Documents, entry)

	return nil
}

func (t *ConversationTurn) DeleteDocument(doc *document.Document) error {
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

func (t *ConversationTurn) MarshalJSON() ([]byte, error) {
	type Alias ConversationTurn

	type messageJSON struct {
		Type            string              `json:"type"`
		UserMessage     *ai.UserMessage     `json:"user_message,omitempty"`
		AIMessage       *ai.AIMessage       `json:"ai_message,omitempty"`
		ToolMessage     *ai.ToolMessage     `json:"tool_message,omitempty"`
		SystemMessage   *ai.SystemMessage   `json:"system_message,omitempty"`
		ResourceMessage *ai.ResourceMessage `json:"resource_message,omitempty"`
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
		case ai.ResourceMessage:
			mj.Type = "resource_message"
			mj.ResourceMessage = &m
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
		Request   *messageJSON   `json:"request"`
		Messages  []*messageJSON `json:"messages"`
		Reply     *messageJSON   `json:"reply,omitempty"`
		Documents []documentJSON `json:"documents"`
	}{
		Alias:     (*Alias)(t),
		Request:   request,
		Messages:  messages,
		Reply:     reply,
		Documents: documents,
	})
}

func (t *ConversationTurn) UnmarshalJSON(data []byte) error {
	type Alias ConversationTurn

	type messageJSON struct {
		Type            string              `json:"type"`
		UserMessage     *ai.UserMessage     `json:"user_message,omitempty"`
		AIMessage       *ai.AIMessage       `json:"ai_message,omitempty"`
		ToolMessage     *ai.ToolMessage     `json:"tool_message,omitempty"`
		SystemMessage   *ai.SystemMessage   `json:"system_message,omitempty"`
		ResourceMessage *ai.ResourceMessage `json:"resource_message,omitempty"`
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
		case "resource_message":
			if mj.ResourceMessage != nil {
				return *mj.ResourceMessage
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
		Request   *messageJSON   `json:"request"`
		Messages  []*messageJSON `json:"messages"`
		Reply     *messageJSON   `json:"reply,omitempty"`
		Documents []documentJSON `json:"documents"`
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

	t.Documents = make([]DocumentEntry, len(aux.Documents))
	for i, dj := range aux.Documents {
		t.Documents[i] = DocumentEntry{
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

	return nil
}
