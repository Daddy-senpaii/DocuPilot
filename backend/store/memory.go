package store

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of Store, ideal for local testing and zero-config setup.
type MemoryStore struct {
	mu           sync.RWMutex
	nextDocID    int64
	nextChunkID  int64
	nextQAID     int64
	documents    map[int64]*Document
	chunks       map[int64][]Chunk
	qaHistory    []QAHistory
}

// NewMemoryStore creates a thread-safe in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextDocID:   1,
		nextChunkID: 1,
		nextQAID:    1,
		documents:   make(map[int64]*Document),
		chunks:      make(map[int64][]Chunk),
		qaHistory:   make([]QAHistory, 0),
	}
}

func (m *MemoryStore) SaveDocument(_ context.Context, doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc.ID == 0 {
		doc.ID = m.nextDocID
		m.nextDocID++
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	if doc.Status == "" {
		doc.Status = "ready"
	}

	docCopy := *doc
	m.documents[doc.ID] = &docCopy
	return nil
}

func (m *MemoryStore) GetDocument(_ context.Context, id int64) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.documents[id]
	if !exists {
		return nil, ErrNotFound
	}
	docCopy := *doc
	return &docCopy, nil
}

func (m *MemoryStore) ListDocuments(_ context.Context) ([]*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Document, 0, len(m.documents))
	for _, doc := range m.documents {
		docCopy := *doc
		result = append(result, &docCopy)
	}

	// Sort newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) DeleteDocument(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.documents[id]; !exists {
		return ErrNotFound
	}

	delete(m.documents, id)
	delete(m.chunks, id)

	// Filter out QA history associated with this document
	filteredQA := make([]QAHistory, 0, len(m.qaHistory))
	for _, qa := range m.qaHistory {
		if qa.DocumentID == nil || *qa.DocumentID != id {
			filteredQA = append(filteredQA, qa)
		}
	}
	m.qaHistory = filteredQA

	return nil
}

func (m *MemoryStore) UpdateDocumentStatus(_ context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[id]
	if !exists {
		return ErrNotFound
	}
	doc.Status = status
	return nil
}

func (m *MemoryStore) SaveChunks(_ context.Context, chunks []Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range chunks {
		if chunks[i].ID == 0 {
			chunks[i].ID = m.nextChunkID
			m.nextChunkID++
		}
		docID := chunks[i].DocumentID
		m.chunks[docID] = append(m.chunks[docID], chunks[i])
	}
	return nil
}

func (m *MemoryStore) GetChunksByDocument(_ context.Context, docID int64) ([]Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunksList := m.chunks[docID]
	result := make([]Chunk, len(chunksList))
	copy(result, chunksList)
	return result, nil
}

func (m *MemoryStore) SearchSimilarChunks(_ context.Context, docID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	docChunks := m.chunks[docID]
	if len(docChunks) == 0 {
		return nil, nil
	}

	scored := make([]ScoredChunk, 0, len(docChunks))
	for _, chk := range docChunks {
		sim := cosineSimilarity(queryEmbedding, chk.Embedding)
		scored = append(scored, ScoredChunk{
			Chunk:      chk,
			Similarity: sim,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})

	if topK > len(scored) {
		topK = len(scored)
	}

	return scored[:topK], nil
}

func (m *MemoryStore) SaveQA(_ context.Context, qa *QAHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if qa.ID == 0 {
		qa.ID = m.nextQAID
		m.nextQAID++
	}
	if qa.CreatedAt.IsZero() {
		qa.CreatedAt = time.Now()
	}

	m.qaHistory = append(m.qaHistory, *qa)
	return nil
}

func (m *MemoryStore) ListQAByDocument(_ context.Context, docID *int64) ([]QAHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]QAHistory, 0)
	for _, qa := range m.qaHistory {
		if docID == nil || (qa.DocumentID != nil && *qa.DocumentID == *docID) {
			result = append(result, qa)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) Close() error {
	return nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
