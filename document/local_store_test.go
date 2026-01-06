package document

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	doc, err := store.Load(context.Background(), tempFile)
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

func TestLocalStore_OptionalMetadata(t *testing.T) {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)

	plainFileName := "plain_file.txt"
	plainFileContent := "plain file content"
	plainFilePath := filepath.Join(tempDir, plainFileName)

	err := os.WriteFile(plainFilePath, []byte(plainFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create plain file: %v", err)
	}

	doc, err := store.Load(context.Background(), plainFileName)
	if err != nil {
		t.Fatalf("Failed to load plain file: %v", err)
	}

	if doc.Filename != plainFileName {
		t.Errorf("Expected filename %s, got %s", plainFileName, doc.Filename)
	}

	content, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Failed to get document bytes: %v", err)
	}

	if string(content) != plainFileContent {
		t.Errorf("Expected content %s, got %s", plainFileContent, string(content))
	}

	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	found := false
	for _, d := range list {
		if d.Filename == plainFileName {
			found = true
			content, err := d.Bytes()
			if err != nil {
				t.Fatalf("Failed to get document bytes: %v", err)
			}
			if string(content) != plainFileContent {
				t.Errorf("Expected content %s, got %s", plainFileContent, string(content))
			}
			break
		}
	}

	if !found {
		t.Error("Plain file not found in list")
	}
}

func TestLocalStore_MixedMetadata(t *testing.T) {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)

	plainFileName := "plain.txt"
	plainFileContent := "plain content"
	plainFilePath := filepath.Join(tempDir, plainFileName)

	err := os.WriteFile(plainFilePath, []byte(plainFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create plain file: %v", err)
	}

	docWithMeta := NewInMemoryDocument("doc1", "doc1.txt", []byte("content with metadata"), nil)
	_, err = store.Save(context.Background(), docWithMeta)
	if err != nil {
		t.Fatalf("Failed to save document with metadata: %v", err)
	}

	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(list))
	}

	foundPlain := false
	foundWithMeta := false
	for _, d := range list {
		if d.Filename == plainFileName {
			foundPlain = true
		}
		if d.ID() == docWithMeta.ID() {
			foundWithMeta = true
		}
	}

	if !foundPlain {
		t.Error("Plain file not found in list")
	}

	if !foundWithMeta {
		t.Error("Document with metadata not found in list")
	}
}

func TestLocalStore_ListAllScenarios(t *testing.T) {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)
	ctx := context.Background()

	scenario1ID := "doc_with_meta"
	scenario2FileName := "doc_without_meta.txt"
	scenario3ID := "meta_only"

	scenario1Content := []byte("content with metadata")
	scenario2Content := []byte("content without metadata")
	scenario3MetaFilename := "meta_only_file.txt"

	docWithMeta := NewInMemoryDocument(scenario1ID, scenario1ID+".txt", scenario1Content, nil)
	_, err := store.Save(ctx, docWithMeta)
	if err != nil {
		t.Fatalf("Failed to save document with metadata: %v", err)
	}

	scenario2Path := filepath.Join(tempDir, scenario2FileName)
	err = os.WriteFile(scenario2Path, scenario2Content, 0644)
	if err != nil {
		t.Fatalf("Failed to create file without metadata: %v", err)
	}

	metaOnlyMetaPath := filepath.Join(tempDir, scenario3ID+".meta.json")
	metaOnlyMeta := DocumentMetadata{
		ID:        scenario3ID,
		Filename:  scenario3MetaFilename,
		FilePath:  scenario3ID,
		MimeType:  "text/plain",
		FileSize:  0,
		CreatedAt: time.Now(),
	}
	metaOnlyMetaJSON, err := json.Marshal(metaOnlyMeta)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}
	err = os.WriteFile(metaOnlyMetaPath, metaOnlyMetaJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to create metadata file: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(list))
	}

	foundScenario1 := false
	foundScenario2 := false
	foundScenario3 := false

	for _, doc := range list {
		if doc.ID() == scenario1ID {
			foundScenario1 = true
			content, err := doc.Bytes()
			if err != nil {
				t.Errorf("Failed to get content for scenario 1: %v", err)
			} else if string(content) != string(scenario1Content) {
				t.Errorf("Scenario 1: Expected content %s, got %s", string(scenario1Content), string(content))
			}
			if doc.Filename != scenario1ID+".txt" {
				t.Errorf("Scenario 1: Expected filename %s, got %s", scenario1ID+".txt", doc.Filename)
			}
		}

		if doc.Filename == scenario2FileName {
			foundScenario2 = true
			content, err := doc.Bytes()
			if err != nil {
				t.Errorf("Failed to get content for scenario 2: %v", err)
			} else if string(content) != string(scenario2Content) {
				t.Errorf("Scenario 2: Expected content %s, got %s", string(scenario2Content), string(content))
			}
			if doc.MimeType == "" {
				t.Error("Scenario 2: Expected MIME type to be set")
			}
		}

		if doc.ID() == scenario3ID {
			foundScenario3 = true
			if doc.Filename != scenario3MetaFilename {
				t.Errorf("Scenario 3: Expected filename %s, got %s", scenario3MetaFilename, doc.Filename)
			}
			content, err := doc.Bytes()
			if err == nil && len(content) != 0 {
				t.Errorf("Scenario 3: Expected empty content, got %d bytes", len(content))
			}
			if doc.FileSize != 0 {
				t.Errorf("Scenario 3: Expected file size 0, got %d", doc.FileSize)
			}
		}
	}

	if !foundScenario1 {
		t.Error("Scenario 1 (file with metadata) not found in list")
	}
	if !foundScenario2 {
		t.Error("Scenario 2 (file without metadata) not found in list")
	}
	if !foundScenario3 {
		t.Error("Scenario 3 (metadata without file) not found in list")
	}
}
