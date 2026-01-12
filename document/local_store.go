package document

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore stores document content as files on the local filesystem.
// It is metadata-agnostic and only handles blob storage/retrieval.
type LocalStore struct {
	dir string
}

var _ Store = &LocalStore{}

// NewLocalStore creates a new LocalStore
func NewLocalStore(dir string) *LocalStore {
	// remove the trailing slash
	dir = strings.TrimSuffix(dir, string(os.PathSeparator))
	return &LocalStore{dir: dir}
}

// ID returns the unique identifier for this store (the directory name)
func (ls *LocalStore) ID() string {
	return ls.dir
}

// Open returns a ReadCloser for reading document content by ID
func (ls *LocalStore) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	contentPath := filepath.Join(ls.dir, id)
	file, err := os.Open(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// Create stores content from a reader and returns the assigned ID
func (ls *LocalStore) Create(ctx context.Context, filename string, reader io.Reader) (string, error) {
	if err := os.MkdirAll(ls.dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Use filename as ID, sanitize it
	id := filepath.Base(filename)
	if id == "" || id == "." || id == "/" {
		return "", fmt.Errorf("invalid filename for ID: %s", filename)
	}

	contentPath := filepath.Join(ls.dir, id)
	file, err := os.Create(contentPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(contentPath)
		return "", fmt.Errorf("failed to write content: %w", err)
	}

	return id, nil
}

// Save updates existing content by ID
func (ls *LocalStore) Save(ctx context.Context, id string, reader io.Reader) error {
	contentPath := filepath.Join(ls.dir, id)
	file, err := os.Create(contentPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// Delete removes content by ID
func (ls *LocalStore) Delete(ctx context.Context, id string) error {
	contentPath := filepath.Join(ls.dir, id)
	if err := os.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete content: %w", err)
	}
	return nil
}

// List returns all document IDs in the store
func (ls *LocalStore) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(ls.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}

	return ids, nil
}
