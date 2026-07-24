// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"sync"
	"syscall/js"
)

type playgroundEditorApplied struct {
	Edit playgroundEditResult    `json:"edit"`
	Page playgroundCompileResult `json:"page"`
}

type playgroundEditorDOM struct {
	host, input, message js.Value
}

type playgroundDOMEditor struct {
	dom                  playgroundEditorDOM
	request              playgroundEditRequest
	page                 uint32
	vectorOnly           bool
	onApplied, onCancel  js.Value
	keydown, input, blur js.Func
	pointerDown, destroy js.Func
	mu                   sync.Mutex
	busy, destroyed      bool
	history              []string
	historyIndex         int
	hiddenGlyphs         []js.Value
}

func mountPlaygroundEditor(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
		return jsError(errors.New("paper-studio-wasm: mountEditor expects one request object"))
	}
	value := arguments[0]
	editor, err := newPlaygroundDOMEditor(value)
	if err != nil {
		return jsError(err)
	}
	return editor.mount()
}

func newPlaygroundDOMEditor(value js.Value) (*playgroundDOMEditor, error) {
	host := value.Get("host")
	onApplied := value.Get("onApplied")
	onCancel := value.Get("onCancel")
	if host.Type() != js.TypeObject || onApplied.Type() != js.TypeFunction || onCancel.Type() != js.TypeFunction {
		return nil, errors.New("paper-studio-wasm: mountEditor requires a host and callbacks")
	}
	request, err := playgroundEditRequestFromJS(value)
	if err != nil {
		return nil, err
	}
	page, err := jsRequiredPage(value)
	if err != nil {
		return nil, err
	}
	editor := &playgroundDOMEditor{
		dom:     buildPlaygroundEditorDOM(host, request.text, playgroundEditorLabel(value)),
		request: request, page: page, onApplied: onApplied, onCancel: onCancel,
		history: []string{request.text}, vectorOnly: jsOptionalBool(value, "vectorOnly"),
	}
	editor.hiddenGlyphs = hidePlaygroundEditorGlyphs(host)
	return editor, nil
}

func buildPlaygroundEditorDOM(host js.Value, text, label string) playgroundEditorDOM {
	document := js.Global().Get("document")
	input := document.Call("createElement", "div")
	input.Set("className", "wasm-direct-editor")
	input.Set("textContent", text)
	input.Set("contentEditable", "plaintext-only")
	input.Set("spellcheck", true)
	input.Call("setAttribute", "aria-label", "Edit selected document text")
	input.Call("setAttribute", "role", "textbox")
	input.Call("setAttribute", "aria-multiline", "true")
	input.Call("setAttribute", "title", label+" · click outside to apply · Escape to cancel")
	message := document.Call("createElement", "span")
	message.Set("className", "wasm-direct-editor-error")
	message.Call("setAttribute", "aria-live", "polite")
	host.Set("textContent", "")
	host.Call("append", input, message)
	adoptPlaygroundEditorTypography(host, input)
	return playgroundEditorDOM{host: host, input: input, message: message}
}

func (editor *playgroundDOMEditor) mount() js.Value {
	editor.keydown = js.FuncOf(editor.handleKeydown)
	editor.input = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.recordHistory()
		return nil
	})
	editor.blur = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.applyEdit()
		return nil
	})
	editor.pointerDown = js.FuncOf(func(_ js.Value, event []js.Value) any {
		event[0].Call("stopPropagation")
		return nil
	})
	editor.destroy = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.unmount()
		return nil
	})
	editor.dom.input.Call("addEventListener", "keydown", editor.keydown)
	editor.dom.input.Call("addEventListener", "input", editor.input)
	editor.dom.input.Call("addEventListener", "blur", editor.blur)
	editor.dom.input.Call("addEventListener", "pointerdown", editor.pointerDown)
	editor.dom.input.Call("addEventListener", "dblclick", editor.pointerDown)
	editor.dom.input.Call("focus", map[string]any{"preventScroll": true})
	selectPlaygroundEditorText(editor.dom.input)

	controller := js.Global().Get("Object").New()
	controller.Set("destroy", editor.destroy)
	return controller
}

func (editor *playgroundDOMEditor) handleKeydown(_ js.Value, event []js.Value) any {
	key := event[0].Get("key").String()
	modifier := event[0].Get("metaKey").Bool() || event[0].Get("ctrlKey").Bool()
	switch {
	case key == "Escape":
		event[0].Call("preventDefault")
		editor.onCancel.Invoke()
	case modifier && (key == "z" || key == "Z"):
		event[0].Call("preventDefault")
		direction := -1
		if event[0].Get("shiftKey").Bool() {
			direction = 1
		}
		editor.moveHistory(direction)
	case modifier && (key == "y" || key == "Y"):
		event[0].Call("preventDefault")
		editor.moveHistory(1)
	case key == "Enter" && modifier:
		event[0].Call("preventDefault")
		editor.applyEdit()
	}
	return nil
}

func (editor *playgroundDOMEditor) recordHistory() {
	text := editor.dom.input.Get("textContent").String()
	if editor.history[editor.historyIndex] == text {
		return
	}
	editor.history = append(editor.history[:editor.historyIndex+1], text)
	editor.historyIndex++
}

func (editor *playgroundDOMEditor) moveHistory(direction int) {
	next := editor.historyIndex + direction
	if next < 0 || next >= len(editor.history) {
		return
	}
	editor.historyIndex = next
	editor.dom.input.Set("textContent", editor.history[next])
	selectPlaygroundEditorText(editor.dom.input)
}

func (editor *playgroundDOMEditor) applyEdit() {
	editor.mu.Lock()
	if editor.busy || editor.destroyed {
		editor.mu.Unlock()
		return
	}
	editor.busy = true
	editor.request.text = editor.dom.input.Get("textContent").String()
	editor.mu.Unlock()
	editor.setBusyDOM(true)
	go func() {
		result, err := applyAndRenderPlaygroundEdit(editor.request, editor.page, editor.vectorOnly)
		if err != nil {
			editor.showError(err)
			return
		}
		editor.onApplied.Invoke(js.Global().Get("JSON").Call("parse", string(result)))
	}()
}

func applyAndRenderPlaygroundEdit(request playgroundEditRequest, page uint32, vectorOnly bool) ([]byte, error) {
	edited, err := applyPlaygroundEdit(request)
	if err != nil {
		return nil, err
	}
	workspace, ok := planCache.load(edited.Hash)
	if !ok {
		return nil, errors.New("paper-studio-wasm: edited workspace is not retained")
	}
	rendered, err := renderPlaygroundPlanPage(playgroundCompileResult{
		OK: true, Pages: workspace.plan.PageCount(), Hash: edited.Hash,
	}, workspace.plan, page, !vectorOnly)
	if err != nil {
		return nil, err
	}
	return json.Marshal(playgroundEditorApplied{Edit: edited, Page: rendered})
}

func (editor *playgroundDOMEditor) showError(err error) {
	editor.dom.message.Set("textContent", err.Error())
	editor.dom.host.Get("classList").Call("add", "has-error")
	editor.mu.Lock()
	editor.busy = false
	editor.mu.Unlock()
	editor.setBusyDOM(false)
}

func (editor *playgroundDOMEditor) setBusyDOM(busy bool) {
	editor.dom.input.Set("contentEditable", map[bool]string{true: "false", false: "plaintext-only"}[busy])
	if busy {
		editor.dom.host.Get("classList").Call("add", "is-busy")
		return
	}
	editor.dom.host.Get("classList").Call("remove", "is-busy")
}

func (editor *playgroundDOMEditor) unmount() {
	editor.mu.Lock()
	if editor.destroyed {
		editor.mu.Unlock()
		return
	}
	editor.destroyed = true
	editor.mu.Unlock()
	editor.dom.input.Call("removeEventListener", "keydown", editor.keydown)
	editor.dom.input.Call("removeEventListener", "input", editor.input)
	editor.dom.input.Call("removeEventListener", "blur", editor.blur)
	editor.dom.input.Call("removeEventListener", "pointerdown", editor.pointerDown)
	editor.dom.input.Call("removeEventListener", "dblclick", editor.pointerDown)
	for _, glyph := range editor.hiddenGlyphs {
		glyph.Get("style").Set("visibility", "")
	}
	editor.dom.host.Set("textContent", "")
	editor.keydown.Release()
	editor.input.Release()
	editor.blur.Release()
	editor.pointerDown.Release()
	editor.destroy.Release()
}

func selectPlaygroundEditorText(input js.Value) {
	selection := js.Global().Get("getSelection").Invoke()
	rangeValue := js.Global().Get("document").Call("createRange")
	rangeValue.Call("selectNodeContents", input)
	selection.Call("removeAllRanges")
	selection.Call("addRange", rangeValue)
}

func adoptPlaygroundEditorTypography(host, input js.Value) {
	hostRect := host.Call("getBoundingClientRect")
	lines := host.Get("parentElement").Call("querySelectorAll", ".document-text-run")
	var best js.Value
	bestArea := 0.0
	for index := 0; index < lines.Get("length").Int(); index++ {
		line := lines.Index(index)
		rect := line.Call("getBoundingClientRect")
		width := overlap(
			hostRect.Get("left").Float(), hostRect.Get("right").Float(),
			rect.Get("left").Float(), rect.Get("right").Float(),
		)
		height := overlap(
			hostRect.Get("top").Float(), hostRect.Get("bottom").Float(),
			rect.Get("top").Float(), rect.Get("bottom").Float(),
		)
		if area := width * height; area > bestArea {
			best, bestArea = line, area
		}
	}
	if bestArea == 0 {
		return
	}
	inputStyle := input.Get("style")
	computed := js.Global().Call("getComputedStyle", best)
	inputStyle.Set("fontFamily", computed.Get("fontFamily").String())
	inputStyle.Set("fontWeight", computed.Get("fontWeight").String())
	inputStyle.Set("fontStyle", computed.Get("fontStyle").String())
	inputStyle.Set("fontSize", computed.Get("fontSize").String())
	color := computed.Get("color").String()
	inputStyle.Set("color", color)
	inputStyle.Set("-webkit-text-fill-color", color)
	inputStyle.Call("setProperty", "--selection-color", color)
}

func hidePlaygroundEditorGlyphs(host js.Value) []js.Value {
	hostRect := host.Call("getBoundingClientRect")
	lines := host.Get("parentElement").Call("querySelectorAll", ".document-text-run")
	hidden := make([]js.Value, 0, lines.Get("length").Int())
	for index := 0; index < lines.Get("length").Int(); index++ {
		line := lines.Index(index)
		rect := line.Call("getBoundingClientRect")
		width := overlap(
			hostRect.Get("left").Float(), hostRect.Get("right").Float(),
			rect.Get("left").Float(), rect.Get("right").Float(),
		)
		height := overlap(
			hostRect.Get("top").Float(), hostRect.Get("bottom").Float(),
			rect.Get("top").Float(), rect.Get("bottom").Float(),
		)
		if width*height <= 0 {
			continue
		}
		line.Get("style").Set("visibility", "hidden")
		hidden = append(hidden, line)
	}
	return hidden
}

func overlap(aStart, aEnd, bStart, bEnd float64) float64 {
	start := max(aStart, bStart)
	end := min(aEnd, bEnd)
	return max(0, end-start)
}

func playgroundEditorLabel(value js.Value) string {
	if jsOptionalString(value, "mode") != "data" {
		return "Paper source"
	}
	if binding := jsOptionalString(value, "binding"); binding != "" {
		return "JSON · " + binding
	}
	return "JSON data"
}
