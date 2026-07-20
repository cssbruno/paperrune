// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperlang

import "testing"

func TestRowColumnGrammarAndFormatterRoundTrip(t *testing.T) {
	const source = "document:\n" +
		"  page:\n" +
		"    body:\n" +
		"      row @summary:\n" +
		"        gap: 8pt\n" +
		"        align-items: \"center\"\n" +
		"        heading @label:\n" +
		"          width: 72pt\n" +
		"          text: \"Label\"\n" +
		"        paragraph @value:\n" +
		"          width: 2fr\n" +
		"          align-self: \"end\"\n" +
		"          text: \"Value\"\n"
	parsed := Parse("row.paper", source)
	if !parsed.OK() {
		t.Fatalf("Parse() diagnostics = %+v", parsed.Diagnostics)
	}
	body := parsed.AST.Root.Members[0].Node.Members[0].Node
	row := body.Members[0].Node
	if row.Kind != NodeRow || row.ID != "@summary" || row.Members[2].Node.Kind != NodeHeading || row.Members[3].Node.Kind != NodeParagraph {
		t.Fatalf("row AST = %#v", row)
	}
	formatted, err := Format(parsed.AST)
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	reparsed := Parse("row.paper", string(formatted))
	if !reparsed.OK() {
		t.Fatalf("formatted Parse() diagnostics = %+v\n%s", reparsed.Diagnostics, formatted)
	}
	second, err := Format(reparsed.AST)
	if err != nil || string(second) != string(formatted) {
		t.Fatalf("formatter is not idempotent: %v\n%s\n%s", err, formatted, second)
	}
}

func TestRowColumnRejectsUnsupportedNestedNodes(t *testing.T) {
	parsed := Parse("bad-row.paper", "document:\n  page:\n    body:\n      row:\n        list:\n          item:\n            text: \"no\"\n")
	if parsed.OK() {
		t.Fatal("row unexpectedly accepted a list child")
	}
}
