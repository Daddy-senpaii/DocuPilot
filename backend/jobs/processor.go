package jobs

import (
	"context"
	"log"
	"sync"
	"time"

	"docupilot/backend/ai"
	"docupilot/backend/semantic"
	"docupilot/backend/store"
)

// DocumentProcessor manages asynchronous background processing for document chunking and vector indexing.
type DocumentProcessor struct {
	store    store.Store
	embedder ai.Embedder
	queue    chan int64
	wg       sync.WaitGroup
	quit     chan struct{}
}

// NewDocumentProcessor creates and starts the background worker pool.
func NewDocumentProcessor(s store.Store, embedder ai.Embedder, numWorkers int) *DocumentProcessor {
	if numWorkers <= 0 {
		numWorkers = 2
	}

	p := &DocumentProcessor{
		store:    s,
		embedder: embedder,
		queue:    make(chan int64, 100),
		quit:     make(chan struct{}),
	}

	for i := 0; i < numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return p
}

// Enqueue adds a document ID to the background processing queue.
func (p *DocumentProcessor) Enqueue(docID int64) {
	select {
	case p.queue <- docID:
	default:
		log.Printf("[Worker] Queue is full, dropping job for doc %d", docID)
	}
}

// ProcessSync allows synchronous processing when needed (e.g. in unit tests).
func (p *DocumentProcessor) ProcessSync(ctx context.Context, docID int64) error {
	return p.processDocument(ctx, docID)
}

func (p *DocumentProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.quit:
			return
		case docID := <-p.queue:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			if err := p.processDocument(ctx, docID); err != nil {
				log.Printf("[Worker %d] Failed to process document %d: %v", id, docID, err)
			}
			cancel()
		}
	}
}

func (p *DocumentProcessor) processDocument(ctx context.Context, docID int64) error {
	_ = p.store.UpdateDocumentStatus(ctx, docID, "processing")

	doc, err := p.store.GetDocument(ctx, docID)
	if err != nil {
		_ = p.store.UpdateDocumentStatus(ctx, docID, "failed")
		return err
	}

	chunkTexts := semantic.ChunkText(doc.Content, semantic.DefaultChunkSize, semantic.DefaultOverlap)
	if len(chunkTexts) == 0 {
		_ = p.store.UpdateDocumentStatus(ctx, docID, "ready")
		return nil
	}

	embeddings, err := p.embedder.EmbedBatch(ctx, chunkTexts)
	if err != nil {
		_ = p.store.UpdateDocumentStatus(ctx, docID, "failed")
		return err
	}

	chunks := make([]store.Chunk, len(chunkTexts))
	for i, text := range chunkTexts {
		chunks[i] = store.Chunk{
			DocumentID: docID,
			ChunkIndex: i,
			Content:    text,
			Embedding:  embeddings[i],
		}
	}

	if err := p.store.SaveChunks(ctx, chunks); err != nil {
		_ = p.store.UpdateDocumentStatus(ctx, docID, "failed")
		return err
	}

	_ = p.store.UpdateDocumentStatus(ctx, docID, "ready")
	log.Printf("[Worker] Successfully indexed document %d (%d chunks created)", docID, len(chunks))
	return nil
}

// Close gracefully stops the worker goroutines.
func (p *DocumentProcessor) Close() {
	close(p.quit)
	p.wg.Wait()
}
