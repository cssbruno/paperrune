const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../web/wasm-renderer.js'), 'utf8');
const rendererVersion = 'layoutengine/go-display-raster@8';

class MockWorker {
  constructor(url) {
    this.url = url;
    this.listeners = new Map();
    this.messages = [];
    this.terminated = false;
    MockWorker.instances.push(this);
  }

  addEventListener(type, listener) {
    this.listeners.set(type, listener);
  }

  postMessage(message, transfer) {
    this.messages.push({message, transfer});
  }

  terminate() {
    this.terminated = true;
  }

  emit(type, data) {
    this.listeners.get(type)?.(type === 'message' ? {data} : data);
  }
}
MockWorker.instances = [];

function renderedMessage(id, overrides = {}) {
  const png = new ArrayBuffer(4);
  return {
    type: 'rendered',
    id,
    png,
    rendererVersion,
    manifest: {
      plan_hash: 'r1', page: 1, media_type: 'image/png', png_byte_length: png.byteLength,
      pixel_width: 20, pixel_height: 30,
      identity: {renderer_version: rendererVersion},
      profile: {dpi: 144},
      page_bounds: {x: 0, y: 0, width: 10, height: 15},
    },
    ...overrides,
  };
}

function loadRenderer() {
  MockWorker.instances = [];
  const timers = [];
  const module = {};
  const response = {
    ok: true,
    clone() { return this; },
    arrayBuffer: async () => new ArrayBuffer(8),
  };
  const context = {
    Worker: MockWorker,
    Promise, Error, Map, Object, ArrayBuffer, Blob,
    Number,
    fetch: async () => response,
    WebAssembly: {
      compileStreaming: async () => module,
      compile: async () => module,
    },
    createImageBitmap: async () => ({width: 20, height: 30, close() {}}),
    setTimeout(callback) { timers.push(callback); return timers.length; },
    clearTimeout() {},
  };
  context.globalThis = context;
  vm.runInNewContext(source, context, {filename: 'wasm-renderer.js'});
  return {renderer: context.PaperStudioWASMRenderer, worker: MockWorker.instances[0], timers};
}

async function initialize(worker) {
  await new Promise(setImmediate);
  assert.equal(worker.messages[0].message.type, 'init');
  worker.emit('message', {type: 'ready', rendererVersion});
  await new Promise(setImmediate);
}

test('compiles on the main thread and validates returned page evidence', async () => {
  const {renderer, worker} = loadRenderer();
  assert.equal(worker.url, '/wasm-renderer-worker.js');
  await initialize(worker);
  const payload = Uint8Array.from([1, 2, 3, 4]).buffer;
  const pending = renderer.renderResponse({arrayBuffer: async () => payload}, {revision: 'r1', page: 1, dpi: 144});
  await new Promise(setImmediate);
  const posted = worker.messages.at(-1);
  assert.equal(posted.message.type, 'render');
  assert.equal(posted.message.payload, payload);
  assert.equal(posted.transfer[0], payload);
  worker.emit('message', renderedMessage(posted.message.id));
  const rendered = await pending;
  assert.equal(rendered.bitmap.width, 20);
  assert.deepEqual([...rendered.viewBox], [0, 0, 10, 15]);
});

test('rejects invalid worker evidence and terminates the worker', async () => {
  const {renderer, worker} = loadRenderer();
  await initialize(worker);
  const pending = renderer.renderResponse({arrayBuffer: async () => new ArrayBuffer(1)}, {revision: 'r1', page: 1, dpi: 144});
  await new Promise(setImmediate);
  const id = worker.messages.at(-1).message.id;
  worker.emit('message', renderedMessage(id, {rendererVersion: 'compromised'}));
  await assert.rejects(pending, /stale or invalid page evidence/);
  assert.equal(worker.terminated, true);
});

test('keeps at most one active render and one latest replacement', async () => {
  const {renderer, worker} = loadRenderer();
  await initialize(worker);
  const expected = {revision: 'r1', page: 1, dpi: 144};
  const first = renderer.renderResponse({arrayBuffer: async () => new ArrayBuffer(1)}, expected);
  await new Promise(setImmediate);
  const second = renderer.renderResponse({arrayBuffer: async () => new ArrayBuffer(2)}, expected);
  await new Promise(setImmediate);
  const third = renderer.renderResponse({arrayBuffer: async () => new ArrayBuffer(3)}, expected);
  await assert.rejects(second, (error) => error.name === 'AbortError' && error.superseded === true);
  const firstID = worker.messages.at(-1).message.id;
  worker.emit('message', renderedMessage(firstID));
  await first;
  await new Promise(setImmediate);
  const thirdPost = worker.messages.at(-1);
  assert.equal(thirdPost.message.payload.byteLength, 3);
  worker.emit('message', renderedMessage(thirdPost.message.id));
  await third;
});

test('watchdog terminates a stuck worker and rejects pending work', async () => {
  const {renderer, worker, timers} = loadRenderer();
  await initialize(worker);
  const pending = renderer.renderResponse({arrayBuffer: async () => new ArrayBuffer(1)}, {revision: 'r1', page: 1, dpi: 144});
  await new Promise(setImmediate);
  timers.at(-1)();
  await assert.rejects(pending, (error) => error.name === 'TimeoutError');
  assert.equal(worker.terminated, true);
  assert.equal(MockWorker.instances.length, 2);
});
