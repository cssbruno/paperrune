// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import {readFile} from 'node:fs/promises';
import path from 'node:path';
import vm from 'node:vm';

import {playgroundSamples} from '../docs/.vitepress/playground-samples.mjs';

const outputDirectory = path.resolve(process.argv[2] || 'docs/.vitepress/dist');
const runtimePath = path.join(outputDirectory, 'wasm_exec.js');
const modulePath = path.join(outputDirectory, 'paperrune.wasm');

vm.runInThisContext(await readFile(runtimePath, 'utf8'), {filename: runtimePath});

const go = new Go();
const module = await WebAssembly.instantiate(await readFile(modulePath), go.importObject);
let runtimeFailure;
go.run(module.instance).catch((error) => { runtimeFailure = error; });
for (let attempt = 0; attempt < 200 && !globalThis.PaperStudioWASM?.compile; attempt += 1) {
  await new Promise((resolve) => setTimeout(resolve, 5));
}
if (runtimeFailure) throw runtimeFailure;
if (!globalThis.PaperStudioWASM?.compile) throw new Error('documentation compiler did not initialize');

const source = `document @smoke:
  schema input:
    string name
    bool active
  page:
    size: "A4"
    margin: 36pt
    body:
      heading:
        visible: active
        text: name
`;
const compiled = await globalThis.PaperStudioWASM.compile({
  source,
  data: '{"name":"WASM documentation smoke","active":true}',
  dataName: 'input',
  page: 1,
});
const renderedPNG = Buffer.from(compiled.png || '', 'base64');
if (!compiled.ok || 'svg' in compiled || compiled.pages !== 1 || compiled.page !== 1 || !compiled.hash ||
    compiled.renderer !== globalThis.PaperStudioWASM.rendererVersion || compiled.dpi <= 0 ||
    compiled.pixel_width <= 0 || compiled.pixel_height <= 0 || renderedPNG.length < 8 ||
    renderedPNG[0] !== 0x89 || renderedPNG[1] !== 0x50 || renderedPNG[2] !== 0x4e || renderedPNG[3] !== 0x47) {
  throw new Error(`documentation compiler returned invalid evidence: ${JSON.stringify(compiled)}`);
}

const invalid = await globalThis.PaperStudioWASM.compile({
  source: source.replace('text: name', 'text: missing'),
  data: '{"name":"WASM documentation smoke","active":true}',
  dataName: 'input',
  page: 1,
});
if (invalid.ok || !invalid.diagnostics?.some((diagnostic) => diagnostic.code === 'PAPER_EXPRESSION_PATH')) {
  throw new Error(`documentation compiler did not preserve diagnostics: ${JSON.stringify(invalid)}`);
}

for (const sample of playgroundSamples) {
  const result = await globalThis.PaperStudioWASM.compile({source: sample.source, data: sample.data, dataName: 'playground', page: 1});
  const samplePNG = Buffer.from(result.png || '', 'base64');
  if (!result.ok || 'svg' in result || samplePNG.length < 8 || samplePNG[0] !== 0x89 || samplePNG[1] !== 0x50 ||
      samplePNG[2] !== 0x4e || samplePNG[3] !== 0x47 || result.diagnostics?.length) {
    throw new Error(`playground sample ${JSON.stringify(sample.name)} failed: ${JSON.stringify(result)}`);
  }
}

console.log(`docs WASM smoke: ${compiled.pages} page, ${playgroundSamples.length} samples, plan ${compiled.hash.slice(0, 12)}`);
process.exit(0);
