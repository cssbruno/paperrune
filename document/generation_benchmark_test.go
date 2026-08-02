// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document_test

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/testsupport/example"
)

const benchmarkWorkerCount40 = 40

type benchmarkPDFOutput func(*document.TestPDFDocument, *bytes.Buffer) error

type benchmarkTableRow struct {
	label string
	value string
}

func benchmarkTableRows(count int, valueScale float64) []benchmarkTableRow {
	rows := make([]benchmarkTableRow, count)
	for row := range rows {
		rows[row] = benchmarkTableRow{
			label: fmt.Sprintf("Row %03d", row),
			value: fmt.Sprintf("%0.2f", float64(row)*valueScale),
		}
	}
	return rows
}

func benchmarkGeneratedPDF(b *testing.B, build func(*document.TestPDFDocument)) {
	b.Helper()
	benchmarkGeneratedPDFOutput(b, build, func(pdf *document.TestPDFDocument, output *bytes.Buffer) error {
		return pdf.Output(output)
	})
}

func benchmarkGeneratedPDFOutput(b *testing.B, build func(*document.TestPDFDocument), outputPDF benchmarkPDFOutput) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	var totalBytes int64

	for i := 0; i < b.N; i++ {
		pdf := document.MustNewTestPDFDocument()
		pdf.SetCompression(false)
		build(pdf)

		var output bytes.Buffer
		if err := outputPDF(pdf, &output); err != nil {
			b.Fatalf("output PDF error = %v", err)
		}
		if output.Len() == 0 {
			b.Fatal("generated empty PDF")
		}
		totalBytes += int64(output.Len())
	}
	elapsed := time.Since(start)
	b.StopTimer()
	reportBenchmarkThroughput(b, totalBytes, elapsed)
}

func benchmarkGeneratedPDFConcurrent(b *testing.B, workers int, build func(*document.TestPDFDocument)) {
	b.Helper()
	benchmarkGeneratedPDFOutputConcurrent(b, workers, build, func(pdf *document.TestPDFDocument, output *bytes.Buffer) error {
		return pdf.Output(output)
	})
}

func benchmarkGeneratedPDFConcurrent40(b *testing.B, build func(*document.TestPDFDocument)) {
	b.Helper()
	benchmarkGeneratedPDFConcurrent(b, benchmarkWorkerCount40, build)
}

func benchmarkGeneratedPDFOutputConcurrent(b *testing.B, workers int, build func(*document.TestPDFDocument), outputPDF benchmarkPDFOutput) {
	b.Helper()
	b.ReportAllocs()
	if workers < 1 {
		b.Fatalf("workers = %d, want >= 1", workers)
	}
	b.ReportMetric(float64(workers), "workers")

	jobs := make(chan struct{}, workers)
	var totalBytes atomic.Int64
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wg.Done()
			for range jobs {
				pdf := document.MustNewTestPDFDocument()
				pdf.SetCompression(false)
				build(pdf)

				var output bytes.Buffer
				if err := outputPDF(pdf, &output); err != nil {
					setErr(fmt.Errorf("output PDF error = %w", err))
					continue
				}
				if output.Len() == 0 {
					setErr(fmt.Errorf("generated empty PDF"))
					continue
				}
				totalBytes.Add(int64(output.Len()))
			}
		}()
	}

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		jobs <- struct{}{}
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	b.StopTimer()
	reportBenchmarkThroughput(b, totalBytes.Load(), elapsed)

	if firstErr != nil {
		b.Fatal(firstErr)
	}
}

func reportBenchmarkThroughput(b *testing.B, totalBytes int64, elapsed time.Duration) {
	b.Helper()
	if b.N == 0 || elapsed <= 0 {
		return
	}
	b.ReportMetric(float64(totalBytes)/float64(b.N), "pdf_bytes")
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "pdf/s")
}

func BenchmarkGenerationText(b *testing.B) {
	benchmarkGeneratedPDF(b, benchmarkGenerationTextBuilder())
}

func BenchmarkGenerationTextConcurrent40(b *testing.B) {
	benchmarkGeneratedPDFConcurrent40(b, benchmarkGenerationTextBuilder())
}

func benchmarkGenerationTextBuilder() func(*document.TestPDFDocument) {
	return func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 10)
		for row := 0; row < 180; row++ {
			pdf.CellFormat(40, 6, fmt.Sprintf("Row %03d", row), "1", 0, "L", false, 0, "")
			pdf.CellFormat(80, 6, "Operational PDF generation benchmark", "1", 0, "L", false, 0, "")
			pdf.CellFormat(40, 6, fmt.Sprintf("%0.2f", float64(row)*1.25), "1", 1, "R", false, 0, "")
		}
	}
}

func BenchmarkGenerationLongText(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("20k_c1.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}

	build := func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Times", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "J", false)
	}
	benchmarkGeneratedPDF(b, build)
}

func BenchmarkGenerationLongTextConcurrent40(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("20k_c1.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}

	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Times", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "J", false)
	})
}

func BenchmarkGenerationBaselineNoCompliance(b *testing.B) {
	benchmarkGenerationBaselineNoCompliance(b, nil)
}

func BenchmarkGenerationBaselineNoComplianceConcurrent40(b *testing.B) {
	benchmarkGeneratedPDFConcurrent40(b, benchmarkGenerationBaselineNoComplianceBuilder(b, nil, true))
}

func BenchmarkGenerationBaselineNoComplianceNoImage(b *testing.B) {
	benchmarkGeneratedPDF(b, benchmarkGenerationBaselineNoComplianceBuilder(b, nil, false))
}

func BenchmarkGenerationBaselineNoComplianceNoImageConcurrent40(b *testing.B) {
	benchmarkGeneratedPDFConcurrent40(b, benchmarkGenerationBaselineNoComplianceBuilder(b, nil, false))
}

func BenchmarkGenerationBaselineNoComplianceCachedImage(b *testing.B) {
	cache := document.NewImageCache()
	if _, err := cache.RegisterImageOptions("logo.png", example.ImageFile("logo.png"), document.ImageOptions{}); err != nil {
		b.Fatalf("RegisterImageOptions(logo.png) error = %v", err)
	}
	benchmarkGenerationBaselineNoCompliance(b, cache)
}

func BenchmarkGenerationBaselineNoComplianceCachedImageConcurrent40(b *testing.B) {
	cache := document.NewImageCache()
	if _, err := cache.RegisterImageOptions("logo.png", example.ImageFile("logo.png"), document.ImageOptions{}); err != nil {
		b.Fatalf("RegisterImageOptions(logo.png) error = %v", err)
	}
	benchmarkGeneratedPDFConcurrent40(b, benchmarkGenerationBaselineNoComplianceBuilder(b, cache, true))
}

func BenchmarkGenerationPDFA4FCompliance(b *testing.B) {
	benchmarkGeneratedPDF(b, benchmarkGenerationPDFA4FComplianceBuilder(b))
}

func BenchmarkGenerationPDFA4FComplianceConcurrent40(b *testing.B) {
	benchmarkGeneratedPDFConcurrent40(b, benchmarkGenerationPDFA4FComplianceBuilder(b))
}

func benchmarkGenerationBaselineNoCompliance(b *testing.B, imageCache *document.ImageCache) {
	benchmarkGeneratedPDF(b, benchmarkGenerationBaselineNoComplianceBuilder(b, imageCache, true))
}

func benchmarkGenerationBaselineNoComplianceBuilder(b *testing.B, imageCache *document.ImageCache, includeImages bool) func(*document.TestPDFDocument) {
	source, err := os.ReadFile("paper.go")
	if err != nil {
		b.Fatalf("ReadFile(paper.go) error = %v", err)
	}
	pageTitles := make([]string, 3)
	for page := range pageTitles {
		pageTitles[page] = fmt.Sprintf("Operational report page %d", page+1)
	}
	rows := make([]benchmarkTableRow, 3*48)
	for row := range rows {
		rows[row] = benchmarkTableRow{
			label: fmt.Sprintf("Item %03d", row),
			value: fmt.Sprintf("%0.2f", float64(row%48+1)*2.35),
		}
	}

	return func(pdf *document.TestPDFDocument) {
		pdf.SetTitle("Baseline no-compliance benchmark", false)
		pdf.SetSubject("PDF generation without PDF/A, PDF/UA, Arlington, or XMP metadata", false)
		pdf.SetAuthor("PaperRune benchmark", false)
		pdf.SetCreator("PaperRune benchmark", false)
		pdf.SetKeywords("benchmark baseline no-compliance", false)
		pdf.SetAttachments([]document.Attachment{{
			Content:     source[:min(len(source), 4096)],
			Filename:    "paper.go",
			Description: "Small benchmark attachment",
		}})

		for page := 0; page < 3; page++ {
			pdf.AddPage()
			pdf.SetFont("Helvetica", "B", 12)
			pdf.SetFillColor(235, 239, 244)
			pdf.Rect(0, 0, 180, 16, "F")
			pdf.Text(6, 10, "PaperRune baseline")
			pdf.SetXY(12, 24)
			pdf.SetFont("Helvetica", "B", 13)
			pdf.CellFormat(0, 8, pageTitles[page], "", 1, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			for row := 0; row < 48; row++ {
				tableRow := rows[page*48+row]
				pdf.CellFormat(26, 5, tableRow.label, "1", 0, "L", false, 0, "")
				pdf.CellFormat(88, 5, "Baseline PDF output without standards validation layers", "1", 0, "L", false, 0, "")
				pdf.CellFormat(24, 5, tableRow.value, "1", 0, "R", false, 0, "")
				pdf.CellFormat(28, 5, "ready", "1", 1, "C", false, 0, "")
			}
			if !includeImages {
				continue
			}
			pdf.SetXY(14, 270)
			if imageCache == nil {
				pdf.ImageOptions(example.ImageFile("logo.png"), 14, 270, 10, 0, false, document.ImageOptions{}, 0, "")
			} else {
				pdf.ImageFromCache("logo.png", imageCache, 14, 270, 10, 0, false, document.ImageOptions{}, 0, "")
			}
		}
	}
}

func benchmarkGenerationPDFA4FComplianceBuilder(b *testing.B) func(*document.TestPDFDocument) {
	cache := benchmarkComplianceFontCache(b)
	icc := []byte("benchmark sRGB ICC placeholder for generation-only benchmark")

	return func(pdf *document.TestPDFDocument) {
		pdf.SetComplianceMetadata(document.ComplianceMetadata{
			PDFA:       document.PDFAMode4F,
			Lang:       "en-US",
			Title:      "PDF/A-4f compliance benchmark",
			Identifier: "urn:uuid:paperrune-benchmark-pdfa4f",
		})
		if err := pdf.SetOutputIntent(icc, "sRGB IEC61966-2.1"); err != nil {
			b.Fatalf("SetOutputIntent() error = %v", err)
		}
		pdf.AddUTF8FontFromCache("DejaVu", "", cache)
		pdf.AddUTF8FontFromCache("DejaVu", "B", cache)
		pdf.AddUTF8FontFromCache("DejaVu", "I", cache)
		pdf.AddUTF8FontFromCache("DejaVu", "BI", cache)
		pdf.SetAttachments([]document.Attachment{{
			Filename:       "benchmark.txt",
			Description:    "PDF/A-4f benchmark attachment",
			MIMEType:       "text/plain",
			AFRelationship: "Data",
			Content:        []byte("Small benchmark attachment for PDF/A-4f generation."),
		}})
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 10)
		for row := 0; row < 96; row++ {
			pdf.CellFormat(38, 5, fmt.Sprintf("Row %03d", row), "1", 0, "L", false, 0, "")
			pdf.CellFormat(104, 5, "PDF/A-4f metadata, output intent, UTF-8 font, and attachment", "1", 0, "L", false, 0, "")
			pdf.CellFormat(24, 5, fmt.Sprintf("%0.2f", float64(row)*1.7), "1", 1, "R", false, 0, "")
		}
	}
}

func benchmarkComplianceFontCache(b *testing.B) *document.FontCache {
	b.Helper()
	fontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	if err != nil {
		b.Fatalf("ReadFile(font) error = %v", err)
	}
	boldFontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed-Bold.ttf"))
	if err != nil {
		b.Fatalf("ReadFile(bold font) error = %v", err)
	}
	italicFontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed-Oblique.ttf"))
	if err != nil {
		b.Fatalf("ReadFile(italic font) error = %v", err)
	}
	boldItalicFontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed-BoldOblique.ttf"))
	if err != nil {
		b.Fatalf("ReadFile(bold italic font) error = %v", err)
	}
	cache := document.NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes() error = %v", err)
	}
	if err := cache.AddUTF8FontFromBytes("DejaVu", "B", boldFontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes(bold) error = %v", err)
	}
	if err := cache.AddUTF8FontFromBytes("DejaVu", "I", italicFontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes(italic) error = %v", err)
	}
	if err := cache.AddUTF8FontFromBytes("DejaVu", "BI", boldItalicFontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes(bold italic) error = %v", err)
	}
	return cache
}

func BenchmarkGenerationUTF8Text(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("utf-8test.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}

	build := func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8Font("DejaVu", "", example.FontFile("DejaVuSansCondensed.ttf"))
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "L", false)
	}
	benchmarkGeneratedPDF(b, build)
}

func BenchmarkGenerationUTF8TextConcurrent40(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("utf-8test.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}

	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8Font("DejaVu", "", example.FontFile("DejaVuSansCondensed.ttf"))
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "L", false)
	})
}

func BenchmarkGenerationUTF8TextCachedFont(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("utf-8test.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}
	fontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}
	cache := document.NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes() error = %v", err)
	}

	build := func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8FontFromCache("DejaVu", "", cache)
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "L", false)
	}
	benchmarkGeneratedPDF(b, build)
}

func BenchmarkGenerationUTF8TextCachedFontConcurrent40(b *testing.B) {
	text, err := os.ReadFile(example.TextFile("utf-8test.txt"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}
	fontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}
	cache := document.NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes() error = %v", err)
	}

	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8FontFromCache("DejaVu", "", cache)
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 11)
		pdf.MultiCell(0, 5, string(text), "", "L", false)
	})
}

func BenchmarkGenerationTextCompressionLevel(b *testing.B) {
	rows := benchmarkTableRows(180, 1.25)
	for _, tc := range []struct {
		name  string
		level int
	}{
		{name: "BestSpeed", level: zlib.BestSpeed},
		{name: "BestCompression", level: zlib.BestCompression},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pdf := document.MustNewTestPDFDocument()
				pdf.SetCompressionLevel(tc.level)
				pdf.AddPage()
				pdf.SetFont("Arial", "", 10)
				for _, row := range rows {
					pdf.CellFormat(40, 6, row.label, "1", 0, "L", false, 0, "")
					pdf.CellFormat(80, 6, "Operational PDF generation benchmark", "1", 0, "L", false, 0, "")
					pdf.CellFormat(40, 6, row.value, "1", 1, "R", false, 0, "")
				}
				var output bytes.Buffer
				if err := pdf.Output(&output); err != nil {
					b.Fatalf("Output() error = %v", err)
				}
				if output.Len() == 0 {
					b.Fatal("generated empty PDF")
				}
			}
			b.StopTimer()
		})
	}
}

func BenchmarkGenerationTextCompressionLevelConcurrent40(b *testing.B) {
	rows := benchmarkTableRows(180, 1.25)
	for _, tc := range []struct {
		name  string
		level int
	}{
		{name: "BestSpeed", level: zlib.BestSpeed},
		{name: "BestCompression", level: zlib.BestCompression},
	} {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
				pdf.SetCompressionLevel(tc.level)
				pdf.AddPage()
				pdf.SetFont("Arial", "", 10)
				for _, row := range rows {
					pdf.CellFormat(40, 6, row.label, "1", 0, "L", false, 0, "")
					pdf.CellFormat(80, 6, "Operational PDF generation benchmark", "1", 0, "L", false, 0, "")
					pdf.CellFormat(40, 6, row.value, "1", 1, "R", false, 0, "")
				}
			})
		})
	}
}

func BenchmarkGenerationImages(b *testing.B) {
	images := benchmarkImageFiles(b)
	benchmarkGeneratedPDF(b, func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 10)
		for i, image := range images {
			x := 10 + float64(i%2)*90
			y := 15 + float64(i/2)*70
			pdf.RegisterImageOptionsReader(image.name, document.ImageOptions{ImageType: image.imageType}, bytes.NewReader(image.data))
			pdf.ImageOptions(image.name, x, y, 60, 0, false, document.ImageOptions{}, 0, "")
			pdf.Text(x, y+50, image.name)
		}
	})
}

func BenchmarkGenerationImagesConcurrent40(b *testing.B) {
	images := benchmarkImageFiles(b)
	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 10)
		for i, image := range images {
			x := 10 + float64(i%2)*90
			y := 15 + float64(i/2)*70
			pdf.RegisterImageOptionsReader(image.name, document.ImageOptions{ImageType: image.imageType}, bytes.NewReader(image.data))
			pdf.ImageOptions(image.name, x, y, 60, 0, false, document.ImageOptions{}, 0, "")
			pdf.Text(x, y+50, image.name)
		}
	})
}

func BenchmarkGenerationImagesCached(b *testing.B) {
	cache := document.NewImageCache()
	images := benchmarkImageFiles(b)
	for _, image := range images {
		if _, err := cache.RegisterImageOptionsReader(image.name, document.ImageOptions{ImageType: image.imageType}, bytes.NewReader(image.data)); err != nil {
			b.Fatalf("RegisterImageOptions(%s) error = %v", image.name, err)
		}
	}

	benchmarkGeneratedPDF(b, func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 10)
		for i, image := range images {
			x := 10 + float64(i%2)*90
			y := 15 + float64(i/2)*70
			pdf.ImageFromCache(image.name, cache, x, y, 60, 0, false, document.ImageOptions{}, 0, "")
			pdf.Text(x, y+50, image.name)
		}
	})
}

func BenchmarkGenerationImagesCachedConcurrent40(b *testing.B) {
	cache := document.NewImageCache()
	images := benchmarkImageFiles(b)
	for _, image := range images {
		if _, err := cache.RegisterImageOptionsReader(image.name, document.ImageOptions{ImageType: image.imageType}, bytes.NewReader(image.data)); err != nil {
			b.Fatalf("RegisterImageOptions(%s) error = %v", image.name, err)
		}
	}

	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 10)
		for i, image := range images {
			x := 10 + float64(i%2)*90
			y := 15 + float64(i/2)*70
			pdf.ImageFromCache(image.name, cache, x, y, 60, 0, false, document.ImageOptions{}, 0, "")
			pdf.Text(x, y+50, image.name)
		}
	})
}

type benchmarkImageFile struct {
	name      string
	imageType string
	data      []byte
}

func benchmarkImageFiles(b *testing.B) []benchmarkImageFile {
	b.Helper()
	names := []string{"logo.png", "logo.jpg", "logo.gif", "logo-rgb.png"}
	images := make([]benchmarkImageFile, len(names))
	for i, name := range names {
		data, err := os.ReadFile(example.ImageFile(name))
		if err != nil {
			b.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		imageType := name[strings.LastIndex(name, ".")+1:]
		images[i] = benchmarkImageFile{name: name, imageType: imageType, data: data}
	}
	return images
}

func BenchmarkGenerationAttachments(b *testing.B) {
	source, err := os.ReadFile("paper.go")
	if err != nil {
		b.Fatalf("ReadFile(paper.go) error = %v", err)
	}
	license, err := os.ReadFile("../LICENSE")
	if err != nil {
		b.Fatalf("ReadFile(LICENSE) error = %v", err)
	}

	benchmarkGeneratedPDF(b, func(pdf *document.TestPDFDocument) {
		attachments := []document.Attachment{
			{Content: source, Filename: "paper.go", Description: "Paper source"},
			{Content: license, Filename: "LICENSE", Description: "License text"},
		}
		pdf.SetAttachments(attachments)
		pdf.AddPage()
		pdf.SetFont("Arial", "", 12)
		for i, attachment := range attachments {
			y := 20 + float64(i)*30
			pdf.SetXY(15, y)
			pdf.Cell(70, 10, strings.TrimSpace(attachment.Description))
			pdf.Rect(12, y-2, 80, 14, "D")
			pdf.AddAttachmentAnnotation(&attachments[i], 12, y-2, 80, 14)
		}
	})
}

func BenchmarkGenerationAttachmentsConcurrent40(b *testing.B) {
	source, err := os.ReadFile("paper.go")
	if err != nil {
		b.Fatalf("ReadFile(paper.go) error = %v", err)
	}
	license, err := os.ReadFile("../LICENSE")
	if err != nil {
		b.Fatalf("ReadFile(LICENSE) error = %v", err)
	}

	benchmarkGeneratedPDFConcurrent40(b, func(pdf *document.TestPDFDocument) {
		attachments := []document.Attachment{
			{Content: source, Filename: "paper.go", Description: "Paper source"},
			{Content: license, Filename: "LICENSE", Description: "License text"},
		}
		pdf.SetAttachments(attachments)
		pdf.AddPage()
		pdf.SetFont("Arial", "", 12)
		for i, attachment := range attachments {
			y := 20 + float64(i)*30
			pdf.SetXY(15, y)
			pdf.Cell(70, 10, strings.TrimSpace(attachment.Description))
			pdf.Rect(12, y-2, 80, 14, "D")
			pdf.AddAttachmentAnnotation(&attachments[i], 12, y-2, 80, 14)
		}
	})
}
