package aigentic

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

// DocumentEntry represents a document with its scope and tool tracking information
type DocumentEntry struct {
	Document *document.Document
	Scope    string
	ToolID   string
}

// ConversationTurn represents a complete conversation turn
// During run: stores all intermediate messages for tool execution
// After completion: compacted to just Request, Reply, Documents
type ConversationTurn struct {
	Request   ai.Message
	messages  []ai.Message
	Reply     ai.Message
	Documents []DocumentEntry
	TraceFile string
	RunID     string
	Timestamp time.Time
	AgentName string
	Hidden    bool
}

// addMessage adds a message to the internal messages slice during run
func (t *ConversationTurn) addMessage(msg ai.Message) {
	t.messages = append(t.messages, msg)
}

// getCurrentMessages returns the intermediate messages during run (excludes Request and Reply)
func (t *ConversationTurn) getCurrentMessages() []ai.Message {
	result := make([]ai.Message, len(t.messages))
	copy(result, t.messages)
	return result
}

// GetMessages returns all messages
func (t *ConversationTurn) GetMessages() []ai.Message {
	result := make([]ai.Message, 0, len(t.messages)+2)
	result = append(result, t.Request)
	result = append(result, t.messages...)
	if t.Reply != nil {
		result = append(result, t.Reply)
	}
	return result
}

// compact drops intermediate messages, keeping only Request, Reply, Documents
func (t *ConversationTurn) compact() {
	t.messages = nil
}

// NewConversationTurn creates a new ConversationTurn with the provided parameters
func NewConversationTurn(userMessage, runID, agentName, traceFile string) *ConversationTurn {
	return &ConversationTurn{
		Request:   ai.UserMessage{Role: ai.UserRole, Content: userMessage},
		messages:  make([]ai.Message, 0),
		Documents: make([]DocumentEntry, 0),
		TraceFile: traceFile,
		RunID:     runID,
		Timestamp: time.Now(),
		AgentName: agentName,
	}
}
