package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStore_Quick(t *testing.T) {
	b := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	s := string(b)
	s2 := string(b)

	if &s == &s2 {
		t.Errorf("s and s2 should not be the same")
	}

	hello := "Hello, World!"
	b3 := []byte(hello)
	s3 := string(b3)

	if &hello == &s3 {
		t.Errorf("hello and s3 should not be the same")
	}

}

func TestLocalStore_Open(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	content := "Hello, World!"
	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create LocalStore
	store := NewLocalStore(tempDir)

	// Test Open
	doc, err := store.Open(context.Background(), tempFile)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Verify document metadata
	if doc.Filename != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", doc.Filename)
	}
	if doc.MimeType != "text/plain; charset=utf-8" {
		t.Errorf("Expected type TXT, got %s", doc.MimeType)
	}
	if doc.FileSize != int64(len(content)) {
		t.Errorf("Expected file size %d, got %d", len(content), doc.FileSize)
	}

	// Access content to trigger lazy loading
	docContent, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Failed to get content: %v", err)
	}
	if string(docContent) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(docContent))
	}

	// Test binary access
	binaryData, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Failed to get binary data: %v", err)
	}
	if len(binaryData) != len(content) {
		t.Errorf("Expected binary data length %d, got %d", len(content), len(binaryData))
	}
}

func TestDocument_IsChunk(t *testing.T) {
	// Test main document
	doc := NewInMemoryDocument(
		"test.txt",
		"test.txt",
		[]byte("test content"),
		nil,
	)
	if doc.IsChunk() {
		t.Error("Main document should not be a chunk")
	}

	// Test chunk document
	chunk := NewInMemoryDocument(
		"test_chunk_0.txt",
		"test_chunk_0.txt",
		[]byte("chunk content"),
		doc,
	)
	if !chunk.IsChunk() {
		t.Error("Chunk document should be identified as chunk")
	}
}
