// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"fmt"
	"sort"

	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/internal/paperscenario"
)

// maxScenarioSchemaDiagnostics bounds the report for one badly shaped fixture.
const maxScenarioSchemaDiagnostics = 32

// validateScenarioFixture checks a resolved .paper fixture against the
// declared schemas with the same rules as external JSON data: kinds must
// match without coercion, required values must be present and non-null, lists
// respect max-items, and undeclared fields are rejected.
//
// Fixture paths omit the schema name, so every schema reads the same fixture
// root. With one schema the root is that schema's object. With several, a root
// field must be declared by at least one schema and must satisfy every schema
// that declares it; required root fields are not enforced because a scenario
// does not select one schema.
func validateScenarioFixture(ast paperlang.AST, fixture paperscenario.Fixture, schemas schemaAnalysis) []paperlang.Diagnostic {
	if len(schemas.descriptors) == 0 {
		return nil
	}
	validator := scenarioSchemaValidator{span: scenarioSpan(ast, fixture.Name)}
	if len(schemas.descriptors) == 1 {
		validator.object(fixture.Values, schemas.descriptors[0].Fields, "", true)
		return validator.diagnostics
	}
	declared := make(map[string][]FieldDescriptor)
	for _, schema := range schemas.descriptors {
		for _, field := range schema.Fields {
			declared[field.Name] = append(declared[field.Name], field)
		}
	}
	for _, field := range fixture.Values {
		descriptors := declared[field.Name]
		if len(descriptors) == 0 {
			validator.add(field.Name, "field is not declared by any schema")
			continue
		}
		for _, descriptor := range descriptors {
			validator.value(field.Value, descriptor, field.Name)
		}
	}
	return validator.diagnostics
}

type scenarioSchemaValidator struct {
	span        paperlang.Span
	diagnostics []paperlang.Diagnostic
}

func (v *scenarioSchemaValidator) add(path, problem string) {
	if len(v.diagnostics) >= maxScenarioSchemaDiagnostics {
		return
	}
	if path == "" {
		path = "(root)"
	}
	v.diagnostics = append(v.diagnostics, paperlang.Diagnostic{
		Code: "PAPER_SCENARIO_SCHEMA", Severity: paperlang.SeverityError,
		Message: fmt.Sprintf("scenario value %s: %s", path, problem),
		Hint:    "make the scenario fixture match the declared schema", Span: v.span,
	})
}

func (v *scenarioSchemaValidator) object(fields []paperscenario.Field, descriptors []FieldDescriptor, path string, requireFields bool) {
	known := make(map[string]bool, len(descriptors))
	for _, descriptor := range descriptors {
		known[descriptor.Name] = true
		childPath := joinScenarioPath(path, descriptor.Name)
		value, found := scenarioField(fields, descriptor.Name)
		if !found {
			if requireFields && descriptor.Required {
				v.add(childPath, "required field is missing")
			}
			continue
		}
		v.value(value, descriptor, childPath)
	}
	unknown := make([]string, 0)
	for _, field := range fields {
		if !known[field.Name] {
			unknown = append(unknown, field.Name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		v.add(joinScenarioPath(path, name), "field is not declared by the schema")
	}
}

func (v *scenarioSchemaValidator) value(value paperscenario.Value, descriptor FieldDescriptor, path string) {
	if value.Kind == paperscenario.Null {
		if descriptor.Required {
			v.add(path, "required value cannot be null")
		}
		return
	}
	expected := map[SchemaKind]paperscenario.Kind{
		SchemaString: paperscenario.String, SchemaNumber: paperscenario.Number, SchemaBool: paperscenario.Bool,
		SchemaObject: paperscenario.Object, SchemaList: paperscenario.List,
	}[descriptor.Kind]
	if expected == "" {
		v.add(path, fmt.Sprintf("schema kind %q is unsupported", descriptor.Kind))
		return
	}
	if value.Kind != expected {
		v.add(path, fmt.Sprintf("expected %s, got %s", descriptor.Kind, value.Kind))
		return
	}
	switch descriptor.Kind {
	case SchemaObject:
		v.object(value.Object, descriptor.Fields, path, true)
	case SchemaList:
		if uint32(len(value.List)) > descriptor.MaxItems { // #nosec G115 -- fixture lists are bounded by scenario limits.
			v.add(path, fmt.Sprintf("list has %d items; schema permits at most %d", len(value.List), descriptor.MaxItems))
		}
		item := FieldDescriptor{Kind: descriptor.ItemKind, Required: descriptor.ItemRequired, Fields: descriptor.Fields}
		for _, entry := range value.List {
			v.value(entry.Value, item, path+"["+entry.Key+"]")
		}
	}
}

func scenarioField(fields []paperscenario.Field, name string) (paperscenario.Value, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return paperscenario.Value{}, false
}

func joinScenarioPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// scenarioSpan locates the selected scenario declaration for diagnostics.
func scenarioSpan(ast paperlang.AST, name string) paperlang.Span {
	if ast.Root != nil {
		for _, member := range ast.Root.Members {
			if member.Node != nil && member.Node.Kind == paperlang.NodeScenario && member.Node.ID == "@"+name {
				return member.Node.HeaderSpan
			}
		}
	}
	return rootSpan(ast)
}
