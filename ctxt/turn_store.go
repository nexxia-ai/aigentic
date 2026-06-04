package ctxt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type turnHeadData struct {
	TurnID             string            `json:"turn_id"`
	RunID              string            `json:"run_id,omitempty"`
	UserMessage        string            `json:"user_message"`
	UserData           string            `json:"user_data"`
	Files              []FileRef         `json:"files"`
	TraceFile          string            `json:"trace_file"`
	Timestamp          time.Time         `json:"timestamp"`
	AgentName          string            `json:"agent_name"`
	Hidden             bool              `json:"hidden"`
	Usage              ai.Usage          `json:"usage,omitempty"`
	StartFileCutoff    time.Time         `json:"start_file_cutoff,omitempty"`
	InjectionBytesUsed int               `json:"injection_bytes_used,omitempty"`
	Request            *messageJSON      `json:"request,omitempty"`
	RequestSnapshot    *messageJSON      `json:"request_snapshot,omitempty"`
	Reply              *messageJSON      `json:"reply,omitempty"`
	SystemTags         []TagEntry        `json:"system_tags"`
	TurnTags           []ai.KeyValue     `json:"turn_tags"`
}

type shardIndexLine struct {
	Version     int       `json:"version"`
	TurnID      string    `json:"turn_id"`
	RunID       string    `json:"run_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	AgentName   string    `json:"agent_name"`
	HasFeedback bool      `json:"has_feedback"`
}

func turnHeadFromTurn(t *Turn) turnHeadData {
	h := turnHeadData{
		TurnID:             t.TurnID,
		RunID:              t.RunID,
		UserMessage:        t.UserMessage,
		UserData:           t.UserData,
		Files:              t.Files,
		TraceFile:          t.TraceFile,
		Timestamp:          t.Timestamp,
		AgentName:          t.AgentName,
		Hidden:             t.Hidden,
		Usage:              t.Usage,
		StartFileCutoff:    t.StartFileCutoff,
		InjectionBytesUsed: t.InjectionBytesUsed,
		SystemTags:         t.systemTags,
		TurnTags:           t.turnTags,
	}
	if t.Request != nil {
		h.Request = messageToJSON(t.Request)
	}
	if t.RequestSnapshot != nil {
		h.RequestSnapshot = messageToJSON(t.RequestSnapshot)
	}
	if t.Reply != nil {
		h.Reply = messageToJSON(t.Reply)
	}
	return h
}

func applyHeadToTurn(t *Turn, h turnHeadData) {
	t.TurnID = h.TurnID
	t.RunID = h.RunID
	t.UserMessage = h.UserMessage
	t.UserData = h.UserData
	t.Files = h.Files
	t.TraceFile = h.TraceFile
	t.Timestamp = h.Timestamp
	t.AgentName = h.AgentName
	t.Hidden = h.Hidden
	t.Usage = h.Usage
	t.StartFileCutoff = h.StartFileCutoff
	t.InjectionBytesUsed = h.InjectionBytesUsed
	t.Request = jsonToMessage(h.Request)
	t.RequestSnapshot = jsonToMessage(h.RequestSnapshot)
	t.Reply = jsonToMessage(h.Reply)
	if h.SystemTags != nil {
		t.systemTags = h.SystemTags
	} else {
		t.systemTags = make([]TagEntry, 0)
	}
	if h.TurnTags != nil {
		t.turnTags = h.TurnTags
	} else {
		t.turnTags = make([]ai.KeyValue, 0)
	}
}

func turnMessagesForSave(t *Turn) []ai.Message {
	msgs := t.messages
	if len(msgs) == 0 && t.Reply != nil {
		msgs = []ai.Message{t.Reply}
	}
	return msgs
}

func saveTurn(dirPath string, t *Turn) error {
	if t.TurnID == "" {
		return fmt.Errorf("turn has no turnID")
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}
	t.SetLedgerDir(dirPath)
	if t.TraceFile == "" {
		t.TraceFile = filepath.Join(dirPath, "trace.txt")
	}

	var msgBuf bytes.Buffer
	for _, msg := range turnMessagesForSave(t) {
		line, err := marshalMessageLine(msg)
		if err != nil {
			return fmt.Errorf("marshal message: %w", err)
		}
		msgBuf.Write(line)
		msgBuf.WriteByte('\n')
	}
	msgPath := filepath.Join(dirPath, TurnMessagesFileName)
	if err := writeAtomic(msgPath, msgBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write messages: %w", err)
	}

	head := turnHeadFromTurn(t)
	headPath := filepath.Join(dirPath, TurnHeadFileName)
	if err := writeAtomicJSON(headPath, struct {
		Version int         `json:"version"`
		Payload turnHeadData `json:"payload"`
	}{Version: storageVersion, Payload: head}); err != nil {
		return fmt.Errorf("write head: %w", err)
	}
	if err := t.saveMeta(); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return nil
}

func loadTurnHead(dirPath, turnID string) (*Turn, error) {
	headPath := filepath.Join(dirPath, TurnHeadFileName)
	var head turnHeadData
	if err := readJSONWithVersion(headPath, &head); err != nil {
		return nil, err
	}
	t := &Turn{messages: make([]ai.Message, 0)}
	applyHeadToTurn(t, head)
	t.TurnID = turnID
	t.SetLedgerDir(dirPath)
	t.TraceFile = filepath.Join(dirPath, "trace.txt")
	if err := t.loadMeta(); err != nil {
		return nil, err
	}
	return t, nil
}

func loadTurnMessages(dirPath string, t *Turn) error {
	msgPath := filepath.Join(dirPath, TurnMessagesFileName)
	var msgs []ai.Message
	err := readJSONLLines(msgPath, func(line []byte, lineNum int, isLast bool) error {
		msg, err := unmarshalMessageLine(line)
		if err != nil {
			return fmt.Errorf("messages.jsonl line %d: %w", lineNum, err)
		}
		if msg != nil {
			msgs = append(msgs, msg)
		}
		return nil
	})
	if err != nil {
		return err
	}
	t.messages = msgs
	if t.Reply == nil {
		t.Reply = lastAssistantMessage(msgs)
	}
	return nil
}

func loadTurn(dirPath, turnID string) (*Turn, error) {
	t, err := loadTurnHead(dirPath, turnID)
	if err != nil {
		return nil, err
	}
	msgPath := filepath.Join(dirPath, TurnMessagesFileName)
	if _, err := os.Stat(msgPath); os.IsNotExist(err) {
		return t, nil
	}
	if err := loadTurnMessages(dirPath, t); err != nil {
		return nil, err
	}
	return t, nil
}

func turnHeadExists(dirPath string) bool {
	_, err := os.Stat(filepath.Join(dirPath, TurnHeadFileName))
	return err == nil
}

func hasFeedbackMeta(meta map[string]string) bool {
	if meta == nil {
		return false
	}
	if meta["feedback"] != "" || meta["feedback_comment"] != "" {
		return true
	}
	return false
}

func appendShardIndex(shardDir string, t *Turn) error {
	meta := t.Meta()
	line := shardIndexLine{
		Version:     storageVersion,
		TurnID:      t.TurnID,
		RunID:       t.RunID,
		Timestamp:   t.Timestamp,
		AgentName:   t.AgentName,
		HasFeedback: hasFeedbackMeta(meta),
	}
	return appendJSONL(filepath.Join(shardDir, ShardIndexFileName), line)
}
