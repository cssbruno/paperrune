const simplePath = /^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$/;

export function normalizeFailure(value, fallback = 'The playground failed unexpectedly.') {
  if (value instanceof Error && value.message.trim()) return value.message.trim();
  if (typeof value === 'string' && value.trim()) return value.trim();
  if (value && typeof value.message === 'string' && value.message.trim()) return value.message.trim();
  return fallback;
}

export function findNodeByID(root, id) {
  if (!root || !id) return null;
  if (root.id === id) return root;
  for (const member of root.members || []) {
    const found = findNodeByID(member.node, id);
    if (found) return found;
  }
  return null;
}

export function outlineNodes(root, limit = 180) {
  const rows = [];
  function walk(node, depth) {
    if (!node || rows.length >= limit) return;
    if (node.id && !['style', 'token', 'theme', 'schema', 'schema-field', 'schema-object-type'].includes(node.kind)) {
      rows.push({id: node.id, kind: node.kind, depth});
    }
    for (const member of node.members || []) walk(member.node, depth + (node.id ? 1 : 0));
  }
  walk(root, 0);
  return rows;
}

export function contentDescriptor(node, dataText = '') {
  if (!node) {
    return {editable: false, reason: 'This layout fragment has no directly editable text.'};
  }

  const bind = property(node, 'bind');
  if (bind) return bindingDescriptor(bind.value, dataText);

  const text = property(node, 'text');
  if (text) {
    if (text.value?.kind === 'string') {
      return sourceDescriptor(node, text.value.string_value ?? '');
    }
    return {editable: false, computed: true, reason: 'This text is computed by an expression. Edit its source or input data.'};
  }

  if (node.kind === 'text' && node.value) {
    if (node.value.kind !== 'string') {
      return {editable: false, computed: true, reason: 'This text is computed by an expression. Edit its source or input data.'};
    }
    return sourceDescriptor(node, node.value.string_value ?? '');
  }

  const textChildren = (node.members || []).map((member) => member.node).filter((child) => child?.kind === 'text');
  if (textChildren.length === 1 && textChildren[0].value?.kind === 'string') {
    return sourceDescriptor(node.id ? node : textChildren[0], textChildren[0].value.string_value ?? '');
  }
  if (textChildren.length > 1) {
    return {editable: false, reason: 'This block contains multiple styled text runs. Edit it in Paper source.'};
  }
  return {editable: false, reason: 'This block has no authored text value.'};
}

export function pickHitTarget(hit, astRoot, dataText = '') {
  const fragments = hit?.Fragments || hit?.fragments || [];
  for (const fragment of fragments) {
    const node = findNodeForFragment(astRoot, fragment);
    if (!node) continue;
    const content = contentDescriptor(node, dataText);
    if (content.editable || content.computed || content.binding) return {id: node.id, node, fragment, content};
  }
  return null;
}

export function traceBindingDescriptor(trace, dataText = '') {
  const provenance = trace?.provenance || trace?.Provenance || trace;
  const bindings = provenance?.bindings || provenance?.Bindings || [];
  const binding = bindings.find((entry) => {
    const pointer = entry?.json_pointer ?? entry?.jsonPointer ?? entry?.JSONPointer;
    return typeof pointer === 'string' && pointer.startsWith('/');
  });
  if (!binding) {
    return {editable: false, reason: 'This repeated binding could not be resolved to a concrete JSON value.'};
  }

  const path = binding.path ?? binding.Path ?? '';
  const pointer = binding.json_pointer ?? binding.jsonPointer ?? binding.JSONPointer;
  let data;
  try {
    data = JSON.parse(dataText);
  } catch {
    return {editable: false, binding: path, pointer, reason: 'Fix the JSON data before editing this binding.'};
  }

  let value;
  try {
    value = valueAtPointer(data, pointer);
  } catch {
    return {
      editable: false,
      binding: path,
      pointer,
      reason: `Binding "${path || pointer}" is missing from JSON data.`,
    };
  }
  if (value && typeof value === 'object') {
    return {
      editable: false,
      binding: path,
      pointer,
      reason: 'Object and list bindings must be edited in JSON data.',
    };
  }
  return {editable: true, mode: 'data', binding: path, pointer, value: String(value ?? '')};
}

export function fragmentBox(fragment) {
  if (!fragment) return null;
  return fragment.ContentBox || fragment.content_box || fragment.BorderBox || fragment.border_box || null;
}

export function boxAsPercent(box, pageWidth, pageHeight, pageX = 0, pageY = 0) {
  if (!box || !(pageWidth > 0) || !(pageHeight > 0)) return null;
  const x = numberField(box, 'X', 'x');
  const y = numberField(box, 'Y', 'y');
  const width = numberField(box, 'Width', 'width');
  const height = numberField(box, 'Height', 'height');
  if (![x, y, width, height].every(Number.isFinite)) return null;
  return {
    left: (x - pageX) / pageWidth * 100,
    top: (y - pageY) / pageHeight * 100,
    width: Math.max(0.8, width / pageWidth * 100),
    height: Math.max(0.8, height / pageHeight * 100),
  };
}

function bindingDescriptor(scalar, dataText) {
  const path = scalar?.string_value ?? scalar?.expression_value ?? '';
  if (!simplePath.test(path)) {
    return {
      editable: false,
      binding: path,
      computed: true,
      reason: 'This binding is an expression. Edit it in JSON data or Paper source.',
    };
  }
  let data;
  try {
    data = JSON.parse(dataText);
  } catch {
    return {editable: false, binding: path, reason: 'Fix the JSON data before editing this binding.'};
  }
  let value = data;
  for (const part of path.split('.')) {
    if (!value || typeof value !== 'object' || !(part in value)) {
      return {editable: false, binding: path, reason: `Binding "${path}" is missing from JSON data.`};
    }
    value = value[part];
  }
  if (value && typeof value === 'object') {
    return {editable: false, binding: path, reason: 'Object and list bindings must be edited in JSON data.'};
  }
  return {
    editable: true,
    mode: 'data',
    binding: path,
    pointer: dottedPathPointer(path),
    value: String(value ?? ''),
  };
}

function dottedPathPointer(path) {
  return `/${path.split('.').map((part) => part.replace(/~/g, '~0').replace(/\//g, '~1')).join('/')}`;
}

function sourceDescriptor(node, value) {
  if (node.id) return {editable: true, mode: 'source', value, target: node.id};
  const offset = nodeSourceOffset(node);
  if (Number.isInteger(offset) && offset >= 0) {
    return {editable: true, mode: 'source', value, sourceOffset: offset};
  }
  return {editable: false, reason: 'This anonymous text has no exact source location.'};
}

function property(node, name) {
  return (node.members || []).find((member) => member.property?.name === name)?.property || null;
}

function findNodeForFragment(root, fragment) {
  const key = String(fragment?.Key || fragment?.key || '');
  const exact = findNodeByID(root, key);
  if (exact) return exact;

  const source = fragment?.Source || fragment?.source;
  const sourceStart = Number(source?.start?.offset ?? source?.Start?.Offset);
  const candidates = [];
  (function walk(node) {
    if (!node) return;
    const start = Number(node.span?.start?.offset ?? node.Span?.Start?.Offset);
    const end = Number(node.span?.end?.offset ?? node.Span?.End?.Offset);
    if (Number.isFinite(sourceStart) && Number.isFinite(start) && Number.isFinite(end) &&
        sourceStart >= start && sourceStart < end) {
      candidates.push({node, width: end - start});
    }
    for (const member of node.members || []) walk(member.node);
  })(root);
  candidates.sort((first, second) => first.width - second.width);
  if (candidates.length) return candidates[0].node;

  let suffix = null;
  (function walk(node) {
    if (!node || suffix) return;
    if (node.id && key.endsWith(node.id)) {
      suffix = node;
      return;
    }
    for (const member of node.members || []) walk(member.node);
  })(root);
  return suffix;
}

function nodeSourceOffset(node) {
  const raw = node?.header_span?.start?.offset ?? node?.HeaderSpan?.Start?.Offset ??
    node?.span?.start?.offset ?? node?.Span?.Start?.Offset;
  return raw === undefined || raw === null ? Number.NaN : Number(raw);
}

function decodePointer(pointer) {
  if (pointer === '') return [];
  if (!pointer.startsWith('/')) throw new Error('Invalid JSON pointer.');
  return pointer.slice(1).split('/').map((part) => {
    if (/~(?:[^01]|$)/.test(part)) throw new Error('Invalid JSON pointer escape.');
    return part.replace(/~1/g, '/').replace(/~0/g, '~');
  });
}

function hasPathMember(parent, part) {
  if (!parent || typeof parent !== 'object') return false;
  if (Array.isArray(parent)) {
    if (!/^(?:0|[1-9][0-9]*)$/.test(part)) return false;
    const index = Number(part);
    return index < parent.length;
  }
  return Object.prototype.hasOwnProperty.call(parent, part);
}

function valueAtPointer(data, pointer) {
  let value = data;
  for (const part of decodePointer(pointer)) {
    if (!hasPathMember(value, part)) throw new Error('JSON pointer does not resolve.');
    value = value[part];
  }
  return value;
}

function numberField(value, first, second) {
  const number = Number(value?.[first] ?? value?.[second]);
  return number;
}
