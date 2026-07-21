// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
)

func TestWriteDocumentRendersSharedBlocks(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Title = "Shared renderer"
	doc.Metadata.Subject = "Renderer test"
	doc.PageTemplate.Header = &layout.HeaderBlock{
		Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Header text"}}, Style: layout.TextStyle{FontSize: 9}}},
	}
	doc.PageTemplate.Footer = &layout.FooterBlock{
		ReservePageArea: true,
	}
	doc.PageTemplate.PageNumbers = layout.PageNumberOptions{Enabled: true, TotalPageAlias: "{total}"}
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Shared Document"}}},
		layout.MetadataGridBlock{Fields: []layout.MetadataField{{Label: "ID", Value: "ABC-123"}, {Label: "Status", Value: "Ready"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "The shared renderer writes model blocks into PDF output."}}},
		layout.ListBlock{Items: []layout.ListItem{
			{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "First item"}}}}},
			{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Second item"}}}}},
		}},
		layout.TableBlock{
			Caption: "Sample table",
			Header:  []layout.TableRow{{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Name"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Value"}}}}}}}},
			Body:    []layout.TableRow{{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Alpha"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "42"}}}}}}}},
		},
		layout.QRVerificationBlock{QR: layout.QRBlock{Label: "Verify", URL: "https://example.test/verify", Size: 18}},
	}
	doc.Signature = &layout.SignatureBlock{Rows: []layout.SignatureRowBlock{{Columns: []layout.SignatureColumn{{Label: "Primary"}, {Label: "Secondary"}}}}}

	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := extractedDocumentText(t, out.Bytes())
	for _, want := range []string{
		"Header text",
		"Shared Document",
		"ID: ABC-123",
		"Sample table",
		"Alpha",
		"Verify",
		"Primary",
		"Page 1 / 1",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("PDF output missing %q", want)
		}
	}
}

func TestWriteDocumentEmitsTaggedRoles(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetComplianceMetadata(ComplianceMetadata{PDFUA2: true, Lang: "en-US", Title: "Tagged document"})
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 2, Segments: []layout.TextSegment{{Text: "Tagged heading"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Tagged paragraph"}}},
		layout.ListBlock{Items: []layout.ListItem{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Tagged item"}}}}}}},
		layout.TableBlock{
			Header: []layout.TableRow{{Cells: []layout.TableCell{{ColSpan: 2, Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Head"}}}}}}}},
			Body:   []layout.TableRow{{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Body"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "More"}}}}}}}},
		},
	}

	pdf.WriteDocument(doc)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	text := output.String()
	for _, want := range []string{
		"/StructTreeRoot ",
		"/S /H2",
		"/S /P",
		"/S /L",
		"/S /LI",
		"/S /Table",
		"/S /TR",
		"/S /TH",
		"/S /TD",
		"/A << /O /Table /Scope /Column /ColSpan 2 >>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("tagged document output missing %q", want)
		}
	}
}

func TestWriteDocumentPageBreakBlockAddsPage(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "before break"}}},
		layout.PageBreakBlock{After: true},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "after break"}}},
	}

	pdf.WriteDocument(doc)
	if got := pdf.PageCount(); got != 2 {
		t.Fatalf("PageCount() = %d, want 2", got)
	}
}

func TestWriteDocumentAcceptsBuiltInBlockPointersAndSkipsTypedNil(t *testing.T) {
	var nilParagraph *layout.ParagraphBlock
	heading := &layout.HeadingBlock{Level: 2, Segments: []layout.TextSegment{{Text: "pointer heading"}}}
	pageBreak := &layout.PageBreakBlock{After: true}
	paragraph := &layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "pointer paragraph"}}}
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{nilParagraph, heading, pageBreak, paragraph}

	pdf.WriteDocument(doc)
	if err := pdf.Error(); err != nil {
		t.Fatalf("WriteDocument() error = %v", err)
	}
	if pdf.PageCount() != 2 {
		t.Fatalf("PageCount() = %d, want pointer PageBreakBlock to add page", pdf.PageCount())
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	text := extractedDocumentText(t, output.Bytes())
	for _, want := range []string{"pointer heading", "pointer paragraph"} {
		if !strings.Contains(text, want) {
			t.Fatalf("PDF output missing %q", want)
		}
	}
}

func TestWriteDocumentErrorsForUnknownFont(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{
		layout.ParagraphBlock{
			Segments: []layout.TextSegment{{Text: "font error text"}},
			Style:    layout.TextStyle{FontFamily: "MissingFont", Bold: true, Italic: true},
		},
	}

	pdf.WriteDocument(doc)
	if err := pdf.Error(); err == nil || !strings.Contains(err.Error(), "layout document plan unsupported") {
		t.Fatalf("Error() = %v, want unsupported planner font error", err)
	}
}

func TestWriteDocumentErrorsForUnavailableBoldItalicFace(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.ensureResourceStore().setFont("custom", fontDefinition{})
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{
		layout.ParagraphBlock{
			Segments: []layout.TextSegment{{Text: "font face error text"}},
			Style:    layout.TextStyle{FontFamily: "custom", Bold: true, Italic: true},
		},
	}

	pdf.WriteDocument(doc)
	if err := pdf.Error(); err == nil || !strings.Contains(err.Error(), "layout document plan unsupported") {
		t.Fatalf("Error() = %v, want unsupported planner font error", err)
	}
}

func TestWriteDocumentErrorsForUnsupportedBlock(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{unsupportedTestBlock{}}

	pdf.WriteDocument(doc)
	if err := pdf.Error(); err == nil || !strings.Contains(err.Error(), "layout document plan unsupported") {
		t.Fatalf("Error() = %v, want unsupported planner block error", err)
	}
}

type unsupportedTestBlock struct{}

func (unsupportedTestBlock) DocumentBlockKind() layout.BlockKind { return "test-unsupported" }

func TestWriteDocumentRendersSignatureMetadata(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Signature = &layout.SignatureBlock{Rows: []layout.SignatureRowBlock{{
		Columns: []layout.SignatureColumn{{
			Label: "Signed by",
			Name:  "Alex Example",
			Role:  "Reviewer",
			Metadata: []layout.MetadataField{
				{Label: "ID", Value: "123"},
			},
		}},
	}}}

	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := extractedDocumentText(t, out.Bytes())
	for _, want := range []string{"Signed by", "Alex Example", "Reviewer", "ID: 123"} {
		if !strings.Contains(content, want) {
			t.Fatalf("PDF output missing signature metadata %q", want)
		}
	}
}

func TestWriteDocumentErrorsForEmptyQRVerificationBlock(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{layout.QRVerificationBlock{QR: layout.QRBlock{Label: "Verify"}}}

	pdf.WriteDocument(doc)
	if err := pdf.Error(); err == nil || !strings.Contains(err.Error(), "layout document plan unsupported") {
		t.Fatalf("Error() = %v, want unsupported planner QR error", err)
	}
}

func TestCellFormatUTF8JustifiedSingleWordDoesNotWriteInvalidNumber(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	fontBytes, err := os.ReadFile("../assets/static/font/DejaVuSansCondensed.ttf")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	pdf.AddUTF8FontFromBytes("dejavu", "", fontBytes)
	pdf.SetFont("dejavu", "", 12)
	pdf.AddPage()

	pdf.CellFormat(80, 8, "Alone", "", 1, "J", false, 0, "")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := out.String()
	if strings.Contains(content, "+Inf") || strings.Contains(content, "-Inf") || strings.Contains(content, "NaN") {
		t.Fatalf("PDF output contains invalid numeric token")
	}
}

func TestEmbeddedUnicodeTextProducesCompletePDF(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	fontBytes, err := os.ReadFile("../assets/static/font/DejaVuSansCondensed.ttf")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	pdf.AddUTF8FontFromBytes("dejavu", "", fontBytes)
	pdf.SetFont("dejavu", "", 12)
	pdf.AddPage()
	pdf.CellFormat(120, 8, "Relatório clínico em Português", "", 1, "L", false, 0, "")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) || !bytes.Contains(out.Bytes(), []byte("%%EOF")) {
		t.Fatal("unicode output is not a complete PDF")
	}
}

func TestWriteDocumentAppliesPageTemplateMargins(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.PageTemplate.Margins = layout.Spacing{Left: 18, Top: 16, Right: 14, Bottom: 22}
	doc.Body = []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Body"}}}}

	pdf.WriteDocument(doc)

	left, top, right, bottom := pdf.GetMargins()
	if left != 18 || top != 16 || right != 14 || bottom != 22 {
		t.Fatalf("margins = %.1f %.1f %.1f %.1f, want 18 16 14 22", left, top, right, bottom)
	}
}

func TestWriteDocumentRendersTemplateFooterOnEveryRendererPage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.PageTemplate.Footer = &layout.FooterBlock{
		ReservePageArea: true,
		Blocks:          []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Repeated footer"}}}},
	}
	doc.Body = []layout.Block{
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Page one"}}},
		layout.PageBreakBlock{After: true},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Page two"}}},
	}

	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if got := strings.Count(extractedDocumentText(t, out.Bytes()), "Repeated footer"); got != 2 {
		t.Fatalf("footer count = %d, want 2", got)
	}
}

func TestWriteDocumentSelectsTemplateHeadersAndFootersPerPage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.PageTemplate.Header = &layout.HeaderBlock{
		Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Default header"}}}},
	}
	doc.PageTemplate.FirstPageHeader = &layout.HeaderBlock{
		Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "First header"}}}},
	}
	doc.PageTemplate.Footer = &layout.FooterBlock{
		ReservePageArea: true,
		Blocks:          []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Default footer"}}}},
	}
	doc.PageTemplate.FirstPageFooter = &layout.FooterBlock{
		ReservePageArea: true,
		Blocks:          []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "First footer"}}}},
	}
	doc.PageTemplate.EvenPageFooter = &layout.FooterBlock{
		ReservePageArea: true,
		Blocks:          []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Even footer"}}}},
	}
	doc.Body = []layout.Block{
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Page one body"}}},
		layout.PageBreakBlock{After: true},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Page two body"}}},
		layout.PageBreakBlock{After: true},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Page three body"}}},
	}

	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := extractedDocumentText(t, out.Bytes())
	for _, want := range []string{"First header", "Default header", "First footer", "Even footer", "Default footer"} {
		if !strings.Contains(content, want) {
			t.Fatalf("PDF output missing %q", want)
		}
	}
	if got := strings.Count(content, "Default header"); got != 2 {
		t.Fatalf("default header count = %d, want 2", got)
	}
}

func TestWriteDocumentMapsLayoutAttachments(t *testing.T) {
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Attachments = []layout.AttachmentBlock{{
		Name:        "evidence.txt",
		Description: "Evidence",
		Data:        []byte("attached"),
	}}

	pdf.WriteDocument(doc)

	if len(pdf.attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(pdf.attachments))
	}
	if pdf.attachments[0].Filename != "evidence.txt" || !bytes.Equal(pdf.attachments[0].Content, []byte("attached")) {
		t.Fatalf("attachment = %#v, want mapped layout attachment", pdf.attachments[0])
	}
}

func TestWriteDocumentInlineImagesUseContentHashAndFit(t *testing.T) {
	pixel := decodeDocumentRenderTestPNG(t)
	pdf := mustNewPDFDocument()
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{
		layout.ImageBlock{Data: pixel, Format: "png", Width: 16, Height: 8, Fit: layout.ImageFitContain},
		layout.ImageBlock{Data: pixel, Format: "png", Width: 16, Height: 8, Fit: layout.ImageFitCover},
	}

	pdf.WriteDocument(doc)

	if err := pdf.Error(); err != nil {
		t.Fatalf("WriteDocument() error = %v", err)
	}
	resources := pdf.ensureResourceStore()
	if got := len(resources.images); got != 1 {
		t.Fatalf("registered images = %d, want deterministic reuse of identical inline data", got)
	}
	for name := range resources.images {
		if !strings.HasPrefix(name, "plan-image-") {
			t.Fatalf("registered image name = %q, want hash-based document image name", name)
		}
	}
}

func extractedDocumentText(t *testing.T, pdf []byte) string {
	t.Helper()
	var text strings.Builder
	if bytes.Contains(pdf, []byte("/ActualText")) {
		for offset := 0; offset < len(pdf); {
			index := bytes.Index(pdf[offset:], []byte("/ActualText"))
			if index < 0 {
				break
			}
			offset += index + len("/ActualText")
			for offset < len(pdf) && (pdf[offset] == ' ' || pdf[offset] == '\t' || pdf[offset] == '\r' || pdf[offset] == '\n') {
				offset++
			}
			value, next, ok := generatedPDFTestLiteral(pdf, offset)
			if ok {
				text.WriteString(value)
				offset = next
			}
		}
	}
	markedDepth := 0
	for offset := 0; offset < len(pdf); {
		if offset+3 <= len(pdf) && string(pdf[offset:offset+3]) == "BDC" {
			markedDepth++
			offset += 3
			continue
		}
		if offset+3 <= len(pdf) && string(pdf[offset:offset+3]) == "EMC" {
			if markedDepth > 0 {
				markedDepth--
			}
			offset += 3
			continue
		}
		index := bytes.IndexByte(pdf[offset:], '(')
		if index < 0 {
			break
		}
		offset += index
		value, next, ok := generatedPDFTestLiteral(pdf, offset)
		if !ok {
			offset++
			continue
		}
		operator := next
		for operator < len(pdf) && (pdf[operator] == ' ' || pdf[operator] == '\t' || pdf[operator] == '\r' || pdf[operator] == '\n') {
			operator++
		}
		if markedDepth == 0 && operator+2 <= len(pdf) && string(pdf[operator:operator+2]) == "Tj" {
			text.WriteString(value)
		}
		offset = next
	}
	return text.String()
}

// generatedPDFTestLiteral decodes the literal strings emitted by this
// package's uncompressed test output. It is test-only and is not a PDF input
// API or a general-purpose parser.
func generatedPDFTestLiteral(pdf []byte, start int) (string, int, bool) {
	if start < 0 || start >= len(pdf) || pdf[start] != '(' {
		return "", start, false
	}
	decoded := make([]byte, 0, 32)
	depth := 1
	for index := start + 1; index < len(pdf); index++ {
		char := pdf[index]
		if char == '\\' {
			index++
			if index >= len(pdf) {
				return "", start, false
			}
			escaped := pdf[index]
			switch escaped {
			case 'n':
				decoded = append(decoded, '\n')
			case 'r':
				decoded = append(decoded, '\r')
			case 't':
				decoded = append(decoded, '\t')
			case 'b':
				decoded = append(decoded, '\b')
			case 'f':
				decoded = append(decoded, '\f')
			case '\n':
			case '\r':
				if index+1 < len(pdf) && pdf[index+1] == '\n' {
					index++
				}
			default:
				if escaped >= '0' && escaped <= '7' {
					value := int(escaped - '0')
					for count := 1; count < 3 && index+1 < len(pdf) && pdf[index+1] >= '0' && pdf[index+1] <= '7'; count++ {
						index++
						value = value*8 + int(pdf[index]-'0')
					}
					decoded = append(decoded, byte(value))
				} else {
					decoded = append(decoded, escaped)
				}
			}
			continue
		}
		switch char {
		case '(':
			depth++
			decoded = append(decoded, char)
		case ')':
			depth--
			if depth == 0 {
				if len(decoded) >= 2 && decoded[0] == 0xfe && decoded[1] == 0xff {
					runes := make([]rune, 0, (len(decoded)-2)/2)
					for position := 2; position+1 < len(decoded); position += 2 {
						runes = append(runes, rune(uint16(decoded[position])<<8|uint16(decoded[position+1])))
					}
					return string(runes), index + 1, true
				}
				return string(decoded), index + 1, true
			}
			decoded = append(decoded, char)
		default:
			decoded = append(decoded, char)
		}
	}
	return "", start, false
}

func decodeDocumentRenderTestPNG(t *testing.T) []byte {
	t.Helper()
	const pixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	data, err := base64.StdEncoding.DecodeString(pixelPNG)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	return data
}
