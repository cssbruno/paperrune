// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func TestStringWidthCacheUsesBoundedRing(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetFont("Helvetica", "", 12)
	for i := 0; i < stringWidthCacheLimit+16; i++ {
		pdf.GetStringSymbolWidth(fmt.Sprintf("cache-key-%03d", i))
	}
	if got := len(pdf.stringWidthCache); got != stringWidthCacheLimit {
		t.Fatalf("string width cache size = %d, want %d", got, stringWidthCacheLimit)
	}
	if got := len(pdf.stringWidthKeys); got != stringWidthCacheLimit {
		t.Fatalf("string width key ring size = %d, want %d", got, stringWidthCacheLimit)
	}
	if got := pdf.stringWidthKeyNext; got != 16 {
		t.Fatalf("string width key ring next = %d, want 16", got)
	}
	first, _ := pdf.stringWidthCacheKey("cache-key-000")
	if _, ok := pdf.stringWidthCache[first]; ok {
		t.Fatal("oldest string width entry was not evicted")
	}
	recent, _ := pdf.stringWidthCacheKey(fmt.Sprintf("cache-key-%03d", stringWidthCacheLimit+15))
	if _, ok := pdf.stringWidthCache[recent]; !ok {
		t.Fatal("most recent string width entry is missing")
	}
}

func TestContentCommandBufferReuseIsBounded(t *testing.T) {
	pdf := mustNewPDFDocument()
	buffer := pdf.contentCommandBuffer(128)
	buffer = append(buffer, make([]byte, 128)...)
	pdf.retainContentCommandBuffer(buffer)
	retainedCapacity := cap(pdf.contentScratch)
	if retainedCapacity < 128 {
		t.Fatalf("retained content scratch capacity = %d, want at least 128", retainedCapacity)
	}
	if reused := pdf.contentCommandBuffer(64); cap(reused) != retainedCapacity {
		t.Fatalf("reused content scratch capacity = %d, want %d", cap(reused), retainedCapacity)
	}

	oversized := make([]byte, 0, maxContentScratchCapacity+1)
	pdf.retainContentCommandBuffer(oversized)
	if got := cap(pdf.contentScratch); got != retainedCapacity {
		t.Fatalf("oversized content scratch changed retained capacity to %d, want %d", got, retainedCapacity)
	}
}

func TestPageContentCommandBufferWritesIntoCurrentPage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	before := pdf.pages[pdf.page].String()
	buffer := pdf.pageContentCommandBuffer(64)
	buffer = append(buffer, "BT /F1 12 Tf ET"...)
	pdf.outbytes(buffer)
	if got, want := pdf.pages[pdf.page].String(), before+"BT /F1 12 Tf ET\n"; got != want {
		t.Fatalf("page content = %q, want %q", got, want)
	}
}

func TestPlannedFixedAndColorFormattingMatchesCanonicalPDFNumbers(t *testing.T) {
	values := []layoutengine.Fixed{
		layoutengine.MinFixed, -layoutengine.Fixed(1<<53) - 1,
		-1_000_000_000, layoutengine.Fixed(-12*layoutengine.FixedScale - 1), layoutengine.Fixed(-layoutengine.FixedScale),
		-1, 0, 1, layoutengine.Fixed(layoutengine.FixedScale), layoutengine.Fixed(12*layoutengine.FixedScale + 1), 1_000_000_000,
		layoutengine.Fixed(1<<53) + 1, layoutengine.MaxFixed,
	}
	for _, value := range values {
		got := string(appendPDFFixed(nil, value))
		want := strconv.FormatFloat(value.Points(), 'f', 10, 64)
		if got != want {
			t.Fatalf("fixed %d = %q, want %q", value, got, want)
		}
	}
	for value := 0; value <= 255; value++ {
		got := string(appendPDFColorComponentSpace(nil, uint8(value)))
		want := strconv.FormatFloat(float64(value)/255, 'f', 10, 64) + " "
		if got != want {
			t.Fatalf("color %d = %q, want %q", value, got, want)
		}
	}
}

func BenchmarkPerfUTF8ToUTF16(b *testing.B) {
	text := strings.Repeat("ASCII Ελληνικά こんにちは 😀 ", 64)
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = utf8toutf16(text, false)
	}
}

var (
	benchmarkTaggedKidsSink []byte
)

func BenchmarkPerfTaggedElementKidsLarge(b *testing.B) {
	pdf := &pdfDocument{}
	pdf.tagged.pageObjNums = make([]int, 65)
	for i := range pdf.tagged.pageObjNums {
		pdf.tagged.pageObjNums[i] = 1000 + i
	}
	elem := &taggedElement{
		Page:   1,
		MCID:   0,
		ObjRef: 9001,
	}
	for i := 1; i <= 256; i++ {
		elem.Marked = append(elem.Marked, taggedMarkedContent{Page: i%64 + 1, MCID: i})
		elem.Children = append(elem.Children, &taggedElement{ObjNum: 2000 + i})
	}
	kidCount := taggedElementKidCount(elem)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := make([]byte, 0, 8+kidCount*32)
		out = append(out, "/K ["...)
		out = pdf.appendTaggedElementKids(out, elem, true)
		out = append(out, ']')
		benchmarkTaggedKidsSink = out
	}
}

func BenchmarkPerfReplaceAliasesManyPages(b *testing.B) {
	const pages = 50
	const aliases = 20

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		pdf := benchmarkAliasPDF(pages, aliases)
		b.StartTimer()

		pdf.replaceAliases()
	}
}

func BenchmarkPerfReplaceAliasesNoMatchesManyPages(b *testing.B) {
	const pages = 50
	const aliases = 20

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		pdf := benchmarkNoMatchAliasPDF(pages, aliases)
		b.StartTimer()

		pdf.replaceAliases()
	}
}

func benchmarkAliasPDF(pages, aliases int) *pdfDocument {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetFont("Helvetica", "", 10)
	for i := 0; i < aliases; i++ {
		pdf.RegisterAlias(fmt.Sprintf("{mark %d}", i), strconv.Itoa(i+1))
	}
	for page := 0; page < pages; page++ {
		pdf.AddPage()
		for row := 0; row < 40; row++ {
			for i := 0; i < aliases; i++ {
				pdf.Cell(8, 4, fmt.Sprintf("{mark %d}", i))
			}
			pdf.Ln(4)
		}
	}
	return pdf
}

func benchmarkNoMatchAliasPDF(pages, aliases int) *pdfDocument {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetFont("Helvetica", "", 10)
	for i := 0; i < aliases; i++ {
		pdf.RegisterAlias(fmt.Sprintf("{mark %d}", i), strconv.Itoa(i+1))
	}
	for page := 0; page < pages; page++ {
		pdf.AddPage()
		for row := 0; row < 40; row++ {
			pdf.Cell(80, 4, "plain report row without page markers")
			pdf.Ln(4)
		}
	}
	return pdf
}

func BenchmarkPerfRegisterImageOptionsReaderPNGAlpha(b *testing.B) {
	data := benchmarkAlphaPNG(b, 128, 128)
	options := ImageOptions{ImageType: "png"}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pdf := mustNewPDFDocument()
		pdf.RegisterImageOptionsReader("alpha.png", options, bytes.NewReader(data))
		if !pdf.Ok() {
			b.Fatalf("RegisterImageOptionsReader() error = %v", pdf.Error())
		}
	}
}

func benchmarkAlphaPNG(tb testing.TB, width, height int) []byte {
	tb.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x),
				G: uint8(y),
				B: uint8(x + y),
				A: uint8((x*y)%255 + 1),
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func BenchmarkPerfAddUTF8FontFromCache(b *testing.B) {
	fontBytes, err := os.ReadFile("../assets/static/font/DejaVuSansCondensed.ttf")
	if err != nil {
		b.Fatalf("ReadFile() error = %v", err)
	}
	cache := NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		b.Fatalf("AddUTF8FontFromBytes() error = %v", err)
	}

	b.SetBytes(int64(len(fontBytes)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pdf := mustNewPDFDocument()
		pdf.AddUTF8FontFromCache("DejaVu", "", cache)
		if !pdf.Ok() {
			b.Fatalf("AddUTF8FontFromCache() error = %v", pdf.Error())
		}
	}
}
