package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewMemoryEntry creates a new memory entry with generated ID and timestamps
func NewMemoryEntry(content, category string, tags []string, metadata map[string]string, priority int) *MemoryEntry {
	now := time.Now()
	return &MemoryEntry{
		ID:          uuid.New().String(),
		Content:     content,
		Category:    category,
		Tags:        tags,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessCount: 0,
		Priority:    priority,
	}
}

// UpdateContent updates the content and sets the updated timestamp
func (e *MemoryEntry) UpdateContent(content string) {
	e.Content = content
	e.UpdatedAt = time.Now()
}

// IncrementAccess increments the access count
func (e *MemoryEntry) UpdateAccess() {
	e.AccessCount++
}

// HasTag checks if the entry has a specific tag
func (e *MemoryEntry) HasTag(tag string) bool {
	for _, t := range e.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasAnyTag checks if the entry has any of the specified tags
func (e *MemoryEntry) HasAnyTag(tags []string) bool {
	for _, tag := range tags {
		if e.HasTag(tag) {
			return true
		}
	}
	return false
}

// MatchesCategory checks if the entry matches the category (empty category matches all)
func (e *MemoryEntry) MatchesCategory(category string) bool {
	if category == "" {
		return true
	}
	return e.Category == category
}

// Validate validates the memory entry
func (e *MemoryEntry) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("memory entry ID cannot be empty")
	}
	if e.Content == "" {
		return fmt.Errorf("memory entry content cannot be empty")
	}
	if e.Priority < 1 || e.Priority > 10 {
		return fmt.Errorf("memory entry priority must be between 1 and 10")
	}
	return nil
}

// GetSize returns the approximate size of the memory entry in characters
func (e *MemoryEntry) GetSize() int {
	size := len(e.Content) + len(e.Category)
	for _, tag := range e.Tags {
		size += len(tag)
	}
	for k, v := range e.Metadata {
		size += len(k) + len(v)
	}
	return size
}

// FilterEntries filters entries based on category and tags
func FilterEntries(entries []*MemoryEntry, category string, tags []string) []*MemoryEntry {
	var filtered []*MemoryEntry
	for _, entry := range entries {
		if entry.MatchesCategory(category) {
			if len(tags) == 0 || entry.HasAnyTag(tags) {
				filtered = append(filtered, entry)
			}
		}
	}
	return filtered
}

// SearchEntries performs text search on memory entries
func SearchEntries(entries []*MemoryEntry, query string) []*MemoryEntry {
	if query == "" {
		return entries
	}

	query = strings.ToLower(query)
	var results []*MemoryEntry
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Content), query) ||
			strings.Contains(strings.ToLower(entry.Category), query) {
			results = append(results, entry)
		}
	}
	return results
}
