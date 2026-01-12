package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
)

// InMemoryStore stores document content in memory.
// It is metadata-agnostic and only handles blob storage/retrieval.
type InMemoryStore struct {
	id      string
	content map[string][]byte
	mu      sync.RWMutex
}

var defaultMemoryStore = NewInMemoryStore()

// NewInMemoryStore creates a new InMemoryStore
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		id:      uuid.New().String(),
		content: make(map[string][]byte),
	}
}

var _ Store = &InMemoryStore{}

// ID returns the unique identifier for this store (a UUID)
func (s *InMemoryStore) ID() string {
	return s.id
}

// Open returns a ReadCloser for reading document content by ID
func (s *InMemoryStore) Open(ctx context.Context, id string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.content[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

// Create stores content from a reader and returns the assigned ID
func (s *InMemoryStore) Create(ctx context.Context, filename string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read from reader: %w", err)
	}

	id := filename
	if id == "" {
		id = fmt.Sprintf("doc_%d", len(s.content))
	}

	s.mu.Lock()
	s.content[id] = data
	s.mu.Unlock()

	return id, nil
}

// Save updates existing content by ID
func (s *InMemoryStore) Save(ctx context.Context, id string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read from reader: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.content[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	s.content[id] = data
	return nil
}

// Delete removes content by ID
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.content[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	delete(s.content, id)
	return nil
}

// List returns all document IDs in the store
func (s *InMemoryStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.content))
	for id := range s.content {
		ids = append(ids, id)
	}

	return ids, nil
}
