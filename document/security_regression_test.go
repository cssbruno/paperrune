// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestSecurityMalformedUTF8DoesNotPanic(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Output panicked: %v", r)
		}
	}()

	pdf.SetTitle(string([]byte{0xe2}), true)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(10, 10, "ok")

	if err := pdf.Output(&bytes.Buffer{}); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
}

func TestSecurityPNGAlphaOverflowReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterImageOptionsReader panicked: %v", r)
		}
	}()

	_ = pdf.RegisterImageOptionsReader("huge-alpha", ImageOptions{ImageType: "png"}, bytes.NewReader(securityPNG(
		0x7fffffff,
		0x7fffffff,
		6,
		securityPNGChunk("IDAT", securityZlibBytes(nil)),
		securityPNGChunk("IEND", nil),
	)))
	if pdf.Error() == nil {
		t.Fatal("expected invalid PNG alpha channel size error")
	}
}

func TestSecurityPNGAlphaDecodedLimitReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterImageOptionsReader panicked: %v", r)
		}
	}()

	_ = pdf.RegisterImageOptionsReader("large-alpha", ImageOptions{ImageType: "png"}, bytes.NewReader(securityPNG(
		100000,
		1000,
		6,
		securityPNGChunk("IDAT", securityZlibBytes(nil)),
		securityPNGChunk("IEND", nil),
	)))
	if pdf.Error() == nil {
		t.Fatal("expected PNG alpha decoded-size error")
	}
}

func TestSecurityPNGNonAlphaPixelLimitReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterImageOptionsReader panicked: %v", r)
		}
	}()

	_ = pdf.RegisterImageOptionsReader("large-rgb", ImageOptions{ImageType: "png"}, bytes.NewReader(securityPNG(
		10000,
		6000,
		2,
		securityPNGChunk("IDAT", securityZlibBytes([]byte{0})),
		securityPNGChunk("IEND", nil),
	)))
	if pdf.Error() == nil {
		t.Fatal("expected PNG pixel-count limit error")
	}
}

func TestSecurityPNGDimensionLimitReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterImageOptionsReader panicked: %v", r)
		}
	}()

	_ = pdf.RegisterImageOptionsReader("wide-rgb", ImageOptions{ImageType: "png"}, bytes.NewReader(securityPNG(
		uint32(maxImageDimension+1),
		1,
		2,
		securityPNGChunk("IDAT", securityZlibBytes([]byte{0})),
		securityPNGChunk("IEND", nil),
	)))
	if pdf.Error() == nil {
		t.Fatal("expected PNG dimension limit error")
	}
}

func TestSecurityOversizedGIFDimensionsRejected(t *testing.T) {
	pdf := mustNewPDFDocument()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterImageOptionsReader panicked: %v", r)
		}
	}()

	_ = pdf.RegisterImageOptionsReader("huge-gif", ImageOptions{ImageType: "gif"}, bytes.NewReader(securityGIF(65535, 65535)))
	if pdf.Error() == nil {
		t.Fatal("expected GIF dimension limit error")
	}
}

func TestSecurityMaskImageFileSizeLimit(t *testing.T) {
	maskPath := filepath.Join(t.TempDir(), "oversized-mask.png")
	file, err := os.Create(maskPath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := file.Truncate(int64(maxImageSourceBytes) + 1); err != nil {
		_ = file.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	pdf := mustNewPDFDocument()
	pdf.applyExternalImageMask(&ImageInfo{w: 1, h: 1}, maskPath, ImageOptions{ImageType: "png"})
	if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "image data exceeds maximum size") {
		t.Fatalf("mask error = %v, want image data size limit", pdf.Error())
	}
}

func TestSecurityFontCacheFileSizeLimit(t *testing.T) {
	fontPath := filepath.Join(t.TempDir(), "oversized.ttf")
	file, err := os.Create(fontPath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := file.Truncate(int64(maxFontSourceBytes) + 1); err != nil {
		_ = file.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	err = NewFontCache().AddUTF8Font("bad", "", fontPath)
	if err == nil || !strings.Contains(err.Error(), "font data exceeds maximum size") {
		t.Fatalf("AddUTF8Font() error = %v, want font data size limit", err)
	}
}

func TestSecurityInvalidLinkIDReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.Link(10, 10, 20, 20, 999)
	if pdf.Error() == nil {
		t.Fatal("expected invalid link id error")
	}
}

func TestSecurityValidLinkIDStillWorks(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	link := pdf.AddLink()
	pdf.SetLink(link, 10, 1)
	pdf.Link(10, 10, 20, 20, link)
	if err := pdf.Output(&bytes.Buffer{}); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
}

func TestSecurityInvalidLinkDestinationPageReturnsError(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	link := pdf.AddLink()
	pdf.SetLink(link, 10, 99)
	if pdf.Error() == nil {
		t.Fatal("expected invalid link destination page error")
	}
}

func TestSecurityDirectLinksRejectUnsafeSchemes(t *testing.T) {
	tests := []struct {
		name string
		run  func(*pdfDocument)
	}{
		{
			name: "LinkString",
			run: func(pdf *pdfDocument) {
				pdf.LinkString(1, 1, 10, 10, "javascript:app.alert(1)")
			},
		},
		{
			name: "WriteLinkString",
			run: func(pdf *pdfDocument) {
				pdf.WriteLinkString(5, "bad", " JaVaScRiPt:app.alert(1)")
			},
		},
		{
			name: "CellFormat",
			run: func(pdf *pdfDocument) {
				pdf.CellFormat(10, 5, "bad", "", 0, "", false, 0, "data:text/html,<script>alert(1)</script>")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := mustNewPDFDocument()
			pdf.AddPage()
			pdf.SetFont("Helvetica", "", 12)
			tt.run(pdf)
			if pdf.Error() == nil || !strings.Contains(pdf.Error().Error(), "unsupported link scheme") {
				t.Fatalf("%s error = %v, want unsupported link scheme", tt.name, pdf.Error())
			}
		})
	}
}

func TestSecurityAliasReplacementEscaped(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Write(5, "{alias}")
	pdf.RegisterAlias("{alias}", "\") Tj ET\nq 1 0 0 rg\nBT (")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if regexp.MustCompile(`[^\\]\) Tj ET\nq 1 0 0 rg`).MatchString(out.String()) {
		t.Fatal("alias replacement injected raw PDF operators")
	}
}

func TestSecurityUnsafeUTF8FontNameRejected(t *testing.T) {
	fontBytes, err := os.ReadFile(securityFixturePath(t, "assets", "static", "font", "DejaVuSansCondensed.ttf"))
	if err != nil {
		t.Fatalf("ReadFile font: %v", err)
	}
	pdf := mustNewPDFDocument()
	pdf.AddUTF8FontFromBytes("Bad/Font", "", fontBytes)
	if pdf.Error() == nil {
		t.Fatal("expected invalid UTF-8 font name error")
	}
}

func securityFixturePath(t *testing.T, elems ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	parts := append([]string{filepath.Dir(file), ".."}, elems...)
	return filepath.Clean(filepath.Join(parts...))
}

func TestSecurityNonFiniteDrawingInputsRejected(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddPage()
	pdf.SubWrite(math.NaN(), "x", 6, 0, 0, "")
	if pdf.Error() == nil {
		t.Fatal("expected non-finite SubWrite error")
	}
}

func securityPNG(width, height uint32, pdfColor byte, chunks ...[]byte) []byte {
	var out bytes.Buffer
	out.WriteString("\x89PNG\x0d\x0a\x1a\x0a")
	var ihdr bytes.Buffer
	_ = binary.Write(&ihdr, binary.BigEndian, width)
	_ = binary.Write(&ihdr, binary.BigEndian, height)
	ihdr.Write([]byte{8, pdfColor, 0, 0, 0})
	out.Write(securityPNGChunk("IHDR", ihdr.Bytes()))
	for _, chunk := range chunks {
		out.Write(chunk)
	}
	return out.Bytes()
}

func securityPNGChunk(chunkType string, data []byte) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint32(len(data)))
	out.WriteString(chunkType)
	out.Write(data)
	_ = binary.Write(&out, binary.BigEndian, uint32(0))
	return out.Bytes()
}

func securityGIF(width, height uint16) []byte {
	var out bytes.Buffer
	out.WriteString("GIF89a")
	_ = binary.Write(&out, binary.LittleEndian, width)
	_ = binary.Write(&out, binary.LittleEndian, height)
	out.Write([]byte{0, 0, 0, ';'})
	return out.Bytes()
}

func securityZlibBytes(data []byte) []byte {
	var out bytes.Buffer
	w := zlib.NewWriter(&out)
	_, _ = w.Write(data)
	_ = w.Close()
	return out.Bytes()
}

func TestSecurityImageInfoValidationRejectsPDFSyntax(t *testing.T) {
	info := &ImageInfo{
		data: []byte("x"),
		w:    1,
		h:    1,
		cs:   "DeviceRGB",
		bpc:  8,
		f:    "FlateDecode",
		dp:   "/Predictor 15 /Colors 3 /BitsPerComponent 8 /Columns 1 " + ">>",
	}
	if err := info.validForPDF(); err == nil {
		t.Fatal("expected invalid decode parameters error")
	}
}
