// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import path from 'node:path';
import vm from 'node:vm';
import {inflateSync} from 'node:zlib';

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

let layoutSample;
let layoutResult;
let academicSample;
let academicResult;
for (const sample of playgroundSamples) {
  assertNoMixedBindAndText(sample);
  const result = await globalThis.PaperStudioWASM.compile({source: sample.source, data: sample.data, dataName: 'playground', page: 1});
  const samplePNG = Buffer.from(result.png || '', 'base64');
  if (!result.ok || 'svg' in result || samplePNG.length < 8 || samplePNG[0] !== 0x89 || samplePNG[1] !== 0x50 ||
      samplePNG[2] !== 0x4e || samplePNG[3] !== 0x47 || result.diagnostics?.length) {
    throw new Error(`playground sample ${JSON.stringify(sample.name)} failed: ${JSON.stringify(result)}`);
  }
  if (sample.name === 'Layout specimen') {
    layoutSample = sample;
    layoutResult = result;
  }
  if (sample.name === 'Academic article') {
    academicSample = sample;
    academicResult = result;
  }
}

if (!layoutSample) throw new Error('layout specimen sample is missing');
if (!academicSample || !academicResult) throw new Error('academic article sample is missing');
const academicPageHashes = [
  '0636267885c0fb7c3fa0cde41dcc198953ef111d776681b83e2d3c487342d2b5',
  '6d2adf863cafa775c83d09c63f2e298a6ec762eb9b3b4de956c380c93004fae5',
  'e45c77a790a5fdc51d1e692ce008419270e68135927d4b00d06033551440ec15',
  '1d2faad7a7399f019c5b332b7508acc1a5049e06661c6f64cc28db77ebaaeb61',
];
for (let page = 1; page <= academicPageHashes.length; page += 1) {
  const result = page === 1 ? academicResult : await globalThis.PaperStudioWASM.compile({
    source: academicSample.source,
    data: academicSample.data,
    dataName: 'playground',
    page,
  });
  const pngHash = createHash('sha256').update(Buffer.from(result.png || '', 'base64')).digest('hex');
  if (!result.ok || result.pages !== academicPageHashes.length || result.page !== page ||
      result.pixel_width !== 1191 || result.pixel_height !== 1684 || pngHash !== academicPageHashes[page - 1]) {
    throw new Error(`academic article page ${page} pixels changed: pages=${result.pages}, size=${result.pixel_width}x${result.pixel_height}, png=${pngHash}`);
  }
}
assertFractionBand(Buffer.from(layoutResult.png || '', 'base64'), 2, 32);
const compactLayout = await globalThis.PaperStudioWASM.compile({
  source: layoutSample.source,
  data: '{"title":"A measured page.","intro":"Alternate compact geometry for regression coverage.","columns":3,"compact":true}',
  dataName: 'playground',
  page: 1,
});
if (!compactLayout.ok || compactLayout.diagnostics?.length) {
  throw new Error(`alternate layout specimen failed: ${JSON.stringify(compactLayout)}`);
}
assertFractionBand(Buffer.from(compactLayout.png || '', 'base64'), 3, 16);

console.log(`docs WASM smoke: ${compiled.pages} page, ${playgroundSamples.length} samples, plan ${compiled.hash.slice(0, 12)}`);
process.exit(0);

function assertNoMixedBindAndText(sample) {
  const lines = sample.source.split('\n');
  for (let index = 0; index < lines.length; index += 1) {
    const bind = lines[index].match(/^(\s*)bind:/);
    if (!bind) continue;
    const propertyIndent = bind[1].length;
    for (let cursor = index + 1; cursor < lines.length; cursor += 1) {
      if (!lines[cursor].trim()) continue;
      const indent = lines[cursor].match(/^\s*/)[0].length;
      if (indent < propertyIndent) break;
      if (indent === propertyIndent && /^\s*text:/.test(lines[cursor])) {
        throw new Error(`playground sample ${JSON.stringify(sample.name)} mixes bind and text at source lines ${index + 1} and ${cursor + 1}`);
      }
    }
  }
}

function assertFractionBand(png, ratio, expectedGap) {
  const image = decodeRasterPNG(png);
  const primary = [0x11, 0x1a, 0x21, 0xff];
  const support = [0xf2, 0xee, 0xe6, 0xff];
  let best;
  for (let y = 0; y < image.height; y += 1) {
    const primaryRun = longestColorRun(image, y, primary);
    const supportRun = longestColorRun(image, y, support);
    if (!primaryRun || !supportRun || primaryRun.end > supportRun.start) continue;
    const candidate = {primary: primaryRun, support: supportRun, gap: supportRun.start - primaryRun.end};
    if (!best || primaryRun.width + supportRun.width > best.primary.width + best.support.width) best = candidate;
  }
  if (!best || Math.abs(best.gap - expectedGap) > 2 || Math.abs(best.primary.width - ratio * best.support.width) > ratio + 1) {
    throw new Error(`layout specimen fraction band is wrong: ratio=${ratio}, expected_gap=${expectedGap}, actual=${JSON.stringify(best)}`);
  }
}

function longestColorRun(image, y, color) {
  const row = y * image.rowBytes;
  let best;
  for (let x = 0; x < image.width;) {
    const offset = row + 1 + x * 4;
    if (image.raw[offset] !== color[0] || image.raw[offset + 1] !== color[1] ||
        image.raw[offset + 2] !== color[2] || image.raw[offset + 3] !== color[3]) {
      x += 1;
      continue;
    }
    const start = x;
    while (x < image.width) {
      const current = row + 1 + x * 4;
      if (image.raw[current] !== color[0] || image.raw[current + 1] !== color[1] ||
          image.raw[current + 2] !== color[2] || image.raw[current + 3] !== color[3]) break;
      x += 1;
    }
    const run = {start, end: x, width: x - start};
    if (!best || run.width > best.width) best = run;
  }
  return best;
}

function decodeRasterPNG(png) {
  if (png.length < 33 || !png.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))) {
    throw new Error('layout specimen returned an invalid PNG');
  }
  let width = 0;
  let height = 0;
  const compressed = [];
  for (let offset = 8; offset + 12 <= png.length;) {
    const length = png.readUInt32BE(offset);
    const type = png.toString('ascii', offset + 4, offset + 8);
    const data = png.subarray(offset + 8, offset + 8 + length);
    if (type === 'IHDR') {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      if (data[8] !== 8 || data[9] !== 6) throw new Error('layout specimen PNG is not 8-bit RGBA');
    } else if (type === 'IDAT') {
      compressed.push(data);
    }
    offset += length + 12;
    if (type === 'IEND') break;
  }
  const raw = inflateSync(Buffer.concat(compressed));
  const rowBytes = 1 + width * 4;
  if (!width || !height || raw.length !== rowBytes * height) throw new Error('layout specimen PNG dimensions are inconsistent');
  for (let y = 0; y < height; y += 1) {
    if (raw[y * rowBytes] !== 0) throw new Error('layout specimen PNG used an unexpected row filter');
  }
  return {width, height, rowBytes, raw};
}
