(function (root) {
  'use strict';

  let ready = null;
  let runtimeFailure = null;
  let rendering = false;

  async function initialize(module) {
    if (!(module instanceof WebAssembly.Module)) throw new Error('WASM worker requires a precompiled module');
    const go = new Go();
    const instance = await WebAssembly.instantiate(module, go.importObject);
    go.run(instance).catch((error) => { runtimeFailure = error; });
    for (let attempt = 0; attempt < 100 && !root.PaperStudioWASM; attempt += 1) {
      await new Promise((resolve) => setTimeout(resolve, 0));
    }
    if (runtimeFailure) throw runtimeFailure;
    if (!root.PaperStudioWASM?.render) throw new Error('Go WASM renderer did not initialize');
    root.postMessage({type: 'ready', rendererVersion: root.PaperStudioWASM.rendererVersion});
    return root.PaperStudioWASM;
  }

  function fail(id, error) {
    root.postMessage({
      type: 'error',
      id,
      error: String(error?.message || error || 'WASM worker failed'),
      status: Number(error?.status || 0),
    });
  }

  async function render(message) {
    if (rendering) throw new Error('WASM worker received concurrent render jobs');
    rendering = true;
    try {
      const engine = await ready;
      const result = await engine.render(new Uint8Array(message.payload));
      const manifest = result?.manifest;
      const expected = message.expected;
      if (!manifest || manifest.plan_hash !== expected.revision || manifest.page !== expected.page ||
          manifest.identity?.renderer_version !== engine.rendererVersion || manifest.media_type !== 'image/png' ||
          manifest.profile?.dpi !== expected.dpi) {
        throw new Error('WASM renderer returned stale or invalid page evidence');
      }
      const png = result.png.slice().buffer;
      root.postMessage({
        type: 'rendered',
        id: message.id,
        png,
        manifest,
        rendererVersion: engine.rendererVersion,
      }, [png]);
    } finally {
      rendering = false;
    }
  }

  root.onmessage = (event) => {
    const message = event.data;
    if (message?.type === 'init' && !ready) {
      ready = initialize(message.module);
      ready.catch((error) => root.postMessage({type: 'fatal', error: String(error?.message || error)}));
      return;
    }
    if (message?.type === 'render') render(message).catch((error) => fail(message.id, error));
  };
})(globalThis);
