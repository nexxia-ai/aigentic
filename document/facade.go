package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"
)

// Upload uploads a file from the filesystem to the specified store and returns a Document.
func Upload(ctx context.Context, storeName, filePath string) (*Document, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	filename := filepath.Base(filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return Create(ctx, storeName, filename, file)
}

// Create creates a document from a reader in the specified store and returns a Document.
func Create(ctx context.Context, storeName, filename string, reader io.Reader) (*Document, error) {
	store, exists := GetStore(storeName)
	if !exists {
		return nil, fmt.Errorf("store %s not found", storeName)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}

	docID, err := store.Create(ctx, filename, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create document in store: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	doc := &Document{
		id:         docID,
		Filename:   filename,
		FilePath:   filename,
		FileSize:   int64(len(data)),
		MimeType:   mimeType,
		Selected:   false,
		ChunkIndex: -1,
		CreatedAt:  time.Now(),
		store:      store,
	}

	if err := saveMetadata(ctx, storeName, doc); err != nil {
		// Clean up the document from the store if metadata save fails
		store.Delete(ctx, docID)
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return doc, nil
}

// Open opens a document from the specified store by ID and returns a Document.
func Open(ctx context.Context, storeName, id string) (*Document, error) {
	store, exists := GetStore(storeName)
	if !exists {
		return nil, fmt.Errorf("store %s not found", storeName)
	}

	// Try to load metadata first
	doc, err := loadMetadata(ctx, storeName, id)
	if err == nil {
		// Metadata exists, hydrate document from metadata
		doc.store = store

		// If source document ID exists, try to load it
		if doc.SourceDocID != "" {
			sourceDoc, err := Open(ctx, storeName, doc.SourceDocID)
			if err == nil {
				doc.SourceDoc = sourceDoc
			}
		}

		return doc, nil
	}

	// No metadata found, try to load directly from store and infer metadata
	reader, err := store.Open(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to open document: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read document content: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(id))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	doc = &Document{
		id:         id,
		Filename:   filepath.Base(id),
		FilePath:   id,
		FileSize:   int64(len(data)),
		MimeType:   mimeType,
		Selected:   false,
		ChunkIndex: -1,
		CreatedAt:  time.Now(),
		store:      store,
	}

	// Save metadata for future loads
	if err := saveMetadata(ctx, storeName, doc); err != nil {
		// Log but don't fail if metadata save fails
	}

	return doc, nil
}

// List lists all documents in the specified store.
func List(ctx context.Context, storeName string) ([]*Document, error) {
	store, exists := GetStore(storeName)
	if !exists {
		return nil, fmt.Errorf("store %s not found", storeName)
	}

	ids, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	var docs []*Document
	for _, id := range ids {
		doc, err := Open(ctx, storeName, id)
		if err != nil {
			// Skip documents that can't be loaded
			continue
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// Delete deletes a document from the specified store by ID.
func Delete(ctx context.Context, storeName, id string) error {
	store, exists := GetStore(storeName)
	if !exists {
		return fmt.Errorf("store %s not found", storeName)
	}

	// Delete from content store
	if err := store.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete document from store: %w", err)
	}

	// Delete metadata
	if err := deleteMetadata(ctx, storeName, id); err != nil {
		// Log but don't fail if metadata delete fails
	}

	return nil
}

// Remove removes a document using its associated store.
func Remove(ctx context.Context, doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document is nil")
	}

	if doc.store == nil {
		return fmt.Errorf("document has no backing store")
	}

	storeID := doc.store.ID()
	if storeID == "" {
		return fmt.Errorf("document store has no ID")
	}

	docID := doc.ID()

	// Delete from content store
	if err := doc.store.Delete(ctx, docID); err != nil {
		return fmt.Errorf("failed to delete document from store: %w", err)
	}

	// Delete metadata
	if err := deleteMetadata(ctx, storeID, docID); err != nil {
		// Log but don't fail if metadata delete fails
	}

	return nil
}
