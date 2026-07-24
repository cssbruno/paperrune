<script setup>
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue';

const props = defineProps({
  textRuns: {type: Array, default: () => []},
  fonts: {type: Array, default: () => []},
  planHash: {type: String, default: ''},
  page: {type: Number, default: 1},
  pageX: {type: Number, default: 0},
  pageY: {type: Number, default: 0},
  pageWidth: {type: Number, default: 0},
  pageHeight: {type: Number, default: 0},
  fixedScale: {type: Number, default: 64},
  overflow: {type: Object, default: null},
  state: {type: String, default: 'loading'},
  status: {type: String, default: ''},
  failure: {type: String, default: ''},
  stale: {type: Boolean, default: false},
  loadProgress: {type: Number, default: 0},
  selection: {type: Object, default: null},
  inlineEditor: {type: Object, default: null},
  wasmEngine: {type: Object, default: null},
});

const emit = defineEmits(['page-point', 'edit-point', 'move-selection', 'editor-applied', 'editor-error', 'render-error', 'cancel-inline', 'retry']);
const pageStage = ref(null);
const graphicsCanvas = ref(null);
const displayTextRuns = ref([]);
const editorHost = ref(null);
const dragState = ref(null);
let resizeObserver;
let measureToken = 0;
let wasmEditorController;
const loadedFontFamilies = new Set();
const overflowText = computed(() => {
  if (!props.overflow?.records) return '';
  const edges = [
    ['left', props.overflow.left_fixed],
    ['top', props.overflow.top_fixed],
    ['right', props.overflow.right_fixed],
    ['bottom', props.overflow.bottom_fixed],
  ].filter(([, amount]) => Number(amount) > 0)
    .map(([edge, amount]) => `${formatFixedPoints(amount)}pt ${edge}`);
  return `Content extends outside the page${edges.length ? ` · ${edges.join(' · ')}` : ''}`;
});
const pageStyle = computed(() => ({
  aspectRatio: props.pageWidth > 0 && props.pageHeight > 0 ? `${props.pageWidth} / ${props.pageHeight}` : undefined,
}));

watch([
  () => props.wasmEngine,
  () => props.planHash,
  () => props.page,
  () => props.pageWidth,
  () => props.pageHeight,
  () => props.textRuns,
  () => props.fonts,
], () => {
  displayTextRuns.value = [];
  void refreshRenderedPage();
}, {immediate: true});

watch([() => props.inlineEditor, () => props.wasmEngine], ([editor, engine]) => {
  destroyWASMEditor();
  if (editor && engine) void mountWASMEditor(editor, engine);
}, {immediate: true});

onMounted(() => {
  if (typeof ResizeObserver === 'function') {
    resizeObserver = new ResizeObserver(() => void refreshRenderedPage());
    if (pageStage.value) resizeObserver.observe(pageStage.value);
  }
  void refreshRenderedPage();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  destroyWASMEditor();
});

async function mountWASMEditor(editor, engine) {
  await nextTick();
  if (props.inlineEditor !== editor || props.wasmEngine !== engine || !editorHost.value) return;
  const request = {
    host: editorHost.value,
    hash: editor.planHash,
    page: editor.page,
    text: editor.value,
    mode: editor.mode,
    binding: editor.binding,
    vectorOnly: true,
    onApplied: (result) => emit('editor-applied', {transaction: editor.transaction, result}),
    onCancel: () => emit('cancel-inline'),
  };
  if (editor.mode === 'data') request.jsonPointer = editor.pointer;
  else if (Number.isInteger(editor.sourceOffset)) request.sourceOffset = editor.sourceOffset;
  else request.target = editor.target;
  const controller = engine.mountEditor(request);
  if (controller instanceof Error) {
    emit('editor-error', {transaction: editor.transaction, error: controller});
    return;
  }
  wasmEditorController = controller;
}

function destroyWASMEditor() {
  wasmEditorController?.destroy?.();
  wasmEditorController = undefined;
}

async function materializeDocumentText() {
  const token = ++measureToken;
  displayTextRuns.value = [];
  await installDocumentFonts(props.fonts);
  await nextTick();
  if (token !== measureToken || !pageStage.value || !(props.pageWidth > 0)) return;
  const stageBounds = pageStage.value.getBoundingClientRect();
  if (!(stageBounds.width > 0 && stageBounds.height > 0)) return;
  const scale = stageBounds.width / props.pageWidth;
  const context = document.createElement('canvas').getContext('2d');
  displayTextRuns.value = props.textRuns.map((run, index) => materializeTextRun(run, index, scale, context));
}

async function installDocumentFonts(fonts) {
  if (typeof FontFace !== 'function' || !document.fonts) return;
  await Promise.all(fonts.map(async (font) => {
    if (!font?.family || !font?.data || loadedFontFamilies.has(font.family)) return;
    const binary = atob(font.data);
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
    const loaded = await new FontFace(font.family, bytes.buffer).load();
    document.fonts.add(loaded);
    loadedFontFamilies.add(font.family);
  }));
}

function materializeTextRun(run, index, scale, context) {
  const fontSize = Number(run.font_size_fixed || 0) * scale;
  const family = run.font_family || 'sans-serif';
  const weight = run.font_weight || '400';
  const fontStyle = run.font_style || 'normal';
  context.font = `${fontStyle} ${weight} ${fontSize}px ${family}`;
  const metrics = context.measureText(run.text || '');
  const ascent = metrics.actualBoundingBoxAscent || fontSize * .8;
  const descent = metrics.actualBoundingBoxDescent || fontSize * .2;
  const naturalWidth = metrics.width || 1;
  const targetWidth = Number(run.width_fixed || 0) * scale;
  return {
    index,
    text: run.text || '',
    style: {
      left: `${(Number(run.x_fixed || 0) - props.pageX) * scale}px`,
      top: `${(Number(run.baseline_fixed || 0) - props.pageY) * scale - ascent}px`,
      height: `${Math.max(1, ascent + descent)}px`,
      color: run.color || '#000000',
      '--selection-color': run.color || '#000000',
      opacity: String(Number(run.opacity_fixed || props.fixedScale) / Math.max(1, props.fixedScale)),
      fontFamily: family,
      fontWeight: weight,
      fontStyle,
      fontSize: `${fontSize}px`,
      transform: `scaleX(${targetWidth > 0 ? targetWidth / naturalWidth : 1})`,
    },
  };
}

async function refreshRenderedPage() {
  await nextTick();
  await Promise.all([materializeDocumentText(), paintWASMPage()]);
}

async function paintWASMPage() {
  await nextTick();
  const engine = props.wasmEngine;
  const canvas = graphicsCanvas.value;
  if (!engine?.paintPage || !canvas || !props.planHash || !(props.pageWidth > 0) || !(props.pageHeight > 0)) return;
  const result = engine.paintPage({
    canvas,
    hash: props.planHash,
    page: props.page,
  });
  if (result instanceof Error) emit('render-error', result);
}

function pointFromEvent(event) {
  const bounds = pageStage.value?.getBoundingClientRect();
  if (!bounds || !(bounds.width > 0) || !(bounds.height > 0) || !(props.pageWidth > 0) || !(props.pageHeight > 0)) return null;
  const x = Math.max(0, Math.min(bounds.width, event.clientX - bounds.left));
  const y = Math.max(0, Math.min(bounds.height, event.clientY - bounds.top));
  return {
    xFixed: props.pageX + Math.round(x / bounds.width * props.pageWidth),
    yFixed: props.pageY + Math.round(y / bounds.height * props.pageHeight),
  };
}

function selectAt(event) {
  if (props.stale) return;
  if (globalThis.getSelection?.()?.isCollapsed === false) return;
  const point = pointFromEvent(event);
  if (point) emit('page-point', point);
}

function editAt(event) {
  if (props.stale) return;
  event.preventDefault();
  globalThis.getSelection?.()?.removeAllRanges();
  const point = pointFromEvent(event);
  if (point) emit('edit-point', point);
}

function overlayStyle(bounds) {
  if (!bounds) return {};
  return {
    left: `${bounds.left}%`,
    top: `${bounds.top}%`,
    width: `${bounds.width}%`,
    height: `${bounds.height}%`,
  };
}

function movableSelectionStyle(bounds) {
  const style = overlayStyle(bounds);
  if (dragState.value) style.transform = `translate(${dragState.value.dx}px, ${dragState.value.dy}px)`;
  return style;
}

function beginSelectionMove(event) {
  if (props.stale || !props.selection?.node?.id || props.inlineEditor) return;
  event.preventDefault();
  event.stopPropagation();
  event.currentTarget.setPointerCapture?.(event.pointerId);
  dragState.value = {pointerId: event.pointerId, x: event.clientX, y: event.clientY, dx: 0, dy: 0};
}

function updateSelectionMove(event) {
  const drag = dragState.value;
  if (!drag || drag.pointerId !== event.pointerId) return;
  drag.dx = event.clientX - drag.x;
  drag.dy = event.clientY - drag.y;
  dragState.value = {...drag};
}

function finishSelectionMove(event) {
  const drag = dragState.value;
  if (!drag || drag.pointerId !== event.pointerId) return;
  event.preventDefault();
  event.stopPropagation();
  dragState.value = null;
  const bounds = pageStage.value?.getBoundingClientRect();
  if (!bounds || Math.hypot(drag.dx, drag.dy) < 3) return;
  emit('move-selection', {
    target: props.selection.node.id,
    xFixed: Math.round(drag.dx / bounds.width * props.pageWidth),
    yFixed: Math.round(drag.dy / bounds.height * props.pageHeight),
  });
}

function formatFixedPoints(value) {
  const points = Number(value) / Math.max(1, props.fixedScale);
  return Number.isInteger(points) ? String(points) : points.toFixed(1);
}

</script>

<template>
  <section class="studio-canvas" aria-label="Editable document canvas">
    <div class="canvas-scroll">
      <div
        v-if="pageWidth > 0 && pageHeight > 0"
        ref="pageStage"
        class="page-stage"
        :class="{'is-stale': stale}"
        :style="pageStyle"
        tabindex="0"
        aria-label="Rendered document page. Drag to select text or double-click text to edit."
        @click="selectAt"
        @dblclick="editAt"
      >
        <canvas ref="graphicsCanvas" class="document-graphics" aria-hidden="true"></canvas>
        <div class="document-text-plane" aria-label="Selectable document text">
          <span
            v-for="run in displayTextRuns"
            :key="`${run.index}-${run.text}`"
            class="document-text-run"
            :style="run.style"
          >{{ run.text }}</span>
        </div>
        <div v-if="selection?.bounds" class="block-selection" :class="{'is-moving': dragState}" :style="movableSelectionStyle(selection.bounds)">
          <span>{{ selection.id }}</span>
          <button
            v-if="selection.node?.id"
            type="button"
            class="selection-move-handle"
            title="Drag to move this block"
            aria-label="Drag to move selected block"
            @pointerdown="beginSelectionMove"
            @pointermove="updateSelectionMove"
            @pointerup="finishSelectionMove"
            @pointercancel="dragState = null"
          ><i></i><i></i><i></i><i></i><i></i><i></i></button>
        </div>
        <div
          v-if="inlineEditor"
          ref="editorHost"
          class="inline-editor-host"
          :style="overlayStyle(inlineEditor.bounds)"
        ></div>
        <div v-if="overflowText" class="overflow-warning" role="alert">
          <strong>Layout overflow</strong>
          <span>{{ overflowText }}</span>
        </div>
        <div v-if="stale" class="stale-veil" aria-live="polite">
          <span>Last valid render</span>
        </div>
      </div>

      <div v-else class="canvas-empty" :class="state">
        <span class="empty-signal" aria-hidden="true"></span>
        <strong>{{ state === 'error' || state === 'offline' ? 'Document unavailable' : 'Preparing exact page' }}</strong>
        <p>{{ failure || status || 'Loading the PaperRune compiler…' }}</p>
        <div v-if="loadProgress > 0 && loadProgress < 1" class="load-progress" aria-hidden="true">
          <i :style="{width: `${Math.round(loadProgress * 100)}%`}"></i>
        </div>
        <button v-if="state === 'error' || state === 'offline'" type="button" @click="$emit('retry')">Retry compiler</button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.studio-canvas { min-width: 0; min-height: 0; background: #cbc8c0; }
.canvas-scroll { width: 100%; height: 100%; min-height: 0; overflow: auto; padding: clamp(22px, 4vw, 54px); }
.page-stage {
  position: relative;
  width: min(100%, 760px);
  margin: 0 auto;
  background: white;
  outline: none;
  box-shadow: 0 25px 70px rgba(30, 32, 36, .2), 0 2px 8px rgba(30, 32, 36, .12);
  transition: filter .18s ease, opacity .18s ease, transform .18s ease;
}
.page-stage:focus-visible { box-shadow: 0 25px 70px rgba(30, 32, 36, .2), 0 0 0 3px rgba(46, 91, 214, .28); }
.document-graphics {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: block;
  width: 100%;
  height: 100%;
  pointer-events: none;
  transition: filter .18s ease, opacity .18s ease;
}
.page-stage.is-stale .document-graphics { filter: saturate(.72); opacity: .68; }
.document-text-plane { position: absolute; inset: 0; z-index: 2; pointer-events: none; user-select: text; }
.document-text-run {
  position: absolute;
  display: inline-block;
  line-height: 1;
  white-space: pre;
  transform-origin: left top;
  pointer-events: auto;
  user-select: text;
}
.document-text-run::selection {
  background: rgba(46, 91, 214, .28);
  color: var(--selection-color);
  -webkit-text-fill-color: var(--selection-color);
  text-shadow: none;
}
.block-selection {
  position: absolute;
  z-index: 4;
  border: 1.5px solid #2e5bd6;
  background: rgba(46, 91, 214, .035);
  pointer-events: none;
  animation: selection-in .14s ease-out;
  transition: transform .08s linear;
}
.block-selection.is-moving { border-style: dashed; background: rgba(46, 91, 214, .08); transition: none; }
.block-selection span {
  position: absolute;
  left: -1px;
  bottom: 100%;
  padding: 3px 6px;
  background: #2e5bd6;
  color: white;
  font: 600 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.selection-move-handle {
  position: absolute;
  left: 50%;
  top: -14px;
  display: grid;
  grid-template-columns: repeat(3, 2px);
  gap: 2px;
  width: 22px;
  height: 14px;
  margin-left: -11px;
  border: 0;
  border-radius: 3px 3px 0 0;
  padding: 4px 6px;
  background: #2e5bd6;
  cursor: grab;
  pointer-events: auto;
  touch-action: none;
}
.selection-move-handle:active { cursor: grabbing; }
.selection-move-handle i { width: 2px; height: 2px; border-radius: 50%; background: white; }
.inline-editor-host {
  position: absolute;
  z-index: 8;
  min-width: 1px;
  min-height: 1px;
}
.inline-editor-host :deep(.wasm-direct-editor) {
  box-sizing: border-box;
  width: 100%;
  min-height: 100%;
  overflow: visible;
  border: 0;
  outline: 0;
  padding: 0;
  background: transparent;
  color: inherit;
  line-height: normal;
  white-space: pre-wrap;
  word-break: break-word;
  caret-color: #2e5bd6;
  box-shadow: inset 0 -1px #2e5bd6;
}
.inline-editor-host.is-busy { opacity: .72; pointer-events: none; }
.inline-editor-host :deep(.wasm-direct-editor-error) { display: none; }
.inline-editor-host.has-error :deep(.wasm-direct-editor) { box-shadow: inset 0 -1px #b3312b; }
.inline-editor-host.has-error :deep(.wasm-direct-editor-error) {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  display: block;
  width: max(220px, 100%);
  padding: 5px 7px;
  background: #fff1ef;
  color: #9b2d27;
  font: 10px/1.35 ui-monospace, SFMono-Regular, Menlo, monospace;
  box-shadow: 0 3px 10px rgba(65, 24, 20, .14);
}
.overflow-warning {
  position: absolute;
  z-index: 9;
  top: 10px;
  right: 10px;
  display: grid;
  gap: 2px;
  max-width: min(300px, calc(100% - 20px));
  padding: 8px 10px;
  border: 1px solid #c47a16;
  background: rgba(255, 247, 226, .97);
  color: #71420b;
  box-shadow: 0 5px 16px rgba(83, 52, 14, .16);
  pointer-events: none;
}
.overflow-warning strong { font: 700 10px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; }
.overflow-warning span { font: 11px/1.35 ui-sans-serif, system-ui, sans-serif; }
.stale-veil { position: absolute; inset: 0; z-index: 7; display: grid; place-items: start end; padding: 10px; cursor: wait; pointer-events: auto; }
.stale-veil span { padding: 5px 8px; background: rgba(28, 31, 37, .82); color: white; font: 600 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; letter-spacing: .07em; }
.canvas-empty { display: grid; place-items: center; align-content: center; width: min(100%, 760px); min-height: min(72vh, 840px); margin: 0 auto; border: 1px solid rgba(37, 40, 45, .14); background: rgba(246, 244, 239, .72); color: #66676a; text-align: center; }
.canvas-empty strong { margin-top: 14px; color: #282b30; font-size: 15px; }
.canvas-empty p { max-width: 430px; margin: 7px 28px 0; font-size: 12px; line-height: 1.5; }
.empty-signal { width: 11px; height: 11px; border-radius: 50%; background: #2e5bd6; box-shadow: 0 0 0 7px rgba(46, 91, 214, .1); animation: pulse 1s ease-in-out infinite alternate; }
.canvas-empty.error .empty-signal, .canvas-empty.offline .empty-signal { background: #b23b32; box-shadow: 0 0 0 7px rgba(178, 59, 50, .1); animation: none; }
.canvas-empty button { margin-top: 16px; border: 0; border-radius: 3px; padding: 8px 12px; background: #20242a; color: white; cursor: pointer; }
.load-progress { width: min(280px, 60%); height: 3px; margin-top: 16px; overflow: hidden; background: #d8d5ce; }
.load-progress i { display: block; height: 100%; background: #2e5bd6; transition: width .16s ease; }
@keyframes pulse { to { transform: scale(1.35); opacity: .55; } }
@keyframes selection-in { from { opacity: .25; transform: scale(.985); } }
@media (prefers-reduced-motion: reduce) {
  .page-stage, .empty-signal, .block-selection { animation: none; transition: none; }
}
</style>
