// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall/js"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/layoutengine"
	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/internal/paperscenario"
)

var renderFunction js.Func
var compileFunction js.Func
var hitFunction js.Func
var traceFunction js.Func
var editTextFunction js.Func
var renderCache layoutengine.WebDisplayRenderCache
var planCache playgroundPlanCache

const (
	maxPlaygroundSourceBytes = 1 << 20
	maxPlaygroundDataBytes   = 4 << 20
	maxPlaygroundPlanCache   = 8
	playgroundFile           = "playground.paper"
	maxSafeJSInteger         = 1<<53 - 1
)

func main() {
	renderFunction = js.FuncOf(renderPage)
	compileFunction = js.FuncOf(compilePaper)
	hitFunction = js.FuncOf(hitPaper)
	traceFunction = js.FuncOf(tracePaper)
	editTextFunction = js.FuncOf(editPaperText)
	engine := js.Global().Get("Object").New()
	engine.Set("formatVersion", layoutengine.WebDisplayRenderPayloadVersion)
	engine.Set("rendererVersion", layoutengine.DisplayRasterRendererVersion)
	engine.Set("render", renderFunction)
	engine.Set("compile", compileFunction)
	engine.Set("hit", hitFunction)
	engine.Set("trace", traceFunction)
	engine.Set("editText", editTextFunction)
	js.Global().Set("PaperStudioWASM", engine)
	<-make(chan struct{})
}

type playgroundCompileResult struct {
	OK              bool                       `json:"ok"`
	Pages           int                        `json:"pages"`
	Page            uint32                     `json:"page,omitempty"`
	Hash            string                     `json:"hash,omitempty"`
	Diagnostics     []document.PaperDiagnostic `json:"diagnostics,omitempty"`
	Error           string                     `json:"error,omitempty"`
	SourceRevision  string                     `json:"source_revision,omitempty"`
	AST             json.RawMessage            `json:"ast,omitempty"`
	SVG             string                     `json:"svg,omitempty"`
	PageXFixed      int64                      `json:"page_x_fixed"`
	PageYFixed      int64                      `json:"page_y_fixed"`
	PageWidthFixed  int64                      `json:"page_width_fixed"`
	PageHeightFixed int64                      `json:"page_height_fixed"`
	FixedScale      int64                      `json:"fixed_scale"`
	PNG             string                     `json:"png,omitempty"`
	PixelWidth      uint32                     `json:"pixel_width,omitempty"`
	PixelHeight     uint32                     `json:"pixel_height,omitempty"`
	DPI             uint32                     `json:"dpi,omitempty"`
	Renderer        string                     `json:"renderer,omitempty"`
}

type playgroundPlanCache struct {
	mu    sync.Mutex
	plans map[string]playgroundCachedPlan
	order []string
}

type playgroundCachedPlan struct {
	plan        document.PaperPlan
	itemIndexes map[string]int
	source      string
	data        string
	scenario    string
	options     document.PaperJSONOptions
}

type playgroundEditResult struct {
	playgroundCompileResult
	Applied bool   `json:"applied"`
	Source  string `json:"source"`
	Data    string `json:"data"`
}

func (cache *playgroundPlanCache) store(plan document.PaperPlan, itemIndexes map[string]int, source, data, scenario string, options document.PaperJSONOptions) {
	hash := plan.Hash()
	if hash == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.plans == nil {
		cache.plans = make(map[string]playgroundCachedPlan, maxPlaygroundPlanCache)
	}
	for index, existing := range cache.order {
		if existing != hash {
			continue
		}
		cache.order = append(cache.order[:index], cache.order[index+1:]...)
		break
	}
	cache.plans[hash] = playgroundCachedPlan{
		plan: plan, itemIndexes: itemIndexes,
		source: source, data: data, scenario: scenario, options: options,
	}
	cache.order = append(cache.order, hash)
	if len(cache.order) <= maxPlaygroundPlanCache {
		return
	}
	oldest := cache.order[0]
	cache.order = cache.order[1:]
	delete(cache.plans, oldest)
}

func (cache *playgroundPlanCache) load(hash string) (playgroundCachedPlan, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	plan, ok := cache.plans[hash]
	if !ok {
		return playgroundCachedPlan{}, false
	}
	for index, existing := range cache.order {
		if existing != hash {
			continue
		}
		cache.order = append(cache.order[:index], cache.order[index+1:]...)
		break
	}
	cache.order = append(cache.order, hash)
	return plan, true
}

func compilePaper(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: compile expects one request object")))
			return nil
		}
		request := arguments[0]
		sourceValue := request.Get("source")
		if sourceValue.Type() != js.TypeString {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: compile source must be a string")))
			return nil
		}
		source := sourceValue.String()
		data := jsOptionalString(request, "data")
		scenario := strings.TrimPrefix(strings.TrimSpace(jsOptionalString(request, "scenario")), "@")
		if len(source) == 0 || len(source) > maxPlaygroundSourceBytes || len(data) > maxPlaygroundDataBytes {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: compile input exceeds playground limits")))
			return nil
		}
		page := uint32(1)
		if pageValue := request.Get("page"); pageValue.Type() == js.TypeNumber {
			requested := pageValue.Float()
			if math.IsNaN(requested) || math.IsInf(requested, 0) || requested < 1 || requested > math.MaxUint32 || math.Trunc(requested) != requested {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: compile page must be a positive 32-bit integer")))
				return nil
			}
			page = uint32(requested) // #nosec G115 -- explicitly bounded to an integral uint32 above.
		} else if pageValue.Type() != js.TypeUndefined && pageValue.Type() != js.TypeNull {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: compile page must be a number")))
			return nil
		}
		options := document.PaperJSONOptions{Name: jsOptionalString(request, "dataName"), Schema: jsOptionalString(request, "schema"), Locale: jsOptionalString(request, "locale")}
		go func() {
			result, err := compilePlaygroundRequest(source, data, scenario, page, options)
			encoded, encodeErr := json.Marshal(result)
			if encodeErr != nil {
				reject.Invoke(jsError(encodeErr))
				return
			}
			if err != nil && len(result.Diagnostics) == 0 {
				reject.Invoke(jsError(err))
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

func compilePlaygroundRequest(source, data, scenario string, page uint32, options document.PaperJSONOptions) (playgroundCompileResult, error) {
	parsed := paperlang.Parse(playgroundFile, source)
	ast, astErr := parsed.AST.CanonicalJSON()
	if astErr != nil {
		return playgroundCompileResult{}, astErr
	}
	var plan document.PaperPlan
	var planned document.PaperPlanResult
	var err error
	switch {
	case data != "" && scenario != "":
		return playgroundCompileFailure(playgroundCompileResult{
			SourceRevision: string(paperedit.SourceRevision(source)), AST: ast,
		}, errors.New("paper-studio-wasm: choose JSON data or a declared scenario, not both"))
	case data != "":
		plan, planned, err = document.PlanPaperJSONWithOptions(playgroundFile, source, []byte(data), options)
	case scenario != "":
		plan, planned, err = document.PlanPaperScenario(playgroundFile, source, scenario)
	default:
		plan, planned, err = document.PlanPaper(playgroundFile, source)
	}
	sourceRevision := planned.SourceRevision
	if sourceRevision == "" {
		sourceRevision = string(paperedit.SourceRevision(source))
	}
	result := playgroundCompileResult{
		OK: planned.OK(), Pages: planned.Pages, Hash: planned.Hash,
		Diagnostics: planned.Diagnostics, SourceRevision: sourceRevision, AST: ast,
	}
	if err != nil {
		return playgroundCompileFailure(result, err)
	}
	if page == 0 || int(page) > plan.PageCount() {
		return playgroundCompileFailure(result, errors.New("paper-studio-wasm: requested page is outside the compiled plan"))
	}
	capture, err := plan.CaptureDisplayPageSVG(context.Background(), page, nil)
	if err != nil {
		return playgroundCompileFailure(result, err)
	}
	request := document.DefaultPaperPlanWebRenderRequest(page)
	payload, err := plan.WebDisplayRenderPayload(context.Background(), request)
	if err != nil {
		return playgroundCompileFailure(result, err)
	}
	artifact, err := layoutengine.RenderWebDisplayPayloadCached(context.Background(), payload, &renderCache)
	if err != nil {
		return playgroundCompileFailure(result, err)
	}
	manifest := artifact.Manifest()
	result.Page = page
	result.SVG = string(capture.SVG)
	result.PageXFixed, result.PageYFixed = capture.PageX, capture.PageY
	result.PageWidthFixed, result.PageHeightFixed = capture.PageWidth, capture.PageHeight
	result.FixedScale = capture.FixedScale
	result.PNG = base64.StdEncoding.EncodeToString(artifact.PNG())
	result.PixelWidth, result.PixelHeight = manifest.PixelWidth, manifest.PixelHeight
	result.DPI, result.Renderer = manifest.Profile.DPI, manifest.Identity.RendererVersion
	planCache.store(plan, playgroundJSONItemIndexes(parsed.AST, data, options), source, data, scenario, options)
	return result, nil
}

func playgroundCompileFailure(result playgroundCompileResult, err error) (playgroundCompileResult, error) {
	result.OK = false
	result.Error = err.Error()
	return result, err
}

func playgroundJSONItemIndexes(ast paperlang.AST, data string, options document.PaperJSONOptions) map[string]int {
	if data == "" {
		return nil
	}
	schemas := papercompile.ExtractSchemas(ast)
	if !schemas.OK() {
		return nil
	}
	fixture, err := papercompile.FixtureFromJSONData([]byte(data), schemas.Schemas, papercompile.JSONDataOptions{
		Name: options.Name, Schema: options.Schema, Locale: options.Locale,
	})
	if err != nil {
		return nil
	}
	indexes := make(map[string]int)
	for _, field := range fixture.Values {
		playgroundIndexFixtureValue(field.Value, field.Name, nil, indexes)
	}
	return indexes
}

func playgroundIndexFixtureValue(value paperscenario.Value, path string, parentKeys []string, indexes map[string]int) {
	switch value.Kind {
	case paperscenario.Object:
		for _, field := range value.Object {
			childPath := field.Name
			if path != "" {
				childPath = path + "." + field.Name
			}
			playgroundIndexFixtureValue(field.Value, childPath, parentKeys, indexes)
		}
	case paperscenario.List:
		for index, item := range value.List {
			indexes[playgroundItemIndexKey(path, parentKeys, item.Key)] = index
			keys := append(append([]string(nil), parentKeys...), item.Key)
			playgroundIndexFixtureValue(item.Value, path+"[]", keys, indexes)
		}
	}
}

func playgroundItemIndexKey(path string, parentKeys []string, itemKey string) string {
	return path + "|" + strings.Join(parentKeys, "/") + "|" + itemKey
}

func playgroundTraceJSONPointers(encoded []byte, itemIndexes map[string]int) ([]byte, error) {
	var trace map[string]any
	if err := json.Unmarshal(encoded, &trace); err != nil {
		return nil, fmt.Errorf("paper-studio-wasm: decode trace: %w", err)
	}
	provenance, _ := trace["provenance"].(map[string]any)
	bindings, _ := provenance["bindings"].([]any)
	for _, raw := range bindings {
		binding, _ := raw.(map[string]any)
		path, _ := binding["path"].(string)
		instance, _ := binding["instance"].(string)
		if pointer := playgroundBindingJSONPointer(path, instance, itemIndexes); pointer != "" {
			binding["json_pointer"] = pointer
		}
	}
	return json.Marshal(trace)
}

func playgroundBindingJSONPointer(path, instance string, itemIndexes map[string]int) string {
	path = strings.TrimPrefix(strings.TrimSpace(path), "@")
	if root := strings.IndexByte(path, '.'); root >= 0 {
		path = path[root+1:]
	}
	if path == "" {
		return ""
	}
	instanceKeys := playgroundInstanceKeys(instance)
	keyCursor := 0
	parentKeys := make([]string, 0, len(instanceKeys))
	pointer := make([]string, 0, strings.Count(path, ".")+2)
	canonical := ""
	for _, rawSegment := range strings.Split(path, ".") {
		list := strings.HasSuffix(rawSegment, "[]")
		segment := strings.TrimSuffix(rawSegment, "[]")
		if segment == "" {
			return ""
		}
		pointer = append(pointer, strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1"))
		current := segment
		if canonical != "" {
			current = canonical + "." + segment
		}
		if !list {
			canonical = current
			continue
		}
		found := false
		for index := keyCursor; index < len(instanceKeys); index++ {
			itemKey := instanceKeys[index]
			itemIndex, ok := itemIndexes[playgroundItemIndexKey(current, parentKeys, itemKey)]
			if !ok {
				continue
			}
			pointer = append(pointer, strconv.Itoa(itemIndex))
			parentKeys = append(parentKeys, itemKey)
			keyCursor = index + 1
			found = true
			break
		}
		if !found {
			return ""
		}
		canonical = current + "[]"
	}
	return "/" + strings.Join(pointer, "/")
}

func playgroundInstanceKeys(instance string) []string {
	keys := make([]string, 0, strings.Count(instance, "["))
	for cursor := 0; cursor < len(instance); {
		start := strings.IndexByte(instance[cursor:], '[')
		if start < 0 {
			break
		}
		start += cursor + 1
		end := strings.IndexByte(instance[start:], ']')
		if end < 0 {
			break
		}
		end += start
		if end > start {
			keys = append(keys, instance[start:end])
		}
		cursor = end + 1
	}
	return keys
}

func hitPaper(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: hit expects one request object")))
			return nil
		}
		request := arguments[0]
		hash, err := jsRequiredString(request, "hash")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		if !playgroundDigest(hash) {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: hit hash must be a lowercase SHA-256 digest")))
			return nil
		}
		page, err := jsRequiredPage(request)
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		x, err := jsFixedCoordinate(request, "x_fixed")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		y, err := jsFixedCoordinate(request, "y_fixed")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		go func() {
			cached, ok := planCache.load(hash)
			if !ok {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: hit plan hash is not retained")))
				return
			}
			hit, hitErr := cached.plan.HitTest(page, x, y)
			if hitErr != nil {
				reject.Invoke(jsError(hitErr))
				return
			}
			resolve.Invoke(js.Global().Get("JSON").Call("parse", string(hit.JSON())))
		}()
		return nil
	})
	result := promise.New(executor)
	executor.Release()
	return result
}

func tracePaper(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: trace expects one request object")))
			return nil
		}
		request := arguments[0]
		hash, err := jsRequiredString(request, "hash")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		if !playgroundDigest(hash) {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: trace hash must be a lowercase SHA-256 digest")))
			return nil
		}
		fragment, err := jsRequiredUint32(request, "fragment")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		go func() {
			cached, ok := planCache.load(hash)
			if !ok {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: trace plan hash is not retained")))
				return
			}
			trace, traceErr := cached.plan.TraceFragment(fragment)
			if traceErr != nil {
				reject.Invoke(jsError(traceErr))
				return
			}
			encoded, encodeErr := playgroundTraceJSONPointers(trace.JSON(), cached.itemIndexes)
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

func editPaperText(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText expects one request object")))
			return nil
		}
		request := arguments[0]
		hash, err := jsRequiredString(request, "hash")
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		if !playgroundDigest(hash) {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText hash must be a lowercase SHA-256 digest")))
			return nil
		}
		page, err := jsRequiredPage(request)
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		textValue := request.Get("text")
		if textValue.Type() != js.TypeString {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText text must be a string")))
			return nil
		}
		text := textValue.String()
		if len(text) > paperedit.MaxReplacementBytes {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText input exceeds playground limits")))
			return nil
		}
		jsonPointer := strings.TrimSpace(jsOptionalString(request, "jsonPointer"))
		target := strings.TrimSpace(jsOptionalString(request, "target"))
		sourceOffset, hasSourceOffset, err := jsOptionalSourceOffset(request)
		if err != nil {
			reject.Invoke(jsError(err))
			return nil
		}
		selectors := 0
		if jsonPointer != "" {
			selectors++
		}
		if target != "" {
			selectors++
		}
		if hasSourceOffset {
			selectors++
		}
		if selectors != 1 {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText requires exactly one of jsonPointer, target, or sourceOffset")))
			return nil
		}
		if len(jsonPointer) > 4096 || len(target) > 256 {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: editText input exceeds playground limits")))
			return nil
		}
		go func() {
			workspace, ok := planCache.load(hash)
			if !ok {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: editText workspace hash is not retained")))
				return
			}
			source, data := workspace.source, workspace.data
			if jsonPointer != "" {
				edited, editErr := playgroundEditJSONData(data, jsonPointer, text)
				if editErr != nil {
					reject.Invoke(jsError(editErr))
					return
				}
				data = edited.Data
			} else {
				operation, operationErr := playgroundTextOperation(source, target, sourceOffset, hasSourceOffset, text)
				if operationErr != nil {
					reject.Invoke(jsError(operationErr))
					return
				}
				edited, editErr := paperedit.Apply(paperedit.Transaction{
					File: playgroundFile, Source: source,
					ExpectedRevision: paperedit.SourceRevision(source),
					Operations:       []paperedit.Operation{operation},
				})
				if editErr != nil {
					reject.Invoke(jsError(editErr))
					return
				}
				source = edited.Source
			}
			compiled, compileErr := compilePlaygroundRequest(source, data, workspace.scenario, page, workspace.options)
			if compileErr != nil {
				reject.Invoke(jsError(compileErr))
				return
			}
			result := playgroundEditResult{
				playgroundCompileResult: compiled,
				Applied:                 true,
				Source:                  source,
				Data:                    data,
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

func playgroundTextOperation(source, target string, sourceOffset uint64, hasSourceOffset bool, text string) (paperedit.Operation, error) {
	parsed := paperlang.Parse(playgroundFile, source)
	if !parsed.OK() {
		return nil, errors.New("paper-studio-wasm: editText source is invalid")
	}
	node, matches := playgroundSourceNode(parsed.AST.Root, target, sourceOffset, hasSourceOffset)
	label := target
	if hasSourceOffset {
		label = fmt.Sprintf("offset:%d", sourceOffset)
	}
	if matches != 1 {
		return nil, fmt.Errorf("paper-studio-wasm: editText target %s is missing or ambiguous", label)
	}

	var inline *paperlang.Node
	var textProperty *paperlang.Property
	for _, member := range node.Members {
		if member.Property != nil && member.Property.Name == "bind" {
			return nil, fmt.Errorf("paper-studio-wasm: editText target %s is data-bound", label)
		}
		if member.Property != nil && member.Property.Name == "text" {
			if textProperty != nil {
				return nil, fmt.Errorf("paper-studio-wasm: editText target %s has ambiguous text", label)
			}
			textProperty = member.Property
		}
		if member.Node == nil || member.Node.Kind != paperlang.NodeText {
			continue
		}
		if inline != nil {
			return nil, fmt.Errorf("paper-studio-wasm: editText target %s has ambiguous text", label)
		}
		inline = member.Node
	}
	if textProperty != nil && inline != nil {
		return nil, fmt.Errorf("paper-studio-wasm: editText target %s has ambiguous text", label)
	}
	if textProperty != nil {
		if textProperty.Value.Kind != paperlang.ScalarString || textProperty.Value.StringValue == nil {
			return nil, fmt.Errorf("paper-studio-wasm: editText target %s is data-bound or expression-backed", label)
		}
		if hasSourceOffset {
			return paperedit.SetPropertyAtOffset{
				NodeOffset: sourceOffset, Name: "text", Value: paperedit.StringValue(text),
			}, nil
		}
		return paperedit.SetProperty{Target: target, Name: "text", Value: paperedit.StringValue(text)}, nil
	}
	if inline != nil {
		if !playgroundLiteralText(inline) {
			return nil, fmt.Errorf("paper-studio-wasm: editText target %s is data-bound or expression-backed", label)
		}
		if inline.ID != "" {
			if _, inlineMatches := playgroundSourceTarget(parsed.AST.Root, inline.ID); inlineMatches != 1 {
				return nil, fmt.Errorf("paper-studio-wasm: editText inline target %s is ambiguous", inline.ID)
			}
			return paperedit.ReplaceText{Target: inline.ID, Text: text}, nil
		}
		if hasSourceOffset {
			return nil, fmt.Errorf("paper-studio-wasm: anonymous inline text at %s must be edited using its own source offset", label)
		}
		return paperedit.ReplaceInlineText{Parent: target, Text: text}, nil
	}

	switch node.Kind {
	case paperlang.NodeText:
		if !playgroundLiteralText(node) {
			return nil, fmt.Errorf("paper-studio-wasm: editText target %s is data-bound or expression-backed", label)
		}
		if hasSourceOffset {
			return paperedit.ReplaceTextAtOffset{NodeOffset: sourceOffset, Text: text}, nil
		}
		return paperedit.ReplaceText{Target: target, Text: text}, nil
	default:
		return nil, fmt.Errorf("paper-studio-wasm: editText target %s has no authored text literal", label)
	}
}

func playgroundSourceNode(root *paperlang.Node, target string, sourceOffset uint64, byOffset bool) (*paperlang.Node, int) {
	if !byOffset {
		return playgroundSourceTarget(root, target)
	}
	var found *paperlang.Node
	matches := 0
	var walk func(*paperlang.Node)
	walk = func(node *paperlang.Node) {
		if node == nil {
			return
		}
		if node.HeaderSpan.Start.Offset == sourceOffset {
			found = node
			matches++
		}
		for _, member := range node.Members {
			walk(member.Node)
		}
	}
	walk(root)
	return found, matches
}

func playgroundSourceTarget(root *paperlang.Node, target string) (*paperlang.Node, int) {
	var found *paperlang.Node
	matches := 0
	var walk func(*paperlang.Node)
	walk = func(node *paperlang.Node) {
		if node == nil {
			return
		}
		if node.ID == target {
			found = node
			matches++
		}
		for _, member := range node.Members {
			walk(member.Node)
		}
	}
	walk(root)
	return found, matches
}

func playgroundLiteralText(node *paperlang.Node) bool {
	return node != nil && node.Value != nil && node.Value.Kind == paperlang.ScalarString && node.Value.StringValue != nil
}

func playgroundDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func jsRequiredString(object js.Value, name string) (string, error) {
	value := object.Get(name)
	if value.Type() != js.TypeString || value.String() == "" {
		return "", fmt.Errorf("paper-studio-wasm: %s must be a non-empty string", name)
	}
	return value.String(), nil
}

func jsOptionalSourceOffset(object js.Value) (uint64, bool, error) {
	value := object.Get("sourceOffset")
	if value.Type() == js.TypeUndefined || value.Type() == js.TypeNull {
		return 0, false, nil
	}
	if value.Type() != js.TypeNumber {
		return 0, false, errors.New("paper-studio-wasm: sourceOffset must be a non-negative integer")
	}
	number := value.Float()
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > maxSafeJSInteger || math.Trunc(number) != number {
		return 0, false, errors.New("paper-studio-wasm: sourceOffset must be a non-negative safe integer")
	}
	return uint64(number), true, nil
}

func jsRequiredPage(object js.Value) (uint32, error) {
	value := object.Get("page")
	if value.Type() != js.TypeNumber {
		return 0, errors.New("paper-studio-wasm: page must be a number")
	}
	number := value.Float()
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 1 || number > math.MaxUint32 || math.Trunc(number) != number {
		return 0, errors.New("paper-studio-wasm: page must be a positive 32-bit integer")
	}
	return uint32(number), nil // #nosec G115 -- explicitly bounded to an integral uint32 above.
}

func jsRequiredUint32(object js.Value, name string) (uint32, error) {
	value := object.Get(name)
	if value.Type() != js.TypeNumber {
		return 0, fmt.Errorf("paper-studio-wasm: %s must be a number", name)
	}
	number := value.Float()
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 1 || number > math.MaxUint32 || math.Trunc(number) != number {
		return 0, fmt.Errorf("paper-studio-wasm: %s must be a positive 32-bit integer", name)
	}
	return uint32(number), nil // #nosec G115 -- explicitly bounded to an integral uint32 above.
}

func jsFixedCoordinate(object js.Value, name string) (int64, error) {
	value := object.Get(name)
	if value.Type() != js.TypeNumber {
		return 0, fmt.Errorf("paper-studio-wasm: %s must be a number", name)
	}
	number := value.Float()
	if math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number ||
		number < -maxSafeJSInteger || number > maxSafeJSInteger {
		return 0, fmt.Errorf("paper-studio-wasm: %s must be a safe integer", name)
	}
	return int64(number), nil // #nosec G115 -- explicitly bounded to a safe integral int64 above.
}

func jsOptionalString(object js.Value, name string) string {
	value := object.Get(name)
	if value.Type() == js.TypeString {
		return value.String()
	}
	return ""
}

func renderPage(_ js.Value, arguments []js.Value) any {
	promise := js.Global().Get("Promise")
	executor := js.FuncOf(func(_ js.Value, callbacks []js.Value) any {
		resolve, reject := callbacks[0], callbacks[1]
		if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: render expects one Uint8Array")))
			return nil
		}
		length := arguments[0].Get("byteLength").Int()
		if length <= 0 || length > layoutengine.WebDisplayRenderMaxPayloadBytes {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: payload length is invalid")))
			return nil
		}
		payload := make([]byte, length)
		if copied := js.CopyBytesToGo(payload, arguments[0]); copied != length {
			reject.Invoke(jsError(errors.New("paper-studio-wasm: payload copy was incomplete")))
			return nil
		}
		go func() {
			artifact, err := layoutengine.RenderWebDisplayPayloadCached(context.Background(), payload, &renderCache)
			if err != nil {
				reject.Invoke(jsError(err))
				return
			}
			manifestJSON, err := artifact.CanonicalManifestJSON()
			if err != nil {
				reject.Invoke(jsError(err))
				return
			}
			png := artifact.PNG()
			encoded := js.Global().Get("Uint8Array").New(len(png))
			if copied := js.CopyBytesToJS(encoded, png); copied != len(png) {
				reject.Invoke(jsError(errors.New("paper-studio-wasm: PNG copy was incomplete")))
				return
			}
			result := js.Global().Get("Object").New()
			result.Set("manifest", js.Global().Get("JSON").Call("parse", string(manifestJSON)))
			result.Set("png", encoded)
			resolve.Invoke(result)
		}()
		return nil
	})
	result := promise.New(executor)
	executor.Release()
	return result
}

func jsError(err error) js.Value {
	return js.Global().Get("Error").New(err.Error())
}
