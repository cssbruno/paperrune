// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/internal/paperscenario"
)

func TestCompileScenarioSourceExtractsResolvableDeterministicFixtures(t *testing.T) {
	source := "document @invoice:\n" +
		"  scenario @child:\n" +
		"    parent: \"@base\"\n" +
		"    locale: \"pt-BR\"\n" +
		"    value @paid: true\n" +
		"    value @nothing: null\n" +
		"    object @customer:\n" +
		"      value @name: \"Ada\"\n" +
		"      value @score: +12.50\n" +
		"    keyed-list @lines:\n" +
		"      object @line-a:\n" +
		"        value @sku: \"A-1\"\n" +
		"        value @quantity: 2\n" +
		"      value @summary: \"one line\"\n" +
		"  scenario @base:\n" +
		"    locale: \"en-US\"\n" +
		"    value @currency: \"USD\"\n" +
		"  page:\n" +
		"    body:\n" +
		"      text: \"preview\"\n"
	first := CompileScenarioSource("invoice.paper", source)
	second := CompileScenarioSource("invoice.paper", source)
	if !first.OK() || !reflect.DeepEqual(first, second) {
		t.Fatalf("scenario compile = %#v / %#v", first, second)
	}
	want := []paperscenario.Scenario{
		{
			Name: "child", Parent: "base", Locale: "pt-BR",
			Values: []paperscenario.Field{
				{Name: "paid", Value: paperscenario.Value{Kind: paperscenario.Bool, Bool: true}},
				{Name: "nothing", Value: paperscenario.Value{Kind: paperscenario.Null}},
				{Name: "customer", Value: paperscenario.Value{Kind: paperscenario.Object, Object: []paperscenario.Field{
					{Name: "name", Value: paperscenario.Value{Kind: paperscenario.String, String: "Ada"}},
					{Name: "score", Value: paperscenario.Value{Kind: paperscenario.Number, Number: "12.5"}},
				}}},
				{Name: "lines", Value: paperscenario.Value{Kind: paperscenario.List, List: []paperscenario.Item{
					{Key: "line-a", Value: paperscenario.Value{Kind: paperscenario.Object, Object: []paperscenario.Field{
						{Name: "sku", Value: paperscenario.Value{Kind: paperscenario.String, String: "A-1"}},
						{Name: "quantity", Value: paperscenario.Value{Kind: paperscenario.Number, Number: "2"}},
					}}},
					{Key: "summary", Value: paperscenario.Value{Kind: paperscenario.String, String: "one line"}},
				}}},
			},
		},
		{Name: "base", Locale: "en-US", Values: []paperscenario.Field{{Name: "currency", Value: paperscenario.Value{Kind: paperscenario.String, String: "USD"}}}},
	}
	if !reflect.DeepEqual(first.Scenarios, want) {
		t.Fatalf("scenarios = %#v\nwant %#v", first.Scenarios, want)
	}
	fixtures, err := paperscenario.Resolve(first.Scenarios, paperscenario.Limits{})
	if err != nil || len(fixtures) != 2 || fixtures[0].Locale != "pt-BR" {
		t.Fatalf("Resolve() = %#v, %v", fixtures, err)
	}

	// Scenario declarations are compile-time data and must not appear as visual
	// nodes in the ordinary layout compiler.
	parsed := paperlang.Parse("invoice.paper", source)
	layoutResult := Compile(parsed.AST)
	if !layoutResult.OK() {
		t.Fatalf("layout compiler did not ignore scenario declarations: %#v", layoutResult.Diagnostics)
	}
}

func TestCompileScenarioSourceReportsLocatedSemanticDiagnostics(t *testing.T) {
	source := "document:\n" +
		"  scenario @broken:\n" +
		"    parent: \"missing\"\n" +
		"    mystery: true\n" +
		"    value @distance: 12pt\n" +
		"    value @same: 1\n" +
		"    value @same: 2\n" +
		"    keyed-list @lines:\n" +
		"      value @one: \"a\"\n" +
		"      value @one: \"b\"\n"
	first := CompileScenarioSource("broken.paper", source)
	second := CompileScenarioSource("broken.paper", source)
	if first.OK() || !reflect.DeepEqual(first.Diagnostics, second.Diagnostics) {
		t.Fatalf("diagnostics = %#v / %#v", first.Diagnostics, second.Diagnostics)
	}
	codes := make(map[string]bool)
	for _, diagnostic := range first.Diagnostics {
		codes[diagnostic.Code] = true
		if diagnostic.Span.File != "broken.paper" || diagnostic.Span.Start.Line == 0 {
			t.Fatalf("diagnostic is not source located: %#v", diagnostic)
		}
	}
	for _, code := range []string{"PAPER_SCENARIO_PROPERTY", "PAPER_SCENARIO_UNIT", "PAPER_SCENARIO_FIELD_DUPLICATE", "PAPER_SCENARIO_LIST_KEY_DUPLICATE", "PAPER_SCENARIO_PARENT_MISSING"} {
		if !codes[code] {
			t.Fatalf("codes = %#v, want %s; diagnostics=%#v", codes, code, first.Diagnostics)
		}
	}
}

func TestExtractScenariosEnforcesExistingHardLimits(t *testing.T) {
	source := "document:\n  scenario @bounded:\n    object @outer:\n      value @inner: 1\n"
	limits := paperscenario.DefaultLimits()
	limits.MaxNodes = 1
	result := CompileScenarioSourceWithLimits("bounded.paper", source, limits)
	if result.OK() || len(result.Diagnostics) == 0 || result.Diagnostics[len(result.Diagnostics)-1].Code != "PAPER_SCENARIO_NODE_LIMIT" {
		t.Fatalf("limited result = %#v", result)
	}
}

func TestCompileScenarioSourceDetectsParentCycles(t *testing.T) {
	source := "document:\n  scenario @a:\n    parent: \"b\"\n  scenario @b:\n    parent: \"a\"\n"
	result := CompileScenarioSource("cycle.paper", source)
	if result.OK() {
		t.Fatal("cycle unexpectedly accepted")
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		found = found || diagnostic.Code == "PAPER_SCENARIO_PARENT_CYCLE"
	}
	if !found {
		t.Fatalf("cycle diagnostics = %#v", result.Diagnostics)
	}
}

func TestCompileScenarioSourceKeepsAuthoredNumberPrecision(t *testing.T) {
	source := "document @invoice:\n" +
		"  scenario @exact:\n" +
		"    value @total: 12345678901234567.89\n" +
		"    value @tiny: -0.000000000000000001\n" +
		"    value @zero: -0.00\n" +
		"  page:\n" +
		"    body:\n" +
		"      text: \"preview\"\n"
	compiled := CompileScenarioSource("invoice.paper", source)
	if !compiled.OK() || len(compiled.Scenarios) != 1 {
		t.Fatalf("scenario compile = %#v", compiled)
	}
	want := []paperscenario.Field{
		{Name: "total", Value: paperscenario.Value{Kind: paperscenario.Number, Number: "12345678901234567.89"}},
		{Name: "tiny", Value: paperscenario.Value{Kind: paperscenario.Number, Number: "-0.000000000000000001"}},
		{Name: "zero", Value: paperscenario.Value{Kind: paperscenario.Number, Number: "0"}},
	}
	if !reflect.DeepEqual(compiled.Scenarios[0].Values, want) {
		t.Fatalf("fixture values = %#v", compiled.Scenarios[0].Values)
	}
}

func TestVisibleConditionRejectsWrongFixtureKinds(t *testing.T) {
	for value, wantOK := range map[string]bool{"null": false, "\"yes\"": false, "true": true, "false": true} {
		source := "document:\n" +
			"  schema s:\n" +
			"    bool hidden\n" +
			"  scenario @demo:\n" +
			"    value @hidden: " + value + "\n" +
			"  page:\n" +
			"    body:\n" +
			"      paragraph @p:\n" +
			"        visible: !hidden\n" +
			"        text: \"shown\"\n" +
			"      paragraph @q:\n" +
			"        text: \"other\"\n"
		compiled := CompileScenario(paperlang.Parse("visible.paper", source).AST, "demo")
		if compiled.OK() != wantOK || !wantOK && !hasCompileDiagnostic(compiled.Diagnostics, "PAPER_VISIBLE_TYPE") {
			t.Fatalf("hidden=%s diagnostics = %+v", value, compiled.Diagnostics)
		}
	}
}

func TestCompileScenarioValidatesFixtureAgainstSchema(t *testing.T) {
	const page = "  page:\n    body:\n      paragraph:\n        text: \"preview\"\n"
	const schema = "  schema invoice:\n" +
		"    string number\n" +
		"    optional string note\n" +
		"    object address:\n" +
		"      string city\n" +
		"      string street\n" +
		"    list string tags:\n" +
		"      max-items: 2\n"
	valid := "  scenario @ok:\n" +
		"    value @number: \"INV-1\"\n" +
		"    object @address:\n" +
		"      value @city: \"London\"\n" +
		"      value @street: \"1 Way\"\n" +
		"    keyed-list @tags:\n" +
		"      value @a: \"x\"\n"
	for name, test := range map[string]struct {
		scenario string
		want     []string
	}{
		"valid":          {scenario: valid},
		"optional null":  {scenario: valid + "    value @note: null\n"},
		"undeclared":     {scenario: valid + "    value @extra: 1\n", want: []string{"extra: field is not declared"}},
		"nested missing": {scenario: "  scenario @ok:\n    value @number: \"INV-1\"\n    object @address:\n      value @city: \"London\"\n    keyed-list @tags:\n      value @a: \"x\"\n", want: []string{"address.street: required field is missing"}},
		"wrong kind":     {scenario: valid + "    value @note: true\n", want: []string{"note: expected string, got bool"}},
		"too many items": {scenario: "  scenario @ok:\n    value @number: \"INV-1\"\n    object @address:\n      value @city: \"L\"\n      value @street: \"S\"\n    keyed-list @tags:\n      value @a: \"x\"\n      value @b: \"y\"\n      value @c: \"z\"\n", want: []string{"tags: list has 3 items; schema permits at most 2"}},
		"list item kind": {scenario: "  scenario @ok:\n    value @number: \"INV-1\"\n    object @address:\n      value @city: \"L\"\n      value @street: \"S\"\n    keyed-list @tags:\n      value @a: 7\n", want: []string{"tags[a]: expected string, got number"}},
	} {
		t.Run(name, func(t *testing.T) {
			parsed := paperlang.Parse("schema.paper", "document:\n"+schema+test.scenario+page)
			if !parsed.OK() {
				t.Fatalf("parse diagnostics = %+v", parsed.Diagnostics)
			}
			var got []string
			for _, diagnostic := range CompileScenario(parsed.AST, "ok").Diagnostics {
				if diagnostic.Code == "PAPER_SCENARIO_SCHEMA" {
					got = append(got, diagnostic.Message)
				}
			}
			if len(got) != len(test.want) {
				t.Fatalf("schema diagnostics = %q, want %q", got, test.want)
			}
			for index, want := range test.want {
				if !strings.Contains(got[index], want) {
					t.Fatalf("schema diagnostics = %q, want %q", got, test.want)
				}
			}
		})
	}
}

func TestCompileScenarioSchemaRulesWithInheritanceAndSeveralSchemas(t *testing.T) {
	const page = "  page:\n    body:\n      paragraph:\n        text: \"preview\"\n"
	inherited := "document:\n" +
		"  schema invoice:\n" +
		"    object address:\n" +
		"      string city\n" +
		"      string street\n" +
		"  scenario @base:\n" +
		"    object @address:\n" +
		"      value @city: \"Paris\"\n" +
		"      value @street: \"1 Rue\"\n" +
		"  scenario @child:\n" +
		"    parent: \"@base\"\n" +
		"    object @address:\n" +
		"      value @city: \"London\"\n" + page
	if compiled := CompileScenario(paperlang.Parse("inherit.paper", inherited).AST, "child"); !compiled.OK() {
		t.Fatalf("child overriding one inherited field = %+v", compiled.Diagnostics)
	}
	several := "document:\n" +
		"  schema invoice:\n" +
		"    string number\n" +
		"  schema patient:\n" +
		"    string name\n" +
		"    number number\n" +
		"  scenario @mixed:\n" +
		"    value @name: \"Ada\"\n" +
		"    value @number: \"INV-1\"\n" +
		"    value @other: true\n" + page
	var messages []string
	for _, diagnostic := range CompileScenario(paperlang.Parse("several.paper", several).AST, "mixed").Diagnostics {
		if diagnostic.Code == "PAPER_SCENARIO_SCHEMA" {
			messages = append(messages, diagnostic.Message)
		}
	}
	want := []string{"scenario value number: expected number, got string", "scenario value other: field is not declared by any schema"}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("several-schema diagnostics = %q, want %q", messages, want)
	}
}
