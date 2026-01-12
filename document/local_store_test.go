package document

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStore_StoreOperations(t *testing.T) {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)
	storeID := store.ID()

	if err := RegisterStore(store); err != nil {
		t.Fatalf("Failed to register store: %v", err)
	}

	// Cleanup: unregister store
	t.Cleanup(func() {
		UnregisterStore(storeID)
	})

	ctx := context.Background()

	t.Run("Create and Open", func(t *testing.T) {
		filename := "test.txt"
		content := "Hello, World!"
		reader := bytes.NewReader([]byte(content))

		docID, err := store.Create(ctx, filename, reader)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		if docID != filename {
			t.Errorf("Expected ID %s, got %s", filename, docID)
		}

		readCloser, err := store.Open(ctx, docID)
		if err != nil {
			t.Fatalf("Failed to open document: %v", err)
		}
		defer readCloser.Close()

		readContent, err := io.ReadAll(readCloser)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}

		if string(readContent) != content {
			t.Errorf("Expected content %s, got %s", content, string(readContent))
		}
	})

	t.Run("Save", func(t *testing.T) {
		docID := "update.txt"
		initialContent := "initial"
		updatedContent := "updated"

		reader1 := bytes.NewReader([]byte(initialContent))
		if _, err := store.Create(ctx, docID, reader1); err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		reader2 := bytes.NewReader([]byte(updatedContent))
		if err := store.Save(ctx, docID, reader2); err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		readCloser, err := store.Open(ctx, docID)
		if err != nil {
			t.Fatalf("Failed to open document: %v", err)
		}
		defer readCloser.Close()

		readContent, err := io.ReadAll(readCloser)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}

		if string(readContent) != updatedContent {
			t.Errorf("Expected content %s, got %s", updatedContent, string(readContent))
		}
	})

	t.Run("List", func(t *testing.T) {
		files := []string{"file1.txt", "file2.txt", "file3.txt"}
		for _, file := range files {
			content := "content for " + file
			reader := bytes.NewReader([]byte(content))
			if _, err := store.Create(ctx, file, reader); err != nil {
				t.Fatalf("Failed to create %s: %v", file, err)
			}
		}

		ids, err := store.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list documents: %v", err)
		}

		if len(ids) < len(files) {
			t.Errorf("Expected at least %d documents, got %d", len(files), len(ids))
		}

		found := make(map[string]bool)
		for _, id := range ids {
			found[id] = true
		}

		for _, file := range files {
			if !found[file] {
				t.Errorf("Expected file %s in list", file)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		docID := "delete_me.txt"
		content := "delete this"
		reader := bytes.NewReader([]byte(content))
		if _, err := store.Create(ctx, docID, reader); err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		if err := store.Delete(ctx, docID); err != nil {
			t.Fatalf("Failed to delete document: %v", err)
		}

		_, err := store.Open(ctx, docID)
		if err == nil {
			t.Error("Expected error when opening deleted document")
		}
	})
}

func TestDocument_FacadeFunctions(t *testing.T) {
	tempDir := t.TempDir()
	store := NewLocalStore(tempDir)
	storeID := store.ID()

	if err := RegisterStore(store); err != nil {
		t.Fatalf("Failed to register store: %v", err)
	}

	ctx := context.Background()

	t.Run("Upload", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "upload_test.txt")
		content := "upload test content"
		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		doc, err := Upload(ctx, storeID, tempFile)
		if err != nil {
			t.Fatalf("Failed to upload document: %v", err)
		}

		if doc.Filename != "upload_test.txt" {
			t.Errorf("Expected filename 'upload_test.txt', got '%s'", doc.Filename)
		}

		docContent, err := doc.Bytes()
		if err != nil {
			t.Fatalf("Failed to get content: %v", err)
		}

		if string(docContent) != content {
			t.Errorf("Expected content %s, got %s", content, string(docContent))
		}
	})

	t.Run("Create", func(t *testing.T) {
		filename := "create_test.txt"
		content := "create test content"
		reader := bytes.NewReader([]byte(content))

		doc, err := Create(ctx, storeID, filename, reader)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		if doc.Filename != filename {
			t.Errorf("Expected filename %s, got %s", filename, doc.Filename)
		}

		docContent, err := doc.Bytes()
		if err != nil {
			t.Fatalf("Failed to get content: %v", err)
		}

		if string(docContent) != content {
			t.Errorf("Expected content %s, got %s", content, string(docContent))
		}
	})

	t.Run("Open", func(t *testing.T) {
		filename := "open_test.txt"
		content := "open test content"
		reader := bytes.NewReader([]byte(content))

		createdDoc, err := Create(ctx, storeID, filename, reader)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		docID := createdDoc.ID()

		openedDoc, err := Open(ctx, storeID, docID)
		if err != nil {
			t.Fatalf("Failed to open document: %v", err)
		}

		if openedDoc.Filename != filename {
			t.Errorf("Expected filename %s, got %s", filename, openedDoc.Filename)
		}

		docContent, err := openedDoc.Bytes()
		if err != nil {
			t.Fatalf("Failed to get content: %v", err)
		}

		if string(docContent) != content {
			t.Errorf("Expected content %s, got %s", content, string(docContent))
		}
	})

	t.Run("List", func(t *testing.T) {
		files := []string{"list1.txt", "list2.txt", "list3.txt"}
		for _, file := range files {
			content := "content for " + file
			reader := bytes.NewReader([]byte(content))
			if _, err := Create(ctx, storeID, file, reader); err != nil {
				t.Fatalf("Failed to create %s: %v", file, err)
			}
		}

		docs, err := List(ctx, storeID)
		if err != nil {
			t.Fatalf("Failed to list documents: %v", err)
		}

		if len(docs) < len(files) {
			t.Errorf("Expected at least %d documents, got %d", len(files), len(docs))
		}

		found := make(map[string]bool)
		for _, doc := range docs {
			found[doc.Filename] = true
		}

		for _, file := range files {
			if !found[file] {
				t.Errorf("Expected file %s in list", file)
			}
		}
	})

	t.Run("Delete", func(t *testing.T) {
		filename := "delete_test.txt"
		content := "delete test content"
		reader := bytes.NewReader([]byte(content))

		doc, err := Create(ctx, storeID, filename, reader)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		docID := doc.ID()

		if err := Delete(ctx, storeID, docID); err != nil {
			t.Fatalf("Failed to delete document: %v", err)
		}

		_, err = Open(ctx, storeID, docID)
		if err == nil {
			t.Error("Expected error when opening deleted document")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		filename := "remove_test.txt"
		content := "remove test content"
		reader := bytes.NewReader([]byte(content))

		doc, err := Create(ctx, storeID, filename, reader)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		docID := doc.ID()

		if err := Remove(ctx, doc); err != nil {
			t.Fatalf("Failed to remove document: %v", err)
		}

		_, err = Open(ctx, storeID, docID)
		if err == nil {
			t.Error("Expected error when opening removed document")
		}
	})
}

func TestDocument_IsChunk(t *testing.T) {
	doc := NewInMemoryDocument(
		"test.txt",
		"test.txt",
		[]byte("test content"),
		nil,
	)
	if doc.IsChunk() {
		t.Error("Main document should not be a chunk")
	}

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

func TestStore_Registry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		store := NewInMemoryStore()
		storeID := store.ID()

		if err := RegisterStore(store); err != nil {
			t.Fatalf("Failed to register store: %v", err)
		}

		retrievedStore, exists := GetStore(storeID)
		if !exists {
			t.Error("Store should exist after registration")
		}

		if retrievedStore != store {
			t.Error("Retrieved store should be the same instance")
		}

		t.Cleanup(func() {
			UnregisterStore(storeID)
		})
	})

	t.Run("ListStores", func(t *testing.T) {
		store1 := NewInMemoryStore()
		store2 := NewInMemoryStore()
		store3 := NewInMemoryStore()
		stores := []Store{store1, store2, store3}
		storeIDs := make([]string, len(stores))

		for i, store := range stores {
			if err := RegisterStore(store); err != nil {
				t.Fatalf("Failed to register store %d: %v", i, err)
			}
			storeIDs[i] = store.ID()
		}

		t.Cleanup(func() {
			for _, id := range storeIDs {
				UnregisterStore(id)
			}
		})

		registeredStores := ListStores()
		found := make(map[string]bool)
		for _, id := range registeredStores {
			found[id] = true
		}

		for _, id := range storeIDs {
			if !found[id] {
				t.Errorf("Store %s should be in list", id)
			}
		}
	})

	t.Run("Register duplicate", func(t *testing.T) {
		store1 := NewLocalStore(t.TempDir())
		store2 := NewLocalStore(store1.ID())

		if err := RegisterStore(store1); err != nil {
			t.Fatalf("Failed to register first store: %v", err)
		}

		err := RegisterStore(store2)
		if err == nil {
			t.Error("Expected error when registering duplicate store ID")
		}

		t.Cleanup(func() {
			UnregisterStore(store1.ID())
		})
	})
}
