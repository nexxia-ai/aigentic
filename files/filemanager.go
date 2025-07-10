package files

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileInfo represents information about an uploaded file
type FileInfo struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MIMEType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// OpenAIFileManager manages temporary files for OpenAI chat sessions
type OpenAIFileManager struct {
	apiKey  string
	baseURL string
	client  *http.Client
	files   map[string]FileInfo // Track uploaded files
	mu      sync.RWMutex
}

// NewOpenAIFileManager creates a new OpenAI file manager
func NewOpenAIFileManager(apiKey string) *OpenAIFileManager {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &OpenAIFileManager{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client:  &http.Client{Timeout: 60 * time.Second},
		files:   make(map[string]FileInfo),
	}
}

// AddFile uploads a file to OpenAI and returns FileInfo
func (fm *OpenAIFileManager) AddFile(ctx context.Context, filePath string) (*FileInfo, error) {
	// Read file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Determine MIME type
	mimeType := getMimeType(filePath)

	// Upload to OpenAI
	fileID, err := fm.uploadToOpenAI(ctx, file, stat.Name(), stat.Size())
	if err != nil {
		return nil, err
	}

	// Create FileInfo
	fileInfo := &FileInfo{
		ID:        fileID,
		Filename:  stat.Name(),
		MIMEType:  mimeType,
		Size:      stat.Size(),
		CreatedAt: time.Now(),
	}

	// Store in memory
	fm.mu.Lock()
	fm.files[fileID] = *fileInfo
	fm.mu.Unlock()

	return fileInfo, nil
}

// ListFiles retrieves all files from OpenAI and returns them
func (fm *OpenAIFileManager) ListFiles(ctx context.Context) ([]FileInfo, error) {
	// Retry logic for server errors
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", fm.baseURL+"/files", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+fm.apiKey)

		// Make request
		resp, err := fm.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Parse response
			var listResp struct {
				Data []struct {
					ID        string `json:"id"`
					Object    string `json:"object"`
					Bytes     int64  `json:"bytes"`
					CreatedAt int64  `json:"created_at"`
					Filename  string `json:"filename"`
					Purpose   string `json:"purpose"`
				} `json:"data"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return nil, fmt.Errorf("failed to decode response: %w", err)
			}

			// Convert to FileInfo slice
			var files []FileInfo
			for _, file := range listResp.Data {
				// Only include files with purpose "assistants"
				if file.Purpose == "assistants" {
					files = append(files, FileInfo{
						ID:        file.ID,
						Filename:  file.Filename,
						Size:      file.Bytes,
						CreatedAt: time.Unix(file.CreatedAt, 0),
						MIMEType:  getMimeType(file.Filename),
					})
				}
			}

			return files, nil
		}

		body, _ := io.ReadAll(resp.Body)

		// If it's a server error (5xx), retry with exponential backoff
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			// Wait before retrying (exponential backoff)
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// For non-retryable errors or final attempt, return the error
		return nil, fmt.Errorf("list files failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("list files failed after %d attempts", maxRetries)
}

// RemoveFile deletes a file from OpenAI
func (fm *OpenAIFileManager) RemoveFile(ctx context.Context, fileID string) error {
	// Delete from OpenAI
	err := fm.deleteFromOpenAI(ctx, fileID)
	if err != nil {
		return err
	}

	// Remove from memory
	fm.mu.Lock()
	delete(fm.files, fileID)
	fm.mu.Unlock()

	return nil
}

// Close deletes all files and cleans up
func (fm *OpenAIFileManager) Close(ctx context.Context) error {
	fm.mu.RLock()
	fileIDs := make([]string, 0, len(fm.files))
	for fileID := range fm.files {
		fileIDs = append(fileIDs, fileID)
	}
	fm.mu.RUnlock()

	for _, fileID := range fileIDs {
		if err := fm.RemoveFile(ctx, fileID); err != nil {
			// Log error but continue with cleanup
			fmt.Printf("Failed to remove file %s: %v\n", fileID, err)
		}
	}

	return nil
}

// uploadToOpenAI uploads a file to OpenAI's file API
func (fm *OpenAIFileManager) uploadToOpenAI(ctx context.Context, file *os.File, filename string, size int64) (string, error) {
	// Retry logic for server errors
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Reset file pointer to beginning for retries
		if attempt > 1 {
			if _, err := file.Seek(0, 0); err != nil {
				return "", fmt.Errorf("failed to reset file pointer: %w", err)
			}
		}

		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add file field
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return "", fmt.Errorf("failed to create form file: %w", err)
		}

		// Copy file content
		_, err = io.Copy(part, file)
		if err != nil {
			return "", fmt.Errorf("failed to copy file content: %w", err)
		}

		// Add purpose field
		err = writer.WriteField("purpose", "assistants")
		if err != nil {
			return "", fmt.Errorf("failed to add purpose field: %w", err)
		}

		writer.Close()

		// Create request
		req, err := http.NewRequestWithContext(ctx, "POST", fm.baseURL+"/files", &buf)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+fm.apiKey)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Make request
		resp, err := fm.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to upload file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Parse response
			var uploadResp struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
				return "", fmt.Errorf("failed to decode response: %w", err)
			}
			return uploadResp.ID, nil
		}

		body, _ := io.ReadAll(resp.Body)

		// If it's a server error (5xx), retry with exponential backoff
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			// Wait before retrying (exponential backoff)
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// For non-retryable errors or final attempt, return the error
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return "", fmt.Errorf("upload failed after %d attempts", maxRetries)
}

// deleteFromOpenAI deletes a file from OpenAI's file API
func (fm *OpenAIFileManager) deleteFromOpenAI(ctx context.Context, fileID string) error {
	// Retry logic for server errors
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create request
		req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/files/%s", fm.baseURL, fileID), nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+fm.apiKey)

		// Make request
		resp, err := fm.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		body, _ := io.ReadAll(resp.Body)

		// If it's a server error (5xx), retry with exponential backoff
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			// Wait before retrying (exponential backoff)
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		// For non-retryable errors or final attempt, return the error
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("delete failed after %d attempts", maxRetries)
}

// getMimeType determines the MIME type based on file extension
func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
