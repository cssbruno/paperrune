// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"testing"

	"github.com/cssbruno/paperrune/layout"
)

// TestTypedCanonicalPlanPreservesSimpleCompatibilityBaselines compares the
// public lowering adapter with the canonical plan/painter output.
func TestTypedCanonicalPlanPreservesSimpleCompatibilityBaselines(t *testing.T) {
	paragraph := func(text string) layout.ParagraphBlock {
		return layout.ParagraphBlock{
			Segments: []layout.TextSegment{{Text: text}},
			Style:    layout.TextStyle{FontFamily: "Helvetica", FontSize: 11, LineHeight: 14},
		}
	}
	heading := func(text string) layout.HeadingBlock {
		return layout.HeadingBlock{
			Level:    2,
			Segments: []layout.TextSegment{{Text: text}},
			Style:    layout.TextStyle{FontFamily: "Helvetica", FontSize: 13, LineHeight: 16},
		}
	}

	fixtures := []struct {
		name      string
		document  *layout.LayoutDocument
		wantPages int
	}{
		{
			name: "single-page-flow",
			document: &layout.LayoutDocument{
				PageTemplate: compatibilityPageTemplate(),
				Body: []layout.Block{
					heading("ALPHA"),
					paragraph("BRAVO"),
					paragraph("CHARLIE"),
				},
			},
			wantPages: 1,
		},
		{
			name: "explicit-page-break",
			document: &layout.LayoutDocument{
				PageTemplate: compatibilityPageTemplate(),
				Body: []layout.Block{
					paragraph("DELTA"),
					layout.PageBreakBlock{After: true},
					paragraph("ECHO"),
				},
			},
			wantPages: 2,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			adapted := renderAdaptedTypedCompatibilityPDF(t, fixture.document)
			planned, planPages := renderPlannedTypedCompatibilityPDF(t, fixture.document)

			if planPages != fixture.wantPages {
				t.Fatalf("painted pages = %d, want %d", planPages, fixture.wantPages)
			}
			if !bytes.Equal(adapted, planned) {
				t.Fatalf("adapter and planned output differ: %d/%d bytes", len(adapted), len(planned))
			}
		})
	}
}

func compatibilityPageTemplate() layout.PageTemplate {
	return layout.PageTemplate{Margins: layout.Spacing{Top: 12, Right: 12, Bottom: 12, Left: 12}}
}

func newTypedCompatibilityPDF() *pdfDocument {
	return mustNewPDFDocument(
		WithUnit(UnitPoint),
		WithCustomPageSize(Size{Wd: 220, Ht: 160}),
		WithNoCompression(),
		WithDeterministicOutput(),
	)
}

func renderAdaptedTypedCompatibilityPDF(t *testing.T, model *layout.LayoutDocument) []byte {
	t.Helper()
	pdf := newTypedCompatibilityPDF()
	pdf.WriteDocument(model)
	if err := pdf.Error(); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	return outputTypedCompatibilityPDF(t, pdf)
}

func renderPlannedTypedCompatibilityPDF(t *testing.T, model *layout.LayoutDocument) ([]byte, int) {
	t.Helper()
	planner := newTypedCompatibilityPDF()
	plan, err := planner.PlanLayoutDocument(model)
	if err != nil {
		t.Fatalf("PlanLayoutDocument: %v", err)
	}
	if planner.PageCount() != 0 {
		t.Fatalf("planning mutated source: page count %d", planner.PageCount())
	}
	target := newTypedCompatibilityPDF()
	written, err := target.WriteLayoutDocumentPlan(plan)
	if err != nil {
		t.Fatalf("WriteLayoutDocumentPlan: %v", err)
	}
	if written != plan.PageCount() {
		t.Fatalf("written pages = %d, plan pages = %d", written, plan.PageCount())
	}
	return outputTypedCompatibilityPDF(t, target), plan.PageCount()
}

func outputTypedCompatibilityPDF(t *testing.T, pdf *pdfDocument) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := pdf.OutputWithOptions(&output, OutputOptions{Deterministic: true}); err != nil {
		t.Fatalf("output PDF: %v", err)
	}
	return append([]byte(nil), output.Bytes()...)
}
