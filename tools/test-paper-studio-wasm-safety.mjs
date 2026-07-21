// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import {Worker, isMainThread, parentPort, workerData} from 'node:worker_threads';

function canonicalJSON(value) {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJSON(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

async function rendererWorker() {
  const runtimeSource = await readFile(workerData.runtimePath, 'utf8');
  vm.runInThisContext(runtimeSource, {filename: workerData.runtimePath});
  const go = new Go();
  const instance = await WebAssembly.instantiate(workerData.module, go.importObject);
  let runtimeFailure = null;
  go.run(instance).catch((error) => { runtimeFailure = error; });
  for (let attempt = 0; attempt < 500 && !globalThis.PaperStudioWASM; attempt += 1) {
    await new Promise((resolve) => setTimeout(resolve, 1));
  }
  if (runtimeFailure) throw runtimeFailure;
  const engine = globalThis.PaperStudioWASM;
  if (!engine?.render) throw new Error(`${workerData.label} renderer did not initialize`);
  const memory = instance.exports.memory || instance.exports.mem || null;

  parentPort.postMessage({type: 'ready', rendererVersion: engine.rendererVersion});
  parentPort.on('message', async ({id, payload}) => {
    try {
      const result = await engine.render(new Uint8Array(payload));
      const png = new Uint8Array(result.png);
      parentPort.postMessage({
        id,
        outcome: {
          ok: true,
          manifest: canonicalJSON(result.manifest),
          pngBytes: png.byteLength,
          pngSHA256: createHash('sha256').update(png).digest('hex'),
          memoryBytes: memory?.buffer?.byteLength ?? null,
        },
      });
    } catch {
      parentPort.postMessage({id, outcome: {ok: false, memoryBytes: memory?.buffer?.byteLength ?? null}});
    }
  });
}

class RendererClient {
  constructor(label, module, runtimePath, maximumRenders = Number.POSITIVE_INFINITY) {
    this.label = label;
    this.module = module;
    this.runtimePath = runtimePath;
    this.maximumRenders = maximumRenders;
    this.nextID = 1;
    this.pending = new Map();
    this.generation = 0;
    this.spawn();
  }

  spawn() {
    this.generation += 1;
    this.renderCount = 0;
    const worker = new Worker(new URL(import.meta.url), {
      workerData: {label: this.label, module: this.module, runtimePath: this.runtimePath},
    });
    this.worker = worker;
    this.ready = new Promise((resolve, reject) => {
      this.readyResolve = resolve;
      this.readyReject = reject;
    });
    worker.on('message', (message) => {
      if (this.worker !== worker) return;
      if (message.type === 'ready') {
        this.rendererVersion = message.rendererVersion;
        this.readyResolve();
        return;
      }
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      pending.resolve(message.outcome);
    });
    worker.on('error', (error) => {
      if (this.worker !== worker) return;
      this.readyReject(error);
      for (const pending of this.pending.values()) pending.reject(error);
      this.pending.clear();
    });
  }

  async render(payload) {
    const started = performance.now();
    await this.ready;
    const generation = this.generation;
    const id = this.nextID++;
    const outcome = await new Promise((resolve, reject) => {
      this.pending.set(id, {resolve, reject});
      this.worker.postMessage({id, payload});
    });
    outcome.generation = generation;
    this.renderCount += 1;
    if (!outcome.ok || this.renderCount >= this.maximumRenders) await this.restart();
    outcome.elapsedMs = performance.now() - started;
    return outcome;
  }

  async restart() {
    const worker = this.worker;
    this.worker = null;
    await worker.terminate();
    this.spawn();
  }

  async close() {
    if (this.worker) await this.worker.terminate();
  }
}

function assertEquivalent(left, right, context) {
  if (left.ok !== right.ok) {
    throw new Error(`${context}: Go accepted=${left.ok}, TinyGo accepted=${right.ok}`);
  }
  if (!left.ok) return;
  if (left.manifest !== right.manifest || left.pngBytes !== right.pngBytes || left.pngSHA256 !== right.pngSHA256) {
    throw new Error(`${context}: renderer outputs differ`);
  }
}

function xorshift32(state) {
  let value = state >>> 0;
  value ^= value << 13;
  value ^= value >>> 17;
  value ^= value << 5;
  return value >>> 0;
}

function fuzzPayload(source, index) {
  let state = xorshift32(0x9e3779b9 ^ index);
  if (index === 0) return new Uint8Array();
  if (index % 4 === 1) {
    const result = Uint8Array.from(source);
    const changes = 1 + (state % 4);
    for (let change = 0; change < changes; change += 1) {
      state = xorshift32(state);
      const offset = state % result.length;
      state = xorshift32(state);
      result[offset] ^= 1 + (state & 0xff);
    }
    return result;
  }
  if (index % 4 === 2) {
    return source.slice(0, Math.max(1, state % source.length));
  }
  if (index % 4 === 3) {
    const extra = 1 + (state % 32);
    const result = new Uint8Array(source.length + extra);
    result.set(source);
    for (let offset = source.length; offset < result.length; offset += 1) {
      state = xorshift32(state);
      result[offset] = state & 0xff;
    }
    return result;
  }
  const length = 1 + (state % Math.min(source.length, 4096));
  const result = new Uint8Array(length);
  for (let offset = 0; offset < result.length; offset += 1) {
    state = xorshift32(state);
    result[offset] = state & 0xff;
  }
  return result;
}

async function main() {
  const [baseURL, sessionToken, goModule, goRuntime, tinyModule, tinyRuntime] = process.argv.slice(2);
  if (!baseURL || !sessionToken || !goModule || !goRuntime || !tinyModule || !tinyRuntime) {
    throw new Error('usage: test-paper-studio-wasm-safety.mjs BASE_URL TOKEN GO_WASM GO_EXEC TINY_WASM TINY_EXEC');
  }
  const headers = {'X-Paper-Studio-Token': sessionToken};
  const [compiledGoModule, compiledTinyGoModule] = await Promise.all([
    WebAssembly.compile(await readFile(goModule)),
    WebAssembly.compile(await readFile(tinyModule)),
  ]);
  const go = new RendererClient('Go', compiledGoModule, goRuntime);
  const maximumTinyGoRenders = Number(process.env.PAPER_STUDIO_WASM_MAX_RENDERS_PER_WORKER || 2);
  const tinygo = new RendererClient('TinyGo', compiledTinyGoModule, tinyRuntime, maximumTinyGoRenders);
  try {
    await Promise.all([go.ready, tinygo.ready]);
    if (go.rendererVersion !== tinygo.rendererVersion) throw new Error('renderer versions differ');
    const workspaceResponse = await fetch(`${baseURL}/api/workspace`, {headers});
    if (!workspaceResponse.ok) throw new Error(`workspace returned ${workspaceResponse.status}`);
    const workspace = await workspaceResponse.json();
    const pageCount = Number.isInteger(workspace.pages) ? workspace.pages : workspace.pages?.length;
    if (!Number.isInteger(pageCount) || pageCount < 0) throw new Error('workspace returned an invalid page count');
    if (pageCount === 0) {
      console.log(JSON.stringify({status: 'skipped', reason: 'fixture has no renderable pages'}));
      return;
    }
    if (!workspace.revision) throw new Error('renderable workspace has no revision');

    let firstPayload = null;
    for (let page = 1; page <= pageCount; page += 1) {
      const response = await fetch(`${baseURL}/api/page/${page}.render?revision=${encodeURIComponent(workspace.revision)}`, {headers});
      if (!response.ok) throw new Error(`page ${page} payload returned ${response.status}`);
      const payload = new Uint8Array(await response.arrayBuffer());
      firstPayload ||= payload;
      const goResult = await go.render(payload);
      const tinyResult = await tinygo.render(payload);
      assertEquivalent(goResult, tinyResult, `page ${page}`);
    }

    const fuzzCases = Number(process.env.PAPER_STUDIO_WASM_FUZZ_CASES || 0);
    for (let index = 0; index < fuzzCases; index += 1) {
      const payload = fuzzPayload(firstPayload, index);
      const goResult = await go.render(payload);
      const tinyResult = await tinygo.render(payload);
      assertEquivalent(goResult, tinyResult, `fuzz case ${index}`);
    }

    const soakRenders = Number(process.env.PAPER_STUDIO_WASM_SOAK_RENDERS || 0);
    const generationMemory = new Map();
    const tinyGoRenderTimes = [];
    let maximumWorkerGrowth = 0;
    for (let index = 0; index < soakRenders; index += 1) {
      const goResult = await go.render(firstPayload);
      const tinyResult = await tinygo.render(firstPayload);
      assertEquivalent(goResult, tinyResult, `soak render ${index}`);
      tinyGoRenderTimes.push(tinyResult.elapsedMs);
      if (!Number.isInteger(tinyResult.memoryBytes)) throw new Error('TinyGo linear memory is not observable');
      const baseline = generationMemory.get(tinyResult.generation) ?? tinyResult.memoryBytes;
      generationMemory.set(tinyResult.generation, baseline);
      maximumWorkerGrowth = Math.max(maximumWorkerGrowth, tinyResult.memoryBytes - baseline);
    }
    const maximumGrowth = Number(process.env.PAPER_STUDIO_WASM_SOAK_MAX_GROWTH || 64 * 1024 * 1024);
    if (maximumWorkerGrowth > maximumGrowth) {
      throw new Error(`TinyGo worker memory grew by ${maximumWorkerGrowth} bytes before recycling`);
    }
    const sortedRenderTimes = tinyGoRenderTimes.sort((left, right) => left - right);
    const warmP95Ms = sortedRenderTimes.length === 0 ? null : sortedRenderTimes[Math.floor((sortedRenderTimes.length - 1) * 0.95)];
    const maximumWarmP95Ms = Number(process.env.PAPER_STUDIO_WASM_SOAK_P95_MS || 100);
    if (warmP95Ms !== null && warmP95Ms > maximumWarmP95Ms) {
      throw new Error(`TinyGo recycled-worker render p95 was ${warmP95Ms}ms`);
    }
    console.log(JSON.stringify({status: 'pass', pages: pageCount, fuzzCases, soakRenders, maximumTinyGoRenders, maximumWorkerGrowth, warmP95Ms}));
  } finally {
    await Promise.all([go.close(), tinygo.close()]);
  }
}

if (isMainThread) {
  await main();
} else {
  await rendererWorker();
}
