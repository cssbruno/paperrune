const assert = require('node:assert/strict');
const model = require('../web/tooling-model.js');

const workspace = {ast: {root: {id: '@doc', members: [
  {node: {kind: 'page', id: '@sheet', members: [
    {node: {kind: 'body', id: '@body', members: [{node: {kind: 'paragraph', id: '@text', members: []}}]}},
  ]}},
]}}};
const metadata = {templateTargets: [
  {id: '@sheet', kind: 'page'},
  {id: '@body', kind: 'body'},
]};

assert.equal(model.availability(metadata, 'header'), true);
assert.equal(model.availability(metadata, 'paragraph'), true);
assert.equal(model.compatibleTarget(metadata, 'header').id, '@sheet');
assert.equal(model.compatibleTarget(metadata, 'paragraph').id, '@body');
assert.equal(model.nextID(workspace, 'paragraph'), '@text-2');
assert.deepEqual(model.prepareTemplateDraft(workspace, metadata, 'footer'), {
  operation: 'template', target: '@sheet', template: 'footer', id: '@footer',
});
assert.deepEqual(model.prepareTemplateDraft(workspace, metadata, 'table', '@body'), {
  operation: 'template', target: '@body', template: 'table', id: '@table',
});
assert.throws(() => model.prepareTemplateDraft(workspace, {templateTargets: []}, 'header'), /Add a page/);
console.log('tooling model tests passed');
