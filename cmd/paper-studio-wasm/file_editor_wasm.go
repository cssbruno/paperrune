// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"sync"
	"syscall/js"
	"time"

	"github.com/cssbruno/paperrune/document"
)

const playgroundFileEditDelay = 360 * time.Millisecond
const maxPlaygroundFileWorkspaces = 16

type playgroundFileSnapshot struct {
	playgroundCompileResult
	Source string `json:"source"`
	Data   string `json:"data"`
}

type playgroundFileEditor struct {
	host, input           js.Value
	onEditing, onSnapshot js.Value
	workspace             *playgroundFileWorkspace
	kind                  string
	page                  uint32
	vectorOnly            bool
	inputEvent, destroy   js.Func
	timer                 *time.Timer
	mu                    sync.Mutex
	destroyed             bool
}

type playgroundFileWorkspace struct {
	mu                     sync.Mutex
	source, data, scenario string
	options                document.PaperJSONOptions
	revision               uint64
}

type playgroundFileWorkspaceStore struct {
	mu     sync.Mutex
	drafts map[string]*playgroundFileWorkspace
	order  []string
}

var fileWorkspaces playgroundFileWorkspaceStore

func mountPlaygroundFileEditor(_ js.Value, arguments []js.Value) any {
	if len(arguments) != 1 || arguments[0].Type() != js.TypeObject {
		return jsError(errors.New("paper-studio-wasm: mountFileEditor expects one request object"))
	}
	editor, err := newPlaygroundFileEditor(arguments[0])
	if err != nil {
		return jsError(err)
	}
	return editor.mount()
}

func newPlaygroundFileEditor(value js.Value) (*playgroundFileEditor, error) {
	host := value.Get("host")
	onEditing := value.Get("onEditing")
	onSnapshot := value.Get("onSnapshot")
	if host.Type() != js.TypeObject || onEditing.Type() != js.TypeFunction || onSnapshot.Type() != js.TypeFunction {
		return nil, errors.New("paper-studio-wasm: mountFileEditor requires a host and callbacks")
	}
	hash, err := jsRequiredString(value, "hash")
	if err != nil || !playgroundDigest(hash) {
		return nil, errors.New("paper-studio-wasm: mountFileEditor hash must be a lowercase SHA-256 digest")
	}
	workspace, ok := planCache.load(hash)
	if !ok {
		return nil, errors.New("paper-studio-wasm: mountFileEditor workspace hash is not retained")
	}
	kind := jsOptionalString(value, "kind")
	if kind != "source" && kind != "data" {
		return nil, errors.New("paper-studio-wasm: mountFileEditor kind must be source or data")
	}
	page, err := jsRequiredPage(value)
	if err != nil {
		return nil, err
	}
	draft := fileWorkspaces.load(hash, workspace)
	source, data, _, _, _ := draft.snapshot()
	input := buildPlaygroundFileEditorDOM(host, kind, source, data)
	return &playgroundFileEditor{
		host: host, input: input, onEditing: onEditing, onSnapshot: onSnapshot,
		workspace: draft, kind: kind, page: page, vectorOnly: jsOptionalBool(value, "vectorOnly"),
	}, nil
}

func (store *playgroundFileWorkspaceStore) load(hash string, cached playgroundCachedPlan) *playgroundFileWorkspace {
	store.mu.Lock()
	defer store.mu.Unlock()
	if draft := store.drafts[hash]; draft != nil {
		return draft
	}
	if store.drafts == nil {
		store.drafts = make(map[string]*playgroundFileWorkspace, maxPlaygroundFileWorkspaces)
	}
	draft := &playgroundFileWorkspace{
		source: cached.source, data: cached.data, scenario: cached.scenario, options: cached.options,
	}
	store.drafts[hash] = draft
	store.order = append(store.order, hash)
	if len(store.order) > maxPlaygroundFileWorkspaces {
		delete(store.drafts, store.order[0])
		store.order = store.order[1:]
	}
	return draft
}

func (workspace *playgroundFileWorkspace) update(kind, draft string) uint64 {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if kind == "source" {
		workspace.source = draft
	} else {
		workspace.data = draft
	}
	workspace.revision++
	return workspace.revision
}

func (workspace *playgroundFileWorkspace) snapshot() (string, string, string, document.PaperJSONOptions, uint64) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.source, workspace.data, workspace.scenario, workspace.options, workspace.revision
}

func (workspace *playgroundFileWorkspace) current(revision uint64) bool {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return revision == workspace.revision
}

func buildPlaygroundFileEditorDOM(host js.Value, kind, source, data string) js.Value {
	input := js.Global().Get("document").Call("createElement", "textarea")
	input.Set("className", "wasm-file-editor")
	input.Set("spellcheck", false)
	input.Call("setAttribute", "aria-label", map[string]string{
		"source": "Paper source",
		"data":   "JSON data",
	}[kind])
	if kind == "source" {
		input.Set("value", source)
	} else {
		input.Set("value", data)
	}
	host.Set("textContent", "")
	host.Call("append", input)
	return input
}

func (editor *playgroundFileEditor) mount() js.Value {
	editor.inputEvent = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.queue()
		return nil
	})
	editor.destroy = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		editor.unmount()
		return nil
	})
	editor.input.Call("addEventListener", "input", editor.inputEvent)
	controller := js.Global().Get("Object").New()
	controller.Set("destroy", editor.destroy)
	return controller
}

func (editor *playgroundFileEditor) queue() {
	editor.mu.Lock()
	if editor.destroyed {
		editor.mu.Unlock()
		return
	}
	draft := editor.input.Get("value").String()
	revision := editor.workspace.update(editor.kind, draft)
	if editor.timer != nil {
		editor.timer.Stop()
	}
	editor.timer = time.AfterFunc(playgroundFileEditDelay, func() {
		editor.compile(revision)
	})
	editor.mu.Unlock()
	editor.onEditing.Invoke()
}

func (editor *playgroundFileEditor) compile(revision uint64) {
	source, data, scenario, options, _ := editor.workspace.snapshot()
	snapshot := compilePlaygroundFileSnapshot(source, data, scenario, editor.page, options, editor.vectorOnly)
	editor.mu.Lock()
	current := !editor.destroyed && editor.workspace.current(revision)
	editor.mu.Unlock()
	if !current {
		return
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	editor.onSnapshot.Invoke(js.Global().Get("JSON").Call("parse", string(encoded)))
}

func compilePlaygroundFileSnapshot(source, data, scenario string, page uint32, options document.PaperJSONOptions, vectorOnly bool) playgroundFileSnapshot {
	result, plan, err := planPlaygroundRequest(source, data, scenario, options)
	if err == nil {
		result, err = renderPlaygroundPlanPage(result, plan, page, !vectorOnly)
	}
	if err != nil && result.Error == "" {
		result, _ = playgroundCompileFailure(result, err)
	}
	return playgroundFileSnapshot{playgroundCompileResult: result, Source: source, Data: data}
}

func (editor *playgroundFileEditor) unmount() {
	editor.mu.Lock()
	if editor.destroyed {
		editor.mu.Unlock()
		return
	}
	editor.destroyed = true
	if editor.timer != nil {
		editor.timer.Stop()
	}
	editor.mu.Unlock()
	editor.input.Call("removeEventListener", "input", editor.inputEvent)
	editor.host.Set("textContent", "")
	editor.inputEvent.Release()
	editor.destroy.Release()
}
