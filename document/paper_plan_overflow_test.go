// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import "testing"

func TestPaperPlanPageOverflowReportsOffPageGeometry(t *testing.T) {
	contained, result, err := PlanPaper("contained.paper", paperCanvasSource)
	if err != nil || !result.OK() {
		t.Fatalf("PlanPaper(contained) = %#v, %v", result, err)
	}
	overflow, err := contained.PageOverflow(1)
	if err != nil || overflow.Records != 0 {
		t.Fatalf("PageOverflow(contained) = %#v, %v", overflow, err)
	}

	source := "document @d:\n" +
		"  page @p:\n" +
		"    width: 200pt\n" +
		"    height: 120pt\n" +
		"    margin: 12pt\n" +
		"    body @b:\n" +
		"      canvas @diagram:\n" +
		"        width: 160pt\n" +
		"        height: 80pt\n" +
		"        anchor @outside:\n" +
		"          width: 40pt\n" +
		"          height: 20pt\n" +
		"          left: \"canvas.left - 20pt\"\n" +
		"          top: \"canvas.top\"\n" +
		"          background: \"#336699\"\n"
	plan, result, err := PlanPaper("overflow.paper", source)
	if err != nil || !result.OK() {
		t.Fatalf("PlanPaper(overflow) = %#v, %v", result, err)
	}
	overflow, err = plan.PageOverflow(1)
	if err != nil {
		t.Fatal(err)
	}
	if overflow.Records == 0 || overflow.Left <= 0 || overflow.Top != 0 || overflow.Right != 0 || overflow.Bottom != 0 {
		t.Fatalf("PageOverflow(overflow) = %#v", overflow)
	}
	if _, err := plan.PageOverflow(0); err == nil {
		t.Fatal("PageOverflow accepted page zero")
	}
	if _, err := (PaperPlan{}).PageOverflow(1); err == nil {
		t.Fatal("PageOverflow accepted an empty plan")
	}
}
