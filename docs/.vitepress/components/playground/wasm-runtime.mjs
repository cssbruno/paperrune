const DEFAULT_SLOW_AFTER_MS = 2_000;
const DEFAULT_TIMEOUT_MS = 60_000;
const DEFAULT_SCRIPT_TIMEOUT_MS = 15_000;
const DEFAULT_INITIALIZATION_TIMEOUT_MS = 30_000;

const RETRYABLE_KINDS = new Set([
  'offline',
  'network',
  'http',
  'timeout',
  'runtime-script',
  'initialization',
]);

export class PlaygroundRuntimeError extends Error {
  constructor(kind, message, {
    stage = null,
    status = null,
    retryable = RETRYABLE_KINDS.has(kind),
    cause,
  } = {}) {
    super(message);
    this.name = 'PlaygroundRuntimeError';
    this.kind = kind;
    this.stage = stage;
    this.status = status;
    this.retryable = retryable;
    if (cause !== undefined) this.cause = cause;
  }
}

export function createPlaygroundRuntimeLoader({
  runtimeUrl,
  wasmUrl,
  slowAfterMs = DEFAULT_SLOW_AFTER_MS,
  timeoutMs = DEFAULT_TIMEOUT_MS,
  scriptTimeoutMs = DEFAULT_SCRIPT_TIMEOUT_MS,
  initializationTimeoutMs = DEFAULT_INITIALIZATION_TIMEOUT_MS,
  onStatus,
  environment = globalThis,
  document: documentObject = environment.document,
  navigator: navigatorObject = environment.navigator,
  fetch: fetchFunction = environment.fetch?.bind(environment),
  webAssembly = environment.WebAssembly,
} = {}) {
  requireURL(runtimeUrl, 'runtimeUrl');
  requireURL(wasmUrl, 'wasmUrl');

  const listeners = new Set();
  let snapshot = freezeSnapshot({
    state: 'idle',
    phase: 'idle',
    stage: null,
    slow: false,
    loaded: 0,
    total: null,
    progress: null,
    attempt: 0,
    error: null,
  });
  let attemptNumber = 0;
  let pendingPromise = null;
  let pendingController = null;
  let cachedEngine = null;

  if (typeof onStatus === 'function') listeners.add(onStatus);

  function getSnapshot() {
    return snapshot;
  }

  function subscribe(listener, {immediate = true} = {}) {
    if (typeof listener !== 'function') {
      throw new TypeError('Playground runtime status listener must be a function');
    }
    listeners.add(listener);
    if (immediate) callListener(listener, snapshot);
    return () => listeners.delete(listener);
  }

  function publish(update, attempt = attemptNumber) {
    if (attempt !== attemptNumber) return;
    snapshot = freezeSnapshot({...snapshot, ...update, attempt});
    for (const listener of listeners) callListener(listener, snapshot);
  }

  function publishLoading(stage, update = {}, attempt = attemptNumber) {
    publish({
      state: 'loading',
      phase: snapshot.slow ? 'slow' : 'loading',
      stage,
      error: null,
      ...update,
    }, attempt);
  }

  function load({signal} = {}) {
    if (signal?.aborted) return Promise.reject(abortedError(signal));

    const availableEngine = findEngine(environment, cachedEngine);
    if (availableEngine) {
      cachedEngine = availableEngine;
      publish({
        state: 'ready',
        phase: 'ready',
        stage: null,
        slow: false,
        progress: 1,
        error: null,
      });
      return Promise.resolve(availableEngine);
    }

    if (pendingPromise) return waitForCaller(pendingPromise, signal);

    const attempt = ++attemptNumber;
    const controller = new AbortController();
    pendingController = controller;
    const unlinkSignal = forwardAbort(signal, controller);
    let slowTimer;
    let timeoutTimer;

    publish({
      state: 'loading',
      phase: 'loading',
      stage: 'runtime',
      slow: false,
      loaded: 0,
      total: null,
      progress: null,
      attempt,
      error: null,
    }, attempt);

    if (slowAfterMs > 0) {
      slowTimer = setTimeout(() => {
        publish({state: 'loading', phase: 'slow', slow: true}, attempt);
      }, slowAfterMs);
    }
    if (timeoutMs > 0) {
      timeoutTimer = setTimeout(() => {
        controller.abort(new PlaygroundRuntimeError(
          'timeout',
          `Compiler loading timed out after ${formatDuration(timeoutMs)}`,
          {stage: snapshot.stage},
        ));
      }, timeoutMs);
    }

    const operation = runAttempt({
      attempt,
      signal: controller.signal,
      environment,
      documentObject,
      navigatorObject,
      fetchFunction,
      webAssembly,
      runtimeUrl,
      wasmUrl,
      scriptTimeoutMs,
      initializationTimeoutMs,
      publishLoading,
    });

    const managed = operation.then((engine) => {
      cachedEngine = engine;
      publish({
        state: 'ready',
        phase: 'ready',
        stage: null,
        slow: false,
        progress: 1,
        error: null,
      }, attempt);
      return engine;
    }).catch((error) => {
      const failure = normalizeFailure(error, {
        signal: controller.signal,
        navigatorObject,
        stage: snapshot.stage,
      });
      publish({
        state: 'error',
        phase: 'error',
        stage: failure.stage,
        slow: false,
        error: failure,
      }, attempt);
      throw failure;
    }).finally(() => {
      clearTimeout(slowTimer);
      clearTimeout(timeoutTimer);
      unlinkSignal();
      if (pendingPromise === managed) pendingPromise = null;
      if (pendingController === controller) pendingController = null;
    });

    pendingPromise = managed;
    return waitForCaller(managed, signal);
  }

  function reset() {
    attemptNumber += 1;
    if (pendingController && !pendingController.signal.aborted) {
      pendingController.abort(new PlaygroundRuntimeError(
        'aborted',
        'Compiler loading was reset',
        {stage: snapshot.stage, retryable: true},
      ));
    }
    pendingController = null;
    pendingPromise = null;
    cachedEngine = null;
    publish({
      state: 'idle',
      phase: 'idle',
      stage: null,
      slow: false,
      loaded: 0,
      total: null,
      progress: null,
      error: null,
    });
  }

  function retry(options) {
    reset();
    return load(options);
  }

  return Object.freeze({load, retry, reset, subscribe, getSnapshot});
}

export function formatPlaygroundRuntimeStatus(snapshot) {
  if (!snapshot) return 'Loading compiler…';
  if (snapshot.state === 'ready') return 'Compiler ready';
  if (snapshot.state === 'error') return snapshot.error?.message || 'Compiler loading failed';
  if (snapshot.state === 'idle') return 'Compiler not loaded';

  const prefix = snapshot.slow ? 'Still ' : '';
  switch (snapshot.stage) {
    case 'runtime':
      return `${prefix}loading WebAssembly runtime…`;
    case 'download':
      if (snapshot.progress !== null) {
        return `${prefix}downloading compiler… ${Math.round(snapshot.progress * 100)}%`;
      }
      return `${prefix}downloading compiler…`;
    case 'instantiate':
      return `${prefix}preparing compiler…`;
    case 'initialize':
      return `${prefix}starting compiler…`;
    default:
      return `${prefix}loading compiler…`;
  }
}

async function runAttempt({
  attempt,
  signal,
  environment,
  documentObject,
  navigatorObject,
  fetchFunction,
  webAssembly,
  runtimeUrl,
  wasmUrl,
  scriptTimeoutMs,
  initializationTimeoutMs,
  publishLoading,
}) {
  if (typeof fetchFunction !== 'function') {
    throw new PlaygroundRuntimeError(
      'unsupported',
      'This browser does not provide the Fetch API required by the compiler',
      {stage: 'download'},
    );
  }
  if (!webAssembly?.instantiate) {
    throw new PlaygroundRuntimeError(
      'unsupported',
      'This browser does not support WebAssembly',
      {stage: 'instantiate'},
    );
  }

  publishLoading('runtime', {}, attempt);
  await loadRuntimeScript({
    url: runtimeUrl,
    signal,
    timeoutMs: scriptTimeoutMs,
    environment,
    documentObject,
    navigatorObject,
  });

  let go;
  try {
    go = new environment.Go();
  } catch (cause) {
    throw new PlaygroundRuntimeError(
      'initialization',
      'The Go WebAssembly runtime could not be created',
      {stage: 'runtime', cause},
    );
  }

  publishLoading('download', {loaded: 0, total: null, progress: null}, attempt);
  const wasmBytes = await downloadWasm({
    url: wasmUrl,
    signal,
    fetchFunction,
    navigatorObject,
    onProgress(loaded, total, complete = false) {
      publishLoading('download', {
        loaded,
        total,
        progress: complete ? 1 : total ? Math.min(loaded / total, 1) : null,
      }, attempt);
    },
  });

  publishLoading('instantiate', {}, attempt);
  let instantiated;
  try {
    instantiated = await waitWithSignal(
      webAssembly.instantiate(wasmBytes, go.importObject),
      signal,
      'instantiate',
    );
  } catch (cause) {
    throw new PlaygroundRuntimeError(
      'instantiation',
      'The compiler WebAssembly module could not be prepared',
      {stage: 'instantiate', cause},
    );
  }

  publishLoading('initialize', {}, attempt);
  return initializeRuntime({
    go,
    instance: instantiated.instance || instantiated,
    signal,
    timeoutMs: initializationTimeoutMs,
    environment,
  });
}

async function loadRuntimeScript({
  url,
  signal,
  timeoutMs,
  environment,
  documentObject,
  navigatorObject,
}) {
  if (typeof environment.Go === 'function') return;
  if (!documentObject?.createElement || !documentObject.head?.appendChild) {
    throw new PlaygroundRuntimeError(
      'unsupported',
      'The compiler runtime requires a browser document',
      {stage: 'runtime'},
    );
  }

  let script = findRuntimeScript(documentObject, url);
  if (script && script.dataset?.paperruneRuntimeState !== 'loading') {
    removeScript(script);
    script = null;
  }

  if (!script) {
    script = documentObject.createElement('script');
    script.src = url;
    script.async = true;
    script.dataset.paperruneRuntime = url;
    script.dataset.paperruneRuntimeState = 'loading';
    const loaded = waitForRuntimeScript({
      script,
      signal,
      timeoutMs,
      environment,
      navigatorObject,
      removeOnFailure: true,
    });
    documentObject.head.appendChild(script);
    await loaded;
    return;
  }

  await waitForRuntimeScript({
    script,
    signal,
    timeoutMs,
    environment,
    navigatorObject,
    removeOnFailure: true,
  });
}

function waitForRuntimeScript({
  script,
  signal,
  timeoutMs,
  environment,
  navigatorObject,
  removeOnFailure,
}) {
  return new Promise((resolve, reject) => {
    let settled = false;
    let timer;

    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      script.removeEventListener('load', onLoad);
      script.removeEventListener('error', onError);
      signal?.removeEventListener('abort', onAbort);
      if (error) {
        script.dataset.paperruneRuntimeState = 'failed';
        if (removeOnFailure) removeScript(script);
        reject(error);
      } else {
        script.dataset.paperruneRuntimeState = 'loaded';
        resolve();
      }
    };
    const onLoad = () => {
      if (typeof environment.Go !== 'function') {
        finish(new PlaygroundRuntimeError(
          'runtime-script',
          'The WebAssembly runtime loaded without the Go API',
          {stage: 'runtime'},
        ));
        return;
      }
      finish();
    };
    const onError = (cause) => {
      const offline = navigatorObject?.onLine === false;
      finish(new PlaygroundRuntimeError(
        offline ? 'offline' : 'network',
        offline
          ? 'The compiler runtime is unavailable while the browser is offline'
          : 'The compiler runtime could not be downloaded',
        {stage: 'runtime', cause},
      ));
    };
    const onAbort = () => finish(abortedError(signal, 'runtime'));

    script.addEventListener('load', onLoad, {once: true});
    script.addEventListener('error', onError, {once: true});
    signal?.addEventListener('abort', onAbort, {once: true});
    if (timeoutMs > 0) {
      timer = setTimeout(() => {
        finish(new PlaygroundRuntimeError(
          'timeout',
          `The compiler runtime download timed out after ${formatDuration(timeoutMs)}`,
          {stage: 'runtime'},
        ));
      }, timeoutMs);
    }
    if (signal?.aborted) onAbort();
  });
}

async function downloadWasm({
  url,
  signal,
  fetchFunction,
  navigatorObject,
  onProgress,
}) {
  if (navigatorObject?.onLine === false) {
    throw new PlaygroundRuntimeError(
      'offline',
      'The compiler cannot be downloaded while the browser is offline',
      {stage: 'download'},
    );
  }

  let response;
  try {
    response = await waitWithSignal(
      fetchFunction(url, {cache: 'no-cache', signal}),
      signal,
      'download',
    );
  } catch (cause) {
    throw normalizeFailure(cause, {signal, navigatorObject, stage: 'download'});
  }

  if (!response?.ok) {
    const status = Number(response?.status) || 0;
    throw new PlaygroundRuntimeError(
      'http',
      `Compiler download failed with HTTP ${status || 'error'}`,
      {
        stage: 'download',
        status,
        retryable: status === 0 || status === 408 || status === 429 || status >= 500,
      },
    );
  }

  const contentLength = parseContentLength(response.headers?.get?.('content-length'));
  if (!response.body?.getReader) {
    try {
      const bytes = new Uint8Array(await waitWithSignal(
        response.arrayBuffer(),
        signal,
        'download',
      ));
      onProgress(bytes.byteLength, contentLength || bytes.byteLength, true);
      return bytes;
    } catch (cause) {
      throw normalizeFailure(cause, {signal, navigatorObject, stage: 'download'});
    }
  }

  const reader = response.body.getReader();
  const chunks = [];
  let loaded = 0;
  try {
    while (true) {
      const {done, value} = await waitWithSignal(reader.read(), signal, 'download');
      if (done) break;
      const chunk = value instanceof Uint8Array ? value : new Uint8Array(value);
      chunks.push(chunk);
      loaded += chunk.byteLength;
      onProgress(loaded, contentLength);
    }
  } catch (cause) {
    throw normalizeFailure(cause, {signal, navigatorObject, stage: 'download'});
  } finally {
    reader.releaseLock?.();
  }

  const bytes = new Uint8Array(loaded);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  onProgress(loaded, contentLength || loaded, true);
  return bytes;
}

async function initializeRuntime({go, instance, signal, timeoutMs, environment}) {
  let runtimeFinished = false;
  let runtimeFailure = null;
  try {
    Promise.resolve(go.run(instance)).then(
      () => {
        runtimeFinished = true;
        runtimeFailure = new PlaygroundRuntimeError(
          'initialization',
          'The compiler runtime stopped before exposing its API',
          {stage: 'initialize'},
        );
      },
      (cause) => {
        runtimeFinished = true;
        runtimeFailure = new PlaygroundRuntimeError(
          'initialization',
          'The compiler runtime stopped during initialization',
          {stage: 'initialize', cause},
        );
      },
    );
  } catch (cause) {
    throw new PlaygroundRuntimeError(
      'initialization',
      'The compiler runtime could not be started',
      {stage: 'initialize', cause},
    );
  }

  const startedAt = Date.now();
  while (true) {
    const engine = findEngine(environment);
    if (engine) return engine;
    if (signal?.aborted) throw abortedError(signal, 'initialize');
    if (runtimeFinished) throw runtimeFailure;
    if (timeoutMs > 0 && Date.now() - startedAt >= timeoutMs) {
      throw new PlaygroundRuntimeError(
        'timeout',
        `Compiler initialization timed out after ${formatDuration(timeoutMs)}`,
        {stage: 'initialize'},
      );
    }
    await abortableDelay(20, signal, 'initialize');
  }
}

function normalizeFailure(error, {signal, navigatorObject, stage}) {
  if (error instanceof PlaygroundRuntimeError) return error;
  if (signal?.aborted) return abortedError(signal, stage);
  if (navigatorObject?.onLine === false) {
    return new PlaygroundRuntimeError(
      'offline',
      'The compiler connection was interrupted while the browser was offline',
      {stage, cause: error},
    );
  }
  return new PlaygroundRuntimeError(
    'network',
    'The compiler could not be downloaded because of a network error',
    {stage, cause: error},
  );
}

function abortedError(signal, stage = null) {
  if (signal?.reason instanceof PlaygroundRuntimeError) return signal.reason;
  return new PlaygroundRuntimeError(
    'aborted',
    'Compiler loading was cancelled',
    {stage, retryable: true, cause: signal?.reason},
  );
}

function waitForCaller(promise, signal) {
  return waitWithSignal(promise, signal, null);
}

function waitWithSignal(promise, signal, stage) {
  const operation = Promise.resolve(promise);
  if (!signal) return operation;
  if (signal.aborted) {
    operation.catch(() => {});
    return Promise.reject(abortedError(signal, stage));
  }
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      cleanup();
      reject(abortedError(signal, stage));
    };
    const cleanup = () => signal.removeEventListener('abort', onAbort);
    signal.addEventListener('abort', onAbort, {once: true});
    operation.then(
      (value) => {
        cleanup();
        resolve(value);
      },
      (error) => {
        cleanup();
        reject(error);
      },
    );
  });
}

function forwardAbort(signal, controller) {
  if (!signal) return () => {};
  const onAbort = () => {
    if (!controller.signal.aborted) controller.abort(abortedError(signal));
  };
  signal.addEventListener('abort', onAbort, {once: true});
  if (signal.aborted) onAbort();
  return () => signal.removeEventListener('abort', onAbort);
}

function abortableDelay(milliseconds, signal, stage) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(abortedError(signal, stage));
      return;
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort);
      resolve();
    }, milliseconds);
    const onAbort = () => {
      clearTimeout(timer);
      signal.removeEventListener('abort', onAbort);
      reject(abortedError(signal, stage));
    };
    signal?.addEventListener('abort', onAbort, {once: true});
  });
}

function findRuntimeScript(documentObject, url) {
  const scripts = documentObject.querySelectorAll?.('script[data-paperrune-runtime]') || [];
  return [...scripts].find((script) => (
    script.dataset?.paperruneRuntime === url ||
    script.getAttribute?.('src') === url
  )) || null;
}

function removeScript(script) {
  if (typeof script.remove === 'function') script.remove();
  else script.parentNode?.removeChild?.(script);
}

function findEngine(environment, cachedEngine) {
  if (cachedEngine?.compile) return cachedEngine;
  if (environment.PaperStudioWASM?.compile) return environment.PaperStudioWASM;
  return null;
}

function parseContentLength(value) {
  const length = Number.parseInt(value || '', 10);
  return Number.isFinite(length) && length > 0 ? length : null;
}

function requireURL(value, name) {
  if (typeof value !== 'string' || !value.trim()) {
    throw new TypeError(`${name} must be a non-empty string`);
  }
}

function formatDuration(milliseconds) {
  const seconds = milliseconds / 1_000;
  return Number.isInteger(seconds) ? `${seconds} seconds` : `${seconds.toFixed(1)} seconds`;
}

function freezeSnapshot(value) {
  return Object.freeze(value);
}

function callListener(listener, value) {
  try {
    listener(value);
  } catch (error) {
    console.error('PaperRune playground runtime status listener failed', error);
  }
}
