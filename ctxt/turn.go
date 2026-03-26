package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type TagEntry struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Turn struct {
	TurnID       string            `json:"turn_id"`
	RunID        string            `json:"run_id,omitempty"`
	agentContext *AgentContext     `json:"-"`
	ledgerDir    string            `json:"-"` // set by PrepareTurn for Dir()
	Request      ai.Message        `json:"-"`
	UserMessage  string            `json:"user_message"`
	UserData     string            `json:"user_data"`
	messages     []ai.Message      `json:"-"`
	Reply        ai.Message        `json:"-"`
	Files        []FileRef         `json:"files"`
	TraceFile    string            `json:"trace_file"`
	Timestamp    time.Time         `json:"timestamp"`
	AgentName    string            `json:"agent_name"`
	Hidden       bool              `json:"hidden"`
	Usage        ai.Usage          `json:"usage,omitempty"`
	meta         map[string]string `json:"-"`
	systemTags   []TagEntry
	turnTags     []ai.KeyValue
}

func NewTurn(agentContext *AgentContext, userMessage, userData, agentName, turnID string) *Turn {
	return &Turn{
		TurnID:       turnID,
		agentContext: agentContext,
		UserMessage:  userMessage,
		UserData:     userData,
		messages:     make([]ai.Message, 0),
		Files:        make([]FileRef, 0),
		TraceFile:    "",
		Timestamp:    time.Now(),
		AgentName:    agentName,
		systemTags:   make([]TagEntry, 0),
		turnTags:     make([]ai.KeyValue, 0),
	}
}

func (t *Turn) AddMessage(msg ai.Message) {
	t.messages = append(t.messages, msg)
}

func (t *Turn) AddFile(ref FileRef) {
	t.Files = append(t.Files, ref)
}

func (t *Turn) PromptFiles() []FileRef {
	var out []FileRef
	for _, f := range t.Files {
		if f.IncludeInPrompt {
			out = append(out, f)
		}
	}
	return out
}

func (t *Turn) FilesForTool(toolID string) []FileRef {
	var out []FileRef
	for _, f := range t.Files {
		if f.ToolID == toolID {
			out = append(out, f)
		}
	}
	return out
}

func (t *Turn) SetFileMeta(path string, meta map[string]string) error {
	for i := range t.Files {
		if t.Files[i].Path == path {
			t.Files[i].SetMeta(meta)
			return nil
		}
	}
	return fmt.Errorf("file not found: %s", path)
}

func (t *Turn) FileMeta(path string) map[string]string {
	for i := range t.Files {
		if t.Files[i].Path == path {
			return t.Files[i].Meta()
		}
	}
	return nil
}

func (t *Turn) Dir() string {
	return t.ledgerDir
}

func (t *Turn) SetLedgerDir(dir string) {
	t.ledgerDir = dir
}

func (t *Turn) loadFromFile(turnJSONPath string) error {
	data, err := os.ReadFile(turnJSONPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, t); err != nil {
		return err
	}
	turnDir := filepath.Dir(turnJSONPath)
	t.ledgerDir = turnDir
	t.TraceFile = filepath.Join(turnDir, "trace.txt")
	return t.loadMeta()
}

func (t *Turn) metaPath() string {
	if t.ledgerDir == "" {
		return ""
	}
	return filepath.Join(t.ledgerDir, "meta.json")
}

func (t *Turn) loadMeta() error {
	path := t.metaPath()
	if path == "" {
		t.meta = nil
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.meta = nil
			return nil
		}
		return err
	}
	var meta map[string]string
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	if len(meta) == 0 {
		t.meta = nil
		return nil
	}
	t.meta = meta
	return nil
}

func (t *Turn) saveMeta() error {
	path := t.metaPath()
	if path == "" {
		return nil
	}
	if len(t.meta) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(t.meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (t *Turn) InjectSystemTag(tagName string, content string) {
	t.systemTags = append(t.systemTags, TagEntry{Name: tagName, Content: content})
}

func (t *Turn) SystemTags() []TagEntry {
	if len(t.systemTags) == 0 {
		return nil
	}
	out := make([]TagEntry, len(t.systemTags))
	copy(out, t.systemTags)
	return out
}

func (t *Turn) InjectTurnTag(tagName string, content string) {
	t.turnTags = append(t.turnTags, ai.KeyValue{Key: tagName, Value: content})
}

func (t *Turn) AppendTurnTags(kv []ai.KeyValue) {
	t.turnTags = append(t.turnTags, kv...)
}

func (t *Turn) SetMeta(meta map[string]string) {
	if t.meta == nil {
		t.meta = make(map[string]string)
	}
	for k, v := range meta {
		if v == "" {
			delete(t.meta, k)
		} else {
			t.meta[k] = v
		}
	}
	if len(t.meta) == 0 {
		t.meta = nil
	}
	_ = t.saveMeta()
}

func (t *Turn) Meta() map[string]string {
	if len(t.meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(t.meta))
	for k, v := range t.meta {
		out[k] = v
	}
	return out
}

func (t *Turn) TurnTags() []ai.KeyValue {
	if t.turnTags == nil {
		return nil
	}
	out := make([]ai.KeyValue, len(t.turnTags))
	copy(out, t.turnTags)
	return out
}

func lastAssistantMessage(msgs []ai.Message) ai.Message {
	for i := len(msgs) - 1; i >= 0; i-- {
		if m, ok := msgs[i].(ai.AIMessage); ok && (m.Role == ai.AssistantRole || m.Role == "") {
			return m
		}
	}
	return nil
}

func (t *Turn) MarshalJSON() ([]byte, error) {
	type Alias Turn

	type messageJSON struct {
		Type          string            `json:"type"`
		UserMessage   *ai.UserMessage   `json:"user_message,omitempty"`
		AIMessage     *ai.AIMessage     `json:"ai_message,omitempty"`
		ToolMessage   *ai.ToolMessage   `json:"tool_message,omitempty"`
		SystemMessage *ai.SystemMessage `json:"system_message,omitempty"`
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
		default:
			return nil
		}
		return mj
	}

	var request *messageJSON
	if t.Request != nil {
		request = messageToJSON(t.Request)
	}

	messages := make([]*messageJSON, len(t.messages))
	for i, msg := range t.messages {
		messages[i] = messageToJSON(msg)
	}
	if len(messages) == 0 && t.Reply != nil {
		messages = []*messageJSON{messageToJSON(t.Reply)}
	}

	return json.Marshal(&struct {
		*Alias
		Request    *messageJSON `json:"request"`
		Messages   []*messageJSON `json:"messages"`
		Reply      *messageJSON `json:"-"`
		SystemTags []TagEntry   `json:"system_tags"`
		TurnTags   []ai.KeyValue `json:"turn_tags"`
	}{
		Alias:      (*Alias)(t),
		Request:    request,
		Messages:   messages,
		Reply:      nil,
		SystemTags: t.systemTags,
		TurnTags:   t.turnTags,
	})
}

type legacyDocumentJSON struct {
	FilePath string `json:"file_path"`
	MimeType string `json:"mime_type"`
	ToolID   string `json:"tool_id"`
}

type legacyFileRefEntry struct {
	Path            string `json:"path"`
	IncludeInPrompt bool   `json:"include_in_prompt"`
	MimeType        string `json:"mime_type,omitempty"`
	Ephemeral       bool   `json:"ephemeral"`
	UserUpload      bool   `json:"user_upload"`
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	type Alias Turn

	type messageJSON struct {
		Type          string            `json:"type"`
		UserMessage   *ai.UserMessage   `json:"user_message,omitempty"`
		AIMessage     *ai.AIMessage     `json:"ai_message,omitempty"`
		ToolMessage   *ai.ToolMessage   `json:"tool_message,omitempty"`
		SystemMessage *ai.SystemMessage `json:"system_message,omitempty"`
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
		}
		return nil
	}

	aux := &struct {
		*Alias
		Request    *messageJSON   `json:"request"`
		Messages   []*messageJSON `json:"messages"`
		Reply      *messageJSON   `json:"reply,omitempty"`
		SystemTags []TagEntry    `json:"system_tags"`
		TurnTags   []ai.KeyValue  `json:"turn_tags"`
		Files      []FileRef      `json:"files"`
		Documents  []legacyDocumentJSON `json:"documents"`
		FileRefs   []legacyFileRefEntry `json:"file_refs"`
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if aux.Request != nil {
		t.Request = jsonToMessage(aux.Request)
	}

	t.messages = make([]ai.Message, len(aux.Messages))
	for i, mj := range aux.Messages {
		t.messages[i] = jsonToMessage(mj)
	}
	t.Reply = lastAssistantMessage(t.messages)

	if aux.SystemTags != nil {
		t.systemTags = aux.SystemTags
	} else {
		t.systemTags = make([]TagEntry, 0)
	}

	if aux.TurnTags != nil {
		t.turnTags = aux.TurnTags
	} else {
		t.turnTags = make([]ai.KeyValue, 0)
	}

	if len(aux.Files) > 0 {
		t.Files = aux.Files
	} else {
		t.Files = make([]FileRef, 0)
		for _, d := range aux.Documents {
			if d.FilePath != "" {
				t.Files = append(t.Files, FileRef{
					Path:            d.FilePath,
					MimeType:        d.MimeType,
					ToolID:          d.ToolID,
					IncludeInPrompt: false,
				})
			}
		}
		for _, ref := range aux.FileRefs {
			f := FileRef{
				Path:            ref.Path,
				MimeType:        ref.MimeType,
				IncludeInPrompt: ref.IncludeInPrompt,
				Ephemeral:       ref.Ephemeral,
			}
			if ref.UserUpload {
				f.SetMeta(map[string]string{"visible_to_user": "true"})
			}
			t.Files = append(t.Files, f)
		}
	}

	return nil
}
