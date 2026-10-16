package store

import "time"

// Document represents an uploaded or stored document.
type Document struct {
	ID        int64     `json:"id"`
	Filename  string    `json:"filename"`
	Content   string    `json:"content"`
	CharCount int       `json:"char_count"`
	Status    string    `json:"status"` // "ready", "processing", "failed"
	CreatedAt time.Time `json:"created_at"`
}

// Chunk represents a segmented passage of a document, optionally with vector embedding.
type Chunk struct {
	ID         int64     `json:"id"`
	DocumentID int64     `json:"document_id"`
	ChunkIndex int       `json:"chunk_index"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding,omitempty"`
}

// ScoredChunk is a passage retrieved through semantic similarity search.
type ScoredChunk struct {
	Chunk
	Similarity float64 `json:"similarity"`
}

// QAHistory represents a past question and answer entry.
type QAHistory struct {
	ID         int64     `json:"id"`
	DocumentID *int64    `json:"document_id,omitempty"`
	Question   string    `json:"question"`
	Answer     string    `json:"answer"`
	CreatedAt  time.Time `json:"created_at"`
}
