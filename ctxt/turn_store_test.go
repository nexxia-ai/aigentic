package ctxt

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestTurnStoreRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "20250604-abc12345")
	turn := &Turn{
		TurnID:      "20250604-abc12345",
		UserMessage: "hi",
		Request:     ai.UserMessage{Role: ai.UserRole, Content: "hi"},
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "hello"},
		Timestamp:   time.Now().UTC(),
		AgentName:   "agent",
	}
	turn.AddMessage(ai.ToolMessage{Role: ai.ToolRole, Content: "tool out"})
	turn.AddMessage(turn.Reply)

	if err := saveTurn(dir, turn); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, TurnHeadFileName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, TurnMessagesFileName)); err != nil {
		t.Fatal(err)
	}

	got, err := loadTurn(dir, turn.TurnID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.messages) < 2 {
		t.Fatalf("messages=%d", len(got.messages))
	}
}

func TestLedgerHeadDoesNotReadMessagesFile(t *testing.T) {
	base := t.TempDir()
	ledger := NewLedger(base)
	turnID, dir, err := ledger.PrepareTurn(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	turn := &Turn{
		TurnID:      turnID,
		UserMessage: "u",
		Request:     ai.UserMessage{Role: ai.UserRole, Content: "u"},
		Reply:       ai.AIMessage{Role: ai.AssistantRole, Content: "big"},
		Timestamp:   time.Now().UTC(),
	}
	turn.AddMessage(ai.AIMessage{Role: ai.AssistantRole, Content: "x"})
	if err := saveTurn(dir, turn); err != nil {
		t.Fatal(err)
	}
	msgPath := filepath.Join(dir, TurnMessagesFileName)
	if err := os.WriteFile(msgPath, []byte("{invalid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Get(turnID); err == nil {
		t.Fatal("expected Get to fail on corrupt messages file")
	}
	head, err := ledger.Head(turnID)
	if err != nil {
		t.Fatal(err)
	}
	if head.UserMessage != "u" {
		t.Fatalf("head user=%q", head.UserMessage)
	}
}

func TestLoadTurnFailsOnCorruptMessagesLine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "20250604-deadbeef")
	turn := testTurnFixture("20250604-deadbeef", "a", nil)
	if err := saveTurn(dir, turn); err != nil {
		t.Fatal(err)
	}
	msgPath := filepath.Join(dir, TurnMessagesFileName)
	if err := os.WriteFile(msgPath, []byte("{bad\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTurn(dir, turn.TurnID); err == nil {
		t.Fatal("expected error")
	}
}
