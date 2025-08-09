package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
)

type WriteFileParams struct {
	FileName  string `json:"file_name"`
	StoreName string `json:"store_name"`
	Content   string `json:"content"`
}

const (
	WriteFileToolName    = "write_file"
	writeFileDescription = `File writing tool that creates or overwrites files in a specified store.

WHEN TO USE THIS TOOL:
- Use when you need to create new files or overwrite existing files in a store
- Helpful for saving documents, text files, configuration files, or any content
- Perfect for writing files to different locations or stores
- Useful for creating PDFs, Word documents, text files, and other document formats
- Can write binary files by providing base64-encoded content

HOW TO USE:
- Provide the name of the file you want to write
- Specify the store name where the file should be written
- Provide the content to write to the file
- The tool will create the file or overwrite existing files

FEATURES:
- Writes complete file content to specified store
- Handles various file formats including documents, text files, and readable content
- Creates files in a structured format
- Supports writing to different store locations
- Works with common document types found in filesystems
- Automatically detects base64-encoded binary content and decodes it
- Can handle images, executables, and other binary file types
- Overwrites existing files without prompting

LIMITATIONS:
- Binary files must be provided as base64-encoded strings

TIPS:
- Use with other file tools to first list available files in a store
- Ensure the store name is correct and writable
- For binary files, provide content as base64-encoded strings
- Works well with documents, PDFs, Word files, and other common file types
- Binary files should be provided with base64 encoding for proper handling`
)

func NewWriteFileTool() *ai.Tool {
	return &ai.Tool{
		Name:        WriteFileToolName,
		Description: writeFileDescription,
		InputSchema: map[string]interface{}{
			"file_name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the file to write",
			},
			"store_name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the store where the file should be written",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file (use base64 encoding for binary files)",
			},
		},
		Execute: writeFileExecute,
	}
}

func writeFileExecute(args map[string]interface{}) (*ai.ToolResult, error) {
	// Parse parameters
	jsonData, err := json.Marshal(args)
	if err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error marshaling parameters: %v", err),
			}},
			Error: true,
		}, nil
	}

	var params WriteFileParams
	if err := json.Unmarshal(jsonData, &params); err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error parsing parameters: %v", err),
			}},
			Error: true,
		}, nil
	}

	// Validate required parameters
	if params.FileName == "" {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "file_name is required",
			}},
			Error: true,
		}, nil
	}

	if params.StoreName == "" {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "store_name is required",
			}},
			Error: true,
		}, nil
	}

	// Construct file path based on store name
	filePath := filepath.Join(params.StoreName, params.FileName)

	// Ensure the store directory exists
	storeDir := filepath.Dir(filePath)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error creating store directory: %v", err),
			}},
			Error: true,
		}, nil
	}

	// Determine if content is base64-encoded binary
	var fileContent []byte
	if strings.HasPrefix(params.Content, "[BINARY FILE - Base64 Encoded]") {
		// Extract base64 content after the header
		lines := strings.Split(params.Content, "\n")
		if len(lines) < 2 {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: "Invalid base64 content format",
				}},
				Error: true,
			}, nil
		}

		base64Content := strings.Join(lines[1:], "\n")
		decoded, err := base64.StdEncoding.DecodeString(base64Content)
		if err != nil {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: fmt.Sprintf("Error decoding base64 content: %v", err),
				}},
				Error: true,
			}, nil
		}
		fileContent = decoded
	} else {
		// Regular text content
		fileContent = []byte(params.Content)
	}

	// Write file content
	err = os.WriteFile(filePath, fileContent, 0644)
	if err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error writing file: %v", err),
			}},
			Error: true,
		}, nil
	}

	// Return success message
	fileType := "text"
	if strings.HasPrefix(params.Content, "[BINARY FILE - Base64 Encoded]") {
		fileType = "binary"
	}

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: fmt.Sprintf("Successfully wrote %s file: %s in store: %s", fileType, params.FileName, params.StoreName),
		}},
		Error: false,
	}, nil
}
