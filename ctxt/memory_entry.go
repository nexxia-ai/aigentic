package ctxt

import "time"

type MemoryEntry struct {
	ID          string
	Description string
	Content     string
	RunID       string
	Timestamp   time.Time
}
