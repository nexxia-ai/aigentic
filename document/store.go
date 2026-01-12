package document

import (
	"context"
	"io"
)

// Store provides read/write access to document content as raw bytes.
// Stores are metadata-agnostic and only handle blob storage/retrieval.
type Store interface {
	// ID returns the unique identifier for this store instance
	ID() string

	// Open returns a ReadCloser for reading document content by ID
	Open(ctx context.Context, id string) (io.ReadCloser, error)

	// Create stores content from a reader and returns the assigned ID
	Create(ctx context.Context, filename string, reader io.Reader) (id string, err error)

	// Save updates existing content by ID
	Save(ctx context.Context, id string, reader io.Reader) error

	// Delete removes content by ID
	Delete(ctx context.Context, id string) error

	// List returns all document IDs in the store
	List(ctx context.Context) ([]string, error)
}
