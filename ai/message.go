package ai

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
	_ Message = ResourceMessage{}
)

type ToolCall struct {
	ID     string
	Type   string
	Name   string
	Args   string
	Result any
}

type AIMessage struct {
	Role            MessageRole
	Content         string
	Think           string
	OriginalContent string
	ToolCalls       []ToolCall
	Extra           map[string]any
	Response        Response
}

func (m AIMessage) Value() (MessageRole, string) {
	return m.Role, m.Content
}

type UserMessage struct {
	Role    MessageRole
	Content string
}

func (m UserMessage) Value() (MessageRole, string) {
	return m.Role, m.Content
}

type ToolMessage struct {
	Role       MessageRole
	Content    string
	ToolCallID string
}

func (m ToolMessage) Value() (MessageRole, string) {
	return m.Role, m.Content
}

type SystemMessage struct {
	Role    MessageRole
	Content string
}

func (m SystemMessage) Value() (MessageRole, string) {
	return m.Role, m.Content
}

type ResourceMessage struct {
	Role        MessageRole
	URI         string `json:"uri"`                   // The URI of this resource.
	Name        string `json:"name"`                  // A human-readable name for this resource.
	Description string `json:"description,omitempty"` // A description of what this resource represents.
	MIMEType    string `json:"mimeType,omitempty"`    // The MIME type of this resource, if known.
	Body        any    `json:"body"`                  // The body of the resource.
	Type        string `json:"type"`                  // The type of the resource. "text", "image", "resource"
	Attributes  map[string]any
}

func (m ResourceMessage) Value() (MessageRole, string) {
	return m.Role, m.Name
}

// Response represents the model's response
type Response struct {
	ID          string    `json:"id"`
	Object      string    `json:"object"`
	Created     int64     `json:"created"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Usage       Usage     `json:"usage"`
	ServiceTier string    `json:"service_tier"`
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
