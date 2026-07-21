// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"testing"
)

type architectureNilStringer struct{}

func (*architectureNilStringer) String() string {
	panic("typed nil Stringer must not be invoked")
}

func TestNilSafeHTMLAndParserBoundaries(t *testing.T) {
	pdf := mustNewPDFDocument()
	html := pdf.htmlNew()
	html.WriteCompiled(6, nil)
	if err := pdf.Error(); err == nil || err.Error() != "compiled HTML is nil" {
		t.Fatalf("WriteCompiled(nil) error = %v, want compiled HTML is nil", err)
	}

	length, ok := parseHTMLBoxLength("2mm", nil, 0)
	if !ok || math.Abs(length-2*72/25.4) > 1e-9 {
		t.Fatalf("parseHTMLBoxLength(2mm, nil) = (%v, %v)", length, ok)
	}
	rendered, err := renderCompiledHTMLTemplateValue((*architectureNilStringer)(nil))
	if err != nil || rendered != "" {
		t.Fatalf("typed nil Stringer rendered as %q with error %v, want empty string", rendered, err)
	}
}

func TestInternalSliceBoundariesRejectInvalidInputWithoutPanicking(t *testing.T) {
	font := &utf8FontFile{fileReader: &fileReader{}}
	if got := font.patchBytes(nil, 0, nil); got != nil || font.fileReader.err != nil {
		t.Fatalf("empty nil font patch = (%v, %v), want (nil, nil)", got, font.fileReader.err)
	}
	_ = font.patchBytes(nil, 0, []byte{1})
	if font.fileReader.err == nil {
		t.Fatal("non-empty nil font patch did not report an error")
	}

	normalizer := &svgPathNormalizer{}
	if err := normalizer.append('M', nil); err == nil {
		t.Fatal("SVG move command with no arguments was accepted")
	}
	if err := normalizer.append('Z', nil); err != nil {
		t.Fatalf("valid SVG close-path command failed: %v", err)
	}
}

func TestSymbolUsesItsOwnCoreFontAndMetrics(t *testing.T) {
	pdf := mustNewPDFDocument(WithNoCompression())
	pdf.AddPage()
	pdf.SetFont("Symbol", "BI", 12)
	if err := pdf.Error(); err != nil {
		t.Fatalf("SetFont(Symbol) error = %v", err)
	}
	if pdf.currentFont.Name != "Symbol" || pdf.fontFamily != "symbol" || pdf.fontStyle != "" {
		t.Fatalf("selected font = name:%q family:%q style:%q, want Symbol/symbol/empty", pdf.currentFont.Name, pdf.fontFamily, pdf.fontStyle)
	}
	for text, want := range map[string]int{" ": 250, "A": 722, "a": 631} {
		if got := pdf.GetStringSymbolWidth(text); got != want {
			t.Fatalf("Symbol width for %q = %d, want AFM width %d", text, got, want)
		}
	}
	pdf.Text(20, 20, "ABG")
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("/BaseFont /Symbol")) {
		t.Fatal("output does not contain the Symbol core font")
	}
	if bytes.Contains(output.Bytes(), []byte("/BaseFont /ZapfDingbats")) {
		t.Fatal("selecting Symbol unexpectedly emitted ZapfDingbats")
	}
	if bytes.Contains(output.Bytes(), []byte("/Encoding /WinAnsiEncoding")) {
		t.Fatal("Symbol core font unexpectedly emitted WinAnsiEncoding")
	}
}

func TestSetWordSpacingTracksStateGeometryAndPageTransitions(t *testing.T) {
	for _, spacing := range []float64{3.25, -2.5} {
		t.Run(fmt.Sprintf("spacing_%g", spacing), func(t *testing.T) {
			pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
			pdf.AddPage()
			pdf.SetFont("Helvetica", "", 12)
			pdf.SetWordSpacing(spacing)
			if pdf.ws != spacing {
				t.Fatalf("word spacing state = %g, want %g", pdf.ws, spacing)
			}

			text := "one two"
			baseWidth := pdf.GetStringWidth(text)
			pdf.CellFormat(100, 12, text, "", 0, "L", false, 0, "https://example.invalid")
			links := pdf.pageLinks[1]
			if len(links) != 1 {
				t.Fatalf("link count = %d, want 1", len(links))
			}
			if want := baseWidth + spacing; math.Abs(links[0].wd-want) > 0.0001 {
				t.Fatalf("link width = %g, want rendered text width %g", links[0].wd, want)
			}

			pdf.SetY(pdf.pageBreakTrigger - 1)
			pdf.CellFormat(100, 12, "page break", "", 0, "L", false, 0, "")
			if pdf.PageCount() != 2 {
				t.Fatalf("page count = %d, want automatic second page", pdf.PageCount())
			}
			if pdf.ws != spacing {
				t.Fatalf("word spacing after automatic page break = %g, want %g", pdf.ws, spacing)
			}
			command := appendPDFNumber(nil, spacing, 5)
			command = append(command, " Tw"...)
			if !bytes.Contains(pdf.pages[2].Bytes(), command) {
				t.Fatalf("second page is missing restored word-spacing command %q", command)
			}

			pdf.SetHeaderFunc(func() {
				pdf.SetWordSpacing(spacing * 2)
			})
			pdf.AddPage()
			if pdf.ws != spacing || !bytes.Contains(pdf.pages[3].Bytes(), command) {
				t.Fatalf("manual page transition and header did not preserve word spacing %g", spacing)
			}
		})
	}
}

func TestSetWordSpacingRejectsInvalidNumericValues(t *testing.T) {
	for _, spacing := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.MaxFloat64} {
		pdf := mustNewPDFDocument()
		pdf.AddPage()
		pdf.SetWordSpacing(1)
		pdf.SetWordSpacing(spacing)
		if pdf.Error() == nil {
			t.Fatalf("SetWordSpacing(%v) did not reject an invalid numeric value", spacing)
		}
		if pdf.ws != 1 {
			t.Fatalf("SetWordSpacing(%v) changed valid prior state to %v", spacing, pdf.ws)
		}
	}
}

func TestSetWordSpacingBeforePageAndFont(t *testing.T) {
	pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
	pdf.SetWordSpacing(1)
	if err := pdf.Error(); err != nil {
		t.Fatalf("SetWordSpacing() before selecting a font: %v", err)
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Text(20, 20, "A B")
	if err := pdf.Error(); err != nil {
		t.Fatalf("rendering with preconfigured word spacing: %v", err)
	}
	if !bytes.Contains(pdf.pages[1].Bytes(), []byte("1.00000 Tw")) {
		t.Fatalf("page content does not restore preconfigured word spacing:\n%s", pdf.pages[1].String())
	}
}

func TestSetWordSpacingAppliesToUTF8Text(t *testing.T) {
	pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
	pdf.AddUTF8FontFromBytes("fixture", "", readUTF8FontFixture(t))
	pdf.AddPage()
	pdf.SetFont("fixture", "", 10)
	pdf.SetWordSpacing(2)
	pdf.Text(20, 20, "A B")
	content := pdf.pages[1].Bytes()
	if !bytes.Contains(content, []byte("] TJ ET")) {
		t.Fatalf("UTF-8 Text did not emit a spaced TJ array:\n%s", content)
	}
	// A 2-point word spacing at 10-point font size is represented by a -200
	// displacement in a PDF TJ array.
	if !bytes.Contains(content, []byte(" -200.00000 ")) {
		t.Fatalf("UTF-8 Text did not emit the expected word-spacing displacement:\n%s", content)
	}

	pdf.RTL()
	x, y := 100.0, 30.0
	text := "A B"
	pdf.Text(x, y, text)
	wantPosition := appendPDFNumberSpace(nil, x-pdf.GetStringWidth(text)-2, 2)
	wantPosition = appendPDFNumberSpace(wantPosition, pdf.h-y, 2)
	wantPosition = append(wantPosition, "Td "...)
	if !bytes.Contains(pdf.pages[1].Bytes(), wantPosition) {
		t.Fatalf("RTL UTF-8 Text did not include word spacing in its origin; want %q in:\n%s", wantPosition, pdf.pages[1].Bytes())
	}

	pdf.LTR()
	pdf.SetXY(20, 40)
	pdf.CellFormat(100, 12, "A B", "", 0, "J", false, 0, "https://example.invalid/justified")
	links := pdf.pageLinks[1]
	if len(links) != 1 {
		t.Fatalf("justified UTF-8 link count = %d, want 1", len(links))
	}
	if want := 100 - 2*pdf.cMargin; math.Abs(links[0].wd-want) > 1e-9 {
		t.Fatalf("justified UTF-8 link width = %g, want rendered width %g", links[0].wd, want)
	}
}

func TestUTF8CellWordSpacingHandlesEdgeSpaces(t *testing.T) {
	for _, text := range []string{"A ", " A", "A  B", "   "} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
			pdf.AddUTF8FontFromBytes("fixture", "", readUTF8FontFixture(t))
			pdf.AddPage()
			pdf.SetFont("fixture", "", 10)
			pdf.SetWordSpacing(2)
			pdf.CellFormat(100, 12, text, "", 0, "L", false, 0, "")
			content := pdf.pages[1].Bytes()
			if !bytes.Contains(content, []byte("] TJ ET")) {
				t.Fatalf("UTF-8 text with ASCII spaces did not use a TJ array:\n%s", content)
			}
			if !bytes.Contains(content, []byte("-200.000")) {
				t.Fatalf("UTF-8 text did not apply the configured word spacing:\n%s", content)
			}
		})
	}
}

func TestMultiCellPreservesAndMeasuresWordSpacing(t *testing.T) {
	for _, utf8Font := range []bool{false, true} {
		for _, spacing := range []float64{6, -2} {
			name := fmt.Sprintf("utf8_%t_spacing_%g", utf8Font, spacing)
			t.Run(name, func(t *testing.T) {
				pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
				if utf8Font {
					pdf.AddUTF8FontFromBytes("fixture", "", readUTF8FontFixture(t))
				}
				pdf.AddPage()
				if utf8Font {
					pdf.SetFont("fixture", "", 10)
				} else {
					pdf.SetFont("Helvetica", "", 10)
				}
				pdf.SetWordSpacing(spacing)
				text := "A A A"
				width := pdf.GetStringWidth(text) + 2*pdf.cMargin + spacing*1.5
				if width <= 2*pdf.cMargin {
					width = pdf.GetStringWidth(text) + 2*pdf.cMargin
				}
				var wantLines int
				if utf8Font {
					wantLines = len(pdf.SplitText(text, width))
				} else {
					wantLines = len(pdf.SplitLines([]byte(text), width))
				}
				startY := pdf.GetY()
				pdf.MultiCell(width, 7, text, "", "L", false)
				gotLines := int(math.Round((pdf.GetY() - startY) / 7))
				if gotLines != wantLines {
					t.Fatalf("MultiCell lines = %d, split helper lines = %d", gotLines, wantLines)
				}
				if pdf.ws != spacing {
					t.Fatalf("word spacing after MultiCell = %g, want %g", pdf.ws, spacing)
				}

				pdf.MultiCell(width, 7, text, "", "J", false)
				if pdf.ws != spacing {
					t.Fatalf("word spacing after justified MultiCell = %g, want %g", pdf.ws, spacing)
				}
			})
		}
	}
}

func TestWordSpacingAffectsWriteAdvanceAndLineSplitting(t *testing.T) {
	const spacing = 6.0
	text := "A A"

	core := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
	core.AddPage()
	core.SetFont("Helvetica", "", 10)
	naturalWidth := core.GetStringWidth(text)
	core.SetWordSpacing(spacing)
	startX := core.GetX()
	core.Write(12, text)
	if want := startX + naturalWidth + spacing; math.Abs(core.GetX()-want) > 1e-9 {
		t.Fatalf("core Write advance = %g, want %g", core.GetX(), want)
	}
	wrapWidth := naturalWidth + 2*core.cMargin + spacing/2
	lines := core.SplitLines([]byte(text), wrapWidth)
	if len(lines) != 2 || core.SplitLineCount([]byte(text), wrapWidth) != len(lines) {
		t.Fatalf("word-spaced SplitLines = %q (count %d), want two matching lines", lines, core.SplitLineCount([]byte(text), wrapWidth))
	}

	unicodePDF := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
	unicodePDF.AddUTF8FontFromBytes("fixture", "", readUTF8FontFixture(t))
	unicodePDF.AddPage()
	unicodePDF.SetFont("fixture", "", 10)
	naturalWidth = unicodePDF.GetStringWidth(text)
	unicodePDF.SetWordSpacing(spacing)
	startX = unicodePDF.GetX()
	unicodePDF.Write(12, text)
	if want := startX + naturalWidth + spacing; math.Abs(unicodePDF.GetX()-want) > 1e-9 {
		t.Fatalf("UTF-8 Write advance = %g, want %g", unicodePDF.GetX(), want)
	}
	wrapWidth = naturalWidth + 2*unicodePDF.cMargin + spacing/2
	unicodeLines := unicodePDF.SplitText(text, wrapWidth)
	if len(unicodeLines) != 2 || unicodePDF.SplitTextCount(text, wrapWidth) != len(unicodeLines) {
		t.Fatalf("word-spaced SplitText = %q (count %d), want two matching lines", unicodeLines, unicodePDF.SplitTextCount(text, wrapWidth))
	}
}

func TestServerSafePolicyDoesNotUseGlobalUTF8SubsetCache(t *testing.T) {
	restoreUTF8SubsetCache := isolateUTF8SubsetCacheForTest()
	t.Cleanup(restoreUTF8SubsetCache)
	fontData := readUTF8FontFixture(t)

	build := func(options ...Option) {
		pdf := mustNewPDFDocument(options...)
		pdf.AddUTF8FontFromBytes("fixture", "", fontData)
		pdf.AddPage()
		pdf.SetFont("fixture", "", 12)
		pdf.Cell(100, 12, "isolated subset 世界")
		var output bytes.Buffer
		if err := pdf.Output(&output); err != nil {
			t.Fatalf("Output() error = %v", err)
		}
	}

	build(WithServerSafeDefaults(), WithNoCompression())
	utf8SubsetCache.Lock()
	serverEntries, serverBytes := len(utf8SubsetCache.entries), utf8SubsetCache.bytes
	utf8SubsetCache.Unlock()
	if serverEntries != 0 || serverBytes != 0 {
		t.Fatalf("server-safe document populated global subset cache: entries=%d bytes=%d", serverEntries, serverBytes)
	}

	build(WithNoCompression())
	utf8SubsetCache.Lock()
	sharedEntries, sharedBytes := len(utf8SubsetCache.entries), utf8SubsetCache.bytes
	utf8SubsetCache.Unlock()
	if sharedEntries == 0 || sharedBytes <= 0 || sharedBytes > maxUTF8SubsetCacheBytes {
		t.Fatalf("shared subset cache bounds = entries:%d bytes:%d", sharedEntries, sharedBytes)
	}
}

func TestUTF8SubsetCacheRejectsEntryOverByteBudget(t *testing.T) {
	restoreUTF8SubsetCache := isolateUTF8SubsetCacheForTest()
	t.Cleanup(restoreUTF8SubsetCache)
	key := utf8SubsetCacheKey{count: 1}
	storeUTF8SubsetCache(key, utf8SubsetCacheValue{data: make([]byte, maxUTF8SubsetCacheBytes+1)})
	utf8SubsetCache.Lock()
	defer utf8SubsetCache.Unlock()
	if len(utf8SubsetCache.entries) != 0 || utf8SubsetCache.bytes != 0 {
		t.Fatalf("oversized cache entry was retained: entries=%d bytes=%d", len(utf8SubsetCache.entries), utf8SubsetCache.bytes)
	}
}

func isolateUTF8SubsetCacheForTest() func() {
	utf8SubsetCache.Lock()
	entries := utf8SubsetCache.entries
	order := utf8SubsetCache.order
	cacheBytes := utf8SubsetCache.bytes
	utf8SubsetCache.entries = make(map[utf8SubsetCacheKey]utf8SubsetCacheValue)
	utf8SubsetCache.order = nil
	utf8SubsetCache.bytes = 0
	utf8SubsetCache.Unlock()
	return func() {
		utf8SubsetCache.Lock()
		utf8SubsetCache.entries = entries
		utf8SubsetCache.order = order
		utf8SubsetCache.bytes = cacheBytes
		utf8SubsetCache.Unlock()
	}
}
