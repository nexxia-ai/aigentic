package document

import (
	"fmt"
	"sync"
)

var (
	storeRegistry = make(map[string]Store)
	registryMu    sync.RWMutex
)

func init() {
	RegisterStore(defaultMemoryStore)
}

// RegisterStore registers a store using its ID() as the unique identifier.
// It returns an error if a store with the same ID already exists.
func RegisterStore(s Store) error {
	if s == nil {
		return fmt.Errorf("store cannot be nil")
	}

	id := s.ID()
	if id == "" {
		return fmt.Errorf("store ID cannot be empty")
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := storeRegistry[id]; exists {
		return fmt.Errorf("store with ID %s already registered", id)
	}

	storeRegistry[id] = s
	return nil
}

// GetStore returns the store registered with the given ID.
// It returns false if the store doesn't exist.
func GetStore(id string) (Store, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	store, exists := storeRegistry[id]
	return store, exists
}

// ListStores returns all registered store IDs
func ListStores() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	ids := make([]string, 0, len(storeRegistry))
	for id := range storeRegistry {
		ids = append(ids, id)
	}
	return ids
}

// UnregisterStore removes a store from the registry by ID.
func UnregisterStore(id string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(storeRegistry, id)
}
