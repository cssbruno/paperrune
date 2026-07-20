const assert = require('node:assert/strict');
const model = require('../web/tooling-model.js');

const workspace = {ast: {root: {id: '@doc', members: [
  {node: {kind: 'page', id: '@sheet', members: [
    {node: {kind: 'body', id: '@body', members: [
      {node: {kind: 'paragraph', id: '@text', members: []}},
      {node: {kind: 'page-break', id: '@page-break', members: []}},
    ]}},
  ]}},
]}}};
const metadata = {templateTargets: [
  {id: '@sheet', kind: 'page'},
  {id: '@body', kind: 'body'},
  {id: '@running-header', kind: 'header'},
  {id: '@nested', kind: 'column'},
], templateChoices: {
  page: ['header', 'footer'],
  body: ['paragraph', 'table', 'page-break'],
  header: ['paragraph', 'table'],
  column: ['paragraph', 'table'],
}};

assert.equal(model.availability(metadata, 'header'), true);
assert.equal(model.availability(metadata, 'paragraph'), true);
assert.equal(model.compatibleTarget(metadata, 'header').id, '@sheet');
assert.equal(model.compatibleTarget(metadata, 'paragraph').id, '@body');
assert.equal(model.nextID(workspace, 'paragraph'), '@text-2');
assert.equal(model.nextID(workspace, 'page-break'), '@page-break-2');
assert.deepEqual(model.prepareTemplateDraft(workspace, metadata, 'footer'), {
  operation: 'template', target: '@sheet', template: 'footer', id: '@footer',
});
assert.deepEqual(model.prepareTemplateDraft(workspace, metadata, 'table', '@body'), {
  operation: 'template', target: '@body', template: 'table', id: '@table',
});
assert.deepEqual(model.prepareTemplateDraft(workspace, metadata, 'page-break', '@nested'), {
  operation: 'template', target: '@body', template: 'page-break', id: '@page-break-2',
});
assert.equal(model.prepareTemplateDraft(workspace, metadata, 'page-break', '@running-header').target, '@body');
assert.equal(model.availability({...metadata, templateChoices: {...metadata.templateChoices, body: ['paragraph']}}, 'page-break'), false);
assert.equal(model.availability({templateTargets: [{id: '@nested', kind: 'column'}], templateChoices: {column: ['paragraph']}}, 'page-break'), false);
assert.throws(() => model.prepareTemplateDraft(workspace, {templateTargets: []}, 'header'), /Add a page/);
console.log('tooling model tests passed');
