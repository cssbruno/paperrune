<script setup>
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue';
import {withBase} from 'vitepress';
import {playgroundSamples as samples} from '../playground-samples.mjs';

let runtimePromise;

const selectedSample = ref(0);
const source = ref(samples[0].source);
const data = ref(samples[0].data);
const activeEditor = ref('source');
const state = ref('loading');
const status = ref('Loading compiler…');
const diagnostics = ref([]);
const svg = ref('');
const pages = ref(0);
const page = ref(1);
const planHash = ref('');
let compileSequence = 0;
let debounceTimer;

const sourceLines = computed(() => source.value.split('\n').length);
const previewDocument = computed(() => svg.value ? `<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0;min-height:100%;background:#d8d3c8}body{display:grid;place-items:center;padding:16px;box-sizing:border-box}svg{display:block;width:100%;max-width:760px;height:auto;box-shadow:0 18px 50px rgba(20,22,27,.18)}</style></head><body>${svg.value}</body></html>` : '');

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[data-paperrune-runtime="${src}"]`);
    if (existing) {
      if (globalThis.Go) resolve();
      else existing.addEventListener('load', resolve, {once: true});
      return;
    }
    const script = document.createElement('script');
    script.src = src;
    script.dataset.paperruneRuntime = src;
    script.addEventListener('load', resolve, {once: true});
    script.addEventListener('error', () => reject(new Error('Could not load the Go WebAssembly runtime')), {once: true});
    document.head.appendChild(script);
  });
}

async function loadRuntime() {
  if (globalThis.PaperStudioWASM?.compile) return globalThis.PaperStudioWASM;
  if (!runtimePromise) runtimePromise = (async () => {
    await loadScript(withBase('/wasm_exec.js'));
    const go = new globalThis.Go();
    const response = await fetch(withBase('/paperrune.wasm'), {cache: 'no-cache'});
    if (!response.ok) throw new Error(`Compiler download failed (${response.status})`);
    let instantiated;
    if (WebAssembly.instantiateStreaming) {
      try {
        instantiated = await WebAssembly.instantiateStreaming(response.clone(), go.importObject);
      } catch (_) {
        instantiated = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      }
    } else {
      instantiated = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
    }
    go.run(instantiated.instance).catch((error) => {
      console.error('PaperRune WASM runtime stopped', error);
    });
    for (let attempt = 0; attempt < 200 && !globalThis.PaperStudioWASM?.compile; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
    if (!globalThis.PaperStudioWASM?.compile) throw new Error('Compiler initialized without the playground API');
    return globalThis.PaperStudioWASM;
  })();
  return runtimePromise;
}

async function compile(targetPage = page.value) {
  const sequence = ++compileSequence;
  state.value = 'compiling';
  status.value = 'Compiling in WebAssembly…';
  try {
    const engine = await loadRuntime();
    const result = await engine.compile({source: source.value, data: data.value, page: targetPage, dataName: 'playground'});
    if (sequence !== compileSequence) return;
    diagnostics.value = result.diagnostics || [];
    pages.value = result.pages || 0;
    planHash.value = result.hash || '';
    if (!result.ok || !result.svg) {
      svg.value = '';
      state.value = 'error';
      status.value = diagnostics.value.length ? `${diagnostics.value.length} diagnostic${diagnostics.value.length === 1 ? '' : 's'}` : (result.error || 'Compilation failed');
      return;
    }
    page.value = result.page;
    svg.value = result.svg;
    state.value = diagnostics.value.length ? 'warning' : 'ready';
    status.value = `${result.pages} page${result.pages === 1 ? '' : 's'} · plan ${result.hash.slice(0, 10)}`;
  } catch (error) {
    if (sequence !== compileSequence) return;
    svg.value = '';
    diagnostics.value = [];
    state.value = 'error';
    status.value = error instanceof Error ? error.message : String(error);
  }
}

function scheduleCompile() {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => compile(1), 550);
}

function chooseSample(event) {
  const index = Number(event.target.value);
  selectedSample.value = index;
  source.value = samples[index].source;
  data.value = samples[index].data;
  page.value = 1;
  compile(1);
}

watch([source, data], scheduleCompile);
onMounted(() => compile(1));
onBeforeUnmount(() => clearTimeout(debounceTimer));
</script>

<template>
  <section class="playground-shell" aria-label="PaperRune WebAssembly playground">
    <header class="playground-bar">
      <div class="playground-heading">
        <a class="back-link" :href="withBase('/')" aria-label="Back to PaperRune documentation">← PaperRune</a>
        <div class="playground-status">
          <span class="status-dot" :class="state" aria-hidden="true"></span>
          <span aria-live="polite">{{ status }}</span>
        </div>
      </div>
      <div class="playground-actions">
        <label>
          <span class="sr-only">Example</span>
          <select :value="selectedSample" @change="chooseSample">
            <option v-for="(sample, index) in samples" :key="sample.name" :value="index">{{ sample.name }}</option>
          </select>
        </label>
      </div>
    </header>

    <div class="playground-workspace">
      <div class="editor-pane">
        <nav class="editor-tabs" aria-label="Playground inputs">
          <button type="button" :class="{active: activeEditor === 'source'}" @click="activeEditor = 'source'">Paper <span>{{ sourceLines }} lines</span></button>
          <button type="button" :class="{active: activeEditor === 'data'}" @click="activeEditor = 'data'">JSON data</button>
        </nav>
        <textarea v-show="activeEditor === 'source'" v-model="source" aria-label="Paper source" spellcheck="false"></textarea>
        <textarea v-show="activeEditor === 'data'" v-model="data" aria-label="JSON data" spellcheck="false"></textarea>
      </div>

      <div class="preview-pane">
        <div class="preview-toolbar">
          <span>Exact SVG preview</span>
          <div v-if="pages > 1" class="page-controls" aria-label="Preview pages">
            <button type="button" :disabled="page <= 1 || state === 'compiling'" @click="compile(page - 1)">←</button>
            <span>{{ page }} / {{ pages }}</span>
            <button type="button" :disabled="page >= pages || state === 'compiling'" @click="compile(page + 1)">→</button>
          </div>
        </div>
        <iframe v-if="previewDocument" :srcdoc="previewDocument" sandbox="" title="Compiled Paper document preview"></iframe>
        <div v-else class="preview-empty">
          <strong>{{ state === 'loading' || state === 'compiling' ? 'Planning document' : 'No preview' }}</strong>
          <span>{{ state === 'error' ? 'Fix the diagnostics below and compile again.' : 'The exact page appears here.' }}</span>
        </div>
      </div>
    </div>

    <div v-if="diagnostics.length" class="diagnostic-list" aria-live="polite">
      <article v-for="(diagnostic, index) in diagnostics" :key="`${diagnostic.code}-${index}`">
        <div><strong>{{ diagnostic.code }}</strong><span>{{ diagnostic.stage }} · {{ diagnostic.start_line || 1 }}:{{ diagnostic.start_column || 1 }}</span></div>
        <p>{{ diagnostic.message }}</p>
        <small v-if="diagnostic.hint">{{ diagnostic.hint }}</small>
      </article>
    </div>
  </section>
</template>

<style scoped>
.playground-shell {
  --play-border: rgba(23, 26, 31, 0.17);
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  width: 100vw;
  height: 100vh;
  height: 100svh;
  background: #e8e3d8;
  color: #171a1f;
  overflow: hidden;
}
.playground-shell, .playground-shell * { box-sizing: border-box; }
.playground-bar, .preview-toolbar, .editor-tabs {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.playground-bar { min-height: 58px; padding: 10px max(20px, calc((100vw - 1320px) / 2)); border-bottom: 1px solid var(--play-border); }
.playground-heading, .playground-status, .playground-actions { display: flex; align-items: center; gap: 10px; }
.playground-heading { gap: 14px; }
.back-link { color: inherit; font-weight: 750; text-decoration: none; }
.back-link:hover { color: #2459d3; }
.back-link::after { content: ''; display: inline-block; width: 1px; height: 18px; margin-left: 14px; vertical-align: middle; background: var(--play-border); }
.playground-status { font: 600 0.78rem/1.2 var(--vp-font-family-mono); text-transform: uppercase; letter-spacing: 0.06em; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #777; transition: background .2s, transform .2s; }
.status-dot.compiling, .status-dot.loading { background: #2459d3; animation: pulse 1s infinite alternate; }
.status-dot.ready { background: #1d8a57; }
.status-dot.warning { background: #b16b00; }
.status-dot.error { background: #c0362c; }
select, button { font: inherit; color: inherit; }
select { max-width: 180px; border: 0; border-bottom: 1px solid #8f8b83; background: transparent; padding: 7px 22px 7px 4px; }
button { border: 0; background: none; cursor: pointer; }
button:disabled { cursor: wait; opacity: .48; }
.playground-workspace { display: grid; grid-template-columns: minmax(0, 1fr) minmax(380px, 1fr); min-height: 0; overflow: hidden; }
.editor-pane, .preview-pane { min-width: 0; min-height: 0; }
.editor-pane { display: grid; grid-template-rows: 48px 1fr; border-right: 1px solid var(--play-border); background: #171a1f; }
.editor-tabs { justify-content: flex-start; gap: 22px; padding: 0 20px; color: #9fa5af; border-bottom: 1px solid rgba(255,255,255,.12); }
.editor-tabs button { height: 48px; color: inherit; font: 650 .77rem/1 var(--vp-font-family-mono); text-transform: uppercase; letter-spacing: .06em; }
.editor-tabs button.active { color: #fff; box-shadow: inset 0 -2px #87a7ff; }
.editor-tabs span { margin-left: 5px; opacity: .58; text-transform: none; }
textarea { display: block; width: 100%; height: 100%; min-height: 0; resize: none; border: 0; outline: 0; padding: 22px; background: #171a1f; color: #ece8df; caret-color: #87a7ff; font: 13px/1.65 var(--vp-font-family-mono); tab-size: 2; }
.preview-pane { display: grid; grid-template-rows: 48px 1fr; background: #d8d3c8; }
.preview-toolbar { padding: 0 18px; border-bottom: 1px solid var(--play-border); font: 650 .76rem/1 var(--vp-font-family-mono); text-transform: uppercase; letter-spacing: .06em; }
.page-controls { display: flex; align-items: center; gap: 8px; }
.page-controls button { width: 27px; height: 27px; border: 1px solid #aaa59d; border-radius: 50%; }
iframe { display: block; width: 100%; height: 100%; min-height: 0; border: 0; background: #d8d3c8; }
.preview-empty { display: grid; place-content: center; gap: 5px; text-align: center; color: #66645f; }
.preview-empty strong { color: #2c2e32; }
.diagnostic-list { max-height: min(28svh, 260px); padding: 0 max(20px, calc((100vw - 1320px) / 2)); overflow-y: auto; background: #f4f0e7; }
.diagnostic-list article { padding: 16px 0; border-top: 1px solid var(--play-border); }
.diagnostic-list article > div { display: flex; justify-content: space-between; gap: 20px; }
.diagnostic-list strong { font: 700 .78rem/1.2 var(--vp-font-family-mono); color: #a22c24; }
.diagnostic-list span, .diagnostic-list small { color: #6d6c69; font: .76rem/1.4 var(--vp-font-family-mono); }
.diagnostic-list p { margin: 8px 0 3px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; }
@keyframes pulse { to { transform: scale(1.45); opacity: .55; } }
@media (prefers-reduced-motion: reduce) { .status-dot { animation: none; transition: none; } }
@media (max-width: 900px) {
  .playground-workspace { grid-template-columns: 1fr; grid-template-rows: minmax(0, 1fr) minmax(0, 1fr); }
  .editor-pane { border-right: 0; border-bottom: 1px solid var(--play-border); }
}
@media (max-width: 560px) {
  .playground-bar { flex-direction: column; align-items: stretch; gap: 12px; padding: 12px 16px; }
  .playground-heading { justify-content: space-between; }
  .back-link::after { display: none; }
  .playground-actions { justify-content: space-between; width: 100%; }
  .playground-actions label { flex: 1; }
  select { width: 100%; max-width: none; }
  textarea { padding: 16px; font-size: 12px; }
}
</style>
