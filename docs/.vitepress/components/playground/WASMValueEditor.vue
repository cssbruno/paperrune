<script setup>
import {nextTick, onBeforeUnmount, ref, watch} from 'vue';

const props = defineProps({
  engine: {type: Object, default: null},
  hash: {type: String, default: ''},
  page: {type: Number, default: 1},
  descriptor: {type: Object, default: null},
  target: {type: String, default: ''},
  disabled: {type: Boolean, default: false},
});

const emit = defineEmits(['applied', 'error']);
const host = ref(null);
const applying = ref(false);
let controller;
let mountSequence = 0;

watch(
  [
    () => props.engine,
    () => props.hash,
    () => props.page,
    () => props.descriptor,
    () => props.target,
  ],
  () => void remount(),
  {immediate: true},
);

onBeforeUnmount(destroy);

async function remount() {
  const sequence = ++mountSequence;
  destroy();
  applying.value = false;
  await nextTick();
  const descriptor = props.descriptor;
  if (sequence !== mountSequence || !host.value || !props.engine || !props.hash || !descriptor?.editable) return;
  const mountedHash = props.hash;
  const request = {
    host: host.value,
    hash: mountedHash,
    page: props.page,
    text: String(descriptor.value ?? ''),
    mode: descriptor.mode,
    binding: descriptor.binding || '',
    vectorOnly: true,
    autoFocus: false,
    applyOnBlur: false,
    onApplied: (result) => {
      applying.value = false;
      emit('applied', {hash: mountedHash, result});
    },
    onError: () => {
      applying.value = false;
    },
    onCancel: () => {},
  };
  if (descriptor.mode === 'data') request.jsonPointer = descriptor.pointer;
  else if (Number.isInteger(descriptor.sourceOffset)) request.sourceOffset = descriptor.sourceOffset;
  else request.target = descriptor.target || props.target;
  const mounted = props.engine.mountEditor(request);
  if (mounted instanceof Error) {
    emit('error', mounted);
    return;
  }
  controller = mounted;
}

function apply() {
  if (!controller?.apply || props.disabled || applying.value) return;
  applying.value = true;
  controller.apply();
}

function destroy() {
  controller?.destroy?.();
  controller = undefined;
}
</script>

<template>
  <section class="value-editor">
    <div ref="host" class="wasm-value-editor-host"></div>
    <footer>
      <span>Ctrl/Cmd + Enter also applies</span>
      <button type="button" :title="applying ? 'Applying value' : 'Apply value'" :aria-label="applying ? 'Applying value' : 'Apply value'" :disabled="disabled || applying" @click="apply">{{ applying ? '…' : '✓' }}</button>
    </footer>
  </section>
</template>

<style scoped>
.value-editor { overflow: hidden; border: 1px solid #c9c6bd; border-radius: 4px; background: #fff; }
.wasm-value-editor-host { min-height: 92px; }
.wasm-value-editor-host :deep(.wasm-direct-editor) {
  min-height: 92px;
  padding: 10px 11px;
  outline: 0;
  background: #fff;
  color: #20242a;
  caret-color: #2e5bd6;
  font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  word-break: break-word;
}
.wasm-value-editor-host :deep(.wasm-direct-editor::selection) {
  background: #b9c9fa;
  color: #20242a;
  -webkit-text-fill-color: #20242a;
}
.wasm-value-editor-host :deep(.wasm-direct-editor-error) {
  display: none;
  padding: 7px 10px;
  border-top: 1px solid #e1aaa6;
  background: #fff1ef;
  color: #9b2d27;
  font: 10px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.wasm-value-editor-host.has-error :deep(.wasm-direct-editor-error) { display: block; }
.value-editor footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 7px; border-top: 1px solid #ddd9d0; background: #f4f2ec; }
.value-editor footer span { color: #74767a; font: 8px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; }
.value-editor button { display: grid; place-items: center; width: 30px; height: 28px; border: 0; border-radius: 3px; padding: 0; background: #20242a; color: #fff; font: 700 13px/1 ui-sans-serif, system-ui, sans-serif; }
.value-editor button:disabled { cursor: default; opacity: .45; }
</style>
