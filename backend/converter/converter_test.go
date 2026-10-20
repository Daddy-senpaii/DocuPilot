package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupportedPairs(t *testing.T) {
	c := NewLibreOfficeConverter("", nil)
	pairs := c.SupportedPairs()

	expectedCount := 8
	if len(pairs) != expectedCount {
		t.Fatalf("expected %d supported pairs, got %d", expectedCount, len(pairs))
	}

	expectedPairs := map[string][]string{
		FormatPDF:  {FormatDOCX, FormatXLSX, FormatPPTX},
		FormatDOCX: {FormatPDF, FormatTXT},
		FormatXLSX: {FormatPDF},
		FormatPPTX: {FormatPDF},
		FormatTXT:  {FormatPDF},
	}

	for src, targets := range expectedPairs {
		for _, tgt := range targets {
			if !c.IsSupported(src, tgt) {
				t.Errorf("expected pair %s -> %s to be supported", src, tgt)
			}
			// Test with leading dots and mixed case
			if !c.IsSupported("."+strings.ToUpper(src), "."+tgt) {
				t.Errorf("expected sanitized pair .%s -> .%s to be supported", strings.ToUpper(src), tgt)
			}
		}
	}

	// Verify unsupported pair
	if c.IsSupported("jpg", "png") {
		t.Error("expected unsupported pair jpg -> png to return false")
	}
	if c.IsSupported("xlsx", "docx") {
		t.Error("expected unsupported pair xlsx -> docx to return false")
	}
}

func TestMimeTypes(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"pdf", "application/pdf"},
		{".PDF", "application/pdf"},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{"txt", "text/plain; charset=utf-8"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		actual := GetMimeType(tt.format)
		if actual != tt.expected {
			t.Errorf("GetMimeType(%q) = %q, expected %q", tt.format, actual, tt.expected)
		}
	}
}

func TestConvertValidationErrors(t *testing.T) {
	c := NewLibreOfficeConverter("/mock/soffice", nil)
	ctx := context.Background()

	// Empty data
	_, err := c.Convert(ctx, ConvertRequest{
		Filename:     "test.docx",
		SourceFormat: "docx",
		TargetFormat: "pdf",
		Data:         nil,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot convert an empty file") {
		t.Fatalf("expected error for empty file, got: %v", err)
	}

	// Unsupported conversion
	_, err = c.Convert(ctx, ConvertRequest{
		Filename:     "test.mp3",
		SourceFormat: "mp3",
		TargetFormat: "pdf",
		Data:         []byte("audio-bytes"),
	})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected error for unsupported conversion, got: %v", err)
	}
}

func TestConvertMissingBinaryError(t *testing.T) {
	// Disabled binary simulates missing LibreOffice on system
	c := &LibreOfficeConverter{
		binaryPath: "disabled",
		runner:     DefaultCommandRunner,
		timeout:    10 * time.Second,
	}

	_, err := c.Convert(context.Background(), ConvertRequest{
		Filename:     "report.pdf",
		SourceFormat: "pdf",
		TargetFormat: "docx",
		Data:         []byte("%PDF-1.5 dummy"),
	})
	if err == nil || !strings.Contains(err.Error(), "LibreOffice is required") {
		t.Fatalf("expected LibreOffice missing error, got: %v", err)
	}
}

func TestConvertWithMockRunner(t *testing.T) {
	// Mock runner simulates LibreOffice creating an output file in the outdir
	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		var outDir string
		var targetFormat string
		var baseName string

		for i, arg := range args {
			if arg == "--outdir" && i+1 < len(args) {
				outDir = args[i+1]
			}
			if arg == "--convert-to" && i+1 < len(args) {
				targetFormat = args[i+1]
				if strings.Contains(targetFormat, ":") {
					targetFormat = strings.Split(targetFormat, ":")[0]
				}
			}
			if i == len(args)-1 {
				baseName = strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
			}
		}

		if outDir != "" && targetFormat != "" {
			outPath := filepath.Join(outDir, baseName+"."+targetFormat)
			_ = os.WriteFile(outPath, []byte("converted-content-output"), 0600)
		}

		return []byte("convert success"), nil
	}

	c := NewLibreOfficeConverter("/fake/bin/soffice", mockRunner)
	c.binaryPath = "/fake/bin/soffice"

	testCases := []struct {
		filename string
		src      string
		tgt      string
	}{
		{"document.pdf", "pdf", "docx"},
		{"report.docx", "docx", "pdf"},
		{"sheet.xlsx", "xlsx", "pdf"},
		{"data.pdf", "pdf", "xlsx"},
		{"slides.pptx", "pptx", "pdf"},
		{"deck.pdf", "pdf", "pptx"},
		{"notes.txt", "txt", "pdf"},
		{"memo.docx", "docx", "txt"},
	}

	for _, tc := range testCases {
		res, err := c.Convert(context.Background(), ConvertRequest{
			Filename:     tc.filename,
			SourceFormat: tc.src,
			TargetFormat: tc.tgt,
			Data:         []byte("sample file content bytes"),
		})
		if err != nil {
			t.Fatalf("conversion for %s -> %s failed: %v", tc.src, tc.tgt, err)
		}
		if res == nil {
			t.Fatalf("expected non-nil result for %s -> %s", tc.src, tc.tgt)
		}
		expectedExt := "." + tc.tgt
		if !strings.HasSuffix(res.Filename, expectedExt) {
			t.Errorf("expected output filename ending with %s, got: %s", expectedExt, res.Filename)
		}
		if string(res.Data) != "converted-content-output" {
			t.Errorf("unexpected output data: %s", string(res.Data))
		}
		if res.MimeType != GetMimeType(tc.tgt) {
			t.Errorf("expected mime %s, got %s", GetMimeType(tc.tgt), res.MimeType)
		}
	}
}

func TestDocxToTextFallback(t *testing.T) {
	// Create an in-memory zip resembling a minimal .docx archive
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	docWriter, err := zipWriter.Create("word/document.xml")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello DocuPilot</w:t></w:r></w:p>
    <w:p><w:r><w:t>Second paragraph text.</w:t></w:r></w:p>
  </w:body>
</w:document>`

	if _, err := docWriter.Write([]byte(xmlContent)); err != nil {
		t.Fatalf("failed to write XML: %v", err)
	}
	_ = zipWriter.Close()

	// Converter with disabled binary to force Go fallback
	c := &LibreOfficeConverter{
		binaryPath: "disabled",
		runner:     DefaultCommandRunner,
		timeout:    10 * time.Second,
	}

	res, err := c.Convert(context.Background(), ConvertRequest{
		Filename:     "sample.docx",
		SourceFormat: "docx",
		TargetFormat: "txt",
		Data:         buf.Bytes(),
	})
	if err != nil {
		t.Fatalf("expected Go fallback for docx to txt to succeed, got error: %v", err)
	}

	text := string(res.Data)
	if !strings.Contains(text, "Hello DocuPilot") || !strings.Contains(text, "Second paragraph text.") {
		t.Errorf("expected extracted text, got: %q", text)
	}
	if res.Filename != "sample.txt" {
		t.Errorf("expected output filename sample.txt, got %s", res.Filename)
	}
}
