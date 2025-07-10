# AI Files Package

The `ai/files` package provides file management capabilities for OpenAI's file API, allowing you to upload files and reference them in chat conversations.

## Overview

This package implements a simple file manager that:
- Uploads files to OpenAI's file API
- Returns `FileInfo` with file metadata
- Allows manual embedding of file references in user messages
- Automatically cleans up files when the manager is closed

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/irai/rag/ai"
    "github.com/irai/rag/ai/files"
)

func main() {
    // Create file manager for chat session
    fileManager := files.NewOpenAIFileManager("")
    defer fileManager.Close(context.Background())

    // Upload a file and get FileInfo
    fileInfo, err := fileManager.AddFile(context.Background(), "document.pdf")
    if err != nil {
        log.Fatal(err)
    }

    // Create prompt with file reference
    prompt := fmt.Sprintf(`Please analyze this document and summarize the key points.

Attached file: file://%s (%s)

Focus on the main findings and recommendations.`,
        fileInfo.ID, fileInfo.Filename)

    // Create user message
    userMsg := ai.UserMessage{
        Role:    ai.UserRole,
        Content: prompt,
    }

    // Use with OpenAI model
    model := ai.NewOpenAIModel("gpt-4o-mini", "")
    messages := []ai.Message{userMsg}
    response, err := model.Generate(context.Background(), messages, nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", response.Content)
}
```

### Multiple Files

```go
// Upload multiple files
files := []string{"report.pdf", "data.csv", "presentation.pptx"}
var fileInfos []*files.FileInfo

for _, filePath := range files {
    fileInfo, err := fileManager.AddFile(context.Background(), filePath)
    if err != nil {
        log.Printf("Failed to upload %s: %v", filePath, err)
        continue
    }
    fileInfos = append(fileInfos, fileInfo)
}

// Build prompt with all file references
prompt := "Please analyze the following documents:\n\n"
for _, fileInfo := range fileInfos {
    prompt += fmt.Sprintf("- file://%s (%s, %d bytes)\n", 
        fileInfo.ID, fileInfo.Filename, fileInfo.Size)
}
prompt += "\nProvide a comprehensive analysis of all documents."
```

## API Reference

### FileInfo

```go
type FileInfo struct {
    ID        string    `json:"id"`         // OpenAI file ID
    Filename  string    `json:"filename"`   // Original filename
    MIMEType  string    `json:"mime_type"`  // File MIME type
    Size      int64     `json:"size"`       // File size in bytes
    CreatedAt time.Time `json:"created_at"` // Upload timestamp
}
```

### OpenAIFileManager

```go
// NewOpenAIFileManager creates a new file manager
func NewOpenAIFileManager(apiKey string) *OpenAIFileManager

// AddFile uploads a file and returns FileInfo
func (fm *OpenAIFileManager) AddFile(ctx context.Context, filePath string) (*FileInfo, error)

// RemoveFile deletes a specific file
func (fm *OpenAIFileManager) RemoveFile(ctx context.Context, fileID string) error

// Close deletes all files and cleans up resources
func (fm *OpenAIFileManager) Close(ctx context.Context) error
```

## File Reference Format

Files are referenced in prompts using the format:
```
file://{fileID} ({filename})
```

Example:
```
file://file-abc123 (document.pdf)
```

## Environment Variables

- `OPENAI_API_KEY`: Your OpenAI API key (used if not provided to constructor)

## Notes

- Files are automatically deleted when the FileManager is closed
- File references are manually embedded in user prompts
- The package is designed for temporary file management during chat sessions
- Files uploaded with purpose "assistants" for OpenAI compatibility

## Testing

Run tests with:
```bash
go test ./ai/files
```

Integration tests require `OPENAI_API_KEY` environment variable. 