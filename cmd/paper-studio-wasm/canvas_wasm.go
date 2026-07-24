// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"math"
	"syscall/js"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func paintPlaygroundPage(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
		return jsError(errors.New("paper-studio-wasm: paintPage expects one request object"))
	}
	request := arguments[0]
	canvas := request.Get("canvas")
	if canvas.Type() != js.TypeObject {
		return jsError(errors.New("paper-studio-wasm: paintPage requires a canvas"))
	}
	hash, err := jsRequiredString(request, "hash")
	if err != nil || !playgroundDigest(hash) {
		return jsError(errors.New("paper-studio-wasm: paintPage hash must be a lowercase SHA-256 digest"))
	}
	page, err := jsRequiredPage(request)
	if err != nil {
		return jsError(err)
	}
	workspace, ok := planCache.load(hash)
	if !ok {
		return jsError(errors.New("paper-studio-wasm: paintPage workspace hash is not retained"))
	}
	graphics, err := workspace.plan.WebDisplayGraphicsPage(page)
	if err != nil {
		return jsError(err)
	}
	if err := paintPlaygroundCanvas(canvas, graphics); err != nil {
		return jsError(err)
	}
	return true
}

func paintPlaygroundCanvas(canvas js.Value, page document.PaperPlanWebGraphicsPage) error {
	if page.Width <= 0 || page.Height <= 0 || page.FixedScale <= 0 {
		return errors.New("paper-studio-wasm: paintPage received invalid page geometry")
	}
	cssWidth := canvas.Get("clientWidth").Float()
	cssHeight := canvas.Get("clientHeight").Float()
	if cssWidth <= 0 || cssHeight <= 0 {
		return errors.New("paper-studio-wasm: paintPage canvas has no visible size")
	}
	dpr := 1.0
	if value := js.Global().Get("devicePixelRatio"); value.Type() == js.TypeNumber && value.Float() > 0 {
		dpr = value.Float()
	}
	pixelWidth := int(math.Ceil(cssWidth * dpr))
	pixelHeight := int(math.Ceil(cssHeight * dpr))
	canvas.Set("width", pixelWidth)
	canvas.Set("height", pixelHeight)
	context := canvas.Call("getContext", "2d")
	if context.IsNull() || context.IsUndefined() {
		return errors.New("paper-studio-wasm: paintPage could not acquire a 2D context")
	}
	context.Call("setTransform", float64(pixelWidth)/float64(page.Width), 0, 0, float64(pixelHeight)/float64(page.Height), 0, 0)
	context.Set("fillStyle", "#ffffff")
	context.Call("fillRect", 0, 0, float64(page.Width), float64(page.Height))

	for _, command := range page.Commands {
		switch command.Kind {
		case layoutengine.CommandSaveState:
			context.Call("save")
		case layoutengine.CommandRestoreState:
			context.Call("restore")
		case layoutengine.CommandTransform:
			if int(command.Payload) >= len(page.Transforms) {
				return errors.New("paper-studio-wasm: paintPage transform payload is outside the plan")
			}
			transform := page.Transforms[command.Payload]
			scale := float64(page.FixedScale)
			context.Call("transform", float64(transform.A)/scale, float64(transform.B)/scale,
				float64(transform.C)/scale, float64(transform.D)/scale, float64(transform.TX), float64(transform.TY))
		case layoutengine.CommandClip:
			if int(command.Payload) >= len(page.Clips) {
				return errors.New("paper-studio-wasm: paintPage clip payload is outside the plan")
			}
			clip := page.Clips[command.Payload]
			if int(clip.Path) >= len(page.Paths) {
				return errors.New("paper-studio-wasm: paintPage clip path is outside the plan")
			}
			canvasPath(context, page.Paths[clip.Path])
			context.Call("clip", canvasFillRule(clip.Rule))
		case layoutengine.CommandFillPath:
			if int(command.Payload) >= len(page.Fills) {
				return errors.New("paper-studio-wasm: paintPage fill payload is outside the plan")
			}
			fill := page.Fills[command.Payload]
			if int(fill.Path) >= len(page.Paths) {
				return errors.New("paper-studio-wasm: paintPage fill path is outside the plan")
			}
			context.Call("save")
			context.Set("fillStyle", canvasColor(fill.Color))
			context.Set("globalAlpha", canvasOpacity(fill.Opacity))
			canvasPath(context, page.Paths[fill.Path])
			context.Call("fill", canvasFillRule(fill.Rule))
			context.Call("restore")
		case layoutengine.CommandStrokePath:
			if int(command.Payload) >= len(page.Strokes) {
				return errors.New("paper-studio-wasm: paintPage stroke payload is outside the plan")
			}
			stroke := page.Strokes[command.Payload]
			if int(stroke.Path) >= len(page.Paths) {
				return errors.New("paper-studio-wasm: paintPage stroke path is outside the plan")
			}
			context.Call("save")
			context.Set("strokeStyle", canvasColor(stroke.Color))
			context.Set("globalAlpha", canvasOpacity(stroke.Opacity))
			context.Set("lineWidth", float64(stroke.Width))
			context.Set("lineCap", canvasLineCap(stroke.LineCap))
			context.Set("lineJoin", canvasLineJoin(stroke.LineJoin))
			dash := js.Global().Get("Array").New(len(stroke.Dash))
			for index, value := range stroke.Dash {
				dash.SetIndex(index, float64(value))
			}
			context.Call("setLineDash", dash)
			context.Set("lineDashOffset", float64(stroke.DashOffset))
			canvasPath(context, page.Paths[stroke.Path])
			context.Call("stroke")
			context.Call("restore")
		}
	}
	return nil
}

func canvasPath(context js.Value, path layoutengine.PlannedPath) {
	context.Call("beginPath")
	for _, segment := range path.Segments {
		switch segment.Kind {
		case layoutengine.PathMoveTo:
			context.Call("moveTo", float64(segment.Point.X), float64(segment.Point.Y))
		case layoutengine.PathLineTo:
			context.Call("lineTo", float64(segment.Point.X), float64(segment.Point.Y))
		case layoutengine.PathCubicTo:
			context.Call("bezierCurveTo", float64(segment.Control1.X), float64(segment.Control1.Y),
				float64(segment.Control2.X), float64(segment.Control2.Y), float64(segment.Point.X), float64(segment.Point.Y))
		case layoutengine.PathClose:
			context.Call("closePath")
		}
	}
}

func canvasFillRule(rule layoutengine.FillRule) string {
	if rule == layoutengine.FillEvenOdd {
		return "evenodd"
	}
	return "nonzero"
}

func canvasLineCap(value layoutengine.StrokeLineCap) string {
	switch value {
	case layoutengine.StrokeCapRound:
		return "round"
	case layoutengine.StrokeCapSquare:
		return "square"
	default:
		return "butt"
	}
}

func canvasLineJoin(value layoutengine.StrokeLineJoin) string {
	switch value {
	case layoutengine.StrokeJoinRound:
		return "round"
	case layoutengine.StrokeJoinBevel:
		return "bevel"
	default:
		return "miter"
	}
}

func canvasColor(color layoutengine.CoreRGBColor) string {
	if !color.Set {
		return "#000000"
	}
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}

func canvasOpacity(value layoutengine.Fixed) float64 {
	if value == 0 {
		return 1
	}
	return math.Max(0, math.Min(1, float64(value)/float64(layoutengine.FixedScale)))
}
