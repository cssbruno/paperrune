// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/paperrune/internal/paperexpr"
	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/internal/paperscenario"
)

// scenarioConditionEvaluator resolves data expressions and removes nodes whose
// explicitly authored `visible` expression is false. It runs only with an
// explicit scenario/data fixture; ordinary Compile never reads fixture data.
type scenarioConditionEvaluator struct {
	ctx         context.Context
	schemas     schemaAnalysis
	fixture     paperscenario.Fixture
	limits      paperexpr.LanguageLimits
	provenance  map[*paperlang.Node]expansionProvenance
	diagnostics []paperlang.Diagnostic
}

func evaluateScenarioConditions(ctx context.Context, ast paperlang.AST, provenance map[*paperlang.Node]expansionProvenance, schemas schemaAnalysis, fixture paperscenario.Fixture, limits paperexpr.LanguageLimits) []paperlang.Diagnostic {
	if ctx == nil {
		ctx = context.Background()
	}
	evaluator := scenarioConditionEvaluator{ctx: ctx, schemas: schemas, fixture: fixture, limits: limits, provenance: provenance}
	if ast.Root != nil {
		evaluator.filterChildren(ast.Root)
	}
	return evaluator.diagnostics
}

func (e *scenarioConditionEvaluator) filterChildren(parent *paperlang.Node) {
	filtered := make([]paperlang.Member, 0, len(parent.Members))
	for _, member := range parent.Members {
		if member.Node == nil {
			filtered = append(filtered, member)
			continue
		}
		include := e.include(member.Node)
		if include {
			e.resolveProperties(member.Node)
			e.filterChildren(member.Node)
			filtered = append(filtered, member)
		} else {
			e.validateHiddenExpressions(member.Node)
		}
	}
	parent.Members = filtered
}

func (e *scenarioConditionEvaluator) include(node *paperlang.Node) bool {
	condition, duplicate, legacy := takeVisibilityProperty(node)
	if legacy != nil {
		e.add("PAPER_WHEN_REMOVED", "property \"when\" was removed", "use an unquoted boolean expression such as visible: active", legacy.Span)
	}
	if condition == nil {
		return true
	}
	if duplicate != nil {
		e.add("PAPER_VISIBLE_DUPLICATE", "property \"visible\" is repeated on "+string(node.Kind), "remove the duplicate; the first expression is retained", duplicate.Span)
	}
	if !conditionNodeKind(node.Kind) {
		e.add("PAPER_VISIBLE_NODE", fmt.Sprintf("visible is unsupported on %s", node.Kind), "put visible on a renderable node or component use", condition.Span)
		return true
	}
	if condition.Value.Kind == paperlang.ScalarBool && condition.Value.BoolValue != nil {
		return *condition.Value.BoolValue
	}
	if condition.Value.Kind == paperlang.ScalarString {
		e.add("PAPER_VISIBLE_QUOTED", "visible expressions are not quoted strings", "remove the outer quotes", condition.Value.Span)
		return true
	}
	if condition.Value.Kind != paperlang.ScalarExpression || condition.Value.ExpressionValue == nil {
		e.add("PAPER_VISIBLE_VALUE", "visible must be a boolean expression", "use a declared boolean path or comparison", condition.Value.Span)
		return true
	}

	environment, root, problem := e.conditionEnvironment(node)
	if problem != "" {
		e.add("PAPER_WHEN_CONTEXT", problem, "use a declared primitive schema path available in this node's binding context", condition.Value.Span)
		return true
	}
	program, kind, value, err := e.evaluate(node, strings.TrimSpace(*condition.Value.ExpressionValue), environment, root)
	if err != nil {
		code := "PAPER_VISIBLE_EXPRESSION"
		if errors.Is(err, paperexpr.ErrLimit) {
			code = "PAPER_VISIBLE_LIMIT"
		} else if errors.Is(err, paperexpr.ErrBinding) {
			code = "PAPER_VISIBLE_PATH"
		} else if errors.Is(err, paperexpr.ErrType) {
			code = "PAPER_VISIBLE_TYPE"
		}
		e.add(code, err.Error(), "use declared primitive paths and a boolean result", locatedExpressionSpan(condition.Value.Span, strings.TrimSpace(*condition.Value.ExpressionValue), err))
		return true
	}
	if kind != paperexpr.Bool {
		e.add("PAPER_VISIBLE_TYPE", "visible expression must return bool", "compare values or use a boolean field", condition.Value.Span)
		return true
	}
	if value.Kind != paperexpr.Bool {
		e.add("PAPER_VISIBLE_TYPE", "visible expression evaluated to a non-bool value", "make the expression return true or false", condition.Value.Span)
		return true
	}
	_ = program
	return value.Bool
}

// takeVisibilityProperty removes visibility syntax before normal lowering. The
// first authored property wins, matching compiler duplicate-property behavior.
func takeVisibilityProperty(node *paperlang.Node) (first, duplicate, legacy *paperlang.Property) {
	members := make([]paperlang.Member, 0, len(node.Members))
	for _, member := range node.Members {
		if member.Property == nil || member.Property.Name != "visible" && member.Property.Name != "when" {
			members = append(members, member)
			continue
		}
		if member.Property.Name == "when" {
			if legacy == nil {
				legacy = member.Property
			}
			continue
		}
		if first == nil {
			first = member.Property
		} else if duplicate == nil {
			duplicate = member.Property
		}
	}
	node.Members = members
	return first, duplicate, legacy
}

func conditionNodeKind(kind paperlang.NodeKind) bool {
	switch kind {
	case paperlang.NodeParagraph, paperlang.NodeHeading, paperlang.NodeList, paperlang.NodeItem, paperlang.NodeRow, paperlang.NodeColumn,
		paperlang.NodeImage, paperlang.NodeTable, paperlang.NodeTableRow, paperlang.NodeTableCell, paperlang.NodePageBreak, paperlang.NodeUse,
		paperlang.NodeCanvas, paperlang.NodeAnchor:
		return true
	default:
		return false
	}
}

func (e *scenarioConditionEvaluator) resolveProperties(node *paperlang.Node) {
	environment, root, problem := e.conditionEnvironment(node)
	if node.Value != nil && node.Value.Kind == paperlang.ScalarExpression && node.Value.ExpressionValue != nil {
		e.resolveScalar(node, node.Value, environment, root, problem)
	}
	members := node.Members[:0]
	for _, member := range node.Members {
		property := member.Property
		if property == nil || property.Value.Kind != paperlang.ScalarExpression || property.Value.ExpressionValue == nil {
			members = append(members, member)
			continue
		}
		e.resolveScalar(node, &property.Value, environment, root, problem)
		// A computed null optional property is exactly the same as an omitted
		// property. Required shorthand values (for example text:) live on
		// node.Value and therefore remain available for the normal required-value
		// diagnostic path.
		if property.Value.Kind != paperlang.ScalarNull || node.Kind == paperlang.NodeUse && property.Name == "component" {
			members = append(members, member)
		}
	}
	node.Members = members
}

func (e *scenarioConditionEvaluator) validateHiddenExpressions(node *paperlang.Node) {
	if node == nil {
		return
	}
	environment, _, problem := e.conditionEnvironment(node)
	if problem == "" {
		validate := func(scalar *paperlang.Scalar) {
			if scalar == nil || scalar.Kind != paperlang.ScalarExpression || scalar.ExpressionValue == nil {
				return
			}
			if _, _, err := paperexpr.Compile(strings.TrimSpace(*scalar.ExpressionValue), environment, e.limits); err != nil {
				e.add("PAPER_EXPRESSION_STATIC", err.Error(), "fix the expression even though its node is currently hidden", scalar.Span)
			}
		}
		validate(node.Value)
		for _, member := range node.Members {
			if member.Property != nil && member.Property.Name != "visible" {
				validate(&member.Property.Value)
			}
		}
	}
	for _, member := range node.Members {
		e.validateHiddenExpressions(member.Node)
	}
}

func (e *scenarioConditionEvaluator) resolveScalar(node *paperlang.Node, scalar *paperlang.Scalar, environment []paperexpr.PathKind, root paperscenario.Value, problem string) {
	if problem != "" {
		e.add("PAPER_EXPRESSION_CONTEXT", problem, "use a declared path available in this node's binding context", scalar.Span)
		return
	}
	expression := strings.TrimSpace(*scalar.ExpressionValue)
	_, _, value, err := e.evaluate(node, expression, environment, root)
	if err != nil {
		code := "PAPER_EXPRESSION"
		if errors.Is(err, paperexpr.ErrLimit) {
			code = "PAPER_EXPRESSION_LIMIT"
		} else if errors.Is(err, paperexpr.ErrBinding) {
			code = "PAPER_EXPRESSION_PATH"
		} else if errors.Is(err, paperexpr.ErrType) {
			code = "PAPER_EXPRESSION_TYPE"
		}
		e.add(code, err.Error(), "use declared paths and a result compatible with the property", locatedExpressionSpan(scalar.Span, expression, err))
		return
	}
	*scalar = expressionScalar(value, scalar.Span)
}

func (e *scenarioConditionEvaluator) evaluate(node *paperlang.Node, source string, environment []paperexpr.PathKind, root paperscenario.Value) (paperexpr.Program, paperexpr.Kind, paperexpr.Value, error) {
	program, kind, err := paperexpr.Compile(source, environment, e.limits)
	if err != nil {
		return paperexpr.Program{}, paperexpr.Null, paperexpr.Value{}, err
	}
	bindings, err := e.expressionBindings(node, program.Paths, root)
	if err != nil {
		return program, kind, paperexpr.Value{}, fmt.Errorf("%w: %v", paperexpr.ErrBinding, err)
	}
	value, err := paperexpr.Evaluate(e.ctx, program, bindings, e.limits.Program)
	return program, kind, value, err
}

func expressionScalar(value paperexpr.Value, span paperlang.Span) paperlang.Scalar {
	scalar := paperlang.Scalar{Span: span}
	switch value.Kind {
	case paperexpr.Null:
		scalar.Kind, scalar.Raw = paperlang.ScalarNull, "null"
	case paperexpr.Bool:
		scalar.Kind, scalar.Raw = paperlang.ScalarBool, strconv.FormatBool(value.Bool)
		boolean := value.Bool
		scalar.BoolValue = &boolean
	case paperexpr.Integer:
		scalar.Kind, scalar.Raw = paperlang.ScalarNumber, strconv.FormatInt(value.Integer, 10)
		number := float64(value.Integer)
		scalar.NumberValue = &number
	case paperexpr.String:
		scalar.Kind = paperlang.ScalarString
		text := value.String
		scalar.StringValue = &text
		scalar.Raw = strconv.Quote(text)
	}
	return scalar
}

func (e *scenarioConditionEvaluator) conditionEnvironment(node *paperlang.Node) ([]paperexpr.PathKind, paperscenario.Value, string) {
	origin := e.provenance[node]
	if origin.loopItem {
		return append([]paperexpr.PathKind(nil), origin.loopEnvironment...), origin.loopRoot, ""
	}
	if origin.repeatItem {
		if len(origin.repeatFields) != 0 && origin.repeatValue.Kind == paperscenario.Object {
			if origin.bindingBase != "" && origin.repeatItemBase != "" &&
				origin.bindingBase != origin.repeatItemBase && !strings.HasPrefix(origin.bindingBase, strings.TrimSuffix(origin.repeatItemBase, ".")+".") {
				fields, value, problem := e.bindingContext(origin.bindingBase)
				if problem != "" {
					return nil, paperscenario.Value{}, problem
				}
				return repeatExpressionEnvironment(fields, "item"), value, ""
			}
			fields, value, problem := repeatConditionContext(origin)
			if problem != "" {
				return nil, paperscenario.Value{}, problem
			}
			return repeatExpressionEnvironment(fields, "item"), value, ""
		}
		fields, err := repeatSchemaItem(origin.repeatSource, e.schemas)
		if err != nil {
			return nil, paperscenario.Value{}, err.Error()
		}
		items, err := repeatFixtureItems(e.fixture, origin.repeatSource)
		if err != nil {
			return nil, paperscenario.Value{}, err.Error()
		}
		for _, item := range items {
			if item.Key == origin.repeatKey {
				if item.Value.Kind != paperscenario.Object {
					return nil, paperscenario.Value{}, fmt.Sprintf("repeat item %s[%s] is not an object", origin.repeatSource, origin.repeatKey)
				}
				return repeatExpressionEnvironment(fields, "item"), item.Value, ""
			}
		}
		return nil, paperscenario.Value{}, fmt.Sprintf("repeat source %s has no stable item key %q", origin.repeatSource, origin.repeatKey)
	}

	if origin.bindingBase != "" {
		fields, value, problem := e.bindingContext(origin.bindingBase)
		if problem != "" {
			return nil, paperscenario.Value{}, problem
		}
		return repeatExpressionEnvironment(fields, "item"), value, ""
	}

	// Scenario fixture fields share the top-level namespace. Merge all declared
	// schemas deterministically; conflicting field contracts are diagnosed.
	kinds := make(map[string]paperexpr.Kind)
	for _, schema := range e.schemas.descriptors {
		for _, field := range schema.Fields {
			for _, path := range repeatExpressionEnvironment([]FieldDescriptor{field}, "") {
				if prior, exists := kinds[path.Path]; exists && prior != path.Kind {
					return nil, paperscenario.Value{}, fmt.Sprintf("schema path %q has conflicting primitive types", path.Path)
				}
				kinds[path.Path] = path.Kind
			}
		}
	}
	environment := make([]paperexpr.PathKind, 0, len(kinds))
	for path, kind := range kinds {
		environment = append(environment, paperexpr.PathKind{Path: path, Kind: kind})
	}
	return environment, paperscenario.Value{Kind: paperscenario.Object, Object: e.fixture.Values}, ""
}

func (e *scenarioConditionEvaluator) expressionBindings(node *paperlang.Node, paths []string, root paperscenario.Value) ([]paperexpr.Binding, error) {
	origin := e.provenance[node]
	if origin.loopItem {
		return loopExpressionBindings(paths, root, origin.loopIndex, origin.loopFirst, origin.loopLast)
	}
	if origin.repeatItem {
		return scopedConditionBindings(paths, root, "item")
	}
	return conditionBindings(paths, root)
}

func repeatConditionContext(origin expansionProvenance) ([]FieldDescriptor, paperscenario.Value, string) {
	fields := origin.repeatFields
	value := origin.repeatValue
	base := strings.TrimSuffix(origin.repeatItemBase, ".")
	binding := strings.TrimSuffix(origin.bindingBase, ".")
	if binding == "" || binding == base {
		return fields, value, ""
	}
	prefix := base + "."
	if !strings.HasPrefix(binding, prefix) {
		return nil, paperscenario.Value{}, fmt.Sprintf("condition binding %q is outside repeat item context %q", binding, base)
	}
	for _, name := range strings.Split(strings.TrimPrefix(binding, prefix), ".") {
		if strings.HasSuffix(name, "[]") {
			return nil, paperscenario.Value{}, fmt.Sprintf("condition context %q crosses a nested collection", binding)
		}
		field := findSchemaField(fields, name)
		if field == nil || field.Kind != SchemaObject {
			return nil, paperscenario.Value{}, fmt.Sprintf("condition context %q does not resolve to an object", binding)
		}
		resolved, found, problem := lookupFixtureFields(value.Object, []string{name})
		if problem != "" || !found || resolved.Kind != paperscenario.Object {
			return nil, paperscenario.Value{}, fmt.Sprintf("selected repeat item has no object for condition context %q", binding)
		}
		fields = field.Fields
		value = resolved
	}
	return fields, value, ""
}

func (e *scenarioConditionEvaluator) bindingContext(path string) ([]FieldDescriptor, paperscenario.Value, string) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "@") {
		return nil, paperscenario.Value{}, fmt.Sprintf("binding context %q is not absolute", path)
	}
	schema := e.schemas.byName[parts[0]]
	if schema == nil {
		return nil, paperscenario.Value{}, fmt.Sprintf("binding context schema %s is not declared", parts[0])
	}
	fields := schema.Fields
	fixtureFields := e.fixture.Values
	if len(parts) == 1 {
		return fields, paperscenario.Value{Kind: paperscenario.Object, Object: fixtureFields}, ""
	}
	for _, name := range parts[1:] {
		field := findSchemaField(fields, strings.TrimSuffix(name, "[]"))
		if field == nil || field.Kind != SchemaObject {
			return nil, paperscenario.Value{}, fmt.Sprintf("binding context %q does not resolve to an object", path)
		}
		value, found, problem := lookupFixtureFields(fixtureFields, []string{strings.TrimSuffix(name, "[]")})
		if problem != "" || !found || value.Kind != paperscenario.Object {
			return nil, paperscenario.Value{}, fmt.Sprintf("selected scenario @%s has no object for binding context %q", e.fixture.Name, path)
		}
		fields = field.Fields
		fixtureFields = value.Object
	}
	return fields, paperscenario.Value{Kind: paperscenario.Object, Object: fixtureFields}, ""
}

func conditionBindings(paths []string, root paperscenario.Value) ([]paperexpr.Binding, error) {
	return scopedConditionBindings(paths, root, "")
}

func scopedConditionBindings(paths []string, root paperscenario.Value, scope string) ([]paperexpr.Binding, error) {
	bindings := make([]paperexpr.Binding, 0, len(paths))
	for _, path := range paths {
		lookup := path
		if scope != "" {
			prefix := scope + "."
			if !strings.HasPrefix(path, prefix) {
				return nil, fmt.Errorf("binding %q is outside the %s scope", path, scope)
			}
			lookup = strings.TrimPrefix(path, prefix)
		}
		value, found, collection := resolveConditionPath(root, lookup)
		if !found {
			return nil, fmt.Errorf("when binding %q is missing", path)
		}
		if collection {
			return nil, fmt.Errorf("when binding %q resolves to a collection", path)
		}
		converted, err := conditionPrimitive(value)
		if err != nil {
			return nil, fmt.Errorf("when binding %q: %w", path, err)
		}
		bindings = append(bindings, paperexpr.Binding{Path: path, Value: converted})
	}
	return bindings, nil
}

func resolveConditionPath(root paperscenario.Value, path string) (paperscenario.Value, bool, bool) {
	current := root
	for _, name := range strings.Split(path, ".") {
		if current.Kind != paperscenario.Object {
			return paperscenario.Value{}, true, true
		}
		found := false
		for _, field := range current.Object {
			if field.Name == name {
				current, found = field.Value, true
				break
			}
		}
		if !found {
			return paperscenario.Value{}, false, false
		}
	}
	return current, true, current.Kind == paperscenario.Object || current.Kind == paperscenario.List
}

func conditionPrimitive(value paperscenario.Value) (paperexpr.Value, error) {
	switch value.Kind {
	case paperscenario.Null:
		return paperexpr.Value{Kind: paperexpr.Null}, nil
	case paperscenario.Bool:
		return paperexpr.Value{Kind: paperexpr.Bool, Bool: value.Bool}, nil
	case paperscenario.String:
		return paperexpr.Value{Kind: paperexpr.String, String: value.String}, nil
	case paperscenario.Number:
		integer, err := strconv.ParseInt(value.Number, 10, 64)
		if err != nil {
			return paperexpr.Value{}, errors.New("number is not a canonical int64")
		}
		return paperexpr.Value{Kind: paperexpr.Integer, Integer: integer}, nil
	default:
		return paperexpr.Value{}, fmt.Errorf("value is non-primitive %s", value.Kind)
	}
}

func (e *scenarioConditionEvaluator) add(code, message, hint string, span paperlang.Span) {
	e.diagnostics = append(e.diagnostics, paperlang.Diagnostic{Code: code, Severity: paperlang.SeverityError, Message: message, Hint: hint, Span: span})
}
