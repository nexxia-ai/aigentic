package ai

import "github.com/nexxia-ai/aigentic/document"

type ToolContent struct {
	Type    string
	Content any
}

type ToolResult struct {
	Content   []ToolContent
	Documents []*document.Document
	Error     bool
}

