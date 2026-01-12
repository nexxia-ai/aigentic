package document

type DocumentProcessor interface {
	Process(doc *Document) ([]*Document, error)
}
