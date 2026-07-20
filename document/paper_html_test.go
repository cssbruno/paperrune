// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0

package document

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestPaperPlanExportHTMLIsDeterministicExactPageSVG(t *testing.T) {
	source := "document @web:\n" +
		"  title: \"Paper web export\"\n" +
		"  language: \"en-US\"\n" +
		"  page @sheet:\n" +
		"    width: 180pt\n" +
		"    height: 100pt\n" +
		"    margin: 10pt\n" +
		"    body @body:\n" +
		"      paragraph @first:\n" +
		"        text: \"First page\"\n" +
		"      page-break @next:\n" +
		"      paragraph @second:\n" +
		"        text: \"Second page\"\n"
	plan, result, err := PlanPaper("web.paper", source)
	if err != nil || !result.OK() {
		t.Fatalf("PlanPaper() = %#v, %v", result, err)
	}
	first, err := plan.ExportHTML()
	if err != nil {
		t.Fatal(err)
	}
	second, err := plan.ExportHTML()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("deterministic HTML = %v, equal=%t", err, bytes.Equal(first, second))
	}
	for _, expected := range [][]byte{
		[]byte("<!doctype html>"), []byte(`lang="en-US"`), []byte("Paper web export"),
		[]byte(`aria-label="Page 1 of 2"`), []byte(`aria-label="Page 2 of 2"`),
		[]byte(`data-format="display-plan-preview"`), []byte("<text "),
	} {
		if !bytes.Contains(first, expected) {
			t.Fatalf("HTML lacks %q:\n%s", expected, first)
		}
	}
	if bytes.Count(first, []byte("<svg ")) != 2 || bytes.Contains(first, []byte("<?xml")) {
		t.Fatalf("embedded SVG pages are invalid:\n%s", first)
	}
}

func TestPaperPlanExportHTMLRejectsInvalidContextAndEmptyPlan(t *testing.T) {
	if _, err := (PaperPlan{}).ExportHTML(); err == nil {
		t.Fatal("empty plan unexpectedly exported")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (PaperPlan{hash: "x", pages: 1}).ExportHTMLContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled export = %v", err)
	}
}
