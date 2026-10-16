package semantic

import (
	"strings"
	"testing"
)

func TestChunkTextEmpty(t *testing.T) {
	chunks := ChunkText("", 500, 50)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty string, got %d", len(chunks))
	}
}

func TestChunkTextShort(t *testing.T) {
	text := "This is a short text that fits in one chunk."
	chunks := ChunkText(text, 100, 20)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Fatalf("expected chunk to equal text, got %q", chunks[0])
	}
}

func TestChunkTextLongWithOverlap(t *testing.T) {
	text := strings.Repeat("DocuPilot makes learning Go and AI easy. ", 50)
	chunkSize := 200
	overlap := 40
	chunks := ChunkText(text, chunkSize, overlap)

	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	for i, c := range chunks {
		if len([]rune(c)) > chunkSize*2 {
			t.Fatalf("chunk %d exceeded reasonable length: %d", i, len([]rune(c)))
		}
	}
}
