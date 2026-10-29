// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"testing"
)

func TestAddSpotColorClampsComponents(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddSpotColor("Brand", 120, 20, 101, 40)
	pdf.SetDrawSpotColor("Brand", 100)

	name, cyan, magenta, yellow, black := pdf.GetDrawSpotColor()
	if name != "Brand" {
		t.Fatalf("spot color name = %q, want Brand", name)
	}
	if cyan != 100 || magenta != 20 || yellow != 100 || black != 40 {
		t.Fatalf("CMYK = %d,%d,%d,%d, want 100,20,100,40", cyan, magenta, yellow, black)
	}
}

func TestAddSpotColorRejectsDuplicateName(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddSpotColor("Brand", 0, 0, 0, 0)
	pdf.AddSpotColor("Brand", 1, 2, 3, 4)
	if pdf.Error() == nil {
		t.Fatal("expected duplicate spot color name error")
	}
}

func TestSetSpotColorRejectsUnknownName(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetFillSpotColor("Missing", 50)
	if pdf.Error() == nil {
		t.Fatal("expected unregistered spot color error")
	}
}

func TestSetTextSpotColorMarksColorStateDirty(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddSpotColor("Fill", 0, 0, 0, 0)
	pdf.AddSpotColor("Text", 0, 0, 0, 100)
	pdf.SetFillSpotColor("Fill", 100)
	pdf.SetTextSpotColor("Text", 100)
	if !pdf.colorFlag {
		t.Fatal("text spot color should mark fill and text colors as different")
	}
}

func TestSpotColorSeparationNameIsEscaped(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.AddSpotColor("Brand Blue]>>\nendobj", 10, 20, 30, 40)
	pdf.AddPage()
	pdf.SetFillSpotColor("Brand Blue]>>\nendobj", 100)
	pdf.Rect(10, 10, 20, 20, "F")
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("[/Separation /Brand#20Blue#5D#3E#3E#0Aendobj")) {
		t.Fatalf("separation name was not escaped:\n%s", out.Bytes())
	}
}
