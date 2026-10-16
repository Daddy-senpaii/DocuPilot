package semantic

import (
	"strings"
	"unicode"
)

// DefaultChunkSize is the target character size for document segments.
const DefaultChunkSize = 600

// DefaultOverlap is the character overlap between consecutive segments to maintain context continuity.
const DefaultOverlap = 100

// ChunkText splits a long text document into overlapping chunks suitable for semantic embedding.
func ChunkText(text string, chunkSize, overlap int) []string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = DefaultOverlap
	}

	runes := []rune(clean)
	totalRunes := len(runes)

	if totalRunes <= chunkSize {
		return []string{clean}
	}

	var chunks []string
	start := 0

	for start < totalRunes {
		end := start + chunkSize
		if end >= totalRunes {
			chunk := strings.TrimSpace(string(runes[start:]))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		// Try to break at a paragraph or sentence boundary near the target end
		breakPoint := end
		for i := end; i > start+chunkSize/2; i-- {
			if runes[i] == '\n' || runes[i] == '.' || runes[i] == '?' || runes[i] == '!' {
				breakPoint = i + 1
				break
			}
			if unicode.IsSpace(runes[i]) && breakPoint == end {
				breakPoint = i
			}
		}

		chunk := strings.TrimSpace(string(runes[start:breakPoint]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		// Advance with overlap
		advance := breakPoint - overlap
		if advance <= start {
			advance = breakPoint
		}
		start = advance
	}

	return chunks
}
