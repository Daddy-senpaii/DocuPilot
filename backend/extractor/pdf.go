package extractor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dslipak/pdf"
)

// TextExtractor defines the contract for extracting text from a document stream.
type TextExtractor interface {
	ExtractText(reader io.ReaderAt, size int64) (string, error)
}

// PDFExtractor implements TextExtractor for PDF documents using pure Go.
type PDFExtractor struct{}

// NewPDFExtractor creates a new instance of PDFExtractor.
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// ExtractText reads a PDF from an io.ReaderAt and extracts text across all pages.
func (e *PDFExtractor) ExtractText(reader io.ReaderAt, size int64) (string, error) {
	if size <= 0 {
		return "", errors.New("cannot extract text from an empty file")
	}

	pdfReader, err := pdf.NewReader(reader, size)
	if err != nil {
		return "", fmt.Errorf("unable to read PDF structure: %w", err)
	}

	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", errors.New("the PDF document contains zero pages")
	}

	var textBuilder strings.Builder
	for pageIndex := 1; pageIndex <= numPages; pageIndex++ {
		page := pdfReader.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}

		plainText, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		cleanPageText := strings.TrimSpace(plainText)
		if cleanPageText != "" {
			if textBuilder.Len() > 0 {
				textBuilder.WriteString("\n\n")
			}
			textBuilder.WriteString(cleanPageText)
		}
	}

	result := strings.TrimSpace(textBuilder.String())
	if result == "" {
		return "", errors.New("the PDF appears to be empty, scanned as images, or does not contain selectable text")
	}

	return result, nil
}

// ValidatePDFHeader checks that the file starts with the standard %PDF- magic bytes.
func ValidatePDFHeader(header []byte) error {
	if len(header) < 5 || !bytes.HasPrefix(header, []byte("%PDF-")) {
		return errors.New("invalid file format: the uploaded file is not a valid PDF")
	}
	return nil
}
