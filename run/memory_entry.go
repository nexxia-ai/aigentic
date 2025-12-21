package run

import "time"

type MemoryEntry struct {
	ID          string
	Description string
	Content     string
	Scope       string
	RunID       string
	Timestamp   time.Time
}
