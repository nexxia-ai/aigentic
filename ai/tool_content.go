package ai

type ToolContent struct {
	Type    string
	Content any
}

type ToolResult struct {
	Content []ToolContent
	Error   bool
}
