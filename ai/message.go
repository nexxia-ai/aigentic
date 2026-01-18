package ai

import "encoding/json"

type MessageRole string

const (
	UserRole      MessageRole = "user"
	AssistantRole MessageRole = "assistant"
	ToolRole      MessageRole = "tool"
	SystemRole    MessageRole = "system"
)

type Message interface {
	Value() (role MessageRole, content string)
}

var (
	_ Message = UserMessage{}
	_ Message = AIMessage{}
	_ Message = ToolMessage{}
	_ Message = SystemMessage{}
)

type ContentPartType string

const (
	ContentPartText      ContentPartType = "text"
	ContentPartImage     ContentPartType = "image"
	ContentPartImageURL  ContentPartType = "image_url"
	ContentPartAudio     ContentPartType = "audio"
	ContentPartVideo     ContentPartType = "video"
	ContentPartFile      ContentPartType = "file"
	ContentPartInputFile ContentPartType = "input_file"
)

type ContentPart struct {
	Type     ContentPartType `json:"type"`
	Text     string          `json:"text,omitempty"`
	MimeType string          `json:"mime_type,omitempty"`
	Data     []byte          `json:"data,omitempty"`
	URI      string          `json:"uri,omitempty"`
	FileID   string          `json:"file_id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Detail   string          `json:"detail,omitempty"`
}

type ToolCall struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result any    `json:"result,omitempty"`
}

type AIMessage struct {
	Role      MessageRole    `json:"role"`
	Content   string         `json:"content,omitempty"`
	Parts     []ContentPart  `json:"parts,omitempty"`
	Think     string         `json:"think,omitempty"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
	Response  Response       `json:"response,omitempty"`
}

func (m AIMessage) Value() (MessageRole, string) {
	if m.Content != "" {
		return m.Role, m.Content
	}
	if len(m.Parts) > 0 {
		for _, part := range m.Parts {
			if part.Type == ContentPartText && part.Text != "" {
				return m.Role, part.Text
			}
		}
	}
	return m.Role, ""
}

type UserMessage struct {
	Role    MessageRole   `json:"role"`
	Content string        `json:"content,omitempty"`
	Parts   []ContentPart `json:"parts,omitempty"`
}

func (m UserMessage) Value() (MessageRole, string) {
	if m.Content != "" {
		return m.Role, m.Content
	}
	if len(m.Parts) > 0 {
		for _, part := range m.Parts {
			if part.Type == ContentPartText && part.Text != "" {
				return m.Role, part.Text
			}
		}
	}
	return m.Role, ""
}

type ToolMessage struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCallID string      `json:"tool_call_id"`
	ToolName   string      `json:"tool_name"`
}

func (m ToolMessage) Value() (MessageRole, string) {
	return m.Role, m.Content
}

type SystemMessage struct {
	Role    MessageRole   `json:"role"`
	Content string        `json:"content,omitempty"`
	Parts   []ContentPart `json:"parts,omitempty"`
}

func (m SystemMessage) Value() (MessageRole, string) {
	if m.Content != "" {
		return m.Role, m.Content
	}
	if len(m.Parts) > 0 {
		for _, part := range m.Parts {
			if part.Type == ContentPartText && part.Text != "" {
				return m.Role, part.Text
			}
		}
	}
	return m.Role, ""
}

// Response represents the model's response
type Response struct {
	ID          string    `json:"id"`
	Object      string    `json:"object"`
	Created     int64     `json:"created"`
	Model       string    `json:"model"`
	Messages    []Message `json:"-"`
	Usage       Usage     `json:"usage"`
	ServiceTier string    `json:"service_tier"`
}

func (r Response) MarshalJSON() ([]byte, error) {
	type Alias Response

	var msgs []map[string]any
	if len(r.Messages) > 0 {
		msgs = make([]map[string]any, len(r.Messages))
		for i, msg := range r.Messages {
			msgBytes, err := json.Marshal(msg)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(msgBytes, &msgs[i]); err != nil {
				return nil, err
			}
		}
	}

	return json.Marshal(&struct {
		Alias
		Messages []map[string]any `json:"messages,omitempty"`
	}{
		Alias:    Alias(r),
		Messages: msgs,
	})
}

func (r *Response) UnmarshalJSON(data []byte) error {
	type Alias Response

	aux := &struct {
		*Alias
		Messages []map[string]any `json:"messages,omitempty"`
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	r.Messages = make([]Message, 0)
	return nil
}

// Usage represents token usage information
type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
		AudioTokens  int `json:"audio_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens          int `json:"reasoning_tokens"`
		AudioTokens              int `json:"audio_tokens"`
		AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
		RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
	} `json:"completion_tokens_details"`
}
