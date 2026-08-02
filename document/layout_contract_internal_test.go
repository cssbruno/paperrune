// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func TestWriteDocumentAppliesLanguageToCatalog(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Language = "pt-BR"
	doc.Body = []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Ola"}}}}
	pdf.WriteDocument(doc)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if !strings.Contains(output.String(), "/Lang (pt-BR)") {
		t.Fatal("PDF catalog is missing the layout document language")
	}
}

func TestWriteDocumentRendersSegmentStyleAndLink(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{
		{Text: "plain "},
		{Text: "linked", Link: "https://example.test/segment", Style: layout.TextStyle{Bold: true, Color: layout.DocumentColor{R: 180, Set: true}}},
	}}}
	pdf.WriteDocument(doc)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := output.String()
	if !strings.Contains(content, "/URI (https://example.test/segment)") {
		t.Fatal("styled text segment did not emit its link annotation")
	}
	if !strings.Contains(content, "/BaseFont /Helvetica-Bold") {
		t.Fatal("styled text segment did not register its bold font")
	}
}

func TestWriteDocumentRendersQRCodeImage(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.QR = &layout.QRBlock{Value: "verified-value", Label: "Verify", Size: 18}
	pdf.WriteDocument(doc)
	if image, ok := pdf.ensureResourceStore().image(QRCodeImageName("verified-value")); !ok || image == nil {
		t.Fatal("QR block did not register its deterministic image")
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := output.String()
	if !strings.Contains(content, "/Subtype /Image") {
		t.Fatal("QR block did not render its registered image")
	}
}

func TestTypedTableRepeatsHeaderAcrossPages(t *testing.T) {
	pdf := mustNewPDFDocument(WithCustomPageSize(Size{Wd: 90, Ht: 90}))
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.PageTemplate.Margins = layout.Spacing{Top: 8, Right: 8, Bottom: 8, Left: 8}
	body := make([]layout.TableRow, 30)
	for i := range body {
		body[i] = layout.TableRow{Cells: []layout.TableCell{{Blocks: []layout.Block{
			layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: fmt.Sprintf("row-%02d", i)}}},
		}}}}
	}
	doc.Body = []layout.Block{layout.TableBlock{
		Header: []layout.TableRow{{Cells: []layout.TableCell{{Blocks: []layout.Block{
			layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "repeated-header"}}},
		}}}}},
		Body:  body,
		Style: layout.TableStyle{RepeatHeader: true, BorderCollapse: true},
	}}
	pdf.WriteDocument(doc)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	t.Logf("table extracted=%q raw=%q", extractedDocumentText(t, output.Bytes()), output.String())
	if pdf.PageCount() < 2 {
		t.Fatalf("PageCount() = %d, want a multipage table", pdf.PageCount())
	}
	if count := strings.Count(extractedDocumentText(t, output.Bytes()), "repeated-header"); count != pdf.PageCount() {
		t.Fatalf("header occurrences = %d, want one on each of %d pages", count, pdf.PageCount())
	}
}

func TestTypedTableFixedColumnsRetainAuthoredWidthOnWiderPage(t *testing.T) {
	pdf := mustNewPDFDocument(WithCustomPageSize(Size{Wd: 595.275590551, Ht: 841.88976378}))
	doc := layout.NewLayoutDocument()
	doc.PageTemplate.Margins = layout.Spacing{Top: 12, Right: 12, Bottom: 12, Left: 12}
	doc.Body = []layout.Block{layout.TableBlock{
		Columns: []layout.TableColumn{{Width: 84}, {Width: 84}},
		Body: []layout.TableRow{{Cells: []layout.TableCell{
			{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "left"}}}}},
			{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "right"}}}}},
		}}},
	}}

	plan, err := pdf.PlanLayoutDocument(doc)
	if err != nil {
		t.Fatalf("PlanLayoutDocument() error = %v", err)
	}
	if plan.PageCount() == 0 {
		t.Fatal("PlanLayoutDocument() returned no pages")
	}
	want, err := layoutengine.FixedFromPoints(168)
	if err != nil {
		t.Fatal(err)
	}
	bodyWidth, err := layoutengine.FixedFromPoints(571.275590551)
	if err != nil {
		t.Fatal(err)
	}
	columnWidth, err := layoutengine.FixedFromPoints(84)
	if err != nil {
		t.Fatal(err)
	}
	tableWidth := typedTablePlanWidth(bodyWidth, []layoutengine.TableColumn{
		{Kind: layoutengine.TableTrackFixed, Width: columnWidth},
		{Kind: layoutengine.TableTrackFixed, Width: columnWidth},
	})
	if tableWidth != want {
		t.Fatalf("typedTablePlanWidth() = %v, want authored fixed width %v", tableWidth, want)
	}
}

func TestTypedBoxBackgroundUsesMeasuredContentHeight(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := layout.NewLayoutDocument()
	doc.Body = []layout.Block{layout.ParagraphBlock{
		Segments: []layout.TextSegment{{Text: "background"}},
		Style:    layout.TextStyle{LineHeight: 8},
		Box: layout.BoxStyle{
			Padding:         layout.Spacing{Top: 2, Right: 2, Bottom: 2, Left: 2},
			BackgroundColor: layout.DocumentColor{R: 240, G: 240, B: 240, Set: true},
		},
	}}
	plan, err := pdf.PlanLayoutDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	projection := plan.plan.Projection()
	if len(projection.Fills) != 1 {
		t.Fatalf("planned fills = %#v, want one background", projection.Fills)
	}
	fill := projection.Fills[0]
	if int(fill.Path) >= len(projection.Paths) {
		t.Fatalf("background fill path = %d, paths = %d", fill.Path, len(projection.Paths))
	}
	want := pdf.UnitToPointConvert(12)
	if got := projection.Paths[fill.Path].Bounds.Height.Points(); math.Abs(got-want) > 1.0/float64(layoutengine.FixedScale) {
		t.Fatalf("planned background height = %.6f, want %.6f", got, want)
	}
	pdf.WriteDocument(doc)
	if pdf.Error() != nil || !strings.Contains(pdf.pages[pdf.page].String(), " h f") {
		t.Fatalf("unified background was not painted: %v\n%s", pdf.Error(), pdf.pages[pdf.page].String())
	}
}
