(function (root, factory) {
  const model = factory();
  if (typeof module === 'object' && module.exports) module.exports = model;
  else root.PaperStudioEditModel = model;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  'use strict';

  const boxProperties = Object.freeze([
    'margin', 'margin-top', 'margin-right', 'margin-bottom', 'margin-left',
    'padding', 'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
    'border-width', 'border-top-width', 'border-right-width', 'border-bottom-width', 'border-left-width',
    'border-radius', 'border-color', 'background',
  ]);
  const textKinds = new Set(['paragraph', 'heading', 'list', 'cell']);
  const contentKinds = new Set(['paragraph', 'heading', 'text']);
  const appearanceKinds = new Set(['paragraph', 'heading', 'list', 'image', 'cell']);
  const conditionKinds = new Set(['paragraph', 'heading', 'list', 'row', 'column', 'image', 'table']);
  const boxKinds = new Set(['paragraph', 'heading', 'list', 'image', 'cell', 'anchor']);
  const coreFonts = Object.freeze(['Courier', 'Helvetica', 'Times', 'Symbol', 'ZapfDingbats']);
  const layoutItemKinds = new Set(['paragraph', 'heading', 'image', 'table', 'row', 'column']);
  const layoutKinds = new Set(['row', 'column']);
  const imageProperties = Object.freeze(['fit', 'focus-x', 'focus-y', 'width', 'height', 'max-width', 'max-height', 'align', 'caption', 'alt', 'decorative']);
  const canvasItemConstraints = Object.freeze(['left', 'right', 'center-x', 'top', 'bottom', 'center-y']);
  const flowKinds = new Set(['heading', 'paragraph', 'list', 'page-break', 'text', 'row', 'column', 'image', 'table', 'canvas', 'use', 'repeat', 'loop']);
  const flowParents = new Set(['body', 'row', 'column']);

  function findSelection(root, target) {
    let result = null;
    let matches = 0;
    function walk(node, parent) {
      if (!node) return;
      if (node.id === target) {
        matches += 1;
        result = {target, node, parent, root};
      }
      for (const member of node.members || []) walk(member.node, node);
    }
    walk(root, null);
    return matches === 1 ? result : null;
  }

  function findTextSelectionAtLine(root, line) {
    let result = null;
    const wanted = Number(line);
    function walk(node) {
      if (!node) return;
      const start = Number(node.span?.start?.line || 0);
      const end = Number(node.span?.end?.line || start);
      if (textKinds.has(node.kind) && node.id && start > 0 && wanted >= start && wanted <= end) {
        result = findSelection(root, node.id);
      }
      for (const member of node.members || []) walk(member.node);
    }
    walk(root);
    return result;
  }

  function operations(selection) {
    if (!selection) return [];
    const result = [];
    if (contentKinds.has(selection.node.kind) && contentState(selection).available) result.push('content');
    if (selection.node.kind === 'document') result.push('document');
    if (textKinds.has(selection.node.kind)) result.push('text');
    if (appearanceKinds.has(selection.node.kind)) result.push('appearance');
    if (conditionKinds.has(selection.node.kind)) result.push('condition');
    if (boxKinds.has(selection.node.kind)) result.push('box');
    if (selection.node.kind === 'list') result.push('list');
    if (layoutKinds.has(selection.node.kind)) result.push('layout-container');
    if (layoutItemKinds.has(selection.node.kind) && layoutKinds.has(selection.parent?.kind) && selection.parent?.id) result.push('layout-item');
    if (selection.node.kind === 'image') result.push('image');
    if (['table', 'table-column', 'table-row', 'cell'].includes(selection.node.kind)) result.push('table');
    if (selection.node.kind === 'page') result.push('page');
    if (selection.node.kind === 'anchor' && selection.parent?.kind === 'canvas' && selection.parent?.id) result.push('canvas');
    if (selection.node.kind === 'canvas') result.push('canvas-container');
    if (['header', 'footer'].includes(selection.node.kind) && selection.parent?.kind === 'page' && selection.parent?.id) result.push('region');
    if (flowKinds.has(selection.node.kind) && flowDestinations(selection).length) result.push('flow');
    return result;
  }

  function layoutItemProperties(selection) {
    const row = selection?.parent?.kind === 'row';
    const main = row ? ['width', 'min-width', 'max-width'] : ['height', 'min-height', 'max-height'];
    const cross = row ? ['height', 'min-height', 'max-height'] : ['width', 'min-width', 'max-width'];
    return [...main, ...cross, 'flex-grow', 'flex-shrink', 'align-self'];
  }

  function layoutContainerProperties(selection) {
    const dimension = selection?.node?.kind === 'row' ? 'height' : 'width';
    return ['gap', 'line-gap', dimension, 'wrap', 'justify-content', 'align-items', 'align-content', 'reverse'];
  }

  function properties(operation, selection) {
    if (operation === 'content') {
      const content = contentState(selection);
      if (!content.available) return [];
      return content.runs.length > 1 ? ['runs'] : ['text'];
    }
    if (operation === 'document') return ['title', 'language', 'theme'];
    if (operation === 'text') return selection?.node?.kind === 'heading' ? ['font', 'size', 'line-height', 'color', 'align', 'bold', 'italic', 'level'] : ['font', 'size', 'line-height', 'color', 'align', 'bold', 'italic'];
    if (operation === 'appearance') return selection?.node?.kind === 'image' ? ['style'] : ['style', 'font-token', 'size-token', 'line-height-token', 'color-token'];
    if (operation === 'condition') return ['when'];
    if (operation === 'box') return boxProperties;
    if (operation === 'list') return ['ordered', 'marker'];
    if (operation === 'layout-item') return layoutItemProperties(selection);
    if (operation === 'layout-container') return layoutContainerProperties(selection);
    if (operation === 'image') {
      if (layoutKinds.has(selection?.parent?.kind)) return imageProperties.filter((name) => !['width', 'height', 'max-width', 'max-height'].includes(name));
      return imageProperties;
    }
    if (operation === 'table') {
      return [];
    }
    if (operation === 'page') return ['margin', 'margin-top', 'margin-right', 'margin-bottom', 'margin-left', 'page-numbers', 'page-number-format', 'page-total-alias'];
    if (operation === 'canvas') return [...canvasItemConstraints, 'width', 'height', 'alt'];
    if (operation === 'canvas-container') return ['width', 'height', 'default-horizontal', 'default-vertical'];
    if (operation === 'region') return boxProperties;
    if (operation === 'flow') return ['destination'];
    return [];
  }

  function baseValueSpec(operation, property, selection) {
    if (operation === 'content') {
      const content = contentState(selection);
      if (property === 'runs') return {kind: 'rich-text', label: 'Styled text runs', currentValue: content.runs, authored: true};
      return {kind: 'multiline', label: 'Text content', currentValue: content.text, authored: content.authored};
    }
    if (operation === 'document' && property === 'title') return {kind: 'text', label: 'Document title'};
    if (operation === 'document' && property === 'language') return {kind: 'text', label: 'Language tag', defaultValue: 'en', required: true};
    if (operation === 'document' && property === 'theme') return {kind: 'reference', label: 'Theme @id', prefix: '@', required: true};
    if (operation === 'appearance' && property === 'style') return {kind: 'reference', label: 'Style @id', prefix: '@', required: true};
    if (operation === 'appearance') return {kind: 'name', label: 'Theme token', required: true};
    if (operation === 'condition') return {kind: 'text', label: 'Condition expression', defaultValue: 'true', required: true};
    if (operation === 'text' && property === 'font') return {kind: 'choice', label: 'Replacement font', choices: coreFonts, field: 'text'};
    if (operation === 'text' && ['size', 'line-height'].includes(property)) return {kind: 'length', label: 'Physical size (pt)', allowAuto: false, allowPercent: false, defaultValue: property === 'size' ? '12pt' : '14pt'};
    if (operation === 'text' && property === 'color') return {kind: 'color', label: 'Text color'};
    if (operation === 'text' && property === 'align') return {kind: 'choice', label: 'Text alignment', choices: ['left', 'center', 'right', 'justify']};
    if (operation === 'text' && ['bold', 'italic'].includes(property)) return {kind: 'boolean', label: 'Value', choices: ['true', 'false']};
    if (operation === 'text' && property === 'level') return {kind: 'integer', label: 'Heading level', min: 1, max: 6, field: 'count'};
    if (operation === 'list' && property === 'ordered') return {kind: 'boolean', label: 'Numbered list', choices: ['true', 'false']};
    if (operation === 'list' && property === 'marker') return {kind: 'choice', label: 'Marker', choices: ['decimal', 'dash', 'asterisk']};
    if ((operation === 'box' || operation === 'region') && ['border-color', 'background'].includes(property)) return {kind: 'color', label: 'Color'};
    if (operation === 'box' || operation === 'region') return {kind: 'length', label: 'Spacing (pt)', allowAuto: false, allowPercent: false, positive: false, pointsOnly: true};
    if (operation === 'layout-item' && ['flex-grow', 'flex-shrink'].includes(property)) return {kind: 'number', label: 'Flex factor', min: 0, max: 4294.967295, step: 0.1, field: 'number'};
    if (operation === 'layout-item' && property === 'align-self') return {kind: 'choice', label: 'Alignment', choices: ['start', 'center', 'end', 'stretch']};
    if (operation === 'layout-item' && ['width', 'height', 'min-width', 'max-width', 'min-height', 'max-height'].includes(property)) {
      const main = selection?.parent?.kind === 'row' ? 'width' : 'height';
      const constraint = property.startsWith('min-') || property.startsWith('max-');
      return {kind: 'length', label: `${constraint ? 'Constraint' : 'Size'} (auto, %, or pt${property === main ? ', or fr' : ''})`, positive: !constraint, allowFraction: property === main};
    }
    if (operation === 'layout-container' && ['gap', 'line-gap'].includes(property)) return {kind: 'length', label: 'Spacing (pt)', positive: false, allowAuto: false, allowPercent: false, defaultValue: '0pt'};
    if (operation === 'layout-container' && ['width', 'height'].includes(property)) return {kind: 'length', label: 'Physical size (pt)', allowAuto: false, allowPercent: false, defaultValue: '100pt'};
    if (operation === 'layout-container' && property === 'wrap') return {kind: 'choice', label: 'Wrapping', choices: ['nowrap', 'wrap', 'wrap-reverse']};
    if (operation === 'layout-container' && property === 'justify-content') return {kind: 'choice', label: 'Along the flow', choices: ['start', 'center', 'end', 'space-between', 'space-around', 'space-evenly']};
    if (operation === 'layout-container' && property === 'align-items') return {kind: 'choice', label: 'Across the flow', choices: ['start', 'center', 'end', 'stretch']};
    if (operation === 'layout-container' && property === 'align-content') return {kind: 'choice', label: 'Wrapped lines', choices: ['start', 'center', 'end', 'stretch', 'space-between', 'space-around', 'space-evenly']};
    if (operation === 'layout-container' && property === 'reverse') return {kind: 'boolean', label: 'Reverse order', choices: ['true', 'false']};
    if (operation === 'image' && property === 'fit') return {kind: 'choice', label: 'Fit', choices: ['auto', 'contain', 'cover']};
    if (operation === 'image' && ['focus-x', 'focus-y'].includes(property)) return {kind: 'number', label: 'Ratio', min: 0, max: 1, step: 0.05, field: 'number'};
    if (operation === 'image' && ['width', 'max-width'].includes(property)) return {kind: 'length', label: 'Size (auto, %, or pt)', allowPercent: true};
    if (operation === 'image' && ['height', 'max-height'].includes(property)) return {kind: 'length', label: 'Size (auto or pt)', allowPercent: false};
    if (operation === 'image' && property === 'align') return {kind: 'choice', label: 'Alignment', choices: ['left', 'center', 'right']};
    if (operation === 'image' && property === 'caption') return {kind: 'text', label: 'Caption'};
    if (operation === 'image' && property === 'alt') return {kind: 'text', label: 'Alt text'};
    if ((operation === 'image' && property === 'decorative') || (operation === 'table' && ['repeat-header', 'header-cell'].includes(property))) return {kind: 'boolean', label: 'Value', choices: ['true', 'false']};
    if (operation === 'table' && property === 'split') return {kind: 'choice', label: 'Split', choices: ['rows', 'avoid'], field: 'split'};
    if (operation === 'table' && property === 'caption') return {kind: 'text', label: 'Caption'};
    if (operation === 'table' && property === 'vertical-align') return {kind: 'choice', label: 'Vertical alignment', choices: ['top', 'middle', 'bottom']};
    if (operation === 'table' && ['width', 'min-width', 'max-width'].includes(property)) return {kind: 'length', label: 'Size (auto, %, or pt)'};
    if (operation === 'table' && ['keep-together', 'keep-with-next'].includes(property)) return {kind: 'boolean', label: 'Pagination policy', choices: ['true', 'false']};
    if (operation === 'table' && ['orphans', 'widows'].includes(property)) return {kind: 'integer', label: 'Minimum rows', min: 1, max: 1048576, field: 'count'};
    if (operation === 'table' && ['colspan', 'rowspan'].includes(property)) return {kind: 'integer', label: 'Cell span', min: 1, max: 1024, field: 'count'};
    if (operation === 'page' && ['margin', 'margin-top', 'margin-right', 'margin-bottom', 'margin-left'].includes(property)) return {kind: 'length', label: 'Page spacing (pt)', allowAuto: false, allowPercent: false, positive: false, pointsOnly: true};
    if (operation === 'page' && property === 'page-numbers') return {kind: 'boolean', label: 'Page numbers', choices: ['true', 'false']};
    if (operation === 'page' && property === 'page-number-format') return {kind: 'text', label: 'Page number format', defaultValue: 'Page %d of {pages}'};
    if (operation === 'page' && property === 'page-total-alias') return {kind: 'text', label: 'Total-page alias', defaultValue: '{pages}'};
    if (operation === 'canvas' && canvasItemConstraints.includes(property)) return {kind: 'constraint', label: 'Target anchor'};
    if (operation === 'canvas' && ['width', 'height'].includes(property)) return {kind: 'length', label: 'Physical size (pt)', allowAuto: false, allowPercent: false, defaultValue: '100pt', pointsOnly: true};
    if (operation === 'canvas' && property === 'alt') return {kind: 'text', label: 'Alt text'};
    if (operation === 'canvas-container' && ['width', 'height'].includes(property)) return {kind: 'length', label: 'Physical size (pt)', allowAuto: false, allowPercent: false, defaultValue: '100pt', pointsOnly: true};
    if (operation === 'canvas-container' && property === 'default-horizontal') return {kind: 'choice', label: 'Default horizontal anchor', choices: ['left', 'right', 'center-x']};
    if (operation === 'canvas-container' && property === 'default-vertical') return {kind: 'choice', label: 'Default vertical anchor', choices: ['top', 'bottom', 'center-y']};
    if (operation === 'flow') return {kind: 'text', label: 'Destination @id'};
    return {kind: 'number', label: 'Points', min: 0, max: 1000000, step: 0.25};
  }

  const propertyHelp = Object.freeze({
    'flex-grow': 'How strongly this item receives leftover space compared with its siblings.',
    'flex-shrink': 'How strongly this item gives up space when the row or column is crowded.',
    'justify-content': 'Places items along the row or column’s direction.',
    'align-items': 'Places every item across the row or column’s direction.',
    'align-content': 'Places wrapped lines when the container has extra cross-axis space.',
    'align-self': 'Overrides cross-axis alignment for this item only.',
    orphans: 'Minimum rows kept at the bottom of a page before the table continues.',
    widows: 'Minimum rows kept at the top of the next page when the table continues.',
    colspan: 'Number of table columns this cell occupies.',
    rowspan: 'Number of table rows this cell occupies.',
    'repeat-header': 'Repeats the table header after a page break.',
    'keep-together': 'Tries to keep this row on one page.',
    'keep-with-next': 'Tries to keep this row beside the following row.',
    'page-total-alias': 'Placeholder used by the page-number format for the total page count.',
    when: 'Shows this content only when the data expression evaluates to true.',
    decorative: 'Marks an image as visual decoration so alternative text is not required.',
    'default-horizontal': 'Anchor used when a canvas item has no horizontal constraint.',
    'default-vertical': 'Anchor used when a canvas item has no vertical constraint.',
  });

  function scalarValue(scalar) {
    if (!scalar) return null;
    if (scalar.kind === 'string') return scalar.string_value ?? '';
    if (scalar.kind === 'bool') return String(Boolean(scalar.bool_value));
    if (scalar.kind === 'number') return String(scalar.number_value ?? scalar.raw ?? '');
    if (scalar.kind === 'unit') return `${scalar.unit_value?.number ?? ''}${scalar.unit_value?.unit ?? ''}`;
    return scalar.raw ?? '';
  }

  function authoredValue(selection, property) {
    for (const member of selection?.node?.members || []) {
      if (member.property?.name === property) return {authored: true, value: scalarValue(member.property.value)};
    }
    return {authored: false, value: ''};
  }

  function contentState(selection) {
    const node = selection?.node;
    if (!node || !contentKinds.has(node.kind)) return {available: false, authored: false, text: '', runs: []};
    if (node.kind === 'text') return {available: Boolean(node.id && node.value), authored: Boolean(node.value), text: scalarValue(node.value) ?? '', runs: []};
    const propertyText = authoredValue(selection, 'text');
    const children = (node.members || []).map(member => member.node).filter(child => child?.kind === 'text');
    if (propertyText.authored && children.length) return {available: false, authored: true, text: '', runs: []};
    if (children.length === 1 && children[0].value) {
      const run = children[0].id ? [{target: children[0].id, text: scalarValue(children[0].value) ?? ''}] : [];
      return {available: true, authored: true, text: scalarValue(children[0].value) ?? '', runs: run};
    }
    if (children.length > 1 && children.length <= 7 && children.every(child => child.id && child.value)) {
      return {available: true, authored: true, text: '', runs: children.map(child => ({target: child.id, text: scalarValue(child.value) ?? ''}))};
    }
    if (children.length > 1) return {available: false, authored: true, text: '', runs: []};
    return {available: true, authored: propertyText.authored, text: propertyText.value, runs: []};
  }

  function nodesByKind(root, kind) {
    const result = [];
    (function walk(node) {
      if (!node) return;
      if (node.kind === kind && node.id) result.push(node.id);
      for (const member of node.members || []) walk(member.node);
    })(root);
    return result;
  }

  function directProperty(node, name) {
    for (const member of node?.members || []) {
      if (member.property?.name === name) return scalarValue(member.property.value);
    }
    return '';
  }

  function themeTokenChoices(selection, property) {
    const wantedType = ({'font-token': 'string', 'size-token': 'length', 'line-height-token': 'length', 'color-token': 'color'})[property];
    const themes = new Map();
    (function walk(node) {
      if (!node) return;
      if (node.kind === 'theme' && node.id) themes.set(node.id, node);
      for (const member of node.members || []) walk(member.node);
    })(selection?.root);
    const selectedTheme = directProperty(selection?.root, 'theme');
    const queue = selectedTheme && themes.has(selectedTheme) ? [themes.get(selectedTheme)] : [...themes.values()];
    const visited = new Set();
    const choices = [];
    while (queue.length) {
      const theme = queue.shift();
      if (!theme || visited.has(theme.id)) continue;
      visited.add(theme.id);
      for (const member of theme.members || []) {
        const token = member.node;
        if (token?.kind === 'token' && token.id && (!wantedType || directProperty(token, 'type') === wantedType) && !choices.includes(token.id.slice(1))) {
          choices.push(token.id.slice(1));
        }
      }
      const parent = directProperty(theme, 'parent');
      if (themes.has(parent)) queue.push(themes.get(parent));
    }
    return choices;
  }

  function valueSpec(operation, property, selection) {
    const spec = {...baseValueSpec(operation, property, selection)};
    if (operation !== 'content') {
      const current = authoredValue(selection, property);
      spec.authored = current.authored;
      if (current.authored) spec.currentValue = current.value;
    }
    spec.help = propertyHelp[property] || ({
      length: 'Choose a number and one of the units available for this property.',
      reference: 'Choose an existing authored ID; references are checked exactly.',
      name: 'Choose a token defined by a theme in this document.',
      constraint: 'Attach this edge or center to the canvas or to a sibling anchor.',
    }[spec.kind] || `Changes the authored ${property.replaceAll('-', ' ')} value on this item.`);
    if (spec.kind === 'length') {
      spec.units = ['pt'];
      if (spec.allowPercent !== false) spec.units.push('%');
      if (spec.allowFraction) spec.units.push('fr');
      if (spec.allowAuto !== false) spec.units.unshift('auto');
    }
    if (spec.kind === 'reference') {
      const kind = property === 'theme' ? 'theme' : 'style';
      spec.choices = nodesByKind(selection?.root, kind);
      if (!spec.choices.length) spec.help = `Create a compatible ${kind} before setting this reference.`;
    }
    if (spec.kind === 'name') {
      spec.choices = themeTokenChoices(selection, property);
      if (!spec.choices.length) spec.help = 'The selected theme has no compatible token for this property; create one in the theme first.';
    }
    if (spec.kind === 'constraint') {
      const horizontal = ['left', 'right', 'center-x'].includes(property);
      const anchors = horizontal ? ['left', 'right', 'center-x'] : ['top', 'bottom', 'center-y'];
      const siblings = (selection?.parent?.members || []).map((member) => member.node).filter((node) => node?.id && node.id !== selection.target);
      spec.targets = ['canvas', ...siblings.map((node) => node.id)];
      spec.anchors = anchors;
    }
    return spec;
  }

  function propertyGroup(property) {
    if (property.startsWith('margin')) return 'Outer spacing';
    if (property.startsWith('padding')) return 'Inner spacing';
    if (property.startsWith('border') || ['background', 'style'].includes(property)) return 'Appearance';
    if (['font', 'size', 'line-height', 'color', 'align', 'bold', 'italic', 'level', 'font-token', 'size-token', 'line-height-token', 'color-token'].includes(property)) return 'Typography';
    if (['width', 'height', 'min-width', 'max-width', 'min-height', 'max-height', 'flex-grow', 'flex-shrink'].includes(property)) return 'Size';
    if (['orphans', 'widows', 'keep-together', 'keep-with-next', 'split', 'repeat-header', 'page-numbers', 'page-number-format', 'page-total-alias'].includes(property)) return 'Pagination';
    if (canvasItemConstraints.includes(property) || property.startsWith('default-')) return 'Position';
    return 'General';
  }

  function buildPayload(workspace, selection, operation, property, rawValue) {
    if (!workspace?.source_revision || !workspace?.revision) throw new Error('Exact source and plan revisions are unavailable');
    if (!operations(selection).includes(operation) || !propertiesForSelection(selection, operation).includes(property)) throw new Error('Handle is unavailable for this selection');
    const payload = {
      source_revision: workspace.source_revision,
      plan_revision: workspace.revision,
      scenario: workspace.scenario || '',
      operation,
      target: selection.target,
      property,
    };
    const spec = valueSpec(operation, property, selection);
    if (operation === 'content') {
      if (property === 'runs') {
        if (!Array.isArray(rawValue) || rawValue.length === 0 || rawValue.length > 7) throw new Error('Rich text requires one through seven addressed runs');
        payload.runs = rawValue.map(run => {
          if (!/^@[A-Za-z][A-Za-z0-9_-]*$/.test(run?.target || '')) throw new Error('Every rich-text run needs an exact readable @id');
          return {target: run.target, text: String(run.text ?? '')};
        });
      } else {
        payload.text = String(rawValue ?? '');
      }
    } else if (operation === 'flow') {
      const destination = String(rawValue || '').trim();
      if (!/^@[A-Za-z][A-Za-z0-9_-]*$/.test(destination) || !flowDestinations(selection).some((node) => node.id === destination)) {
        throw new Error('Choose an existing body, row, or column destination');
      }
      payload.new_parent = destination;
    } else if (spec.kind === 'constraint') {
      const match = String(rawValue || '').trim().match(/^(canvas|@[A-Za-z][A-Za-z0-9_-]*)\.(left|right|center-x|top|bottom|center-y)(?:\s*([+-])\s*(\d+(?:\.\d+)?)pt)?$/);
      if (!match) throw new Error('Use canvas.left or @sibling.right + 8pt');
      payload.text = match[1];
      payload.kind = match[2];
      if (match[3]) payload.points = Number(match[4]) * (match[3] === '-' ? -1 : 1);
    } else if (spec.kind === 'length') {
      const length = String(rawValue || '').trim().toLowerCase();
      if (length === 'auto') {
        if (spec.allowAuto === false) throw new Error('Use a physical size in points');
        payload.length = length;
      }
      else {
        const match = length.match(/^(\d+(?:\.\d+)?)(%|pt|fr)?$/);
        if (!match) throw new Error('Use auto, a percentage such as 50%, a physical size such as 48pt, or a fraction such as 1fr');
        const value = Number(match[1]);
        const unit = match[2] || 'pt';
        if (unit === 'fr' && (!spec.allowFraction || !Number.isInteger(value) || value <= 0 || value > 4294967295)) throw new Error('Fractions are available only on the parent flow axis and must be positive whole values such as 1fr or 2fr');
        if (unit === '%' && spec.allowPercent === false) throw new Error('Percentages are unavailable here; use a physical size in points');
        const maximum = unit === '%' ? 100 : unit === 'fr' ? 4294967295 : 1000000;
        if (!Number.isFinite(value) || value < 0 || (spec.positive !== false && value === 0) || value > maximum) throw new Error(`Use ${spec.positive === false ? 'a non-negative' : 'a positive'} ${unit === '%' ? 'percentage up to 100%' : 'physical size'}`);
        if (spec.pointsOnly && unit === 'pt') payload.points = value;
        else if (match[2]) payload.length = `${value}${unit}`;
        else payload.points = value;
      }
    } else if (spec.kind === 'color') {
      const color = String(rawValue || '').toLowerCase();
      if (!/^#[0-9a-f]{6}$/.test(color)) throw new Error('Use a six-digit color such as #315ee8');
      payload.color = color;
    } else if (spec.kind === 'choice') {
      const choice = String(rawValue || '').trim();
      if (!spec.choices.includes(choice)) throw new Error(`Choose one of: ${spec.choices.join(', ')}`);
      payload[spec.field || 'kind'] = choice;
    } else if (spec.kind === 'reference') {
      const reference = String(rawValue || '').trim();
      if (!/^@[A-Za-z][A-Za-z0-9_-]*$/.test(reference)) throw new Error(`Use one exact ${spec.prefix || '@'}name reference`);
      payload.text = reference;
    } else if (spec.kind === 'name') {
      const name = String(rawValue || '').trim();
      if (!name || /\s/.test(name) || name.includes('\0')) throw new Error('Use one theme token name without whitespace');
      payload.text = name;
    } else if (spec.kind === 'text') {
      const value = String(rawValue || '');
      if (spec.required && !value.trim()) throw new Error(`${spec.label} is required`);
      payload.text = value;
    } else if (spec.kind === 'boolean') {
      const boolean = String(rawValue).trim();
      if (!spec.choices.includes(boolean)) throw new Error('Choose true or false');
      payload.bool = boolean === 'true';
    } else if (spec.kind === 'integer') {
      const value = Number(rawValue);
      if (!Number.isInteger(value) || String(rawValue).trim() === '' || value < spec.min || value > spec.max) throw new Error(`Use a whole number from ${spec.min} through ${spec.max}`);
      payload[spec.field || 'count'] = value;
    } else {
      const value = Number(rawValue);
      if (!Number.isFinite(value) || String(rawValue).trim() === '' || value < spec.min || value > spec.max) {
        throw new Error(spec.field === 'number' ? 'Factor must be a finite number in range' : 'Points must be a finite non-negative number');
      }
      if (spec.field === 'number') payload.number = value;
      else payload.points = value;
    }
    return payload;
  }

  function defaultValue(spec) {
    if (spec.currentValue !== undefined) return String(spec.currentValue);
    if (spec.defaultValue !== undefined) return String(spec.defaultValue);
    if (spec.kind === 'color') return '#315ee8';
    if (spec.kind === 'constraint') return 'canvas.left';
    if (spec.kind === 'length') return spec.allowAuto === false ? (spec.positive === false ? '0pt' : '1pt') : 'auto';
    if (spec.kind === 'text' || spec.kind === 'multiline') return '';
    if (spec.kind === 'reference' || spec.kind === 'name') return '';
    if (spec.kind === 'integer') return String(spec.min ?? 1);
    if (spec.kind === 'choice' || spec.kind === 'boolean') return String(spec.choices?.[0] ?? '');
    return String(spec.min ?? 0);
  }

  function buildResetPayload(workspace, selection, operation, property) {
    if (!workspace?.source_revision || !workspace?.revision) throw new Error('Exact source and plan revisions are unavailable');
    if (!operations(selection).includes(operation) || !propertiesForSelection(selection, operation).includes(property)) throw new Error('Handle is unavailable for this selection');
    if (!authoredValue(selection, property).authored) throw new Error('This property is already using its inherited or built-in value');
    return {source_revision: workspace.source_revision, plan_revision: workspace.revision, scenario: workspace.scenario || '', operation, target: selection.target, property, reset: true};
  }

  function propertiesForSelection(selection, operation) {
    if (operation === 'region') return properties(operation, selection);
    if (operation !== 'table') return properties(operation, selection);
    if (selection?.node?.kind === 'table') return ['caption', 'split', 'repeat-header'];
    if (selection?.node?.kind === 'table-column') return ['width', 'min-width', 'max-width'];
    if (selection?.node?.kind === 'table-row') return ['keep-together', 'keep-with-next', 'orphans', 'widows'];
    if (selection?.node?.kind === 'cell') return ['header-cell', 'vertical-align', 'colspan', 'rowspan'];
    return [];
  }

  function flowDestinations(selection) {
    if (!selection?.root || !flowKinds.has(selection.node?.kind)) return [];
    const descendants = new Set();
    (function mark(node) {
      if (!node) return;
      if (node.id) descendants.add(node.id);
      for (const member of node.members || []) mark(member.node);
    })(selection.node);
    const result = [];
    (function walk(node) {
      if (!node) return;
      if (flowParents.has(node.kind) && node.id && !descendants.has(node.id)) {
        const allowed = node.kind === 'body' || ['heading', 'paragraph', 'use'].includes(selection.node.kind);
        if (allowed) result.push(node);
      }
      for (const member of node.members || []) walk(member.node);
    })(selection.root);
    return result;
  }

  return Object.freeze({coreFonts, findSelection, findTextSelectionAtLine, operations, properties: propertiesForSelection, propertyGroup, authoredValue, contentState, valueSpec, defaultValue, buildPayload, buildResetPayload, flowDestinations});
});
