// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import assert from 'node:assert/strict';

import {
  createPlaygroundRuntimeLoader,
  formatPlaygroundRuntimeStatus,
  PlaygroundRuntimeError,
} from '../docs/.vitepress/components/playground/wasm-runtime.mjs';

async function testStreamingProgressAndSlowStatus() {
  const context = createContext({
    fetch: async () => new Response(new ReadableStream({
      start(controller) {
        controller.enqueue(Uint8Array.of(0, 1));
        setTimeout(() => {
          controller.enqueue(Uint8Array.of(2, 3));
          controller.close();
        }, 8);
      },
    }), {status: 200, headers: {'content-length': '4'}}),
  });
  const statuses = [];
  const loader = createLoader(context, {
    slowAfterMs: 1,
    onStatus: (status) => statuses.push(status),
  });

  const engine = await loader.load();
  assert.equal(engine, context.engine);
  assert.equal(loader.getSnapshot().state, 'ready');
  assert.equal(loader.getSnapshot().progress, 1);
  assert(statuses.some(({stage, loaded, total}) => stage === 'download' && loaded === 2 && total === 4));
  assert(statuses.some(({phase}) => phase === 'slow'));
  assert.match(formatPlaygroundRuntimeStatus({state: 'loading', stage: 'download', slow: false, progress: 0.5}), /50%/);
}

async function testRejectedLoadCanRetry() {
  let fetchCalls = 0;
  const context = createContext({
    fetch: async () => {
      fetchCalls += 1;
      if (fetchCalls === 1) throw new TypeError('temporary connection failure');
      return wasmResponse();
    },
  });
  const loader = createLoader(context);

  await assert.rejects(loader.load(), (error) => error instanceof PlaygroundRuntimeError && error.kind === 'network');
  assert.equal(loader.getSnapshot().error.kind, 'network');
  assert.equal(await loader.load(), context.engine);
  assert.equal(fetchCalls, 2, 'a rejected cached promise must not prevent the next load');
}

async function testFailureKinds() {
  const offline = createContext({fetch: async () => {
    throw new Error('fetch should not run while offline');
  }});
  offline.environment.navigator.onLine = false;
  await assert.rejects(createLoader(offline).load(), hasKind('offline'));

  const http = createContext({fetch: async () => new Response('', {status: 503})});
  await assert.rejects(createLoader(http).load(), (error) => (
    error.kind === 'http' && error.status === 503 && error.retryable
  ));

  const timedOut = createContext({
    fetch: () => new Promise(() => {}),
  });
  await assert.rejects(createLoader(timedOut, {timeoutMs: 5}).load(), hasKind('timeout'));

  const initialization = createContext({
    fetch: async () => wasmResponse(),
    run: () => Promise.reject(new Error('runtime stopped')),
  });
  await assert.rejects(createLoader(initialization).load(), hasKind('initialization'));
}

async function testAbortSignal() {
  const context = createContext({
    fetch: (_url, {signal}) => new Promise((_resolve, reject) => {
      rejectWhenAborted(signal, reject);
    }),
  });
  const controller = new AbortController();
  const loading = createLoader(context).load({signal: controller.signal});
  controller.abort();
  await assert.rejects(loading, hasKind('aborted'));
}

async function testPreviouslyFailedScriptIsReplaced() {
  const documentObject = new FakeDocument();
  const failed = documentObject.createElement('script');
  failed.src = '/wasm_exec.js';
  failed.dataset.paperruneRuntime = '/wasm_exec.js';
  failed.dataset.paperruneRuntimeState = 'failed';
  documentObject.scripts.push(failed);

  const context = createContext({
    documentObject,
    fetch: async () => wasmResponse(),
    installGoOnAppend: true,
  });
  const loader = createLoader(context);
  assert.equal(await loader.load(), context.engine);
  assert.equal(failed.removed, true);
  assert.equal(documentObject.appended.length, 1);
  assert.notEqual(documentObject.appended[0], failed);
}

function createLoader(context, overrides = {}) {
  return createPlaygroundRuntimeLoader({
    runtimeUrl: '/wasm_exec.js',
    wasmUrl: '/paperrune.wasm',
    environment: context.environment,
    document: context.documentObject,
    navigator: context.environment.navigator,
    fetch: context.environment.fetch,
    webAssembly: context.environment.WebAssembly,
    slowAfterMs: 100,
    timeoutMs: 1_000,
    scriptTimeoutMs: 100,
    initializationTimeoutMs: 100,
    ...overrides,
  });
}

function createContext({
  fetch,
  run,
  documentObject = new FakeDocument(),
  installGoOnAppend = false,
} = {}) {
  const engine = Object.freeze({compile() {}});
  const environment = {
    document: documentObject,
    navigator: {onLine: true},
    fetch,
    WebAssembly: {
      async instantiate(bytes) {
        assert(bytes instanceof Uint8Array);
        return {instance: {}};
      },
    },
  };

  const Go = class {
    constructor() {
      this.importObject = {};
    }

    run(instance) {
      if (run) return run(instance, environment, engine);
      environment.PaperStudioWASM = engine;
      return new Promise(() => {});
    }
  };

  if (installGoOnAppend) {
    documentObject.onAppend = (script) => {
      queueMicrotask(() => {
        environment.Go = Go;
        script.dispatchEvent(new Event('load'));
      });
    };
  } else {
    environment.Go = Go;
  }

  return {documentObject, engine, environment};
}

function wasmResponse() {
  return new Response(Uint8Array.of(0, 97, 115, 109), {
    status: 200,
    headers: {'content-length': '4'},
  });
}

function hasKind(kind) {
  return (error) => error instanceof PlaygroundRuntimeError && error.kind === kind;
}

function rejectWhenAborted(signal, reject) {
  if (signal.aborted) reject(signal.reason);
  else signal.addEventListener('abort', () => reject(signal.reason), {once: true});
}

class FakeDocument {
  constructor() {
    this.scripts = [];
    this.appended = [];
    this.onAppend = null;
    this.head = {
      appendChild: (script) => {
        if (!this.scripts.includes(script)) this.scripts.push(script);
        this.appended.push(script);
        this.onAppend?.(script);
      },
    };
  }

  createElement(tagName) {
    assert.equal(tagName, 'script');
    return new FakeScript(this);
  }

  querySelectorAll(selector) {
    assert.equal(selector, 'script[data-paperrune-runtime]');
    return this.scripts.filter(({dataset}) => dataset.paperruneRuntime);
  }
}

class FakeScript extends EventTarget {
  constructor(documentObject) {
    super();
    this.documentObject = documentObject;
    this.dataset = {};
    this.src = '';
    this.async = false;
    this.removed = false;
  }

  getAttribute(name) {
    return name === 'src' ? this.src : null;
  }

  remove() {
    this.removed = true;
    const index = this.documentObject.scripts.indexOf(this);
    if (index >= 0) this.documentObject.scripts.splice(index, 1);
  }
}

await testStreamingProgressAndSlowStatus();
await testRejectedLoadCanRetry();
await testFailureKinds();
await testAbortSignal();
await testPreviouslyFailedScriptIsReplaced();

console.log('playground WASM runtime loader: resilient loading and retry verified');
