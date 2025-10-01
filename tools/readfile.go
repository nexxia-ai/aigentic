package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/nexxia-ai/aigentic/ai"
)

type ReadFileParams struct {
	FileName  string `json:"file_name"`
	StoreName string `json:"store_name"`
}

const (
	ReadFileToolName    = "read_file"
	readFileDescription = `File reading tool that retrieves and returns the content of files from a specified store.

WHEN TO USE THIS TOOL:
- Use when you need to read the contents of any file from a store
- Helpful for examining documents, text files, configuration files, or any readable content
- Perfect for accessing files stored in different locations or stores
- Useful for reading PDFs, Word documents, text files, and other document formats
- Can read binary files and encode them in base64 format

HOW TO USE:
- Provide the name of the file you want to read
- Specify the store name where the file is located
- The tool will return the complete file content as text or base64-encoded binary

FEATURES:
- Reads complete file content from specified store
- Handles various file formats including documents, text files, and readable content
- Returns content in a structured format
- Supports files from different store locations
- Works with common document types found in filesystems
- Automatically detects binary files and encodes them in base64
- Can handle images, executables, and other binary file types

LIMITATIONS:
- Cannot read encrypted files
- File must exist in the specified store
- Store must be accessible and valid
- Binary files are returned as base64-encoded strings

TIPS:
- Use with other file tools to first list available files in a store
- Ensure the store name is correct and accessible
- For large files, consider using view tool with offset/limit parameters
- Works well with documents, PDFs, Word files, and other common file types
- Binary files will be automatically detected and base64-encoded for transmission`
)

func NewReadFileTool() *ai.Tool {
	type ReadFileInput struct {
		FileName  string `json:"file_name" description:"The name of the file to read"`
		StoreName string `json:"store_name" description:"The name of the store where the file is located"`
	}

	return ai.NewTool(
		ReadFileToolName,
		readFileDescription,
		func(ctx context.Context, input ReadFileInput) (string, error) {
			args := map[string]interface{}{
				"file_name":  input.FileName,
				"store_name": input.StoreName,
			}
			result, err := readFileExecute(args)
			if err != nil {
				return "", err
			}
			return result.Content[0].Content.(string), nil
		},
	)
}

func readFileExecute(args map[string]interface{}) (*ai.ToolResult, error) {
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

	var params ReadFileParams
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

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: fmt.Sprintf("File not found: %s in store: %s", params.FileName, params.StoreName),
				}},
				Error: true,
			}, nil
		}
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error accessing file: %v", err),
			}},
			Error: true,
		}, nil
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Path is a directory, not a file: %s", filePath),
			}},
			Error: true,
		}, nil
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Error reading file: %v", err),
			}},
			Error: true,
		}, nil
	}

	// Check if content is valid UTF-8 (text file)
	if utf8.Valid(content) {
		// Return as text content
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: string(content),
			}},
			Error: false,
		}, nil
	} else {
		// Binary file - encode as base64
		base64Content := base64.StdEncoding.EncodeToString(content)
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("[BINARY FILE - Base64 Encoded]\n%s", base64Content),
			}},
			Error: false,
		}, nil
	}
}
