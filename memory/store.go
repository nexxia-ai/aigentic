package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore implements MemoryStore using JSON file storage
type FileStore struct {
	config   *MemoryConfig
	data     map[MemoryCompartment][]*MemoryEntry
	mutex    sync.RWMutex
	filePath string
}

// NewFileStore creates a new file-based memory store
func NewFileStore(config *MemoryConfig) *FileStore {
	if config == nil {
		config = DefaultMemoryConfig()
	}

	store := &FileStore{
		config:   config,
		data:     make(map[MemoryCompartment][]*MemoryEntry),
		filePath: config.StoragePath,
	}

	// Load existing data
	store.load()
	return store
}

// load reads data from the JSON file
func (s *FileStore) load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Initialize compartments
	s.data[RunMemory] = make([]*MemoryEntry, 0)
	s.data[SessionMemory] = make([]*MemoryEntry, 0)
	s.data[PlanMemory] = make([]*MemoryEntry, 0)

	// Try to read existing file
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, start with empty data
		}
		return fmt.Errorf("failed to read memory file: %w", err)
	}

	var fileData map[MemoryCompartment][]*MemoryEntry
	if err := json.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to parse memory file: %w", err)
	}

	// Merge loaded data with initialized compartments
	for compartment, entries := range fileData {
		if _, exists := s.data[compartment]; exists {
			s.data[compartment] = entries
		}
	}

	return nil
}

// save writes data to the JSON file atomically
func (s *FileStore) save() error {
	return s.saveData(s.getDataCopy())
}

// getDataCopy creates a copy of the data without locking
func (s *FileStore) getDataCopy() map[MemoryCompartment][]*MemoryEntry {
	data := make(map[MemoryCompartment][]*MemoryEntry)
	for k, v := range s.data {
		data[k] = make([]*MemoryEntry, len(v))
		copy(data[k], v)
	}
	return data
}

// saveData writes the provided data to the JSON file atomically
func (s *FileStore) saveData(data map[MemoryCompartment][]*MemoryEntry) error {

	// Create directory if it doesn't exist
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Write to temporary file first
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode memory data: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, s.filePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// Save adds or updates a memory entry
func (s *FileStore) Save(compartment MemoryCompartment, entry *MemoryEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid memory entry: %w", err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check size limit
	currentSize := s.getCompartmentSizeUnsafe(compartment)
	if currentSize+entry.GetSize() > s.config.MaxSizePerCompartment {
		return fmt.Errorf("memory compartment size limit exceeded: %d characters", s.config.MaxSizePerCompartment)
	}

	// Find existing entry by ID
	entries := s.data[compartment]
	for i, existing := range entries {
		if existing.ID == entry.ID {
			entries[i] = entry
			return s.saveData(s.getDataCopy())
		}
	}

	// Add new entry
	s.data[compartment] = append(entries, entry)
	return s.saveData(s.getDataCopy())
}

// Get retrieves a memory entry by ID
func (s *FileStore) Get(compartment MemoryCompartment, id string) (*MemoryEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entries := s.data[compartment]
	for _, entry := range entries {
		if entry.ID == id {
			entry.UpdateAccess()
			return entry, nil
		}
	}

	return nil, fmt.Errorf("memory entry not found: %s", id)
}

// GetAll retrieves all memory entries from a compartment
func (s *FileStore) GetAll(compartment MemoryCompartment) ([]*MemoryEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entries := make([]*MemoryEntry, len(s.data[compartment]))
	copy(entries, s.data[compartment])
	return entries, nil
}

// Search retrieves memory entries matching category and tags
func (s *FileStore) Search(compartment MemoryCompartment, category string, tags []string) ([]*MemoryEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	entries := s.data[compartment]
	return FilterEntries(entries, category, tags), nil
}

// Delete removes a memory entry by ID
func (s *FileStore) Delete(compartment MemoryCompartment, id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	entries := s.data[compartment]
	for i, entry := range entries {
		if entry.ID == id {
			s.data[compartment] = append(entries[:i], entries[i+1:]...)
			return s.saveData(s.getDataCopy())
		}
	}

	return fmt.Errorf("memory entry not found: %s", id)
}

// Clear removes memory entries matching category and tags
func (s *FileStore) Clear(compartment MemoryCompartment, category string, tags []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	entries := s.data[compartment]
	var filtered []*MemoryEntry

	for _, entry := range entries {
		// Keep entries that don't match the clear criteria
		shouldKeep := true

		// If category is specified, only keep entries that don't match
		if category != "" && entry.MatchesCategory(category) {
			shouldKeep = false
		}

		// If tags are specified, only keep entries that don't have any of the tags
		if len(tags) > 0 && entry.HasAnyTag(tags) {
			shouldKeep = false
		}

		// If both category and tags are empty, clear all entries
		if category == "" && len(tags) == 0 {
			shouldKeep = false
		}

		if shouldKeep {
			filtered = append(filtered, entry)
		}
	}

	s.data[compartment] = filtered
	return s.saveData(s.getDataCopy())
}

// GetSize returns the current size of a compartment
func (s *FileStore) GetSize(compartment MemoryCompartment) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.getCompartmentSizeUnsafe(compartment)
}

// getCompartmentSizeUnsafe calculates compartment size without locking
func (s *FileStore) getCompartmentSizeUnsafe(compartment MemoryCompartment) int {
	size := 0
	for _, entry := range s.data[compartment] {
		size += entry.GetSize()
	}
	return size
}

// GetMaxSize returns the maximum size per compartment
func (s *FileStore) GetMaxSize() int {
	return s.config.MaxSizePerCompartment
}
