// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"syscall/js"

	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

const playgroundMaxMovePoints = 144.0

var playgroundReadableID = regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_-]{0,127}$`)

type playgroundNodeMutation struct {
	hash, target string
	page         uint32
	vectorOnly   bool
	properties   []paperedit.PropertySpec
	moveX, moveY float64
}

type playgroundEditHistory struct {
	mu       sync.Mutex
	previous map[string]string
	next     map[string]string
}

var playgroundHistory playgroundEditHistory

func mutatePlaygroundNode(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: mutateNode expects one request object")))
			return nil
		}
		request, err := playgroundNodeMutationFromJS(arguments[0])
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		go func() {
			result, mutationErr := applyAndRenderPlaygroundNodeMutation(request)
			if mutationErr != nil {
				reject.Invoke(jsError(mutationErr))
				return
			}
			encoded, encodeErr := json.Marshal(result)
			if encodeErr != nil {
				reject.Invoke(jsError(encodeErr))
				return
			}
			resolve.Invoke(js.Global().Get("JSON").Call("parse", string(encoded)))
		}()
		return nil
	})
	result := promise.New(executor)
	executor.Release()
	return result
}

func playgroundNodeMutationFromJS(value js.Value) (playgroundNodeMutation, error) {
	hash, err := jsRequiredString(value, "hash")
	if err != nil || !playgroundDigest(hash) {
		return playgroundNodeMutation{}, errors.New("paper-studio-wasm: mutateNode hash must be a lowercase SHA-256 digest")
	}
	target := strings.TrimSpace(jsOptionalString(value, "target"))
	if !playgroundReadableID.MatchString(target) {
		return playgroundNodeMutation{}, errors.New("paper-studio-wasm: mutateNode target must be one readable @id")
	}
	page, err := jsRequiredPage(value)
	if err != nil {
		return playgroundNodeMutation{}, err
	}
	request := playgroundNodeMutation{
		hash: hash, target: target, page: page, vectorOnly: jsOptionalBool(value, "vectorOnly"),
	}
	request.moveX, err = playgroundOptionalFiniteNumber(value, "moveXPoints")
	if err != nil {
		return playgroundNodeMutation{}, err
	}
	request.moveY, err = playgroundOptionalFiniteNumber(value, "moveYPoints")
	if err != nil {
		return playgroundNodeMutation{}, err
	}
	if math.Abs(request.moveX) > playgroundMaxMovePoints || math.Abs(request.moveY) > playgroundMaxMovePoints {
		return playgroundNodeMutation{}, errors.New("paper-studio-wasm: mutateNode move exceeds the 144pt interaction limit")
	}
	request.moveX = math.Round(request.moveX*2) / 2
	request.moveY = math.Round(request.moveY*2) / 2
	request.properties, err = playgroundMutationProperties(value.Get("properties"))
	if err != nil {
		return playgroundNodeMutation{}, err
	}
	if len(request.properties) == 0 && request.moveX == 0 && request.moveY == 0 {
		return playgroundNodeMutation{}, errors.New("paper-studio-wasm: mutateNode requires a property or movement")
	}
	return request, nil
}

func playgroundMutationProperties(value js.Value) ([]paperedit.PropertySpec, error) {
	if value.IsUndefined() || value.IsNull() {
		return nil, nil
	}
	if !js.Global().Get("Array").Call("isArray", value).Bool() || value.Length() > 16 {
		return nil, errors.New("paper-studio-wasm: mutateNode properties must be a bounded array")
	}
	properties := make([]paperedit.PropertySpec, 0, value.Length())
	seen := make(map[string]struct{}, value.Length())
	for index := 0; index < value.Length(); index++ {
		item := value.Index(index)
		if item.Type() != js.TypeObject {
			return nil, errors.New("paper-studio-wasm: mutateNode property must be an object")
		}
		name := strings.TrimSpace(jsOptionalString(item, "name"))
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("paper-studio-wasm: mutateNode property %q is duplicated", name)
		}
		seen[name] = struct{}{}
		property, err := playgroundMutationProperty(name, item)
		if err != nil {
			return nil, err
		}
		properties = append(properties, property)
	}
	return properties, nil
}

func playgroundMutationProperty(name string, item js.Value) (paperedit.PropertySpec, error) {
	kind := strings.TrimSpace(jsOptionalString(item, "kind"))
	switch name {
	case "style":
		text := strings.TrimSpace(jsOptionalString(item, "text"))
		if kind != "string" || !playgroundReadableID.MatchString(text) {
			return paperedit.PropertySpec{}, errors.New("paper-studio-wasm: style must reference one exact @style")
		}
		return paperedit.PropertySpec{Name: name, Value: paperedit.StringValue(text)}, nil
	case "color":
		text := strings.TrimSpace(jsOptionalString(item, "text"))
		if kind != "string" || !validPlaygroundColor(text) {
			return paperedit.PropertySpec{}, errors.New("paper-studio-wasm: color must be a six-digit hex value")
		}
		return paperedit.PropertySpec{Name: name, Value: paperedit.StringValue(strings.ToUpper(text))}, nil
	case "align":
		text := strings.TrimSpace(jsOptionalString(item, "text"))
		if kind != "string" || (text != "left" && text != "center" && text != "right" && text != "justify") {
			return paperedit.PropertySpec{}, errors.New("paper-studio-wasm: align must be left, center, right, or justify")
		}
		return paperedit.PropertySpec{Name: name, Value: paperedit.StringValue(text)}, nil
	case "bold", "italic":
		if kind != "bool" || item.Get("bool").Type() != js.TypeBoolean {
			return paperedit.PropertySpec{}, fmt.Errorf("paper-studio-wasm: %s must be boolean", name)
		}
		return paperedit.PropertySpec{Name: name, Value: paperedit.BoolValue(item.Get("bool").Bool())}, nil
	case "size", "line-height":
		number, err := playgroundRequiredFiniteNumber(item, "number")
		if kind != "unit" || err != nil || number < 1 || number > 240 {
			return paperedit.PropertySpec{}, fmt.Errorf("paper-studio-wasm: %s must be between 1pt and 240pt", name)
		}
		return paperedit.PropertySpec{Name: name, Value: paperedit.UnitValue(number, "pt")}, nil
	default:
		return paperedit.PropertySpec{}, fmt.Errorf("paper-studio-wasm: property %q is not editable on the canvas", name)
	}
}

func applyAndRenderPlaygroundNodeMutation(request playgroundNodeMutation) (playgroundEditorApplied, error) {
	workspace, ok := planCache.load(request.hash)
	if !ok {
		return playgroundEditorApplied{}, errors.New("paper-studio-wasm: mutateNode workspace hash is not retained")
	}
	parsed := paperlang.Parse(playgroundFile, workspace.source)
	if !parsed.OK() {
		return playgroundEditorApplied{}, errors.New("paper-studio-wasm: mutateNode source is invalid")
	}
	node, matches := playgroundSourceTarget(parsed.AST.Root, request.target)
	if matches != 1 {
		return playgroundEditorApplied{}, errors.New("paper-studio-wasm: mutateNode target is missing or ambiguous")
	}
	properties := append([]paperedit.PropertySpec(nil), request.properties...)
	if request.moveX != 0 {
		properties = append(properties, paperedit.PropertySpec{
			Name: "margin-left", Value: paperedit.UnitValue(playgroundNodePointProperty(node, "margin-left")+request.moveX, "pt"),
		})
	}
	if request.moveY != 0 {
		properties = append(properties, paperedit.PropertySpec{
			Name: "margin-top", Value: paperedit.UnitValue(playgroundNodePointProperty(node, "margin-top")+request.moveY, "pt"),
		})
	}
	edited, err := paperedit.Apply(paperedit.Transaction{
		File: playgroundFile, Source: workspace.source,
		ExpectedRevision: paperedit.SourceRevision(workspace.source),
		Operations: []paperedit.Operation{paperedit.SetProperties{
			Target: request.target, Properties: properties,
		}},
	})
	if err != nil {
		return playgroundEditorApplied{}, err
	}
	planned, plan, err := planPlaygroundRequest(edited.Source, workspace.data, workspace.scenario, workspace.options)
	if err != nil {
		return playgroundEditorApplied{}, err
	}
	playgroundHistory.record(request.hash, planned.Hash)
	renderPage := request.page
	if int(renderPage) > planned.Pages {
		renderPage = uint32(planned.Pages) // #nosec G115 -- compiled page count is bounded by the retained plan
	}
	rendered, err := renderPlaygroundPlanPage(playgroundCompileResult{
		OK: true, Pages: planned.Pages, Hash: planned.Hash,
	}, plan, renderPage, !request.vectorOnly)
	if err != nil {
		return playgroundEditorApplied{}, err
	}
	return playgroundEditorApplied{
		Edit: playgroundEditResult{
			OK: true, Applied: true, Pages: planned.Pages, Hash: planned.Hash,
			Diagnostics: planned.Diagnostics, SourceRevision: planned.SourceRevision, AST: planned.AST,
			Source: edited.Source, Data: workspace.data,
		},
		Page: rendered,
	}, nil
}

func playgroundNodePointProperty(node *paperlang.Node, name string) float64 {
	for _, member := range node.Members {
		property := member.Property
		if property == nil || property.Name != name || property.Value.Kind != paperlang.ScalarUnit ||
			property.Value.UnitValue == nil || property.Value.UnitValue.Unit != "pt" {
			continue
		}
		return property.Value.UnitValue.Number
	}
	return 0
}

func (history *playgroundEditHistory) record(before, after string) {
	if before == "" || after == "" || before == after {
		return
	}
	history.mu.Lock()
	defer history.mu.Unlock()
	if history.previous == nil {
		history.previous = make(map[string]string)
		history.next = make(map[string]string)
	}
	history.previous[after] = before
	history.next[before] = after
}

func (history *playgroundEditHistory) target(hash string, direction int) string {
	history.mu.Lock()
	defer history.mu.Unlock()
	if direction < 0 {
		return history.previous[hash]
	}
	return history.next[hash]
}

func playgroundHistoryState(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
		return jsError(errors.New("paper-studio-wasm: historyState expects one request object"))
	}
	hash := strings.TrimSpace(jsOptionalString(arguments[0], "hash"))
	if !playgroundDigest(hash) {
		return jsError(errors.New("paper-studio-wasm: historyState hash must be a lowercase SHA-256 digest"))
	}
	result := js.Global().Get("Object").New()
	result.Set("canUndo", playgroundHistory.target(hash, -1) != "")
	result.Set("canRedo", playgroundHistory.target(hash, 1) != "")
	return result
}

func renderPlaygroundHistoryPage(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: historyPage expects one request object")))
			return nil
		}
		request := arguments[0]
		hash := strings.TrimSpace(jsOptionalString(request, "hash"))
		direction, err := playgroundRequiredFiniteNumber(request, "direction")
		if !playgroundDigest(hash) || err != nil || (direction != -1 && direction != 1) {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: historyPage requires a hash and direction -1 or 1")))
			return nil
		}
		page, err := jsRequiredPage(request)
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		go func() {
			target := playgroundHistory.target(hash, int(direction))
			workspace, ok := planCache.load(target)
			if target == "" || !ok {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: no document history is available in that direction")))
				return
			}
			if int(page) > workspace.plan.PageCount() {
				page = uint32(workspace.plan.PageCount()) // #nosec G115 -- retained page count is bounded by the plan
			}
			ast, astErr := marshalPlaygroundAST(workspace.source)
			if astErr != nil {
				reject.Invoke(jsError(astErr))
				return
			}
			rendered, renderErr := renderPlaygroundPlanPage(playgroundCompileResult{
				OK: true, Pages: workspace.plan.PageCount(), Hash: target,
			}, workspace.plan, page, !jsOptionalBool(request, "vectorOnly"))
			if renderErr != nil {
				reject.Invoke(jsError(renderErr))
				return
			}
			result := playgroundEditorApplied{
				Edit: playgroundEditResult{
					OK: true, Applied: true, Pages: workspace.plan.PageCount(), Hash: target,
					SourceRevision: string(paperedit.SourceRevision(workspace.source)), AST: ast,
					Source: workspace.source, Data: workspace.data,
				},
				Page: rendered,
			}
			encoded, encodeErr := json.Marshal(result)
			if encodeErr != nil {
				reject.Invoke(jsError(encodeErr))
				return
			}
			resolve.Invoke(js.Global().Get("JSON").Call("parse", string(encoded)))
		}()
		return nil
	})
	result := promise.New(executor)
	executor.Release()
	return result
}

func marshalPlaygroundAST(source string) (json.RawMessage, error) {
	parsed := paperlang.Parse(playgroundFile, source)
	if !parsed.OK() {
		return nil, errors.New("paper-studio-wasm: retained history source is invalid")
	}
	return parsed.AST.CanonicalJSON()
}

func playgroundOptionalFiniteNumber(value js.Value, name string) (float64, error) {
	field := value.Get(name)
	if field.IsUndefined() || field.IsNull() {
		return 0, nil
	}
	return playgroundRequiredFiniteNumber(value, name)
}

func playgroundRequiredFiniteNumber(value js.Value, name string) (float64, error) {
	field := value.Get(name)
	if field.Type() != js.TypeNumber || math.IsNaN(field.Float()) || math.IsInf(field.Float(), 0) {
		return 0, fmt.Errorf("paper-studio-wasm: %s must be a finite number", name)
	}
	return field.Float(), nil
}

func validPlaygroundColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, character := range value[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}
