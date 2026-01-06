package document

// Stage wraps a processor with an optional backing store
type Stage struct {
	Name      string
	Processor DocumentProcessor
	Store     Store
}

