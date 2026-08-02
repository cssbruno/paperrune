// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
)

type planBeforePaintSnapshot struct {
	page       int
	pageCount  int
	x, y       float64
	fontFamily string
	fontStyle  string
	fontSize   float64
	underline  bool
	strikeout  bool
	pages      [][]byte
}

func snapshotPlanBeforePaint(pdf *pdfDocument) planBeforePaintSnapshot {
	snapshot := planBeforePaintSnapshot{
		page: pdf.PageNo(), pageCount: pdf.PageCount(), x: pdf.GetX(), y: pdf.GetY(),
		fontFamily: pdf.fontFamily, fontStyle: pdf.fontStyle, fontSize: pdf.fontSizePt,
		underline: pdf.underline, strikeout: pdf.strikeout,
		pages: make([][]byte, len(pdf.pages)),
	}
	for index, page := range pdf.pages {
		if page != nil {
			snapshot.pages[index] = append([]byte(nil), page.Bytes()...)
		}
	}
	return snapshot
}

func assertPlanBeforePaintUnchanged(t *testing.T, pdf *pdfDocument, before planBeforePaintSnapshot) {
	t.Helper()
	after := snapshotPlanBeforePaint(pdf)
	if before.page != after.page || before.pageCount != after.pageCount || before.x != after.x || before.y != after.y ||
		before.fontFamily != after.fontFamily || before.fontStyle != after.fontStyle || before.fontSize != after.fontSize ||
		before.underline != after.underline || before.strikeout != after.strikeout || len(before.pages) != len(after.pages) {
		t.Fatalf("planning mutated document state: before=%+v after=%+v", before, after)
	}
	for index := range before.pages {
		if !bytes.Equal(before.pages[index], after.pages[index]) {
			t.Fatalf("planning mutated page %d: before=%q after=%q", index, before.pages[index], after.pages[index])
		}
	}
}

func TestResourcePlanningDoesNotMutateOutputDocument(t *testing.T) {
	cases := []struct {
		name string
		plan func(*pdfDocument) error
	}{
		{
			name: "typed-font-measurement",
			plan: func(pdf *pdfDocument) error {
				_, err := pdf.PlanLayoutDocument(&layout.LayoutDocument{
					Language: "en-US",
					Body: []layout.Block{layout.ParagraphBlock{
						Segments: []layout.TextSegment{{Text: "font measurement stays in the plan"}},
						Style:    layout.TextStyle{FontFamily: "Helvetica", FontSize: 11, LineHeight: 13},
					}},
				})
				return err
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			pdf := mustNewPDFDocument(
				WithUnit(UnitPoint),
				WithCustomPageSize(Size{Wd: 220, Ht: 180}),
				WithNoCompression(),
				WithDeterministicOutput(),
			)
			before := snapshotPlanBeforePaint(pdf)
			if err := test.plan(pdf); err != nil {
				t.Fatal(err)
			}
			assertPlanBeforePaintUnchanged(t, pdf, before)
		})
	}
}
