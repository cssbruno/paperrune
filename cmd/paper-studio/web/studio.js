(() => {
'use strict';

const state = {
  workspace: null,
  revision: '',
  sourceRevision: '',
  scenario: '',
  loadedScenario: '',
  page: 1,
  zoom: 1,
  zoomMode: 'fit-page',
  zoomRender: 0,
  zoomTimer: null,
  resizeTimer: null,
  thumbnailRender: 0,
  detailRender: 0,
  activePageMeta: null,
  pageMeta: new Map(),
  wasmCacheKeys: [],
  inspections: new Map(),
  objectURLs: new Set(),
  selectionFragments: [],
  overlays: new Set(),
  editSelection: null,
  editDraft: null,
  editFeedback: null,
  inlineEdit: null,
  history: {can_undo: false, can_redo: false, undo_count: 0, redo_count: 0},
  committing: false,
  resources: [],
  resourceCatalogEditable: false,
  resourceFormOpen: false,
  review: null,
  authoring: null,
  authoringDraft: {operation: 'template'},
  authoringFeedback: null,
  outlineCollapsed: new Set(),
  pageSetupDraft: null,
  pageSetupFeedback: null,
  loading: false,
  refreshPromise: null,
  delivery: null,
  deliveryRevision: '',
  deliveryLoading: false,
  deliveryDownloading: false,
  deliveryError: '',
  changeStream: null,
};

const $ = (selector) => document.querySelector(selector);
const app = $('#app');
const pageImage = $('#page-image');
const geometryImage = $('#geometry-image');
const inspectionLayer = $('#inspection-layer');
const textSelectionLayer = $('#text-selection-layer');
const overlapPicker = $('#overlap-picker');
const selectionLayer = $('#selection-layer');
const inlineTextEditor = $('#inline-text-editor');
const inlineTextInput = $('#inline-text-input');
const canvasScroll = $('#canvas-scroll');
const studioSessionToken = new URLSearchParams(window.location.hash.slice(1)).get('token') || '';
if (studioSessionToken) window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);

function previewRevisionLocked() {
  return PaperStudioMutationGate.revisionsLocked(
    state.workspace, state.revision, state.sourceRevision, app.classList.contains('is-stale'),
  );
}

function visualMutationsLocked() {
  return PaperStudioMutationGate.visualMutationsLocked(
    state.workspace, state.revision, state.sourceRevision, app.classList.contains('is-stale'), state.committing,
  );
}

function setPreviewStale(stale) {
  app.classList.toggle('is-stale', stale);
  renderVerificationState();
  renderDelivery();
  renderEditControls();
  renderAuthoringControls();
  renderPageSetup();
  renderResources();
  syncInsertTools();
  renderHistoryActions();
  document.querySelectorAll('.font-replacement-apply').forEach((button) => { button.disabled = visualMutationsLocked(); });
}

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (studioSessionToken) headers.set('X-Paper-Studio-Token', studioSessionToken);
  const response = await fetch(path, {cache: 'no-store', ...options, headers});
  const type = response.headers.get('content-type') || '';
  if (!response.ok) {
    const failure = type.includes('json') ? await response.json() : {error: await response.text()};
    const error = new Error(failure.error || `Request failed (${response.status})`);
    error.status = response.status;
    throw error;
  }
  return type.includes('json') ? response.json() : response;
}

async function refresh({quiet = false} = {}) {
  if (state.refreshPromise) {
    await state.refreshPromise;
    if (quiet) return;
  }
  if (state.refreshPromise) return state.refreshPromise;
  const pending = performRefresh({quiet});
  state.refreshPromise = pending;
  try {
    return await pending;
  } finally {
    if (state.refreshPromise === pending) state.refreshPromise = null;
  }
}

async function performRefresh({quiet = false} = {}) {
  state.loading = true;
  if (!quiet) setPreviewStale(true);
  try {
    const query = state.scenario ? `?scenario=${encodeURIComponent(state.scenario)}` : '';
    const workspace = await api(`/api/workspace${query}`);
    const workspaceScenario = workspace.scenario || '';
    const changed = workspace.revision !== state.revision || workspace.source_revision !== state.sourceRevision || workspaceScenario !== state.loadedScenario;
    state.workspace = workspace;
    state.revision = workspace.revision;
    state.sourceRevision = workspace.source_revision;
    state.scenario = workspaceScenario;
    state.loadedScenario = state.scenario;
    await loadHistory();
    if (changed) {
      closeInlineTextEditor();
      clearObjectURLs();
      state.pageMeta.clear();
      state.inspections.clear();
      state.selectionFragments = [];
      state.activePageMeta = null;
      state.delivery = null;
      state.deliveryRevision = '';
      state.deliveryDownloading = false;
      state.deliveryError = '';
      closeOverlapPicker();
      state.page = Math.min(Math.max(1, state.page), Math.max(1, workspace.pages));
      renderWorkspace();
      await loadResources(workspace.revision);
      await loadAuthoring(workspace.revision);
      await loadReview(workspace.revision);
      loadDeliveryStatus(workspace.revision);
      if (workspace.pages) await showPage(state.page);
    } else if (!quiet) {
      renderStatus();
    }
    if (!changed) await loadResources(workspace.revision);
    if (!changed) await loadReview(workspace.revision);
    app.classList.toggle('has-no-plan', !workspace.pages);
    app.classList.remove('is-loading');
  } catch (error) {
    showFailure(error);
  } finally {
    state.loading = false;
    setPreviewStale(false);
  }
}

async function loadHistory() {
  const query = state.scenario ? `?scenario=${encodeURIComponent(state.scenario)}` : '';
  try {
    state.history = await api(`/api/history${query}`);
  } catch (_) {
    state.history = {can_undo: false, can_redo: false, undo_count: 0, redo_count: 0};
  }
  renderHistoryActions();
}

function renderHistoryActions() {
  const locked = visualMutationsLocked();
  const undo = $('#history-undo');
  const redo = $('#history-redo');
  setButtonUnavailable(undo, locked || !state.history.can_undo, locked ? 'Undo waits for the current exact plan and any active edit' : 'Nothing to undo');
  setButtonUnavailable(redo, locked || !state.history.can_redo, locked ? 'Redo waits for the current exact plan and any active edit' : 'Nothing to redo');
  undo.title = locked ? 'Undo waits for the current exact plan and any active edit' : state.history.can_undo ? `Undo ${state.history.undo_label || 'last edit'} (${state.history.undo_count} available)` : 'Nothing to undo';
  redo.title = locked ? 'Redo waits for the current exact plan and any active edit' : state.history.can_redo ? `Redo ${state.history.redo_label || 'last edit'} (${state.history.redo_count} available)` : 'Nothing to redo';
  undo.setAttribute('aria-label', state.history.can_undo ? `Undo ${state.history.undo_label || 'last edit'}` : 'Nothing to undo');
  redo.setAttribute('aria-label', state.history.can_redo ? `Redo ${state.history.redo_label || 'last edit'}` : 'Nothing to redo');
}

function setButtonUnavailable(button, unavailable, reason = '') {
  button.disabled = false;
  button.setAttribute('aria-disabled', String(unavailable));
  if (unavailable) button.dataset.disabledReason = reason;
  else delete button.dataset.disabledReason;
  if (unavailable && reason) button.title = reason;
}

function buttonUnavailable(button) {
  return button?.getAttribute('aria-disabled') === 'true';
}

async function applyHistory(action) {
  if (visualMutationsLocked() || !state.history[`can_${action}`]) return;
  const label = state.history[`${action}_label`] || 'last edit';
  state.committing = true;
  state.editFeedback = {tone: 'working', text: `${action === 'undo' ? 'Undoing' : 'Redoing'} ${label}…`};
  renderEditControls();
  try {
    await api('/api/history', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify({
      source_revision: state.workspace.source_revision, plan_revision: state.workspace.revision, scenario: state.scenario, action,
    })});
    state.editFeedback = {tone: 'success', text: action === 'undo' ? `Undid ${label}` : `Redid ${label}`};
    await refresh();
  } catch (error) {
    state.editFeedback = {tone: error.status === 409 ? 'stale' : 'error', text: error.status === 409 ? 'History changed · refreshed safely' : error.message};
    if (error.status === 409) await refresh();
  } finally {
    state.committing = false;
    renderEditControls();
    renderHistoryActions();
  }
}

function connectChangeStream() {
  state.changeStream?.close?.();
  state.changeStream = null;
  const query = new URLSearchParams();
  if (state.scenario) query.set('scenario', state.scenario);
  if (state.sourceRevision) query.set('source_revision', state.sourceRevision);
  const controller = new AbortController();
  const stream = {close: () => controller.abort()};
  state.changeStream = stream;
  (async () => {
    while (!controller.signal.aborted && state.changeStream === stream) {
      try {
        const response = await fetch(`/api/changes?${query.toString()}`, {
          cache: 'no-store', signal: controller.signal,
          headers: {'X-Paper-Studio-Token': studioSessionToken},
        });
        if (!response.ok || !response.body) throw new Error(`Change stream failed (${response.status})`);
        await readChangeStream(response.body, controller.signal);
      } catch (error) {
        if (controller.signal.aborted) return;
      }
      await new Promise((resolve) => window.setTimeout(resolve, 1000));
    }
  })();
}

async function readChangeStream(body, signal) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffered = '';
  try {
    while (!signal.aborted) {
      const {value, done} = await reader.read();
      if (done) return;
      buffered += decoder.decode(value, {stream: true}).replaceAll('\r\n', '\n');
      for (let boundary = buffered.indexOf('\n\n'); boundary >= 0; boundary = buffered.indexOf('\n\n')) {
        const frame = buffered.slice(0, boundary);
        buffered = buffered.slice(boundary + 2);
        handleChangeFrame(frame);
      }
    }
  } finally {
    reader.releaseLock();
  }
}

function handleChangeFrame(frame) {
  const lines = frame.split('\n');
  if (!lines.some((line) => line === 'event: changed')) return;
  const data = lines.filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trimStart()).join('\n');
  let message;
  try { message = JSON.parse(data); } catch (_) { return; }
  const expected = String(message.source_revision || '');
  if (!expected || expected === state.sourceRevision) return;
  refresh({quiet: true}).then(() => {
    if (state.sourceRevision !== expected) return refresh({quiet: true});
  });
}

async function loadAuthoring(revision) {
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  try {
    const payload = await api(`/api/authoring?revision=${encodeURIComponent(revision)}${scenario}`);
    if (revision !== state.revision) return;
    state.authoring = PaperStudioAuthoringModel.normalize(payload, state.workspace);
  } catch (error) {
    if (error.status !== 409 && revision === state.revision) state.authoring = null;
  }
  renderAuthoringControls();
  syncInsertTools();
}

async function loadResources(revision) {
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  try {
    const payload = await api(`/api/resources?revision=${encodeURIComponent(revision)}&source_revision=${encodeURIComponent(state.workspace?.source_revision||'')}${scenario}`);
    if (revision !== state.revision) return;
    state.resources = PaperStudioResourceModel.normalize(payload, revision, state.workspace?.source_revision, state.workspace?.plan_hash);
    state.resourceCatalogEditable = Boolean(payload.catalog_editable);
  } catch (error) {
    if (error.status === 409 || revision !== state.revision) return;
    state.resources = [];
    state.resourceCatalogEditable = false;
  }
  renderResources();
}

async function loadReview(revision) {
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  try {
    const payload = await api(`/api/review?revision=${encodeURIComponent(revision)}&source_revision=${encodeURIComponent(state.workspace?.source_revision || '')}${scenario}`);
    if (revision !== state.revision) return;
    state.review = PaperStudioReviewModel.normalizeReview(payload, state.workspace);
  } catch (error) {
    if (revision === state.revision && error.status !== 409) state.review = null;
  }
  renderReviewControls();
}

async function submitReview(payload, errorTarget) {
  try {
    const response = await api('/api/review', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
    state.review = PaperStudioReviewModel.normalizeReview(response, state.workspace);
    renderReviewControls();
  } catch (error) {
    if (error.status === 409) await refresh();
    else if (errorTarget?.isConnected) errorTarget.textContent = error.message;
  }
}

function reviewReferencePayload(file, bytes, bitmap, digest) {
  let binary = '';
  const view = new Uint8Array(bytes);
  for (let offset = 0; offset < view.length; offset += 0x8000) binary += String.fromCharCode(...view.subarray(offset, offset + 0x8000));
  return {
    source_revision: state.workspace.source_revision, plan_revision: state.workspace.revision, scenario: state.workspace.scenario || '',
    kind: 'reference', page: state.page, reference_kind: file.type, reference_digest: digest,
    reference_data_base64: btoa(binary), width: bitmap.width, height: bitmap.height, transform: [1, 0, 0, 1, 0, 0],
  };
}

function renderReviewControls() {
  const target = $('#review-controls');
  if (!target) return;
  target.replaceChildren();
  const selected = state.editSelection?.target;
  const comments = (state.review?.comments || []).filter(item => !selected || item.target === selected);
  const annotations = (state.review?.annotations || []).filter(item => !selected || item.target === selected);
  $('#review-count').textContent = String((state.review?.comments?.length || 0) + (state.review?.annotations?.length || 0));
  if (state.review?.reference) {
    const reference = document.createElement('div'); reference.className = 'review-reference';
    reference.textContent = `Reference · ${state.review.reference.diff_status || 'stored'} · ${state.review.reference.changedPixels} changed pixels`;
    target.append(reference);
  }
  for (const item of [...comments, ...annotations]) {
    const entry = document.createElement('article'); entry.className = 'review-entry';
    const meta = document.createElement('small'); meta.textContent = `${item.target} · page ${item.page}`;
    const body = document.createElement('div'); body.textContent = item.body || item.note || item.label || 'Pinned annotation';
    entry.append(meta, body); target.append(entry);
  }
  if (!selected) {
    const empty = document.createElement('span'); empty.className = 'quiet'; empty.textContent = 'Select an addressed node to add a comment or pin.'; target.append(empty);
  } else {
    const form = document.createElement('form'); form.className = 'review-form';
    const label = document.createElement('label'); label.className = 'edit-field';
    const caption = document.createElement('span'); caption.textContent = `Comment on ${selected}`;
    const body = document.createElement('textarea'); body.rows = 3; body.maxLength = 4096; body.placeholder = 'Describe the requested change';
    const error = document.createElement('small'); error.className = 'edit-inline-error'; error.setAttribute('role', 'alert');
    const actions = document.createElement('div'); actions.className = 'edit-actions';
    const pin = document.createElement('button'); pin.type = 'button'; pin.className = 'edit-secondary'; pin.textContent = 'Pin annotation';
    const add = document.createElement('button'); add.type = 'submit'; add.className = 'edit-commit'; add.textContent = 'Add comment';
    label.append(caption, body, error); actions.append(pin, add); form.append(label, actions);
    form.addEventListener('submit', event => {
      event.preventDefault(); error.textContent = '';
      try { void submitReview(PaperStudioReviewModel.commentPayload(state.workspace, selected, state.page, body.value), error); }
      catch (failure) { error.textContent = failure.message; }
    });
    pin.addEventListener('click', () => { error.textContent = ''; void submitReview(PaperStudioReviewModel.annotationPayload(state.workspace, selected, state.page, body.value), error); });
    target.append(form);
  }
  const reference = document.createElement('label'); reference.className = 'review-reference-picker';
  const referenceCaption = document.createElement('span'); referenceCaption.textContent = 'Compare PNG/JPEG reference';
  const file = document.createElement('input'); file.type = 'file'; file.accept = 'image/png,image/jpeg';
  const referenceError = document.createElement('small'); referenceError.className = 'edit-inline-error'; referenceError.setAttribute('role', 'alert');
  file.addEventListener('change', async () => {
    const selectedFile = file.files?.[0]; if (!selectedFile) return;
    referenceError.textContent = '';
    if (selectedFile.size > 8 * 1024 * 1024) { referenceError.textContent = 'Reference images must be 8 MB or smaller'; return; }
    try {
      const bytes = await selectedFile.arrayBuffer();
      const hash = await crypto.subtle.digest('SHA-256', bytes);
      const digest = [...new Uint8Array(hash)].map(value => value.toString(16).padStart(2, '0')).join('');
      const bitmap = await createImageBitmap(selectedFile);
      const payload = reviewReferencePayload(selectedFile, bytes, bitmap, digest); bitmap.close();
      await submitReview(payload, referenceError);
    } catch (failure) { referenceError.textContent = failure.message; }
  });
  reference.append(referenceCaption, file, referenceError); target.append(reference);
}

function renderResources() {
  const add = $('#resource-add');
  if (add) {
    add.hidden = !state.resourceCatalogEditable;
    add.disabled = visualMutationsLocked();
    add.textContent = state.resourceFormOpen ? 'Close' : 'Add';
    add.setAttribute('aria-expanded', String(state.resourceFormOpen));
  }
  const target=$('#resources'); target.replaceChildren(); $('#resource-count').textContent=String(state.resources.length);
  if (state.resourceFormOpen && state.resourceCatalogEditable) target.append(resourceCatalogForm());
  if (!state.resources.length) { const empty=document.createElement('span');empty.className='quiet resource-empty';empty.textContent='No catalog assets';target.append(empty);return; }
  for (const item of state.resources) {
    const row=document.createElement('article');row.className='resource-item';
    const heading=document.createElement('div');heading.className='resource-name';const name=document.createElement('span');name.textContent=item.name;const type=document.createElement('span');type.className='resource-type';type.textContent=item.mediaType;heading.append(name,type);
    const meta=document.createElement('div');meta.className='resource-meta';meta.textContent=PaperStudioResourceModel.usageLabel(item);
    const digest=document.createElement('div');digest.className='resource-meta resource-digest';digest.textContent=`sha256 ${item.digest.slice(0,12)}…`;
    row.append(heading,meta,digest);
    const lifecycle=[];if(item.kind==='font'){lifecycle.push(`license ${item.license}`);if(item.fallback.length)lifecycle.push(`fallback ${item.fallback.join(' → ')}`);}else{if(item.defaultFocusX!==null||item.defaultFocusY!==null)lifecycle.push(`default focus ${item.defaultFocusX??'auto'},${item.defaultFocusY??'auto'}`);if(item.replaces)lifecycle.push(`replaces ${item.replaces}`);}if(lifecycle.length){const line=document.createElement('div');line.className='resource-meta resource-lifecycle';line.textContent=lifecycle.join(' · ');row.append(line);}
    for (const usage of item.usages) { const button=document.createElement('button');button.className='resource-use';button.type='button';button.textContent=`${usage.node||'anonymous'} · ${usage.decorative?'decorative':usage.alt||'missing alt'} · focus ${usage.focus_x??'auto'},${usage.focus_y??'auto'}${usage.scenario?` · ${usage.scenario}`:''}`;button.addEventListener('click',()=>{const found=walkNodes(state.workspace?.ast?.root).find(({node})=>node.id===usage.node);if(found)selectSourceNode(found.node,document.querySelector(`.outline-row[data-key="${CSS.escape(usage.node)}"]`));});row.append(button); }
    if(item.kind==='image'&&item.replaces){const previous=state.resources.find(candidate=>candidate.name===item.replaces);for(const usage of previous?.usages||[]){const replace=document.createElement('button');replace.className='resource-use resource-replace';replace.type='button';replace.disabled=visualMutationsLocked();replace.textContent=`Replace ${usage.node} with ${item.name}`;replace.addEventListener('click',()=>commitResourceReplacement(usage.node,item.name));row.append(replace);}}
    if (state.resourceCatalogEditable && item.usages.length === 0) { const remove=document.createElement('button'); remove.className='resource-use resource-remove'; remove.type='button'; remove.disabled=visualMutationsLocked(); remove.textContent=`Remove ${item.name}`; remove.addEventListener('click',()=>removeResource(item.name)); row.append(remove); }
    target.append(row);
  }
}

function resourceFormField(labelText, name, {type = 'text', placeholder = '', required = false, min = '', max = '', step = ''} = {}) {
  const label = document.createElement('label');
  label.className = 'resource-field';
  const caption = document.createElement('span'); caption.textContent = labelText;
  const input = document.createElement('input');
  Object.assign(input, {type, name, placeholder, required});
  if (min !== '') input.min = min;
  if (max !== '') input.max = max;
  if (step !== '') input.step = step;
  label.append(caption, input);
  return label;
}

function resourceCatalogForm() {
  const form = document.createElement('form');
  form.className = 'resource-form';
  form.noValidate = true;
  const heading = document.createElement('strong'); heading.textContent = 'Add catalog resource';
  const name = resourceFormField('Catalog name', 'name', {placeholder: 'hero-image', required: true});
  name.querySelector('input').pattern = '[a-z0-9][a-z0-9._-]*';
  const path = resourceFormField('Project-relative path', 'path', {placeholder: 'assets/hero.png', required: true});
  const mediaLabel = document.createElement('label'); mediaLabel.className = 'resource-field';
  const mediaCaption = document.createElement('span'); mediaCaption.textContent = 'Media type';
  const media = document.createElement('select'); media.name = 'mediaType'; media.required = true;
  for (const value of ['image/png', 'image/jpeg', 'font/ttf', 'font/otf', 'font/woff2']) {
    const option = document.createElement('option'); option.value = value; option.textContent = value; media.append(option);
  }
  mediaLabel.append(mediaCaption, media);
  const advanced = document.createElement('details'); advanced.className = 'resource-advanced';
  const summary = document.createElement('summary'); summary.textContent = 'Type-specific metadata';
  const fields = document.createElement('div'); fields.className = 'resource-fields';
  fields.append(
    resourceFormField('Font family', 'family', {placeholder: 'Readable Sans'}),
    resourceFormField('License', 'license', {placeholder: 'OFL-1.1'}),
    resourceFormField('Weight', 'weight', {type: 'number', min: '1', max: '1000', step: '1'}),
    resourceFormField('Style', 'style', {placeholder: 'normal'}),
    resourceFormField('Fallbacks', 'fallback', {placeholder: 'Arial, sans-serif'}),
    resourceFormField('Replaces image', 'replaces', {placeholder: 'old-hero'}),
    resourceFormField('Focus X', 'focusX', {type: 'number', min: '0', max: '1', step: '0.01'}),
    resourceFormField('Focus Y', 'focusY', {type: 'number', min: '0', max: '1', step: '0.01'}),
  );
  advanced.append(summary, fields);
  const error = document.createElement('output'); error.className = 'field-error'; error.setAttribute('aria-live', 'polite');
  const actions = document.createElement('div'); actions.className = 'edit-actions';
  const cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'edit-secondary'; cancel.textContent = 'Cancel';
  const submit = document.createElement('button'); submit.type = 'submit'; submit.className = 'edit-commit'; submit.textContent = 'Add resource';
  cancel.addEventListener('click', () => { state.resourceFormOpen = false; renderResources(); $('#resource-add')?.focus(); });
  actions.append(cancel, submit);
  form.append(heading, name, path, mediaLabel, advanced, error, actions);
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    error.textContent = '';
    if (!form.checkValidity()) { form.reportValidity(); return; }
    const values = Object.fromEntries(new FormData(form));
    values.fallback = String(values.fallback || '').split(',').map(value => value.trim()).filter(Boolean);
    let payload;
    try { payload = PaperStudioResourceModel.catalogAddPayload(state.workspace, values); }
    catch (failure) { error.textContent = failure.message; return; }
    state.committing = true; submit.disabled = true; submit.textContent = 'Adding…';
    try {
      await api('/api/resources', {method:'POST', headers:{'content-type':'application/json'}, body:JSON.stringify(payload)});
      state.resourceFormOpen = false;
      await refresh();
    } catch (failure) {
      if (failure.status === 409) await refresh(); else error.textContent = failure.message;
    } finally { state.committing = false; renderResources(); }
  });
  queueMicrotask(() => form.querySelector('input')?.focus());
  return form;
}

function toggleResourceForm() {
  if (visualMutationsLocked() || !state.workspace || !state.resourceCatalogEditable) return;
  state.resourceFormOpen = !state.resourceFormOpen;
  renderResources();
}

async function removeResource(name) {
  if (visualMutationsLocked() || !state.workspace || !state.resourceCatalogEditable || !window.confirm(`Remove ${name} from the explicit catalog?`)) return;
  let payload;
  try { payload = PaperStudioResourceModel.catalogRemovePayload(state.workspace, name); } catch (error) { showFailure(error); return; }
  state.committing = true; renderResources();
  try { await api('/api/resources', {method:'POST', headers:{'content-type':'application/json'}, body:JSON.stringify(payload)}); await refresh(); }
  catch (error) { if (error.status === 409) await refresh(); else showFailure(error); }
  finally { state.committing = false; renderResources(); }
}

async function commitResourceReplacement(target, resource) {
  if(visualMutationsLocked()||!state.workspace)return;state.committing=true;renderResources();
  let payload;try{payload=PaperStudioResourceModel.replacementPayload(state.workspace,state.resources,target,resource);}catch(error){state.committing=false;showFailure(error);renderResources();return;}
  try{await api('/api/edit',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});await refresh();}catch(error){if(error.status===409)await refresh();else showFailure(error);}finally{state.committing=false;renderResources();}
}

function renderWorkspace() {
  const workspace = state.workspace;
  $('#file-name').textContent = workspace.file.split(/[\\/]/).pop();
  $('#revision').textContent = workspace.revision;
  $('#page-total').textContent = workspace.pages;
  renderSource(workspace.source || '', workspace.diagnostics || []);
  renderOutline(workspace.ast?.root);
  renderScenarios(workspace.ast?.root);
  renderIssues(workspace.diagnostics || []);
  renderThumbnails(workspace.pages, workspace.page_rail || []);
  renderBaseline(workspace.baseline || {status: 'none'});
  reconcileEditSelection();
  renderAuthoringControls();
  renderPageSetup();
  renderDelivery();
  renderStatus();
  syncInsertTools();
}

function openAuthoringTools() {
  app.classList.remove('right-collapsed');
  const disclosure = document.querySelector('.authoring-disclosure');
  disclosure.open = true;
  disclosure.scrollIntoView({block: 'nearest', behavior: 'smooth'});
}

function pulseInsertTool(button) {
  document.querySelectorAll('.insert-tool.is-active').forEach(item => item.classList.remove('is-active'));
  button.classList.add('is-active');
  window.setTimeout(() => button.classList.remove('is-active'), 700);
}

function syncInsertTools() {
  const tooling = globalThis.PaperStudioToolingModel;
  const descriptions = {header: 'Add a repeating page header', footer: 'Add a repeating page footer', heading: 'Add a heading', paragraph: 'Add a text paragraph', image: 'Add an image block', table: 'Add a table', section: 'Add a semantic section', 'page-break': 'Add an explicit page break'};
  document.querySelectorAll('.insert-tool[data-template]').forEach(button => {
    const unavailable = !tooling || !tooling.availability(state.authoring, button.dataset.template) || visualMutationsLocked();
    const reason = visualMutationsLocked() ? 'Wait for the current exact plan before inserting content' : `Select a compatible addressed parent before adding ${button.dataset.template.replaceAll('-', ' ')}`;
    setButtonUnavailable(button, unavailable, reason);
    if (!unavailable) button.title = descriptions[button.dataset.template];
  });
  const data = document.querySelector('.insert-tool[data-authoring-operation="binding"]');
  if (data) {
    const unavailable = !state.authoring || visualMutationsLocked() || (!state.authoring.bindingTargets.length && !state.authoring.documentTarget);
    const reason = visualMutationsLocked() ? 'Wait for the current exact plan before editing data bindings' : 'Add a schema and select bindable content to enable data binding';
    setButtonUnavailable(data, unavailable, reason);
    if (!unavailable) data.title = 'Bind content to a schema path';
  }
  const enabled = document.querySelectorAll('.insert-tool:not([aria-disabled="true"])').length;
  $('#insert-context').textContent = visualMutationsLocked() ? 'Tools wait for the current exact plan.' : enabled ? 'Available tools match the current source context.' : 'Select an addressed compatible parent to enable tools.';
}

function prepareInsertTool(button) {
  if (buttonUnavailable(button) || !state.authoring) return;
  const template = button.dataset.template;
  try {
    state.authoringDraft = PaperStudioToolingModel.prepareTemplateDraft(
      state.workspace, state.authoring, template, state.authoringDraft.target,
    );
    state.authoringFeedback = null;
    renderAuthoringControls();
    openAuthoringTools();
    pulseInsertTool(button);
  } catch (error) {
    state.authoringFeedback = {tone: 'error', text: error.message};
    renderAuthoringControls();
    openAuthoringTools();
  }
}

function prepareDataTool(button) {
  if (buttonUnavailable(button) || !state.authoring) return;
  state.authoringDraft = state.authoring.bindingTargets.length
    ? {operation: 'binding'}
    : {operation: 'schema', target: state.authoring.documentTarget, id: '@new-schema'};
  state.authoringFeedback = null;
  renderAuthoringControls();
  openAuthoringTools();
  pulseInsertTool(button);
}

async function loadDeliveryStatus(revision) {
  if (!revision || state.deliveryLoading || state.deliveryRevision === revision) return;
  state.deliveryLoading = true;
  state.deliveryError = '';
  renderDelivery();
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  try {
    const payload = await api(`/api/delivery?revision=${encodeURIComponent(revision)}${scenario}`);
    if (revision !== state.revision) return;
    state.delivery = payload;
    state.deliveryRevision = revision;
  } catch (error) {
    if (error.status !== 409 && revision === state.revision) state.deliveryError = error.message;
  } finally {
    if (revision === state.revision) state.deliveryLoading = false;
    renderDelivery();
  }
}

function renderDelivery() {
  const status = $('#delivery-status');
  const summary = $('#delivery-summary');
  const actions = $('#delivery-actions');
  if (!status || !summary || !actions) return;
  actions.replaceChildren();
	if (state.deliveryLoading) {
		status.textContent = 'Inspecting…';
		summary.textContent = 'Checking the exact retained plan…';
    return;
  }
  if (state.deliveryError) {
    status.textContent = 'Unavailable';
    summary.textContent = state.deliveryError;
    return;
  }
  const delivery = state.delivery;
  if (!delivery) {
    status.textContent = 'Not inspected';
		summary.textContent = 'Planning, export, and publish are separate statuses.';
    return;
  }
  const preflight = delivery.preflight || {};
  const exportStatus = delivery.export || {};
  const publish = delivery.publish || {};
  status.textContent = exportStatus.status === 'ready' ? 'Ready to export' : preflight.status;
  summary.textContent = `Plan ${preflight.status} · Export ${exportStatus.status}`;
  if (exportStatus.status === 'ready') {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'delivery-export';
    button.textContent = state.deliveryDownloading ? 'Generating PDF…' : 'Export PDF';
    button.disabled = state.deliveryDownloading || previewRevisionLocked();
    button.addEventListener('click', exportDeliveryPDF);
    actions.append(button);
  }
  const publishLine = document.createElement('span');
  publishLine.className = 'delivery-publish';
  publishLine.textContent = `Publish: ${publish.reason || 'separate authorized capability'}`;
  actions.append(publishLine);
}

async function exportDeliveryPDF() {
  if (state.deliveryDownloading || previewRevisionLocked()) return;
  state.deliveryDownloading = true;
  state.deliveryError = '';
  renderDelivery();
  try {
    const base = (state.workspace?.file || 'document').split(/[\\/]/).pop().replace(/\.[^.]+$/, '') || 'document';
    const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
    const response = await api(`/api/export.pdf?revision=${encodeURIComponent(state.revision)}${scenario}`);
    await downloadStudioResponse(response, `${base}.pdf`);
  } catch (error) {
    state.deliveryError = error.message;
  } finally {
    state.deliveryDownloading = false;
    renderDelivery();
  }
}

async function downloadStudioResponse(response, fallbackName) {
  const disposition = response.headers.get('content-disposition') || '';
  const match = /filename="([A-Za-z0-9._ -]{1,200})"/.exec(disposition);
  const filename = match ? match[1] : fallbackName;
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  state.objectURLs.add(url);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.hidden = true;
  document.body.append(link);
  link.click();
  link.remove();
  window.setTimeout(() => {
    URL.revokeObjectURL(url);
    state.objectURLs.delete(url);
  }, 1000);
}

function renderPageSetup() {
  const target = $('#page-setup-controls');
  if (!target) return;
  target.replaceChildren();
  let current;
  try {
    current = PaperStudioPageSetupModel.dimensions(state.workspace);
  } catch (error) {
    $('#page-size-summary').textContent = 'Unavailable';
    target.innerHTML = '<span class="quiet">Add one addressed page master to enable page setup.</span>';
    return;
  }
  if (!state.pageSetupDraft || state.pageSetupDraft.target !== current.target || state.pageSetupDraft.revision !== state.revision) {
    state.pageSetupDraft = {
      target: current.target, revision: state.revision, preset: current.preset, orientation: current.orientation, unit: 'mm',
      width: Number((current.width * 25.4 / 72).toFixed(2)), height: Number((current.height * 25.4 / 72).toFixed(2)),
    };
  }
  const draft = state.pageSetupDraft;
  $('#page-size-summary').textContent = `${draft.preset} · ${draft.orientation}`;
  const form = document.createElement('form');
  form.className = 'page-setup-form';
  form.addEventListener('submit', event => { event.preventDefault(); commitPageSetup({...draft}); });
  const preset = authoringSelect('Preset', [...Object.keys(PaperStudioPageSetupModel.presets), 'Custom'], draft.preset, value => {
    draft.preset = value;
    renderPageSetup();
  });
  preset.classList.add('page-size-preset');
  form.append(preset, authoringSelect('Orientation', ['portrait', 'landscape'], draft.orientation, value => {
    draft.orientation = value;
    renderPageSetup();
  }));
  if (draft.preset === 'Custom') {
    form.append(
      pageSetupInput('Width', draft.width, value => { draft.width = value; }),
      pageSetupInput('Height', draft.height, value => { draft.height = value; }),
      authoringSelect('Unit', ['mm', 'in', 'pt'], draft.unit, value => {
        const points = {mm: 72 / 25.4, in: 72, pt: 1};
        draft.width = Number((Number(draft.width) * points[draft.unit] / points[value]).toFixed(3));
        draft.height = Number((Number(draft.height) * points[draft.unit] / points[value]).toFixed(3));
        draft.unit = value;
        renderPageSetup();
      }),
    );
  }
  const actions = document.createElement('div'); actions.className = 'edit-actions page-setup-actions';
  const cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'edit-secondary'; cancel.textContent = 'Cancel';
  cancel.disabled = visualMutationsLocked();
  cancel.addEventListener('click', () => { state.pageSetupDraft = null; state.pageSetupFeedback = null; renderPageSetup(); });
  const apply = document.createElement('button'); apply.type = 'submit'; apply.className = 'edit-commit'; apply.textContent = state.committing ? 'Applying…' : 'Apply page size';
  apply.disabled = visualMutationsLocked();
  actions.append(cancel, apply);
  form.append(actions);
  target.append(form);
  if (state.pageSetupFeedback) {
    const feedback = document.createElement('div');
    feedback.className = `edit-feedback is-${state.pageSetupFeedback.tone}`;
    feedback.textContent = state.pageSetupFeedback.text;
    target.append(feedback);
  }
}

function pageSetupInput(labelText, value, change) {
  const field = document.createElement('label');
  field.className = 'edit-field';
  const caption = document.createElement('span');
  caption.textContent = labelText;
  const input = document.createElement('input');
  input.type = 'number';
  input.min = '0.001';
  input.max = '14400';
  input.step = '0.01';
  input.value = value;
  input.disabled = visualMutationsLocked();
  input.addEventListener('change', () => change(input.value));
  field.append(caption, input);
  return field;
}

async function commitPageSetup(draft) {
  if (visualMutationsLocked()) return;
  let payload;
  try {
    payload = PaperStudioPageSetupModel.buildPayload(state.workspace, draft);
  } catch (error) {
    state.pageSetupFeedback = {tone: 'error', text: error.message};
    renderPageSetup();
    return;
  }
  state.committing = true;
  state.pageSetupFeedback = {tone: 'working', text: 'Saving page size…'};
  renderPageSetup();
  try {
    await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
    state.pageSetupDraft = null;
    state.pageSetupFeedback = null;
    await refresh();
  } catch (error) {
    state.pageSetupFeedback = {tone: 'error', text: error.status === 409 ? 'Page changed · refresh and try again' : error.message};
    if (error.status === 409) await refresh();
  } finally {
    state.committing = false;
    renderPageSetup();
    renderEditControls();
  }
}

function renderAuthoringControls() {
  const target = $('#authoring-controls');
  if (!target) return;
  target.replaceChildren();
  const metadata = state.authoring;
  if (!metadata) { const quiet=document.createElement('span'); quiet.className='quiet'; quiet.textContent='Authoring metadata unavailable'; target.append(quiet); return; }
  const available = [];
  if (metadata.templateTargets.length) available.push('template');
  if (metadata.documentTarget) available.push('import');
  if (metadata.documentTarget) available.push('schema');
  if (metadata.documentTarget) available.push('schema-object');
  if (metadata.schemaFields.length) available.push('schema-field');
  if (metadata.bindingTargets.length && metadata.schemas.some(schema => schema.fields.length)) available.push('binding');
  if (metadata.documentTarget && metadata.schemas.length) available.push('scenario-create');
  if (metadata.documentTarget && metadata.schemas.length) available.push('scenario-matrix');
  if (metadata.scenarioValues.length) available.push('scenario-value');
  if (!available.length) { const quiet=document.createElement('span'); quiet.className='quiet'; quiet.textContent='Add readable IDs and a schema to enable authoring'; target.append(quiet); return; }
  const draft = state.authoringDraft;
  draft.operation = available.includes(draft.operation) ? draft.operation : available[0];
  const form=document.createElement('form'); form.className='authoring-form';
  form.append(authoringSelect('Action', available, draft.operation, value => { draft.operation=value; renderAuthoringControls(); }));
  if (draft.operation === 'template') {
    const targets=metadata.templateTargets.map(item=>item.id); draft.target=targets.includes(draft.target)?draft.target:targets[0];
    const targetKind=metadata.templateTargets.find(item=>item.id===draft.target)?.kind;
	const repeatPaths=metadata.schemas.flatMap(schema=>schema.fields.filter(field=>field.kind==='list').map(field=>field.path));
	const choices=PaperStudioAuthoringModel.templateChoices(metadata,state.workspace,targetKind);
    draft.template=choices.includes(draft.template)?draft.template:choices[0]; draft.id=draft.id|| (draft.template==='page'?'@sheet':'@new-content');
    form.append(authoringSelect('Inside',targets,draft.target,value=>{draft.target=value;renderAuthoringControls();}),authoringSelect('Palette',choices,draft.template,value=>{draft.template=value;renderAuthoringControls();}),authoringInput('Readable ID',draft.id,value=>draft.id=value));
    if (draft.template === 'document-preset') {
      const presets=['blank','letter','prescription','medical-report','invoice','contract','certificate','table-report'];
      draft.preset=presets.includes(draft.preset)?draft.preset:'prescription';
      form.append(authoringSelect('Document design',presets,draft.preset,value=>draft.preset=value));
      const note=document.createElement('p');note.className='quiet';note.textContent='Creates one complete page master with header, footer, body hierarchy, metadata, and preset-specific content.';form.append(note);
    }
    if (draft.template === 'repeat') {
      draft.path=repeatPaths.includes(draft.path)?draft.path:repeatPaths[0];
      form.append(authoringSelect('Bounded list',repeatPaths,draft.path,value=>draft.path=value));
    }
    if (draft.template === 'component') {
      draft.component=metadata.components.includes(draft.component)?draft.component:metadata.components[0];
      form.append(authoringSelect('Component',metadata.components,draft.component,value=>{draft.component=value;renderAuthoringControls();}));
      const gallery=document.createElement('figure');gallery.className='component-gallery';
      const preview=document.createElement('div');preview.className='component-gallery-preview';
      const image=document.createElement('img');image.loading='lazy';image.alt=`${draft.component} exact current-theme preview`;
      const query=new URLSearchParams({preview_format:'2',revision:metadata.revision,source_revision:metadata.sourceRevision,component:draft.component});
      if(state.scenario)query.set('scenario',state.scenario);image.src=`/api/component-preview.svg?${query}`;preview.append(image);
      const caption=document.createElement('figcaption');caption.textContent=`${draft.component} · exact planner geometry · current theme`;
      gallery.append(preview,caption);form.append(gallery);
    }
  } else if (draft.operation === 'schema') {
    draft.target = metadata.documentTarget; draft.id = draft.id || '@new-schema';
    form.append(authoringInput('Schema ID', draft.id, value => draft.id=value));
  } else if (draft.operation === 'schema-object') {
    draft.target = metadata.documentTarget; draft.id = draft.id || '@NewObject';
    form.append(authoringInput('Custom object ID', draft.id, value => draft.id=value));
  } else if (draft.operation === 'import') {
    draft.target = metadata.documentTarget; draft.importPath = draft.importPath || 'styles/design.paper';
    form.append(authoringInput('Project-relative import', draft.importPath, value => draft.importPath=value));
  } else if (draft.operation === 'binding') {
    const targets=metadata.bindingTargets.map(item=>item.id); const paths=metadata.schemas.flatMap(schema=>schema.fields.map(field=>field.path));
    draft.target=targets.includes(draft.target)?draft.target:targets[0]; draft.path=paths.includes(draft.path)?draft.path:paths[0];
    const formats=['','string','bool','integer','decimal','currency'];
    draft.format=formats.includes(draft.format)?draft.format:'';
    form.append(authoringSelect('Node',targets,draft.target,value=>draft.target=value),authoringSelect('Schema path',paths,draft.path,value=>draft.path=value),authoringSelect('Required', ['','true','false'], draft.required === undefined ? '' : String(draft.required), value=>draft.required=value),authoringSelect('Format',formats,draft.format,value=>{draft.format=value;renderAuthoringControls();}));
    if (draft.format === 'integer' || draft.format === 'decimal' || draft.format === 'currency') {
      const locales=['en-US','pt-BR','ar']; draft.formatLocale=locales.includes(draft.formatLocale)?draft.formatLocale:'en-US';
      form.append(authoringSelect('Locale',locales,draft.formatLocale,value=>draft.formatLocale=value));
    }
    if (draft.format === 'currency') {
      const currencies=['USD','BRL','EUR','SAR']; draft.formatCurrency=currencies.includes(draft.formatCurrency)?draft.formatCurrency:'USD';
      form.append(authoringSelect('Currency',currencies,draft.formatCurrency,value=>draft.formatCurrency=value));
    }
    if (draft.format === 'decimal') {
      draft.minFraction=draft.minFraction ?? 0; draft.maxFraction=draft.maxFraction ?? 2;
      form.append(authoringNumberInput('Min fraction',draft.minFraction,value=>draft.minFraction=value),authoringNumberInput('Max fraction',draft.maxFraction,value=>draft.maxFraction=value));
    }
  } else if (draft.operation === 'schema-field') {
    const targets=metadata.schemaFields; const targetIDs=targets.map(item=>item.id); draft.target=targetIDs.includes(draft.target)?draft.target:targetIDs[0];
    const types=['string','number','bool','object','list',...metadata.objectTypes]; draft.kind=types.includes(draft.kind)?draft.kind:'string'; draft.id=draft.id||'@new-field';
    form.append(authoringSelect('Parent',targetIDs,draft.target,value=>draft.target=value),authoringSelect('Field type',types,draft.kind,value=>{draft.kind=value;renderAuthoringControls();}),authoringInput('Readable ID',draft.id,value=>draft.id=value));
    if (draft.kind === 'list') {
      const items=['string','number','bool','object',...metadata.objectTypes]; draft.itemType=items.includes(draft.itemType)?draft.itemType:'string'; draft.maxItems=draft.maxItems||16;
      form.append(authoringSelect('List item type',items,draft.itemType,value=>draft.itemType=value),authoringNumberInput('Max items',draft.maxItems,value=>draft.maxItems=value,{max:'1000000'}));
    }
  } else if (draft.operation === 'scenario-value') {
    const scenarios=[...new Set(metadata.scenarioValues.map(item=>item.scenario))]; draft.target=scenarios.includes(draft.target)?draft.target:scenarios[0];
    const values=metadata.scenarioValues.filter(item=>item.scenario===draft.target); const paths=values.map(item=>item.path); draft.path=paths.includes(draft.path)?draft.path:paths[0];
    const choice=values.find(item=>item.path===draft.path); draft.text=draft.text===undefined?choice?.value||'':draft.text;
    form.append(authoringSelect('Scenario',scenarios,draft.target,value=>{draft.target=value;draft.path='';draft.text=undefined;renderAuthoringControls();}),authoringSelect('Fixture path',paths,draft.path,value=>{draft.path=value;const next=values.find(item=>item.path===value);draft.text=next?.value||'';renderAuthoringControls();}));
    if (choice?.kind === 'bool') form.append(authoringSelect('Value',['true','false'],draft.text,value=>draft.text=value));
    else form.append(authoringInput(choice?.kind === 'number' ? 'Number value' : 'String value',draft.text,value=>draft.text=value));
  } else if (draft.operation === 'scenario-matrix') {
    const schemas=metadata.schemas.map(item=>item.name); draft.target=metadata.documentTarget; draft.schema=schemas.includes(draft.schema)?draft.schema:schemas[0]; draft.cases=draft.cases||'@empty:empty,@typical:typical,@stress:stress';
    form.append(authoringSelect('Schema',schemas,draft.schema,value=>draft.schema=value),authoringInput('Cases (id:preset, …)',draft.cases,value=>draft.cases=value));
  } else {
    const schemas=metadata.schemas.map(item=>item.name); draft.target=metadata.documentTarget; draft.schema=schemas.includes(draft.schema)?draft.schema:schemas[0]; draft.preset=metadata.presets.includes(draft.preset)?draft.preset:'typical'; draft.id=draft.id||'@stress-case';
    form.append(authoringSelect('Schema',schemas,draft.schema,value=>draft.schema=value),authoringSelect('Matrix case',metadata.presets,draft.preset,value=>draft.preset=value),authoringInput('Scenario ID',draft.id,value=>draft.id=value));
  }
  const submit=document.createElement('button'); submit.type='submit'; submit.className='edit-commit'; submit.disabled=visualMutationsLocked(); submit.textContent=state.committing?'Committing…':'Create exact patch'; form.append(submit);
  form.addEventListener('submit',async event=>{event.preventDefault();if(visualMutationsLocked())return;let payload;try{payload=PaperStudioAuthoringModel.buildPayload(state.workspace,metadata,draft);}catch(error){state.authoringFeedback={tone:'error',text:error.message};renderAuthoringControls();return;}state.committing=true;state.authoringFeedback={tone:'working',text:'Committing against exact revisions…'};renderAuthoringControls();try{const result=await api('/api/edit',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});state.authoringFeedback={tone:'success',text:`Committed · ${result.patch_count} minimal patch${result.patch_count===1?'':'es'}`};await refresh();}catch(error){state.authoringFeedback={tone:error.status===409?'stale':'error',text:error.status===409?'Stale authoring state · refreshed without applying':error.message};if(error.status===409)await refresh();}finally{state.committing=false;renderAuthoringControls();renderEditControls();}});
  target.append(form);
  if(state.authoringFeedback){const feedback=document.createElement('div');feedback.className=`edit-feedback is-${state.authoringFeedback.tone}`;feedback.textContent=state.authoringFeedback.text;target.append(feedback);}
}

function authoringSelect(labelText, values, selected, change) {
  const field=document.createElement('label');field.className='edit-field';const caption=document.createElement('span');caption.textContent=labelText;const select=document.createElement('select');select.disabled=visualMutationsLocked();
  for(const value of values){const option=document.createElement('option');option.value=value;option.textContent=value.replaceAll('-',' ');option.selected=value===selected;select.append(option);}select.addEventListener('change',()=>change(select.value));field.append(caption,select);return field;
}

function authoringInput(labelText, value, change) {
  const field=document.createElement('label');field.className='edit-field';const caption=document.createElement('span');caption.textContent=labelText;const input=document.createElement('input');input.type='text';input.value=value;input.maxLength=128;input.disabled=visualMutationsLocked();input.addEventListener('input',()=>change(input.value));field.append(caption,input);return field;
}

function authoringNumberInput(labelText, value, change, limits = {}) {
  const field=document.createElement('label');field.className='edit-field';const caption=document.createElement('span');caption.textContent=labelText;const input=document.createElement('input');input.type='number';input.min=limits.min ?? '0';input.max=limits.max ?? '18';input.step=limits.step ?? '1';input.value=value;input.disabled=visualMutationsLocked();input.addEventListener('input',()=>change(input.value));field.append(caption,input);return field;
}

function renderSource(source, issues = []) {
  const target = $('#source');
  const lines = source.split('\n');
  const annotations = new Map(PaperStudioIssueModel.sourceAnnotations(issues, lines.length).map(annotation => [annotation.line, annotation]));
  target.replaceChildren();
  lines.forEach((line, index) => {
    const lineNumber = index + 1;
    const row = document.createElement('span');
    row.className = 'source-line';
    row.dataset.line = String(lineNumber);
    const code = document.createElement('span');
    code.className = 'source-line-code';
    code.innerHTML = PaperStudioSyntaxModel.highlight(line) || '&#8203;';
    row.append(code);
    const annotation = annotations.get(lineNumber);
    if (annotation) {
      row.classList.add('has-issue', `is-${annotation.severity}`);
      const marker = document.createElement('button');
      marker.type = 'button';
      marker.className = 'source-diagnostic';
      marker.textContent = annotation.label;
      marker.title = annotation.title;
      marker.setAttribute('aria-label', `Line ${lineNumber}: ${annotation.label}`);
      marker.addEventListener('click', () => focusSourceLine(lineNumber));
      row.append(marker);
    }
    target.append(row);
  });
  $('#source-gutter').textContent = Array.from({length: lines.length}, (_, index) => index + 1).join('\n');
}

function nodeLabel(node) {
  if (node.id) return node.id;
  const value = node.value?.string_value ?? node.value?.raw;
  return value ? String(value) : node.kind;
}

function walkNodes(node, depth = 0, output = [], parent = null) {
  if (!node) return output;
  output.push({node, depth, parent});
  for (const member of node.members || []) if (member.node) walkNodes(member.node, depth + 1, output, node);
  return output;
}

function outlineNodeKey(node) {
  const start = node.header_span?.start || node.span?.start || {};
  return node.id || `${node.kind}:${start.line || 0}:${start.column || 0}`;
}

function renderOutline(root) {
  const outline = $('#outline');
  outline.replaceChildren();
  const nodes = walkNodes(root);
  $('#node-count').textContent = `${nodes.length} nodes`;
  let hiddenBelow = -1;
  for (const {node, depth, parent} of nodes) {
    if (hiddenBelow >= 0 && depth > hiddenBelow) continue;
    hiddenBelow = -1;
    const collapseKey = outlineNodeKey(node);
    const hasChildren = (node.members || []).some(member => member.node);
    const collapsed = hasChildren && state.outlineCollapsed.has(collapseKey);
    const row = document.createElement('button');
    const selected = Boolean(node.id && node.id === state.editSelection?.target);
    row.className = 'outline-row';
    row.setAttribute('role', 'treeitem');
    row.setAttribute('aria-level', String(depth + 1));
    row.setAttribute('aria-selected', String(selected));
    if (selected) row.classList.add('is-selected');
    if (hasChildren) row.setAttribute('aria-expanded', String(!collapsed));
    row.tabIndex = selected || (!state.editSelection && outline.children.length === 0) ? 0 : -1;
    if (node.id) row.dataset.key = node.id;
    row.dataset.collapseKey = collapseKey;
    row.dataset.depth = String(depth);
    if (parent) row.dataset.parentKey = outlineNodeKey(parent);
    row.style.setProperty('--outline-indent', `${Math.min(depth, 12) * 17}px`);
    row.innerHTML = `<span class="outline-disclosure" aria-hidden="true"></span><span class="outline-label"></span><span class="outline-kind"></span>`;
    row.querySelector('.outline-disclosure').textContent = hasChildren ? (collapsed ? '›' : '⌄') : '·';
    row.querySelector('.outline-label').textContent = nodeLabel(node);
    const childCount = (node.members || []).filter(member => member.node).length;
    row.querySelector('.outline-kind').textContent = childCount ? `${node.kind} · ${childCount} ${childCount === 1 ? 'item' : 'items'}` : node.kind;
    row.title = `${node.kind} · ${nodeLabel(node)}`;
    row.addEventListener('click', event => {
      if (hasChildren && event.target.closest('.outline-disclosure')) {
        if (collapsed) state.outlineCollapsed.delete(collapseKey); else state.outlineCollapsed.add(collapseKey);
        renderOutline(root);
        document.querySelector(`.outline-row[data-collapse-key="${CSS.escape(collapseKey)}"]`)?.focus();
        return;
      }
      selectSourceNode(node, row);
    });
    outline.append(row);
    if (collapsed) hiddenBelow = depth;
  }
}

$('#outline').addEventListener('keydown', (event) => {
  const rows = [...event.currentTarget.querySelectorAll('.outline-row')];
  const current = rows.indexOf(document.activeElement);
  if (current < 0) return;
  let next = current;
  if (event.key === 'ArrowDown') next = Math.min(rows.length - 1, current + 1);
  else if (event.key === 'ArrowUp') next = Math.max(0, current - 1);
  else if (event.key === 'Home') next = 0;
  else if (event.key === 'End') next = rows.length - 1;
  else if (event.key === 'ArrowRight') {
    const expanded = rows[current].getAttribute('aria-expanded');
    if (expanded === 'false') {
      event.preventDefault();
      state.outlineCollapsed.delete(rows[current].dataset.collapseKey);
      const key = rows[current].dataset.collapseKey;
      renderOutline(state.workspace?.ast?.root);
      document.querySelector(`.outline-row[data-collapse-key="${CSS.escape(key)}"]`)?.focus();
      return;
    }
    if (expanded === 'true' && Number(rows[current + 1]?.dataset.depth) > Number(rows[current].dataset.depth)) next = current + 1;
    else return;
  }
  else if (event.key === 'ArrowLeft') {
    if (rows[current].getAttribute('aria-expanded') === 'true') {
      event.preventDefault();
      state.outlineCollapsed.add(rows[current].dataset.collapseKey);
      const key = rows[current].dataset.collapseKey;
      renderOutline(state.workspace?.ast?.root);
      document.querySelector(`.outline-row[data-collapse-key="${CSS.escape(key)}"]`)?.focus();
      return;
    }
    const parentKey = rows[current].dataset.parentKey;
    if (!parentKey) return;
    next = rows.findIndex(row => row.dataset.collapseKey === parentKey);
    if (next < 0) return;
  }
  else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); rows[current].click(); return; }
  else return;
  event.preventDefault();
  rows.forEach((row, index) => { row.tabIndex = index === next ? 0 : -1; });
  rows[next]?.focus();
});

function renderScenarios(root) {
  const scenarios = walkNodes(root).filter(({node}) => node.kind === 'scenario').map(({node}) => nodeLabel(node));
  const target = $('#scenarios');
  target.replaceChildren();
  if (!scenarios.length) {
    target.innerHTML = '<span class="quiet">No authored scenarios</span>';
    return;
  }
  for (const choice of [{label: 'Default', value: ''}, ...scenarios.map((scenario) => ({label: scenario, value: scenario}))]) {
    const wrapper = document.createElement('div');
    wrapper.className = 'scenario-choice';
    const item = document.createElement('button');
    item.className = 'scenario';
    item.textContent = choice.label;
    item.classList.toggle('is-active', normalizeScenario(choice.value) === normalizeScenario(state.scenario));
    item.setAttribute('aria-pressed', String(normalizeScenario(choice.value) === normalizeScenario(state.scenario)));
    item.addEventListener('click', async () => {
      if (normalizeScenario(choice.value) === normalizeScenario(state.scenario)) return;
      state.scenario = choice.value;
      await refresh();
    });
    wrapper.append(item);
    if (choice.value) {
      const actions = document.createElement('span');
      actions.className = 'scenario-actions';
      const rename = document.createElement('button');
      rename.type = 'button'; rename.className = 'scenario-action'; rename.textContent = 'Rename';
      rename.disabled = visualMutationsLocked();
      rename.addEventListener('click', async () => {
        const id = window.prompt('New scenario ID', `@${normalizeScenario(choice.value)}-copy`);
        if (id === null) return;
        await commitScenarioLifecycle({action: 'rename', target: choice.value, id});
      });
      const remove = document.createElement('button');
      remove.type = 'button'; remove.className = 'scenario-action'; remove.textContent = 'Delete';
      remove.disabled = visualMutationsLocked();
      remove.addEventListener('click', async () => {
        if (!window.confirm(`Delete scenario ${choice.label}?`)) return;
        await commitScenarioLifecycle({action: 'delete', target: choice.value});
      });
      actions.append(rename, remove);
      wrapper.append(actions);
    }
    target.append(wrapper);
  }
}

async function commitScenarioLifecycle(draft) {
  if (visualMutationsLocked()) return;
  let payload;
  try {
    payload = PaperStudioAuthoringModel.buildScenarioLifecyclePayload(state.workspace, state.authoring, draft);
  } catch (error) {
    state.authoringFeedback = {tone: 'error', text: error.message};
    renderScenarios(state.workspace?.ast?.root);
    return;
  }
  state.committing = true;
  try {
    const result = await api('/api/edit', {method:'POST', headers:{'content-type':'application/json'}, body:JSON.stringify(payload)});
    state.scenario = result.scenario || '';
    state.authoringFeedback = {tone:'success', text:`Scenario ${draft.action} committed`};
    await refresh();
  } catch (error) {
    state.authoringFeedback = {tone:error.status===409?'stale':'error', text:error.status===409?'Stale scenario state · refreshed without applying':error.message};
    if (error.status===409) await refresh();
  } finally {
    state.committing = false;
    renderScenarios(state.workspace?.ast?.root);
    renderAuthoringControls();
  }
}

function normalizeScenario(value) {
  return String(value || '').trim().replace(/^@/, '');
}

function renderIssues(issues) {
  $('#issue-count').textContent = issues.length;
  const target = $('#issues');
  target.replaceChildren();
  if (!issues.length) {
    target.innerHTML = '<div class="inspector-empty">No current plan diagnostics.</div>';
    return;
  }
  for (const issue of issues) {
    const item = document.createElement('div');
    item.className = 'issue';
    if (issue.start_line) item.dataset.line = String(issue.start_line);
    const focus = document.createElement('button');
    focus.type = 'button';
    focus.className = 'issue-focus';
    focus.innerHTML = '<span class="issue-code"></span><span class="issue-message"></span><span class="issue-location"></span>';
    focus.querySelector('.issue-code').textContent = issue.code || issue.stage;
    focus.querySelector('.issue-message').textContent = issue.message;
    focus.querySelector('.issue-location').textContent = issue.start_line ? `Line ${issue.start_line}:${issue.start_column || 1}` : issue.stage;
    focus.addEventListener('click', () => focusSourceLine(issue.start_line || 1));
    const copy = document.createElement('button');
    copy.type = 'button';
    copy.className = 'issue-copy';
    copy.textContent = 'Copy';
    copy.setAttribute('aria-label', `Copy ${issue.code || issue.stage || 'issue'} error`);
    copy.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(PaperStudioIssueModel.format(issue));
        copy.textContent = 'Copied';
        copy.dataset.status = 'copied';
        setTimeout(() => { copy.textContent = 'Copy'; delete copy.dataset.status; }, 1600);
      } catch (_) {
        copy.textContent = 'Copy failed';
        copy.dataset.status = 'failed';
      }
    });
    item.append(focus, copy);
    if (issue.code === 'PAPER_COMPILE_FONT') {
      const selection = PaperStudioEditModel.findTextSelectionAtLine(state.workspace?.ast?.root, issue.start_line);
      if (selection) item.append(fontReplacementOffer(selection));
    }
    target.append(item);
  }
}

function fontReplacementOffer(selection) {
  const controls = document.createElement('div');
  controls.className = 'font-replacement';
  const select = document.createElement('select');
  select.setAttribute('aria-label', `Replacement font for ${selection.target}`);
  for (const font of PaperStudioEditModel.coreFonts) select.append(new Option(font, font));
  const apply = document.createElement('button');
  apply.type = 'button';
  apply.className = 'font-replacement-apply';
  apply.textContent = 'Replace font';
  apply.disabled = visualMutationsLocked();
  const status = document.createElement('span');
  status.className = 'font-replacement-status';
  apply.addEventListener('click', async () => {
    if (visualMutationsLocked()) return;
    let payload;
    try {
      payload = PaperStudioEditModel.buildPayload(state.workspace, selection, 'text', 'font', select.value);
    } catch (error) {
      status.textContent = error.message;
      return;
    }
    state.committing = true;
    apply.disabled = true;
    status.textContent = 'Applying exact patch…';
    try {
      await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
      await refresh();
    } catch (error) {
      status.textContent = error.status === 409 ? 'Source changed; refresh and choose again.' : error.message;
    } finally {
      state.committing = false;
      apply.disabled = visualMutationsLocked();
      renderEditControls();
      renderAuthoringControls();
      renderResources();
    }
  });
  controls.append(select, apply, status);
  return controls;
}

function renderBaseline(baseline) {
  const target = $('#baseline-state');
  target.textContent = PaperStudioRailModel.baselineLabel(baseline);
  target.title = baseline.revision
    ? `Baseline ${baseline.revision.slice(0, 12)} · ${baseline.scenario || 'default'} · ${baseline.status.replaceAll('_', ' ')}`
    : target.textContent;
  target.dataset.status = baseline.status || 'none';
}

function renderThumbnails(count, summaries) {
  const target = $('#thumbnails');
  target.replaceChildren();
  const byPage = PaperStudioRailModel.pageSummaryMap(summaries);
  for (let page = 1; page <= count; page++) {
    const summary = byPage.get(page) || PaperStudioRailModel.fallbackPageSummary(page);
    const item = document.createElement('div');
    item.className = `thumbnail${page === state.page ? ' is-active' : ''}${summary.changed ? ' is-changed' : ''}`;
    const button = document.createElement('button');
    button.className = 'thumbnail-page';
    button.dataset.page = page;
    if (page === state.page) button.setAttribute('aria-current', 'page');
    button.setAttribute('aria-label', `Page ${page}, ${summary.selector} selector, ${(summary.regions || []).join(', ') || 'no retained regions'}`);
    button.innerHTML = `<span class="thumbnail-sheet"><canvas role="img" aria-label="Page ${page} WASM thumbnail"></canvas></span><span class="thumbnail-label">${page}</span>`;
    button.addEventListener('click', () => showPage(page));
    item.append(button);

    const stateLine = document.createElement('div');
    stateLine.className = 'thumbnail-state';
    const selector = document.createElement('span');
    selector.className = 'master-state';
    selector.textContent = summary.selector;
    selector.title = `${summary.selector} page-master selector`;
    stateLine.append(selector);
    for (const region of summary.regions || []) {
      const label = document.createElement('span');
      label.className = `region-state${(summary.repeated_regions || []).includes(region) ? ' is-repeated' : ''}`;
      label.textContent = region.slice(0, 1).toUpperCase();
      label.title = `${region} region${(summary.repeated_regions || []).includes(region) ? ' · repeated master content' : ''}`;
      stateLine.append(label);
    }
    item.append(stateLine);

    const badges = document.createElement('div');
    badges.className = 'thumbnail-badges';
    if (summary.changed) {
      const changed = document.createElement('button');
      changed.className = 'rail-badge is-change';
      changed.textContent = summary.change_kind === 'added' ? '+' : 'Δ';
      changed.title = `Page ${page} ${summary.change_kind} from exact baseline`;
      changed.setAttribute('aria-label', changed.title);
      changed.addEventListener('click', async () => {
        await showPage(page);
        renderInspector({plan_revision: state.revision, page, baseline: state.workspace.baseline, change_kind: summary.change_kind, content_hash: summary.content_hash}, 'Changed page');
      });
      badges.append(changed);
    }
    for (const issue of summary.issues || []) {
      const badge = document.createElement('button');
      badge.className = `rail-badge is-issue is-${issue.severity || 'warning'}`;
      badge.textContent = '!';
      badge.title = `${issue.code}: ${issue.message}`;
      badge.setAttribute('aria-label', `Page ${page} issue ${badge.title}`);
      badge.addEventListener('click', () => selectRailIssue(summary, issue));
      badges.append(badge);
    }
    item.append(badges);
    target.append(item);
  }
}

async function selectRailIssue(summary, issue) {
  const revision = state.revision;
  await showPage(summary.page);
  if (revision !== state.revision) return;
  if (issue.start_line) focusSourceLine(issue.start_line);
  let causal = null;
  const selector = issue.key ? {key: issue.key} : issue.fragment ? {fragment: issue.fragment} : null;
  if (selector) {
    try {
      causal = await api('/api/explain', {
        method: 'POST', headers: {'content-type': 'application/json'},
        body: JSON.stringify({revision, scenario: state.scenario, selector}),
      });
      if (revision !== state.revision || causal.plan_hash !== revision) return;
      state.selectionFragments = causal.targets?.[0]?.fragments || [];
    } catch (error) {
      if (error.status === 409) return refresh();
    }
  } else if (issue.has_bounds && issue.bounds?.length === 4) {
    const [x, y, width, height] = issue.bounds;
    state.selectionFragments = [{page: summary.page, border_box: {x, y, width, height}}];
  }
  renderSelectionRects({reveal: true});
  renderInspector({plan_revision: revision, page: summary.page, issue, causal}, 'Plan issue');
}

async function loadSVG(page, kind) {
  const key = `${state.revision}:${kind}:${page}`;
  if (state.pageMeta.has(key)) return state.pageMeta.get(key);
  const suffix = kind === 'geometry' ? '.geometry.svg' : '.svg';
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  const response = await api(`/api/page/${page}${suffix}?revision=${encodeURIComponent(state.revision)}${scenario}`);
  const text = await response.text();
  const match = text.match(/viewBox="([^\"]+)"/);
  const viewBox = match ? match[1].trim().split(/\s+/).map(Number) : [0, 0, 1, 1];
  const blob = new Blob([text], {type: 'image/svg+xml'});
  const url = URL.createObjectURL(blob);
  state.objectURLs.add(url);
  const result = {url, viewBox, text};
  state.pageMeta.set(key, result);
  return result;
}

async function loadSelectableText(page) {
  const key = `${state.revision}:selectable-text:${page}`;
  if (state.pageMeta.has(key)) return state.pageMeta.get(key);
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  const response = await api(`/api/page/${page}.svg?revision=${encodeURIComponent(state.revision)}${scenario}`);
  const result = {text: await response.text()};
  state.pageMeta.set(key, result);
  return result;
}

async function loadWASMPage(page, dpi = renderDPIForZoom()) {
  const key = `${state.revision}:wasm:${page}:${dpi}`;
  if (state.pageMeta.has(key)) {
    state.wasmCacheKeys = state.wasmCacheKeys.filter((candidate) => candidate !== key);
    state.wasmCacheKeys.push(key);
    return state.pageMeta.get(key);
  }
  const revision = state.revision;
  const scenario = state.scenario ? `&scenario=${encodeURIComponent(state.scenario)}` : '';
  const response = await api(`/api/page/${page}.render?revision=${encodeURIComponent(revision)}&dpi=${dpi}${scenario}`);
  const result = await PaperStudioWASMRenderer.renderResponse(response, {revision, page, dpi});
  if (revision !== state.revision) {
    result.bitmap.close();
    const error = new Error('WASM page belongs to a stale plan revision');
    error.status = 409;
    throw error;
  }
  state.pageMeta.set(key, result);
  state.wasmCacheKeys.push(key);
  while (state.wasmCacheKeys.length > 6) {
    const expired = state.wasmCacheKeys.shift();
    const cached = state.pageMeta.get(expired);
    state.pageMeta.delete(expired);
    cached?.bitmap?.close?.();
  }
  return result;
}

function paintWASMCanvas(canvas, rendered) {
  canvas.width = rendered.pixelWidth;
  canvas.height = rendered.pixelHeight;
  const context = canvas.getContext('2d', {alpha: false});
  context.imageSmoothingEnabled = false;
  context.clearRect(0, 0, canvas.width, canvas.height);
  context.drawImage(rendered.bitmap, 0, 0);
}

function paintWASMThumbnail(page, rendered) {
  const canvas = document.querySelector(`.thumbnail-page[data-page="${CSS.escape(String(page))}"] canvas`);
  if (!canvas) return;
  const width = Math.max(1, Math.min(96, rendered.pixelWidth));
  const height = Math.max(1, Math.round(width * rendered.pixelHeight / rendered.pixelWidth));
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d', {alpha: false});
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = 'high';
  context.clearRect(0, 0, width, height);
  context.drawImage(rendered.bitmap, 0, 0, width, height);
}

function queueWASMThumbnails(activePage) {
  const serial = ++state.thumbnailRender;
  const revision = state.revision;
  const pages = Array.from(document.querySelectorAll('.thumbnail-page'), (button) => Number(button.dataset.page))
    .filter((page) => page !== activePage);
  const idle = () => new Promise((resolve) => {
    if (typeof requestIdleCallback === 'function') requestIdleCallback(resolve, {timeout: 250});
    else setTimeout(resolve, 0);
  });
  (async () => {
    for (const page of pages) {
      await idle();
      if (serial !== state.thumbnailRender || revision !== state.revision) return;
      const rendered = await loadWASMPage(page, 36);
      if (serial !== state.thumbnailRender || revision !== state.revision) return;
      paintWASMThumbnail(page, rendered);
    }
  })().catch((error) => { if (error.status === 409) refresh({quiet: true}); });
}

function queueWASMPageDetail(page) {
  const serial = ++state.detailRender;
  const revision = state.revision;
  const dpi = renderDPIForZoom();
  if ((state.activePageMeta?.manifest?.profile?.dpi || 0) >= dpi) return;
  const idle = (callback) => {
    if (typeof requestIdleCallback === 'function') requestIdleCallback(callback, {timeout: 400});
    else setTimeout(callback, 0);
  };
  idle(() => {
    loadWASMPage(page, dpi).then((rendered) => {
      if (serial !== state.detailRender || revision !== state.revision || page !== state.page) return;
      paintWASMPage(rendered);
      renderInspectionOverlays();
      renderSelectionRects();
    }).catch((error) => { if (error.status === 409) refresh({quiet: true}); });
  });
}

function paintWASMPage(rendered) {
  paintWASMCanvas(pageImage, rendered);
  state.activePageMeta = rendered;
  applyPageStageSize(rendered);
}

async function loadInspection(page) {
  const key = `${state.revision}:${page}`;
  if (state.inspections.has(key)) return state.inspections.get(key);
  const revision = state.revision;
  const inspection = await api('/api/inspect', {
    method: 'POST', headers: {'content-type': 'application/json'},
    body: JSON.stringify({revision, scenario: state.scenario, page}),
  });
  if (revision !== state.revision || inspection.plan_hash !== revision) {
    const error = new Error('Inspection belongs to a stale plan revision');
    error.status = 409;
    throw error;
  }
  state.inspections.set(key, inspection);
  return inspection;
}

async function showPage(page) {
  if (!state.workspace?.pages || page < 1 || page > state.workspace.pages) return;
  const revision = state.revision;
  const selectedTarget = state.editSelection?.target || '';
  if (page !== state.page) {
    closeInlineTextEditor();
    selectionLayer.replaceChildren();
    textSelectionLayer.replaceChildren();
    canvasScroll.scrollTop = 0;
    canvasScroll.scrollLeft = 0;
  }
  setPreviewStale(true);
  try {
    const [display, geometry, selectableText] = await Promise.all([
      loadWASMPage(page, previewDPIForZoom()), loadSVG(page, 'geometry'), loadSelectableText(page), loadInspection(page),
    ]);
    if (revision !== state.revision) return;
    state.page = page;
    paintWASMPage(display);
    paintWASMThumbnail(page, display);
    queueWASMPageDetail(page);
    queueWASMThumbnails(page);
    geometryImage.src = geometry.url;
    renderSelectableText(selectableText.text);
    $('#page-label').textContent = `Page ${page} of ${state.workspace.pages}`;
    document.querySelectorAll('.thumbnail-page').forEach((button) => {
      const active = Number(button.dataset.page) === page;
      button.closest('.thumbnail')?.classList.toggle('is-active', active);
      if (active) button.setAttribute('aria-current', 'page'); else button.removeAttribute('aria-current');
    });
    renderSelectionRects();
    renderInspectionOverlays();
    if (selectedTarget === (state.editSelection?.target || '')) renderPageInspectionEvidence();
    closeOverlapPicker();
    renderStatus();
  } catch (error) {
    if (error.superseded) return;
    if (error.status === 409) await refresh(); else showFailure(error);
  } finally {
    setPreviewStale(state.loading);
  }
}

function renderSelectableText(svgText) {
  textSelectionLayer.replaceChildren();
  const parsed = new DOMParser().parseFromString(svgText, 'image/svg+xml');
  if (parsed.querySelector('parsererror')) return;
  const source = parsed.documentElement;
  source.querySelectorAll('script, foreignObject, image, a, use').forEach((node) => node.remove());
  source.querySelectorAll('rect, path').forEach((node) => { if (!node.closest('clipPath')) node.remove(); });
  const svg = document.importNode(source, true);
  svg.setAttribute('aria-hidden', 'true');
  svg.setAttribute('focusable', 'false');
  textSelectionLayer.append(svg);
  window.setTimeout(() => materializeSelectableGlyphs(svg), 0);
}

function materializeSelectableGlyphs(svg) {
  if (!svg.isConnected) return;
  const layerBounds = textSelectionLayer.getBoundingClientRect();
  if (!(layerBounds.width > 0 && layerBounds.height > 0)) {
    window.setTimeout(() => materializeSelectableGlyphs(svg), 16);
    return;
  }
  const plane = document.createElement('div');
  plane.className = 'selectable-glyph-plane';
  const lines = [];
  let line = null;
  for (const text of svg.querySelectorAll('text')) {
    const rect = text.getBoundingClientRect();
    const usable = rect.width > 0 && rect.height > 0;
    const family = text.getAttribute('font-family') || 'sans-serif';
    const nextTop = usable ? rect.top - layerBounds.top : line?.top ?? 0;
    const nextLeft = usable ? rect.left - layerBounds.left : line?.right ?? 0;
    const nextHeight = usable ? rect.height : line?.height ?? 1;
    const separated = line && usable && (Math.abs(nextTop - line.top) > 1 || family !== line.family || nextLeft < line.lastLeft || nextLeft - line.right > nextHeight * 4);
    if (!line || separated) {
      line = {text: '', left: nextLeft, top: nextTop, right: nextLeft, lastLeft: nextLeft, height: nextHeight, family};
      lines.push(line);
    }
    line.text += text.textContent;
    if (usable) {
      line.right = rect.right - layerBounds.left;
      line.lastLeft = nextLeft;
      line.height = Math.max(line.height, nextHeight);
    }
  }
  for (const item of lines) {
    const selectable = document.createElement('span');
    selectable.className = 'selectable-line';
    selectable.textContent = item.text;
    selectable.style.left = `${item.left / layerBounds.width * 100}%`;
    selectable.style.top = `${item.top / layerBounds.height * 100}%`;
    selectable.style.fontFamily = item.family;
    selectable.style.fontSize = `${Math.max(0.1, item.height / layerBounds.height * 100)}cqh`;
    plane.append(selectable);
    if (item.text.length > 1) {
      const naturalWidth = selectable.getBoundingClientRect().width;
      const spacing = ((item.right - item.left) - naturalWidth) / (item.text.length - 1);
      selectable.style.letterSpacing = `${spacing / layerBounds.width * 100}cqw`;
    }
  }
  svg.remove();
  textSelectionLayer.append(plane);
}

function inspectionTarget() {
  return state.inspections.get(`${state.revision}:${state.page}`)?.targets?.[0] || null;
}

function addInspectionRect(rect, className, label = '') {
  const meta = state.activePageMeta;
  if (!meta || !rect || !(rect.width >= 0 && rect.height >= 0)) return null;
  const [viewX, viewY, viewWidth, viewHeight] = meta.viewBox;
  if (!(viewWidth > 0 && viewHeight > 0)) return null;
  const mark = document.createElement('div');
  mark.className = `inspection-mark ${className}`;
  mark.style.left = `${((rect.x - viewX) / viewWidth) * 100}%`;
  mark.style.top = `${((rect.y - viewY) / viewHeight) * 100}%`;
  mark.style.width = `${(rect.width / viewWidth) * 100}%`;
  mark.style.height = `${(rect.height / viewHeight) * 100}%`;
  if (label) {
    const tag = document.createElement('span');
    tag.textContent = label;
    mark.append(tag);
  }
  inspectionLayer.append(mark);
  return mark;
}

function renderInspectionOverlays() {
  inspectionLayer.replaceChildren();
  const target = inspectionTarget();
  if (!target || !state.overlays.size) return;
  const fragments = [...(target.fragments || []), ...(target.continuation_fragments || [])]
    .filter((fragment, index, all) => fragment.page === state.page && all.findIndex((entry) => entry.id === fragment.id) === index);
  const fragmentByID = new Map(fragments.map((fragment) => [fragment.id, fragment]));
  const classifiedInstances = globalThis.PaperStudioInstanceModel?.classifyFragments(fragments) || [];
  const instanceByFragment = new Map(classifiedInstances.map((entry) => [entry.fragment.id, entry]));
  const semantics = target.semantics || [];
  const semanticByID = new Map(semantics.map((entry) => [entry.Node?.id ?? entry.node?.id, entry.Node ?? entry.node]));
  const semanticByIdentity = new Map(semantics.map((entry) => {
    const node = entry.Node ?? entry.node;
    return [`${node?.key || ''}\u0000${node?.instance || ''}`, node];
  }));
  const reading = (target.reading_order || []).map((entry) => entry.Occurrence ?? entry.occurrence).filter(Boolean);
  const readingByFragment = new Map(reading.map((entry) => [entry.fragment, entry]));

  const boxModels = globalThis.PaperStudioInspectionModel?.boxModelMarks(fragments, state.page) || [];
  for (const box of boxModels) {
    if (state.overlays.has('margin')) addInspectionRect(box.margin, 'is-margin');
    if (state.overlays.has('border')) addInspectionRect(box.border, 'is-border');
    if (state.overlays.has('padding')) addInspectionRect(box.padding, 'is-padding');
    if (state.overlays.has('content')) addInspectionRect(box.content, 'is-content');
  }

  for (const fragment of fragments) {
    if (state.overlays.has('instances')) {
      const instance = instanceByFragment.get(fragment.id);
      if (instance) addInspectionRect(fragment.border_box, instance.className, instance.label);
    }
    if (state.overlays.has('reading')) {
      const occurrence = readingByFragment.get(fragment.id);
      if (occurrence) addInspectionRect(fragment.border_box, 'is-reading', String(occurrence.reading_index + 1));
    }
    if (state.overlays.has('roles')) {
      const occurrence = readingByFragment.get(fragment.id);
      const source = fragment.source_identity || {};
      const semantic = semanticByID.get(occurrence?.semantic) || semanticByIdentity.get(`${source.key || ''}\u0000${source.instance || ''}`);
      if (semantic?.role) addInspectionRect(fragment.border_box, `is-role is-role-${semantic.role}`, semantic.role);
    }
  }
  if (state.overlays.has('regions')) {
    const regions = globalThis.PaperStudioInspectionModel?.pageRegionMarks(target.page_regions || [], state.page) || [];
    for (const region of regions) addInspectionRect(region.rect, `is-region is-region-${region.region}`, region.label);
  }
  if (state.overlays.has('baselines')) {
    const baselines = globalThis.PaperStudioInspectionModel?.baselineMarks(target.lines || [], state.page) || [];
    for (const baseline of baselines) addInspectionRect(baseline.rect, 'is-baseline', baseline.label);
  }
  if (state.overlays.has('guides')) {
    const guides = globalThis.PaperStudioInspectionModel?.layoutGuideMarks(target.grid_tracks || [], state.page) || [];
    for (const guide of guides) addInspectionRect(guide.rect, `is-layout-guide is-layout-guide-${guide.axis}`, guide.label);
  }
  if (state.overlays.has('cells')) {
    const cells = globalThis.PaperStudioInspectionModel?.tableCellMarks(fragments, state.page) || [];
    for (const cell of cells) addInspectionRect(cell.rect, `is-table-cell${cell.tableHeader ? ' is-table-header-cell' : ''}`, cell.label);
  }
  const issues = globalThis.PaperStudioInspectionModel?.issueMarks(target, state.page) || {};
  if (state.overlays.has('overflow')) for (const mark of issues.overflow || []) addInspectionRect(mark.rect, 'is-overflow', mark.label);
  if (state.overlays.has('clips')) for (const mark of issues.clips || []) addInspectionRect(mark.rect, 'is-clip', mark.label);
  if (state.overlays.has('collisions')) for (const mark of issues.collisions || []) addInspectionRect(mark.rect, 'is-collision', mark.label);
  if (state.overlays.has('breaks')) {
    for (const entry of target.breaks || []) {
      const decision = entry.decision || {};
      const fragment = fragmentByID.get(decision.triggering_fragment) || fragmentByID.get(decision.preceding_fragment);
      if (fragment) addInspectionRect(fragment.border_box, 'is-break', `${decision.from_page}→${decision.to_page} · ${String(decision.reason || 'break').replaceAll('_', ' ')}`);
    }
  }
}

function renderPageInspectionEvidence() {
  const target = inspectionTarget();
  if (!target) return;
  const pageRegions = (globalThis.PaperStudioInspectionModel?.pageRegionMarks(target.page_regions || [], state.page) || []).map((entry) => ({
    region: entry.region, master: entry.master, bounds: entry.rect,
  }));
  const regions = pageRegions.map((entry) => entry.region);
  const fragments = [...(target.fragments || []), ...(target.continuation_fragments || [])]
    .filter((fragment, index, all) => all.findIndex((entry) => entry.id === fragment.id) === index);
  const fragmentInstances = (globalThis.PaperStudioInstanceModel?.classifyFragments(fragments) || []).map((entry) => ({
    id: entry.fragment.id,
    key: entry.key,
    instance: entry.instance,
    region: entry.region,
    kind: entry.kind,
    repeated: entry.repeated,
  }));
  const tableCells = (globalThis.PaperStudioInspectionModel?.tableCellMarks(fragments, state.page) || []).map((entry) => ({
    semantic: entry.cell,
    fragment: entry.fragment,
    table_header: entry.tableHeader,
  }));
  const layoutGuides = (globalThis.PaperStudioInspectionModel?.layoutGuideMarks(target.grid_tracks || [], state.page) || []).map((entry) => ({
    group: entry.group,
    axis: entry.axis,
    index: entry.guideIndex,
    gap_after: entry.gapAfter,
    bounds: entry.rect,
  }));
  const repeated = fragmentInstances.filter(entry => entry.repeated).length;
  const tableSummary = tableCells.length || layoutGuides.length ? `${tableCells.length} cells · ${layoutGuides.length} layout guides` : 'None';
  const provenance = globalThis.PaperStudioProvenanceModel?.forFragments(target.provenance, fragments) || {bindings: [], styleTokens: [], computedStyles: []};
  renderInspectorRows([
    ['Page', `${state.page} of ${state.workspace?.pages || state.page}`],
    ['Regions', regions.join(', ') || 'Body'],
    ['Repeated', repeated ? `${repeated} instances` : 'None'],
    ['Tables', tableSummary],
    ['Bindings', provenance.bindings.length ? `${provenance.bindings.length} paths` : 'None'],
    ['Style tokens', provenance.styleTokens.length ? `${provenance.styleTokens.length} properties` : 'None'],
    ['Computed style', provenance.computedStyles.length ? `${provenance.computedStyles.length} nodes` : 'None'],
    ['Reading', `${(target.reading_order || []).length} entries`],
    ['Overlays', state.overlays.size ? [...state.overlays].join(', ') : 'None'],
  ], 'Page');
  renderProvenance({provenance: target.provenance, fragments});
  renderBreakLedger(target);
}

function renderBreakLedger(target) {
  const breaks = target?.breaks || [];
  if (!breaks.length) return;
  const section = document.createElement('section');
  section.className = 'break-ledger';
  const heading = document.createElement('div');
  heading.className = 'provenance-heading';
  heading.textContent = `Break ledger · ${breaks.length}`;
  section.append(heading);
  for (const entry of breaks) {
    const decision = entry.decision || {};
    const row = document.createElement('div');
    row.className = 'break-ledger-item';
    const reason = String(decision.reason || 'break').replaceAll('_', ' ');
    row.textContent = `${decision.from_page || '?'} → ${decision.to_page || '?'} · ${reason} · trigger ${decision.triggering_fragment || decision.preceding_fragment || 'none'}`;
    section.append(row);
  }
  $('#inspector-content').append(section);
}

function renderSelectionRects({reveal = false} = {}) {
  const meta = state.activePageMeta;
  selectionLayer.replaceChildren();
  const selectedPages = new Set(state.selectionFragments.map((fragment) => fragment.page));
  document.querySelectorAll('.thumbnail-page').forEach((button) => button.closest('.thumbnail')?.classList.toggle('has-selection', selectedPages.has(Number(button.dataset.page))));
  if (!meta) return;
  const [viewX, viewY, viewWidth, viewHeight] = meta.viewBox;
  if (!(viewWidth > 0 && viewHeight > 0)) return;
  let first;
  for (const fragment of state.selectionFragments.filter((entry) => entry.page === state.page)) {
    const rect = fragment.border_box || fragment.content_box;
    if (!rect || rect.width < 0 || rect.height < 0) continue;
    const box = document.createElement('div');
    box.className = 'selection-box';
    box.style.left = `${((rect.x - viewX) / viewWidth) * 100}%`;
    box.style.top = `${((rect.y - viewY) / viewHeight) * 100}%`;
    box.style.width = `${(rect.width / viewWidth) * 100}%`;
    box.style.height = `${(rect.height / viewHeight) * 100}%`;
    selectionLayer.append(box);
    first ||= box;
  }
  if (reveal && first) first.scrollIntoView({block: 'center', inline: 'center'});
}

async function hitPage(event) {
  if (event.shiftKey || previewRevisionLocked()) return;
  const revision = state.revision;
  const meta = state.activePageMeta;
  if (!meta) return;
  const bounds = pageImage.getBoundingClientRect();
  const [x, y, width, height] = meta.viewBox;
  const xFixed = Math.round(x + ((event.clientX - bounds.left) / bounds.width) * width);
  const yFixed = Math.round(y + ((event.clientY - bounds.top) / bounds.height) * height);
  const pulse = $('#selection-pulse');
  pulse.style.left = `${event.clientX - bounds.left}px`;
  pulse.style.top = `${event.clientY - bounds.top}px`;
  pulse.classList.remove('is-visible');
  requestAnimationFrame(() => pulse.classList.add('is-visible'));
  try {
    const result = await api('/api/hit', {
      method: 'POST', headers: {'content-type': 'application/json'},
      body: JSON.stringify({revision: state.revision, scenario: state.scenario, page: state.page, x_fixed: xFixed, y_fixed: yFixed}),
    });
    if (revision !== state.revision || previewRevisionLocked()) return;
    const fragments = result.Fragments || [];
    const fragment = fragments[0];
    if (fragments.length > 1) openOverlapPicker(result, event.clientX - bounds.left, event.clientY - bounds.top);
    else closeOverlapPicker();
    await selectHitFragment(result, fragment);
  } catch (error) {
    if (error.status === 409) await refresh(); else showFailure(error);
  }
}

function inlineEditBounds() {
  const meta = state.activePageMeta;
  if (!meta) return null;
  const rects = state.selectionFragments
    .filter((fragment) => fragment.page === state.page)
    .map((fragment) => fragment.content_box || fragment.border_box)
    .filter((rect) => rect && rect.width >= 0 && rect.height >= 0);
  if (!rects.length) return null;
  const left = Math.min(...rects.map((rect) => rect.x));
  const top = Math.min(...rects.map((rect) => rect.y));
  const right = Math.max(...rects.map((rect) => rect.x + rect.width));
  const bottom = Math.max(...rects.map((rect) => rect.y + rect.height));
  const [viewX, viewY, viewWidth, viewHeight] = meta.viewBox;
  if (!(viewWidth > 0 && viewHeight > 0)) return null;
  return {
    left: ((left - viewX) / viewWidth) * 100,
    top: ((top - viewY) / viewHeight) * 100,
    width: Math.max(12, ((right - left) / viewWidth) * 100),
    height: Math.max(3, ((bottom - top) / viewHeight) * 100),
  };
}

function beginInlineTextEdit() {
  if (visualMutationsLocked() || !state.editSelection) return false;
  const content = PaperStudioEditModel.contentState(state.editSelection);
  const bounds = inlineEditBounds();
  if (!content.available || !content.authored || content.runs.length > 1 || !bounds) {
    state.editFeedback = {
      tone: 'error',
      text: content.bound
        ? 'This content is data-bound; edit its JSON value instead of adding static text.'
        : 'This content is not one directly editable authored text value; use the inspector.',
    };
    renderEditControls();
    return false;
  }
  const property = PaperStudioEditModel.properties(state.editSelection, 'content')[0];
  if (property !== 'text') return false;
  state.inlineEdit = {target: state.editSelection.target, original: content.text};
  inlineTextEditor.style.left = `${Math.max(0, Math.min(96, bounds.left))}%`;
  inlineTextEditor.style.top = `${Math.max(0, Math.min(96, bounds.top))}%`;
  inlineTextEditor.style.width = `${Math.max(18, Math.min(100 - bounds.left, bounds.width))}%`;
  inlineTextInput.style.minHeight = `${Math.max(44, pageImage.clientHeight * bounds.height / 100)}px`;
  inlineTextInput.value = content.text;
  inlineTextEditor.hidden = false;
  inlineTextEditor.classList.remove('is-committing');
  inlineTextEditor.querySelector('.inline-text-actions span').textContent = 'Esc to cancel · ⌘/Ctrl Enter to apply';
  requestAnimationFrame(() => {
    inlineTextInput.focus({preventScroll: true});
    inlineTextInput.setSelectionRange(0, inlineTextInput.value.length);
  });
  return true;
}

function closeInlineTextEditor() {
  state.inlineEdit = null;
  inlineTextEditor.hidden = true;
  inlineTextEditor.classList.remove('is-committing');
  inlineTextInput.value = '';
}

async function commitInlineTextEdit() {
  if (!state.inlineEdit || visualMutationsLocked() || state.inlineEdit.target !== state.editSelection?.target) return;
  const value = inlineTextInput.value;
  if (value === state.inlineEdit.original) {
    closeInlineTextEditor();
    return;
  }
  let payload;
  try {
    payload = PaperStudioEditModel.buildPayload(state.workspace, state.editSelection, 'content', 'text', value);
  } catch (error) {
    inlineTextEditor.querySelector('.inline-text-actions span').textContent = error.message;
    return;
  }
  state.committing = true;
  inlineTextEditor.classList.add('is-committing');
  inlineTextEditor.querySelector('.inline-text-actions span').textContent = 'Applying exact source patch…';
  try {
    await api('/api/validate-edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
    const result = await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
    state.editFeedback = {tone: 'success', text: `Text committed · ${result.patch_count} minimal patch`};
    closeInlineTextEditor();
    await refresh();
  } catch (error) {
    inlineTextEditor.classList.remove('is-committing');
    inlineTextEditor.querySelector('.inline-text-actions span').textContent = error.status === 409 ? 'The document changed; refresh before applying.' : error.message;
    if (error.status === 409) await refresh();
  } finally {
    state.committing = false;
    renderHistoryActions();
  }
}

async function selectHitFragment(result, fragment) {
  const revision = state.revision;
  try {
    if (!fragment?.Key) {
      state.selectionFragments = fragment ? [{page: state.page, border_box: fragment.BorderBox, content_box: fragment.ContentBox}] : [];
      renderSelectionRects();
      renderInspector(result, 'Pixel trace');
      return;
    }
    selectEditableTarget(fragment.Key);
    markOutlineKey(fragment.Key);
    focusSourceLine(fragment.Source?.start?.line || 1);
    const explanation = await api('/api/explain', {
      method: 'POST', headers: {'content-type': 'application/json'},
      body: JSON.stringify({revision: state.revision, scenario: state.scenario, selector: {key: fragment.Key}}),
    });
    if (revision !== state.revision || previewRevisionLocked()) return;
    state.selectionFragments = explanation.targets?.[0]?.fragments || [];
    renderSelectionRects({reveal: true});
    renderInspector({causal: explanation, hit: result}, 'Pixel trace');
  } catch (error) {
    if (error.status === 409) await refresh(); else showFailure(error);
  }
}

function openOverlapPicker(result, left, top) {
  overlapPicker.replaceChildren();
  const heading = document.createElement('div');
  heading.className = 'overlap-heading';
  heading.textContent = `${result.FragmentMatchCount || result.Fragments.length} overlaps · topmost first`;
  overlapPicker.append(heading);
  result.Fragments.forEach((fragment, index) => {
    const choice = document.createElement('button');
    choice.setAttribute('role', 'option');
    choice.setAttribute('aria-selected', String(index === 0));
    choice.innerHTML = '<span class="overlap-order"></span><span class="overlap-name"></span><span class="overlap-region"></span>';
    choice.querySelector('.overlap-order').textContent = String(index + 1);
    choice.querySelector('.overlap-name').textContent = fragment.Key || fragment.Instance || `Fragment ${fragment.ID}`;
    choice.querySelector('.overlap-region').textContent = fragment.Region || 'page';
    choice.addEventListener('click', async (event) => {
      event.stopPropagation();
      overlapPicker.querySelectorAll('[role="option"]').forEach((item) => item.setAttribute('aria-selected', String(item === choice)));
      await selectHitFragment(result, fragment);
    });
    overlapPicker.append(choice);
  });
  overlapPicker.style.left = `${Math.min(Math.max(8, left + 12), Math.max(8, pageImage.clientWidth - 202))}px`;
  overlapPicker.style.top = `${Math.max(8, top + 12)}px`;
  overlapPicker.hidden = false;
  overlapPicker.querySelector('[role="option"]')?.focus({preventScroll: true});
}

function closeOverlapPicker() {
  overlapPicker.hidden = true;
  overlapPicker.replaceChildren();
}

function markOutlineKey(key) {
  document.querySelectorAll('.outline-row').forEach((item) => {
    const selected = item.dataset.key === key;
    item.classList.toggle('is-selected', selected);
    item.setAttribute('aria-selected', String(selected));
    item.tabIndex = selected ? 0 : -1;
  });
}

async function selectSourceNode(node, row) {
  app.classList.remove('mobile-outline-open');
  $('#toggle-outline-mobile')?.setAttribute('aria-expanded', 'false');
  selectEditableTarget(node.id || '');
  if (node.id) markOutlineKey(node.id);
  else document.querySelectorAll('.outline-row').forEach((item) => {
    const selected = item === row;
    item.classList.toggle('is-selected', selected);
    item.setAttribute('aria-selected', String(selected));
  });
  focusSourceLine(node.header_span?.start?.line || node.span?.start?.line || 1);
  const selector = node.id ? {key: node.id} : {};
  if (!Object.keys(selector).length) {
    renderInspector(node, 'Source node');
    return;
  }
  try {
    const explanation = await api('/api/explain', {
      method: 'POST', headers: {'content-type': 'application/json'},
      body: JSON.stringify({revision: state.revision, scenario: state.scenario, selector}),
    });
    const fragments = explanation.targets?.[0]?.fragments || [];
    state.selectionFragments = fragments;
    renderSelectionRects();
    if (!fragments.length) {
      renderInspector({key: node.id, kind: node.kind}, 'Source node');
      return;
    }
    const fragment = fragments[0];
    if (fragment?.page) {
      await showPage(fragment.page);
      renderSelectionRects({reveal: true});
    }
    renderInspector({source: {key: node.id, kind: node.kind}, causal: explanation}, 'Causal trace');
  } catch (error) {
    renderInspector(node, 'Source node');
  }
}

function renderInspector(value, kind) {
  const useful = /(key|kind|role|page|region|reason|message|source|instance|operation|property|value)$/i;
  const blocked = /(revision|hash|bounds|box|fixed|geometry|selection|content_hash)/i;
  const seen = new Set();
  const rows = flatten(value)
    .filter(([key, entry]) => !blocked.test(key) && useful.test(key) && entry !== '' && entry !== null && entry !== undefined)
    .map(([key, entry]) => [key.split('.').at(-1).replaceAll('_', ' '), entry])
    .filter(([key]) => !seen.has(key) && seen.add(key))
    .slice(0, 10);
  renderInspectorRows(rows, kind);
  const causal = value?.causal;
  const target = causal?.targets?.[0] || value?.targets?.[0] || value;
  const fragments = target?.fragments || [];
  renderProvenance({provenance: fragments.length ? target?.provenance || causal?.provenance || value?.provenance : null, fragments});
}

function renderProvenance(value) {
  const target = $('#inspector-content');
  const model = globalThis.PaperStudioProvenanceModel;
  if (!model) return;
  const selected = model.forFragments(value?.provenance, value?.fragments || []);
  if (!selected.bindings.length && !selected.styleTokens.length && !selected.computedStyles.length) return;
  const section = document.createElement('section');
  section.className = 'provenance-evidence';
  const heading = document.createElement('div');
  heading.className = 'provenance-heading';
  heading.textContent = 'Source provenance';
  section.append(heading);
  for (const binding of selected.bindings) {
    const item = document.createElement('div');
    item.className = 'provenance-item';
    item.textContent = `Data · ${model.bindingLabel(binding)}`;
    section.append(item);
  }
  for (const token of selected.styleTokens) {
    const item = document.createElement('div');
    item.className = 'provenance-item';
    item.textContent = `Token · ${model.tokenLabel(token)}`;
    section.append(item);
  }
  for (const style of selected.computedStyles) {
    const item = document.createElement('div');
    item.className = 'provenance-item';
    item.textContent = `Style · ${model.computedStyleLabel(style)}`;
    section.append(item);
  }
  target.append(section);
}

function renderInspectorRows(rows, kind) {
  $('#selection-kind').textContent = kind;
  const target = $('#inspector-content');
  target.replaceChildren();
  if (!rows.length) {
    target.innerHTML = '<div class="inspector-empty">Select the page or an outline item to inspect it.</div>';
    return;
  }
  for (const [key, entry] of rows) {
    const row = document.createElement('div');
    row.className = 'property';
    row.innerHTML = '<span class="property-key"></span><span class="property-value"></span>';
    row.querySelector('.property-key').textContent = key;
    row.querySelector('.property-value').textContent = String(entry);
    target.append(row);
  }
  $('#cursor-state').textContent = `Page ${state.page} · ${kind}`;
}

function selectEditableTarget(target) {
  if (target && matchMedia('(max-width: 760px)').matches && !$('#inspector').contains(document.activeElement)) {
    state.inspectorReturnFocus = document.activeElement;
  }
  state.editSelection = target ? PaperStudioEditModel.findSelection(state.workspace?.ast?.root, target) : null;
  if (target && state.authoring?.templateTargets?.some(item => item.id === target)) {
    state.authoringDraft.target = target;
	renderAuthoringControls();
  }
  state.editDraft = null;
  state.editFeedback = null;
  app.classList.toggle('has-edit-selection', Boolean(state.editSelection));
  const inspector = $('#inspector');
  inspector.setAttribute('role', state.editSelection && matchMedia('(max-width: 760px)').matches ? 'dialog' : 'complementary');
  if (state.editSelection && matchMedia('(max-width: 760px)').matches) inspector.setAttribute('aria-modal', 'true');
  else inspector.removeAttribute('aria-modal');
  renderEditControls();
  renderReviewControls();
  if (state.editSelection && matchMedia('(max-width: 760px)').matches) requestAnimationFrame(() => $('#inspector-close').focus());
}

function closeSelectionInspector() {
  closeInlineTextEditor();
  state.selectionFragments = [];
  renderSelectionRects();
  $('#selection-pulse').classList.remove('is-visible');
  closeOverlapPicker();
  renderInspector({}, 'Nothing selected');
  const returnFocus = state.inspectorReturnFocus?.isConnected ? state.inspectorReturnFocus : document.querySelector('.outline-row[aria-selected="true"]');
  selectEditableTarget('');
  state.inspectorReturnFocus = null;
  returnFocus?.focus?.({preventScroll: true});
}

function reconcileEditSelection() {
  if (state.editSelection?.target) {
    state.editSelection = PaperStudioEditModel.findSelection(state.workspace?.ast?.root, state.editSelection.target);
  }
  renderEditControls();
  renderReviewControls();
}

function renderEditControls() {
  const target = $('#edit-controls');
  target.replaceChildren();
  const operations = PaperStudioEditModel.operations(state.editSelection);
  const bindingAvailable = state.authoring?.bindingTargets?.some((item) => item.id === state.editSelection?.target) &&
    state.authoring.schemas.some((schema) => schema.fields.length);
  const lifecycleAvailable = state.authoring?.lifecycleTargets?.some((item) => item.id === state.editSelection?.target);
  if (bindingAvailable) operations.push('binding');
  if (!operations.length) {
	target.hidden = !lifecycleAvailable;
	if (lifecycleAvailable) renderNodeLifecycleControls(target);
    return;
  }
  target.hidden = false;
  const operation = operations.includes(state.editDraft?.operation) ? state.editDraft.operation : operations[0];
  if (operation === 'binding') {
    renderBindingEditControls(target, operations);
    return;
  }
  const availableProperties = PaperStudioEditModel.properties(state.editSelection, operation);
  const property = availableProperties.includes(state.editDraft?.property) ? state.editDraft.property : availableProperties[0];
  state.editDraft = {operation, property};

  const heading = document.createElement('div');
  heading.className = 'edit-heading';
  const identity = document.createElement('span');
  identity.textContent = state.editSelection.target;
  const authority = document.createElement('span');
  authority.className = 'edit-authority';
  authority.textContent = 'source + plan locked';
  heading.append(identity, authority);

  const form = document.createElement('form');
  form.className = 'edit-form';
  form.setAttribute('aria-label', `Edit ${state.editSelection.target}`);
  const operationField = studioSelect('Edit', operations, operation, (value) => ({
    content: 'content',
    document: 'document details',
    appearance: 'style & theme',
    condition: 'visibility',
    text: 'typography',
    'layout-item': 'size & position',
    'layout-container': 'layout',
    'canvas-container': 'canvas',
    canvas: 'canvas item',
    box: 'spacing & border',
    flow: 'move',
    region: 'header / footer',
  })[value] || value.replaceAll('-', ' '));
  operationField.select.addEventListener('change', () => {
    state.editDraft = {operation: operationField.select.value, property: PaperStudioEditModel.properties(state.editSelection, operationField.select.value)[0]};
    renderEditControls();
  });
  const propertyField = studioPropertySelect(availableProperties, property);
  propertyField.select.addEventListener('change', () => {
    state.editDraft.property = propertyField.select.value;
    renderEditControls();
  });
  const valueSpec = PaperStudioEditModel.valueSpec(operation, property, state.editSelection);
  if (operation === 'flow') {
    valueSpec.kind = 'choice';
    valueSpec.label = 'Destination';
    valueSpec.choices = PaperStudioEditModel.flowDestinations(state.editSelection).map((node) => node.id);
  }
  const valueField = studioValueField(valueSpec);
  const submit = document.createElement('button');
  submit.type = 'submit';
  submit.className = 'edit-commit';
  submit.textContent = state.committing ? 'Committing…' : 'Apply exact patch';
  submit.disabled = visualMutationsLocked();
  const reset = document.createElement('button');
  reset.type = 'button';
  reset.className = 'edit-secondary edit-reset';
  reset.textContent = 'Reset property';
  reset.disabled = visualMutationsLocked() || !valueSpec.authored || operation === 'flow' || operation === 'content';
  reset.title = valueSpec.authored ? 'Remove this authored override and use inheritance or the built-in default' : 'No authored override exists for this property';
  const actions = document.createElement('div');
  actions.className = 'edit-actions';
  actions.append(reset, submit);
  form.append(operationField.label, propertyField.label);
  if (propertyField.searchLabel) form.append(propertyField.searchLabel);
  form.append(valueField.label, actions);

  let validationTimer = 0;
  let validationSerial = 0;
  let validationController = null;
  const validate = () => {
    clearTimeout(validationTimer);
    validationController?.abort();
    const serial = ++validationSerial;
    let payload;
    try {
      payload = PaperStudioEditModel.buildPayload(state.workspace, state.editSelection, operation, property, valueField.read());
      valueField.setError('');
      submit.disabled = true;
    } catch (error) {
      valueField.setError(error.message);
      submit.disabled = true;
      return;
    }
    if (visualMutationsLocked()) return;
    validationTimer = setTimeout(async () => {
      validationController = new AbortController();
      try {
        await api('/api/validate-edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload), signal: validationController.signal});
        if (serial !== validationSerial || !valueField.label.isConnected) return;
        valueField.setError('');
        submit.disabled = visualMutationsLocked();
      } catch (error) {
        if (error.name === 'AbortError') return;
        if (serial !== validationSerial || !valueField.label.isConnected) return;
        valueField.setError(error.status === 409 ? 'The source changed; refresh before editing' : error.message);
        submit.disabled = true;
      }
    }, 250);
  };
  valueField.label.addEventListener('input', validate);
  valueField.label.addEventListener('change', validate);
  validate();
  reset.addEventListener('click', async () => {
    if (reset.disabled) return;
    let payload;
    try { payload = PaperStudioEditModel.buildResetPayload(state.workspace, state.editSelection, operation, property); }
    catch (error) { valueField.setError(error.message); return; }
    state.committing = true;
    state.editFeedback = {tone: 'working', text: `Resetting ${property}…`};
    renderEditControls();
    try {
      const result = await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
      state.editFeedback = {tone: 'success', text: `Reset · ${result.patch_count} exact patch`};
      await refresh();
    } catch (error) {
      state.editFeedback = {tone: error.status === 409 ? 'stale' : 'error', text: error.status === 409 ? 'Selection changed · refreshed safely' : error.message};
      if (error.status === 409) await refresh();
    } finally {
      state.committing = false;
      renderEditControls();
      renderHistoryActions();
    }
  });
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    if (visualMutationsLocked()) return;
    let payload;
    try {
      payload = PaperStudioEditModel.buildPayload(state.workspace, state.editSelection, operation, property, valueField.read());
    } catch (error) {
      state.editFeedback = {tone: 'error', text: error.message};
      renderEditControls();
      return;
    }
    state.committing = true;
    state.editFeedback = globalThis.PaperStudioReviewModel?.optimisticFeedback(`${operation} ${property}`) || {tone: 'working', text: 'Speculative preview · exact patch pending server confirmation'};
    app.classList.add('is-committing');
    renderEditControls();
    try {
      const result = await api('/api/edit', {
        method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload),
      });
      state.editFeedback = {tone: 'success', text: `Committed · ${result.patch_count} minimal patch`};
      await refresh();
    } catch (error) {
      if (error.status === 409) {
        state.editFeedback = {tone: 'stale', text: 'Stale selection · refreshed without applying'};
        await refresh();
      } else {
        state.editFeedback = {tone: 'error', text: error.message};
      }
    } finally {
      state.committing = false;
      app.classList.remove('is-committing');
      renderEditControls();
    }
  });
  target.append(heading, form);
  if (state.editFeedback) {
    const feedback = document.createElement('div');
    feedback.className = `edit-feedback is-${state.editFeedback.tone}`;
    feedback.textContent = state.editFeedback.text;
    target.append(feedback);
  }
  renderNodeLifecycleControls(target);
}

function renderNodeLifecycleControls(target) {
  const selected = state.editSelection?.target;
  if (!selected || !state.authoring?.lifecycleTargets?.some((item) => item.id === selected)) return;
  const details = document.createElement('details');
  details.className = 'node-lifecycle';
  const summary = document.createElement('summary');
  summary.textContent = 'Rename or delete node';
  const form = document.createElement('form');
  form.className = 'node-lifecycle-form';
  const idField = document.createElement('label');
  idField.className = 'edit-field';
  const caption = document.createElement('span');
  caption.textContent = 'New readable ID';
  const input = document.createElement('input');
  input.value = selected;
  input.spellcheck = false;
  input.autocomplete = 'off';
  const error = document.createElement('small');
  error.className = 'edit-inline-error';
  error.setAttribute('role', 'alert');
  idField.append(caption, input, error);
  const actions = document.createElement('div');
  actions.className = 'node-lifecycle-actions';
  const rename = document.createElement('button');
  rename.type = 'submit';
  rename.className = 'edit-secondary';
  rename.textContent = 'Rename node';
  const remove = document.createElement('button');
  remove.type = 'button';
  remove.className = 'edit-secondary node-delete';
  remove.textContent = 'Delete node';
  remove.title = 'Requires a second click; compilation must still succeed';
  actions.append(rename, remove);
  form.append(idField, actions);
  details.append(summary, form);
  target.append(details);

  const payloadFor = (action) => PaperStudioAuthoringModel.buildNodeLifecyclePayload(state.workspace, state.authoring, {action, target: selected, id: action === 'rename' ? input.value.trim() : ''});
	let validationTimer = 0;
	let validationSerial = 0;
	let validationController = null;
  const validateRename = () => {
	clearTimeout(validationTimer);
	validationController?.abort();
	const serial = ++validationSerial;
	let payload;
	try {
	  payload = payloadFor('rename');
	  error.textContent = '';
	  rename.disabled = true;
	} catch (validationError) {
	  error.textContent = validationError.message;
	  rename.disabled = true;
	  return;
	}
	if (visualMutationsLocked()) return;
	validationTimer = setTimeout(async () => {
	  validationController = new AbortController();
	  try {
		await api('/api/validate-edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload), signal: validationController.signal});
		if (serial !== validationSerial || !details.isConnected) return;
		error.textContent = '';
		rename.disabled = visualMutationsLocked();
	  } catch (validationError) {
		if (validationError.name === 'AbortError' || serial !== validationSerial || !details.isConnected) return;
		error.textContent = validationError.status === 409 ? 'The source changed; refresh before renaming' : validationError.message;
		rename.disabled = true;
	  }
	}, 250);
  };
  input.addEventListener('input', validateRename);
  validateRename();

  const commit = async (payload) => {
    const renamedTarget = payload.property === 'rename' ? payload.id : '';
    state.committing = true;
    state.editFeedback = {tone: 'working', text: `${payload.property === 'rename' ? 'Renaming' : 'Deleting'} ${selected} against exact revisions…`};
    renderEditControls();
    try {
      const result = await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
      state.editFeedback = {tone: 'success', text: `Committed · ${result.patch_count} exact patch${result.patch_count === 1 ? '' : 'es'}`};
      await refresh();
      selectEditableTarget(renamedTarget);
      if (renamedTarget) requestAnimationFrame(() => document.querySelector('.node-lifecycle summary')?.focus());
    } catch (commitError) {
      state.editFeedback = {tone: commitError.status === 409 ? 'stale' : 'error', text: commitError.status === 409 ? 'Selection changed · refreshed without applying' : commitError.message};
      if (commitError.status === 409) await refresh();
    } finally {
      state.committing = false;
      renderEditControls();
      renderHistoryActions();
    }
  };
  form.addEventListener('submit', (event) => {
    event.preventDefault();
	if (rename.disabled) return;
    try { void commit(payloadFor('rename')); }
    catch (validationError) { error.textContent = validationError.message; }
  });
	let disarmTimer = 0;
  remove.addEventListener('click', async () => {
    if (visualMutationsLocked()) return;
    if (remove.dataset.armed !== 'true') {
	  remove.disabled = true;
	  remove.textContent = 'Checking delete…';
	  try {
		const payload = payloadFor('delete');
		await api('/api/validate-edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
		if (!details.isConnected) return;
		remove.dataset.armed = 'true';
		remove.textContent = `Confirm delete ${selected}`;
		remove.title = 'Click again within five seconds to delete this node.';
		clearTimeout(disarmTimer);
		disarmTimer = setTimeout(() => {
		  if (!remove.isConnected) return;
		  delete remove.dataset.armed;
		  remove.textContent = 'Delete node';
		  remove.title = 'Requires a second click; compilation must still succeed';
		}, 5000);
	  } catch (validationError) {
		error.textContent = validationError.status === 409 ? 'The source changed; refresh before deleting' : validationError.message;
		remove.textContent = 'Delete node';
	  } finally {
		remove.disabled = false;
	  }
      return;
    }
	clearTimeout(disarmTimer);
    try { void commit(payloadFor('delete')); }
    catch (validationError) { error.textContent = validationError.message; }
  });
}

function renderBindingEditControls(target, operations) {
  const selection = state.editSelection;
  const metadata = state.authoring;
  const paths = metadata.schemas.flatMap((schema) => schema.fields.map((field) => field.path));
  const value = (name) => PaperStudioEditModel.authoredValue(selection, name);
  const bind = value('bind');
  const draft = state.editDraft?.operation === 'binding' ? state.editDraft : {operation: 'binding'};
  draft.target = selection.target;
  draft.path = paths.includes(draft.path) ? draft.path : paths.includes(bind.value) ? bind.value : paths[0];
  draft.required ??= value('bind-required').authored ? value('bind-required').value : '';
  draft.format ??= value('format').value;
  draft.formatLocale ??= value('format-locale').value;
  draft.formatCurrency ??= value('format-currency').value;
  draft.minFraction ??= value('format-min-fraction').value;
  draft.maxFraction ??= value('format-max-fraction').value;
  state.editDraft = draft;

  const heading = document.createElement('div');
  heading.className = 'edit-heading';
  const identity = document.createElement('span');
  identity.textContent = selection.target;
  const authority = document.createElement('span');
  authority.className = 'edit-authority';
  authority.textContent = 'source + plan locked';
  heading.append(identity, authority);

  const form = document.createElement('form');
  form.className = 'edit-form';
  form.setAttribute('aria-label', `Edit binding on ${selection.target}`);
  const operationField = studioSelect('Edit', operations, 'binding', (item) => item === 'binding' ? 'data binding' : item.replaceAll('-', ' '));
  operationField.select.addEventListener('change', () => {
    state.editDraft = {operation: operationField.select.value};
    renderEditControls();
  });
  const pathField = authoringSelect('Schema path', paths, draft.path, (next) => { draft.path = next; });
  const requiredField = authoringSelect('Required', ['', 'true', 'false'], String(draft.required ?? ''), (next) => { draft.required = next; });
  const formats = ['', 'string', 'bool', 'integer', 'decimal', 'currency'];
  draft.format = formats.includes(draft.format) ? draft.format : '';
  const formatField = authoringSelect('Format', formats, draft.format, (next) => { draft.format = next; renderEditControls(); });
  form.append(operationField.label, pathField, requiredField, formatField);
  if (['integer', 'decimal', 'currency'].includes(draft.format)) {
    const locales = ['en-US', 'pt-BR', 'ar'];
    draft.formatLocale = locales.includes(draft.formatLocale) ? draft.formatLocale : 'en-US';
    form.append(authoringSelect('Locale', locales, draft.formatLocale, (next) => { draft.formatLocale = next; }));
  } else {
    draft.formatLocale = '';
  }
  if (draft.format === 'currency') {
    const currencies = ['USD', 'BRL', 'EUR', 'SAR'];
    draft.formatCurrency = currencies.includes(draft.formatCurrency) ? draft.formatCurrency : 'USD';
    form.append(authoringSelect('Currency', currencies, draft.formatCurrency, (next) => { draft.formatCurrency = next; }));
  } else {
    draft.formatCurrency = '';
  }
  if (draft.format === 'decimal') {
    draft.minFraction = draft.minFraction === '' ? 0 : draft.minFraction;
    draft.maxFraction = draft.maxFraction === '' ? 2 : draft.maxFraction;
    form.append(authoringNumberInput('Min fraction', draft.minFraction, (next) => { draft.minFraction = next; }),
      authoringNumberInput('Max fraction', draft.maxFraction, (next) => { draft.maxFraction = next; }));
  } else {
    draft.minFraction = '';
    draft.maxFraction = '';
  }
  const actions = document.createElement('div');
  actions.className = 'edit-actions';
  const reset = document.createElement('button');
  reset.type = 'button';
  reset.className = 'edit-secondary edit-reset';
  reset.textContent = 'Remove binding';
  reset.disabled = visualMutationsLocked() || !bind.authored;
  reset.title = bind.authored ? 'Remove the binding and all of its formatting metadata' : 'This node has no authored binding';
  const submit = document.createElement('button');
  submit.type = 'submit';
  submit.className = 'edit-commit';
  submit.disabled = visualMutationsLocked();
  submit.textContent = state.committing ? 'Committing…' : 'Apply exact patch';
  const bindingError = document.createElement('small');
  bindingError.className = 'edit-inline-error edit-form-error';
  bindingError.setAttribute('role', 'alert');
  actions.append(reset, submit);
  form.append(bindingError, actions);

  let validationTimer = 0;
  let validationSerial = 0;
  let validationController = null;
  const validate = () => {
    clearTimeout(validationTimer);
    validationController?.abort();
    const serial = ++validationSerial;
    let payload;
    try {
      payload = PaperStudioAuthoringModel.buildPayload(state.workspace, metadata, draft);
      bindingError.textContent = '';
      submit.disabled = true;
    } catch (error) {
      bindingError.textContent = error.message;
      submit.disabled = true;
      return;
    }
    if (visualMutationsLocked()) return;
    validationTimer = setTimeout(async () => {
      validationController = new AbortController();
      try {
        await api('/api/validate-edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload), signal: validationController.signal});
        if (serial !== validationSerial || !form.isConnected) return;
        bindingError.textContent = '';
        submit.disabled = visualMutationsLocked();
      } catch (error) {
        if (error.name === 'AbortError') return;
        if (serial !== validationSerial || !form.isConnected) return;
        bindingError.textContent = error.status === 409 ? 'The source changed; refresh before editing' : error.message;
        submit.disabled = true;
      }
    }, 250);
  };
  form.addEventListener('input', validate);
  form.addEventListener('change', validate);

  const commit = async (payload, workingText) => {
    state.committing = true;
    state.editFeedback = {tone: 'working', text: workingText};
    renderEditControls();
    try {
      const result = await api('/api/edit', {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
      state.editFeedback = {tone: 'success', text: `Committed · ${result.patch_count} minimal patch${result.patch_count === 1 ? '' : 'es'}`};
      await refresh();
    } catch (error) {
      state.editFeedback = {tone: error.status === 409 ? 'stale' : 'error', text: error.status === 409 ? 'Selection changed · refreshed safely' : error.message};
      if (error.status === 409) await refresh();
    } finally {
      state.committing = false;
      renderEditControls();
    }
  };
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    if (visualMutationsLocked()) return;
    try { void commit(PaperStudioAuthoringModel.buildPayload(state.workspace, metadata, draft), 'Validating binding against the schema…'); }
    catch (error) { state.editFeedback = {tone: 'error', text: error.message}; renderEditControls(); }
  });
  reset.addEventListener('click', () => {
    if (reset.disabled) return;
    void commit({source_revision: state.workspace.source_revision, plan_revision: state.workspace.revision, scenario: state.workspace.scenario || '', operation: 'binding', target: selection.target, property: 'bind', reset: true}, 'Removing binding metadata…');
  });
  target.append(heading, form);
  validate();
  if (state.editFeedback) {
    const feedback = document.createElement('div');
    feedback.className = `edit-feedback is-${state.editFeedback.tone}`;
    feedback.textContent = state.editFeedback.text;
    target.append(feedback);
  }
  renderNodeLifecycleControls(target);
}

function studioSelect(labelText, options, selected, display = (value) => value.replaceAll('-', ' ')) {
  const label = document.createElement('label');
  label.className = 'edit-field';
  const caption = document.createElement('span');
  caption.textContent = labelText;
  const select = document.createElement('select');
  select.disabled = visualMutationsLocked();
  for (const value of options) {
    const option = document.createElement('option');
    option.value = value;
    option.textContent = display(value);
    option.selected = value === selected;
    select.append(option);
  }
  label.append(caption, select);
  return {label, select};
}

function studioPropertySelect(options, selected) {
  const field = studioSelect('Property', [], selected);
  const populate = (query = '') => {
    const normalized = query.trim().toLowerCase();
    field.select.replaceChildren();
    const groups = new Map();
    for (const value of options) {
      if (normalized && !value.replaceAll('-', ' ').includes(normalized) && value !== selected) continue;
      const group = value === selected && normalized && !value.includes(normalized) ? 'Current' : PaperStudioEditModel.propertyGroup(value);
      if (!groups.has(group)) groups.set(group, []);
      groups.get(group).push(value);
    }
    for (const [name, values] of groups) {
      const group = document.createElement('optgroup');
      group.label = name;
      for (const value of values) {
        const option = document.createElement('option');
        option.value = value;
        option.textContent = value.replaceAll('-', ' ');
        option.selected = value === selected;
        group.append(option);
      }
      field.select.append(group);
    }
  };
  populate();
  if (options.length > 8) {
    const searchLabel = document.createElement('label');
    searchLabel.className = 'edit-field edit-property-search';
    const caption = document.createElement('span');
    caption.textContent = 'Find property';
    const search = document.createElement('input');
    search.type = 'search';
    search.placeholder = 'spacing, border, size…';
    search.setAttribute('aria-label', 'Filter available properties');
    search.addEventListener('input', () => populate(search.value));
    searchLabel.append(caption, search);
    field.searchLabel = searchLabel;
  }
  return field;
}

let studioFieldSequence = 0;
function studioValueField(spec) {
  const label = document.createElement('label');
  label.className = 'edit-field edit-value';
  const caption = document.createElement('span');
  caption.textContent = spec.label;
  const sourceState = document.createElement('small');
  sourceState.className = `edit-source-state ${spec.authored ? 'is-authored' : ''}`;
  sourceState.textContent = spec.authored ? 'Authored' : 'Using default';
  caption.append(sourceState);
  let input;
  let read;
  let unavailable = false;
  if (spec.kind === 'rich-text') {
    const controls = document.createElement('div');
    controls.className = 'rich-text-runs';
    const runs = Array.isArray(spec.currentValue) ? spec.currentValue : [];
    const inputs = runs.map(run => {
      const row = document.createElement('div'); row.className = 'rich-text-run';
      const target = document.createElement('code'); target.textContent = run.target;
      const area = document.createElement('textarea'); area.rows = 3; area.value = run.text; area.dataset.target = run.target;
      area.disabled = visualMutationsLocked();
      row.append(target, area); controls.append(row);
      return area;
    });
    input = inputs[0];
    read = () => inputs.map(area => ({target: area.dataset.target, text: area.value}));
    label.append(caption, controls);
  } else if (spec.kind === 'multiline') {
    input = document.createElement('textarea');
    input.rows = 5;
    input.value = PaperStudioEditModel.defaultValue(spec);
    read = () => input.value;
  } else if (spec.kind === 'constraint') {
    const controls = document.createElement('span');
    controls.className = 'edit-constraint';
    const current = PaperStudioEditModel.defaultValue(spec);
    const match = current.match(/^(canvas|@[A-Za-z][A-Za-z0-9_-]*)\.(left|right|center-x|top|bottom|center-y)(?:\s*([+-])\s*(\d+(?:\.\d+)?)pt)?$/);
    const target = document.createElement('select');
    target.setAttribute('aria-label', 'Constraint target');
    for (const value of spec.targets || ['canvas']) {
      const option = document.createElement('option');
      option.value = value;
      option.textContent = value === 'canvas' ? 'Canvas' : value;
      option.selected = value === (match?.[1] || 'canvas');
      target.append(option);
    }
    const anchor = document.createElement('select');
    anchor.setAttribute('aria-label', 'Constraint anchor');
    for (const value of spec.anchors || []) {
      const option = document.createElement('option');
      option.value = value;
      option.textContent = value.replace('-', ' ');
      option.selected = value === (match?.[2] || spec.anchors?.[0]);
      anchor.append(option);
    }
    const offset = document.createElement('input');
    offset.type = 'number';
    offset.step = '0.25';
    offset.min = '-1000000';
    offset.max = '1000000';
    offset.value = match?.[4] ? `${match[3] === '-' ? '-' : ''}${match[4]}` : '0';
    offset.setAttribute('aria-label', 'Constraint offset in points');
    const unit = document.createElement('span');
    unit.className = 'edit-constraint-unit';
    unit.textContent = 'pt';
    controls.append(target, anchor, offset, unit);
    label.append(caption, controls);
    input = target;
    read = () => {
      const value = Number(offset.value || 0);
      const suffix = value === 0 ? '' : ` ${value < 0 ? '-' : '+'} ${Math.abs(value)}pt`;
      return `${target.value}.${anchor.value}${suffix}`;
    };
    for (const control of [target, anchor, offset]) control.disabled = visualMutationsLocked();
  } else if (spec.kind === 'length') {
    const measure = document.createElement('span');
    measure.className = 'edit-measure';
    input = document.createElement('input');
    input.type = 'number';
    input.min = spec.positive === false ? '0' : '0.000001';
    input.step = spec.units?.includes('fr') ? '1' : '0.25';
    const unit = document.createElement('select');
    const current = PaperStudioEditModel.defaultValue(spec);
    const match = current.match(/^(\d+(?:\.\d+)?)(pt|%|fr)$/);
    for (const value of spec.units || ['pt']) {
      const option = document.createElement('option');
      option.value = value;
      option.textContent = value === 'auto' ? 'Auto' : value;
      option.selected = value === (current === 'auto' ? 'auto' : match?.[2] || 'pt');
      unit.append(option);
    }
    input.value = match?.[1] || (current === 'auto' ? '' : current.replace(/[^0-9.]/g, ''));
    const syncUnit = () => {
      input.disabled = visualMutationsLocked() || unit.value === 'auto';
      input.step = unit.value === 'fr' ? '1' : '0.25';
      input.max = unit.value === '%' ? '100' : unit.value === 'fr' ? '4294967295' : '1000000';
    };
    unit.addEventListener('change', syncUnit);
    syncUnit();
    measure.append(input, unit);
    label.append(caption, measure);
    read = () => unit.value === 'auto' ? 'auto' : `${input.value}${unit.value}`;
  } else if (spec.kind === 'choice' || spec.kind === 'boolean' || spec.kind === 'reference' || spec.kind === 'name') {
    input = document.createElement('select');
    const current = PaperStudioEditModel.defaultValue(spec);
    const available = spec.choices || [];
    const choices = available.includes(current) || !current ? available : [current, ...available];
    if (!choices.length) {
      const option = document.createElement('option');
      option.value = '';
      option.textContent = spec.kind === 'name' ? 'No compatible theme tokens' : 'No compatible references';
      input.append(option);
      unavailable = true;
      input.title = spec.kind === 'name' ? 'Create a token of the required type in the selected theme first' : 'Create a compatible referenced item first';
    }
    for (const value of choices) {
      const option = document.createElement('option');
      option.value = option.textContent = value;
      option.selected = value === current;
      input.append(option);
    }
    read = () => input.value;
  } else {
    input = document.createElement('input');
    input.type = spec.kind === 'color' ? 'color' : ['text', 'length', 'constraint', 'reference', 'name'].includes(spec.kind) ? 'text' : 'number';
    input.value = PaperStudioEditModel.defaultValue(spec);
    if (spec.min !== undefined) input.min = String(spec.min);
    if (spec.max !== undefined) input.max = String(spec.max);
    if (spec.step !== undefined) input.step = String(spec.step);
    input.required = spec.required === true;
    if (spec.suggestions?.length) {
      const id = `edit-suggestions-${Math.random().toString(36).slice(2)}`;
      const list = document.createElement('datalist');
      list.id = id;
      for (const value of spec.suggestions) { const option = document.createElement('option'); option.value = value; list.append(option); }
      input.setAttribute('list', id);
      label.append(list);
    }
    read = () => input.value;
  }
  input.disabled = visualMutationsLocked() || unavailable;
  if (!label.contains(caption)) label.append(caption, input);
  const help = document.createElement('small');
  help.className = 'edit-help';
  help.textContent = spec.help;
  const error = document.createElement('small');
  error.className = 'edit-inline-error';
  error.setAttribute('role', 'alert');
  const fieldID = `edit-field-${++studioFieldSequence}`;
  help.id = `${fieldID}-help`;
  error.id = `${fieldID}-error`;
  for (const control of label.querySelectorAll('input, select, textarea')) control.setAttribute('aria-describedby', `${help.id} ${error.id}`);
  label.append(help, error);
  return {label, input, read, setError(message) {
    error.textContent = message;
    label.classList.toggle('has-error', Boolean(message));
    for (const control of label.querySelectorAll('input, select, textarea')) control.setAttribute('aria-invalid', String(Boolean(message)));
  }};
}

function flatten(value, prefix = '', result = []) {
  if (value === null || typeof value !== 'object') {
    result.push([prefix || 'value', value]);
    return result;
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => flatten(item, `${prefix}[${index}]`, result));
  } else {
    Object.entries(value).forEach(([key, entry]) => flatten(entry, prefix ? `${prefix}.${key}` : key, result));
  }
  return result;
}

function focusSourceLine(line) {
  const source = $('#source');
  const lineHeight = parseFloat(getComputedStyle(source).lineHeight) || 19.8;
  const row = source.querySelector(`.source-line[data-line="${Number(line)}"]`);
  source.querySelectorAll('.source-line.is-focused').forEach(item => item.classList.remove('is-focused'));
  document.querySelectorAll('.issue.is-selected').forEach(item => item.classList.remove('is-selected'));
  if (row) {
    row.classList.add('is-focused');
    source.scrollTop = Math.max(0, row.offsetTop - lineHeight * 3);
  } else {
    source.scrollTop = Math.max(0, (line - 4) * lineHeight);
  }
  const issue = document.querySelector(`.issue[data-line="${Number(line)}"]`);
  if (issue) {
    issue.classList.add('is-selected');
    issue.scrollIntoView({block: 'nearest'});
  }
}

function setMode(mode) {
  app.dataset.mode = mode;
  document.querySelectorAll('.mode').forEach((button) => {
    const active = button.dataset.mode === mode;
    button.classList.toggle('is-active', active);
    button.setAttribute('aria-pressed', String(active));
  });
  if (mode === 'review') app.classList.add('show-geometry');
  if (mode === 'accessibility') {
    document.querySelector('.overlay-disclosure').open = true;
    for (const overlay of ['reading', 'roles']) state.overlays.add(overlay);
    document.querySelectorAll('.inspection-toggle').forEach((button) => button.setAttribute('aria-pressed', String(state.overlays.has(button.dataset.overlay))));
    renderInspectionOverlays();
    renderPageInspectionEvidence();
  }
}

function renderDPIForZoom() {
  return PaperStudioViewportModel.renderDPI(state.zoom, window.devicePixelRatio);
}

function previewDPIForZoom() {
  return PaperStudioViewportModel.previewDPI(state.zoom, window.devicePixelRatio);
}

function renderedPageSize(rendered = state.activePageMeta) {
  if (!rendered?.manifest?.profile?.dpi) return null;
  return {
    width: rendered.pixelWidth * 72 / rendered.manifest.profile.dpi,
    height: rendered.pixelHeight * 72 / rendered.manifest.profile.dpi,
  };
}

function viewportPadding() {
  const style = getComputedStyle(canvasScroll);
  const number = (property) => Number.parseFloat(style[property]) || 0;
  return {
    inline: number('paddingLeft') + number('paddingRight'),
    block: number('paddingTop') + number('paddingBottom'),
  };
}

function fittedZoom(mode = state.zoomMode, rendered = state.activePageMeta) {
  const page = renderedPageSize(rendered);
  if (!page || canvasScroll.clientWidth <= 0 || canvasScroll.clientHeight <= 0) return state.zoom;
  const padding = viewportPadding();
  return PaperStudioViewportModel.fitZoom({
    pageWidth: page.width,
    pageHeight: page.height,
    viewportWidth: canvasScroll.clientWidth,
    viewportHeight: canvasScroll.clientHeight,
    paddingInline: padding.inline,
    paddingBlock: padding.block,
    mode: mode === 'fit-width' ? 'width' : 'page',
  });
}

function syncZoomControls() {
  $('#zoom-value').value = String(Math.round(state.zoom * 100));
  $('#zoom-mode').value = state.zoomMode;
}

function applyPageStageSize(rendered = state.activePageMeta) {
  const page = renderedPageSize(rendered);
  if (!page) return;
  if (state.zoomMode !== 'custom') state.zoom = fittedZoom(state.zoomMode, rendered);
  $('#page-stage').style.width = `${Math.max(1, page.width * state.zoom)}px`;
  syncZoomControls();
}

function scheduleZoomRender() {
  const serial = ++state.zoomRender;
  ++state.detailRender;
  const page = state.page;
  const revision = state.revision;
  const dpi = renderDPIForZoom();
  clearTimeout(state.zoomTimer);
  state.zoomTimer = null;
  if ((state.activePageMeta?.manifest?.profile?.dpi || 0) >= dpi) return;
  state.zoomTimer = setTimeout(() => {
    state.zoomTimer = null;
    loadWASMPage(page, dpi).then(rendered => {
      if (serial !== state.zoomRender || page !== state.page || revision !== state.revision) return;
      paintWASMPage(rendered);
      renderInspectionOverlays();
      renderSelectionRects();
    }).catch(error => { if (error.superseded) return; if (error.status === 409) refresh(); else showFailure(error); });
  }, 140);
}

function setZoom(next, mode = 'custom') {
  if (!Number.isFinite(next)) {
    syncZoomControls();
    return;
  }
  state.zoomMode = mode;
  state.zoom = PaperStudioViewportModel.zoom(next);
  applyPageStageSize();
  scheduleZoomRender();
}

function setZoomMode(mode) {
  if (mode === 'custom') {
    state.zoomMode = mode;
    syncZoomControls();
    return;
  }
  if (mode !== 'fit-page' && mode !== 'fit-width') return;
  setZoom(fittedZoom(mode), mode);
}

function resizeViewportProjection() {
  clearTimeout(state.resizeTimer);
  if (!state.activePageMeta) return;
  applyPageStageSize();
  renderInspectionOverlays();
  renderSelectionRects();
  state.resizeTimer = setTimeout(() => {
    state.resizeTimer = null;
    scheduleZoomRender();
  }, 160);
}

function renderStatus() {
  const workspace = state.workspace;
  $('#page-label').textContent = workspace?.pages ? `Page ${state.page} of ${workspace.pages}` : 'No page';
  $('#cursor-state').textContent = workspace?.pages ? `Page ${state.page} · No selection` : 'No planned page';
  renderVerificationState();
}

function renderVerificationState() {
  const badge = $('#verification-state');
  if (!badge) return;
  let label = 'Plan preview';
  let className = 'is-preview';
  if (!state.workspace?.pages) {
    label = 'Unavailable';
    className = 'is-stale';
  } else if (app.classList.contains('is-stale')) {
    label = 'Plan stale';
    className = 'is-stale';
  }
  badge.textContent = label;
  badge.className = `verification-state ${className}`;
  badge.title = label === 'Plan preview'
    ? 'The canvas shows the exact current retained plan.'
    : 'The canvas no longer matches the current plan revision.';
}

function showFailure(error) {
  $('#connection-state').textContent = `Workspace error · ${error.message}`;
  app.classList.add('has-no-plan');
}

function clearObjectURLs() {
  clearTimeout(state.zoomTimer);
  state.zoomTimer = null;
  for (const url of state.objectURLs) URL.revokeObjectURL(url);
  state.objectURLs.clear();
  for (const value of state.pageMeta.values()) value.bitmap?.close?.();
  state.wasmCacheKeys = [];
}

document.querySelectorAll('.mode').forEach((button) => button.addEventListener('click', () => setMode(button.dataset.mode)));
$('#toggle-outline-mobile')?.addEventListener('click', event => {
  const open = app.classList.toggle('mobile-outline-open');
  event.currentTarget.setAttribute('aria-expanded', String(open));
  if (open) $('#outline .outline-row[tabindex="0"]')?.focus();
});
document.querySelectorAll('.insert-tool[data-template]').forEach(button => button.addEventListener('click', () => prepareInsertTool(button)));
document.querySelector('.insert-tool[data-authoring-operation="binding"]')?.addEventListener('click', event => prepareDataTool(event.currentTarget));
document.querySelector('.insert-tool[data-open-authoring]')?.addEventListener('click', event => { openAuthoringTools(); pulseInsertTool(event.currentTarget); });
$('#toggle-overlay').addEventListener('click', (event) => {
  const enabled = app.classList.toggle('show-geometry');
  event.currentTarget.setAttribute('aria-pressed', String(enabled));
});
document.querySelectorAll('.inspection-toggle').forEach((button) => button.addEventListener('click', () => {
  const overlay = button.dataset.overlay;
  if (state.overlays.has(overlay)) state.overlays.delete(overlay); else state.overlays.add(overlay);
  button.setAttribute('aria-pressed', String(state.overlays.has(overlay)));
  renderInspectionOverlays();
  renderPageInspectionEvidence();
}));
overlapPicker.addEventListener('keydown', (event) => {
  const options = [...overlapPicker.querySelectorAll('[role="option"]')];
  const current = options.indexOf(document.activeElement);
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const delta = event.key === 'ArrowDown' ? 1 : -1;
    options[(current + delta + options.length) % options.length]?.focus();
  }
});
$('#refresh').addEventListener('click', () => refresh());
$('#history-undo').addEventListener('click', () => applyHistory('undo'));
$('#history-redo').addEventListener('click', () => applyHistory('redo'));
$('#inspector-close').addEventListener('click', closeSelectionInspector);
$('#inspector').addEventListener('keydown', (event) => {
  if (event.key !== 'Tab' || !state.editSelection || !matchMedia('(max-width: 760px)').matches) return;
  const focusable = [...event.currentTarget.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), summary, [tabindex]:not([tabindex="-1"])')]
    .filter((item) => item.getClientRects().length);
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
});
$('#resource-add')?.addEventListener('click', toggleResourceForm);
$('#zoom-in').addEventListener('click', () => setZoom(state.zoom + .1));
$('#zoom-out').addEventListener('click', () => setZoom(state.zoom - .1));
$('#zoom-mode').addEventListener('change', event => setZoomMode(event.currentTarget.value));
$('#zoom-value').addEventListener('change', event => setZoom(Number(event.currentTarget.value) / 100));
$('#zoom-value').addEventListener('keydown', event => {
  if (event.key !== 'Enter') return;
  event.preventDefault();
  setZoom(Number(event.currentTarget.value) / 100);
  event.currentTarget.blur();
});
pageImage.addEventListener('click', hitPage);
textSelectionLayer.addEventListener('click', (event) => {
  if (!window.getSelection()?.isCollapsed) return;
  void hitPage(event);
});
textSelectionLayer.addEventListener('dblclick', async (event) => {
  event.preventDefault();
  window.getSelection()?.removeAllRanges();
  await hitPage(event);
  beginInlineTextEdit();
});
inlineTextEditor.addEventListener('submit', (event) => {
  event.preventDefault();
  void commitInlineTextEdit();
});
$('#inline-text-cancel').addEventListener('click', closeInlineTextEditor);
inlineTextInput.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') {
    event.preventDefault();
    event.stopPropagation();
    closeInlineTextEditor();
    pageImage.focus?.({preventScroll: true});
  } else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault();
    void commitInlineTextEdit();
  }
});
inlineTextEditor.addEventListener('focusout', () => {
  window.setTimeout(() => {
    if (!inlineTextEditor.hidden && !inlineTextEditor.contains(document.activeElement)) void commitInlineTextEdit();
  }, 80);
});
window.addEventListener('keydown', (event) => {
  const typing = event.target.matches('input, select, textarea, button, pre, [contenteditable="true"]');
  if (event.key === 'Escape') {
    if (app.classList.contains('mobile-outline-open')) {
      app.classList.remove('mobile-outline-open');
      $('#toggle-outline-mobile')?.setAttribute('aria-expanded', 'false');
      $('#toggle-outline-mobile')?.focus();
      return;
    }
    closeSelectionInspector();
    return;
  }
  if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === 'z') {
    if (typing) return;
    event.preventDefault();
    applyHistory(event.shiftKey ? 'redo' : 'undo');
    return;
  }
  if (typing) return;
  if ((event.key === 'Enter' || event.key === 'F2') && state.editSelection) {
    event.preventDefault();
    beginInlineTextEdit();
    return;
  }
  if (event.key === 'ArrowLeft') showPage(state.page - 1);
  if (event.key === 'ArrowRight') showPage(state.page + 1);
  if (event.key === '+' || event.key === '=') setZoom(state.zoom + .1);
  if (event.key === '-') setZoom(state.zoom - .1);
  if (event.key === '[') app.classList.toggle('left-collapsed');
  if (event.key === ']') app.classList.toggle('right-collapsed');
});
window.addEventListener('beforeunload', () => { state.changeStream?.close?.(); clearObjectURLs(); });
if (typeof ResizeObserver === 'function') new ResizeObserver(resizeViewportProjection).observe(canvasScroll);
else window.addEventListener('resize', resizeViewportProjection);

syncZoomControls();
if (!studioSessionToken) showFailure(new Error('Paper Studio session token is missing from the launch URL'));
else refresh()
  .then(connectChangeStream)
  .catch(showFailure);
})();
