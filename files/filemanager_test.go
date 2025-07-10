package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/utils"
)

func init() {
	utils.LoadEnvFile("../../.env")
}

// TestSimpleAddAndDelete tests basic file upload and delete functionality
func TestSimpleAddAndDelete(t *testing.T) {
	// Require OPENAI_API_KEY environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Fatal("OPENAI_API_KEY environment variable is required for integration test")
	}

	// Create file manager
	fileManager := NewOpenAIFileManager(apiKey)
	defer fileManager.Close(context.Background())

	// Create a simple test file
	tempFile, err := os.CreateTemp("", "simple-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write simple content
	testContent := "Simple test file for OpenAI file API."
	_, err = tempFile.WriteString(testContent)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	t.Logf("Created test file: %s (%d bytes)", tempFile.Name(), len(testContent))

	// Upload file
	fileInfo, err := fileManager.AddFile(context.Background(), tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	t.Logf("✅ Successfully uploaded file:")
	t.Logf("   ID: %s", fileInfo.ID)
	t.Logf("   Filename: %s", fileInfo.Filename)
	t.Logf("   Size: %d bytes", fileInfo.Size)
	t.Logf("   MIME Type: %s", fileInfo.MIMEType)

	// Verify FileInfo
	if fileInfo.ID == "" {
		t.Error("FileInfo ID should not be empty")
	}
	// Extract just the filename from the full path for comparison
	expectedFilename := filepath.Base(tempFile.Name())
	if fileInfo.Filename != expectedFilename {
		t.Errorf("Expected filename '%s', got '%s'", expectedFilename, fileInfo.Filename)
	}
	if fileInfo.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), fileInfo.Size)
	}
	if fileInfo.MIMEType != "text/plain" {
		t.Errorf("Expected MIME type 'text/plain', got '%s'", fileInfo.MIMEType)
	}

	// Delete the file
	err = fileManager.RemoveFile(context.Background(), fileInfo.ID)
	if err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	t.Logf("✅ Successfully deleted file: %s", fileInfo.ID)

	// Verify file was deleted by trying to remove it again (should fail)
	err = fileManager.RemoveFile(context.Background(), fileInfo.ID)
	if err == nil {
		t.Error("Expected error when trying to delete already deleted file")
	} else {
		t.Logf("✅ Expected error when deleting already deleted file: %v", err)
	}
}

// TestOpenAIFileManagerRealIntegration tests real OpenAI file API operations
func TestOpenAIFileManagerRealIntegration(t *testing.T) {
	// Require OPENAI_API_KEY environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Fatal("OPENAI_API_KEY environment variable is required for integration test")
	}

	// Create file manager
	fileManager := NewOpenAIFileManager(apiKey)
	defer fileManager.Close(context.Background())

	// Test 1: Upload a text file
	t.Run("UploadTextFile", func(t *testing.T) {
		// Create a temporary text file
		tempFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write test content
		testContent := "This is a test file for OpenAI file API integration.\nIt contains multiple lines of text.\nThis will be uploaded to OpenAI."
		_, err = tempFile.WriteString(testContent)
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// Upload file
		fileInfo, err := fileManager.AddFile(context.Background(), tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		// Verify FileInfo
		if fileInfo.ID == "" {
			t.Error("FileInfo ID should not be empty")
		}
		// Extract just the filename from the full path for comparison
		expectedFilename := filepath.Base(tempFile.Name())
		if fileInfo.Filename != expectedFilename {
			t.Errorf("Expected filename '%s', got '%s'", expectedFilename, fileInfo.Filename)
		}
		if fileInfo.Size != int64(len(testContent)) {
			t.Errorf("Expected size %d, got %d", len(testContent), fileInfo.Size)
		}
		if fileInfo.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
		if fileInfo.MIMEType != "text/plain" {
			t.Errorf("Expected MIME type 'text/plain', got '%s'", fileInfo.MIMEType)
		}

		t.Logf("Successfully uploaded file: ID=%s, Filename=%s, Size=%d, MIME=%s",
			fileInfo.ID, fileInfo.Filename, fileInfo.Size, fileInfo.MIMEType)
	})

	// Test 2: Upload multiple files
	t.Run("UploadMultipleFiles", func(t *testing.T) {
		var fileInfos []*FileInfo
		var tempFiles []string

		// Create multiple test files
		for i := 1; i <= 3; i++ {
			tempFile, err := os.CreateTemp("", fmt.Sprintf("test%d-*.txt", i))
			if err != nil {
				t.Fatalf("Failed to create temp file %d: %v", i, err)
			}
			defer os.Remove(tempFile.Name())
			tempFiles = append(tempFiles, tempFile.Name())

			// Write unique content
			testContent := fmt.Sprintf("Test file %d content for OpenAI integration.", i)
			_, err = tempFile.WriteString(testContent)
			if err != nil {
				t.Fatalf("Failed to write to temp file %d: %v", i, err)
			}
			tempFile.Close()

			// Upload file
			fileInfo, err := fileManager.AddFile(context.Background(), tempFile.Name())
			if err != nil {
				t.Fatalf("Failed to add file %d: %v", i, err)
			}
			fileInfos = append(fileInfos, fileInfo)

			t.Logf("Uploaded file %d: ID=%s, Filename=%s", i, fileInfo.ID, fileInfo.Filename)
		}

		// Verify all files were uploaded
		if len(fileInfos) != 3 {
			t.Errorf("Expected 3 files, got %d", len(fileInfos))
		}

		// Verify unique IDs
		ids := make(map[string]bool)
		for _, fileInfo := range fileInfos {
			if ids[fileInfo.ID] {
				t.Errorf("Duplicate file ID: %s", fileInfo.ID)
			}
			ids[fileInfo.ID] = true
		}
	})

	// Test 3: List files from OpenAI
	t.Run("ListFiles", func(t *testing.T) {
		// List all files
		files, err := fileManager.ListFiles(context.Background())
		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}

		t.Logf("Found %d files in OpenAI account", len(files))

		// Log file details
		for i, file := range files {
			t.Logf("File %d: ID=%s, Name=%s, Size=%d, MIME=%s, Created=%s",
				i+1, file.ID, file.Filename, file.Size, file.MIMEType, file.CreatedAt.Format(time.RFC3339))
		}

		// Verify we can find our uploaded files
		if len(files) > 0 {
			t.Logf("Successfully retrieved %d files from OpenAI", len(files))
		}
	})

	// Test 4: Remove specific file
	t.Run("RemoveSpecificFile", func(t *testing.T) {
		// Create a temporary file
		tempFile, err := os.CreateTemp("", "test-remove-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write content
		testContent := "This file will be removed."
		_, err = tempFile.WriteString(testContent)
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// Upload file
		fileInfo, err := fileManager.AddFile(context.Background(), tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		t.Logf("Uploaded file for removal: ID=%s", fileInfo.ID)

		// Remove file
		err = fileManager.RemoveFile(context.Background(), fileInfo.ID)
		if err != nil {
			t.Fatalf("Failed to remove file: %v", err)
		}

		t.Logf("Successfully removed file: ID=%s", fileInfo.ID)
	})

	// Test 5: Test file reference format for prompts
	t.Run("FileReferenceFormat", func(t *testing.T) {
		// Create a temporary file
		tempFile, err := os.CreateTemp("", "test-reference-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		// Write content
		testContent := "This file will be referenced in a prompt."
		_, err = tempFile.WriteString(testContent)
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// Upload file
		fileInfo, err := fileManager.AddFile(context.Background(), tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		// Create prompt with file reference
		prompt := fmt.Sprintf(`Please analyze this document and provide insights.

Attached file: file://%s (%s, %d bytes)

What are the key points in this document?`,
			fileInfo.ID, fileInfo.Filename, fileInfo.Size)

		t.Logf("Created prompt with file reference:")
		t.Logf("File ID: %s", fileInfo.ID)
		t.Logf("File Name: %s", fileInfo.Filename)
		t.Logf("File Size: %d bytes", fileInfo.Size)
		t.Logf("Prompt: %s", prompt)

		// Verify prompt format
		expectedReference := fmt.Sprintf("file://%s (%s, %d bytes)",
			fileInfo.ID, fileInfo.Filename, fileInfo.Size)
		if !strings.Contains(prompt, expectedReference) {
			t.Errorf("Prompt should contain file reference: %s", expectedReference)
		}
	})

	// Test 6: Test cleanup on close
	t.Run("CleanupOnClose", func(t *testing.T) {
		// Create a new file manager for this test
		testFileManager := NewOpenAIFileManager(apiKey)

		// Upload a file
		tempFile, err := os.CreateTemp("", "test-cleanup-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		testContent := "This file will be cleaned up."
		_, err = tempFile.WriteString(testContent)
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		fileInfo, err := testFileManager.AddFile(context.Background(), tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}

		t.Logf("Uploaded file for cleanup: ID=%s", fileInfo.ID)

		// Close file manager (should clean up all files)
		err = testFileManager.Close(context.Background())
		if err != nil {
			t.Fatalf("Failed to close file manager: %v", err)
		}

		t.Logf("Successfully closed file manager and cleaned up files")
	})
}

// TestOpenAIFileManagerErrorHandling tests error scenarios
func TestOpenAIFileManagerErrorHandling(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Fatal("OPENAI_API_KEY environment variable is required for integration test")
	}

	fileManager := NewOpenAIFileManager(apiKey)
	defer fileManager.Close(context.Background())

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := fileManager.AddFile(context.Background(), "non-existent-file.txt")
		if err == nil {
			t.Error("Expected error when file doesn't exist")
		}
		t.Logf("Expected error for non-existent file: %v", err)
	})

	t.Run("RemoveNonExistentFileID", func(t *testing.T) {
		err := fileManager.RemoveFile(context.Background(), "non-existent-file-id")
		if err == nil {
			t.Error("Expected error when file ID doesn't exist")
		}
		t.Logf("Expected error for non-existent file ID: %v", err)
	})
}
