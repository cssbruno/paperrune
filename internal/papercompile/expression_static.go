// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/paperrune/internal/paperexpr"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

type expressionExpectation struct {
	kind     paperexpr.Kind
	optional bool
}

var booleanExpressionProperties = map[string]bool{
	"visible": true, "bold": true, "italic": true, "decorative": true,
	"page-numbers": true, "page-number-hide-first": true, "repeat-header": true,
	"header-cell": true, "bind-required": true,
}

var integerExpressionProperties = map[string]bool{
	"level": true, "span": true, "colspan": true, "rowspan": true,
	"start": true, "page-number-start": true, "format-min-fraction": true,
	"format-max-fraction": true,
}

var stringExpressionProperties = map[string]bool{
	"font": true, "color": true, "align": true, "alt": true, "caption": true,
	"source": true, "format": true, "format-locale": true, "format-currency": true,
	"page-number-format": true, "page-number-align": true, "page-number-position": true,
}

var unitExpressionProperties = map[string]bool{
	"size": true, "line-height": true, "width": true, "min-width": true, "max-width": true,
	"height": true, "min-height": true, "max-height": true, "gap": true, "line-gap": true,
	"margin": true, "margin-top": true, "margin-right": true, "margin-bottom": true, "margin-left": true,
	"padding": true, "padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true,
	"border-width": true, "border-top-width": true, "border-right-width": true, "border-bottom-width": true, "border-left-width": true,
	"border-radius": true, "left": true, "right": true, "top": true, "bottom": true, "center-x": true, "center-y": true,
}

func staticExpressionDiagnostics(ast paperlang.AST, schemas schemaAnalysis, limits paperexpr.LanguageLimits) []paperlang.Diagnostic {
	rootEnvironment := staticSchemaEnvironment(schemas)
	var diagnostics []paperlang.Diagnostic
	var walk func(*paperlang.Node, []paperexpr.PathKind, []FieldDescriptor)
	walk = func(node *paperlang.Node, environment []paperexpr.PathKind, itemFields []FieldDescriptor) {
		if node == nil {
			return
		}
		current := environment
		currentItems := itemFields
		if node.Kind == paperlang.NodeComponent {
			current = append(append([]paperexpr.PathKind(nil), rootEnvironment...), componentArgsEnvironment(node)...)
		}
		if node.Kind == paperlang.NodeRepeat {
			fields, err := staticRepeatFields(node, schemas, itemFields)
			if err == nil {
				currentItems = fields
				current = repeatExpressionEnvironment(fields, "item")
			}
		}
		if node.Kind == paperlang.NodeLoop {
			current = append(append([]paperexpr.PathKind(nil), current...),
				paperexpr.PathKind{Path: "loop.first", Kind: paperexpr.Bool},
				paperexpr.PathKind{Path: "loop.index", Kind: paperexpr.Integer},
				paperexpr.PathKind{Path: "loop.last", Kind: paperexpr.Bool})
		}
		current = uniqueExpressionEnvironment(current)
		check := func(scalar *paperlang.Scalar, expectation *expressionExpectation, subject string) {
			if scalar == nil || scalar.Kind != paperlang.ScalarExpression || scalar.ExpressionValue == nil {
				return
			}
			source := strings.TrimSpace(*scalar.ExpressionValue)
			program, kind, err := paperexpr.Compile(source, current, limits)
			if err != nil {
				diagnostics = append(diagnostics, staticExpressionDiagnostic(err, source, scalar.Span))
				return
			}
			if expectation != nil && kind != expectation.kind && (!expectation.optional || kind != paperexpr.Null) {
				diagnostics = append(diagnostics, paperlang.Diagnostic{Code: "PAPER_EXPRESSION_PROPERTY_TYPE", Severity: paperlang.SeverityError,
					Message: fmt.Sprintf("%s expression returns %s, expected %s", subject, expressionKindName(kind), expressionKindName(expectation.kind)),
					Hint:    "make the expression result match the receiving property", Span: scalar.Span})
			} else if expectation != nil && program.ResultNullable && !expectation.optional {
				diagnostics = append(diagnostics, paperlang.Diagnostic{Code: "PAPER_EXPRESSION_PROPERTY_NULLABLE", Severity: paperlang.SeverityError,
					Message: fmt.Sprintf("%s expression may return null", subject), Hint: "guard the optional value or return a non-null fallback", Span: scalar.Span})
			}
		}
		if node.Value != nil {
			var expectation *expressionExpectation
			if node.Kind == paperlang.NodeText {
				value := expressionExpectation{kind: paperexpr.String}
				expectation = &value
			}
			check(node.Value, expectation, string(node.Kind))
		}
		for _, member := range node.Members {
			if member.Property != nil {
				if node.Kind == paperlang.NodeUse && member.Property.Name == "component" && member.Property.Value.Kind == paperlang.ScalarExpression && member.Property.Value.ExpressionValue != nil {
					source := strings.TrimSpace(*member.Property.Value.ExpressionValue)
					if _, err := paperexpr.ValidateComponentSelection(source, limits); err != nil {
						diagnostics = append(diagnostics, paperlang.Diagnostic{Code: "PAPER_COMPONENT_SELECTION_TYPE", Severity: paperlang.SeverityError,
							Message: err.Error(), Hint: "return only unquoted @component references or null", Span: locatedExpressionSpan(member.Property.Value.Span, source, err)})
						continue
					}
				}
				expectation, known := propertyExpressionExpectation(node.Kind, member.Property.Name)
				if known {
					check(&member.Property.Value, &expectation, member.Property.Name)
				} else {
					check(&member.Property.Value, nil, member.Property.Name)
				}
			}
		}
		for _, member := range node.Members {
			walk(member.Node, current, currentItems)
		}
	}
	walk(ast.Root, rootEnvironment, nil)
	return diagnostics
}

func propertyExpressionExpectation(nodeKind paperlang.NodeKind, name string) (expressionExpectation, bool) {
	if nodeKind == paperlang.NodeUse && name == "component" {
		return expressionExpectation{kind: paperexpr.String, optional: true}, true
	}
	if booleanExpressionProperties[name] {
		return expressionExpectation{kind: paperexpr.Bool, optional: name != "visible"}, true
	}
	if nodeKind == paperlang.NodePage && name == "size" {
		return expressionExpectation{kind: paperexpr.String, optional: true}, true
	}
	if integerExpressionProperties[name] {
		return expressionExpectation{kind: paperexpr.Integer, optional: true}, true
	}
	if stringExpressionProperties[name] {
		return expressionExpectation{kind: paperexpr.String, optional: true}, true
	}
	if unitExpressionProperties[name] {
		return expressionExpectation{kind: paperexpr.Unit, optional: true}, true
	}
	return expressionExpectation{}, false
}

func staticSchemaEnvironment(schemas schemaAnalysis) []paperexpr.PathKind {
	var environment []paperexpr.PathKind
	for _, schema := range schemas.descriptors {
		environment = append(environment, repeatExpressionEnvironment(schema.Fields, "")...)
	}
	return uniqueExpressionEnvironment(environment)
}

func componentArgsEnvironment(component *paperlang.Node) []paperexpr.PathKind {
	var environment []paperexpr.PathKind
	for _, member := range component.Members {
		if member.Node == nil || member.Node.Kind != paperlang.NodeProp {
			continue
		}
		name := strings.TrimPrefix(member.Node.ID, "@")
		required, hasDefault := false, false
		for _, contract := range member.Node.Members {
			if contract.Property == nil {
				continue
			}
			if contract.Property.Name == "required" && contract.Property.Value.BoolValue != nil {
				required = *contract.Property.Value.BoolValue
			}
			if contract.Property.Name == "default" {
				hasDefault = true
			}
		}
		for _, contract := range member.Node.Members {
			if contract.Property == nil || contract.Property.Name != "type" || contract.Property.Value.StringValue == nil {
				continue
			}
			kind, ok := componentExpressionKind(strings.TrimSpace(*contract.Property.Value.StringValue))
			if !ok {
				continue
			}
			environment = append(environment, paperexpr.PathKind{Path: "args." + name, Kind: kind, Optional: !required && !hasDefault})
		}
	}
	return environment
}

func staticRepeatFields(node *paperlang.Node, schemas schemaAnalysis, parent []FieldDescriptor) ([]FieldDescriptor, error) {
	source := findNodeProperty(node, "source")
	if source == nil || source.Value.StringValue == nil {
		return nil, errors.New("repeat source is unavailable")
	}
	path := strings.TrimSpace(*source.Value.StringValue)
	if len(parent) != 0 {
		return nestedRepeatSchemaItem(parent, path)
	}
	canonical, err := qualifySchemaPath(path, schemas)
	if err != nil {
		return nil, err
	}
	return repeatSchemaItem(canonical, schemas)
}

func uniqueExpressionEnvironment(environment []paperexpr.PathKind) []paperexpr.PathKind {
	contracts := make(map[string]paperexpr.PathKind)
	conflicts := make(map[string]bool)
	for _, entry := range environment {
		if prior, exists := contracts[entry.Path]; exists && prior.Kind != entry.Kind {
			conflicts[entry.Path] = true
			continue
		}
		if prior, exists := contracts[entry.Path]; exists {
			entry.Optional = entry.Optional || prior.Optional
		}
		contracts[entry.Path] = entry
	}
	result := make([]paperexpr.PathKind, 0, len(contracts))
	for path, contract := range contracts {
		if !conflicts[path] {
			contract.Path = path
			result = append(result, contract)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func staticExpressionDiagnostic(err error, source string, span paperlang.Span) paperlang.Diagnostic {
	code := "PAPER_EXPRESSION_STATIC"
	if errors.Is(err, paperexpr.ErrBinding) {
		code = "PAPER_EXPRESSION_PATH"
	} else if errors.Is(err, paperexpr.ErrType) {
		code = "PAPER_EXPRESSION_TYPE"
	} else if errors.Is(err, paperexpr.ErrLimit) {
		code = "PAPER_EXPRESSION_LIMIT"
	}
	span = locatedExpressionSpan(span, source, err)
	return paperlang.Diagnostic{Code: code, Severity: paperlang.SeverityError, Message: err.Error(), Hint: "fix the typed expression", Span: span}
}

func locatedExpressionSpan(span paperlang.Span, source string, err error) paperlang.Span {
	if located := new(paperexpr.ExpressionError); errors.As(err, &located) {
		return expressionErrorSpan(span, source, located.Offset, located.End)
	}
	return span
}

func expressionErrorSpan(span paperlang.Span, source string, start, end uint32) paperlang.Span {
	if uint64(start) > uint64(len(source)) {
		return span
	}
	if end < start || uint64(end) > uint64(len(source)) {
		end = start
	}
	startText, endText := source[:start], source[:end]
	result := span
	result.Start.Offset += uint64(start)
	result.End.Offset = span.Start.Offset + uint64(end)
	result.Start.Column += uint32(utf8.RuneCountInString(startText)) // #nosec G115 -- the sliced prefix length is bounded by the uint32 expression offset
	result.End.Line = result.Start.Line
	result.End.Column = span.Start.Column + uint32(utf8.RuneCountInString(endText)) // #nosec G115 -- the sliced prefix length is bounded by the uint32 expression offset
	return result
}

func expressionKindName(kind paperexpr.Kind) string {
	switch kind {
	case paperexpr.Bool:
		return "bool"
	case paperexpr.Integer:
		return "number"
	case paperexpr.String:
		return "string"
	case paperexpr.Null:
		return "null"
	case paperexpr.Unit:
		return "unit"
	default:
		return "unknown"
	}
}

func deferStaticExpressionValues(ast paperlang.AST, schemas schemaAnalysis, limits paperexpr.LanguageLimits) {
	environment := staticSchemaEnvironment(schemas)
	var walk func(*paperlang.Node)
	walk = func(node *paperlang.Node) {
		if node == nil {
			return
		}
		deferScalar := func(scalar *paperlang.Scalar, expectation *expressionExpectation) {
			if scalar == nil || scalar.Kind != paperlang.ScalarExpression || scalar.ExpressionValue == nil {
				return
			}
			kind := paperexpr.String
			if _, inferred, err := paperexpr.Compile(strings.TrimSpace(*scalar.ExpressionValue), environment, limits); err == nil {
				kind = inferred
			} else if expectation != nil {
				kind = expectation.kind
			}
			*scalar = expressionScalar(zeroExpressionValue(kind), scalar.Span)
		}
		if node.Value != nil {
			var expectation *expressionExpectation
			if node.Kind == paperlang.NodeText {
				value := expressionExpectation{kind: paperexpr.String}
				expectation = &value
			}
			deferScalar(node.Value, expectation)
		}
		for _, member := range node.Members {
			if member.Property != nil {
				expectation, known := propertyExpressionExpectation(node.Kind, member.Property.Name)
				if known {
					deferScalar(&member.Property.Value, &expectation)
				} else {
					deferScalar(&member.Property.Value, nil)
				}
			}
			walk(member.Node)
		}
	}
	walk(ast.Root)
}

func zeroExpressionValue(kind paperexpr.Kind) paperexpr.Value {
	switch kind {
	case paperexpr.Bool:
		return paperexpr.Value{Kind: paperexpr.Bool, Bool: true}
	case paperexpr.Integer:
		return paperexpr.Value{Kind: paperexpr.Integer}
	case paperexpr.Null:
		return paperexpr.Value{Kind: paperexpr.Null}
	case paperexpr.Unit:
		return paperexpr.Value{Kind: paperexpr.Unit, Integer: 1, Unit: "pt"}
	default:
		return paperexpr.Value{Kind: paperexpr.String}
	}
}
