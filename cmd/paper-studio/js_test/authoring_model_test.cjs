const assert = require('node:assert/strict');
const model = require('../web/authoring-model.js');
const workspace = {revision:'plan-1', plan_hash:'plan-1', source_revision:'source-1', scenario:''};
const bodyTemplates = [
  'paragraph','heading','list','image','table','canvas','row','column','page-break','note-box','metadata-grid','signature-row','qr-verification','clause',
  'title-block','two-column','image-caption','quote','checklist','disclaimer','divider','cover-block','recipient-block','code-block','status-banner',
  'numbered-steps','timeline','comparison-table','approval-block','image-grid','invoice-totals','kpi-strip','table-of-contents','risk-register','change-log',
  'decision-record','pros-cons','faq-block','repeat','component','section',
];
const nestedTemplates = bodyTemplates.filter(template => !['list','canvas','page-break','cover-block','image-grid','repeat'].includes(template));
const payload = {format_version:1, revision:'plan-1', plan_hash:'plan-1', source_revision:'source-1', document_target:'@doc',
  template_targets:[{id:'@body',kind:'body'}], binding_targets:[{id:'@copy',kind:'paragraph'}],
	lifecycle_targets:[{id:'@copy',kind:'paragraph'}],
  template_choices:{document:['document-preset','page'],body:bodyTemplates,column:nestedTemplates,list:['item'],item:['text','paragraph','component'],table:['table-row'],'table-row':['cell'],cell:['text','paragraph','image','list']},
  schemas:[{name:'@invoice',fields:[{path:'@invoice.total',kind:'number',required:true},{path:'@invoice.items',kind:'list',required:true}]}],
  object_types:['Address'],
  schema_field_targets:[{id:'@invoice',kind:'schema',schema:'@invoice',path:''}],
  scenarios:['@review'], scenario_values:[{scenario:'@review',path:'total',kind:'number',value:'10'},{scenario:'@review',path:'customer.name',kind:'string',value:'Ada'}],
  stress_presets:['empty','typical','stress'], components:['@card']};
const metadata = model.normalize(payload, workspace);
assert.deepEqual(['paragraph','heading',...(metadata.components.length?['component']:[])], ['paragraph','heading','component']);
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'template',target:'@body',template:'section',id:'@summary'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'template',property:'',target:'@body',template:'section',id:'@summary'});
const bootstrapMetadata = model.normalize({...payload, template_targets:[{id:'@doc',kind:'document'}]}, workspace);
assert.equal(model.buildPayload(workspace, bootstrapMetadata, {operation:'template',target:'@doc',template:'page',id:'@sheet'}).template, 'page');
assert.equal(model.buildPayload(workspace, bootstrapMetadata, {operation:'template',target:'@doc',template:'document-preset',preset:'prescription',id:'@sheet'}).preset, 'prescription');
assert.equal(model.buildPayload(workspace, metadata, {operation:'template',target:'@body',template:'repeat',path:'@invoice.items',id:'@lines'}).path, '@invoice.items');
const scenarioWorkspace = {...workspace,scenario:'@review'};
const scenarioMetadata = model.normalize({...payload,scenario:'@review',template_choices:{...payload.template_choices,body:[...bodyTemplates,'loop']}}, scenarioWorkspace);
assert.equal(model.buildPayload(scenarioWorkspace, scenarioMetadata, {operation:'template',target:'@body',template:'loop',id:'@copies'}).template, 'loop');
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'import',target:'@doc',importPath:'styles/design.paper'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'import',property:'',target:'@doc',import_path:'styles/design.paper'});
assert.throws(() => model.buildPayload(workspace, bootstrapMetadata, {operation:'template',target:'@doc',template:'section',id:'@bad'}), /compatible/);
assert.equal(model.buildPayload(workspace, metadata, {operation:'binding',target:'@copy',path:'@invoice.total',required:true}).path, '@invoice.total');
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'binding',target:'@copy',path:'@invoice.total',required:true,format:'decimal',formatLocale:'pt-BR',minFraction:'2',maxFraction:'2'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'binding',property:'',target:'@copy',path:'@invoice.total',required:true,format:'decimal',format_locale:'pt-BR',format_min_fraction:2,format_max_fraction:2});
assert.equal(model.buildPayload(workspace, metadata, {operation:'scenario-create',target:'@doc',schema:'@invoice',preset:'stress',id:'@stress'}).preset, 'stress');
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'scenario-matrix',target:'@doc',schema:'@invoice',cases:'@empty:empty,@stress:stress'}).cases,
  [{name:'@empty',preset:'empty'},{name:'@stress',preset:'stress'}]);
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'schema-field',target:'@invoice',kind:'list',itemType:'object',maxItems:'24',id:'@lines'}).weight, 24);
assert.equal(model.buildPayload(workspace, metadata, {operation:'schema-field',target:'@invoice',kind:'Address',id:'@billing'}).kind, 'Address');
assert.equal(model.buildPayload(workspace, metadata, {operation:'schema-field',target:'@invoice',kind:'list',itemType:'Address',maxItems:'5',id:'@history'}).text, 'Address');
assert.deepEqual(model.buildPayload(workspace, metadata, {operation:'scenario-value',target:'@review',path:'customer.name',text:'Grace'}).text, 'Grace');
assert.deepEqual(model.buildScenarioLifecyclePayload(workspace, metadata, {action:'rename',target:'@review',id:'@review-copy'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'scenario',target:'@review',property:'rename',id:'@review-copy'});
assert.deepEqual(model.buildScenarioLifecyclePayload(workspace, metadata, {action:'delete',target:'@review'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'scenario',target:'@review',property:'delete'});
assert.throws(() => model.buildScenarioLifecyclePayload(workspace, metadata, {action:'rename',target:'@review',id:'@review'}), /distinct/);
assert.deepEqual(model.buildNodeLifecyclePayload(workspace, metadata, {action:'rename',target:'@copy',id:'@renamed'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'node',target:'@copy',property:'rename',id:'@renamed'});
assert.deepEqual(model.buildNodeLifecyclePayload(workspace, metadata, {action:'delete',target:'@copy'}),
  {source_revision:'source-1',plan_revision:'plan-1',scenario:'',operation:'node',target:'@copy',property:'delete'});
assert.equal(model.buildPayload(workspace, metadata, {operation:'template',target:'@body',template:'component',component:'@card',id:'@card-1'}).component, '@card');
assert.equal(model.buildPayload(workspace, metadata, {operation:'template',target:'@body',template:'heading',id:'@heading'}).template, 'heading');
for (const template of [
  'title-block','two-column','image-caption','quote','checklist','disclaimer','divider',
  'cover-block','recipient-block','code-block','status-banner','numbered-steps','timeline','comparison-table',
  'approval-block','image-grid','invoice-totals','kpi-strip','table-of-contents','risk-register','change-log','decision-record','pros-cons','faq-block',
]) {
  assert.equal(model.buildPayload(workspace, metadata, {operation:'template',target:'@body',template,id:`@${template}`}).template, template);
}
const nestedMetadata = model.normalize({...payload, template_targets:[
  {id:'@body',kind:'body'},{id:'@tasks',kind:'list'},{id:'@first',kind:'item'},
  {id:'@grid',kind:'table'},{id:'@row',kind:'table-row'},{id:'@cell',kind:'cell'},
]}, workspace);
assert.deepEqual(model.templateChoices(nestedMetadata, workspace, 'list'), ['item']);
assert.deepEqual(model.templateChoices(nestedMetadata, workspace, 'table-row'), ['cell']);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('title-block'), true);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('two-column'), true);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('checklist'), true);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('timeline'), true);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('numbered-steps'), true);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('image-grid'), false);
assert.equal(model.templateChoices(nestedMetadata, workspace, 'column').includes('risk-register'), true);
assert.equal(model.buildPayload(workspace, nestedMetadata, {operation:'template',target:'@tasks',template:'item',id:'@second'}).template, 'item');
assert.equal(model.buildPayload(workspace, nestedMetadata, {operation:'template',target:'@row',template:'cell',id:'@extra'}).template, 'cell');
assert.throws(() => model.buildPayload(workspace, nestedMetadata, {operation:'template',target:'@row',template:'canvas',id:'@bad'}), /compatible/);
assert.equal(model.buildPayload(workspace, metadata, {operation:'schema',target:'@doc',id:'@receipt'}).id, '@receipt');
assert.equal(model.buildPayload(workspace, metadata, {operation:'schema-object',target:'@doc',id:'@Address'}).id, '@Address');
assert.throws(() => model.normalize({...payload, revision:'old'}, workspace), /Stale/);
assert.throws(() => model.normalize({...payload, source_revision:'old'}, workspace), /Stale/);
assert.throws(() => model.buildPayload(workspace, metadata, {operation:'binding',target:'@copy',path:'invented'}), /compiler-provided/);
console.log('authoring model tests passed');
