package document

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

var (
	metadataStore Store = NewInMemoryStore()
)

// getMetadataStore returns the metadata store, initializing it if necessary
func getMetadataStore() Store {
	return metadataStore
}

// metadataKey generates a metadata storage key for a document
func metadataKey(storeName, docID string) string {
	return filepath.Join("meta", storeName, docID+".json")
}

// saveMetadata persists document metadata to the metadata store
func saveMetadata(ctx context.Context, storeName string, doc *Document) error {
	key := metadataKey(storeName, doc.ID())
	store := getMetadataStore()

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	createReader := func() io.Reader {
		return strings.NewReader(string(docJSON))
	}

	if _, err := store.Create(ctx, key, createReader()); err != nil {
		// Try to save if it already exists
		if saveErr := store.Save(ctx, key, createReader()); saveErr != nil {
			return fmt.Errorf("failed to save metadata: %w", saveErr)
		}
	}

	return nil
}

// loadMetadata retrieves document metadata from the metadata store
func loadMetadata(ctx context.Context, storeName, docID string) (*Document, error) {
	key := metadataKey(storeName, docID)
	store := getMetadataStore()

	reader, err := store.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &doc, nil
}

// deleteMetadata removes document metadata from the metadata store
func deleteMetadata(ctx context.Context, storeName, docID string) error {
	key := metadataKey(storeName, docID)
	store := getMetadataStore()
	return store.Delete(ctx, key)
}
