(function (root) {
  'use strict';

  const RENDERER_VERSION = 'layoutengine/go-display-raster@7';
  const RENDER_TIMEOUT_MS = 15000;
  const MAX_PIXEL_DIMENSION = 16384;
  const MAX_PIXELS = 64 * 1024 * 1024;

  let workerState = null;
  let active = null;
  let replacement = null;
  let requestID = 0;
  let latestRequestID = 0;

  const modulePromise = compileRendererModule();

  async function compileRendererModule() {
    const response = await fetch('/paper-studio.wasm', {cache: 'no-cache', credentials: 'omit'});
    if (!response.ok) throw new Error(`WASM renderer unavailable (${response.status})`);
    if (WebAssembly.compileStreaming) {
      try {
        return await WebAssembly.compileStreaming(response.clone());
      } catch (_) {
        // Keep a same-origin fallback for servers that report the wrong media type.
      }
    }
    return WebAssembly.compile(await response.arrayBuffer());
  }

  function ensureWorker() {
    if (workerState) return workerState;
    const worker = new Worker('/wasm-renderer-worker.js');
    let resolveReady;
    let rejectReady;
    const ready = new Promise((resolve, reject) => {
      resolveReady = resolve;
      rejectReady = reject;
    });
    const state = {worker, ready, resolveReady, rejectReady, initialized: false};
    workerState = state;
    worker.addEventListener('message', (event) => handleWorkerMessage(state, event.data));
    worker.addEventListener('error', (event) => {
      failWorker(state, new Error(event.message || 'WASM worker crashed'));
    });
    modulePromise.then((module) => {
      if (workerState === state) worker.postMessage({type: 'init', module});
    }).catch((error) => failWorker(state, error));
    return state;
  }

  function handleWorkerMessage(state, message) {
    if (workerState !== state) {
      message?.bitmap?.close?.();
      return;
    }
    if (message?.type === 'ready') {
      if (message.rendererVersion !== RENDERER_VERSION) {
        failWorker(state, new Error('WASM worker initialized an unexpected renderer version'));
        return;
      }
      state.initialized = true;
      state.resolveReady();
      return;
    }
    if (message?.type === 'fatal') {
      failWorker(state, new Error(message.error || 'WASM worker failed'));
      return;
    }
    if (!active || message?.id !== active.id) return;
    const request = active;
    if (message.type !== 'error' && message.type !== 'rendered') {
      failWorker(state, new Error('WASM worker returned an unexpected message'));
      return;
    }
    clearTimeout(request.timer);
    if (message.type === 'error') {
      active = null;
      const error = new Error(message.error || 'WASM render failed');
      if (message.status) error.status = message.status;
      request.reject(error);
      startReplacement();
      return;
    }
    if (message.type !== 'rendered') return;
    finishRendered(state, request, message);
  }

  async function finishRendered(state, request, message) {
    try {
      const result = await validateRendered(message, request.expected);
      if (workerState !== state || active !== request) {
        result.bitmap.close();
        return;
      }
      active = null;
      request.resolve(result);
      startReplacement();
    } catch (error) {
      failWorker(state, error);
    }
  }

  async function validateRendered(message, expected) {
    const manifest = message?.manifest;
    const width = Number(manifest?.pixel_width);
    const height = Number(manifest?.pixel_height);
    if (!manifest || message.rendererVersion !== RENDERER_VERSION ||
        manifest.identity?.renderer_version !== RENDERER_VERSION ||
        manifest.plan_hash !== expected.revision || manifest.page !== expected.page ||
        manifest.media_type !== 'image/png' || manifest.profile?.dpi !== expected.dpi ||
        !Number.isSafeInteger(width) || !Number.isSafeInteger(height) || width <= 0 || height <= 0 ||
        width > MAX_PIXEL_DIMENSION || height > MAX_PIXEL_DIMENSION || width * height > MAX_PIXELS ||
        !(message.png instanceof ArrayBuffer) || manifest.png_byte_length !== message.png.byteLength) {
      throw new Error('WASM renderer returned stale or invalid page evidence');
    }
    const bounds = manifest.page_bounds;
    if (![bounds?.x || 0, bounds?.y || 0, bounds?.width, bounds?.height].every(Number.isFinite) ||
        !(bounds.width > 0 && bounds.height > 0)) {
      throw new Error('WASM renderer returned invalid page dimensions');
    }
    const bitmap = await createImageBitmap(new Blob([message.png], {type: 'image/png'}));
    if (bitmap.width !== width || bitmap.height !== height) {
      bitmap.close();
      throw new Error('WASM renderer pixel dimensions do not match its manifest');
    }
    return {
      bitmap,
      manifest,
      viewBox: [bounds.x || 0, bounds.y || 0, bounds.width, bounds.height],
      pixelWidth: width,
      pixelHeight: height,
    };
  }

  function failWorker(state, error) {
    if (workerState !== state) return;
    state.rejectReady(error);
    state.worker.terminate();
    workerState = null;
    rejectAll(error);
  }

  function rejectAll(error) {
    if (active) {
      clearTimeout(active.timer);
      active.reject(error);
      active = null;
    }
    if (replacement) {
      replacement.reject(error);
      replacement = null;
    }
  }

  function supersededError() {
    const error = new Error('WASM render was superseded by a newer request');
    error.name = 'AbortError';
    error.superseded = true;
    return error;
  }

  function enqueue(request) {
    if (!active) {
      start(request);
      return;
    }
    if (replacement) replacement.reject(supersededError());
    replacement = request;
  }

  function start(request) {
    active = request;
    request.timer = setTimeout(() => {
      if (active !== request) return;
      const error = new Error(`WASM render exceeded ${RENDER_TIMEOUT_MS}ms and the worker was restarted`);
      error.name = 'TimeoutError';
      const state = workerState;
      if (state) {
        failWorker(state, error);
        ensureWorker();
      } else {
        rejectAll(error);
      }
    }, RENDER_TIMEOUT_MS);
    const state = ensureWorker();
    state.ready.then(() => {
      if (active !== request || workerState !== state) return;
      state.worker.postMessage({type: 'render', id: request.id, payload: request.payload, expected: request.expected}, [request.payload]);
    }).catch((error) => failWorker(state, error));
  }

  function startReplacement() {
    if (!replacement) return;
    const next = replacement;
    replacement = null;
    start(next);
  }

  async function renderResponse(response, expected) {
    const id = ++requestID;
    latestRequestID = id;
    const payload = await response.arrayBuffer();
    if (id !== latestRequestID) throw supersededError();
    return new Promise((resolve, reject) => enqueue({id, payload, expected, resolve, reject, timer: 0}));
  }

  const ready = ensureWorker().ready;
  root.PaperStudioWASMRenderer = Object.freeze({ready, renderResponse});
})(globalThis);
