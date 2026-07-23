const test = require('node:test');
const assert = require('node:assert/strict');

const model = require('../web/edit-model.js');

const root = {
  kind: 'document', id: '@report', members: [{node: {
    kind: 'page', id: '@page', members: [
      {node: {kind: 'header', id: '@head', members: []}},
      {node: {kind: 'footer', id: '@foot', members: []}},
      {node: {
      kind: 'body', id: '@body', members: [{node: {
        kind: 'row', id: '@grid', members: [
          {node: {kind: 'paragraph', id: '@left', members: []}},
          {node: {kind: 'image', id: '@art', members: []}},
          {node: {kind: 'table', id: '@ledger', members: [
            {node: {kind: 'table-column', id: '@track', members: []}},
            {node: {kind: 'table-row', id: '@record', members: [
              {node: {kind: 'cell', id: '@cell', members: []}},
            ]}},
          ]}},
          {node: {kind: 'canvas', id: '@diagram', members: [
            {node: {kind: 'anchor', id: '@badge', members: []}},
          ]}},
        ],
      }}],
    }}],
  }}],
};

test('selection exposes only handles supported by exact source structure', () => {
  const left = model.findSelection(root, '@left');
  assert.deepEqual(model.operations(left), ['content', 'text', 'appearance', 'condition', 'box', 'layout-item', 'flow']);
  assert.equal(left.parent.id, '@grid');
  assert.deepEqual(model.operations(model.findSelection(root, '@art')), ['appearance', 'condition', 'box', 'layout-item', 'image', 'flow']);
  assert.deepEqual(model.operations(model.findSelection(root, '@ledger')), ['condition', 'layout-item', 'table', 'flow']);
  assert.deepEqual(model.operations(model.findSelection(root, '@grid')), ['condition', 'layout-container', 'flow']);
  assert.deepEqual(model.operations(model.findSelection(root, '@page')), ['page']);
  assert.deepEqual(model.operations(model.findSelection(root, '@badge')), ['box', 'canvas']);
  assert.deepEqual(model.operations(model.findSelection(root, '@diagram')), ['canvas-container', 'flow']);
  assert.deepEqual(model.operations(model.findSelection(root, '@head')), ['region']);
  assert.deepEqual(model.properties(model.findSelection(root, '@track'), 'table'), ['width', 'min-width', 'max-width']);
  assert.deepEqual(model.operations(model.findSelection(root, '@cell')), ['text', 'appearance', 'box', 'table']);
  assert.equal(model.findSelection(root, '@missing'), null);
});

test('content editor preserves literal and addressed rich-text representations', () => {
  const literalRoot = {kind: 'document', id: '@doc', members: [{node: {kind: 'paragraph', id: '@copy', members: [
    {property: {name: 'text', value: {kind: 'string', string_value: 'Hello'}}},
  ]}}]};
  const literal = model.findSelection(literalRoot, '@copy');
  assert.deepEqual(model.properties(literal, 'content'), ['text']);
  assert.equal(model.valueSpec('content', 'text', literal).currentValue, 'Hello');
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, literal, 'content', 'text', 'Edited').text, 'Edited');

  const richRoot = {kind: 'document', id: '@doc', members: [{node: {kind: 'heading', id: '@title', members: [
    {node: {kind: 'text', id: '@first', value: {kind: 'string', string_value: 'Hello'}, members: []}},
    {node: {kind: 'text', id: '@second', value: {kind: 'string', string_value: ' world'}, members: []}},
  ]}}]};
  const rich = model.findSelection(richRoot, '@title');
  assert.deepEqual(model.properties(rich, 'content'), ['runs']);
  assert.deepEqual(model.buildPayload({source_revision: 's', revision: 'p'}, rich, 'content', 'runs', [
    {target: '@first', text: 'Goodbye'}, {target: '@second', text: ' moon'},
  ]).runs, [{target: '@first', text: 'Goodbye'}, {target: '@second', text: ' moon'}]);

  const boundRoot = {kind: 'document', id: '@doc', members: [{node: {kind: 'paragraph', id: '@name', members: [
    {property: {name: 'bind', value: {kind: 'string', string_value: 'customer.name'}}},
  ]}}]};
  const bound = model.findSelection(boundRoot, '@name');
  assert.deepEqual(model.contentState(bound), {available: false, authored: false, bound: true, text: '', runs: []});
  assert.equal(model.operations(bound).includes('content'), false);
});

test('list controls keep semantic ordering separate from typography and box styling', () => {
  const listRoot = {kind: 'document', id: '@doc', members: [{node: {
    kind: 'list', id: '@steps', members: [],
  }}]};
  const selection = model.findSelection(listRoot, '@steps');
  assert.deepEqual(model.operations(selection), ['text', 'appearance', 'condition', 'box', 'list']);
  assert.deepEqual(model.properties(selection, 'list'), ['ordered', 'marker']);
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'list', 'ordered', 'true').bool, true);
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'list', 'marker', 'asterisk').kind, 'asterisk');
  assert.throws(() => model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'list', 'marker', 'bullet'), /Choose one of/);
});

test('document, page, canvas, appearance, and conditions expose typed missing attributes', () => {
  const workspace = {source_revision: 's', revision: 'p'};
  const document = model.findSelection(root, '@report');
  const page = model.findSelection(root, '@page');
  const canvas = model.findSelection(root, '@diagram');
  const item = model.findSelection(root, '@badge');
  const paragraph = model.findSelection(root, '@left');

  assert.deepEqual(model.operations(document), ['document']);
  assert.deepEqual(model.properties(document, 'document'), ['title', 'language', 'theme']);
  assert.equal(model.buildPayload(workspace, document, 'document', 'theme', '@print').text, '@print');
  assert.throws(() => model.buildPayload(workspace, document, 'document', 'theme', 'print'), /exact/);
  assert.ok(model.properties(page, 'page').includes('page-numbers'));
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-numbers', 'true').bool, true);
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-number-position', 'header').kind, 'header');
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-number-align', 'outer').kind, 'outer');
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-number-hide-first', 'true').bool, true);
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-number-start', '5').count, 5);
  assert.throws(() => model.buildPayload(workspace, page, 'page', 'page-number-align', 'diagonal'), /Choose one of/);
  assert.equal(model.buildPayload(workspace, page, 'page', 'page-number-format', 'Page %d').text, 'Page %d');
  assert.deepEqual(model.properties(canvas, 'canvas-container'), ['width', 'height', 'default-horizontal', 'default-vertical']);
  assert.equal(model.buildPayload(workspace, canvas, 'canvas-container', 'default-horizontal', 'center-x').kind, 'center-x');
  assert.equal(model.buildPayload(workspace, item, 'canvas', 'width', '42pt').points, 42);
  assert.equal(model.buildPayload(workspace, item, 'canvas', 'alt', 'Status badge').text, 'Status badge');
  assert.deepEqual(model.properties(paragraph, 'appearance'), ['style', 'font-token', 'size-token', 'line-height-token', 'color-token']);
  assert.equal(model.buildPayload(workspace, paragraph, 'appearance', 'style', '@body').text, '@body');
  assert.equal(model.buildPayload(workspace, paragraph, 'appearance', 'color-token', 'ink').text, 'ink');
  assert.throws(() => model.buildPayload(workspace, paragraph, 'appearance', 'color-token', 'bad token'), /without whitespace/);
  assert.equal(model.buildPayload(workspace, paragraph, 'condition', 'visible', 'patient.active == true').text, 'patient.active == true');
  assert.throws(() => model.buildPayload(workspace, paragraph, 'condition', 'visible', '  '), /required/);
  assert.equal(model.defaultValue(model.valueSpec('condition', 'visible', paragraph)), 'true');
  assert.equal(model.defaultValue(model.valueSpec('appearance', 'style', paragraph)), '');
});

test('heading, table-row, and cell integer attributes enforce exact bounds', () => {
  const workspace = {source_revision: 's', revision: 'p'};
  const headingRoot = {kind: 'document', id: '@doc', members: [{node: {kind: 'heading', id: '@heading', members: []}}]};
  const heading = model.findSelection(headingRoot, '@heading');
  const row = model.findSelection(root, '@record');
  const cell = model.findSelection(root, '@cell');

  assert.ok(model.properties(heading, 'text').includes('level'));
  assert.equal(model.buildPayload(workspace, heading, 'text', 'level', '6').count, 6);
  assert.throws(() => model.buildPayload(workspace, heading, 'text', 'level', '1.5'), /whole number/);
  assert.deepEqual(model.properties(row, 'table'), ['keep-together', 'keep-with-next', 'orphans', 'widows']);
  assert.equal(model.buildPayload(workspace, row, 'table', 'orphans', '3').count, 3);
  assert.throws(() => model.buildPayload(workspace, row, 'table', 'widows', '0'), /whole number/);
  assert.deepEqual(model.properties(cell, 'table'), ['header-cell', 'vertical-align', 'colspan', 'rowspan']);
  assert.equal(model.buildPayload(workspace, cell, 'table', 'colspan', '1024').count, 1024);
  assert.throws(() => model.buildPayload(workspace, cell, 'table', 'rowspan', '1025'), /whole number/);
});

test('row and column containers expose complete readable layout controls', () => {
  const workspace = {source_revision: 's', revision: 'p'};
  const row = model.findSelection(root, '@grid');
  assert.deepEqual(model.properties(row, 'layout-container'), [
    'gap', 'line-gap', 'height', 'wrap', 'justify-content', 'align-items', 'align-content', 'reverse',
  ]);
  assert.equal(model.buildPayload(workspace, row, 'layout-container', 'gap', '0').points, 0);
  assert.equal(model.buildPayload(workspace, row, 'layout-container', 'height', '80pt').length, '80pt');
  assert.equal(model.buildPayload(workspace, row, 'layout-container', 'justify-content', 'space-between').kind, 'space-between');
  assert.equal(model.buildPayload(workspace, row, 'layout-container', 'reverse', 'true').bool, true);
  assert.throws(() => model.buildPayload(workspace, row, 'layout-container', 'height', '50%'), /Percentages are unavailable/);
  assert.throws(() => model.buildPayload(workspace, row, 'layout-container', 'gap', 'auto'), /physical size/);
  assert.equal(model.defaultValue(model.valueSpec('layout-container', 'gap', row)), '0pt');
  assert.equal(model.defaultValue(model.valueSpec('layout-container', 'height', row)), '100pt');
});

test('image dimensions have one owner inside a row while image-specific attributes remain editable', () => {
  const image = model.findSelection(root, '@art');
  assert.deepEqual(model.properties(image, 'image'), ['fit', 'focus-x', 'focus-y', 'align', 'caption', 'alt', 'decorative']);
  assert.ok(model.properties(image, 'layout-item').includes('width'));
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, image, 'image', 'align', 'center').kind, 'center');
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, image, 'image', 'caption', 'Evidence image').text, 'Evidence image');
});

test('image and table payloads stay typed and selection-specific', () => {
  const workspace = {source_revision: 'source-digest', revision: 'plan-hash'};
  assert.deepEqual(model.buildPayload(workspace, model.findSelection(root, '@art'), 'image', 'focus-x', '0.75'), {
    source_revision: 'source-digest', plan_revision: 'plan-hash', scenario: '', operation: 'image', target: '@art', property: 'focus-x', number: 0.75,
  });
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@art'), 'image', 'decorative', 'false').bool, false);
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@ledger'), 'table', 'split', 'avoid').split, 'avoid');
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@ledger'), 'table', 'caption', 'Quarterly results').text, 'Quarterly results');
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@cell'), 'table', 'header-cell', 'true').bool, true);
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@cell'), 'table', 'vertical-align', 'middle').kind, 'middle');
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@art'), 'layout-item', 'width', '50%').length, '50%');
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@art'), 'layout-item', 'height', 'auto').length, 'auto');
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@track'), 'table', 'width', '100%').length, '100%');
  assert.throws(() => model.buildPayload(workspace, model.findSelection(root, '@track'), 'table', 'header-cell', 'true'), /unavailable/);
  assert.equal(model.buildPayload(workspace, model.findSelection(root, '@page'), 'page', 'margin-left', '18').points, 18);
  assert.deepEqual(model.buildPayload(workspace, model.findSelection(root, '@badge'), 'canvas', 'left', '@left.right + 8pt'), {
    source_revision: 'source-digest', plan_revision: 'plan-hash', scenario: '', operation: 'canvas', target: '@badge', property: 'left',
    text: '@left', kind: 'right', points: 8,
  });
});

test('payload contains review facts and semantic intent but no capabilities', () => {
  const workspace = {source_revision: 'source-digest', revision: 'plan-hash', scenario: '@print'};
  const selection = model.findSelection(root, '@left');
  assert.deepEqual(model.buildPayload(workspace, selection, 'layout-item', 'width', '48.25'), {
    source_revision: 'source-digest', plan_revision: 'plan-hash', scenario: '@print',
    operation: 'layout-item', target: '@left', property: 'width', points: 48.25,
  });
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'width', '50%').length, '50%');
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'width', 'auto').length, 'auto');
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'width', '2fr').length, '2fr');
  assert.throws(() => model.buildPayload(workspace, selection, 'layout-item', 'width', '1.5fr'), /positive whole/);
  assert.throws(() => model.buildPayload(workspace, selection, 'layout-item', 'height', '1fr'), /parent flow axis/);
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'flex-grow', '1.5').number, 1.5);
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'flex-shrink', '0').number, 0);
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'height', '50%').length, '50%');
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'min-height', '0').points, 0);
  assert.equal(model.buildPayload(workspace, selection, 'layout-item', 'align-self', 'stretch').kind, 'stretch');
  assert.ok(model.properties(selection, 'layout-item').includes('max-width'));
  const encoded = JSON.stringify(model.buildPayload(workspace, selection, 'box', 'background', '#AABBCC'));
  assert.equal(encoded.includes('capability'), false);
  assert.equal(encoded.includes('handle'), false);
  assert.equal(JSON.parse(encoded).color, '#aabbcc');
});

test('column sizing swaps the readable main axis without exposing track jargon', () => {
  const columnRoot = {kind: 'document', id: '@doc', members: [{node: {
    kind: 'column', id: '@stack', members: [{node: {kind: 'paragraph', id: '@item', members: []}}],
  }}]};
  const selection = model.findSelection(columnRoot, '@item');
  assert.deepEqual(model.properties(selection, 'layout-item'), [
    'height', 'min-height', 'max-height', 'width', 'min-width', 'max-width', 'flex-grow', 'flex-shrink', 'align-self',
  ]);
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'layout-item', 'height', '3fr').length, '3fr');
  assert.throws(() => model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'layout-item', 'width', '1fr'), /parent flow axis/);
  assert.equal(model.properties(selection, 'layout-item').some((name) => name.includes('track') || name.includes('cross')), false);
});

test('font replacement is explicit, supported-only, and locates the authored text owner', () => {
  const textRoot = {kind: 'document', id: '@report', span: {start: {line: 1}, end: {line: 8}}, members: [{node: {
    kind: 'paragraph', id: '@copy', span: {start: {line: 4}, end: {line: 7}}, members: [],
  }}]};
  const selection = model.findTextSelectionAtLine(textRoot, 5);
  assert.equal(selection.target, '@copy');
  assert.deepEqual(model.coreFonts, ['Courier', 'Helvetica', 'Times', 'Symbol', 'ZapfDingbats']);
  assert.deepEqual(model.buildPayload({source_revision: 's', revision: 'source-bad'}, selection, 'text', 'font', 'Helvetica'), {
    source_revision: 's', plan_revision: 'source-bad', scenario: '', operation: 'text', target: '@copy', property: 'font', text: 'Helvetica',
  });
  assert.throws(() => model.buildPayload({source_revision: 's', revision: 'source-bad'}, selection, 'text', 'font', 'Unavailable Sans'), /Choose one of/);
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'text', 'size', '12pt').length, '12pt');
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'text', 'color', '#123ABC').color, '#123abc');
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'text', 'align', 'justify').kind, 'justify');
  assert.equal(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'text', 'bold', 'true').bool, true);
  assert.ok(model.properties(selection, 'box').includes('margin-left'));
});

test('invalid values and structurally unavailable handles fail closed', () => {
  const workspace = {source_revision: 'source-digest', revision: 'plan-hash'};
  const selection = model.findSelection(root, '@left');
  assert.throws(() => model.buildPayload(workspace, selection, 'layout-item', 'track-weight', '1.5'), /unavailable/);
  assert.throws(() => model.buildPayload(workspace, selection, 'layout-item', 'width', '0fr'), /positive whole/);
  assert.throws(() => model.buildPayload(workspace, selection, 'box', 'background', 'red'), /six-digit/);
  assert.throws(() => model.buildPayload(workspace, model.findSelection(root, '@ledger'), 'box', 'padding', '1'), /unavailable/);
  assert.throws(() => model.buildPayload(workspace, model.findSelection(root, '@art'), 'image', 'decorative', 'yes'), /true or false/);
  assert.throws(() => model.buildPayload({}, selection, 'box', 'padding', '1'), /revisions/);
});

test('editor defaults are valid for their exact value type', () => {
  const selection = model.findSelection(root, '@left');
  assert.equal(model.defaultValue(model.valueSpec('layout-item', 'width', selection)), 'auto');
  assert.equal(model.defaultValue(model.valueSpec('text', 'size', selection)), '12pt');
  assert.equal(model.defaultValue(model.valueSpec('canvas', 'left', model.findSelection(root, '@badge'))), 'canvas.left');
});

test('inspector reads authored values, groups properties, and builds exact resets', () => {
  const authoredRoot = {kind: 'document', id: '@doc', members: [
    {node: {kind: 'style', id: '@body-style', members: []}},
    {node: {kind: 'theme', id: '@print', members: [{node: {kind: 'token', id: '@ink', members: []}}]}},
    {node: {kind: 'page', id: '@page', members: [{property: {name: 'margin-left', value: {kind: 'unit', raw: '18pt', unit_value: {number: 18, unit: 'pt'}}}}]}},
  ]};
  const page = model.findSelection(authoredRoot, '@page');
  const spec = model.valueSpec('page', 'margin-left', page);
  assert.equal(spec.authored, true);
  assert.equal(model.defaultValue(spec), '18pt');
  assert.deepEqual(spec.units, ['pt']);
  assert.equal(model.propertyGroup('margin-left'), 'Outer spacing');
  assert.deepEqual(model.buildPayload({source_revision: 's', revision: 'p'}, page, 'page', 'margin-left', '24pt').points, 24);
  assert.deepEqual(model.buildResetPayload({source_revision: 's', revision: 'p'}, page, 'page', 'margin-left'), {
    source_revision: 's', plan_revision: 'p', scenario: '', operation: 'page', target: '@page', property: 'margin-left', reset: true,
  });
  assert.throws(() => model.buildResetPayload({source_revision: 's', revision: 'p'}, page, 'page', 'margin-right'), /already using/);
});

test('reference controls use authored style, theme, token, and canvas sibling IDs', () => {
  const referenceRoot = {kind: 'document', id: '@doc', members: [
    {property: {name: 'theme', value: {kind: 'string', string_value: '@print'}}},
    {node: {kind: 'style', id: '@body-style', members: []}},
    {node: {kind: 'theme', id: '@print', members: [
      {node: {kind: 'token', id: '@ink', members: [{property: {name: 'type', value: {kind: 'string', string_value: 'color'}}}]}},
      {node: {kind: 'token', id: '@body-size', members: [{property: {name: 'type', value: {kind: 'string', string_value: 'length'}}}]}},
    ]}},
    {node: {kind: 'paragraph', id: '@copy', members: []}},
    {node: {kind: 'canvas', id: '@canvas', members: [
      {node: {kind: 'anchor', id: '@badge', members: []}}, {node: {kind: 'anchor', id: '@logo', members: []}},
    ]}},
  ]};
  const copy = model.findSelection(referenceRoot, '@copy');
  assert.deepEqual(model.valueSpec('appearance', 'style', copy).choices, ['@body-style']);
  assert.deepEqual(model.valueSpec('appearance', 'color-token', copy).choices, ['ink']);
  assert.deepEqual(model.valueSpec('appearance', 'size-token', copy).choices, ['body-size']);
  assert.deepEqual(model.valueSpec('document', 'theme', model.findSelection(referenceRoot, '@doc')).choices, ['@print']);
  const constraint = model.valueSpec('canvas', 'left', model.findSelection(referenceRoot, '@badge'));
  assert.deepEqual(constraint.targets, ['canvas', '@logo']);
  assert.deepEqual(constraint.anchors, ['left', 'right', 'center-x']);
});

test('flow handle exposes only exact semantic destinations and emits a parent target', () => {
  const selection = model.findSelection(root, '@left');
  assert.ok(model.operations(selection).includes('flow'));
  assert.deepEqual(model.flowDestinations(selection).map((node) => node.id), ['@body', '@grid']);
  assert.deepEqual(model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'flow', 'destination', '@grid'), {
    source_revision: 's', plan_revision: 'p', scenario: '', operation: 'flow', target: '@left', property: 'destination', new_parent: '@grid',
  });
  assert.throws(() => model.buildPayload({source_revision: 's', revision: 'p'}, selection, 'flow', 'destination', '@art'), /destination/);
});
