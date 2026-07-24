<script setup>
import {nextTick, onBeforeUnmount, ref, watch} from 'vue';

const props = defineProps({
  engine: {type: Object, default: null},
  hash: {type: String, default: ''},
  page: {type: Number, default: 1},
  kind: {type: String, required: true},
});

const emit = defineEmits(['editing', 'snapshot', 'error']);
const host = ref(null);
let controller;
let mountSequence = 0;

watch(
  [() => props.engine, () => props.hash, () => props.page, () => props.kind],
  () => void remount(),
  {immediate: true},
);

onBeforeUnmount(destroy);

async function remount() {
  const sequence = ++mountSequence;
  destroy();
  await nextTick();
  if (sequence !== mountSequence || !host.value || !props.engine || !props.hash) return;
  const mounted = props.engine.mountFileEditor({
    host: host.value,
    hash: props.hash,
    page: props.page,
    kind: props.kind,
    onEditing: () => emit('editing'),
    onSnapshot: (snapshot) => emit('snapshot', snapshot),
  });
  if (mounted instanceof Error) {
    emit('error', mounted);
    return;
  }
  controller = mounted;
}

function destroy() {
  controller?.destroy?.();
  controller = undefined;
}
</script>

<template>
  <div ref="host" class="wasm-file-editor-host"></div>
</template>

<style scoped>
.wasm-file-editor-host { min-height: 0; height: 100%; }
.wasm-file-editor-host :deep(.wasm-file-editor) {
  display: block;
  width: 100%;
  height: 100%;
  min-height: 220px;
  resize: none;
  border: 0;
  outline: 0;
  padding: 15px;
  background: #20242a;
  color: #ece9e1;
  caret-color: #8da8ff;
  font: 12px/1.62 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  tab-size: 2;
}
</style>
