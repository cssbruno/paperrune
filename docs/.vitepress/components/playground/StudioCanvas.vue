<script setup>
import {nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue';

const props = defineProps({
  image: {type: String, default: ''},
  svg: {type: String, default: ''},
  pageX: {type: Number, default: 0},
  pageY: {type: Number, default: 0},
  pageWidth: {type: Number, default: 0},
  pageHeight: {type: Number, default: 0},
  state: {type: String, default: 'loading'},
  status: {type: String, default: ''},
  failure: {type: String, default: ''},
  stale: {type: Boolean, default: false},
  loadProgress: {type: Number, default: 0},
  selection: {type: Object, default: null},
  inlineEditor: {type: Object, default: null},
});

const emit = defineEmits(['page-point', 'edit-point', 'commit-inline', 'cancel-inline', 'retry']);
const pageStage = ref(null);
const selectableSVG = ref('');
const selectableLines = ref([]);
const inlineDraft = ref('');
const inlineInput = ref(null);
let resizeObserver;
let measureToken = 0;

watch(() => props.svg, (svg) => {
  selectableLines.value = [];
  selectableSVG.value = sanitizeTextSVG(svg);
  void materializeSelectableText();
}, {immediate: true});

watch(() => props.inlineEditor, (editor) => {
  inlineDraft.value = editor?.value ?? '';
  if (editor) void nextTick(() => {
    inlineInput.value?.focus({preventScroll: true});
    inlineInput.value?.select();
  });
}, {immediate: true});

onMounted(() => {
  if (typeof ResizeObserver === 'function') {
    resizeObserver = new ResizeObserver(() => void materializeSelectableText());
    if (pageStage.value) resizeObserver.observe(pageStage.value);
  }
  void materializeSelectableText();
});

onBeforeUnmount(() => resizeObserver?.disconnect());

async function materializeSelectableText() {
  const token = ++measureToken;
  selectableLines.value = [];
  await nextTick();
  if (token !== measureToken || !pageStage.value || !selectableSVG.value) return;
  const stageBounds = pageStage.value.getBoundingClientRect();
  if (!(stageBounds.width > 0 && stageBounds.height > 0)) return;
  const textNodes = [...pageStage.value.querySelectorAll('.selectable-svg text')];
  const grouped = [];
  let line = null;
  for (const text of textNodes) {
    const rect = text.getBoundingClientRect();
    const usable = rect.width > 0 && rect.height > 0;
    const family = text.getAttribute('font-family') || 'serif';
    const top = usable ? rect.top - stageBounds.top : line?.top ?? 0;
    const left = usable ? rect.left - stageBounds.left : line?.right ?? 0;
    const height = usable ? rect.height : line?.height ?? 1;
    const separated = line && usable && (
      Math.abs(top - line.top) > 1.25 ||
      family !== line.family ||
      left < line.lastLeft ||
      left - line.right > height * 4
    );
    if (!line || separated) {
      line = {text: '', left, top, right: left, lastLeft: left, height, family, spacing: 0};
      grouped.push(line);
    }
    line.text += text.textContent || '';
    if (usable) {
      line.right = rect.right - stageBounds.left;
      line.lastLeft = left;
      line.height = Math.max(line.height, height);
    }
  }
  selectableLines.value = grouped.filter((item) => item.text);
  await nextTick();
  if (token !== measureToken) return;
  const spans = [...pageStage.value.querySelectorAll('.selectable-line')];
  selectableLines.value = selectableLines.value.map((item, index) => {
    const naturalWidth = spans[index]?.getBoundingClientRect().width || 0;
    const targetWidth = Math.max(0, item.right - item.left);
    const spacing = item.text.length > 1 && naturalWidth > 0 ? (targetWidth - naturalWidth) / (item.text.length - 1) : 0;
    return {...item, spacing};
  });
}

function sanitizeTextSVG(svgText) {
  if (!svgText || typeof DOMParser === 'undefined' || typeof XMLSerializer === 'undefined') return '';
  const parsed = new DOMParser().parseFromString(svgText, 'image/svg+xml');
  if (parsed.querySelector('parsererror')) return '';
  parsed.querySelectorAll('script, foreignObject, image, a, use').forEach((node) => node.remove());
  parsed.querySelectorAll('rect, path, circle, ellipse, polygon, polyline, line').forEach((node) => {
    if (!node.closest('clipPath')) node.remove();
  });
  const root = parsed.documentElement;
  root.setAttribute('aria-hidden', 'true');
  root.setAttribute('focusable', 'false');
  return new XMLSerializer().serializeToString(root);
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

function lineStyle(line) {
  return {
    left: `${line.left}px`,
    top: `${line.top}px`,
    fontFamily: line.family,
    fontSize: `${Math.max(1, line.height)}px`,
    letterSpacing: `${line.spacing}px`,
  };
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

function handleInlineKeydown(event) {
  if (event.key === 'Escape') {
    event.preventDefault();
    emit('cancel-inline');
  } else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault();
    emit('commit-inline', inlineDraft.value);
  }
}
</script>

<template>
  <section class="studio-canvas" aria-label="Editable document canvas">
    <div class="canvas-scroll">
      <div
        v-if="image"
        ref="pageStage"
        class="page-stage"
        :class="{'is-stale': stale}"
        tabindex="0"
        aria-label="Rendered document page. Drag to select text or double-click text to edit."
        @click="selectAt"
        @dblclick="editAt"
      >
        <img :src="image" alt="Exact PaperRune page rendered by WebAssembly">
        <div class="selectable-svg" v-html="selectableSVG"></div>
        <div class="selectable-plane" aria-label="Selectable document text">
          <span
            v-for="(line, index) in selectableLines"
            :key="`${index}-${line.text}`"
            class="selectable-line"
            :style="lineStyle(line)"
          >{{ line.text }}</span>
        </div>
        <div v-if="selection?.bounds" class="block-selection" :style="overlayStyle(selection.bounds)">
          <span>{{ selection.id }}</span>
        </div>
        <form
          v-if="inlineEditor"
          class="inline-editor"
          :class="{'is-busy': inlineEditor.busy}"
          :style="overlayStyle(inlineEditor.bounds)"
          @submit.prevent="$emit('commit-inline', inlineDraft)"
        >
          <textarea
            ref="inlineInput"
            v-model="inlineDraft"
            :disabled="inlineEditor.busy"
            rows="2"
            spellcheck="true"
            aria-label="Edit selected document text"
            @keydown="handleInlineKeydown"
          ></textarea>
          <div class="inline-actions">
            <span>{{ inlineEditor.error || (inlineEditor.mode === 'data' ? `JSON · ${inlineEditor.binding}` : 'Paper source') }}</span>
            <button type="button" :disabled="inlineEditor.busy" @click="$emit('cancel-inline')">Cancel</button>
            <button type="submit" :disabled="inlineEditor.busy">{{ inlineEditor.busy ? 'Applying…' : 'Apply' }}</button>
          </div>
        </form>
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
.page-stage > img { position: relative; z-index: 1; display: block; width: 100%; height: auto; user-select: none; -webkit-user-drag: none; }
.page-stage.is-stale > img { filter: saturate(.72); opacity: .68; }
.selectable-svg {
  position: absolute;
  inset: 0;
  z-index: 2;
  visibility: hidden;
  overflow: hidden;
  pointer-events: none;
}
.selectable-svg :deep(svg) { display: block; width: 100%; height: 100%; }
.selectable-plane { position: absolute; inset: 0; z-index: 3; pointer-events: none; user-select: text; }
.selectable-line {
  position: absolute;
  display: inline-block;
  color: transparent;
  line-height: 1;
  white-space: pre;
  pointer-events: auto;
  user-select: text;
}
.selectable-plane ::selection { background: rgba(46, 91, 214, .34); color: transparent; }
.block-selection {
  position: absolute;
  z-index: 4;
  border: 1.5px solid #2e5bd6;
  background: rgba(46, 91, 214, .035);
  pointer-events: none;
  animation: selection-in .14s ease-out;
}
.block-selection span {
  position: absolute;
  left: -1px;
  bottom: 100%;
  padding: 3px 6px;
  background: #2e5bd6;
  color: white;
  font: 600 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.inline-editor {
  position: absolute;
  z-index: 8;
  display: grid;
  min-width: min(250px, 72%);
  min-height: 72px;
  overflow: visible;
  border: 1px solid #2e5bd6;
  border-radius: 3px;
  background: rgba(255,255,255,.98);
  box-shadow: 0 16px 38px rgba(18, 29, 52, .24), 0 0 0 3px rgba(46, 91, 214, .13);
}
.inline-editor textarea {
  width: 100%;
  min-height: 54px;
  resize: vertical;
  border: 0;
  outline: 0;
  padding: 9px 10px;
  background: transparent;
  color: #20242a;
  font: 13px/1.45 ui-sans-serif, system-ui, sans-serif;
}
.inline-editor.is-busy { opacity: .72; pointer-events: none; }
.inline-actions { display: flex; align-items: center; gap: 7px; min-height: 32px; padding: 4px 5px 4px 9px; border-top: 1px solid #ddd9d0; background: #f6f4ef; }
.inline-actions span { min-width: 0; margin-right: auto; overflow: hidden; color: #6c6d70; font: 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.inline-actions button { min-height: 24px; padding: 3px 8px; border: 1px solid #cbc8c0; border-radius: 3px; background: white; color: #25282d; font-size: 10px; cursor: pointer; }
.inline-actions button[type="submit"] { border-color: #2e5bd6; background: #2e5bd6; color: white; }
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
