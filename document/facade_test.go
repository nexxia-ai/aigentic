package document

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getMemoryStoreID() string {
	return defaultMemoryStore.ID()
}

func setupLocalStore(t *testing.T, storeName string) string {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)
	storeID := store.ID()
	if err := RegisterStore(store); err != nil {
		t.Fatalf("failed to register store: %v", err)
	}
	t.Cleanup(func() { UnregisterStore(storeID) })
	return storeID
}

func createTestFile(t *testing.T, content string, filename string) string {
	tempFile := filepath.Join(t.TempDir(), filename)
	if content != "" {
		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	return tempFile
}

func assertError(t *testing.T, err error, wantErr bool, errContains string) {
	if wantErr {
		if err == nil {
			t.Error("expected error, got nil")
		} else if errContains != "" && !strings.Contains(err.Error(), errContains) {
			t.Errorf("expected error to contain '%s', got '%v'", errContains, err)
		}
	} else {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func assertDocumentNotNil(t *testing.T, doc *Document) {
	if doc == nil {
		t.Fatal("expected document, got nil")
	}
}

func assertDocumentContent(t *testing.T, doc *Document, expectedContent string) {
	content, err := doc.Bytes()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}
	if string(content) != expectedContent {
		t.Errorf("expected content '%s', got '%s'", expectedContent, string(content))
	}
}

func assertDocumentFilename(t *testing.T, doc *Document, expectedFilename string) {
	if doc.Filename != expectedFilename {
		t.Errorf("expected filename '%s', got '%s'", expectedFilename, doc.Filename)
	}
}

func assertDocumentFileSize(t *testing.T, doc *Document, expectedSize int64) {
	if doc.FileSize != expectedSize {
		t.Errorf("expected file size %d, got %d", expectedSize, doc.FileSize)
	}
}

func assertMimeTypeSet(t *testing.T, doc *Document) {
	if doc.MimeType == "" {
		t.Error("expected mime type to be set")
	}
}

func createDocument(t *testing.T, ctx context.Context, storeName, filename, content string) *Document {
	doc, err := Create(ctx, storeName, filename, bytes.NewReader([]byte(content)))
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}
	return doc
}

func assertDocumentDeleted(t *testing.T, ctx context.Context, storeName, docID string) {
	_, err := Open(ctx, storeName, docID)
	if err == nil {
		t.Error("expected error when opening deleted document")
	}
}

func TestUpload_SuccessfulUploadToMemoryStore(t *testing.T) {
	ctx := context.Background()
	fileContent := "test content"
	filename := "test.txt"
	filePath := createTestFile(t, fileContent, filename)

	doc, err := Upload(ctx, getMemoryStoreID(), filePath)

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	assertDocumentFilename(t, doc, filename)
	assertDocumentContent(t, doc, fileContent)
	assertDocumentFileSize(t, doc, int64(len(fileContent)))
	assertMimeTypeSet(t, doc)
}

func TestUpload_SuccessfulUploadToLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_upload"
	storeID := setupLocalStore(t, storeName)

	fileContent := "local store content"
	filename := "local_test.txt"
	filePath := createTestFile(t, fileContent, filename)

	doc, err := Upload(ctx, storeID, filePath)

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	assertDocumentFilename(t, doc, filename)
	assertDocumentContent(t, doc, fileContent)
}

func TestUpload_UploadWithBinaryContent(t *testing.T) {
	ctx := context.Background()
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	fileContent := string(binaryContent)
	filename := "binary.bin"
	filePath := createTestFile(t, fileContent, filename)

	doc, err := Upload(ctx, getMemoryStoreID(), filePath)

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	content, err := doc.Bytes()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}
	if len(content) != 6 {
		t.Errorf("expected 6 bytes, got %d", len(content))
	}
}

func TestUpload_UploadNonExistentFile(t *testing.T) {
	ctx := context.Background()
	filename := "nonexistent.txt"
	filePath := filepath.Join(t.TempDir(), filename)

	doc, err := Upload(ctx, getMemoryStoreID(), filePath)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if doc != nil {
		t.Error("expected nil document on error")
	}
	if !strings.Contains(err.Error(), "failed to stat file") {
		t.Errorf("expected error to contain 'failed to stat file', got '%v'", err)
	}
}

func TestUpload_UploadToNonExistentStore(t *testing.T) {
	ctx := context.Background()
	fileContent := "test"
	filename := "test.txt"
	filePath := createTestFile(t, fileContent, filename)

	doc, err := Upload(ctx, "nonexistent_store", filePath)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if doc != nil {
		t.Error("expected nil document on error")
	}
	if !strings.Contains(err.Error(), "store nonexistent_store not found") {
		t.Errorf("expected error to contain 'store nonexistent_store not found', got '%v'", err)
	}
}

func TestCreate_SuccessfulCreateInMemoryStore(t *testing.T) {
	ctx := context.Background()
	filename := "create.txt"
	content := "create content"

	doc, err := Create(ctx, getMemoryStoreID(), filename, bytes.NewReader([]byte(content)))

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	assertDocumentFilename(t, doc, filename)
	assertDocumentContent(t, doc, content)
}

func TestCreate_SuccessfulCreateInLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_create"
	storeID := setupLocalStore(t, storeName)

	filename := "local_create.txt"
	content := "local create content"

	doc, err := Create(ctx, storeID, filename, bytes.NewReader([]byte(content)))

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	assertDocumentContent(t, doc, content)
}

func TestCreate_CreateInNonExistentStore(t *testing.T) {
	ctx := context.Background()
	filename := "test.txt"
	content := "test"

	doc, err := Create(ctx, "nonexistent", filename, bytes.NewReader([]byte(content)))

	if err == nil {
		t.Error("expected error, got nil")
	}
	if doc != nil {
		t.Error("expected nil document on error")
	}
	if !strings.Contains(err.Error(), "store nonexistent not found") {
		t.Errorf("expected error to contain 'store nonexistent not found', got '%v'", err)
	}
}

func TestCreate_CreateWithEmptyContent(t *testing.T) {
	ctx := context.Background()
	filename := "empty.txt"
	content := ""

	doc, err := Create(ctx, getMemoryStoreID(), filename, bytes.NewReader([]byte(content)))

	assertError(t, err, false, "")
	assertDocumentNotNil(t, doc)
	docContent, err := doc.Bytes()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}
	if len(docContent) != 0 {
		t.Errorf("expected empty content, got %d bytes", len(docContent))
	}
}

func TestOpen_OpenExistingDocumentFromMemoryStore(t *testing.T) {
	ctx := context.Background()
	doc := createDocument(t, ctx, getMemoryStoreID(), "open_test.txt", "open content")
	docID := doc.ID()

	openedDoc, err := Open(ctx, getMemoryStoreID(), docID)

	assertError(t, err, false, "")
	assertDocumentNotNil(t, openedDoc)
	assertDocumentFilename(t, openedDoc, "open_test.txt")
	assertDocumentContent(t, openedDoc, "open content")
}

func TestOpen_OpenExistingDocumentFromLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_open"
	storeID := setupLocalStore(t, storeName)

	doc := createDocument(t, ctx, storeID, "local_open.txt", "local open content")
	docID := doc.ID()

	openedDoc, err := Open(ctx, storeID, docID)

	assertError(t, err, false, "")
	assertDocumentNotNil(t, openedDoc)
	assertDocumentContent(t, openedDoc, "local open content")
}

func TestOpen_OpenNonExistentDocument(t *testing.T) {
	ctx := context.Background()
	docID := "nonexistent_id"

	doc, err := Open(ctx, getMemoryStoreID(), docID)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if doc != nil {
		t.Error("expected nil document on error")
	}
	if !strings.Contains(err.Error(), "failed to open document") {
		t.Errorf("expected error to contain 'failed to open document', got '%v'", err)
	}
}

func TestOpen_OpenFromNonExistentStore(t *testing.T) {
	ctx := context.Background()
	docID := "some_id"

	doc, err := Open(ctx, "nonexistent", docID)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if doc != nil {
		t.Error("expected nil document on error")
	}
	if !strings.Contains(err.Error(), "store nonexistent not found") {
		t.Errorf("expected error to contain 'store nonexistent not found', got '%v'", err)
	}
}

func TestList_ListDocumentsFromMemoryStore(t *testing.T) {
	ctx := context.Background()
	filenames := []string{"list1.txt", "list2.txt", "list3.txt"}
	for _, filename := range filenames {
		createDocument(t, ctx, getMemoryStoreID(), filename, "content for "+filename)
	}

	docs, err := List(ctx, getMemoryStoreID())

	assertError(t, err, false, "")
	if docs == nil {
		t.Fatal("expected documents slice, got nil")
	}
	if len(docs) < len(filenames) {
		t.Errorf("expected at least %d documents, got %d", len(filenames), len(docs))
	}
	found := make(map[string]bool)
	for _, doc := range docs {
		found[doc.Filename] = true
	}
	for _, filename := range filenames {
		if !found[filename] {
			t.Errorf("expected to find filename '%s' in list", filename)
		}
	}
}

func TestList_ListDocumentsFromLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_list"
	storeID := setupLocalStore(t, storeName)

	filenames := []string{"local1.txt", "local2.txt"}
	for _, filename := range filenames {
		createDocument(t, ctx, storeID, filename, "content")
	}

	docs, err := List(ctx, storeID)

	assertError(t, err, false, "")
	if docs == nil {
		t.Fatal("expected documents slice, got nil")
	}
	if len(docs) < len(filenames) {
		t.Errorf("expected at least %d documents, got %d", len(filenames), len(docs))
	}
}

func TestList_ListFromNonExistentStore(t *testing.T) {
	ctx := context.Background()

	docs, err := List(ctx, "nonexistent")

	if err == nil {
		t.Error("expected error, got nil")
	}
	if docs != nil {
		t.Error("expected nil documents on error")
	}
	if !strings.Contains(err.Error(), "store nonexistent not found") {
		t.Errorf("expected error to contain 'store nonexistent not found', got '%v'", err)
	}
}

func TestList_ListEmptyStore(t *testing.T) {
	ctx := context.Background()

	docs, err := List(ctx, getMemoryStoreID())

	assertError(t, err, false, "")
	if docs == nil {
		t.Fatal("expected documents slice, got nil")
	}
}

func TestList_ListDocumentsWithAutoCreatedMetadataFromExistingFiles(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content of file 1"), 0644); err != nil {
		t.Fatalf("failed to create file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "file2.json"), []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("failed to create file2.json: %v", err)
	}
	store := NewLocalStore(tempDir)
	storeID := store.ID()
	if err := RegisterStore(store); err != nil {
		t.Fatalf("failed to register store: %v", err)
	}
	t.Cleanup(func() { UnregisterStore(storeID) })

	expectedFilenames := []string{"file1.txt", "file2.json"}

	docs, err := List(ctx, storeID)

	assertError(t, err, false, "")
	if docs == nil {
		t.Fatal("expected documents slice, got nil")
	}
	if len(docs) != len(expectedFilenames) {
		t.Errorf("expected %d documents, got %d", len(expectedFilenames), len(docs))
	}
	found := make(map[string]*Document)
	for _, doc := range docs {
		found[doc.Filename] = doc
	}
	for _, filename := range expectedFilenames {
		doc, exists := found[filename]
		if !exists {
			t.Errorf("expected to find document '%s' in list", filename)
			continue
		}
		if doc.Filename != filename {
			t.Errorf("expected filename '%s', got '%s'", filename, doc.Filename)
		}
		if doc.FileSize <= 0 {
			t.Errorf("expected file size > 0 for '%s', got %d", filename, doc.FileSize)
		}
		if doc.MimeType == "" {
			t.Errorf("expected mime type to be set for '%s'", filename)
		}
		if filename == "file1.txt" {
			assertDocumentContent(t, doc, "content of file 1")
			assertDocumentFileSize(t, doc, int64(len("content of file 1")))
		}
		if filename == "file2.json" {
			assertDocumentContent(t, doc, `{"key": "value"}`)
			assertDocumentFileSize(t, doc, int64(len(`{"key": "value"}`)))
		}
	}
}

func TestDelete_DeleteDocumentFromMemoryStore(t *testing.T) {
	ctx := context.Background()
	doc := createDocument(t, ctx, getMemoryStoreID(), "delete_test.txt", "delete me")
	docID := doc.ID()

	err := Delete(ctx, getMemoryStoreID(), docID)

	assertError(t, err, false, "")
	assertDocumentDeleted(t, ctx, getMemoryStoreID(), docID)
}

func TestDelete_DeleteDocumentFromLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_delete"
	storeID := setupLocalStore(t, storeName)

	doc := createDocument(t, ctx, storeID, "local_delete.txt", "delete me")
	docID := doc.ID()

	err := Delete(ctx, storeID, docID)

	assertError(t, err, false, "")
	assertDocumentDeleted(t, ctx, storeID, docID)
}

func TestDelete_DeleteFromNonExistentStore(t *testing.T) {
	ctx := context.Background()
	docID := "some_id"

	err := Delete(ctx, "nonexistent", docID)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "store nonexistent not found") {
		t.Errorf("expected error to contain 'store nonexistent not found', got '%v'", err)
	}
}

func TestRemove_RemoveDocumentWithStore(t *testing.T) {
	ctx := context.Background()
	doc := createDocument(t, ctx, getMemoryStoreID(), "remove_test.txt", "remove me")
	storeID := getMemoryStoreID()
	docID := doc.ID()

	err := Remove(ctx, doc)

	assertError(t, err, false, "")
	assertDocumentDeleted(t, ctx, storeID, docID)
}

func TestRemove_RemoveDocumentFromLocalStore(t *testing.T) {
	ctx := context.Background()
	storeName := "local_remove"
	storeID := setupLocalStore(t, storeName)

	doc := createDocument(t, ctx, storeID, "local_remove.txt", "remove me")
	docID := doc.ID()

	err := Remove(ctx, doc)

	assertError(t, err, false, "")
	assertDocumentDeleted(t, ctx, storeName, docID)
}

func TestRemove_RemoveNilDocument(t *testing.T) {
	ctx := context.Background()
	var doc *Document = nil

	err := Remove(ctx, doc)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "document is nil") {
		t.Errorf("expected error to contain 'document is nil', got '%v'", err)
	}
}

func TestRemove_RemoveDocumentWithoutStore(t *testing.T) {
	ctx := context.Background()
	doc := &Document{
		id:       "test_id",
		Filename: "test.txt",
		store:    nil,
	}

	err := Remove(ctx, doc)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "document has no backing store") {
		t.Errorf("expected error to contain 'document has no backing store', got '%v'", err)
	}
}

func assertDocumentsMatch(t *testing.T, sourceDoc, targetDoc *Document) {
	sourceContent, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get source content: %v", err)
	}

	targetContent, err := targetDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get target content: %v", err)
	}

	if string(sourceContent) != string(targetContent) {
		t.Errorf("content mismatch: source '%s' != target '%s'", string(sourceContent), string(targetContent))
	}

	if sourceDoc.Filename != targetDoc.Filename {
		t.Errorf("filename mismatch: source '%s' != target '%s'", sourceDoc.Filename, targetDoc.Filename)
	}

	if sourceDoc.FileSize != targetDoc.FileSize {
		t.Errorf("file size mismatch: source %d != target %d", sourceDoc.FileSize, targetDoc.FileSize)
	}
}

func assertBinaryDocumentsMatch(t *testing.T, sourceDoc, targetDoc *Document) {
	sourceContent, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get source content: %v", err)
	}

	targetContent, err := targetDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get target content: %v", err)
	}

	if len(sourceContent) != len(targetContent) {
		t.Fatalf("content length mismatch: source %d != target %d", len(sourceContent), len(targetContent))
	}

	for i := range sourceContent {
		if sourceContent[i] != targetContent[i] {
			t.Errorf("byte mismatch at index %d: source 0x%02X != target 0x%02X", i, sourceContent[i], targetContent[i])
		}
	}
}

func TestCopyDocumentBetweenStores_CopyFromMemoryToLocalStore(t *testing.T) {
	ctx := context.Background()
	targetStoreName := "local_target"
	targetStoreID := setupLocalStore(t, targetStoreName)

	content := "memory to local content"
	sourceDoc := createDocument(t, ctx, getMemoryStoreID(), "memory_source.txt", content)

	sourceContent, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to read source content: %v", err)
	}
	if string(sourceContent) != content {
		t.Fatalf("source content mismatch during setup")
	}

	reader, err := sourceDoc.Reader()
	if err != nil {
		t.Fatalf("failed to get source reader: %v", err)
	}
	defer reader.Close()

	targetDoc, err := Create(ctx, targetStoreID, sourceDoc.Filename, reader)
	if err != nil {
		t.Fatalf("failed to create document in target store: %v", err)
	}

	assertDocumentsMatch(t, sourceDoc, targetDoc)

	targetOpened, err := Open(ctx, targetStoreID, targetDoc.ID())
	if err != nil {
		t.Fatalf("failed to reopen document from target store: %v", err)
	}

	assertDocumentsMatch(t, sourceDoc, targetOpened)

	sourceContent2, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to reread source content: %v", err)
	}

	targetContent2, err := targetOpened.Bytes()
	if err != nil {
		t.Fatalf("failed to read target content: %v", err)
	}

	if string(sourceContent2) != string(targetContent2) {
		t.Errorf("content mismatch after reopen: source '%s' != target '%s'", string(sourceContent2), string(targetContent2))
	}
}

func TestCopyDocumentBetweenStores_CopyFromLocalToMemoryStore(t *testing.T) {
	ctx := context.Background()
	sourceStoreName := "local_source"
	sourceStoreID := setupLocalStore(t, sourceStoreName)

	content := "local to memory content"
	sourceDoc := createDocument(t, ctx, sourceStoreID, "local_source.txt", content)

	sourceContent, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to read source content: %v", err)
	}
	if string(sourceContent) != content {
		t.Fatalf("source content mismatch during setup")
	}

	reader, err := sourceDoc.Reader()
	if err != nil {
		t.Fatalf("failed to get source reader: %v", err)
	}
	defer reader.Close()

	targetDoc, err := Create(ctx, getMemoryStoreID(), sourceDoc.Filename, reader)
	if err != nil {
		t.Fatalf("failed to create document in target store: %v", err)
	}

	sourceContent2, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get source content: %v", err)
	}

	targetContent2, err := targetDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to get target content: %v", err)
	}

	if string(sourceContent2) != string(targetContent2) {
		t.Errorf("content mismatch: source '%s' != target '%s'", string(sourceContent2), string(targetContent2))
	}
}

func TestCopyDocumentBetweenStores_CopyDocumentWithBinaryContentBetweenStores(t *testing.T) {
	ctx := context.Background()
	targetStoreName := "local_binary"
	targetStoreID := setupLocalStore(t, targetStoreName)

	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x7F, 0x80}
	sourceDoc := createDocument(t, ctx, getMemoryStoreID(), "binary.bin", string(binaryContent))

	sourceContent, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to read source content: %v", err)
	}
	if string(sourceContent) != string(binaryContent) {
		t.Fatalf("source content mismatch during setup")
	}

	reader, err := sourceDoc.Reader()
	if err != nil {
		t.Fatalf("failed to get source reader: %v", err)
	}
	defer reader.Close()

	targetDoc, err := Create(ctx, targetStoreID, sourceDoc.Filename, reader)
	if err != nil {
		t.Fatalf("failed to create document in target store: %v", err)
	}

	assertBinaryDocumentsMatch(t, sourceDoc, targetDoc)

	targetOpened, err := Open(ctx, targetStoreID, targetDoc.ID())
	if err != nil {
		t.Fatalf("failed to reopen document from target store: %v", err)
	}

	assertBinaryDocumentsMatch(t, sourceDoc, targetOpened)

	sourceContent2, err := sourceDoc.Bytes()
	if err != nil {
		t.Fatalf("failed to reread source content: %v", err)
	}

	targetContent2, err := targetOpened.Bytes()
	if err != nil {
		t.Fatalf("failed to read target content: %v", err)
	}

	if string(sourceContent2) != string(targetContent2) {
		t.Errorf("content mismatch after reopen: source '%s' != target '%s'", string(sourceContent2), string(targetContent2))
	}
}
