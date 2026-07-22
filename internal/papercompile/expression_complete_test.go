// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"testing"

	"github.com/cssbruno/paperrune/internal/paperlang"
)

func TestCompileScenarioSupportsDecimalUnitsAndNullableGuards(t *testing.T) {
	const source = `document:
  schema invoice:
    bool compact
    number amount
    optional string nickname
  scenario @sample:
    value @compact: false
    value @amount: 2.5
  page:
    body:
      row:
        height: amount * 10pt
        paragraph:
          visible: nickname == null || nickname matches "A*"
          size: compact ? 10pt : 12.5pt
          text: "Invoice"
`
	parsed := paperlang.Parse("complete-expressions.paper", source)
	if !parsed.OK() {
		t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
	}
	neutral := Compile(parsed.AST)
	if !neutral.OK() {
		t.Fatalf("neutral diagnostics = %#v", neutral.Diagnostics)
	}
	compiled := CompileScenario(parsed.AST, "sample")
	if !compiled.OK() {
		t.Fatalf("scenario diagnostics = %#v", compiled.Diagnostics)
	}
}

func TestCompileRejectsNullableValueWithoutGuard(t *testing.T) {
	const source = `document:
  schema patient:
    optional string nickname
  page:
    body:
      paragraph:
        text: nickname
`
	parsed := paperlang.Parse("nullable-expression.paper", source)
	if !parsed.OK() {
		t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
	}
	compiled := Compile(parsed.AST)
	if compiled.OK() || !hasCompileDiagnostic(compiled.Diagnostics, "PAPER_EXPRESSION_PROPERTY_NULLABLE") {
		t.Fatalf("diagnostics = %#v", compiled.Diagnostics)
	}
}
