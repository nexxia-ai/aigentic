package ctxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConversationStoreAppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ConversationLogName)
	if err := appendConversationRef(logPath, "t1"); err != nil {
		t.Fatal(err)
	}
	if err := appendConversationRef(logPath, "t2"); err != nil {
		t.Fatal(err)
	}
	refs, err := LoadConversationRefs(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0] != "t1" || refs[1] != "t2" {
		t.Fatalf("refs=%v", refs)
	}
}

func TestConversationStoreDedupesIDs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ConversationLogName)
	_ = appendConversationRef(logPath, "t1")
	_ = appendConversationRef(logPath, "t1")
	refs, err := LoadConversationRefs(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("refs=%v", refs)
	}
}

func TestConversationStoreClear(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ConversationLogName)
	_ = appendConversationRef(logPath, "t1")
	if err := clearConversation(logPath); err != nil {
		t.Fatal(err)
	}
	refs, err := LoadConversationRefs(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("refs=%v", refs)
	}
}

func TestConversationStoreToleratesBadFinalLine(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ConversationLogName)
	content := "{\"turn_id\":\"ok\"}\n{bad\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	refs, err := LoadConversationRefs(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != "ok" {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
}
