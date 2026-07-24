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
	host, form, input, message, cancel, apply js.Value
}

type playgroundDOMEditor struct {
	dom                  playgroundEditorDOM
	request              playgroundEditRequest
	page                 uint32
	onApplied, onCancel  js.Value
	submit, keydown      js.Func
	cancelClick, destroy js.Func
	mu                   sync.Mutex
	busy, destroyed      bool
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
	return &playgroundDOMEditor{
		dom:     buildPlaygroundEditorDOM(host, request.text, playgroundEditorLabel(value)),
		request: request, page: page, onApplied: onApplied, onCancel: onCancel,
	}, nil
}

func buildPlaygroundEditorDOM(host js.Value, text, label string) playgroundEditorDOM {
	document := js.Global().Get("document")
	form := document.Call("createElement", "form")
	form.Set("className", "wasm-inline-editor")
	input := document.Call("createElement", "textarea")
	input.Set("value", text)
	input.Set("rows", 2)
	input.Set("spellcheck", true)
	input.Call("setAttribute", "aria-label", "Edit selected document text")
	actions := document.Call("createElement", "div")
	actions.Set("className", "wasm-inline-actions")
	message := document.Call("createElement", "span")
	message.Set("textContent", label)
	cancel := document.Call("createElement", "button")
	cancel.Set("type", "button")
	cancel.Set("textContent", "Cancel")
	apply := document.Call("createElement", "button")
	apply.Set("type", "submit")
	apply.Set("textContent", "Apply")
	actions.Call("append", message, cancel, apply)
	form.Call("append", input, actions)
	host.Set("textContent", "")
	host.Call("append", form)
	return playgroundEditorDOM{host: host, form: form, input: input, message: message, cancel: cancel, apply: apply}
}

func (editor *playgroundDOMEditor) mount() js.Value {
	editor.submit = js.FuncOf(func(_ js.Value, event []js.Value) any {
		event[0].Call("preventDefault")
		editor.applyEdit()
		return nil
	})
	editor.keydown = js.FuncOf(editor.handleKeydown)
	editor.cancelClick = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.onCancel.Invoke()
		return nil
	})
	editor.destroy = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.unmount()
		return nil
	})
	editor.dom.form.Call("addEventListener", "submit", editor.submit)
	editor.dom.input.Call("addEventListener", "keydown", editor.keydown)
	editor.dom.cancel.Call("addEventListener", "click", editor.cancelClick)
	editor.dom.input.Call("focus", map[string]any{"preventScroll": true})
	editor.dom.input.Call("select")

	controller := js.Global().Get("Object").New()
	controller.Set("destroy", editor.destroy)
	return controller
}

func (editor *playgroundDOMEditor) handleKeydown(_ js.Value, event []js.Value) any {
	key := event[0].Get("key").String()
	switch {
	case key == "Escape":
		event[0].Call("preventDefault")
		editor.onCancel.Invoke()
	case key == "Enter" && (event[0].Get("metaKey").Bool() || event[0].Get("ctrlKey").Bool()):
		event[0].Call("preventDefault")
		editor.applyEdit()
	}
	return nil
}

func (editor *playgroundDOMEditor) applyEdit() {
	editor.mu.Lock()
	if editor.busy || editor.destroyed {
		editor.mu.Unlock()
		return
	}
	editor.busy = true
	editor.request.text = editor.dom.input.Get("value").String()
	editor.mu.Unlock()
	editor.setBusyDOM(true)
	go func() {
		result, err := applyAndRenderPlaygroundEdit(editor.request, editor.page)
		if err != nil {
			editor.showError(err)
			return
		}
		editor.onApplied.Invoke(js.Global().Get("JSON").Call("parse", string(result)))
	}()
}

func applyAndRenderPlaygroundEdit(request playgroundEditRequest, page uint32) ([]byte, error) {
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
	}, workspace.plan, page)
	if err != nil {
		return nil, err
	}
	return json.Marshal(playgroundEditorApplied{Edit: edited, Page: rendered})
}

func (editor *playgroundDOMEditor) showError(err error) {
	editor.dom.message.Set("textContent", err.Error())
	editor.dom.message.Get("classList").Call("add", "is-error")
	editor.mu.Lock()
	editor.busy = false
	editor.mu.Unlock()
	editor.setBusyDOM(false)
}

func (editor *playgroundDOMEditor) setBusyDOM(busy bool) {
	editor.dom.input.Set("disabled", busy)
	editor.dom.cancel.Set("disabled", busy)
	editor.dom.apply.Set("disabled", busy)
	if busy {
		editor.dom.form.Get("classList").Call("add", "is-busy")
		editor.dom.apply.Set("textContent", "Applying…")
		return
	}
	editor.dom.form.Get("classList").Call("remove", "is-busy")
	editor.dom.apply.Set("textContent", "Apply")
}

func (editor *playgroundDOMEditor) unmount() {
	editor.mu.Lock()
	if editor.destroyed {
		editor.mu.Unlock()
		return
	}
	editor.destroyed = true
	editor.mu.Unlock()
	editor.dom.form.Call("removeEventListener", "submit", editor.submit)
	editor.dom.input.Call("removeEventListener", "keydown", editor.keydown)
	editor.dom.cancel.Call("removeEventListener", "click", editor.cancelClick)
	editor.dom.host.Set("textContent", "")
	editor.submit.Release()
	editor.keydown.Release()
	editor.cancelClick.Release()
	editor.destroy.Release()
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
