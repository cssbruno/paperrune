// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
	"github.com/cssbruno/paperrune/internal/paperexpr"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

func TestCompileScenarioEvaluatesVisualWhenWithoutChangingOrdinaryCompile(t *testing.T) {
	t.Parallel()

	const source = `document @doc:
  schema invoice:
    bool show-title
    bool show-note
  scenario @sample:
    value @show-title: true
    value @show-note: false
  page:
    body:
      heading @title:
        visible: show-title
        text: "Visible title"
      paragraph @note:
        visible: show-note
        text: "Hidden note"
      paragraph @always:
        text: "Always"
`
	parsed := paperlang.Parse("conditions.paper", source)
	if !parsed.OK() {
		t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
	}
	scenario := CompileScenario(parsed.AST, "sample")
	if !scenario.OK() {
		t.Fatalf("scenario diagnostics = %#v", scenario.Diagnostics)
	}
	if len(scenario.Document.Body) != 2 {
		t.Fatalf("scenario body = %#v", scenario.Document.Body)
	}
	if got := layout.TextSegmentsPlainText(scenario.Document.Body[0].(layout.HeadingBlock).Segments); got != "Visible title" {
		t.Fatalf("conditional heading = %q", got)
	}
	if got := layout.TextSegmentsPlainText(scenario.Document.Body[1].(layout.ParagraphBlock).Segments); got != "Always" {
		t.Fatalf("unconditional paragraph = %q", got)
	}
	if mappingByID(scenario.Mapping, "@note").ID != "" {
		t.Fatalf("false node retained mapping: %#v", scenario.Mapping.Nodes)
	}
	title := mappingByID(scenario.Mapping, "@title")
	if title.ID == "" || title.Span.File != "conditions.paper" || title.Span.Start.Line != 10 {
		t.Fatalf("true node lost source mapping: %#v", title)
	}

	ordinary := Compile(parsed.AST)
	if !ordinary.OK() || len(ordinary.Document.Body) != 3 {
		t.Fatalf("ordinary compile consulted scenario: body=%#v diagnostics=%#v", ordinary.Document.Body, ordinary.Diagnostics)
	}
}

func TestCompileScenarioResolvesBareAndTernaryTextExpressions(t *testing.T) {
	t.Parallel()

	const source = `document:
  schema:
    bool paid
    string label
  scenario @sample:
    value @paid: false
    value @label: "Invoice"
  page:
    body:
      paragraph @label:
        text: label
      paragraph @status:
        text: paid ? "Paid" : "Payment pending"
      paragraph @switch-status:
        text: switch label:
          case "Invoice": "Matched invoice"
          default: "Unknown"
`
	parsed := paperlang.Parse("computed-text.paper", source)
	if !parsed.OK() {
		t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
	}
	compiled := CompileScenario(parsed.AST, "sample")
	if !compiled.OK() || len(compiled.Document.Body) != 3 {
		t.Fatalf("compile = body %#v diagnostics %#v", compiled.Document.Body, compiled.Diagnostics)
	}
	got := []string{
		layout.TextSegmentsPlainText(compiled.Document.Body[0].(layout.ParagraphBlock).Segments),
		layout.TextSegmentsPlainText(compiled.Document.Body[1].(layout.ParagraphBlock).Segments),
		layout.TextSegmentsPlainText(compiled.Document.Body[2].(layout.ParagraphBlock).Segments),
	}
	if !equalStrings(got, []string{"Invoice", "Payment pending", "Matched invoice"}) {
		t.Fatalf("computed text = %#v", got)
	}
}

func TestCompileWithoutScenarioStaticallyChecksAndDefersExpressions(t *testing.T) {
	valid := paperlang.Parse("static-expression.paper", `document:
  schema:
    string label
  page:
    body:
      paragraph:
        text: label
`)
	compiled := Compile(valid.AST)
	if !valid.OK() || !compiled.OK() || len(compiled.Document.Body) != 1 {
		t.Fatalf("neutral expression compile = body %#v diagnostics %#v / %#v", compiled.Document.Body, valid.Diagnostics, compiled.Diagnostics)
	}

	wrongType := paperlang.Parse("wrong-type.paper", `document:
  schema:
    bool active
  page:
    body:
      paragraph:
        text: active
`)
	invalid := Compile(wrongType.AST)
	if invalid.OK() || !hasCompileDiagnostic(invalid.Diagnostics, "PAPER_EXPRESSION_PROPERTY_TYPE") {
		t.Fatalf("wrong property type diagnostics = %#v", invalid.Diagnostics)
	}

	unknown := paperlang.Parse("unknown-path.paper", `document:
  schema:
    string label
  page:
    body:
      paragraph:
        text: missing
`)
	missing := Compile(unknown.AST)
	if missing.OK() || !hasCompileDiagnostic(missing.Diagnostics, "PAPER_EXPRESSION_PATH") {
		t.Fatalf("unknown path diagnostics = %#v", missing.Diagnostics)
	}
	for _, diagnostic := range missing.Diagnostics {
		if diagnostic.Code == "PAPER_EXPRESSION_PATH" && diagnostic.Span.Start.Column != 15 {
			t.Fatalf("unknown path span = %#v", diagnostic.Span)
		}
	}
}

func TestVisibilityRevalidatesRemainingDocumentStructure(t *testing.T) {
	empty := paperlang.Parse("empty-visible.paper", `document:
  schema:
    bool show
  scenario @hidden:
    value @show: false
  page:
    body:
      paragraph:
        visible: show
        text: "Hidden"
`)
	emptyResult := CompileScenario(empty.AST, "hidden")
	if emptyResult.OK() || !hasCompileDiagnostic(emptyResult.Diagnostics, "PAPER_COMPILE_EMPTY_VISIBLE_BODY") {
		t.Fatalf("empty visible body diagnostics = %#v", emptyResult.Diagnostics)
	}

	canvas := paperlang.Parse("hidden-target.paper", `document:
  schema:
    bool show-target
  scenario @hidden:
    value @show-target: false
  page:
    body:
      canvas:
        width: 100pt
        height: 40pt
        anchor @target:
          visible: show-target
          width: 10pt
          height: 10pt
          left: "canvas.left"
          top: "canvas.top"
        anchor @dependent:
          width: 10pt
          height: 10pt
          left: "@target.right"
          top: "canvas.top"
`)
	canvasResult := CompileScenario(canvas.AST, "hidden")
	if canvasResult.OK() || !hasCompileDiagnostic(canvasResult.Diagnostics, "PAPER_COMPILE_CANVAS_TARGET_HIDDEN") {
		t.Fatalf("hidden canvas target diagnostics = %#v", canvasResult.Diagnostics)
	}
}

func TestCompileScenarioEvaluatesRepeatItemRelativeWhen(t *testing.T) {
	t.Parallel()

	const source = `document @doc:
  schema invoice:
    list object items:
      max-items: 4
      string name
      bool active
  scenario @sample:
    keyed-list @items:
      object @line-a:
        value @name: "Alpha"
        value @active: true
      object @line-b:
        value @name: "Beta"
        value @active: false
      object @line-c:
        value @name: "Gamma"
        value @active: true
  page:
    body:
      repeat @lines:
        source: "items"
        instance-prefix: "lines"
        max-items: 3
        paragraph @line:
          visible: item.active && item.name matches "*a"
          bind: "name"
          text: "placeholder"
`
	parsed := paperlang.Parse("repeat-conditions.paper", source)
	if !parsed.OK() {
		t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
	}
	compiled := CompileScenario(parsed.AST, "sample")
	if !compiled.OK() || len(compiled.Document.Body) != 2 {
		t.Fatalf("compile result = body %#v diagnostics %#v", compiled.Document.Body, compiled.Diagnostics)
	}
	got := make([]string, len(compiled.Document.Body))
	for index := range compiled.Document.Body {
		got[index] = layout.TextSegmentsPlainText(compiled.Document.Body[index].(layout.ParagraphBlock).Segments)
	}
	if !equalStrings(got, []string{"Alpha", "Gamma"}) {
		t.Fatalf("conditional repeat output = %#v", got)
	}
	for _, mapping := range compiled.Mapping.Nodes {
		if mapping.Kind == paperlang.NodeParagraph && strings.Contains(mapping.InstancePath, "line-b") {
			t.Fatalf("false repeat item retained provenance: %#v", mapping)
		}
	}
	for _, key := range []string{"line-a", "line-c"} {
		found := false
		for _, mapping := range compiled.Mapping.Nodes {
			if mapping.Kind == paperlang.NodeParagraph && strings.Contains(mapping.InstancePath, key) {
				found = mapping.DefinitionSpan.File == "repeat-conditions.paper" && mapping.InvocationSpan.File == "repeat-conditions.paper"
			}
		}
		if !found {
			t.Fatalf("repeat item %s lost instance provenance: %#v", key, compiled.Mapping.Nodes)
		}
	}
}

func TestCompileScenarioDiagnosesWhenPathTypeRuntimeBindingAndLimits(t *testing.T) {
	t.Parallel()

	const base = `document:
  schema invoice:
    optional bool active
    number quantity
    string name
  scenario @sample:
    value @quantity: 2
    value @name: "Ada"
%s
  page:
    body:
      paragraph:
        visible: %s
        text: "conditional"
`
	tests := []struct {
		name       string
		fixture    string
		expression string
		code       string
		limits     *ScenarioCompileLimits
	}{
		{name: "unknown-path", fixture: "    value @active: true", expression: `missing`, code: "PAPER_VISIBLE_PATH"},
		{name: "non-bool-result", fixture: "    value @active: true", expression: `quantity + 1`, code: "PAPER_VISIBLE_TYPE"},
		{name: "matches-type", fixture: "    value @active: true", expression: `quantity matches "*"`, code: "PAPER_VISIBLE_TYPE"},
		{name: "invalid-match-pattern", fixture: "    value @active: true", expression: `name matches "bad\\"`, code: "PAPER_VISIBLE_EXPRESSION"},
		{name: "missing-runtime-binding", expression: `active`, code: "PAPER_VISIBLE_PATH"},
		{name: "wrong-property-type", fixture: "    value @active: true", expression: `12`, code: "PAPER_VISIBLE_VALUE"},
	}
	bounded := paperexpr.DefaultLanguageLimits()
	bounded.MaxSourceBytes = 4
	tests = append(tests, struct {
		name       string
		fixture    string
		expression string
		code       string
		limits     *ScenarioCompileLimits
	}{name: "expression-limit", fixture: "    value @active: true", expression: `active`, code: "PAPER_VISIBLE_LIMIT", limits: &ScenarioCompileLimits{Expressions: bounded}})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parsed := paperlang.Parse(test.name+".paper", fmt.Sprintf(base, test.fixture, test.expression))
			if !parsed.OK() {
				t.Fatalf("parse diagnostics = %#v", parsed.Diagnostics)
			}
			var compiled Result
			if test.limits == nil {
				compiled = CompileScenario(parsed.AST, "sample")
			} else {
				compiled = CompileScenarioWithLimits(parsed.AST, "sample", *test.limits)
			}
			if compiled.OK() || !hasCompileDiagnostic(compiled.Diagnostics, test.code) {
				t.Fatalf("diagnostics = %#v, want %s", compiled.Diagnostics, test.code)
			}
			for _, diagnostic := range compiled.Diagnostics {
				if diagnostic.Code == test.code && (diagnostic.Span.File == "" || diagnostic.Span.Start.Line == 0) {
					t.Fatalf("diagnostic is not source-located: %#v", diagnostic)
				}
			}
		})
	}
}
