// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

import {createHash} from 'node:crypto';
import {readFile, readdir} from 'node:fs/promises';
import path from 'node:path';
import vm from 'node:vm';
import {inflateSync} from 'node:zlib';

import {playgroundSampleManifest} from '../docs/.vitepress/playground-samples.mjs';

const outputDirectory = path.resolve(process.argv[2] || 'docs/.vitepress/dist');
const runtimePath = path.join(outputDirectory, 'wasm_exec.js');
const modulePath = path.join(outputDirectory, 'paperrune.wasm');
const sampleDirectory = new URL('../docs/.vitepress/playground-samples/', import.meta.url);
const expectedSampleFiles = playgroundSampleManifest.flatMap(({slug}) => [`${slug}.json`, `${slug}.paper`]).sort();
const actualSampleFiles = (await readdir(sampleDirectory)).filter((name) => name.endsWith('.json') || name.endsWith('.paper')).sort();
if (new Set(playgroundSampleManifest.map(({slug}) => slug)).size !== playgroundSampleManifest.length ||
    new Set(playgroundSampleManifest.map(({name}) => name)).size !== playgroundSampleManifest.length ||
    JSON.stringify(actualSampleFiles) !== JSON.stringify(expectedSampleFiles)) {
  throw new Error(`playground sample manifest does not match files: expected=${JSON.stringify(expectedSampleFiles)} actual=${JSON.stringify(actualSampleFiles)}`);
}
const playgroundSamples = await Promise.all(playgroundSampleManifest.map(async (sample) => ({
  ...sample,
  source: await readFile(new URL(`${sample.slug}.paper`, sampleDirectory), 'utf8'),
  data: await readFile(new URL(`${sample.slug}.json`, sampleDirectory), 'utf8'),
})));

vm.runInThisContext(await readFile(runtimePath, 'utf8'), {filename: runtimePath});

const go = new Go();
const module = await WebAssembly.instantiate(await readFile(modulePath), go.importObject);
let runtimeFailure;
go.run(module.instance).catch((error) => { runtimeFailure = error; });
for (let attempt = 0; attempt < 200 && !globalThis.PaperStudioWASM?.compile; attempt += 1) {
  await new Promise((resolve) => setTimeout(resolve, 5));
}
if (runtimeFailure) throw runtimeFailure;
if (!globalThis.PaperStudioWASM?.compile || !globalThis.PaperStudioWASM?.hit ||
    !globalThis.PaperStudioWASM?.trace || !globalThis.PaperStudioWASM?.editText ||
    !globalThis.PaperStudioWASM?.workspacePage || !globalThis.PaperStudioWASM?.mountEditor ||
    !globalThis.PaperStudioWASM?.mountFileEditor || !globalThis.PaperStudioWASM?.paintPage) {
  throw new Error('documentation compiler did not initialize the Studio API');
}

const source = `document @smoke:
  schema input:
    string name
    bool active
  page:
    size: "A4"
    margin: 36pt
    body:
      heading @title:
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
if (!compiled.ok || compiled.pages !== 1 || compiled.page !== 1 || !compiled.hash ||
    compiled.renderer !== globalThis.PaperStudioWASM.rendererVersion || compiled.dpi <= 0 ||
    compiled.pixel_width <= 0 || compiled.pixel_height <= 0 || renderedPNG.length < 8 ||
    renderedPNG[0] !== 0x89 || renderedPNG[1] !== 0x50 || renderedPNG[2] !== 0x4e || renderedPNG[3] !== 0x47 ||
    typeof compiled.svg !== 'string' || !compiled.svg.includes('<svg ') ||
    !Number.isInteger(compiled.page_width_fixed) || compiled.page_width_fixed <= 0 ||
    !Number.isInteger(compiled.page_height_fixed) || compiled.page_height_fixed <= 0 ||
    compiled.fixed_scale !== 1024 || !/^[0-9a-f]{64}$/.test(compiled.source_revision || '') ||
    compiled.ast?.schema_version !== 2 || compiled.ast?.grammar_version !== 'paper/0.4' ||
    compiled.ast?.root?.id !== '@smoke') {
  throw new Error(`documentation compiler returned invalid evidence: ${JSON.stringify(compiled)}`);
}
const vectorCompiled = await globalThis.PaperStudioWASM.compile({
  source,
  data: '{"name":"WASM documentation smoke","active":true}',
  dataName: 'input',
  page: 1,
  vectorOnly: true,
});
const vectorSmokeRun = vectorCompiled.text_runs?.find((run) => run.text === 'WASM documentation smoke');
if (!vectorCompiled.ok || Object.hasOwn(vectorCompiled, 'svg') || Object.hasOwn(vectorCompiled, 'png') ||
    Object.hasOwn(vectorCompiled, 'pixel_width') || Object.hasOwn(vectorCompiled, 'pixel_height') ||
    !vectorSmokeRun || !vectorSmokeRun.font_family?.includes('Helvetica') ||
    !vectorSmokeRun.font_weight || !vectorSmokeRun.font_style ||
    !(vectorSmokeRun.width_fixed > 0) || !(vectorSmokeRun.font_size_fixed > 0) ||
    !Array.isArray(vectorCompiled.fonts || [])) {
  throw new Error(`documentation compiler rasterized a vector-only page: ${JSON.stringify(vectorCompiled)}`);
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

const editableSource = `document @editable:
  page @sheet:
    size: "A4"
    margin: 36pt
    body @body:
      heading @editable-title:
        level: 1
        text: "Editable title"
      paragraph @editable-paragraph:
        text: "Editable paragraph"
      text @editable-text: "Editable text"
      page-break:
      paragraph @second-page:
        text: "Second page"
`;
const editableFirst = await globalThis.PaperStudioWASM.compile({
  source: editableSource,
  data: '',
  dataName: 'playground',
  page: 1,
});
const editableSecond = await globalThis.PaperStudioWASM.compile({
  source: editableSource,
  data: '',
  dataName: 'playground',
  page: 2,
});
if (!editableFirst.ok || !editableSecond.ok || editableFirst.pages !== 2 ||
    editableSecond.pages !== 2 || editableFirst.hash !== editableSecond.hash) {
  throw new Error(`adjacent editable pages failed: first=${JSON.stringify(editableFirst)} second=${JSON.stringify(editableSecond)}`);
}
const editableHit = await globalThis.PaperStudioWASM.hit({
  hash: editableFirst.hash,
  page: 1,
  x_fixed: 40 * 1024,
  y_fixed: 40 * 1024,
});
if (editableHit.Page !== 1 || !editableHit.Fragments?.some((fragment) => fragment.Key === '@editable-title')) {
  throw new Error(`retained plan hit did not resolve editable title: ${JSON.stringify(editableHit)}`);
}
const edited = await globalThis.PaperStudioWASM.editText({
  hash: editableFirst.hash,
  target: '@editable-title',
  text: 'Edited "title"\\nnext',
});
if (!edited.applied || !edited.ok || edited.source === editableSource ||
    !/^[0-9a-f]{64}$/.test(edited.source_revision || '') || edited.hash === editableFirst.hash ||
    !edited.source.includes('text: "Edited \\"title\\"\\\\nnext"') ||
    Object.hasOwn(edited, 'png') || Object.hasOwn(edited, 'svg')) {
  throw new Error(`authored text edit did not return an image-independent WASM workspace: ${JSON.stringify(edited)}`);
}
const editedPage = await globalThis.PaperStudioWASM.workspacePage({hash: edited.hash, page: 1});
if (!editedPage.ok || editedPage.hash !== edited.hash || !editedPage.png || !editedPage.svg) {
  throw new Error(`edited WASM workspace did not render independently: ${JSON.stringify(editedPage)}`);
}
const paragraphEdited = await globalThis.PaperStudioWASM.editText({
  hash: editableFirst.hash,
  target: '@editable-paragraph',
  text: 'Edited paragraph',
});
const textEdited = await globalThis.PaperStudioWASM.editText({
  hash: editableFirst.hash,
  target: '@editable-text',
  text: 'Edited text',
});
if (!paragraphEdited.applied || !paragraphEdited.source.includes('text: "Edited paragraph"') ||
    !textEdited.applied || !textEdited.source.includes('text @editable-text: "Edited text"')) {
  throw new Error(`paragraph/text literal edits failed: paragraph=${JSON.stringify(paragraphEdited)} text=${JSON.stringify(textEdited)}`);
}
let boundEditError = '';
try {
  await globalThis.PaperStudioWASM.editText({
    hash: compiled.hash,
    target: '@title',
    text: 'Must not replace JSON data',
  });
} catch (error) {
  boundEditError = error instanceof Error ? error.message : String(error);
}
if (!/data-bound|expression-backed/.test(boundEditError)) {
  throw new Error(`data-bound edit was not rejected: ${JSON.stringify(boundEditError)}`);
}

const editableCellSource = `document @editable-cells:
  schema playground:
    string variable
  page @sheet:
    size: "A4"
    margin: 36pt
    body @body:
      table @editable-table:
        table-column:
          width: 34%
        table-column:
          width: 33%
        table-column:
          width: 33%
        table-row:
          cell @addressed-cell:
            text: "Addressed cell"
          cell:
            text: "Anonymous cell"
          cell @bound-cell:
            bind: "variable"
`;
const editableCellCompiled = await globalThis.PaperStudioWASM.compile({
  source: editableCellSource,
  data: '{"variable":"JSON-bound cell"}',
  dataName: 'playground',
  page: 1,
});
if (!editableCellCompiled.ok) {
  throw new Error(`editable cell fixture failed: ${JSON.stringify(editableCellCompiled)}`);
}
const anonymousCellText = findASTNode(editableCellCompiled.ast?.root, (node) =>
  node.kind === 'text' && !node.id && node.value?.string_value === 'Anonymous cell');
if (!Number.isInteger(anonymousCellText?.header_span?.start?.offset)) {
  throw new Error(`anonymous cell text source offset is missing: ${JSON.stringify(anonymousCellText)}`);
}
const addressedCellEdited = await globalThis.PaperStudioWASM.editText({
  hash: editableCellCompiled.hash,
  target: '@addressed-cell',
  text: 'Edited addressed cell',
});
const anonymousCellEdited = await globalThis.PaperStudioWASM.editText({
  hash: editableCellCompiled.hash,
  sourceOffset: anonymousCellText.header_span.start.offset,
  text: 'Edited anonymous cell',
});
if (!addressedCellEdited.applied || !addressedCellEdited.source.includes('text: "Edited addressed cell"') ||
    !anonymousCellEdited.applied || !anonymousCellEdited.source.includes('text: "Edited anonymous cell"')) {
  throw new Error(`cell literal edits failed: addressed=${JSON.stringify(addressedCellEdited)} anonymous=${JSON.stringify(anonymousCellEdited)}`);
}
const boundCellEdited = await globalThis.PaperStudioWASM.editText({
  hash: editableCellCompiled.hash,
  jsonPointer: '/variable',
  text: 'Edited directly in WASM',
});
if (!boundCellEdited.applied ||
    JSON.parse(boundCellEdited.data).variable !== 'Edited directly in WASM' ||
    boundCellEdited.hash === editableCellCompiled.hash) {
  throw new Error(`JSON-bound cell edit did not run in WASM: ${JSON.stringify(boundCellEdited)}`);
}

let layoutSample;
let layoutResult;
let academicSample;
let academicResults;
let renderedSamplePages = 0;
for (const sample of playgroundSamples) {
  assertNoMixedBindAndText(sample);
  const first = await globalThis.PaperStudioWASM.compile({
    source: sample.source,
    data: sample.data,
    dataName: 'playground',
    page: 1,
  });
  if (!Number.isInteger(first.pages) || first.pages < 1) {
    throw new Error(`playground sample ${JSON.stringify(sample.slug)} returned an invalid page count: ${JSON.stringify(first)}`);
  }
  const results = [first];
  assertCompiledSamplePage(sample, first, 1, first.pages);
  if (sample.slug === 'laboratory-report') {
    const pointer = await assertLaboratoryRepeatTrace(first);
    const repeatedEdit = await globalThis.PaperStudioWASM.editText({
      hash: first.hash,
      jsonPointer: pointer,
      text: 'White blood cells',
    });
    if (!repeatedEdit.applied || JSON.parse(repeatedEdit.data).results[0].analyte !== 'White blood cells') {
      throw new Error(`repeated JSON binding edit did not run in WASM: ${JSON.stringify(repeatedEdit)}`);
    }
  }
  for (let page = 2; page <= first.pages; page += 1) {
    const result = await globalThis.PaperStudioWASM.compile({
      source: sample.source,
      data: sample.data,
      dataName: 'playground',
      page,
    });
    assertCompiledSamplePage(sample, result, page, first.pages, first.hash);
    results.push(result);
  }
  renderedSamplePages += results.length;
  if (sample.slug === 'layout-specimen') {
    layoutSample = sample;
    layoutResult = first;
  }
  if (sample.slug === 'academic-article') {
    academicSample = sample;
    academicResults = results;
  }
}

if (!layoutSample) throw new Error('layout specimen sample is missing');
if (!academicSample || !academicResults) throw new Error('academic article sample is missing');
const academicPageHashes = [
  '0636267885c0fb7c3fa0cde41dcc198953ef111d776681b83e2d3c487342d2b5',
  '6d2adf863cafa775c83d09c63f2e298a6ec762eb9b3b4de956c380c93004fae5',
  'e45c77a790a5fdc51d1e692ce008419270e68135927d4b00d06033551440ec15',
  '1d2faad7a7399f019c5b332b7508acc1a5049e06661c6f64cc28db77ebaaeb61',
];
if (academicResults.length !== academicPageHashes.length) {
  throw new Error(`academic article page count changed: expected=${academicPageHashes.length} actual=${academicResults.length}`);
}
for (let page = 1; page <= academicPageHashes.length; page += 1) {
  const result = academicResults[page - 1];
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

console.log(`docs WASM smoke: ${compiled.pages} smoke page, ${playgroundSamples.length} samples / ${renderedSamplePages} pages, plan ${compiled.hash.slice(0, 12)}`);
process.exit(0);

function assertNoMixedBindAndText(sample) {
  const lines = sample.source.split('\n');
  for (let index = 0; index < lines.length; index += 1) {
    const bind = lines[index].match(/^(\s*)bind:/);
    if (!bind) continue;
    const propertyIndent = bind[1].length;
    for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
      if (mixedProperty(lines[cursor], propertyIndent)) {
        throw new Error(`playground sample ${JSON.stringify(sample.slug)} mixes bind and text at source lines ${index + 1} and ${cursor + 1}`);
      }
      if (propertyBlockEnded(lines[cursor], propertyIndent)) break;
    }
    for (let cursor = index + 1; cursor < lines.length; cursor += 1) {
      if (mixedProperty(lines[cursor], propertyIndent)) {
        throw new Error(`playground sample ${JSON.stringify(sample.slug)} mixes bind and text at source lines ${index + 1} and ${cursor + 1}`);
      }
      if (propertyBlockEnded(lines[cursor], propertyIndent)) break;
    }
  }
}

function mixedProperty(line, propertyIndent) {
  return lineIndent(line) === propertyIndent && /^\s*text:/.test(line);
}

function propertyBlockEnded(line, propertyIndent) {
  return line.trim() !== '' && lineIndent(line) < propertyIndent;
}

function lineIndent(line) {
  return line.match(/^\s*/)[0].length;
}

function assertCompiledSamplePage(sample, result, page, pages, expectedHash = result.hash) {
  const samplePNG = Buffer.from(result.png || '', 'base64');
  if (!result.ok || result.page !== page || result.pages !== pages || result.hash !== expectedHash ||
      typeof result.svg !== 'string' || !result.svg.includes('<svg ') ||
      !result.ast?.root || !/^[0-9a-f]{64}$/.test(result.source_revision || '') ||
      !Number.isInteger(result.page_x_fixed) || !Number.isInteger(result.page_y_fixed) ||
      !Number.isInteger(result.page_width_fixed) || result.page_width_fixed <= 0 ||
      !Number.isInteger(result.page_height_fixed) || result.page_height_fixed <= 0 ||
      result.fixed_scale !== 1024 || samplePNG.length < 8 ||
      samplePNG[0] !== 0x89 || samplePNG[1] !== 0x50 || samplePNG[2] !== 0x4e || samplePNG[3] !== 0x47 ||
      result.diagnostics?.length) {
    throw new Error(`playground sample ${JSON.stringify(sample.slug)} page ${page}/${pages} failed: ${JSON.stringify(result)}`);
  }
}

async function assertLaboratoryRepeatTrace(result) {
  const line = svgTextLines(result.svg).find((candidate) => candidate.text === 'Leukocytes');
  if (!line) {
    throw new Error('laboratory report SVG does not contain a Leukocytes text line');
  }
  const hit = await globalThis.PaperStudioWASM.hit({
    hash: result.hash,
    page: result.page,
    x_fixed: Math.trunc(line.x + 1),
    y_fixed: Math.trunc(line.y - 1),
  });
  if (hit.Page !== result.page || !hit.Fragments?.length) {
    throw new Error(`laboratory Leukocytes hit returned no fragment: line=${JSON.stringify(line)} hit=${JSON.stringify(hit)}`);
  }
  const traces = [];
  for (const fragment of hit.Fragments) {
    if (!Number.isInteger(fragment.Fragment) || fragment.Fragment < 1) continue;
    try {
      const trace = await globalThis.PaperStudioWASM.trace({
        hash: result.hash,
        fragment: fragment.Fragment,
      });
      traces.push(trace);
      const binding = trace.provenance?.bindings?.find((candidate) =>
        candidate.json_pointer === '/results/0/analyte');
      if (trace.plan_hash === result.hash && trace.source_revision === result.source_revision && binding) {
        return binding.json_pointer;
      }
    } catch (error) {
      traces.push({error: error instanceof Error ? error.message : String(error)});
    }
  }
  throw new Error(`laboratory Leukocytes trace did not resolve /results/0/analyte: hit=${JSON.stringify(hit)} traces=${JSON.stringify(traces)}`);
}

function svgTextLines(svg) {
  const lines = new Map();
  for (const match of svg.matchAll(/<text\b([^>]*)>([^<]*)<\/text>/g)) {
    const x = match[1].match(/\bx="([^"]+)"/);
    const y = match[1].match(/\by="([^"]+)"/);
    if (!x || !y || !Number.isFinite(Number(x[1])) || !Number.isFinite(Number(y[1]))) continue;
    const glyph = {x: Number(x[1]), y: Number(y[1]), text: match[2]};
    const key = String(glyph.y);
    const line = lines.get(key) || [];
    line.push(glyph);
    lines.set(key, line);
  }
  return [...lines.values()].map((glyphs) => {
    glyphs.sort((left, right) => left.x - right.x);
    return {
      x: glyphs[0].x,
      y: glyphs[0].y,
      text: glyphs.map((glyph) => glyph.text).join(''),
    };
  });
}

function findASTNode(node, predicate) {
  if (!node) return undefined;
  if (predicate(node)) return node;
  for (const member of node.members || []) {
    const found = findASTNode(member.node, predicate);
    if (found) return found;
  }
  return undefined;
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
