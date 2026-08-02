// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestReplaceAliasBytesUsesLongestSinglePassMatches(t *testing.T) {
	pairs := []aliasReplacementBytes{
		{old: []byte("{page total}"), new: []byte("12")},
		{old: []byte("{page}"), new: []byte("3")},
	}
	got := replaceAliasBytes([]byte("{page total}/{page}"), pairs)
	if want := []byte("12/3"); !bytes.Equal(got, want) {
		t.Fatalf("replaceAliasBytes() = %q, want %q", got, want)
	}
	if got := replaceAliasBytes([]byte("no aliases"), pairs); got != nil {
		t.Fatalf("no-match replacement allocated %q", got)
	}
}

func TestReplaceAliasesContextHonorsCancellationBeforeMutation(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.RegisterAlias("{page}", "1")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(20, 5, "{page}")
	before := append([]byte(nil), pdf.pages[1].Bytes()...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pdf.replaceAliasesContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("replaceAliasesContext() error = %v", err)
	}
	if !bytes.Equal(pdf.pages[1].Bytes(), before) {
		t.Fatal("canceled alias replacement mutated the page")
	}
}
