// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBookmarkDestinationsUseActualPageObjectNumbers(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetAttachments([]Attachment{{Content: []byte("payload"), Filename: "payload.txt"}})
	pdf.AddPage()
	pdf.Bookmark("start", 0, -1)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatal(err)
	}
	pageObj := pdf.pageObjectNumber(1)
	if pageObj == 0 || pageObj == 3 {
		t.Fatalf("page object = %d, want shifted object number", pageObj)
	}
	if !strings.Contains(out.String(), sprintf("/Dest [%d 0 R", pageObj)) {
		t.Fatalf("bookmark destination does not reference actual page object %d", pageObj)
	}
}

func TestBookmarkValidationRejectsInvalidLevelsAndMissingPage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.Bookmark("missing page", 0, -1)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "active page") {
		t.Fatalf("Bookmark before AddPage error = %v, want active page error", pdf.Error())
	}

	pdf = mustNewPDFDocument()
	pdf.AddPage()
	pdf.Bookmark("bad first", 1, -1)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "first bookmark level") {
		t.Fatalf("first bookmark level error = %v", pdf.Error())
	}

	pdf = mustNewPDFDocument()
	pdf.AddPage()
	pdf.Bookmark("root", 0, -1)
	pdf.Bookmark("skip", 2, -1)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "cannot jump") {
		t.Fatalf("skipped bookmark level error = %v", pdf.Error())
	}
}

func TestSplitTextPreservesCJKCharacters(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetFont("Helvetica", "", 12)

	const text = "中文かな한글"
	lines := pdf.SplitText(text, 4)
	if got := strings.Join(lines, ""); got != text {
		t.Fatalf("SplitText joined lines = %q, want %q; lines=%q", got, text, lines)
	}
}

func TestAddPageFormatRejectsInvalidOrientationAndSize(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPageFormat("banana", Size{Wd: 100, Ht: 100})
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "incorrect orientation") {
		t.Fatalf("invalid orientation error = %v", pdf.Error())
	}
	if pdf.PageNo() != 0 {
		t.Fatalf("invalid AddPageFormat added page %d", pdf.PageNo())
	}

	pdf = mustNewPDFDocument()
	pdf.AddPageFormat("P", Size{Wd: math.NaN(), Ht: 100})
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "invalid page size") {
		t.Fatalf("invalid page size error = %v", pdf.Error())
	}
	if pdf.PageNo() != 0 {
		t.Fatalf("invalid AddPageFormat added page %d", pdf.PageNo())
	}
}

func TestClipPolygonRejectsInvalidPointCountWithoutEnteringClipState(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()

	pdf.ClipPolygon([]Point{{X: 1, Y: 1}, {X: 2, Y: 2}}, false)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "at least 3 points") {
		t.Fatalf("ClipPolygon error = %v", pdf.Error())
	}
	if pdf.clipNest != 0 {
		t.Fatalf("clipNest = %d, want 0", pdf.clipNest)
	}
}

func TestGetStringWidthWithoutFontSetsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	if width := pdf.GetStringWidth("abc"); width != 0 {
		t.Fatalf("GetStringWidth without font = %.2f, want 0", width)
	}
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "font must be selected") {
		t.Fatalf("GetStringWidth error = %v", pdf.Error())
	}
}

func TestTextAPIsWithoutFontReturnErrorsInsteadOfPanicking(t *testing.T) {
	tests := []struct {
		name string
		run  func(*pdfDocument)
	}{
		{name: "Text", run: func(pdf *pdfDocument) { pdf.Text(10, 10, "abc") }},
		{name: "Write", run: func(pdf *pdfDocument) { pdf.Write(6, "abc") }},
		{name: "MultiCell", run: func(pdf *pdfDocument) { pdf.MultiCell(20, 6, "abc", "", "", false) }},
		{name: "SplitLines", run: func(pdf *pdfDocument) { pdf.SplitLines([]byte("abc"), 20) }},
		{name: "SplitLineCount", run: func(pdf *pdfDocument) { pdf.SplitLineCount([]byte("abc"), 20) }},
		{name: "SplitText", run: func(pdf *pdfDocument) { pdf.SplitText("abc", 20) }},
		{name: "SplitTextCount", run: func(pdf *pdfDocument) { pdf.SplitTextCount("abc", 20) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pdf := mustNewPDFDocument()
			pdf.AddPage()
			test.run(pdf)
			if err := pdf.Error(); err == nil || !strings.Contains(err.Error(), "font must be selected") {
				t.Fatalf("Error() = %v, want missing-font error", err)
			}
		})
	}
}

func TestTextMeasurementToleratesShortFontWidthTables(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.currentFont.Cw = pdf.currentFont.Cw[:32]
	text := string([]byte{'C', 0xff})

	if width := pdf.GetStringWidth(text); width <= 0 {
		t.Fatalf("GetStringWidth() = %.2f, want positive fallback width", width)
	}
	if lines := pdf.SplitLines([]byte(text), 20); len(lines) == 0 {
		t.Fatal("SplitLines() returned no lines")
	}
	pdf.MultiCell(20, 6, text, "", "", false)
	if err := pdf.Error(); err != nil {
		t.Fatalf("text APIs with short width table returned error: %v", err)
	}
}

func TestImageAndAttachmentBoundaryValidation(t *testing.T) {
	pdf := mustNewPDFDocument()
	if info := pdf.RegisterImageOptionsReader("", ImageOptions{ImageType: "png"}, bytes.NewReader(nil)); info != nil {
		t.Fatal("RegisterImageOptionsReader with blank name returned image info")
	}
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "image name") {
		t.Fatalf("blank image name error = %v", pdf.Error())
	}

	pdf = mustNewPDFDocument()
	pdf.AddAttachmentAnnotation(nil, 1, 1, 1, 1)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "requires an attachment") {
		t.Fatalf("nil attachment annotation error = %v", pdf.Error())
	}
}

func TestSetAttachmentsCopiesContent(t *testing.T) {
	pdf := mustNewPDFDocument()
	content := []byte("original")
	attachments := []Attachment{{Content: content, Filename: "a.txt"}}
	pdf.SetAttachments(attachments)
	content[0] = 'X'
	attachments[0].Filename = "changed.txt"

	if got := string(pdf.attachments[0].Content); got != "original" {
		t.Fatalf("attachment content = %q, want original", got)
	}
	if got := pdf.attachments[0].Filename; got != "a.txt" {
		t.Fatalf("attachment filename = %q, want a.txt", got)
	}
}

func TestSetAttachmentsImmutableSharesContent(t *testing.T) {
	pdf := mustNewPDFDocument()
	content := []byte("original")
	pdf.SetAttachmentsImmutable([]Attachment{{Content: content, Filename: "a.txt", MIMEType: " text/plain ", AFRelationship: " Source "}})
	content[0] = 'X'

	if got := string(pdf.attachments[0].Content); got != "Xriginal" {
		t.Fatalf("attachment content = %q, want shared content", got)
	}
	if got := pdf.attachments[0].mimeType; got != "text/plain" {
		t.Fatalf("normalized MIME type = %q, want text/plain", got)
	}
	if got := pdf.attachments[0].afRelationship; got != "Source" {
		t.Fatalf("normalized AFRelationship = %q, want Source", got)
	}
}

func TestAttachmentFromFileLoadsDuringOutput(t *testing.T) {
	fileStr := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(fileStr, []byte("file-backed payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
	if len(pdf.attachments[0].Content) != 0 {
		t.Fatal("file-backed attachment loaded before output")
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "hello")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatal(err)
	}
	if got := string(pdf.attachments[0].Content); got != "file-backed payload" {
		t.Fatalf("loaded attachment content = %q, want file payload", got)
	}
	if got := pdf.attachments[0].Filename; got != "payload.txt" {
		t.Fatalf("attachment filename = %q, want payload.txt", got)
	}
	if !strings.Contains(out.String(), "/EmbeddedFiles") {
		t.Fatal("generated PDF missing /EmbeddedFiles")
	}
}

func TestAttachmentFromFileRejectsOversizeFile(t *testing.T) {
	fileStr := filepath.Join(t.TempDir(), "payload.bin")
	file, err := os.Create(fileStr)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxAttachmentBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	pdf := mustNewPDFDocument()
	pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
	pdf.AddPage()

	var out bytes.Buffer
	err = pdf.Output(&out)
	if err == nil || !strings.Contains(err.Error(), "attachment data exceeds maximum size") {
		t.Fatalf("Output() error = %v, want attachment size limit", err)
	}
}

func TestAttachmentFromFileWithOptionsEagerValidation(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.bin")
	if _, err := AttachmentFromFileWithOptions(missing, AttachmentOptions{Eager: true}); err == nil {
		t.Fatal("AttachmentFromFileWithOptions missing file error = nil")
	}

	fileStr := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(fileStr, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AttachmentFromFileWithOptions(fileStr, AttachmentOptions{Eager: true, MaxBytes: 3}); err == nil || !strings.Contains(err.Error(), "attachment data exceeds maximum size") {
		t.Fatalf("AttachmentFromFileWithOptions oversize error = %v, want size limit", err)
	}
	if attachment, err := AttachmentFromFileWithOptions(fileStr, AttachmentOptions{Eager: true, MaxBytes: 16}); err != nil || attachment.maxBytes != 16 {
		t.Fatalf("AttachmentFromFileWithOptions valid = %#v, %v; want maxBytes 16", attachment, err)
	}
}

func TestAttachmentFromLoaderLoadsDuringOutput(t *testing.T) {
	opened := 0
	loader := AttachmentLoaderFunc(func(ctx context.Context) (io.ReadCloser, int64, error) {
		opened++
		if err := outputCanceledError(ctx); err != nil {
			return nil, 0, err
		}
		return io.NopCloser(strings.NewReader("loader payload")), int64(len("loader payload")), nil
	})

	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetAttachments([]Attachment{AttachmentFromLoader("loader.txt", loader)})
	if len(pdf.attachments[0].Content) != 0 {
		t.Fatal("loader-backed attachment loaded before output")
	}
	pdf.AddPage()

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if opened == 0 {
		t.Fatal("loader was not opened during output")
	}
	if got := string(pdf.attachments[0].Content); got != "loader payload" {
		t.Fatalf("loaded attachment content = %q, want loader payload", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("/EmbeddedFiles")) {
		t.Fatal("output missing embedded file name tree")
	}
}

func TestAttachmentEmptyContentTakesPrecedenceOverLoader(t *testing.T) {
	opened := 0
	loader := AttachmentLoaderFunc(func(context.Context) (io.ReadCloser, int64, error) {
		opened++
		return io.NopCloser(strings.NewReader("loader payload")), int64(len("loader payload")), nil
	})

	pdf, err := newPDFDocument(WithSecurityPolicy(SecurityPolicy{}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.SetCompression(false)
	pdf.SetAttachments([]Attachment{{
		Content:  []byte{},
		Filename: "empty.txt",
		Loader:   loader,
	}})
	if pdf.attachments[0].Content == nil {
		t.Fatal("SetAttachments dropped explicit empty content")
	}
	pdf.AddPage()

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if opened != 0 {
		t.Fatalf("loader opened %d times, want 0", opened)
	}
	if pdf.attachments[0].Content == nil || len(pdf.attachments[0].Content) != 0 {
		t.Fatalf("attachment content = %#v, want explicit empty content", pdf.attachments[0].Content)
	}
	if !bytes.Contains(out.Bytes(), []byte("/Size 0")) {
		t.Fatal("output missing zero-size embedded file metadata")
	}
}

func TestAttachmentFromLoaderWithOptionsEagerValidation(t *testing.T) {
	loader := AttachmentLoaderFunc(func(context.Context) (io.ReadCloser, int64, error) {
		return io.NopCloser(strings.NewReader("payload")), int64(len("payload")), nil
	})
	if _, err := AttachmentFromLoaderWithOptions("loader.txt", loader, AttachmentOptions{Eager: true, MaxBytes: 3}); !errors.Is(err, ErrAttachmentTooLarge) {
		t.Fatalf("AttachmentFromLoaderWithOptions oversize error = %v, want ErrAttachmentTooLarge", err)
	}
	attachment, err := AttachmentFromLoaderWithOptions("loader.txt", loader, AttachmentOptions{Eager: true, MaxBytes: 16})
	if err != nil {
		t.Fatalf("AttachmentFromLoaderWithOptions valid error = %v", err)
	}
	if attachment.maxBytes != 16 {
		t.Fatalf("loader attachment maxBytes = %d, want 16", attachment.maxBytes)
	}
}

func TestAttachmentCompressionCanSpoolToTempFile(t *testing.T) {
	content := bytes.Repeat([]byte("payload"), attachmentCompressionSpoolThreshold/len("payload")+1)
	stream, checksum, err := compressAttachmentWithChecksum(content, defaultCompressionLevel(), true)
	if err != nil {
		t.Fatalf("compressAttachmentWithChecksum() error = %v", err)
	}
	defer stream.cleanup()
	if stream.tempFile == "" {
		t.Fatal("spooled attachment stream tempFile is empty")
	}
	if stream.size <= 0 {
		t.Fatalf("spooled attachment stream size = %d, want > 0", stream.size)
	}
	if checksum != attachmentChecksum(content) {
		t.Fatalf("checksum = %s, want %s", checksum, attachmentChecksum(content))
	}
	if _, err := os.Stat(stream.tempFile); err != nil {
		t.Fatalf("stat spooled attachment stream: %v", err)
	}
	stream.cleanup()
	if _, err := os.Stat(stream.tempFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat after cleanup = %v, want os.ErrNotExist", err)
	}
}

func TestImageParserContextAPICancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pdf := mustNewPDFDocument()
	if _, err := pdf.RegisterImageOptionsReaderContext(ctx, "img", ImageOptions{ImageType: "png"}, strings.NewReader("")); !errors.Is(err, context.Canceled) {
		t.Fatalf("RegisterImageOptionsReaderContext() error = %v, want context.Canceled", err)
	}
}

func TestSecurityPolicyGatesAttachmentLoaders(t *testing.T) {
	loader := AttachmentLoaderFunc(func(context.Context) (io.ReadCloser, int64, error) {
		return io.NopCloser(strings.NewReader("payload")), int64(len("payload")), nil
	})
	pdf, err := newPDFDocument(WithSecurityPolicy(SecurityPolicy{}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.SetAttachments([]Attachment{AttachmentFromLoader("loader.txt", loader)})
	pdf.AddPage()

	var out bytes.Buffer
	err = pdf.Output(&out)
	if !errors.Is(err, ErrSecurityPolicyDenied) {
		t.Fatalf("Output() error = %v, want ErrSecurityPolicyDenied", err)
	}
}

func TestSetMaxAttachmentBytesAppliesDocumentLimit(t *testing.T) {
	fileStr := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(fileStr, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	pdf := mustNewPDFDocument()
	pdf.SetMaxAttachmentBytes(3)
	pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
	pdf.AddPage()

	var out bytes.Buffer
	err := pdf.Output(&out)
	if err == nil || !strings.Contains(err.Error(), "attachment data exceeds maximum size") {
		t.Fatalf("Output() error = %v, want attachment size limit", err)
	}
}

func TestAttachmentOutputDedupesEquivalentFiles(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	a1 := Attachment{Content: []byte("same attachment"), Filename: "a.txt", Description: "same"}
	a2 := Attachment{Content: []byte("same attachment"), Filename: "a.txt", Description: "same"}
	pdf.SetAttachments([]Attachment{a1})
	pdf.AddPage()
	pdf.AddAttachmentAnnotation(&a2, 10, 10, 20, 10)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if got := bytes.Count(out.Bytes(), []byte("/Type /EmbeddedFile")); got != 1 {
		t.Fatalf("embedded file stream count = %d, want 1", got)
	}
	if got := bytes.Count(out.Bytes(), []byte("/Type /Filespec")); got != 1 {
		t.Fatalf("filespec count = %d, want 1", got)
	}
}

func TestEmbeddedFileNamesUseAttachmentSpelling(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetAttachments([]Attachment{{Content: []byte("payload"), Filename: "payload.txt"}})
	pdf.attachments[0].objectNumber = 42

	names := pdf.getEmbeddedFiles()
	if !strings.Contains(names, "(Attachment1)") {
		t.Fatalf("embedded file names = %q, want Attachment spelling", names)
	}
	if strings.Contains(names, "Attachement") {
		t.Fatalf("embedded file names retain legacy typo: %q", names)
	}
}

func TestAddAttachmentAnnotationCopiesInput(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	content := []byte("original")
	attachment := Attachment{Content: content, Filename: "a.txt", MIMEType: " text/plain ", AFRelationship: " Source "}

	pdf.AddAttachmentAnnotation(&attachment, 10, 10, 20, 10)
	content[0] = 'X'
	attachment.Filename = "changed.txt"
	attachment.MIMEType = "application/json"
	attachment.AFRelationship = "Data"

	stored := pdf.pageAttachments[pdf.page][0].Attachment
	if got := string(stored.Content); got != "original" {
		t.Fatalf("annotation attachment content = %q, want original", got)
	}
	if got := stored.Filename; got != "a.txt" {
		t.Fatalf("annotation attachment filename = %q, want a.txt", got)
	}
	if got := stored.mimeType; got != "text/plain" {
		t.Fatalf("annotation attachment MIME type = %q, want text/plain", got)
	}
	if got := stored.afRelationship; got != "Source" {
		t.Fatalf("annotation attachment relationship = %q, want Source", got)
	}
}

func TestCatalogOmitsNamesWhenUnused(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(10, 10, "plain")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("/Names <<")) {
		t.Fatal("catalog contains /Names without JavaScript or attachments")
	}
	if bytes.Contains(out.Bytes(), []byte("/EmbeddedFiles")) {
		t.Fatal("catalog contains /EmbeddedFiles without attachments")
	}
}

func TestSetJavascriptIsUnsupported(t *testing.T) {
	pdf := mustNewPDFDocument()
	if err := pdf.SetJavascriptError("app.alert('blocked')"); !errors.Is(err, ErrJavaScriptUnsupported) {
		t.Fatalf("SetJavascriptError() error = %v, want ErrJavaScriptUnsupported", err)
	}
	if !errors.Is(pdf.Error(), ErrJavaScriptUnsupported) {
		t.Fatalf("document error = %v, want ErrJavaScriptUnsupported", pdf.Error())
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); !errors.Is(err, ErrJavaScriptUnsupported) {
		t.Fatalf("Output() error = %v, want ErrJavaScriptUnsupported", err)
	}
	if bytes.Contains(out.Bytes(), []byte("/JavaScript")) || bytes.Contains(out.Bytes(), []byte("/JS")) {
		t.Fatal("unsupported JavaScript API emitted JavaScript action bytes")
	}
}

func TestDeterministicOutputSortsPageBoxes(t *testing.T) {
	first := deterministicPageBoxOutput(t, []string{"trim", "crop", "bleed", "art"})
	second := deterministicPageBoxOutput(t, []string{"art", "bleed", "crop", "trim"})
	if !bytes.Equal(first, second) {
		t.Fatal("deterministic page-box output changed with insertion order")
	}

	previous := -1
	for _, name := range []string{"/ArtBox", "/BleedBox", "/CropBox", "/TrimBox"} {
		index := bytes.Index(first, []byte(name))
		if index < 0 {
			t.Fatalf("output missing %s", name)
		}
		if index < previous {
			t.Fatalf("%s appeared before previous page box", name)
		}
		previous = index
	}
}

func deterministicPageBoxOutput(t *testing.T, boxOrder []string) []byte {
	t.Helper()
	pdf, err := newPDFDocument(WithDeterministicOutput())
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.SetCompression(false)
	pdf.AddPage()
	for _, name := range boxOrder {
		pdf.SetPageBox(name, 1, 2, 30, 40)
	}
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "boxes")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	return out.Bytes()
}

func TestDeterministicOutputSortsSpotColorResourceDictionaries(t *testing.T) {
	first := deterministicSpotColorOutput(t, false)
	second := deterministicSpotColorOutput(t, true)
	if !bytes.Equal(first, second) {
		t.Fatal("deterministic spot-color output changed with map insertion order")
	}
	assertOutputOrder(t, first, "/CS1", "/CS2")
}

func deterministicSpotColorOutput(t *testing.T, reverse bool) []byte {
	t.Helper()
	pdf, err := newPDFDocument(WithDeterministicOutput())
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.spotColorMap = make(map[string]spotColorType)
	colors := []struct {
		name string
		info spotColorType
	}{
		{name: "Alpha", info: spotColorType{id: 1, cmyk: cmykColorType{c: 1, m: 2, y: 3, k: 4}}},
		{name: "Zeta", info: spotColorType{id: 2, cmyk: cmykColorType{c: 5, m: 6, y: 7, k: 8}}},
	}
	if reverse {
		colors[0], colors[1] = colors[1], colors[0]
	}
	for _, color := range colors {
		pdf.spotColorMap[color.name] = color.info
	}
	pdf.SetFillSpotColor("Alpha", 70)
	pdf.Rect(10, 10, 20, 10, "F")
	pdf.SetDrawSpotColor("Zeta", 50)
	pdf.Rect(35, 10, 20, 10, "D")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	return out.Bytes()
}

func assertOutputOrder(t *testing.T, output []byte, ordered ...string) {
	t.Helper()
	previous := -1
	for _, item := range ordered {
		index := bytes.Index(output, []byte(item))
		if index < 0 {
			t.Fatalf("output missing %s", item)
		}
		if index < previous {
			t.Fatalf("%s appeared before previous item in output", item)
		}
		previous = index
	}
}

func TestDeterministicOutputSortsMapBackedResourceKeys(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCatalogSort(true)
	pdf.AddSpotColor("Zeta", 1, 2, 3, 4)
	pdf.AddSpotColor("Alpha", 5, 6, 7, 8)
	assertStringSlice(t, pdf.spotColorOutputNames(), []string{"Alpha", "Zeta"})

	pdf.aliasMap = map[string]string{
		"zeta":  "Z",
		"alpha": "A",
	}
	pdf.aliasNeedlesDirty = true
	pdf.compiledAliasNeedles()
	assertStringSlice(t, []string{pdf.aliasNeedleStrings[0], pdf.aliasNeedleStrings[2]}, []string{"alpha", "zeta"})
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(%v) = %d, want %d", got, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q in %v", i, got[i], want[i], got)
		}
	}
}

func TestRawWriteBufLatchesReaderErrors(t *testing.T) {
	want := errors.New("reader failed")
	pdf := mustNewPDFDocument()
	if err := pdf.RawWriteBufError(errReader{err: want}); !errors.Is(err, want) {
		t.Fatalf("RawWriteBufError() error = %v, want %v", err, want)
	}
	if !errors.Is(pdf.Error(), want) {
		t.Fatalf("document error = %v, want %v", pdf.Error(), want)
	}
}

func TestRawWriteArtifactBufLatchesReaderErrors(t *testing.T) {
	want := errors.New("artifact reader failed")
	pdf := mustNewPDFDocument()
	if err := pdf.RawWriteArtifactBufError(errReader{err: want}); !errors.Is(err, want) {
		t.Fatalf("RawWriteArtifactBufError() error = %v, want %v", err, want)
	}
	if !errors.Is(pdf.Error(), want) {
		t.Fatalf("document error = %v, want %v", pdf.Error(), want)
	}
}

func TestRawWriteStrErrorReturnsTaggedRestriction(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetComplianceMetadata(ComplianceMetadata{PDFUA2: true, Title: "Raw"})
	err := pdf.RawWriteStrError("0 0 m")
	if err == nil || !strings.Contains(err.Error(), "tagged PDF raw writes") {
		t.Fatalf("RawWriteStrError() error = %v, want tagged raw write rejection", err)
	}
}

func TestRawWriteArtifactStrErrorAllowsTaggedArtifacts(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetComplianceMetadata(ComplianceMetadata{PDFUA2: true, Title: "Artifact"})
	if err := pdf.RawWriteArtifactStrError("0 0 m"); err != nil {
		t.Fatalf("RawWriteArtifactStrError() error = %v", err)
	}
}

func TestPDFOutputSinkCountsAndHashesWrites(t *testing.T) {
	var out bytes.Buffer
	hash := sha256.New()
	sink := newPDFOutputSink(&out, 7, hash)

	if err := sink.WriteString("abc"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := sink.WriteByte('\n'); err != nil {
		t.Fatalf("WriteByte() error = %v", err)
	}
	if _, err := sink.ReadFrom(strings.NewReader("def")); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if got := sink.Len(); got != 14 {
		t.Fatalf("Len() = %d, want 14", got)
	}
	if got := out.String(); got != "abc\ndef" {
		t.Fatalf("output = %q, want abc\\ndef", got)
	}
	wantHash := sha256.Sum256([]byte("abc\ndef"))
	if !bytes.Equal(hash.Sum(nil), wantHash[:]) {
		t.Fatal("hash did not match written bytes")
	}
}

func TestPDFOutputSinkUsesSpecializedWriters(t *testing.T) {
	var out specializedOutputWriter
	sink := newPDFOutputSink(&out, 0, nil)

	if err := sink.WriteString("abc"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := sink.WriteByte('\n'); err != nil {
		t.Fatalf("WriteByte() error = %v", err)
	}
	if got := sink.Len(); got != 4 {
		t.Fatalf("Len() = %d, want 4", got)
	}
	if got := out.String(); got != "abc\n" {
		t.Fatalf("output = %q, want abc\\n", got)
	}
	if out.stringWrites != 1 {
		t.Fatalf("stringWrites = %d, want 1", out.stringWrites)
	}
	if out.byteWrites != 1 {
		t.Fatalf("byteWrites = %d, want 1", out.byteWrites)
	}
	if out.rawWrites != 0 {
		t.Fatalf("rawWrites = %d, want 0", out.rawWrites)
	}
}

type specializedOutputWriter struct {
	bytes.Buffer
	rawWrites    int
	stringWrites int
	byteWrites   int
}

func (w *specializedOutputWriter) Write(p []byte) (int, error) {
	w.rawWrites++
	return w.Buffer.Write(p)
}

func (w *specializedOutputWriter) WriteString(s string) (int, error) {
	w.stringWrites++
	return w.Buffer.WriteString(s)
}

func (w *specializedOutputWriter) WriteByte(b byte) error {
	w.byteWrites++
	return w.Buffer.WriteByte(b)
}

func TestPDFSyntaxBoundaryHelpersWriteExpectedTokens(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.beginPDFDict()
	pdf.endPDFDict()
	pdf.beginPDFStream()
	pdf.endPDFStream()
	pdf.endPDFObject()

	want := "<<\n>>\nstream\nendstream\nendobj\n"
	if got := pdf.buffer.String(); got != want {
		t.Fatalf("syntax helper output = %q, want %q", got, want)
	}
}

func TestPDFResourceNameHelpersWriteExpectedReferences(t *testing.T) {
	if got := fontPDFResourceName(fontDefinition{i: "abc"}).String(); got != "/Fabc" {
		t.Fatalf("fontPDFResourceName() = %q, want /Fabc", got)
	}
	if got := fontPDFResourceRef(fontDefinition{i: "abc", N: 3}); got.name != "/Fabc" || got.objectNumber != 3 {
		t.Fatalf("fontPDFResourceRef() = %#v, want /Fabc 3", got)
	}
	if got := imagePDFResourceName(&ImageInfo{i: "img"}).String(); got != "/Iimg" {
		t.Fatalf("imagePDFResourceName() = %q, want /Iimg", got)
	}
	if got := imagePDFResourceRef(&ImageInfo{i: "img", n: 4}); got.name != "/Iimg" || got.objectNumber != 4 {
		t.Fatalf("imagePDFResourceRef() = %#v, want /Iimg 4", got)
	}
	if got := graphicsStatePDFResourceName(2).String(); got != "/GS2" {
		t.Fatalf("graphicsStatePDFResourceName() = %q, want /GS2", got)
	}
	if got := graphicsStatePDFResourceRef(2, 10); got.name != "/GS2" || got.objectNumber != 10 {
		t.Fatalf("graphicsStatePDFResourceRef() = %#v, want /GS2 10", got)
	}
	if got := shadingPDFResourceName(3).String(); got != "/Sh3" {
		t.Fatalf("shadingPDFResourceName() = %q, want /Sh3", got)
	}
	if got := shadingPDFResourceRef(3, 11); got.name != "/Sh3" || got.objectNumber != 11 {
		t.Fatalf("shadingPDFResourceRef() = %#v, want /Sh3 11", got)
	}
	if got := spotColorPDFResourceName(4).String(); got != "/CS4" {
		t.Fatalf("spotColorPDFResourceName() = %q, want /CS4", got)
	}
	if got := spotColorPDFResourceRef(4, 12); got.name != "/CS4" || got.objectNumber != 12 {
		t.Fatalf("spotColorPDFResourceRef() = %#v, want /CS4 12", got)
	}
	if got := optionalContentPDFResourceName(5).String(); got != "/OC5" {
		t.Fatalf("optionalContentPDFResourceName() = %q, want /OC5", got)
	}
	if got := optionalContentPDFResourceRef(5, 13); got.name != "/OC5" || got.objectNumber != 13 {
		t.Fatalf("optionalContentPDFResourceRef() = %#v, want /OC5 13", got)
	}
	if got := string(appendPDFResourceRefValue(nil, pdfResourceRef{name: pdfResourceName("/TplA"), objectNumber: 9})); got != "/TplA 9 0 R" {
		t.Fatalf("appendPDFResourceRefValue() = %q, want /TplA 9 0 R", got)
	}
	if got := string(appendPDFResourceRef(nil, "/F", "abc", 3)); got != "/Fabc 3 0 R" {
		t.Fatalf("appendPDFResourceRef() = %q, want /Fabc 3 0 R", got)
	}
}

func TestOutputStreamContextDoesNotRetainFinalBuffer(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "streamed")

	var out chunkRecordingWriter
	if err := pdf.OutputStreamContext(context.Background(), &out); err != nil {
		t.Fatalf("OutputStreamContext() error = %v", err)
	}
	if len(out.chunks) < 2 {
		t.Fatalf("streaming output used %d writer call(s), want incremental final writes", len(out.chunks))
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Fatalf("streaming output wrote non-PDF prefix %q", out.Bytes()[:min(len(out.Bytes()), 8)])
	}
	if got := pdf.buffer.Len(); got != 0 {
		t.Fatalf("final buffer length after streaming = %d, want 0", got)
	}
	if !pdf.streamedOutput {
		t.Fatal("streamedOutput flag not set")
	}

	var retry bytes.Buffer
	if err := pdf.Output(&retry); !errors.Is(err, ErrStreamingOutputConsumed) {
		t.Fatalf("Output() after streaming error = %v, want ErrStreamingOutputConsumed", err)
	}
}

func TestOutputFileStreamWritesPDFWithoutRetainingFinalBuffer(t *testing.T) {
	fileStr := filepath.Join(t.TempDir(), "out.pdf")

	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "streamed file")

	if err := pdf.OutputFileStream(fileStr); err != nil {
		t.Fatalf("OutputFileStream() error = %v", err)
	}
	got, err := os.ReadFile(fileStr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-")) {
		t.Fatalf("OutputFileStream() wrote non-PDF prefix %q", got[:min(len(got), 8)])
	}
	if got := pdf.buffer.Len(); got != 0 {
		t.Fatalf("final buffer length after file streaming = %d, want 0", got)
	}
}

func TestOutputOptionsStreamFinalRoutesNormalOutput(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "option streamed")

	var out chunkRecordingWriter
	if err := pdf.OutputWithOptions(&out, OutputOptions{StreamFinal: true}); err != nil {
		t.Fatalf("OutputWithOptions(StreamFinal) error = %v", err)
	}
	if len(out.chunks) < 2 {
		t.Fatalf("stream-final output used %d writer call(s), want incremental final writes", len(out.chunks))
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Fatalf("stream-final output wrote non-PDF prefix %q", out.Bytes()[:min(len(out.Bytes()), 8)])
	}
	if got := pdf.buffer.Len(); got != 0 {
		t.Fatalf("final buffer length after stream-final output = %d, want 0", got)
	}
	if !pdf.streamedOutput {
		t.Fatal("streamedOutput flag not set")
	}

	var retry bytes.Buffer
	if err := pdf.Output(&retry); !errors.Is(err, ErrStreamingOutputConsumed) {
		t.Fatalf("Output() after stream-final output error = %v, want ErrStreamingOutputConsumed", err)
	}
}

func TestOutputPolicyStreamFinalRoutesOutputFile(t *testing.T) {
	fileStr := filepath.Join(t.TempDir(), "out.pdf")

	pdf, err := newPDFDocument(WithOutputPolicy(OutputPolicy{StreamFinal: true}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(20, 10, "policy streamed file")

	if err := pdf.OutputFile(fileStr); err != nil {
		t.Fatalf("OutputFile() with StreamFinal policy error = %v", err)
	}
	got, err := os.ReadFile(fileStr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-")) {
		t.Fatalf("OutputFile() with StreamFinal policy wrote non-PDF prefix %q", got[:min(len(got), 8)])
	}
	if got := pdf.buffer.Len(); got != 0 {
		t.Fatalf("final buffer length after policy stream-final file output = %d, want 0", got)
	}
	if !pdf.streamedOutput {
		t.Fatal("streamedOutput flag not set")
	}
}

type chunkRecordingWriter struct {
	chunks [][]byte
}

func (w *chunkRecordingWriter) Write(p []byte) (int, error) {
	w.chunks = append(w.chunks, append([]byte(nil), p...))
	return len(p), nil
}

func (w *chunkRecordingWriter) Bytes() []byte {
	var out []byte
	for _, chunk := range w.chunks {
		out = append(out, chunk...)
	}
	return out
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestGetImageInfoReturnsClone(t *testing.T) {
	pdf := mustNewPDFDocument()
	info := pdf.newImageInfo()
	info.w = 72
	info.h = 72
	resources := pdf.ensureResourceStore()
	resources.setImage("img", info)

	got := pdf.GetImageInfo("img")
	got.SetDpi(144)
	stored, _ := resources.image("img")
	if stored.dpi != 72 {
		t.Fatalf("registered image dpi = %.2f, want 72", stored.dpi)
	}
}

func TestLinkRequiresActivePage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.LinkString(1, 1, 10, 10, "https://example.com")
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "active page") {
		t.Fatalf("LinkString error = %v, want active page error", pdf.Error())
	}
}

func TestSetPageRejectsInvalidPageNumber(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SetPage(2)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "invalid page number") {
		t.Fatalf("SetPage error = %v, want invalid page number", pdf.Error())
	}
	if got := pdf.PageNo(); got != 1 {
		t.Fatalf("PageNo() = %d, want current page preserved", got)
	}
}

func TestSetDpiRejectsInvalidValues(t *testing.T) {
	info := (&pdfDocument{pageGeometryState: pageGeometryState{k: 1}}).newImageInfo()
	info.SetDpi(0)
	if info.dpi != 72 {
		t.Fatalf("SetDpi(0) changed dpi to %.2f", info.dpi)
	}
	info.SetDpi(math.Inf(1))
	if info.dpi != 72 {
		t.Fatalf("SetDpi(+Inf) changed dpi to %.2f", info.dpi)
	}
	info.SetDpi(144)
	if info.dpi != 144 {
		t.Fatalf("SetDpi(144) = %.2f, want 144", info.dpi)
	}
}

func TestRemoveReturnsUnchangedWhenKeyMissing(t *testing.T) {
	in := []int{1, 2, 3}
	got := remove(in, 4)
	if len(got) != len(in) || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("remove missing key = %#v, want %#v", got, in)
	}
}

func TestSetPageBoxRejectsInvalidExtent(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetPageBox("crop", 1, 1, 0, 10)
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "invalid page box") {
		t.Fatalf("SetPageBox error = %v", pdf.Error())
	}
}

func TestSetMinimumPDFVersionUsesNumericOrdering(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.pdfVersion = "1.10"
	pdf.setMinimumPDFVersion("1.9")
	if got := pdf.pdfVersion; got != "1.10" {
		t.Fatalf("pdf version = %q, want 1.10", got)
	}
	pdf.setMinimumPDFVersion("2.0")
	if got := pdf.pdfVersion; got != "2.0" {
		t.Fatalf("pdf version = %q, want 2.0", got)
	}
}
