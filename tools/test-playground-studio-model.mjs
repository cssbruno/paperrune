// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

import {
  boxAsPercent,
  contentDescriptor,
  findNodeByID,
  outlineNodes,
  pickHitTarget,
  traceBindingDescriptor,
} from '../docs/.vitepress/components/playground/studio-model.mjs';

const literal = node('paragraph', '@literal', [
  property('text', {kind: 'string', string_value: 'Editable copy'}),
]);
const bound = node('heading', '@bound', [
  property('bind', {kind: 'string', string_value: 'report.title'}),
]);
const computed = node('paragraph', '@computed', [
  property('text', {kind: 'expression', expression_value: 'approved ? "YES" : "NO"'}),
]);
const computedPath = node('cell', '@computed-path', [
  property('text', {kind: 'expression', expression_value: 'report.title'}),
]);
const boundCell = node('cell', '@bound-cell', [
  property('bind', {kind: 'string', string_value: 'report.title'}),
]);
const literalCell = node('cell', '@literal-cell', [
  property('text', {kind: 'string', string_value: 'ANALYTE'}),
]);
const anonymousCell = node('cell', '', [
  property('text', {kind: 'string', string_value: 'REFERENCE'}),
], 412);
const repeatedCell = node('cell', '', [
  property('bind', {kind: 'string', string_value: 'analyte'}),
], 518);
const root = node('document', '@document', [
  {node: node('page', '@page', [
    {node: node('body', '@body', [
      {node: literal},
      {node: bound},
      {node: computed},
      {node: computedPath},
      {node: boundCell},
      {node: literalCell},
      {node: anonymousCell},
      {node: repeatedCell},
    ])},
  ])},
]);
const data = '{"report":{"title":"Draft","results":[{"analyte":"Leukocytes","value":5.8}]},"approved":true}';

assert.equal(findNodeByID(root, '@bound'), bound);
assert.deepEqual(contentDescriptor(literal, data), {
  editable: true,
  mode: 'source',
  value: 'Editable copy',
  target: '@literal',
});
assert.deepEqual(contentDescriptor(bound, data), {
  editable: true,
  mode: 'data',
  binding: 'report.title',
  pointer: '/report/title',
  value: 'Draft',
});
assert.equal(contentDescriptor(computed, data).computed, true);
assert.equal(contentDescriptor(computedPath, data).computed, true);
assert.equal(contentDescriptor(computedPath, data).editable, false);
assert.deepEqual(contentDescriptor(boundCell, data), {
  editable: true,
  mode: 'data',
  binding: 'report.title',
  pointer: '/report/title',
  value: 'Draft',
});
assert.deepEqual(contentDescriptor(literalCell, data), {
  editable: true,
  mode: 'source',
  value: 'ANALYTE',
  target: '@literal-cell',
});
assert.deepEqual(contentDescriptor(anonymousCell, data), {
  editable: true,
  mode: 'source',
  value: 'REFERENCE',
  sourceOffset: 412,
});
assert.deepEqual(contentDescriptor(repeatedCell, data), {
  editable: false,
  binding: 'analyte',
  reason: 'Binding "analyte" is missing from JSON data.',
});

const hit = {
  Fragments: [
    {Key: '@missing', ContentBox: {X: 0, Y: 0, Width: 1, Height: 1}},
    {Key: '@expanded-instance-@bound', ContentBox: {X: 10, Y: 20, Width: 30, Height: 40}},
  ],
};
assert.equal(pickHitTarget(hit, root, data).id, '@bound');
assert.deepEqual(boxAsPercent(hit.Fragments[1].ContentBox, 100, 200), {
  left: 10,
  top: 10,
  width: 30,
  height: 20,
});
assert(outlineNodes(root).some(({id}) => id === '@literal'));

const trace = {
  provenance: {
    bindings: [{
      node: '',
      kind: 'cell',
      path: '@report.results[].analyte',
      instance: 'results[item-02d6f8d9b008205a]',
      json_pointer: '/report/results/0/analyte',
    }],
  },
};
assert.deepEqual(traceBindingDescriptor(trace, data), {
  editable: true,
  mode: 'data',
  binding: '@report.results[].analyte',
  pointer: '/report/results/0/analyte',
  value: 'Leukocytes',
});
assert.equal(traceBindingDescriptor({provenance: {bindings: []}}, data).editable, false);

const anonymousHit = {
  Fragments: [{
    Key: '@expanded-cell',
    Source: {start: {offset: 412}},
    ContentBox: {X: 4, Y: 8, Width: 12, Height: 16},
  }],
};
assert.equal(pickHitTarget(anonymousHit, root, data).content.sourceOffset, 412);

const repeatedHit = {
  Fragments: [{
    Key: '@expanded-repeat-cell',
    Source: {start: {offset: 518}},
    ContentBox: {X: 4, Y: 8, Width: 12, Height: 16},
  }],
};
assert.equal(pickHitTarget(repeatedHit, root, data).content.binding, 'analyte');

const canvasSource = await readFile(new URL('../docs/.vitepress/components/playground/StudioCanvas.vue', import.meta.url), 'utf8');
const playgroundSource = await readFile(new URL('../docs/.vitepress/components/Playground.vue', import.meta.url), 'utf8');
const fileEditorComponent = await readFile(new URL('../docs/.vitepress/components/playground/WASMFileEditor.vue', import.meta.url), 'utf8');
const wasmEditorSource = await readFile(new URL('../cmd/paper-studio-wasm/editor_wasm.go', import.meta.url), 'utf8');
const wasmFileEditorSource = await readFile(new URL('../cmd/paper-studio-wasm/file_editor_wasm.go', import.meta.url), 'utf8');
assert(!canvasSource.includes('v-model="inlineDraft"'));
assert(!canvasSource.includes('commit-inline'));
assert(canvasSource.includes('engine.mountEditor(request)'));
assert(canvasSource.includes('class="document-graphics"'));
assert(canvasSource.includes('engine.paintPage({'));
assert(!canvasSource.includes('<img'));
assert(!canvasSource.includes('<svg'));
assert(!canvasSource.includes('v-html'));
assert(!canvasSource.includes('display-svg'));
assert(!canvasSource.includes('sanitizeDisplaySVG'));
assert(canvasSource.includes('vectorOnly: true'));
assert(canvasSource.includes('class="document-text-run"'));
assert(canvasSource.includes("'--selection-color': run.color"));
assert(canvasSource.includes('color: var(--selection-color)'));
assert(!canvasSource.includes('class="selectable-line"'));
assert(!playgroundSource.includes('v-model="source"'));
assert(!playgroundSource.includes('v-model="data"'));
assert(!playgroundSource.includes('watch([source, data]'));
assert(!playgroundSource.includes('result.png'));
assert(!playgroundSource.includes('result.svg'));
assert(!playgroundSource.includes(':svg='));
assert(playgroundSource.includes('vectorOnly: true'));
assert(fileEditorComponent.includes('props.engine.mountFileEditor'));
assert(fileEditorComponent.includes('vectorOnly: true'));
assert(wasmEditorSource.includes('input.Set("contentEditable", "plaintext-only")'));
assert(!wasmEditorSource.includes('document.Call("createElement", "textarea")'));
assert(!wasmEditorSource.includes('textContent", "Apply"'));
assert(wasmEditorSource.includes('editor.moveHistory(direction)'));
assert(wasmEditorSource.includes('applyPlaygroundEdit(request)'));
assert(wasmEditorSource.includes('".document-text-run"'));
assert(!wasmEditorSource.includes('querySelector", ":scope > img"'));
assert(wasmFileEditorSource.includes('time.AfterFunc(playgroundFileEditDelay'));
assert(wasmFileEditorSource.includes('planPlaygroundRequest(source, data'));

console.log('playground Studio model: selection plus WASM-owned inline and file editors verified');

function node(kind, id, members = [], offset = 0) {
  return {
    kind,
    id,
    members,
    header_span: {start: {line: 1, column: 1}},
    span: {start: {offset}, end: {offset: offset + 80}},
  };
}

function property(name, value) {
  return {property: {name, value}};
}
