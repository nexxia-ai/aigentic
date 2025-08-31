package document

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore concrete implementation of FileStore
type LocalStore struct {
	dir string
}

var _ DocumentStore = &LocalStore{}

// NewLocalStore creates a new LocalStore
func NewLocalStore(dir string) *LocalStore {
	// remove the trailing slash
	dir = strings.TrimSuffix(dir, string(os.PathSeparator))
	return &LocalStore{dir: dir}
}

// Open implements FileStore interface - just creates Document with metadata
func (ls *LocalStore) Open(ctx context.Context, filePath string) (*Document, error) {
	if !strings.HasPrefix(filePath, ls.dir) {
		filePath = filepath.Join(ls.dir, filePath)
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Determine MIME type and document type
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))

	// Create document with empty content for lazy loading
	doc := NewInMemoryDocument(
		filepath.Base(filePath),
		filepath.Base(filePath),
		nil, // Empty content for lazy loading
		nil,
	)

	// Store relative path for lazy loading
	relativePath := strings.TrimPrefix(filePath, ls.dir+string(os.PathSeparator))
	doc.FilePath = relativePath
	doc.FileSize = stat.Size()
	doc.MimeType = mimeType

	// Set the loader for lazy loading
	doc.SetLoader(ls.load)

	return doc, nil
}

// Close implements FileStore interface
func (ls *LocalStore) Close(ctx context.Context) error {
	// No cleanup needed for local files
	return nil
}

// Add copies a file to the store location
func (ls *LocalStore) Add(ctx context.Context, filePath string) (*Document, error) {
	// For now, just return the document without copying
	// In a full implementation, this would copy the file to basePath
	return ls.Open(ctx, filePath)
}

// Delete removes a file from the store
func (ls *LocalStore) Delete(ctx context.Context, filePath string) error {
	return os.Remove(filePath)
}

// List returns all documents in the store
func (ls *LocalStore) List(ctx context.Context) ([]*Document, error) {
	var documents []*Document

	// Walk the base path
	err := filepath.Walk(ls.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Create document for each file
		doc, err := ls.Open(ctx, path)
		if err != nil {
			return err
		}

		documents = append(documents, doc)
		return nil
	})

	return documents, err
}

// Private method for lazy loading
func (ls *LocalStore) load(d *Document) ([]byte, error) {
	// Read file from filesystem
	if d.FilePath != "" {
		// Construct full path by joining base directory with relative file path
		fullPath := filepath.Join(ls.dir, d.FilePath)
		if data, err := os.ReadFile(fullPath); err == nil {
			return data, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}
