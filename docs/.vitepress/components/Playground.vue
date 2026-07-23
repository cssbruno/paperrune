<script setup>
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue';
import {withBase} from 'vitepress';
import {playgroundSampleManifest} from '../playground-samples.mjs';
import StudioCanvas from './playground/StudioCanvas.vue';
import {
  boxAsPercent,
  normalizeFailure,
  pickHitTarget,
  traceBindingDescriptor,
} from './playground/studio-model.mjs';
import {
  createPlaygroundRuntimeLoader,
  formatPlaygroundRuntimeStatus,
} from './playground/wasm-runtime.mjs';

const sampleSources = import.meta.glob('../playground-samples/*.paper', {eager: true, query: '?raw', import: 'default'});
const sampleData = import.meta.glob('../playground-samples/*.json', {eager: true, query: '?raw', import: 'default'});
const samples = playgroundSampleManifest.map((sample) => {
  const source = sampleSources[`../playground-samples/${sample.slug}.paper`];
  const data = sampleData[`../playground-samples/${sample.slug}.json`];
  if (typeof source !== 'string' || typeof data !== 'string') throw new Error(`Playground sample files are missing for ${sample.slug}`);
  return Object.freeze({...sample, source, data});
});
const sampleGroups = Object.freeze([...new Set(samples.map((sample) => sample.category))].map((category) => Object.freeze({
  category,
  samples: samples.map((sample, index) => ({sample, index})).filter(({sample}) => sample.category === category),
})));

const selectedSample = ref(0);
const source = ref(samples[0].source);
const data = ref(samples[0].data);
const activePanel = ref('source');
const state = ref('loading');
const status = ref('Loading compiler…');
const failure = ref('');
const diagnostics = ref([]);
const png = ref('');
const svg = ref('');
const ast = ref(null);
const pages = ref(0);
const page = ref(1);
const planHash = ref('');
const sourceRevision = ref('');
const pageX = ref(0);
const pageY = ref(0);
const pageWidth = ref(0);
const pageHeight = ref(0);
const previewStale = ref(false);
const selected = ref(null);
const inlineEditor = ref(null);
const online = ref(true);
const compileSlow = ref(false);
const runtimeSnapshot = ref({
  state: 'idle',
  phase: 'idle',
  slow: false,
  loaded: 0,
  total: 0,
  progress: 0,
  attempt: 0,
  error: null,
});

let runtimeLoader;
let unsubscribeRuntime;
let compileSequence = 0;
let inlineEditSequence = 0;
let debounceTimer;
let compileSlowTimer;
let suppressLiveCompile = false;

const liveCompileDelay = 360;
const sourceLines = computed(() => source.value.split('\n').length);
const documentImage = computed(() => png.value ? `data:image/png;base64,${png.value}` : '');
const loadProgress = computed(() => {
  const snapshot = runtimeSnapshot.value;
  if (Number.isFinite(snapshot.progress) && snapshot.progress > 0) return Math.min(1, snapshot.progress);
  if (snapshot.total > 0) return Math.min(1, snapshot.loaded / snapshot.total);
  return 0;
});
const statusClass = computed(() => {
  if (!online.value && !runtimeSnapshot.value.loaded) return 'offline';
  if (compileSlow.value || runtimeSnapshot.value.slow) return 'slow';
  return state.value;
});
const statusDetail = computed(() => {
  if (!online.value && runtimeSnapshot.value.state === 'ready') return 'Offline · compiler ready locally';
  return status.value;
});
const canRetry = computed(() => Boolean(failure.value) || state.value === 'offline' || runtimeSnapshot.value.state === 'error');
const selectedKind = computed(() => selected.value?.node?.kind || 'Nothing selected');

onMounted(() => {
  online.value = navigator.onLine;
  runtimeLoader = createPlaygroundRuntimeLoader({
    runtimeUrl: withBase('/wasm_exec.js'),
    wasmUrl: withBase('/paperrune.wasm'),
    slowAfterMs: 1800,
    timeoutMs: 120000,
    initializationTimeoutMs: 15000,
  });
  unsubscribeRuntime = runtimeLoader.subscribe((snapshot) => {
    runtimeSnapshot.value = snapshot;
    if (['loading', 'compiling'].includes(state.value) && snapshot.state !== 'ready' && snapshot.state !== 'error') {
      status.value = formatPlaygroundRuntimeStatus(snapshot);
    }
  }, {immediate: true});
  globalThis.addEventListener('online', handleOnline);
  globalThis.addEventListener('offline', handleOffline);
  void compile(1);
});

onBeforeUnmount(() => {
  clearTimeout(debounceTimer);
  clearTimeout(compileSlowTimer);
  unsubscribeRuntime?.();
  globalThis.removeEventListener('online', handleOnline);
  globalThis.removeEventListener('offline', handleOffline);
});

async function compile(targetPage = page.value, {retryRuntime = false} = {}) {
  clearTimeout(debounceTimer);
  clearTimeout(compileSlowTimer);
  const sequence = ++compileSequence;
  previewStale.value = Boolean(png.value);
  compileSlow.value = false;
  state.value = runtimeSnapshot.value.state === 'ready' ? 'compiling' : 'loading';
  status.value = state.value === 'loading' ? 'Loading compiler…' : 'Compiling exact document plan…';
  compileSlowTimer = setTimeout(() => {
    if (sequence !== compileSequence) return;
    compileSlow.value = true;
    status.value = state.value === 'loading'
      ? formatPlaygroundRuntimeStatus({...runtimeSnapshot.value, slow: true})
      : 'Still compiling · large documents can take a moment';
  }, 1800);
  try {
    if (!runtimeLoader) throw new Error('Compiler loader is not ready');
    if (!online.value && runtimeSnapshot.value.state !== 'ready' && !globalThis.PaperStudioWASM?.compile) {
      const offlineError = new Error('You are offline and the compiler has not finished loading.');
      offlineError.kind = 'offline';
      throw offlineError;
    }
    const engine = retryRuntime ? await runtimeLoader.retry() : await runtimeLoader.load();
    if (sequence !== compileSequence) return;
    state.value = 'compiling';
    status.value = 'Compiling exact document plan…';
    const result = await engine.compile({
      source: source.value,
      data: data.value,
      page: targetPage,
      dataName: 'playground',
    });
    if (sequence !== compileSequence) return;
    diagnostics.value = result.diagnostics || [];
    pages.value = result.pages || 0;
    planHash.value = result.hash || '';
    sourceRevision.value = result.source_revision || '';
    if (!result.ok || !result.png) {
      failure.value = diagnostics.value.length
        ? ''
        : normalizeFailure(result.error, result.ok ? 'The compiler returned no document.' : 'Compilation failed.');
      state.value = 'error';
      status.value = diagnostics.value.length
        ? `${diagnostics.value.length} diagnostic${diagnostics.value.length === 1 ? '' : 's'}`
        : failure.value;
      return;
    }
    page.value = result.page;
    png.value = result.png;
    svg.value = result.svg || '';
    ast.value = result.ast || null;
    pageX.value = Number(result.page_x_fixed || 0);
    pageY.value = Number(result.page_y_fixed || 0);
    pageWidth.value = Number(result.page_width_fixed || 0);
    pageHeight.value = Number(result.page_height_fixed || 0);
    previewStale.value = false;
    failure.value = '';
    selected.value = null;
    inlineEditor.value = null;
    state.value = diagnostics.value.length ? 'warning' : 'ready';
    status.value = `${result.pages} page${result.pages === 1 ? '' : 's'} · plan ${result.hash.slice(0, 10)}`;
  } catch (error) {
    if (sequence !== compileSequence) return;
    diagnostics.value = [];
    failure.value = normalizeFailure(error, 'The compiler failed unexpectedly.');
    state.value = !online.value || error?.kind === 'offline' ? 'offline' : 'error';
    status.value = failure.value;
  } finally {
    if (sequence === compileSequence) {
      clearTimeout(compileSlowTimer);
      compileSlow.value = false;
    }
  }
}

function scheduleCompile() {
  if (suppressLiveCompile) return;
  inlineEditSequence += 1;
  clearTimeout(debounceTimer);
  clearTimeout(compileSlowTimer);
  compileSequence += 1;
  previewStale.value = Boolean(png.value);
  selected.value = null;
  inlineEditor.value = null;
  state.value = 'editing';
  status.value = 'Live changes queued…';
  debounceTimer = setTimeout(() => void compile(1), liveCompileDelay);
}

function chooseSample(event) {
  const index = Number(event.target.value);
  if (!Number.isInteger(index) || !samples[index]) return;
  selectedSample.value = index;
  suppressLiveCompile = true;
  source.value = samples[index].source;
  data.value = samples[index].data;
  suppressLiveCompile = false;
  page.value = 1;
  png.value = '';
  svg.value = '';
  ast.value = null;
  selected.value = null;
  inlineEditor.value = null;
  void compile(1);
}

async function selectPagePoint(point, openEditor = false) {
  if (!planHash.value || !runtimeLoader || previewStale.value) return;
  const snapshot = {
    sequence: compileSequence,
    hash: planHash.value,
    page: page.value,
    astRoot: ast.value?.root,
    data: data.value,
    source: source.value,
    sourceRevision: sourceRevision.value,
    pageX: pageX.value,
    pageY: pageY.value,
    pageWidth: pageWidth.value,
    pageHeight: pageHeight.value,
  };
  try {
    const engine = await runtimeLoader.load();
    if (!selectionSnapshotCurrent(snapshot)) return;
    const hit = await engine.hit({
      hash: snapshot.hash,
      page: snapshot.page,
      x_fixed: point.xFixed,
      y_fixed: point.yFixed,
    });
    if (!selectionSnapshotCurrent(snapshot)) return;
    const target = pickHitTarget(hit, snapshot.astRoot, snapshot.data);
    if (!target) {
      selected.value = null;
      inlineEditor.value = null;
      return;
    }
    if (target.content.binding && !target.content.editable && !target.content.computed && engine.trace) {
      const fragment = Number(target.fragment.Fragment ?? target.fragment.fragment);
      if (Number.isInteger(fragment) && fragment > 0) {
        const trace = await engine.trace({hash: snapshot.hash, fragment});
        if (!selectionSnapshotCurrent(snapshot)) return;
        target.content = traceBindingDescriptor(trace, snapshot.data);
      }
    }
    const bounds = boxAsPercent(
      target.fragment.ContentBox || target.fragment.BorderBox || target.fragment.content_box || target.fragment.border_box,
      snapshot.pageWidth,
      snapshot.pageHeight,
      snapshot.pageX,
      snapshot.pageY,
    );
    selected.value = {
      ...target,
      id: target.id || String(target.fragment.Key || target.fragment.key || `${target.node.kind} @ line ${target.node.header_span?.start?.line || '?'}`),
      bounds,
      snapshot,
    };
    activePanel.value = 'inspect';
    if (openEditor) requestInlineEdit();
  } catch (error) {
    if (!selectionSnapshotCurrent(snapshot)) return;
    failure.value = normalizeFailure(error, 'The page selection could not be resolved.');
    status.value = failure.value;
  }
}

function selectionSnapshotCurrent(snapshot) {
  return !previewStale.value &&
    snapshot.sequence === compileSequence &&
    snapshot.hash === planHash.value &&
    snapshot.page === page.value &&
    snapshot.astRoot === ast.value?.root &&
    snapshot.data === data.value &&
    snapshot.source === source.value &&
    snapshot.sourceRevision === sourceRevision.value;
}

function requestInlineEdit() {
  const selection = selected.value;
  if (!selection?.content?.editable || !selection.bounds || !selectionSnapshotCurrent(selection.snapshot)) return;
  const bounds = {
    left: Math.max(1, Math.min(76, selection.bounds.left)),
    top: Math.max(1, Math.min(91, selection.bounds.top)),
    width: Math.max(24, Math.min(98 - selection.bounds.left, selection.bounds.width)),
    height: Math.max(5, selection.bounds.height),
  };
  const transaction = ++inlineEditSequence;
  inlineEditor.value = {
    transaction,
    target: selection.content.target || selection.id,
    sourceOffset: selection.content.sourceOffset,
    mode: selection.content.mode,
    binding: selection.content.binding || '',
    pointer: selection.content.pointer || '',
    value: selection.content.value,
    bounds,
    busy: false,
    error: '',
    source: selection.snapshot.source,
    data: selection.snapshot.data,
    sourceRevision: selection.snapshot.sourceRevision,
    planHash: selection.snapshot.hash,
    compileSequence: selection.snapshot.sequence,
    page: selection.snapshot.page,
  };
}

async function commitInlineEdit(nextText) {
  const editor = inlineEditor.value;
  if (!editor || editor.busy || !runtimeLoader) return;
  const transaction = editor.transaction;
  inlineEditor.value = {...editor, busy: true, error: ''};
  try {
    const engine = await runtimeLoader.load();
    if (!inlineTransactionCurrent(editor, transaction)) return;
    if (editor.mode === 'data') {
      if (!editor.pointer) throw new Error('The selected binding has no concrete JSON pointer.');
      const result = await engine.editText({
        data: editor.data,
        jsonPointer: editor.pointer,
        text: nextText,
      });
      if (!inlineTransactionCurrent(editor, transaction)) return;
      suppressLiveCompile = true;
      data.value = result.data;
      suppressLiveCompile = false;
    } else {
      const request = {
        source: editor.source,
        sourceRevision: editor.sourceRevision,
        text: nextText,
      };
      if (Number.isInteger(editor.sourceOffset)) request.sourceOffset = editor.sourceOffset;
      else request.target = editor.target;
      const result = await engine.editText({
        ...request,
      });
      if (!inlineTransactionCurrent(editor, transaction)) return;
      suppressLiveCompile = true;
      source.value = result.source;
      suppressLiveCompile = false;
    }
    if (inlineEditSequence !== transaction) return;
    inlineEditor.value = null;
    await compile(editor.page);
  } catch (error) {
    suppressLiveCompile = false;
    if (inlineEditSequence !== transaction || inlineEditor.value?.transaction !== transaction) return;
    inlineEditor.value = {
      ...inlineEditor.value,
      busy: false,
      error: normalizeFailure(error, 'The edit could not be applied.'),
    };
  }
}

function inlineTransactionCurrent(editor, transaction) {
  return inlineEditSequence === transaction &&
    inlineEditor.value?.transaction === transaction &&
    editor.compileSequence === compileSequence &&
    editor.planHash === planHash.value &&
    editor.source === source.value &&
    editor.data === data.value &&
    editor.sourceRevision === sourceRevision.value &&
    !previewStale.value;
}

async function retryCompiler() {
  failure.value = '';
  diagnostics.value = [];
  if (!navigator.onLine && runtimeSnapshot.value.state !== 'ready') {
    online.value = false;
    state.value = 'offline';
    failure.value = 'You are offline. Reconnect and the compiler will retry automatically.';
    status.value = failure.value;
    return;
  }
  online.value = true;
  await compile(page.value || 1, {retryRuntime: true});
}

function handleOffline() {
  online.value = false;
  if (runtimeSnapshot.value.state !== 'ready') {
    state.value = 'offline';
    failure.value = 'Connection lost while loading the compiler. Your source and JSON are safe.';
    status.value = failure.value;
  } else {
    status.value = 'Offline · compiler ready locally';
  }
}

function handleOnline() {
  const shouldRetry = state.value === 'offline' || runtimeSnapshot.value.state === 'error';
  online.value = true;
  if (shouldRetry) void retryCompiler();
}

watch([source, data], scheduleCompile, {flush: 'sync'});
</script>

<template>
  <section class="studio-shell" aria-label="Paper Studio WebAssembly playground">
    <header class="studio-topbar">
      <div class="brand-block">
        <a :href="withBase('/')" class="brand" aria-label="Back to PaperRune documentation">
          <i aria-hidden="true"></i><span>Paper</span><strong>Studio</strong>
        </a>
        <span class="web-label">Web</span>
      </div>
      <div class="document-state">
        <strong>{{ samples[selectedSample].name }}</strong>
        <span><i class="status-dot" :class="statusClass" aria-hidden="true"></i><span aria-live="polite">{{ statusDetail }}</span></span>
      </div>
      <div class="top-actions">
        <label class="sample-picker">
          <span>Example</span>
          <select :value="selectedSample" @change="chooseSample">
            <optgroup v-for="group in sampleGroups" :key="group.category" :label="group.category">
              <option v-for="({sample, index}) in group.samples" :key="sample.slug" :value="index">{{ sample.name }}</option>
            </optgroup>
          </select>
        </label>
        <button v-if="canRetry" class="retry-action" type="button" @click="retryCompiler">Retry</button>
      </div>
    </header>

    <div class="studio-toolbar">
      <div class="mode-switcher" aria-label="Inspector view">
        <button type="button" :class="{active: activePanel === 'source'}" @click="activePanel = 'source'">Paper</button>
        <button type="button" :class="{active: activePanel === 'data'}" @click="activePanel = 'data'">JSON</button>
        <button type="button" :class="{active: activePanel === 'inspect'}" @click="activePanel = 'inspect'">Inspect</button>
      </div>
      <span class="edit-hint">Drag to select · double-click text to edit</span>
      <span v-if="!online" class="offline-badge">Offline</span>
      <span v-else-if="loadProgress > 0 && loadProgress < 1" class="download-badge">{{ Math.round(loadProgress * 100) }}% compiler</span>
    </div>

    <div class="studio-workspace">
      <aside class="left-rail" aria-label="Document navigation">
        <header class="rail-heading">
          <span>Pages</span>
          <small>{{ pages || '—' }}</small>
        </header>
        <div class="page-list">
          <button
            v-for="pageNumber in pages"
            :key="pageNumber"
            type="button"
            :class="{active: page === pageNumber}"
            :disabled="state === 'compiling' || state === 'loading'"
            @click="compile(pageNumber)"
          >
            <span>{{ pageNumber }}</span><small>Page {{ pageNumber }}</small>
          </button>
          <p v-if="!pages">Pages appear after a valid plan.</p>
        </div>
      </aside>

      <StudioCanvas
        :image="documentImage"
        :svg="svg"
        :page-x="pageX"
        :page-y="pageY"
        :page-width="pageWidth"
        :page-height="pageHeight"
        :state="state"
        :status="status"
        :failure="failure"
        :stale="previewStale"
        :load-progress="loadProgress"
        :selection="selected"
        :inline-editor="inlineEditor"
        @page-point="selectPagePoint($event, false)"
        @edit-point="selectPagePoint($event, true)"
        @commit-inline="commitInlineEdit"
        @cancel-inline="inlineEditor = null"
        @retry="retryCompiler"
      />

      <aside class="right-panel" aria-label="Paper Studio inspector">
        <div class="panel-heading">
          <span>{{ activePanel === 'source' ? 'Paper source' : activePanel === 'data' ? 'JSON data' : 'Selection' }}</span>
          <small v-if="activePanel === 'source'">{{ sourceLines }} lines</small>
          <small v-else-if="activePanel === 'inspect'">{{ selectedKind }}</small>
        </div>
        <textarea
          v-if="activePanel === 'source'"
          v-model="source"
          aria-label="Paper source"
          spellcheck="false"
        ></textarea>
        <textarea
          v-else-if="activePanel === 'data'"
          v-model="data"
          aria-label="JSON data"
          spellcheck="false"
        ></textarea>
        <div v-else class="selection-inspector">
          <template v-if="selected">
            <div class="selection-title"><strong>{{ selected.id }}</strong><span>{{ selected.node.kind }}</span></div>
            <p>{{ selected.content.reason || (selected.content.mode === 'data' ? `Bound to ${selected.content.binding}` : 'Authored literal text') }}</p>
            <button v-if="selected.content.editable" type="button" @click="requestInlineEdit">Edit on page</button>
            <dl>
              <div><dt>Page</dt><dd>{{ page }} / {{ pages }}</dd></div>
              <div><dt>Plan</dt><dd>{{ planHash.slice(0, 12) }}</dd></div>
              <div><dt>Source</dt><dd>{{ selected.node.header_span?.start?.line || '—' }}:{{ selected.node.header_span?.start?.column || '—' }}</dd></div>
            </dl>
          </template>
          <div v-else class="inspector-empty">
            <strong>Select document text</strong>
            <p>Click a rendered block to inspect it. Drag across the page to copy text, or double-click an editable value.</p>
          </div>
        </div>

        <section v-if="failure || diagnostics.length" class="issues" aria-label="Compiler diagnostics" aria-live="polite">
          <header><span>Issues</span><strong>{{ diagnostics.length || 1 }}</strong></header>
          <article v-if="failure" role="alert">
            <div><strong>{{ state === 'offline' ? 'CONNECTION' : 'PLAYGROUND_ERROR' }}</strong><span>{{ state }}</span></div>
            <p>{{ failure }}</p>
            <button v-if="canRetry" type="button" @click="retryCompiler">Retry compiler</button>
          </article>
          <article v-for="(diagnostic, index) in diagnostics" :key="`${diagnostic.code}-${index}`">
            <div><strong>{{ diagnostic.code }}</strong><span>{{ diagnostic.stage }} · {{ diagnostic.start_line || 1 }}:{{ diagnostic.start_column || 1 }}</span></div>
            <p>{{ diagnostic.message }}</p>
            <small v-if="diagnostic.hint">{{ diagnostic.hint }}</small>
          </article>
        </section>
      </aside>
    </div>

    <footer class="studio-statusbar">
      <span>{{ online ? 'Browser workspace' : 'Working offline' }}</span>
      <span>{{ previewStale ? 'Showing last valid render' : state === 'ready' ? 'Exact WASM render' : status }}</span>
      <span>{{ pages ? `Page ${page} of ${pages}` : 'No page' }}</span>
    </footer>
  </section>
</template>

<style scoped>
.studio-shell {
  --ink: #20242a;
  --muted: #6c6e73;
  --line: #d1cec6;
  --paper: #f7f5ef;
  --accent: #2e5bd6;
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: grid;
  grid-template-rows: 54px 40px minmax(0, 1fr) 26px;
  width: 100vw;
  height: 100vh;
  height: 100svh;
  overflow: hidden;
  background: var(--paper);
  color: var(--ink);
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.studio-shell, .studio-shell * { box-sizing: border-box; }
button, select, textarea { color: inherit; font: inherit; }
button { cursor: pointer; }
.studio-topbar { display: grid; grid-template-columns: minmax(220px, .8fr) minmax(250px, 1.5fr) minmax(280px, 1fr); align-items: center; gap: 20px; padding: 0 18px; border-bottom: 1px solid var(--line); background: #fbfaf7; }
.brand-block, .brand, .document-state > span, .top-actions, .sample-picker, .studio-toolbar, .mode-switcher, .rail-heading, .panel-heading, .selection-title, .issues header, .issues article > div, .studio-statusbar { display: flex; align-items: center; }
.brand-block { gap: 9px; }
.brand { color: var(--ink); text-decoration: none; letter-spacing: -.02em; }
.brand i { width: 12px; height: 18px; margin-right: 8px; border-radius: 1px; background: var(--accent); box-shadow: inset -4px 0 rgba(255,255,255,.24); }
.brand span, .brand strong { font-size: 16px; }
.brand strong { font-weight: 780; }
.web-label { padding: 3px 5px; border: 1px solid #c9c5bb; border-radius: 3px; color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .08em; text-transform: uppercase; }
.document-state { min-width: 0; }
.document-state > strong { display: block; overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.document-state > span { gap: 7px; margin-top: 3px; overflow: hidden; color: var(--muted); font: 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.status-dot { flex: none; width: 7px; height: 7px; border-radius: 50%; background: #85878b; }
.status-dot.ready { background: #228259; }
.status-dot.warning, .status-dot.slow { background: #b37318; }
.status-dot.error, .status-dot.offline { background: #b23b32; }
.status-dot.loading, .status-dot.compiling, .status-dot.editing, .status-dot.slow { animation: pulse .85s ease-in-out infinite alternate; }
.top-actions { justify-content: flex-end; gap: 12px; min-width: 0; }
.sample-picker { gap: 8px; color: var(--muted); font-size: 10px; }
.sample-picker select { width: min(190px, 20vw); border: 0; border-bottom: 1px solid #9e9b94; outline: 0; padding: 5px 18px 5px 2px; background: transparent; font-size: 11px; }
.retry-action { border: 0; border-radius: 3px; padding: 7px 10px; background: var(--ink); color: white; font-size: 10px; }
.studio-toolbar { justify-content: space-between; gap: 16px; padding: 0 16px; border-bottom: 1px solid var(--line); background: #f1efe9; }
.mode-switcher { align-self: stretch; }
.mode-switcher button { border: 0; background: none; color: var(--muted); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .06em; text-transform: uppercase; }
.mode-switcher button { min-width: 68px; border-bottom: 2px solid transparent; }
.mode-switcher button.active { border-color: var(--accent); color: var(--ink); }
.edit-hint, .offline-badge, .download-badge { color: var(--muted); font: 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.offline-badge { color: #a3362f; }
.download-badge { color: var(--accent); }
.studio-workspace { display: grid; grid-template-columns: 154px minmax(360px, 1fr) minmax(300px, 27vw); min-width: 0; min-height: 0; overflow: hidden; }
.left-rail, .right-panel { min-width: 0; min-height: 0; background: #f7f5ef; }
.left-rail { display: grid; grid-template-rows: 38px minmax(0, 1fr); border-right: 1px solid var(--line); }
.rail-heading { justify-content: space-between; padding: 0 12px; border-bottom: 1px solid var(--line); color: var(--muted); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .06em; text-transform: uppercase; }
.rail-heading small { font-size: 8px; font-weight: 500; }
.page-list { min-height: 0; overflow: auto; padding: 10px; }
.page-list > button { width: 100%; border: 0; background: none; text-align: left; }
.page-list > button { display: grid; grid-template-columns: 42px 1fr; align-items: center; gap: 8px; margin-bottom: 7px; padding: 5px; border-radius: 3px; }
.page-list > button > span { display: grid; place-items: center; aspect-ratio: 210 / 297; border: 1px solid #cbc8c0; background: white; color: #777; font: 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; box-shadow: 0 2px 5px rgba(20,22,26,.08); }
.page-list > button small { color: var(--muted); font-size: 9px; }
.page-list > button.active { background: #e6ebf9; }
.page-list > button.active > span { border-color: var(--accent); color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.page-list p { color: var(--muted); font-size: 10px; line-height: 1.5; }
.right-panel { display: grid; grid-template-rows: 38px minmax(0, 1fr) auto; border-left: 1px solid var(--line); overflow: hidden; }
.panel-heading { justify-content: space-between; padding: 0 12px; border-bottom: 1px solid var(--line); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .05em; text-transform: uppercase; }
.panel-heading small { color: var(--muted); font-weight: 500; }
.right-panel > textarea { display: block; width: 100%; height: 100%; min-height: 0; resize: none; border: 0; outline: 0; padding: 15px; background: #20242a; color: #ece9e1; caret-color: #8da8ff; font: 12px/1.62 ui-monospace, SFMono-Regular, Menlo, monospace; tab-size: 2; }
.selection-inspector { min-height: 0; overflow: auto; padding: 14px; }
.selection-title { justify-content: space-between; gap: 12px; padding-bottom: 10px; border-bottom: 1px solid var(--line); }
.selection-title strong { font: 700 12px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; }
.selection-title span { color: var(--muted); font-size: 9px; text-transform: uppercase; }
.selection-inspector > p, .inspector-empty p { color: var(--muted); font-size: 11px; line-height: 1.55; }
.selection-inspector > button { width: 100%; border: 0; border-radius: 3px; padding: 8px; background: var(--accent); color: white; font-size: 10px; }
.selection-inspector dl { margin: 16px 0 0; }
.selection-inspector dl div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-top: 1px solid var(--line); font-size: 10px; }
.selection-inspector dt { color: var(--muted); }
.selection-inspector dd { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.inspector-empty { max-width: 240px; margin: 12vh auto 0; text-align: center; }
.inspector-empty strong { font-size: 12px; }
.issues { max-height: min(38svh, 320px); overflow: auto; border-top: 1px solid var(--line); background: #fbfaf7; }
.issues header { justify-content: space-between; min-height: 34px; padding: 0 12px; font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; letter-spacing: .06em; }
.issues header strong { display: grid; place-items: center; min-width: 18px; height: 18px; border-radius: 9px; background: #b23b32; color: white; font-size: 9px; }
.issues article { padding: 11px 12px; border-top: 1px solid var(--line); }
.issues article > div { justify-content: space-between; gap: 12px; }
.issues article strong { color: #a5352e; font: 700 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; }
.issues article span, .issues article small { color: var(--muted); font: 8px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
.issues article p { margin: 6px 0 3px; font-size: 10px; line-height: 1.45; }
.issues article button { border: 0; padding: 5px 7px; background: var(--ink); color: white; font-size: 9px; }
.studio-statusbar { justify-content: space-between; gap: 20px; padding: 0 12px; border-top: 1px solid #34383e; background: #24282e; color: #bec1c6; font: 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
@keyframes pulse { to { transform: scale(1.35); opacity: .5; } }
@media (prefers-reduced-motion: reduce) { .status-dot { animation: none; } }
@media (max-width: 980px) {
  .studio-workspace { grid-template-columns: 112px minmax(320px, 1fr) minmax(260px, 34vw); }
  .edit-hint { display: none; }
}
@media (max-width: 760px) {
  .studio-shell { grid-template-rows: auto 40px minmax(0, 1fr) 26px; }
  .studio-topbar { grid-template-columns: 1fr auto; gap: 8px; padding: 9px 12px; }
  .document-state { grid-column: 1 / -1; grid-row: 2; }
  .sample-picker > span { display: none; }
  .sample-picker select { width: 150px; }
  .studio-workspace { grid-template-columns: 72px minmax(0, 1fr); grid-template-rows: minmax(0, 58%) minmax(220px, 42%); }
  .left-rail { grid-row: 1 / -1; }
  .right-panel { grid-column: 2; grid-row: 2; border-top: 1px solid var(--line); border-left: 0; }
  .studio-canvas { grid-column: 2; grid-row: 1; }
  .rail-heading { height: 34px; padding: 0 7px; font-size: 7px; }
  .page-list { padding: 6px; }
  .page-list > button { grid-template-columns: 1fr; }
  .page-list > button small { display: none; }
  .studio-statusbar span:nth-child(2) { display: none; }
}
</style>
