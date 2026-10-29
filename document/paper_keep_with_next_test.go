// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func TestTypedKeepWithNextMovesHeadingBeforeFollowerTallerThanPage(t *testing.T) {
	tests := []struct {
		name       string
		lines      int
		follower   layout.BoxStyle
		wantRelax  bool
		headingPg  uint32
		followerPg uint32
	}{
		{name: "follower fits a page", lines: 3, headingPg: 2, followerPg: 2},
		{name: "follower taller than a page", lines: 8, headingPg: 2, followerPg: 2},
		{name: "orphans fit beside heading", lines: 8, follower: layout.BoxStyle{Orphans: 3, Widows: 2}, headingPg: 2, followerPg: 2},
		{name: "orphans exceed an empty page", lines: 8, follower: layout.BoxStyle{Orphans: 5, Widows: 2}, wantRelax: true, headingPg: 1, followerPg: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			planner := paginationTestDocument(t, 70) // 50pt body, 10pt lines: five lines per page.
			follower := make([]string, test.lines)
			for index := range follower {
				follower[index] = "body"
			}
			plan, err := planner.PlanLayoutDocument(&layout.LayoutDocument{Body: []layout.Block{
				paginationParagraph("f1\nf2\nf3\nf4", layout.BoxStyle{}),
				paginationParagraph("HEADING", layout.BoxStyle{KeepWithNext: true}),
				paginationParagraph(strings.Join(follower, "\n"), test.follower),
			}})
			if err != nil {
				t.Fatal(err)
			}
			projection := plan.plan.Projection()
			fragmentsByNode := map[layoutengine.NodeID][]layoutengine.Fragment{}
			var nodes []layoutengine.NodeID
			for _, fragment := range projection.Fragments {
				if _, seen := fragmentsByNode[fragment.Node]; !seen {
					nodes = append(nodes, fragment.Node)
				}
				fragmentsByNode[fragment.Node] = append(fragmentsByNode[fragment.Node], fragment)
			}
			if len(nodes) != 3 {
				t.Fatalf("fragment nodes = %d, want 3: %#v", len(nodes), projection.Fragments)
			}
			heading, first := fragmentsByNode[nodes[1]][0], fragmentsByNode[nodes[2]][0]
			if heading.Page != test.headingPg || first.Page != test.followerPg {
				t.Fatalf("heading page = %d, follower page = %d, want %d and %d", heading.Page, first.Page, test.headingPg, test.followerPg)
			}
			relaxed := false
			for _, diagnostic := range projection.Diagnostics {
				if diagnostic.Code == layoutengine.DiagnosticKeepTooLarge {
					relaxed = true
				}
			}
			if relaxed != test.wantRelax {
				t.Fatalf("KEEP_TOO_LARGE = %v, want %v: %#v", relaxed, test.wantRelax, projection.Diagnostics)
			}
			if test.wantRelax {
				return
			}
			if heading.BorderBox.Y.Points() != 10 || first.BorderBox.Y.Points() != 20 {
				t.Fatalf("heading y = %.1f, follower y = %.1f, want 10 and 20", heading.BorderBox.Y.Points(), first.BorderBox.Y.Points())
			}
			var breakFound bool
			for _, decision := range projection.Breaks {
				if decision.Reason == layoutengine.BreakPaginationConstraint && decision.Triggering == heading.ID {
					breakFound = true
				}
			}
			if !breakFound {
				t.Fatalf("missing pagination-constraint break before heading: %#v", projection.Breaks)
			}
		})
	}
}
