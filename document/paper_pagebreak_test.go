// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cssbruno/paperrune/internal/layoutengine"
	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

const paperExplicitBreakFixture = "document:\n" +
	"  page:\n" +
	"    width: 90pt\n" +
	"    height: 90pt\n" +
	"    margin: 6pt\n" +
	"    body:\n" +
	"      page-break:\n" +
	"      text: \"A\"\n" +
	"      page-break @first:\n" +
	"      page-break @duplicate:\n" +
	"      text: \"B\"\n" +
	"      page-break:\n"

func TestPaperPlannerHonorsExplicitBreakWithoutBlankPages(t *testing.T) {
	parsed := paperlang.Parse("explicit.paper", paperExplicitBreakFixture)
	if !parsed.OK() {
		t.Fatalf("Parse() diagnostics = %+v", parsed.Diagnostics)
	}
	compiled := papercompile.Compile(parsed.AST)
	if !compiled.OK() {
		t.Fatalf("Compile() diagnostics = %+v", compiled.Diagnostics)
	}
	planner, err := newPaperPlanner(compiled.Page)
	if err != nil {
		t.Fatalf("newPaperPlanner() = %v", err)
	}
	plan, err := planner.planPaperTextBlocks(compiled.Document)
	if err != nil {
		t.Fatalf("planPaperTextBlocks() = %v", err)
	}
	projection := plan.Projection()
	if len(projection.Pages) != 2 || len(projection.Fragments) != 2 || len(projection.Breaks) != 1 {
		t.Fatalf("plan pages/fragments/breaks = %d/%d/%+v", len(projection.Pages), len(projection.Fragments), projection.Breaks)
	}
	decision := projection.Breaks[0]
	if decision.Reason != layoutengine.BreakExplicitPageBreak || decision.FromPage != 1 || decision.ToPage != 2 ||
		decision.Preceding != projection.Fragments[0].ID || decision.Triggering != projection.Fragments[1].ID ||
		decision.Required != 0 || decision.Available != 0 {
		t.Fatalf("explicit break decision = %+v", decision)
	}
}

func TestPaperPlannerRecordsEachSeparatedExplicitBreak(t *testing.T) {
	source := "document:\n" +
		"  page:\n" +
		"    body:\n" +
		"      text: \"A\"\n" +
		"      page-break @second:\n" +
		"      text: \"B\"\n" +
		"      page-break @third:\n" +
		"      text: \"C\"\n"
	parsed := paperlang.Parse("separated-breaks.paper", source)
	if !parsed.OK() {
		t.Fatalf("Parse() diagnostics = %+v", parsed.Diagnostics)
	}
	compiled := papercompile.Compile(parsed.AST)
	if !compiled.OK() {
		t.Fatalf("Compile() diagnostics = %+v", compiled.Diagnostics)
	}
	planner, err := newPaperPlanner(compiled.Page)
	if err != nil {
		t.Fatalf("newPaperPlanner() = %v", err)
	}
	plan, err := planner.planPaperTextBlocks(compiled.Document)
	if err != nil {
		t.Fatalf("planPaperTextBlocks() = %v", err)
	}
	projection := plan.Projection()
	if len(projection.Pages) != 3 || len(projection.Fragments) != 3 || len(projection.Breaks) != 2 {
		t.Fatalf("plan pages/fragments/breaks = %d/%d/%+v", len(projection.Pages), len(projection.Fragments), projection.Breaks)
	}
	for index, decision := range projection.Breaks {
		if decision.Reason != layoutengine.BreakExplicitPageBreak || decision.FromPage != uint32(index+1) || decision.ToPage != uint32(index+2) {
			t.Fatalf("break[%d] = %+v", index, decision)
		}
	}
}

func TestPaperPlannerAppliesPageLimitToExplicitBreak(t *testing.T) {
	parsed := paperlang.Parse("limited-break.paper", "document:\n  page:\n    body:\n      text: \"A\"\n      page-break:\n      text: \"B\"\n")
	if !parsed.OK() {
		t.Fatalf("Parse() diagnostics = %+v", parsed.Diagnostics)
	}
	compiled := papercompile.Compile(parsed.AST)
	if !compiled.OK() {
		t.Fatalf("Compile() diagnostics = %+v", compiled.Diagnostics)
	}
	planner, err := newPaperPlanner(compiled.Page)
	if err != nil {
		t.Fatalf("newPaperPlanner() = %v", err)
	}
	planner.limits.MaxPages = 1
	if _, err := planner.planPaperTextBlocks(compiled.Document); !errors.Is(err, ErrPageLimitExceeded) {
		t.Fatalf("planPaperTextBlocks() error = %v, want ErrPageLimitExceeded", err)
	}
}

func TestWritePaperPaintsExplicitBreakFromTheCompletedPlan(t *testing.T) {
	target := mustNewPDFDocument(WithUnit(UnitPoint), WithNoCompression())
	result, err := target.WritePaper("explicit.paper", paperExplicitBreakFixture)
	if err != nil || !result.OK() || result.Pages != 2 || target.PageCount() != 2 {
		t.Fatalf("WritePaper() = %#v, %v; pages=%d", result, err, target.PageCount())
	}
	if !bytes.Contains(target.pages[1].Bytes(), []byte("/ActualText (A)")) ||
		bytes.Contains(target.pages[1].Bytes(), []byte("/ActualText (B)")) ||
		!bytes.Contains(target.pages[2].Bytes(), []byte("/ActualText (B)")) {
		t.Fatalf("explicit break did not preserve source allocation:\npage 1 %s\npage 2 %s", target.pages[1].Bytes(), target.pages[2].Bytes())
	}
}
