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

export function updateBoundData(dataText, path, nextText) {
  const parts = editablePathParts(path);
  let data;
  try {
    data = JSON.parse(dataText);
  } catch (error) {
    throw new Error(`JSON data is invalid: ${normalizeFailure(error)}`);
  }
  if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('JSON data must be an object.');
  let parent = data;
  for (const part of parts.slice(0, -1)) {
    if (!hasPathMember(parent, part)) {
      throw new Error(`Binding "${path}" does not resolve to an editable JSON value.`);
    }
    parent = parent[part];
    if (!parent || typeof parent !== 'object') {
      throw new Error(`Binding "${path}" does not resolve to an editable JSON value.`);
    }
  }
  const leaf = parts.at(-1);
  if (!hasPathMember(parent, leaf)) throw new Error(`Binding "${path}" is missing from JSON data.`);
  parent[leaf] = coerceLike(parent[leaf], nextText);
  return `${JSON.stringify(data, null, 2)}\n`;
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
  return {editable: true, mode: 'data', binding: path, value: String(value ?? '')};
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

function editablePathParts(path) {
  if (typeof path === 'string' && path.startsWith('/')) return decodePointer(path);
  if (simplePath.test(path || '')) return path.split('.');
  throw new Error('Only direct dotted JSON bindings or RFC 6901 JSON pointers can be edited on the page.');
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

function coerceLike(current, nextText) {
  if (typeof current === 'number') {
    const number = Number(nextText);
    if (!Number.isFinite(number)) throw new Error('Enter a valid number for this binding.');
    return number;
  }
  if (typeof current === 'boolean') {
    if (nextText === 'true') return true;
    if (nextText === 'false') return false;
    throw new Error('Enter true or false for this binding.');
  }
  if (current === null) return nextText === 'null' ? null : nextText;
  return nextText;
}
