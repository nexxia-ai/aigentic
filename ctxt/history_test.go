package ctxt

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestLedgerGetResolvesTurns(t *testing.T) {
	basePath := t.TempDir()
	ledger := NewLedger(basePath)

	turn1 := Turn{
		TurnID:      "20260312-abc12345",
		UserMessage: "Hello",
		Request:     ai.UserMessage{Role: ai.UserRole, Content: "Hello"},
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "Hi there"},
		Timestamp:   time.Now(),
		AgentName:   "test",
	}
	if err := ledger.Append(&turn1); err != nil {
		t.Fatalf("failed to append turn: %v", err)
	}

	got, err := ledger.Get("20260312-abc12345")
	if err != nil {
		t.Fatalf("ledger.Get: %v", err)
	}
	if got.UserMessage != "Hello" {
		t.Errorf("expected UserMessage 'Hello', got %q", got.UserMessage)
	}
}

func TestGetMessagesSkipsNilRequest(t *testing.T) {
	tmp := t.TempDir()
	ledger := NewLedger(tmp)
	convPath := filepath.Join(tmp, "conversation.json")
	h := NewConversationHistory(ledger, convPath)
	turnID, _, err := ledger.PrepareTurn(time.Now())
	if err != nil {
		t.Fatalf("PrepareTurn: %v", err)
	}
	turn := Turn{
		TurnID:      turnID,
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

func TestGetMessagesPreservesFullHistory(t *testing.T) {
	tmp := t.TempDir()
	ledger := NewLedger(tmp)
	h := NewConversationHistory(ledger, filepath.Join(tmp, "conversation.json"))

	for i := 0; i < 120; i++ {
		turnID, _, err := ledger.PrepareTurn(time.Now())
		if err != nil {
			t.Fatalf("PrepareTurn %d: %v", i, err)
		}
		h.appendTurn(Turn{
			TurnID:      turnID,
			UserMessage: "question",
			Request:     ai.UserMessage{Role: ai.UserRole, Content: "question"},
			Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "answer"},
			Timestamp:   time.Now(),
			AgentName:   "test",
		})
	}

	if got := h.Len(); got != 120 {
		t.Fatalf("expected 120 stored turn refs, got %d", got)
	}
	if got := len(h.GetTurns()); got != 120 {
		t.Fatalf("expected 120 resolved turns, got %d", got)
	}
	if got := len(h.GetMessages()); got != 240 {
		t.Fatalf("expected 240 history messages, got %d", got)
	}
}
