package ctxt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestLoadTurnsFromDir(t *testing.T) {
	dir := t.TempDir()

	turn1 := Turn{
		TurnID:      "turn-001",
		UserMessage: "Hello",
		Request:     ai.UserMessage{Role: ai.UserRole, Content: "Hello"},
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "Hi there"},
		Timestamp:   time.Now(),
		AgentName:   "test",
	}
	turn1Dir := filepath.Join(dir, "turn-001")
	if err := os.MkdirAll(turn1Dir, 0755); err != nil {
		t.Fatalf("failed to create turn dir: %v", err)
	}
	data1, _ := json.MarshalIndent(turn1, "", "  ")
	if err := os.WriteFile(filepath.Join(turn1Dir, "turn.json"), data1, 0644); err != nil {
		t.Fatalf("failed to write turn.json: %v", err)
	}

	turn2 := Turn{
		TurnID:      "turn-002",
		UserMessage: "How are you?",
		Request:     ai.UserMessage{Role: ai.UserRole, Content: "How are you?"},
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "I'm fine"},
		Timestamp:   time.Now(),
		AgentName:   "test",
	}
	turn2Dir := filepath.Join(dir, "turn-002")
	if err := os.MkdirAll(turn2Dir, 0755); err != nil {
		t.Fatalf("failed to create turn dir: %v", err)
	}
	data2, _ := json.MarshalIndent(turn2, "", "  ")
	if err := os.WriteFile(filepath.Join(turn2Dir, "turn.json"), data2, 0644); err != nil {
		t.Fatalf("failed to write turn.json: %v", err)
	}

	turns := loadTurnsFromDir(dir)
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].UserMessage != "Hello" {
		t.Errorf("expected first turn UserMessage 'Hello', got %q", turns[0].UserMessage)
	}
	if turns[1].UserMessage != "How are you?" {
		t.Errorf("expected second turn UserMessage 'How are you?', got %q", turns[1].UserMessage)
	}
	expectedTrace1 := filepath.Join(dir, "turn-001", "trace.txt")
	if turns[0].TraceFile != expectedTrace1 {
		t.Errorf("expected TraceFile rehydrated to %q, got %q", expectedTrace1, turns[0].TraceFile)
	}
	expectedTrace2 := filepath.Join(dir, "turn-002", "trace.txt")
	if turns[1].TraceFile != expectedTrace2 {
		t.Errorf("expected TraceFile rehydrated to %q, got %q", expectedTrace2, turns[1].TraceFile)
	}
}

func TestGetMessagesSkipsNilRequest(t *testing.T) {
	h := NewConversationHistory(nil)
	turn := Turn{
		TurnID:      "turn-001",
		UserMessage: "fallback message",
		Request:     nil,
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "reply"},
		Timestamp:   time.Now(),
		AgentName:   "test",
	}
	h.appendTurn(turn)

	msgs := h.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(msgs))
	}
	for _, m := range msgs {
		if m == nil {
			t.Error("GetMessages returned nil message")
		}
	}
	userMsg, ok := msgs[0].(ai.UserMessage)
	if !ok {
		t.Errorf("expected first message to be UserMessage, got %T", msgs[0])
	} else if userMsg.Content != "fallback message" {
		t.Errorf("expected synthesized UserMessage content 'fallback message', got %q", userMsg.Content)
	}
}
