// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document_test

import (
	"errors"
	"testing"

	"github.com/cssbruno/paperrune/document"
)

func TestAddUTF8FontErrorReturnsLatchedError(t *testing.T) {
	pdf := document.MustNewTestPDFDocument()

	err := pdf.AddUTF8FontError("bad", "", "../bad.ttf")
	if err == nil {
		t.Fatal("AddUTF8FontError() error = nil")
	}
	if !errors.Is(pdf.Error(), err) {
		t.Fatal("AddUTF8FontError() did not return the latched document error")
	}
}

func TestAddUTF8FontFromBytesErrorReturnsLatchedError(t *testing.T) {
	pdf := document.MustNewTestPDFDocument()

	err := pdf.AddUTF8FontFromBytesError("bad", "", []byte{0, 1})
	if err == nil {
		t.Fatal("AddUTF8FontFromBytesError() error = nil")
	}
	if !errors.Is(pdf.Error(), err) {
		t.Fatal("AddUTF8FontFromBytesError() did not return the latched document error")
	}
}

func TestAddUTF8FontFromCacheErrorReturnsLatchedError(t *testing.T) {
	pdf := document.MustNewTestPDFDocument()

	err := pdf.AddUTF8FontFromCacheError("missing", "", nil)
	if err == nil {
		t.Fatal("AddUTF8FontFromCacheError() error = nil")
	}
	if !errors.Is(pdf.Error(), err) {
		t.Fatal("AddUTF8FontFromCacheError() did not return the latched document error")
	}
}
