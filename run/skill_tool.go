package run

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
)

const (
	readFileToolName = "read_file"
	maxReadFileBytes = 64 * 1024
)

func (r *AgentRun) readFileTool() AgentTool {
	return AgentTool{
		Name:        readFileToolName,
		Description: "Read a file from the current run workspace by relative path.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file under the run workspace.",
				},
			},
			"required": []string{"path"},
		},
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			return run.executeReadFile(args), nil
		},
	}
}

func (r *AgentRun) executeReadFile(args map[string]interface{}) *ToolCallResult {
	if r == nil || r.agentContext == nil {
		return readFileToolError("agent context is not set")
	}

	rawPath, _ := args["path"].(string)
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return readFileToolError("path is required")
	}
	if filepath.IsAbs(path) {
		return readFileToolError("path must be relative")
	}

	doc := r.agentContext.GetDocument(path)
	if doc == nil {
		return readFileToolError(fmt.Sprintf("file not found or invalid path: %s", path))
	}
	content, err := doc.Bytes()
	if err != nil {
		return readFileToolError(fmt.Sprintf("failed to read file %s: %v", path, err))
	}

	truncated := false
	if len(content) > maxReadFileBytes {
		content = content[:maxReadFileBytes]
		truncated = true
	}

	text := string(content)
	if truncated {
		text += fmt.Sprintf("\n\n[truncated to %d bytes]", maxReadFileBytes)
	}

	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: text}},
		},
	}
}

func readFileToolError(message string) *ToolCallResult {
	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: message}},
			Error:   true,
		},
	}
}

