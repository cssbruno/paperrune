// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestBaseDocumentProducesDeterministicBytes(t *testing.T) {
	t.Parallel()

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot() error = %v", err)
	}
	fontPath := filepath.Join(root, "assets", "static", "font", "DejaVuSansCondensed.ttf")
	boldFontPath := filepath.Join(root, "assets", "static", "font", "DejaVuSansCondensed-Bold.ttf")
	render := func() []byte {
		pdf := baseDocument(fontPath, boldFontPath)
		pdf.SetTitle("deterministic compliance fixture", false)
		source := "document @stable:\n  page @sheet:\n    body @body:\n      paragraph @copy:\n        text: \"stable content\"\n"
		if rendered, err := pdf.WritePaper("stable.paper", source); err != nil || !rendered.OK() {
			t.Fatalf("WritePaper() = %#v, %v", rendered, err)
		}
		var output bytes.Buffer
		if err := pdf.Output(&output); err != nil {
			t.Fatalf("Output() error = %v", err)
		}
		return output.Bytes()
	}

	first := render()
	second := render()
	if !bytes.Equal(first, second) {
		t.Fatal("deterministic compliance base changed between identical renders")
	}
	if !bytes.HasPrefix(first, []byte("%PDF-")) || !bytes.Contains(first, []byte("%%EOF")) {
		t.Fatal("generated compliance fixture is incomplete")
	}
}
