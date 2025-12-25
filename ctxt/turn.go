package ctxt

import (
	"fmt"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

type DocumentEntry struct {
	Document *document.Document
	ToolID   string
}

type ConversationTurn struct {
	Request     ai.Message
	UserMessage string
	messages    []ai.Message
	Reply       ai.Message
	Documents   []DocumentEntry
	TraceFile   string
	RunID       string
	Timestamp   time.Time
	AgentName   string
	Hidden      bool
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

func (t *ConversationTurn) GetCurrentMessages() []ai.Message {
	result := make([]ai.Message, len(t.messages))
	copy(result, t.messages)
	return result
}

func (t *ConversationTurn) GetMessages() []ai.Message {
	result := make([]ai.Message, 0, len(t.messages)+2)
	result = append(result, t.Request)
	result = append(result, t.messages...)
	if t.Reply != nil {
		result = append(result, t.Reply)
	}
	return result
}

func (t *ConversationTurn) Compact() {
	t.messages = nil
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
