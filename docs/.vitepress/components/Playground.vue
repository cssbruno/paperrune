<script setup>
import {computed, onBeforeUnmount, onMounted, ref, shallowRef} from 'vue';
import {withBase} from 'vitepress';
import {playgroundSampleManifest} from '../playground-samples.mjs';
import StudioCanvas from './playground/StudioCanvas.vue';
import WASMFileEditor from './playground/WASMFileEditor.vue';
import WASMValueEditor from './playground/WASMValueEditor.vue';
import {
  boxAsPercent,
  contentDescriptor,
  findNodeByID,
  nodePropertyValue,
  normalizeFailure,
  pickHitTarget,
  styleClasses,
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
const activePanel = ref('inspect');
const activeTool = ref('select');
const state = ref('loading');
const status = ref('Loading compiler…');
const failure = ref('');
const diagnostics = ref([]);
const textRuns = ref([]);
const fonts = ref([]);
const ast = ref(null);
const pages = ref(0);
const page = ref(1);
const planHash = ref('');
const sourceRevision = ref('');
const pageX = ref(0);
const pageY = ref(0);
const pageWidth = ref(0);
const pageHeight = ref(0);
const fixedScale = ref(1);
const overflow = ref(null);
const previewStale = ref(false);
const selected = ref(null);
const inlineEditor = ref(null);
const wasmEngine = shallowRef(null);
const online = ref(true);
const compileSlow = ref(false);
const mutationBusy = ref(false);
const canUndo = ref(false);
const canRedo = ref(false);
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
let compileSlowTimer;
const sourceLines = computed(() => source.value.split('\n').length);
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
const selectedTarget = computed(() => selected.value?.node?.id || '');
const selectedStyle = computed(() => nodePropertyValue(selected.value?.node, 'style', ''));
const selectedSize = computed(() => nodePropertyValue(selected.value?.node, 'size', ''));
const selectedColor = computed(() => nodePropertyValue(selected.value?.node, 'color', '#202A33'));
const selectedBold = computed(() => Boolean(nodePropertyValue(selected.value?.node, 'bold', false)));
const selectedItalic = computed(() => Boolean(nodePropertyValue(selected.value?.node, 'italic', false)));
const selectedAlign = computed(() => nodePropertyValue(selected.value?.node, 'align', 'left'));
const availableStyles = computed(() => styleClasses(ast.value?.root));
const canFormatSelection = computed(() => Boolean(selectedTarget.value) && !previewStale.value && !mutationBusy.value);
const selectedValueState = computed(() => {
  const content = selected.value?.content;
  if (!content) {
    return {
      kind: 'none',
      label: 'No selection',
      title: 'Select text or a layout block',
      detail: 'The value source and available editing controls will appear here.',
      syntax: '',
    };
  }
  if (content.mode === 'data' || content.binding) {
    return {
      kind: content.editable ? 'bound' : 'locked',
      label: content.editable ? 'Bound · editable' : 'Bound · locked',
      title: content.binding || content.pointer || 'JSON value',
      detail: content.editable
        ? 'This is a scalar JSON value. Editing it updates sample data through WASM.'
        : content.reason || 'This binding cannot be edited as one scalar value.',
      syntax: content.binding ? `bind: "${content.binding}"` : 'bind: <expression>',
    };
  }
  if (content.computed) {
    return {
      kind: 'computed',
      label: 'Computed · locked',
      title: 'Expression result',
      detail: content.reason || 'Change the expression or its JSON inputs in the file editors.',
      syntax: 'text: <expression>',
    };
  }
  if (content.mode === 'source') {
    return {
      kind: 'fixed',
      label: 'Fixed · editable',
      title: 'Paper source text',
      detail: 'This value is authored with text, is not linked to JSON, and can be edited directly.',
      syntax: 'text: "…"',
    };
  }
  return {
    kind: 'layout',
    label: 'Layout only',
    title: 'No direct text value',
    detail: content.reason || 'Select a text-bearing node to edit its value.',
    syntax: '',
  };
});

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
  globalThis.addEventListener('keydown', handleWorkspaceKeydown);
  void compile(1);
});

onBeforeUnmount(() => {
  clearTimeout(compileSlowTimer);
  unsubscribeRuntime?.();
  globalThis.removeEventListener('online', handleOnline);
  globalThis.removeEventListener('offline', handleOffline);
  globalThis.removeEventListener('keydown', handleWorkspaceKeydown);
});

async function compile(targetPage = page.value, {retryRuntime = false} = {}) {
  clearTimeout(compileSlowTimer);
  const sequence = ++compileSequence;
  previewStale.value = pageWidth.value > 0 && pageHeight.value > 0;
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
    wasmEngine.value = engine;
    state.value = 'compiling';
    status.value = 'Compiling exact document plan…';
    const result = await engine.compile({
      source: source.value,
      data: data.value,
      page: targetPage,
      dataName: 'playground',
      vectorOnly: true,
    });
    if (sequence !== compileSequence) return;
    diagnostics.value = result.diagnostics || [];
    pages.value = result.pages || 0;
    planHash.value = result.hash || '';
    sourceRevision.value = result.source_revision || '';
    if (!result.ok || !(Number(result.page_width_fixed) > 0) || !(Number(result.page_height_fixed) > 0)) {
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
    textRuns.value = result.text_runs || [];
    fonts.value = result.fonts || [];
    ast.value = result.ast || null;
    pageX.value = Number(result.page_x_fixed || 0);
    pageY.value = Number(result.page_y_fixed || 0);
    pageWidth.value = Number(result.page_width_fixed || 0);
    pageHeight.value = Number(result.page_height_fixed || 0);
    fixedScale.value = Number(result.fixed_scale || 1);
    overflow.value = result.overflow || null;
    previewStale.value = false;
    failure.value = '';
    selected.value = null;
    inlineEditor.value = null;
    state.value = diagnostics.value.length ? 'warning' : 'ready';
    status.value = `${result.pages} page${result.pages === 1 ? '' : 's'} · plan ${result.hash.slice(0, 10)}`;
    updateHistoryState();
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

function handleFileEditing() {
  inlineEditSequence += 1;
  clearTimeout(compileSlowTimer);
  compileSequence += 1;
  previewStale.value = pageWidth.value > 0 && pageHeight.value > 0;
  selected.value = null;
  inlineEditor.value = null;
  state.value = 'editing';
  status.value = 'WASM is updating the document…';
}

function chooseSample(event) {
  const index = Number(event.target.value);
  if (!Number.isInteger(index) || !samples[index]) return;
  selectedSample.value = index;
  source.value = samples[index].source;
  data.value = samples[index].data;
  page.value = 1;
  textRuns.value = [];
  fonts.value = [];
  overflow.value = null;
  ast.value = null;
  selected.value = null;
  inlineEditor.value = null;
  canUndo.value = false;
  canRedo.value = false;
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
    left: selection.bounds.left,
    top: selection.bounds.top,
    width: selection.bounds.width,
    height: selection.bounds.height,
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
    planHash: selection.snapshot.hash,
    compileSequence: selection.snapshot.sequence,
    page: selection.snapshot.page,
  };
}

function chooseTool(tool) {
  activeTool.value = tool;
  activePanel.value = 'inspect';
  if (tool === 'edit' && selected.value?.content?.editable) requestInlineEdit();
}

async function mutateSelected(properties = [], movement = null) {
  const selection = selected.value;
  if (!selection?.node?.id || !wasmEngine.value?.mutateNode || mutationBusy.value || previewStale.value) return;
  const target = selection.node.id;
  const retainedBounds = selection.bounds ? {...selection.bounds} : null;
  if (movement && retainedBounds) {
    retainedBounds.left += movement.xFixed / pageWidth.value * 100;
    retainedBounds.top += movement.yFixed / pageHeight.value * 100;
  }
  mutationBusy.value = true;
  previewStale.value = true;
  state.value = 'editing';
  status.value = movement ? 'Moving block with WASM…' : 'Applying formatting with WASM…';
  try {
    const result = await wasmEngine.value.mutateNode({
      hash: planHash.value,
      page: page.value,
      target,
      properties,
      moveXPoints: movement ? movement.xFixed / fixedScale.value : 0,
      moveYPoints: movement ? movement.yFixed / fixedScale.value : 0,
      vectorOnly: true,
    });
    acceptEditedWorkspace(result?.edit, result?.page, {target, bounds: retainedBounds, previous: selection});
  } catch (error) {
    previewStale.value = false;
    failure.value = normalizeFailure(error, 'The WASM editor could not update this element.');
    status.value = failure.value;
    state.value = 'error';
  } finally {
    mutationBusy.value = false;
  }
}

function setSelectedStyle(event) {
  const style = event.target.value;
  if (style) void mutateSelected([{name: 'style', kind: 'string', text: style}]);
}

function setSelectedUnit(name, rawValue) {
  const number = Number(rawValue);
  if (Number.isFinite(number) && number >= 1 && number <= 240) {
    void mutateSelected([{name, kind: 'unit', number}]);
  }
}

function setSelectedString(name, text) {
  void mutateSelected([{name, kind: 'string', text}]);
}

function toggleSelectedBool(name, current) {
  void mutateSelected([{name, kind: 'bool', bool: !current}]);
}

function nudgeSelection(xPoints, yPoints) {
  void mutateSelected([], {
    xFixed: Math.round(xPoints * fixedScale.value),
    yFixed: Math.round(yPoints * fixedScale.value),
  });
}

function moveSelected(payload) {
  if (payload?.target !== selectedTarget.value) return;
  void mutateSelected([], payload);
}

async function travelHistory(direction) {
  if (!wasmEngine.value?.historyPage || mutationBusy.value || !planHash.value) return;
  const preservation = selected.value?.node?.id && selected.value?.bounds
    ? {target: selected.value.node.id, bounds: {...selected.value.bounds}, previous: selected.value}
    : null;
  mutationBusy.value = true;
  previewStale.value = true;
  state.value = 'editing';
  status.value = direction < 0 ? 'Undoing document change…' : 'Redoing document change…';
  try {
    const result = await wasmEngine.value.historyPage({
      hash: planHash.value,
      page: page.value,
      direction,
      vectorOnly: true,
    });
    acceptEditedWorkspace(result?.edit, result?.page, preservation);
  } catch (error) {
    previewStale.value = false;
    failure.value = normalizeFailure(error, 'Document history is unavailable.');
    status.value = failure.value;
    state.value = 'error';
  } finally {
    mutationBusy.value = false;
  }
}

function updateHistoryState() {
  const state = wasmEngine.value?.historyState?.({hash: planHash.value});
  canUndo.value = !(state instanceof Error) && Boolean(state?.canUndo);
  canRedo.value = !(state instanceof Error) && Boolean(state?.canRedo);
}

function handleWorkspaceKeydown(event) {
  const target = event.target;
  if (!(event.metaKey || event.ctrlKey) || !['z', 'y'].includes(event.key.toLowerCase()) ||
      target?.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target?.tagName)) return;
  event.preventDefault();
  const redo = event.key.toLowerCase() === 'y' || event.shiftKey;
  if (redo ? canRedo.value : canUndo.value) void travelHistory(redo ? 1 : -1);
}

function acceptWASMEditorResult(payload) {
  const editor = inlineEditor.value;
  if (!editor || payload?.transaction !== editor.transaction ||
      editor.compileSequence !== compileSequence ||
      editor.planHash !== planHash.value ||
      previewStale.value) return;
  try {
    acceptEditedWorkspace(payload.result?.edit, payload.result?.page);
    inlineEditor.value = null;
  } catch (error) {
    failure.value = normalizeFailure(error, 'The WASM editor returned an invalid workspace.');
    status.value = failure.value;
    state.value = 'error';
  }
}

function acceptValueEditorResult(payload) {
  const selection = selected.value;
  if (!selection || payload?.hash !== planHash.value || previewStale.value) return;
  const preservation = selection.node?.id && selection.bounds
    ? {target: selection.node.id, bounds: {...selection.bounds}, previous: selection}
    : null;
  try {
    acceptEditedWorkspace(payload.result?.edit, payload.result?.page, preservation);
  } catch (error) {
    failure.value = normalizeFailure(error, 'The WASM value editor returned an invalid workspace.');
    status.value = failure.value;
    state.value = 'error';
  }
}

function handleValueEditorError(error) {
  failure.value = normalizeFailure(error, 'The WASM value editor could not be mounted.');
  status.value = failure.value;
  state.value = 'error';
}

function handleWASMEditorError(payload) {
  if (payload?.transaction !== inlineEditor.value?.transaction) return;
  failure.value = normalizeFailure(payload.error, 'The WASM editor could not be mounted.');
  status.value = failure.value;
  state.value = 'error';
  inlineEditor.value = null;
}

function acceptEditedWorkspace(edited, rendered, preserveSelection = null) {
  if (!edited?.ok || !edited.applied) {
    throw new Error(edited?.error || 'WASM did not apply the document edit.');
  }
  if (!rendered?.ok || !(Number(rendered.page_width_fixed) > 0) ||
      !(Number(rendered.page_height_fixed) > 0) || rendered.hash !== edited.hash) {
    throw new Error(rendered?.error || 'WASM did not render the edited workspace.');
  }
  source.value = edited.source;
  data.value = edited.data;
  diagnostics.value = edited.diagnostics || [];
  pages.value = edited.pages || 0;
  page.value = rendered.page;
  planHash.value = edited.hash || '';
  sourceRevision.value = edited.source_revision || '';
  textRuns.value = rendered.text_runs || [];
  fonts.value = rendered.fonts || [];
  ast.value = edited.ast || null;
  pageX.value = Number(rendered.page_x_fixed || 0);
  pageY.value = Number(rendered.page_y_fixed || 0);
  pageWidth.value = Number(rendered.page_width_fixed || 0);
  pageHeight.value = Number(rendered.page_height_fixed || 0);
  fixedScale.value = Number(rendered.fixed_scale || 1);
  overflow.value = rendered.overflow || null;
  previewStale.value = false;
  failure.value = '';
  selected.value = retainedSelection(preserveSelection);
  state.value = diagnostics.value.length ? 'warning' : 'ready';
  status.value = `${edited.pages} page${edited.pages === 1 ? '' : 's'} · plan ${edited.hash.slice(0, 10)}`;
  updateHistoryState();
}

function retainedSelection(preserve) {
  if (!preserve?.target || !preserve.bounds) return null;
  const node = findNodeByID(ast.value?.root, preserve.target);
  if (!node) return null;
  return {
    ...preserve.previous,
    id: preserve.target,
    node,
    content: contentDescriptor(node, data.value),
    bounds: preserve.bounds,
    snapshot: {
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
    },
  };
}

function acceptFileSnapshot(result) {
  source.value = result.source;
  data.value = result.data;
  diagnostics.value = result.diagnostics || [];
  if (!result.ok || !(Number(result.page_width_fixed) > 0) || !(Number(result.page_height_fixed) > 0)) {
    failure.value = diagnostics.value.length ? '' : normalizeFailure(result.error, 'WASM could not compile this draft.');
    state.value = diagnostics.value.length ? 'warning' : 'error';
    status.value = diagnostics.value.length
      ? `${diagnostics.value.length} diagnostic${diagnostics.value.length === 1 ? '' : 's'}`
      : failure.value;
    return;
  }
  pages.value = result.pages || 0;
  page.value = result.page;
  planHash.value = result.hash || '';
  sourceRevision.value = result.source_revision || '';
  textRuns.value = result.text_runs || [];
  fonts.value = result.fonts || [];
  ast.value = result.ast || null;
  pageX.value = Number(result.page_x_fixed || 0);
  pageY.value = Number(result.page_y_fixed || 0);
  pageWidth.value = Number(result.page_width_fixed || 0);
  pageHeight.value = Number(result.page_height_fixed || 0);
  fixedScale.value = Number(result.fixed_scale || 1);
  overflow.value = result.overflow || null;
  previewStale.value = false;
  failure.value = '';
  selected.value = null;
  inlineEditor.value = null;
  state.value = diagnostics.value.length ? 'warning' : 'ready';
  status.value = `${result.pages} page${result.pages === 1 ? '' : 's'} · plan ${result.hash.slice(0, 10)}`;
  updateHistoryState();
}

function handleFileEditorError(error) {
  failure.value = normalizeFailure(error, 'The WASM file editor could not be mounted.');
  status.value = failure.value;
  state.value = 'error';
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
        <button v-if="canRetry" class="retry-action" type="button" title="Retry compiler" aria-label="Retry compiler" @click="retryCompiler">↻</button>
      </div>
    </header>

    <div class="studio-toolbar">
      <div class="history-tools" aria-label="Document history">
        <button type="button" title="Undo (Ctrl/Cmd+Z)" :disabled="!canUndo || mutationBusy" @click="travelHistory(-1)">↶</button>
        <button type="button" title="Redo (Ctrl/Cmd+Shift+Z)" :disabled="!canRedo || mutationBusy" @click="travelHistory(1)">↷</button>
      </div>
      <i class="tool-divider" aria-hidden="true"></i>
      <label class="style-tool">
        <span>Style</span>
        <select :value="selectedStyle" :disabled="!canFormatSelection" @change="setSelectedStyle">
          <option value="" disabled>{{ selectedTarget ? 'Choose style' : 'Select an element' }}</option>
          <option v-for="style in availableStyles" :key="style.id" :value="style.id">{{ style.label }}</option>
        </select>
      </label>
      <label class="size-tool">
        <span>Size</span>
        <input
          type="number"
          min="1"
          max="240"
          step="0.5"
          :value="selectedSize"
          :disabled="!canFormatSelection"
          @change="setSelectedUnit('size', $event.target.value)"
        />
      </label>
      <div class="format-tools" aria-label="Text formatting">
        <button type="button" title="Bold" :class="{active: selectedBold}" :disabled="!canFormatSelection" @click="toggleSelectedBool('bold', selectedBold)"><b>B</b></button>
        <button type="button" title="Italic" :class="{active: selectedItalic}" :disabled="!canFormatSelection" @click="toggleSelectedBool('italic', selectedItalic)"><i>I</i></button>
      </div>
      <i class="tool-divider" aria-hidden="true"></i>
      <div class="align-tools" aria-label="Paragraph alignment">
        <button v-for="alignment in ['left', 'center', 'right', 'justify']" :key="alignment" type="button" :title="`Align ${alignment}`" :class="{active: selectedAlign === alignment}" :disabled="!canFormatSelection" @click="setSelectedString('align', alignment)">{{ alignment === 'left' ? '≡' : alignment === 'center' ? '≣' : alignment === 'right' ? '≡' : '☰' }}</button>
      </div>
      <label class="color-tool" title="Text color">
        <input type="color" :value="selectedColor" :disabled="!canFormatSelection" @change="setSelectedString('color', $event.target.value)" />
      </label>
      <button class="edit-tool" type="button" title="Edit text on page" aria-label="Edit text on page" :disabled="!selected?.content?.editable || mutationBusy" @click="requestInlineEdit">✎</button>
      <span class="edit-hint">{{ selectedTarget ? `${selectedTarget} · drag the six-dot handle to move` : 'Click an element to format · double-click text to edit' }}</span>
      <span v-if="mutationBusy" class="editing-badge">Applying…</span>
      <span v-else-if="!online" class="offline-badge">Offline</span>
      <span v-else-if="loadProgress > 0 && loadProgress < 1" class="download-badge">{{ Math.round(loadProgress * 100) }}% compiler</span>
    </div>

    <div class="studio-workspace">
      <aside class="left-rail" aria-label="Document toolbox and navigation">
        <section class="toolbox" aria-label="Toolbox">
          <header class="rail-heading">
            <span>Tools</span>
          </header>
          <div class="tool-list">
            <button type="button" title="Select and format" aria-label="Select and format" :class="{active: activeTool === 'select'}" @click="chooseTool('select')">
              <b aria-hidden="true">↖</b>
            </button>
            <button type="button" title="Edit text" aria-label="Edit text" :class="{active: activeTool === 'edit'}" :disabled="selected && !selected.content?.editable" @click="chooseTool('edit')">
              <b aria-hidden="true">T</b>
            </button>
            <button type="button" title="Move selected block" aria-label="Move selected block" :class="{active: activeTool === 'move'}" :disabled="!selectedTarget" @click="chooseTool('move')">
              <b aria-hidden="true">✥</b>
            </button>
          </div>
          <div class="value-key" aria-label="Value source legend">
            <strong>Value source</strong>
            <span class="bound" title="JSON bound" aria-label="JSON bound" tabindex="0">{ }</span>
            <span class="fixed" title="Fixed Paper text" aria-label="Fixed Paper text" tabindex="0">T</span>
            <span class="computed" title="Computed or locked" aria-label="Computed or locked" tabindex="0">ƒ</span>
          </div>
        </section>
        <section class="pages-panel" aria-label="Document pages">
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
              <span :aria-label="`Page ${pageNumber}`">{{ pageNumber }}</span>
            </button>
            <p v-if="!pages">Pages appear after a valid plan.</p>
          </div>
        </section>
      </aside>

      <StudioCanvas
        :wasm-engine="wasmEngine"
        :plan-hash="planHash"
        :page="page"
        :text-runs="textRuns"
        :fonts="fonts"
        :page-x="pageX"
        :page-y="pageY"
        :page-width="pageWidth"
        :page-height="pageHeight"
        :fixed-scale="fixedScale"
        :overflow="overflow"
        :state="state"
        :status="status"
        :failure="failure"
        :stale="previewStale"
        :load-progress="loadProgress"
        :selection="selected"
        :inline-editor="inlineEditor"
        @page-point="selectPagePoint($event, activeTool === 'edit')"
        @edit-point="selectPagePoint($event, true)"
        @move-selection="moveSelected"
        @editor-applied="acceptWASMEditorResult"
        @editor-error="handleWASMEditorError"
        @render-error="handleFileEditorError"
        @cancel-inline="inlineEditor = null"
        @retry="retryCompiler"
      />

      <aside class="right-panel" aria-label="Paper Studio inspector">
        <div class="panel-heading">
          <div class="panel-tabs" aria-label="Side panel">
            <button type="button" title="Properties" aria-label="Properties" :class="{active: activePanel === 'inspect'}" @click="activePanel = 'inspect'">⚙</button>
            <button type="button" title="Paper source" aria-label="Paper source" :class="{active: activePanel === 'source'}" @click="activePanel = 'source'">¶</button>
            <button type="button" title="JSON data" aria-label="JSON data" :class="{active: activePanel === 'data'}" @click="activePanel = 'data'">{ }</button>
          </div>
          <small v-if="activePanel === 'source'">{{ sourceLines }} lines</small>
          <small v-else-if="activePanel === 'inspect'">{{ selectedKind }}</small>
        </div>
        <WASMFileEditor
          v-show="activePanel === 'source'"
          :engine="wasmEngine"
          :hash="planHash"
          :page="page"
          kind="source"
          @editing="handleFileEditing"
          @snapshot="acceptFileSnapshot"
          @error="handleFileEditorError"
        />
        <WASMFileEditor
          v-show="activePanel === 'data'"
          :engine="wasmEngine"
          :hash="planHash"
          :page="page"
          kind="data"
          @editing="handleFileEditing"
          @snapshot="acceptFileSnapshot"
          @error="handleFileEditorError"
        />
        <div v-show="activePanel === 'inspect'" class="selection-inspector">
          <template v-if="selected">
            <div class="selection-title"><strong>{{ selected.id }}</strong><span>{{ selected.node.kind }}</span></div>
            <section class="value-origin" :class="selectedValueState.kind">
              <span>{{ selectedValueState.label }}</span>
              <strong>{{ selectedValueState.title }}</strong>
              <p>{{ selectedValueState.detail }}</p>
              <code v-if="selectedValueState.syntax">{{ selectedValueState.syntax }}</code>
            </section>
            <section class="edit-box">
              <header>
                <span>Edit value</span>
                <small>{{ selected.content.editable ? 'WASM editor' : 'Not directly editable' }}</small>
              </header>
              <WASMValueEditor
                v-if="selected.content.editable && !inlineEditor"
                :engine="wasmEngine"
                :hash="planHash"
                :page="page"
                :descriptor="selected.content"
                :target="selectedTarget"
                :disabled="mutationBusy || previewStale"
                @applied="acceptValueEditorResult"
                @error="handleValueEditorError"
              />
              <div v-else-if="inlineEditor" class="edit-box-message">
                <strong>Editing on page</strong>
                <span>Press Ctrl/Cmd + Enter to apply or Escape to cancel.</span>
              </div>
              <div v-else class="edit-box-message">
                <strong>Direct editing unavailable</strong>
                <span>{{ selected.content.reason || 'Choose a fixed text or scalar JSON-bound value.' }}</span>
              </div>
            </section>
            <label class="property-field">
              <span>Style class</span>
              <select :value="selectedStyle" :disabled="!canFormatSelection" @change="setSelectedStyle">
                <option value="" disabled>Choose style</option>
                <option v-for="style in availableStyles" :key="style.id" :value="style.id">{{ style.id }}</option>
              </select>
            </label>
            <div class="property-actions">
              <button v-if="selected.content.editable" class="primary-icon-action" type="button" title="Edit text on page" aria-label="Edit text on page" @click="requestInlineEdit">✎</button>
              <button type="button" :title="selectedBold ? 'Remove bold' : 'Bold'" :aria-label="selectedBold ? 'Remove bold' : 'Bold'" :class="{active: selectedBold}" :disabled="!canFormatSelection" @click="toggleSelectedBool('bold', selectedBold)"><b>B</b></button>
              <button type="button" :title="selectedItalic ? 'Remove italic' : 'Italic'" :aria-label="selectedItalic ? 'Remove italic' : 'Italic'" :class="{active: selectedItalic}" :disabled="!canFormatSelection" @click="toggleSelectedBool('italic', selectedItalic)"><i>I</i></button>
            </div>
            <div class="move-control">
              <span>Move block</span>
              <div>
                <button type="button" aria-label="Move up" :disabled="!canFormatSelection" @click="nudgeSelection(0, -4)">↑</button>
                <button type="button" aria-label="Move left" :disabled="!canFormatSelection" @click="nudgeSelection(-4, 0)">←</button>
                <button type="button" aria-label="Move right" :disabled="!canFormatSelection" @click="nudgeSelection(4, 0)">→</button>
                <button type="button" aria-label="Move down" :disabled="!canFormatSelection" @click="nudgeSelection(0, 4)">↓</button>
              </div>
              <small>4pt steps · or drag the handle on the page</small>
            </div>
            <dl>
              <div><dt>Page</dt><dd>{{ page }} / {{ pages }}</dd></div>
              <div><dt>Plan</dt><dd>{{ planHash.slice(0, 12) }}</dd></div>
              <div><dt>Source</dt><dd>{{ selected.node.header_span?.start?.line || '—' }}:{{ selected.node.header_span?.start?.column || '—' }}</dd></div>
            </dl>
          </template>
          <div v-else class="inspector-empty">
            <strong>Select an element</strong>
            <p>Click a block to see whether its value is JSON-bound, fixed Paper text, computed, or layout-only.</p>
            <div class="empty-value-key">
              <span><i class="bound"></i><b>Bound</b> changes JSON</span>
              <span><i class="fixed"></i><b>Fixed</b> changes Paper text</span>
              <span><i class="computed"></i><b>Computed</b> changes its inputs</span>
            </div>
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
  grid-template-rows: 54px 44px minmax(0, 1fr) 26px;
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
.brand-block, .brand, .document-state > span, .top-actions, .sample-picker, .studio-toolbar, .history-tools, .format-tools, .align-tools, .rail-heading, .panel-heading, .panel-tabs, .selection-title, .issues header, .issues article > div, .studio-statusbar { display: flex; align-items: center; }
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
.retry-action { display: grid; place-items: center; width: 28px; height: 28px; border: 0; border-radius: 3px; padding: 0; background: var(--ink); color: white; font-size: 15px; }
.studio-toolbar { justify-content: flex-start; gap: 5px; overflow-x: auto; padding: 0 12px; border-bottom: 1px solid var(--line); background: #f1efe9; }
.studio-toolbar button { flex: none; height: 28px; border: 0; border-radius: 3px; background: transparent; font-size: 11px; }
.studio-toolbar button:hover:not(:disabled) { background: #e2e0d9; }
.studio-toolbar button.active { background: #dbe3f8; color: #244db8; }
.studio-toolbar button:disabled, .selection-inspector button:disabled { cursor: default; opacity: .38; }
.history-tools, .format-tools, .align-tools { gap: 1px; }
.history-tools button { width: 28px; font-size: 18px; }
.format-tools button, .align-tools button { width: 28px; }
.align-tools button:nth-child(3) { transform: scaleX(-1); }
.tool-divider { flex: none; width: 1px; height: 22px; margin: 0 4px; background: #cbc8c0; }
.style-tool, .size-tool { display: flex; align-items: center; gap: 5px; height: 30px; }
.style-tool span, .size-tool span { color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; }
.style-tool select, .size-tool input { height: 28px; border: 1px solid #c7c4bc; border-radius: 3px; background: #fbfaf7; outline: none; font-size: 10px; }
.style-tool select { width: 122px; padding: 0 22px 0 7px; }
.size-tool input { width: 54px; padding: 0 4px; }
.color-tool { display: grid; place-items: center; flex: none; width: 30px; height: 28px; border-radius: 3px; }
.color-tool input { width: 20px; height: 20px; border: 0; padding: 0; background: none; cursor: pointer; }
.edit-tool { width: 30px; padding: 0; background: var(--ink) !important; color: white; font-size: 15px !important; }
.edit-hint { min-width: 180px; margin-left: auto; overflow: hidden; text-align: right; text-overflow: ellipsis; white-space: nowrap; }
.edit-hint, .offline-badge, .download-badge, .editing-badge { color: var(--muted); font: 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.editing-badge { color: var(--accent); }
.offline-badge { color: #a3362f; }
.download-badge { color: var(--accent); }
.studio-workspace { display: grid; grid-template-columns: 82px minmax(360px, 1fr) minmax(330px, 28vw); min-width: 0; min-height: 0; overflow: hidden; }
.left-rail, .right-panel { min-width: 0; min-height: 0; background: #f7f5ef; }
.left-rail { display: grid; grid-template-rows: auto minmax(0, 1fr); border-right: 1px solid var(--line); overflow: hidden; }
.rail-heading { justify-content: space-between; padding: 0 8px; border-bottom: 1px solid var(--line); color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .05em; text-transform: uppercase; }
.rail-heading small { font-size: 8px; font-weight: 500; }
.toolbox { border-bottom: 1px solid var(--line); background: #f1efe9; }
.toolbox .rail-heading, .pages-panel .rail-heading { display: flex; height: 38px; }
.tool-list { display: grid; gap: 4px; padding: 7px; }
.tool-list button { display: grid; place-items: center; width: 100%; height: 42px; border: 0; border-radius: 3px; padding: 0; background: transparent; }
.tool-list button:hover:not(:disabled) { background: #e3e0d8; }
.tool-list button.active { background: #dbe3f8; color: #244db8; }
.tool-list button:disabled { cursor: default; opacity: .42; }
.tool-list button > b { display: grid; place-items: center; width: 32px; height: 32px; border: 1px solid #c6c3ba; border-radius: 3px; background: #fbfaf7; font-size: 14px; }
.value-key { display: grid; grid-template-columns: repeat(3, 1fr); gap: 4px; padding: 9px 7px 10px; border-top: 1px solid #d8d5cd; }
.value-key > strong { grid-column: 1 / -1; color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .05em; text-transform: uppercase; }
.value-key > span { display: grid; place-items: center; height: 24px; border: 1px solid #cbc8bf; border-radius: 3px; background: #fbfaf7; font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; cursor: help; }
.value-key > span.bound { border-color: #9eafe4; color: #244db8; }
.value-key > span.fixed { border-color: #94b8a6; color: #176745; }
.value-key > span.computed { border-color: #b9ae9e; color: #6f6251; }
.value-key i, .empty-value-key i { flex: none; width: 7px; height: 7px; border-radius: 50%; }
.value-key i.bound, .empty-value-key i.bound { background: #2e5bd6; }
.value-key i.fixed, .empty-value-key i.fixed { background: #21805a; }
.value-key i.computed, .empty-value-key i.computed { background: #8a7c68; }
.pages-panel { display: grid; grid-template-rows: 38px minmax(0, 1fr); min-height: 0; }
.page-list { min-height: 0; overflow: auto; padding: 7px; }
.page-list > button { display: grid; place-items: center; width: 100%; margin-bottom: 7px; border: 0; border-radius: 3px; padding: 5px; background: none; }
.page-list > button > span { display: grid; place-items: center; width: 42px; aspect-ratio: 210 / 297; border: 1px solid #cbc8c0; background: white; color: #777; font: 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; box-shadow: 0 2px 5px rgba(20,22,26,.08); }
.page-list > button.active { background: #e6ebf9; }
.page-list > button.active > span { border-color: var(--accent); color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.page-list p { color: var(--muted); font-size: 10px; line-height: 1.5; }
.right-panel { display: grid; grid-template-rows: 38px minmax(0, 1fr) auto; border-left: 1px solid var(--line); overflow: hidden; }
.panel-heading { justify-content: space-between; padding: 0 8px 0 4px; border-bottom: 1px solid var(--line); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .05em; text-transform: uppercase; }
.panel-heading small { color: var(--muted); font-weight: 500; }
.panel-tabs { align-self: stretch; }
.panel-tabs button { min-width: 34px; border: 0; border-bottom: 2px solid transparent; padding: 0 8px; background: none; color: var(--muted); font: 700 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: none; }
.panel-tabs button.active { border-color: var(--accent); color: var(--ink); }
.selection-inspector { min-height: 0; overflow: auto; padding: 14px; }
.selection-title { justify-content: space-between; gap: 12px; padding-bottom: 10px; border-bottom: 1px solid var(--line); }
.selection-title strong { font: 700 12px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; }
.selection-title span { color: var(--muted); font-size: 9px; text-transform: uppercase; }
.selection-inspector > p, .inspector-empty p { color: var(--muted); font-size: 11px; line-height: 1.55; }
.value-origin { display: grid; gap: 6px; margin-top: 12px; padding: 11px 12px; border-left: 3px solid #8a7c68; background: #eeebe4; }
.value-origin.bound { border-color: #2e5bd6; background: #e8edfb; }
.value-origin.fixed { border-color: #21805a; background: #e6f0ea; }
.value-origin.computed, .value-origin.locked { border-color: #9a6a25; background: #f5ecdc; }
.value-origin > span { width: max-content; border-radius: 2px; padding: 3px 5px; background: rgba(255,255,255,.65); color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .04em; text-transform: uppercase; }
.value-origin > strong { font-size: 11px; overflow-wrap: anywhere; }
.value-origin > p { margin: 0; color: #555960; font-size: 10px; line-height: 1.45; }
.value-origin > code { width: max-content; max-width: 100%; overflow: hidden; padding: 3px 5px; background: rgba(255,255,255,.7); color: #34383e; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.edit-box { margin-top: 14px; }
.edit-box > header { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 7px; }
.edit-box > header span { font-size: 11px; font-weight: 700; }
.edit-box > header small { color: var(--muted); font: 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-transform: uppercase; }
.edit-box-message { display: grid; gap: 5px; min-height: 82px; align-content: center; padding: 12px; border: 1px dashed #c6c2b8; border-radius: 4px; background: #f2f0ea; text-align: center; }
.edit-box-message strong { font-size: 10px; }
.edit-box-message span { color: var(--muted); font-size: 9px; line-height: 1.45; }
.property-field { display: grid; gap: 6px; margin-top: 14px; }
.property-field > span, .move-control > span { color: var(--muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .05em; text-transform: uppercase; }
.property-field select { width: 100%; height: 32px; border: 1px solid var(--line); border-radius: 3px; padding: 0 8px; background: #fbfaf7; font-size: 10px; }
.property-actions { display: grid; grid-template-columns: repeat(3, 32px); gap: 5px; margin-top: 9px; }
.property-actions button { display: grid; place-items: center; width: 32px; height: 31px; border: 0; border-radius: 3px; padding: 0; background: #e7e4dc; font-size: 11px; }
.property-actions button.primary-icon-action { background: var(--accent); color: white; }
.property-actions button.active { background: #dbe3f8; color: #244db8; }
.move-control { display: grid; gap: 7px; margin-top: 18px; padding-top: 14px; border-top: 1px solid var(--line); }
.move-control > div { display: grid; grid-template-columns: repeat(4, 31px); gap: 4px; }
.move-control button { height: 31px; border: 1px solid var(--line); border-radius: 3px; background: #fbfaf7; }
.move-control small { color: var(--muted); font-size: 9px; }
.selection-inspector dl { margin: 16px 0 0; }
.selection-inspector dl div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-top: 1px solid var(--line); font-size: 10px; }
.selection-inspector dt { color: var(--muted); }
.selection-inspector dd { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.inspector-empty { max-width: 250px; margin: 7vh auto 0; text-align: center; }
.inspector-empty strong { font-size: 12px; }
.empty-value-key { display: grid; gap: 7px; margin-top: 18px; padding: 12px; border-top: 1px solid var(--line); text-align: left; }
.empty-value-key span { display: flex; align-items: center; gap: 6px; color: var(--muted); font-size: 9px; }
.empty-value-key b { color: var(--ink); font-size: 9px; }
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
  .studio-workspace { grid-template-columns: 74px minmax(320px, 1fr) minmax(300px, 34vw); }
  .edit-hint { display: none; }
}
@media (max-width: 760px) {
  .studio-shell { grid-template-rows: auto 44px minmax(0, 1fr) 26px; }
  .studio-topbar { grid-template-columns: 1fr auto; gap: 8px; padding: 9px 12px; }
  .document-state { grid-column: 1 / -1; grid-row: 2; }
  .sample-picker > span { display: none; }
  .sample-picker select { width: 150px; }
  .studio-workspace { grid-template-columns: 78px minmax(0, 1fr); grid-template-rows: minmax(0, 58%) minmax(220px, 42%); }
  .left-rail { grid-row: 1 / -1; }
  .right-panel { grid-column: 2; grid-row: 2; border-top: 1px solid var(--line); border-left: 0; }
  .studio-canvas { grid-column: 2; grid-row: 1; }
  .rail-heading { height: 34px; padding: 0 7px; font-size: 7px; }
  .rail-heading small, .value-key { display: none; }
  .tool-list { padding: 5px; }
  .tool-list button { display: grid; grid-template-columns: 1fr; justify-items: center; padding: 4px; }
  .tool-list button > b { width: 30px; height: 30px; }
  .page-list { padding: 6px; }
  .page-list > button { grid-template-columns: 1fr; }
  .page-list > button small { display: none; }
  .studio-statusbar span:nth-child(2) { display: none; }
}
</style>
