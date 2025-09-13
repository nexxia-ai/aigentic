package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryEntry(t *testing.T) {
	entry := NewMemoryEntry("test content", "test", []string{"tag1", "tag2"}, map[string]string{"key": "value"}, 5)

	if entry.ID == "" {
		t.Error("Memory entry ID should not be empty")
	}
	if entry.Content != "test content" {
		t.Error("Memory entry content should match")
	}
	if entry.Category != "test" {
		t.Error("Memory entry category should match")
	}
	if len(entry.Tags) != 2 {
		t.Error("Memory entry should have 2 tags")
	}
	if entry.Priority != 5 {
		t.Error("Memory entry priority should be 5")
	}

	// Test validation
	if err := entry.Validate(); err != nil {
		t.Errorf("Valid entry should not fail validation: %v", err)
	}

	// Test invalid entry
	invalidEntry := &MemoryEntry{ID: "", Content: "test"}
	if err := invalidEntry.Validate(); err == nil {
		t.Error("Invalid entry should fail validation")
	}

	// Test tag functions
	if !entry.HasTag("tag1") {
		t.Error("Entry should have tag1")
	}
	if entry.HasTag("nonexistent") {
		t.Error("Entry should not have nonexistent tag")
	}
	if !entry.HasAnyTag([]string{"tag1", "other"}) {
		t.Error("Entry should have any of the specified tags")
	}

	// Test category matching
	if !entry.MatchesCategory("test") {
		t.Error("Entry should match category 'test'")
	}
	if !entry.MatchesCategory("") {
		t.Error("Entry should match empty category")
	}
	if entry.MatchesCategory("other") {
		t.Error("Entry should not match category 'other'")
	}

	// Test size calculation
	size := entry.GetSize()
	if size <= 0 {
		t.Error("Entry size should be positive")
	}

	// Test update functions
	originalUpdatedAt := entry.UpdatedAt
	time.Sleep(time.Millisecond)
	entry.UpdateContent("new content")
	if entry.Content != "new content" {
		t.Error("Content should be updated")
	}
	if !entry.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}

	originalAccessCount := entry.AccessCount
	entry.UpdateAccess()
	if entry.AccessCount != originalAccessCount+1 {
		t.Error("Access count should be incremented")
	}
}

func TestFileStore(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	store := NewFileStore(config)

	// Test saving memory entry
	entry := NewMemoryEntry("test content", "test", []string{"tag1"}, nil, 5)
	err := store.Save(RunMemory, entry)
	if err != nil {
		t.Errorf("Failed to save memory entry: %v", err)
	}

	// Test retrieving memory entry
	retrieved, err := store.Get(RunMemory, entry.ID)
	if err != nil {
		t.Errorf("Failed to retrieve memory entry: %v", err)
	}
	if retrieved.Content != entry.Content {
		t.Error("Retrieved content should match saved content")
	}

	// Test getting all entries
	allEntries, err := store.GetAll(RunMemory)
	if err != nil {
		t.Errorf("Failed to get all entries: %v", err)
	}
	if len(allEntries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(allEntries))
	}

	// Test searching entries
	searchResults, err := store.Search(RunMemory, "test", []string{"tag1"})
	if err != nil {
		t.Errorf("Failed to search entries: %v", err)
	}
	if len(searchResults) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(searchResults))
	}

	// Test deleting entry
	err = store.Delete(RunMemory, entry.ID)
	if err != nil {
		t.Errorf("Failed to delete entry: %v", err)
	}

	// Verify entry is deleted
	_, err = store.Get(RunMemory, entry.ID)
	if err == nil {
		t.Error("Deleted entry should not be retrievable")
	}

	// Test clearing entries
	entry1 := NewMemoryEntry("content1", "cat1", []string{"tag1"}, nil, 5)
	entry2 := NewMemoryEntry("content2", "cat2", []string{"tag2"}, nil, 5)
	store.Save(RunMemory, entry1)
	store.Save(RunMemory, entry2)

	err = store.Clear(RunMemory, "cat1", nil)
	if err != nil {
		t.Errorf("Failed to clear entries: %v", err)
	}

	allEntries, _ = store.GetAll(RunMemory)
	if len(allEntries) != 1 {
		t.Errorf("Expected 1 entry after clearing, got %d", len(allEntries))
	}
	if allEntries[0].ID != entry2.ID {
		t.Error("Wrong entry remaining after clear")
	}

	// Test size limits
	largeContent := string(make([]byte, 2000)) // 2KB content
	largeEntry := NewMemoryEntry(largeContent, "large", nil, nil, 5)
	err = store.Save(RunMemory, largeEntry)
	if err == nil {
		t.Error("Should fail when exceeding size limit")
	}
}

func TestMemoryToolExecutor(t *testing.T) {
	tempDir := t.TempDir()
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	store := NewFileStore(config)
	executor := NewMemoryToolExecutor(store)

	// Test save_memory
	saveParams := SaveMemoryParams{
		Compartment: "run",
		Content:     "test content",
		Category:    "test",
		Tags:        []string{"tag1"},
		Priority:    5,
	}

	result, err := executor.executeSaveMemory(saveParams)
	if err != nil {
		t.Errorf("Failed to execute save_memory: %v", err)
	}
	if result.Error {
		t.Error("Save memory should not return error")
	}

	// Test get_memory
	getParams := GetMemoryParams{
		Compartment: "run",
		Category:    "test",
	}

	result, err = executor.executeGetMemory(getParams)
	if err != nil {
		t.Errorf("Failed to execute get_memory: %v", err)
	}
	if result.Error {
		t.Error("Get memory should not return error")
	}

	// Test clear_memory
	clearParams := ClearMemoryParams{
		Compartment: "run",
		Category:    "test",
	}

	result, err = executor.executeClearMemory(clearParams)
	if err != nil {
		t.Errorf("Failed to execute clear_memory: %v", err)
	}
	if result.Error {
		t.Error("Clear memory should not return error")
	}

	// Test invalid compartment
	invalidParams := SaveMemoryParams{
		Compartment: "invalid",
		Content:     "test",
	}

	result, err = executor.executeSaveMemory(invalidParams)
	if err != nil {
		t.Errorf("Failed to execute save_memory with invalid compartment: %v", err)
	}
	if !result.Error {
		t.Error("Invalid compartment should return error")
	}

	// Test empty content
	emptyParams := SaveMemoryParams{
		Compartment: "run",
		Content:     "",
	}

	result, err = executor.executeSaveMemory(emptyParams)
	if err != nil {
		t.Errorf("Failed to execute save_memory with empty content: %v", err)
	}
	if !result.Error {
		t.Error("Empty content should return error")
	}
}

func TestMemoryPersistence(t *testing.T) {
	tempDir := t.TempDir()
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	// Create first store and save data
	store1 := NewFileStore(config)
	entry := NewMemoryEntry("persistent content", "test", []string{"persistent"}, nil, 5)
	err := store1.Save(SessionMemory, entry)
	if err != nil {
		t.Errorf("Failed to save entry: %v", err)
	}

	// Create second store and verify data persists
	store2 := NewFileStore(config)
	retrieved, err := store2.Get(SessionMemory, entry.ID)
	if err != nil {
		t.Errorf("Failed to retrieve persisted entry: %v", err)
	}
	if retrieved.Content != entry.Content {
		t.Error("Persisted content should match")
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []*MemoryEntry{
		NewMemoryEntry("content1", "cat1", []string{"tag1", "tag2"}, nil, 5),
		NewMemoryEntry("content2", "cat2", []string{"tag2", "tag3"}, nil, 5),
		NewMemoryEntry("content3", "cat1", []string{"tag3"}, nil, 5),
	}

	// Test filtering by category
	filtered := FilterEntries(entries, "cat1", nil)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 entries with category cat1, got %d", len(filtered))
	}

	// Test filtering by tags
	filtered = FilterEntries(entries, "", []string{"tag1"})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 entry with tag1, got %d", len(filtered))
	}

	// Test filtering by category and tags
	filtered = FilterEntries(entries, "cat1", []string{"tag2"})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 entry with category cat1 and tag2, got %d", len(filtered))
	}

	// Test filtering with empty criteria
	filtered = FilterEntries(entries, "", nil)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 entries with no filter, got %d", len(filtered))
	}
}

func TestSearchEntries(t *testing.T) {
	entries := []*MemoryEntry{
		NewMemoryEntry("hello world", "test", []string{"tag1"}, nil, 5),
		NewMemoryEntry("goodbye world", "other", []string{"tag2"}, nil, 5),
		NewMemoryEntry("hello there", "test", []string{"tag1"}, nil, 5),
	}

	// Test search by content
	results := SearchEntries(entries, "hello")
	if len(results) != 2 {
		t.Errorf("Expected 2 entries containing 'hello', got %d", len(results))
	}

	// Test search by category
	results = SearchEntries(entries, "test")
	if len(results) != 2 {
		t.Errorf("Expected 2 entries with category 'test', got %d", len(results))
	}

	// Test case insensitive search
	results = SearchEntries(entries, "HELLO")
	if len(results) != 2 {
		t.Errorf("Expected 2 entries containing 'HELLO' (case insensitive), got %d", len(results))
	}

	// Test empty query
	results = SearchEntries(entries, "")
	if len(results) != 3 {
		t.Errorf("Expected 3 entries with empty query, got %d", len(results))
	}

	// Test no matches
	results = SearchEntries(entries, "nonexistent")
	if len(results) != 0 {
		t.Errorf("Expected 0 entries for nonexistent query, got %d", len(results))
	}
}

func TestMemoryCompartmentIsolation(t *testing.T) {
	tempDir := t.TempDir()
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	store := NewFileStore(config)

	// Save entries to different compartments
	runEntry := NewMemoryEntry("run content", "run", nil, nil, 5)
	sessionEntry := NewMemoryEntry("session content", "session", nil, nil, 5)
	planEntry := NewMemoryEntry("plan content", "plan", nil, nil, 5)

	store.Save(RunMemory, runEntry)
	store.Save(SessionMemory, sessionEntry)
	store.Save(PlanMemory, planEntry)

	// Verify compartment isolation
	runEntries, _ := store.GetAll(RunMemory)
	if len(runEntries) != 1 || runEntries[0].ID != runEntry.ID {
		t.Error("Run memory should only contain run entry")
	}

	sessionEntries, _ := store.GetAll(SessionMemory)
	if len(sessionEntries) != 1 || sessionEntries[0].ID != sessionEntry.ID {
		t.Error("Session memory should only contain session entry")
	}

	planEntries, _ := store.GetAll(PlanMemory)
	if len(planEntries) != 1 || planEntries[0].ID != planEntry.ID {
		t.Error("Plan memory should only contain plan entry")
	}

	// Test clearing specific compartment
	store.Clear(RunMemory, "", nil)
	runEntries, _ = store.GetAll(RunMemory)
	if len(runEntries) != 0 {
		t.Error("Run memory should be empty after clearing")
	}

	// Verify other compartments are unaffected
	sessionEntries, _ = store.GetAll(SessionMemory)
	if len(sessionEntries) != 1 {
		t.Error("Session memory should be unaffected by run memory clear")
	}

	planEntries, _ = store.GetAll(PlanMemory)
	if len(planEntries) != 1 {
		t.Error("Plan memory should be unaffected by run memory clear")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	store := NewFileStore(config)

	// Test concurrent saves
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			entry := NewMemoryEntry("content", "test", nil, nil, 5)
			store.Save(RunMemory, entry)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all entries were saved
	entries, err := store.GetAll(RunMemory)
	if err != nil {
		t.Errorf("Failed to get all entries: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("Expected 10 entries, got %d", len(entries))
	}
}

func TestFileStoreErrorHandling(t *testing.T) {
	// Test with invalid file path
	config := &MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           "/invalid/path/memory.json",
	}

	store := NewFileStore(config)

	// Should still work for operations that don't require file access
	entry := NewMemoryEntry("test", "test", nil, nil, 5)
	err := store.Save(RunMemory, entry)
	if err != nil {
		t.Errorf("Save should work even with invalid path: %v", err)
	}

	// Test with corrupted file
	tempDir := t.TempDir()
	corruptedFile := filepath.Join(tempDir, "corrupted.json")
	os.WriteFile(corruptedFile, []byte("invalid json"), 0644)

	config.StoragePath = corruptedFile
	store = NewFileStore(config)

	// Should handle corrupted file gracefully
	entry = NewMemoryEntry("test", "test", nil, nil, 5)
	err = store.Save(RunMemory, entry)
	if err != nil {
		t.Errorf("Should handle corrupted file gracefully: %v", err)
	}
}
