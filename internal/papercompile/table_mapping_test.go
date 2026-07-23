// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"testing"

	"github.com/cssbruno/paperrune/internal/paperlang"
)

func TestCompileMapsTableHeaderAndBodyCellContentExactly(t *testing.T) {
	source := "document @doc:\n" +
		"  page @page:\n" +
		"    body @body:\n" +
		"      paragraph @lead:\n" +
		"        text: \"Lead\"\n" +
		"      table @results:\n" +
		"        table-column:\n" +
		"          width: 50%\n" +
		"        table-column:\n" +
		"          width: 50%\n" +
		"        table-header:\n" +
		"          table-row:\n" +
		"            cell:\n" +
		"              text: \"ANALYTE\"\n" +
		"            cell:\n" +
		"              text: \"RESULT\"\n" +
		"        table-row:\n" +
		"          cell:\n" +
		"            text: \"Leukocytes\"\n" +
		"          cell:\n" +
		"            text: \"5.8\"\n"
	parsed := paperlang.Parse("table.paper", source)
	if !parsed.OK() {
		t.Fatalf("Parse() diagnostics = %#v", parsed.Diagnostics)
	}
	compiled := Compile(parsed.AST)
	if !compiled.OK() {
		t.Fatalf("Compile() diagnostics = %#v", compiled.Diagnostics)
	}

	type coordinate struct {
		kind                       paperlang.NodeKind
		body, segment, nestedBlock int
	}
	var got []coordinate
	for _, mapping := range compiled.Mapping.AnonymousNodes {
		if mapping.BodyIndex != 1 || mapping.Kind != paperlang.NodeTableCell && mapping.Kind != paperlang.NodeText {
			continue
		}
		got = append(got, coordinate{mapping.Kind, mapping.BodyIndex, mapping.SegmentIndex, mapping.NestedBlockIndex})
	}
	want := []coordinate{
		{paperlang.NodeTableCell, 1, 0, -1}, {paperlang.NodeText, 1, 0, 0},
		{paperlang.NodeTableCell, 1, 1, -1}, {paperlang.NodeText, 1, 1, 0},
		{paperlang.NodeTableCell, 1, 2, -1}, {paperlang.NodeText, 1, 2, 0},
		{paperlang.NodeTableCell, 1, 3, -1}, {paperlang.NodeText, 1, 3, 0},
	}
	if len(got) != len(want) {
		t.Fatalf("table mappings = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("table mapping[%d] = %#v, want %#v (all=%#v)", index, got[index], want[index], got)
		}
	}
}
