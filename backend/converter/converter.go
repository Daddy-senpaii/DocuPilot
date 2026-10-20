package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Supported conversion formats.
const (
	FormatPDF  = "pdf"
	FormatDOCX = "docx"
	FormatXLSX = "xlsx"
	FormatPPTX = "pptx"
	FormatTXT  = "txt"
)

// ConversionPair defines a supported source to target transformation.
type ConversionPair struct {
	SourceFormat string `json:"source_format"`
	TargetFormat string `json:"target_format"`
	Title        string `json:"title"`
	Description  string `json:"description"`
}

// ConvertRequest contains the input payload for document conversion.
type ConvertRequest struct {
	Filename     string
	SourceFormat string
	TargetFormat string
	Data         []byte
}

// ConvertResult contains the transformed output payload.
type ConvertResult struct {
	Filename string
	MimeType string
	Data     []byte
}

// CommandRunner represents a function executing a system command.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// DefaultCommandRunner executes the system command using standard exec.CommandContext.
func DefaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// DocumentConverter defines the contract for converting documents across formats.
type DocumentConverter interface {
	Convert(ctx context.Context, req ConvertRequest) (*ConvertResult, error)
	SupportedPairs() []ConversionPair
	IsSupported(source, target string) bool
}

// LibreOfficeConverter implements DocumentConverter using headless LibreOffice / soffice CLI.
type LibreOfficeConverter struct {
	binaryPath string
	runner     CommandRunner
	timeout    time.Duration
}

// NewLibreOfficeConverter creates a new converter instance.
func NewLibreOfficeConverter(customBinary string, runner CommandRunner) *LibreOfficeConverter {
	if runner == nil {
		runner = DefaultCommandRunner
	}

	bin := findLibreOfficeBinary(customBinary)
	return &LibreOfficeConverter{
		binaryPath: bin,
		runner:     runner,
		timeout:    60 * time.Second,
	}
}

// SetTimeout configures the maximum time allowed for a single conversion.
func (c *LibreOfficeConverter) SetTimeout(t time.Duration) {
	if t > 0 {
		c.timeout = t
	}
}

// BinaryPath returns the resolved path to LibreOffice/soffice.
func (c *LibreOfficeConverter) BinaryPath() string {
	return c.binaryPath
}

// SupportedPairs returns the official 8 supported conversion pairs.
func (c *LibreOfficeConverter) SupportedPairs() []ConversionPair {
	return []ConversionPair{
		{
			SourceFormat: FormatPDF,
			TargetFormat: FormatDOCX,
			Title:        "PDF to Word",
			Description:  "Extract text and layout from PDF into editable Word document (.docx)",
		},
		{
			SourceFormat: FormatDOCX,
			TargetFormat: FormatPDF,
			Title:        "Word to PDF",
			Description:  "Render Word document (.docx) into standard print-ready PDF",
		},
		{
			SourceFormat: FormatPDF,
			TargetFormat: FormatXLSX,
			Title:        "PDF to Excel",
			Description:  "Import tables and data from PDF into Excel spreadsheet (.xlsx)",
		},
		{
			SourceFormat: FormatXLSX,
			TargetFormat: FormatPDF,
			Title:        "Excel to PDF",
			Description:  "Render Excel spreadsheet (.xlsx) into clean formatted PDF",
		},
		{
			SourceFormat: FormatPDF,
			TargetFormat: FormatPPTX,
			Title:        "PDF to PowerPoint",
			Description:  "Transform PDF pages into editable PowerPoint presentation slides (.pptx)",
		},
		{
			SourceFormat: FormatPPTX,
			TargetFormat: FormatPDF,
			Title:        "PowerPoint to PDF",
			Description:  "Render presentation slides (.pptx) into standard PDF deck",
		},
		{
			SourceFormat: FormatDOCX,
			TargetFormat: FormatTXT,
			Title:        "Word to Text",
			Description:  "Extract plain text contents from Word document (.docx)",
		},
		{
			SourceFormat: FormatTXT,
			TargetFormat: FormatPDF,
			Title:        "Text to PDF",
			Description:  "Typeset plain text (.txt) into paginated PDF document",
		},
	}
}

// IsSupported verifies if the requested source and target format pair is supported.
func (c *LibreOfficeConverter) IsSupported(source, target string) bool {
	src := NormalizeFormat(source)
	tgt := NormalizeFormat(target)

	for _, pair := range c.SupportedPairs() {
		if pair.SourceFormat == src && pair.TargetFormat == tgt {
			return true
		}
	}
	return false
}

// Convert executes the document transformation.
func (c *LibreOfficeConverter) Convert(ctx context.Context, req ConvertRequest) (*ConvertResult, error) {
	if len(req.Data) == 0 {
		return nil, errors.New("cannot convert an empty file")
	}

	src := NormalizeFormat(req.SourceFormat)
	tgt := NormalizeFormat(req.TargetFormat)

	if src == "" {
		src = NormalizeFormat(filepath.Ext(req.Filename))
	}

	if !c.IsSupported(src, tgt) {
		return nil, fmt.Errorf("conversion from '%s' to '%s' is not supported", src, tgt)
	}

	bin := c.binaryPath
	if bin == "" {
		bin = findLibreOfficeBinary("")
		if bin != "" {
			c.binaryPath = bin
		}
	} else if bin == "disabled" {
		bin = ""
	}

	// Direct Go fallback for DOCX to TXT if LibreOffice is not installed
	if src == FormatDOCX && tgt == FormatTXT && bin == "" {
		extractedTxt, err := extractTextFromDocxZip(req.Data)
		if err == nil && len(extractedTxt) > 0 {
			outName := changeExtension(req.Filename, FormatTXT)
			return &ConvertResult{
				Filename: outName,
				MimeType: GetMimeType(FormatTXT),
				Data:     []byte(extractedTxt),
			}, nil
		}
	}

	if bin == "" {
		return nil, errors.New("LibreOffice is required for conversions but was not found. Please install it with 'sudo apt install libreoffice' or set LIBREOFFICE_PATH")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create isolated temporary directory
	tmpDir, err := os.MkdirTemp("", "docupilot-conv-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary working directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clean input filename
	inputBaseName := filepath.Base(req.Filename)
	if inputBaseName == "" || inputBaseName == "." {
		inputBaseName = "document." + src
	} else if NormalizeFormat(filepath.Ext(inputBaseName)) != src {
		inputBaseName = strings.TrimSuffix(inputBaseName, filepath.Ext(inputBaseName)) + "." + src
	}

	inputPath := filepath.Join(tmpDir, inputBaseName)
	if err := os.WriteFile(inputPath, req.Data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write input file to disk: %w", err)
	}

	// Prepare command arguments
	args := []string{"--headless"}

	switch {
	case src == FormatPDF && tgt == FormatDOCX:
		args = append(args, "--infilter=writer_pdf_import", "--convert-to", "docx")
	case src == FormatPDF && tgt == FormatXLSX:
		args = append(args, "--infilter=calc_pdf_import", "--convert-to", "xlsx")
	case src == FormatPDF && tgt == FormatPPTX:
		args = append(args, "--infilter=impress_pdf_import", "--convert-to", "pptx")
	case src == FormatDOCX && tgt == FormatTXT:
		args = append(args, "--convert-to", "txt:Text")
	default:
		args = append(args, "--convert-to", tgt)
	}

	args = append(args, "--outdir", tmpDir, inputPath)

	outputBytes, err := c.runner(ctxTimeout, bin, args...)
	if err != nil {
		errMsg := strings.TrimSpace(string(outputBytes))
		if errMsg != "" {
			return nil, fmt.Errorf("conversion failed: %s (%w)", errMsg, err)
		}
		return nil, fmt.Errorf("conversion process failed: %w", err)
	}

	// Locate converted output file in tmpDir
	expectedExt := "." + tgt
	var convertedPath string

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect conversion output: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), expectedExt) {
			convertedPath = filepath.Join(tmpDir, entry.Name())
			break
		}
	}

	if convertedPath == "" {
		return nil, fmt.Errorf("converter completed but output file with extension '%s' was not found", expectedExt)
	}

	convertedData, err := os.ReadFile(convertedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted output file: %w", err)
	}

	outFilename := changeExtension(req.Filename, tgt)

	return &ConvertResult{
		Filename: outFilename,
		MimeType: GetMimeType(tgt),
		Data:     convertedData,
	}, nil
}

// NormalizeFormat sanitizes and lowercases format extensions.
func NormalizeFormat(ext string) string {
	clean := strings.ToLower(strings.TrimSpace(ext))
	return strings.TrimPrefix(clean, ".")
}

// GetMimeType maps format extensions to their respective standard MIME types.
func GetMimeType(format string) string {
	switch NormalizeFormat(format) {
	case FormatPDF:
		return "application/pdf"
	case FormatDOCX:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case FormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatPPTX:
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case FormatTXT:
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// changeExtension safely swaps the file extension for the output name.
func changeExtension(filename, newExt string) string {
	base := filepath.Base(filename)
	if base == "" || base == "." {
		base = "document"
	}
	cleanExt := NormalizeFormat(newExt)
	origExt := filepath.Ext(base)
	if origExt != "" {
		base = strings.TrimSuffix(base, origExt)
	}
	return base + "." + cleanExt
}

// findLibreOfficeBinary checks custom path, environment variables, and PATH.
func findLibreOfficeBinary(custom string) string {
	if custom != "" {
		if _, err := exec.LookPath(custom); err == nil {
			return custom
		}
	}

	if envPath := os.Getenv("LIBREOFFICE_PATH"); envPath != "" {
		if _, err := exec.LookPath(envPath); err == nil {
			return envPath
		}
	}

	if envPath := os.Getenv("SOFFICE_BIN"); envPath != "" {
		if _, err := exec.LookPath(envPath); err == nil {
			return envPath
		}
	}

	for _, name := range []string{"soffice", "libreoffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	// Standard common installation locations on Linux
	commonLocations := []string{
		"/usr/bin/soffice",
		"/usr/bin/libreoffice",
		"/usr/local/bin/soffice",
		"/opt/libreoffice/program/soffice",
	}
	for _, loc := range commonLocations {
		if info, err := os.Stat(loc); err == nil && !info.IsDir() {
			return loc
		}
	}

	return ""
}

// extractTextFromDocxZip reads document.xml inside a .docx archive to extract raw text paragraphs.
func extractTextFromDocxZip(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			xmlContent, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			return extractTextFromWordXML(xmlContent), nil
		}
	}

	return "", errors.New("word/document.xml not found inside docx archive")
}

// extractTextFromWordXML parses WordprocessingML XML and extracts paragraph text.
func extractTextFromWordXML(xmlData []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var sb strings.Builder
	var inText bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			if elem.Name.Local == "t" {
				inText = true
			} else if elem.Name.Local == "p" || elem.Name.Local == "tr" {
				if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("\n")
				}
			}
		case xml.EndElement:
			if elem.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.Write(elem)
			}
		}
	}

	return strings.TrimSpace(sb.String())
}
