package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/nexxia-ai/aigentic/run"
)

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

func NewReadFileTool() run.AgentTool {
	type ReadFileInput struct {
		FileName  string `json:"file_name" description:"The name of the file to read"`
		StoreName string `json:"store_name" description:"The name of the store where the file is located"`
	}

	return run.NewTool(
		ReadFileToolName,
		readFileDescription,
		func(agentRun *run.AgentRun, input ReadFileInput) (string, error) {
			return readFile(input.FileName, input.StoreName)
		},
	)
}

// readFile reads a file from the specified store and returns its content
func readFile(fileName, storeName string) (string, error) {
	// Validate required parameters
	if fileName == "" {
		return "", fmt.Errorf("file_name is required")
	}

	if storeName == "" {
		return "", fmt.Errorf("store_name is required")
	}

	// Construct file path based on store name
	filePath := filepath.Join(storeName, fileName)

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s in store: %s", fileName, storeName)
		}
		return "", fmt.Errorf("error accessing file: %v", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	// Check if content is valid UTF-8 (text file)
	if utf8.Valid(content) {
		// Return as text content
		return string(content), nil
	}

	// Binary file - encode as base64
	base64Content := base64.StdEncoding.EncodeToString(content)
	return fmt.Sprintf("[BINARY FILE - Base64 Encoded]\n%s", base64Content), nil
}
