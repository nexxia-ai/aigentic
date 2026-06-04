package ctxt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicReaderNeverSeesPartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := writeAtomic(path, []byte("complete"), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" {
		t.Fatalf("got %q", data)
	}
}

func TestReadJSONWithVersionRejectsWrongVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	wrapped := struct {
		Version int            `json:"version"`
		Payload map[string]int `json:"payload"`
	}{Version: 99, Payload: map[string]int{"x": 1}}
	data, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	var out map[string]int
	if err := readJSONWithVersion(path, &out); err == nil {
		t.Fatal("expected version error")
	}
}

func TestReadJSONLLinesSkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.jsonl")
	if err := os.WriteFile(path, []byte("{\"turn_id\":\"a\"}\n\n{\"turn_id\":\"b\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var ids []string
	err := readJSONLLines(path, func(line []byte, _ int, _ bool) error {
		var row conversationLogLine
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		ids = append(ids, row.TurnID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("ids=%v", ids)
	}
}
