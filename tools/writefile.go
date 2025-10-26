package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic"
)

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

func NewWriteFileTool() aigentic.AgentTool {
	type WriteFileInput struct {
		FileName  string `json:"file_name" description:"The name of the file to write"`
		StoreName string `json:"store_name" description:"The name of the store where the file should be written"`
		Content   string `json:"content" description:"The content to write to the file (use base64 encoding for binary files)"`
	}

	return aigentic.NewTool(
		WriteFileToolName,
		writeFileDescription,
		func(run *aigentic.AgentRun, input WriteFileInput) (string, error) {
			return writeFile(input.FileName, input.StoreName, input.Content)
		},
	)
}

// writeFile writes content to a file in the specified store
func writeFile(fileName, storeName, content string) (string, error) {
	// Validate required parameters
	if fileName == "" {
		return "", fmt.Errorf("file_name is required")
	}

	if storeName == "" {
		return "", fmt.Errorf("store_name is required")
	}

	// Construct file path based on store name
	filePath := filepath.Join(storeName, fileName)

	// Ensure the store directory exists
	storeDir := filepath.Dir(filePath)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return "", fmt.Errorf("error creating store directory: %v", err)
	}

	// Determine if content is base64-encoded binary
	var fileContent []byte
	fileType := "text"

	if strings.HasPrefix(content, "[BINARY FILE - Base64 Encoded]") {
		// Extract base64 content after the header
		lines := strings.Split(content, "\n")
		if len(lines) < 2 {
			return "", fmt.Errorf("invalid base64 content format")
		}

		base64Content := strings.Join(lines[1:], "\n")
		decoded, err := base64.StdEncoding.DecodeString(base64Content)
		if err != nil {
			return "", fmt.Errorf("error decoding base64 content: %v", err)
		}
		fileContent = decoded
		fileType = "binary"
	} else {
		// Regular text content
		fileContent = []byte(content)
	}

	// Write file content
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		return "", fmt.Errorf("error writing file: %v", err)
	}

	// Return success message
	return fmt.Sprintf("Successfully wrote %s file: %s in store: %s", fileType, fileName, storeName), nil
}
