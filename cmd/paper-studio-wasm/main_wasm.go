// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"syscall/js"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/layoutengine"
)

var renderFunction js.Func
var compileFunction js.Func
var renderCache layoutengine.WebDisplayRenderCache

const (
	maxPlaygroundSourceBytes = 1 << 20
	maxPlaygroundDataBytes   = 4 << 20
)

func main() {
	renderFunction = js.FuncOf(renderPage)
	compileFunction = js.FuncOf(compilePaper)
	engine := js.Global().Get("Object").New()
	engine.Set("formatVersion", layoutengine.WebDisplayRenderPayloadVersion)
	engine.Set("rendererVersion", layoutengine.DisplayRasterRendererVersion)
	engine.Set("render", renderFunction)
	engine.Set("compile", compileFunction)
	js.Global().Set("PaperStudioWASM", engine)
	<-make(chan struct{})
}

type playgroundCompileResult struct {
	OK          bool                       `json:"ok"`
	Pages       int                        `json:"pages"`
	Page        uint32                     `json:"page,omitempty"`
	Hash        string                     `json:"hash,omitempty"`
	Diagnostics []document.PaperDiagnostic `json:"diagnostics,omitempty"`
	Error       string                     `json:"error,omitempty"`
	SVG         string                     `json:"svg,omitempty"`
	PageWidth   int64                      `json:"page_width,omitempty"`
	PageHeight  int64                      `json:"page_height,omitempty"`
	FixedScale  int64                      `json:"fixed_scale,omitempty"`
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
	var plan document.PaperPlan
	var planned document.PaperPlanResult
	var err error
	switch {
	case data != "" && scenario != "":
		return playgroundCompileResult{}, errors.New("paper-studio-wasm: choose JSON data or a declared scenario, not both")
	case data != "":
		plan, planned, err = document.PlanPaperJSONWithOptions("playground.paper", source, []byte(data), options)
	case scenario != "":
		plan, planned, err = document.PlanPaperScenario("playground.paper", source, scenario)
	default:
		plan, planned, err = document.PlanPaper("playground.paper", source)
	}
	result := playgroundCompileResult{OK: planned.OK(), Pages: planned.Pages, Hash: planned.Hash, Diagnostics: planned.Diagnostics}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	if page == 0 || int(page) > plan.PageCount() {
		return result, errors.New("paper-studio-wasm: requested page is outside the compiled plan")
	}
	capture, err := plan.CaptureDisplayPageSVG(context.Background(), page, nil)
	if err != nil {
		return result, err
	}
	result.Page, result.SVG = page, string(capture.SVG)
	result.PageWidth, result.PageHeight, result.FixedScale = capture.PageWidth, capture.PageHeight, capture.FixedScale
	return result, nil
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
