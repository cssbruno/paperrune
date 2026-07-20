(function (root, factory) {
  const model = factory();
  if (typeof module === 'object' && module.exports) module.exports = model;
  else root.PaperStudioToolingModel = model;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  'use strict';

  const flowKinds = new Set(['body', 'header', 'footer', 'row', 'column']);
  const pageRegionTemplates = new Set(['header', 'footer']);

  function collectIDs(node, output = new Set()) {
    if (!node) return output;
    if (node.id) output.add(node.id);
    for (const member of node.members || []) if (member.node) collectIDs(member.node, output);
    return output;
  }

  function readableStem(template) {
    return ({paragraph: 'text', 'page-break': 'page-break'}[template] || template).replace(/[^a-z0-9-]/g, '-');
  }

  function nextID(workspace, template) {
    const ids = collectIDs(workspace?.ast?.root);
    const stem = `@${readableStem(template)}`;
    if (!ids.has(stem)) return stem;
    for (let suffix = 2; suffix < 10000; suffix += 1) {
      const candidate = `${stem}-${suffix}`;
      if (!ids.has(candidate)) return candidate;
    }
    throw new Error('No readable ID is available for this tool');
  }

  function compatibleTarget(metadata, template, preferred = '') {
    const targets = metadata?.templateTargets || [];
    const choices = metadata?.templateChoices || {};
    const hasServerChoices = Object.keys(choices).length > 0;
    const accepts = item => {
      if (hasServerChoices) return (choices[item.kind] || []).includes(template);
      return pageRegionTemplates.has(template) ? item.kind === 'page' : flowKinds.has(item.kind);
    };
    const selected = targets.find(item => item.id === preferred && accepts(item));
    return selected || targets.find(accepts) || null;
  }

  function prepareTemplateDraft(workspace, metadata, template, preferred = '') {
    const target = compatibleTarget(metadata, template, preferred);
    if (!target) throw new Error(pageRegionTemplates.has(template) ? 'Add a page before creating this region' : 'Add a body, header, or footer before inserting content');
    return {operation: 'template', target: target.id, template, id: nextID(workspace, template)};
  }

  function availability(metadata, template) {
    return Boolean(compatibleTarget(metadata, template));
  }

  return Object.freeze({availability, compatibleTarget, nextID, prepareTemplateDraft});
});
