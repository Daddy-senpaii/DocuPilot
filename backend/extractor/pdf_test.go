package extractor

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidatePDFHeader(t *testing.T) {
	tests := []struct {
		name    string
		header  []byte
		wantErr bool
	}{
		{
			name:    "Valid PDF 1.4 header",
			header:  []byte("%PDF-1.4\n%..."),
			wantErr: false,
		},
		{
			name:    "Valid PDF 1.7 header",
			header:  []byte("%PDF-1.7"),
			wantErr: false,
		},
		{
			name:    "Invalid header - Plain text",
			header:  []byte("Hello World"),
			wantErr: true,
		},
		{
			name:    "Invalid header - Short bytes",
			header:  []byte("%PD"),
			wantErr: true,
		},
		{
			name:    "Invalid header - PNG magic bytes",
			header:  []byte("\x89PNG\r\n\x1a\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePDFHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePDFHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractTextEmptyFile(t *testing.T) {
	ext := NewPDFExtractor()
	reader := bytes.NewReader([]byte{})
	_, err := ext.ExtractText(reader, 0)
	if err == nil {
		t.Fatalf("expected error for 0 size file, got nil")
	}
	if !strings.Contains(err.Error(), "cannot extract text from an empty file") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExtractTextInvalidPDF(t *testing.T) {
	ext := NewPDFExtractor()
	data := []byte("Not a real PDF stream")
	reader := bytes.NewReader(data)
	_, err := ext.ExtractText(reader, int64(len(data)))
	if err == nil {
		t.Fatalf("expected error for corrupted PDF, got nil")
	}
}
