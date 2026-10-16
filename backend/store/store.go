package store

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Store defines the storage operations for DocuPilot documents, chunks, and QA history.
type Store interface {
	SaveDocument(ctx context.Context, doc *Document) error
	GetDocument(ctx context.Context, id int64) (*Document, error)
	ListDocuments(ctx context.Context) ([]*Document, error)
	DeleteDocument(ctx context.Context, id int64) error
	UpdateDocumentStatus(ctx context.Context, id int64, status string) error

	SaveChunks(ctx context.Context, chunks []Chunk) error
	GetChunksByDocument(ctx context.Context, docID int64) ([]Chunk, error)
	SearchSimilarChunks(ctx context.Context, docID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error)

	SaveQA(ctx context.Context, qa *QAHistory) error
	ListQAByDocument(ctx context.Context, docID *int64) ([]QAHistory, error)
	Close() error
}
