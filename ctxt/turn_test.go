package ctxt

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTurnAddFile(t *testing.T) {
	ac, err := New("test", "test", "test", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test context: %v", err)
	}
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	turn.AddFile(FileRef{
		Path:            "uploads/test.pdf",
		MimeType:        "application/pdf",
		ToolID:          "tool1",
		IncludeInPrompt: true,
	})

	assert.Len(t, turn.Files, 1)
	assert.Equal(t, "uploads/test.pdf", turn.Files[0].Path)
	assert.Equal(t, "tool1", turn.Files[0].ToolID)
}

func TestTurnAddMultipleFiles(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	turn.AddFile(FileRef{Path: "uploads/a.pdf", ToolID: "tool1", IncludeInPrompt: true})
	turn.AddFile(FileRef{Path: "uploads/b.txt", ToolID: "tool2", IncludeInPrompt: false})
	turn.AddFile(FileRef{Path: "output/c.png", ToolID: "tool1", IncludeInPrompt: true})

	assert.Len(t, turn.Files, 3)
	assert.Len(t, turn.PromptFiles(), 2)
	assert.Len(t, turn.FilesForTool("tool1"), 2)
	assert.Len(t, turn.FilesForTool("tool2"), 1)
}

func TestTurnPromptFiles(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	turn.AddFile(FileRef{Path: "a", IncludeInPrompt: true})
	turn.AddFile(FileRef{Path: "b", IncludeInPrompt: false})
	turn.AddFile(FileRef{Path: "c", IncludeInPrompt: true})

	prompt := turn.PromptFiles()
	assert.Len(t, prompt, 2)
	assert.Equal(t, "a", prompt[0].Path)
	assert.Equal(t, "c", prompt[1].Path)
}

func TestTurnFilesForTool(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	turn.AddFile(FileRef{Path: "a", ToolID: "t1"})
	turn.AddFile(FileRef{Path: "b", ToolID: "t2"})
	turn.AddFile(FileRef{Path: "c", ToolID: "t1"})

	assert.Len(t, turn.FilesForTool("t1"), 2)
	assert.Len(t, turn.FilesForTool("t2"), 1)
	assert.Len(t, turn.FilesForTool("t3"), 0)
}

func TestTurnSetFileMeta(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	turn.AddFile(FileRef{Path: "uploads/doc.pdf"})

	err := turn.SetFileMeta("uploads/doc.pdf", map[string]string{"visible_to_user": "true"})
	assert.NoError(t, err)
	assert.Equal(t, "true", turn.FileMeta("uploads/doc.pdf")["visible_to_user"])

	err = turn.SetFileMeta("missing", map[string]string{"k": "v"})
	assert.Error(t, err)
}

func TestTurnFileMetaMergeAndDelete(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "", "agent1", "turn-001")

	f := FileRef{Path: "x"}
	f.SetMeta(map[string]string{"a": "1", "b": "2"})
	turn.AddFile(f)

	assert.Equal(t, "1", turn.Files[0].GetMeta("a"))
	f2 := turn.Files[0]
	f2.SetMeta(map[string]string{"b": "", "c": "3"})
	turn.Files[0] = f2

	meta := turn.Files[0].Meta()
	assert.Equal(t, "1", meta["a"])
	assert.Equal(t, "3", meta["c"])
	_, hasB := meta["b"]
	assert.False(t, hasB)
}

func TestLedgerAppendPersistsTurnMetaSidecar(t *testing.T) {
	basePath := t.TempDir()
	ledger := NewLedger(basePath)

	turnID, dirPath, err := ledger.PrepareTurn(time.Now())
	assert.NoError(t, err)

	turn := NewTurn(nil, "test message", "", "agent1", turnID)
	turn.Timestamp = time.Now()
	turn.SetLedgerDir(dirPath)
	turn.SetMeta(map[string]string{"source": "caller"})

	assert.NoError(t, ledger.Append(turn))

	metaData, err := os.ReadFile(filepath.Join(dirPath, "meta.json"))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"source":"caller"}`, string(metaData))

	loaded, err := ledger.Get(turnID)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"source": "caller"}, loaded.Meta())
}

func TestSetMetaPersistsToSidecarWhenLedgerDirKnown(t *testing.T) {
	basePath := t.TempDir()
	ledger := NewLedger(basePath)

	turnID, dirPath, err := ledger.PrepareTurn(time.Now())
	assert.NoError(t, err)

	turn := NewTurn(nil, "test message", "", "agent1", turnID)
	turn.Timestamp = time.Now()
	turn.SetLedgerDir(dirPath)

	turn.SetMeta(map[string]string{"status": "updated"})

	metaData, err := os.ReadFile(filepath.Join(dirPath, "meta.json"))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"status":"updated"}`, string(metaData))

	assert.NoError(t, ledger.Append(turn))

	loaded, err := ledger.Get(turnID)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"status": "updated"}, loaded.Meta())

	turn.SetMeta(map[string]string{"status": ""})

	loaded, err = ledger.Get(turnID)
	assert.NoError(t, err)
	assert.Nil(t, loaded.Meta())
	_, err = os.Stat(filepath.Join(dirPath, "meta.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestTurnMarshalUnmarshalFiles(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "msg", "", "agent", "turn-1")
	f := FileRef{Path: "uploads/x.pdf", MimeType: "application/pdf", IncludeInPrompt: true}
	f.SetMeta(map[string]string{"visible_to_user": "true"})
	turn.AddFile(f)

	data, err := turn.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"files"`)

	var loaded Turn
	err = loaded.UnmarshalJSON(data)
	assert.NoError(t, err)
	assert.Len(t, loaded.Files, 1)
	assert.Equal(t, "uploads/x.pdf", loaded.Files[0].Path)
	assert.Equal(t, "true", loaded.Files[0].GetMeta("visible_to_user"))
}

func TestTurnLedgerPersistsFilesWithMetadata(t *testing.T) {
	basePath := t.TempDir()
	ledger := NewLedger(basePath)

	turnID, _, err := ledger.PrepareTurn(time.Now())
	assert.NoError(t, err)

	turn := NewTurn(nil, "msg", "", "agent", turnID)
	turn.Timestamp = time.Now()
	f := FileRef{Path: "uploads/invoice.pdf", MimeType: "application/pdf", IncludeInPrompt: true}
	f.SetMeta(map[string]string{"visible_to_user": "true", "source": "user"})
	turn.AddFile(f)
	f2 := FileRef{Path: "output/invoice.md", MimeType: "text/markdown", IncludeInPrompt: true}
	f2.SetMeta(map[string]string{"visible_to_user": "false", "derived_from": "uploads/invoice.pdf"})
	turn.AddFile(f2)

	assert.NoError(t, ledger.Append(turn))

	loaded, err := ledger.Get(turnID)
	assert.NoError(t, err)
	assert.Len(t, loaded.Files, 2)
	assert.Equal(t, "uploads/invoice.pdf", loaded.Files[0].Path)
	assert.Equal(t, "true", loaded.Files[0].GetMeta("visible_to_user"))
	assert.Equal(t, "user", loaded.Files[0].GetMeta("source"))
	assert.Equal(t, "output/invoice.md", loaded.Files[1].Path)
	assert.Equal(t, "false", loaded.Files[1].GetMeta("visible_to_user"))
	assert.Equal(t, "uploads/invoice.pdf", loaded.Files[1].GetMeta("derived_from"))
}

func TestTurnLegacyUnmarshalMigratesFileRefs(t *testing.T) {
	// Old format: file_refs with UserUpload -> visible_to_user metadata
	data := []byte(`{"turn_id":"t1","run_id":"r1","user_message":"hi","files":[],"file_refs":[{"path":"uploads/a.pdf","include_in_prompt":true,"user_upload":true},{"path":"output/b.md","include_in_prompt":true,"user_upload":false}],"documents":[],"trace_file":"","agent_name":"agent","hidden":false,"timestamp":"2025-01-01T00:00:00Z"}`)
	var loaded Turn
	err := loaded.UnmarshalJSON(data)
	assert.NoError(t, err)
	assert.Len(t, loaded.Files, 2)
	assert.Equal(t, "uploads/a.pdf", loaded.Files[0].Path)
	assert.Equal(t, "true", loaded.Files[0].GetMeta("visible_to_user"))
	assert.Equal(t, "output/b.md", loaded.Files[1].Path)
	assert.Equal(t, "", loaded.Files[1].GetMeta("visible_to_user"))
}

func TestTurnLegacyUnmarshalMigratesDocuments(t *testing.T) {
	// Old format: documents -> Files
	data := []byte(`{"turn_id":"t1","run_id":"r1","user_message":"hi","files":[],"file_refs":[],"documents":[{"file_path":"uploads/x.pdf","mime_type":"application/pdf","tool_id":"t1"}],"trace_file":"","agent_name":"agent","hidden":false,"timestamp":"2025-01-01T00:00:00Z"}`)
	var loaded Turn
	err := loaded.UnmarshalJSON(data)
	assert.NoError(t, err)
	assert.Len(t, loaded.Files, 1)
	assert.Equal(t, "uploads/x.pdf", loaded.Files[0].Path)
	assert.Equal(t, "application/pdf", loaded.Files[0].MimeType)
	assert.Equal(t, "t1", loaded.Files[0].ToolID)
	assert.True(t, loaded.Files[0].IncludeInPrompt)
}
