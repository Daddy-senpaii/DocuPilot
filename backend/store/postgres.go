package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStore implements Store backed by PostgreSQL and pgvector.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore and initializes tables.
func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

func (p *PostgresStore) migrate() error {
	// Attempt enabling pgvector extension (if installed in Postgres)
	_, _ = p.db.Exec(`CREATE EXTENSION IF NOT EXISTS vector;`)

	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id BIGSERIAL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			char_count INT NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'ready',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS document_chunks (
			id BIGSERIAL PRIMARY KEY,
			document_id BIGINT REFERENCES documents(id) ON DELETE CASCADE,
			chunk_index INT NOT NULL,
			content TEXT NOT NULL,
			embedding TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS qa_history (
			id BIGSERIAL PRIMARY KEY,
			document_id BIGINT REFERENCES documents(id) ON DELETE CASCADE,
			question TEXT NOT NULL,
			answer TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
	}

	for _, q := range queries {
		if _, err := p.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresStore) SaveDocument(ctx context.Context, doc *Document) error {
	query := `INSERT INTO documents (filename, content, char_count, status, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id;`
	if doc.Status == "" {
		doc.Status = "ready"
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	return p.db.QueryRowContext(ctx, query, doc.Filename, doc.Content, doc.CharCount, doc.Status, doc.CreatedAt).Scan(&doc.ID)
}

func (p *PostgresStore) GetDocument(ctx context.Context, id int64) (*Document, error) {
	query := `SELECT id, filename, content, char_count, status, created_at FROM documents WHERE id = $1;`
	doc := &Document{}
	err := p.db.QueryRowContext(ctx, query, id).Scan(&doc.ID, &doc.Filename, &doc.Content, &doc.CharCount, &doc.Status, &doc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (p *PostgresStore) ListDocuments(ctx context.Context) ([]*Document, error) {
	query := `SELECT id, filename, content, char_count, status, created_at FROM documents ORDER BY created_at DESC;`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		doc := &Document{}
		if err := rows.Scan(&doc.ID, &doc.Filename, &doc.Content, &doc.CharCount, &doc.Status, &doc.CreatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (p *PostgresStore) DeleteDocument(ctx context.Context, id int64) error {
	res, err := p.db.ExecContext(ctx, `DELETE FROM documents WHERE id = $1;`, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresStore) UpdateDocumentStatus(ctx context.Context, id int64, status string) error {
	res, err := p.db.ExecContext(ctx, `UPDATE documents SET status = $1 WHERE id = $2;`, status, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresStore) SaveChunks(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO document_chunks (document_id, chunk_index, content, embedding) VALUES ($1, $2, $3, $4);`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chk := range chunks {
		embeddingStr := formatEmbedding(chk.Embedding)
		if _, err := stmt.ExecContext(ctx, chk.DocumentID, chk.ChunkIndex, chk.Content, embeddingStr); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *PostgresStore) GetChunksByDocument(ctx context.Context, docID int64) ([]Chunk, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT id, document_id, chunk_index, content FROM document_chunks WHERE document_id = $1 ORDER BY chunk_index ASC;`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var chk Chunk
		if err := rows.Scan(&chk.ID, &chk.DocumentID, &chk.ChunkIndex, &chk.Content); err != nil {
			return nil, err
		}
		chunks = append(chunks, chk)
	}
	return chunks, nil
}

func (p *PostgresStore) SearchSimilarChunks(ctx context.Context, docID int64, queryEmbedding []float32, topK int) ([]ScoredChunk, error) {
	// Query chunks and rank via cosine similarity
	rows, err := p.db.QueryContext(ctx, `SELECT id, document_id, chunk_index, content, embedding FROM document_chunks WHERE document_id = $1;`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scored []ScoredChunk
	for rows.Next() {
		var chk Chunk
		var embStr sql.NullString
		if err := rows.Scan(&chk.ID, &chk.DocumentID, &chk.ChunkIndex, &chk.Content, &embStr); err != nil {
			return nil, err
		}
		if embStr.Valid {
			chk.Embedding = parseEmbedding(embStr.String)
		}
		sim := cosineSimilarity(queryEmbedding, chk.Embedding)
		scored = append(scored, ScoredChunk{
			Chunk:      chk,
			Similarity: sim,
		})
	}

	if topK > len(scored) {
		topK = len(scored)
	}

	return scored[:topK], nil
}

func (p *PostgresStore) SaveQA(ctx context.Context, qa *QAHistory) error {
	query := `INSERT INTO qa_history (document_id, question, answer, created_at) VALUES ($1, $2, $3, $4) RETURNING id;`
	if qa.CreatedAt.IsZero() {
		qa.CreatedAt = time.Now()
	}
	return p.db.QueryRowContext(ctx, query, qa.DocumentID, qa.Question, qa.Answer, qa.CreatedAt).Scan(&qa.ID)
}

func (p *PostgresStore) ListQAByDocument(ctx context.Context, docID *int64) ([]QAHistory, error) {
	var rows *sql.Rows
	var err error
	if docID != nil {
		rows, err = p.db.QueryContext(ctx, `SELECT id, document_id, question, answer, created_at FROM qa_history WHERE document_id = $1 ORDER BY created_at DESC;`, *docID)
	} else {
		rows, err = p.db.QueryContext(ctx, `SELECT id, document_id, question, answer, created_at FROM qa_history ORDER BY created_at DESC;`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []QAHistory
	for rows.Next() {
		var qa QAHistory
		if err := rows.Scan(&qa.ID, &qa.DocumentID, &qa.Question, &qa.Answer, &qa.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, qa)
	}
	return list, nil
}

func (p *PostgresStore) Close() error {
	return p.db.Close()
}

func formatEmbedding(emb []float32) string {
	if len(emb) == 0 {
		return ""
	}
	strVals := make([]string, len(emb))
	for i, v := range emb {
		strVals[i] = fmt.Sprintf("%f", v)
	}
	return strings.Join(strVals, ",")
}

func parseEmbedding(s string) []float32 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	res := make([]float32, 0, len(parts))
	for _, p := range parts {
		var v float32
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%f", &v); err == nil {
			res = append(res, v)
		}
	}
	return res
}
