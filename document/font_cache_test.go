// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/testsupport/example"
)

func TestFontCacheMatchesUTF8FontFromBytes(t *testing.T) {
	fontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	cache := document.NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		t.Fatalf("AddUTF8FontFromBytes() error = %v", err)
	}

	build := func(addFont func(*document.TestPDFDocument)) []byte {
		pdf := document.MustNewTestPDFDocument()
		pdf.SetCompression(false)
		pdf.SetCatalogSort(true)
		pdf.SetCreationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		pdf.SetModificationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		addFont(pdf)
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 12)
		pdf.MultiCell(0, 6, "Cached UTF-8: Hello, 世界, مرحبا", "", "L", false)
		var out bytes.Buffer
		if err := pdf.Output(&out); err != nil {
			t.Fatalf("Output() error = %v", err)
		}
		return out.Bytes()
	}

	uncached := build(func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8FontFromBytes("DejaVu", "", fontBytes)
	})
	cached := build(func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8FontFromCache("DejaVu", "", cache)
	})
	if !bytes.Equal(uncached, cached) {
		t.Fatal("cached UTF-8 font output differs from AddUTF8FontFromBytes output")
	}
}

func TestAddUTF8FontUsesSharedCacheWithoutChangingOutput(t *testing.T) {
	fontPath := example.FontFile("DejaVuSansCondensed.ttf")
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	build := func(addFont func(*document.TestPDFDocument)) []byte {
		pdf := document.MustNewTestPDFDocument()
		pdf.SetCompression(false)
		pdf.SetCatalogSort(true)
		pdf.SetCreationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		pdf.SetModificationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		addFont(pdf)
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 12)
		pdf.MultiCell(0, 6, "Shared UTF-8 cache: Hello, 世界, مرحبا", "", "L", false)
		var out bytes.Buffer
		if err := pdf.Output(&out); err != nil {
			t.Fatalf("Output() error = %v", err)
		}
		return out.Bytes()
	}

	fromBytes := build(func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8FontFromBytes("DejaVu", "", fontBytes)
	})
	firstPathLoad := build(func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8Font("DejaVu", "", fontPath)
	})
	secondPathLoad := build(func(pdf *document.TestPDFDocument) {
		pdf.AddUTF8Font("DejaVu", "", fontPath)
	})

	if !bytes.Equal(fromBytes, firstPathLoad) {
		t.Fatal("first AddUTF8Font output differs from AddUTF8FontFromBytes output")
	}
	if !bytes.Equal(firstPathLoad, secondPathLoad) {
		t.Fatal("shared cached AddUTF8Font output differs from first AddUTF8Font output")
	}
}

func TestFontCacheConcurrentDocumentsOwnDeterministicSubsetBytes(t *testing.T) {
	fontBytes, err := os.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	cache := document.NewFontCache()
	if err := cache.AddUTF8FontFromBytes("DejaVu", "", fontBytes); err != nil {
		t.Fatal(err)
	}
	texts := []string{
		strings.Repeat("Latin Greek Ελληνικά Cyrillic Кириллица ", 64),
		strings.Repeat("Português médico: coração, pressão, ação. ", 64),
	}
	prepare := func(text string, cached bool) *document.TestPDFDocument {
		t.Helper()
		pdf := document.MustNewTestPDFDocument()
		pdf.SetCompression(false)
		pdf.SetCatalogSort(true)
		pdf.SetCreationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		pdf.SetModificationDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if cached {
			pdf.AddUTF8FontFromCache("DejaVu", "", cache)
		} else {
			pdf.AddUTF8FontFromBytes("DejaVu", "", fontBytes)
		}
		pdf.AddPage()
		pdf.SetFont("DejaVu", "", 10)
		pdf.MultiCell(0, 5, text, "", "L", false)
		return pdf
	}
	render := func(pdf *document.TestPDFDocument) ([]byte, error) {
		var output bytes.Buffer
		if err := pdf.Output(&output); err != nil {
			return nil, err
		}
		return output.Bytes(), nil
	}
	expected := make([][]byte, len(texts))
	for index, text := range texts {
		expected[index], err = render(prepare(text, false))
		if err != nil {
			t.Fatal(err)
		}
	}

	documents := []*document.TestPDFDocument{prepare(texts[0], true), prepare(texts[1], true)}
	type result struct {
		index int
		data  []byte
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, len(documents))
	for index, pdf := range documents {
		go func() {
			<-start
			data, outputErr := render(pdf)
			results <- result{index: index, data: data, err: outputErr}
		}()
	}
	close(start)
	for range documents {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent output %d: %v", result.index, result.err)
		}
		if !bytes.Equal(result.data, expected[result.index]) {
			t.Fatalf("concurrent cached output %d differs from owned-font output", result.index)
		}
	}
}
