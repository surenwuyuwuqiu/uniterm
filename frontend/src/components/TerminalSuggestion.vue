<template>
  <div
    v-show="visible"
    class="terminal-suggestion-popup"
    :style="popupStyle"
  >
    <div
      v-for="(item, index) in items"
      :key="`${item.type}-${index}`"
      class="suggestion-item"
      :class="{ selected: index === selectedIndex, 'ai-result': item.type === 'ai-result', 'ai-preview': item.type === 'ai-preview' }"
      @click="onSelect(index)"
      @mouseenter="onHover(index)"
    >
      <span v-if="item.icon" class="suggestion-icon">{{ item.icon }}</span>
      <span class="suggestion-label">{{ item.label }}</span>
      <span v-if="item.description" class="suggestion-desc">{{ item.description }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SuggestionItem } from '../composables/useSuggestions'

const props = defineProps<{
  visible: boolean
  items: SuggestionItem[]
  selectedIndex: number
  cursorX: number
  cursorY: number
}>()

const emit = defineEmits<{
  select: [index: number]
  hover: [index: number]
}>()

const popupStyle = computed(() => ({
  left: `${props.cursorX}px`,
  top: `${props.cursorY}px`,
}))

function onSelect(index: number) {
  emit('select', index)
}

function onHover(index: number) {
  emit('hover', index)
}
</script>

<style scoped>
.terminal-suggestion-popup {
  position: absolute;
  z-index: 100;
  min-width: 200px;
  max-width: 400px;
  max-height: 200px;
  overflow-y: auto;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  padding: 4px 0;
  backdrop-filter: blur(8px);
}

.suggestion-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 13px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
  transition: background 0.1s ease;
}

.suggestion-item:hover,
.suggestion-item.selected {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.suggestion-item.ai-result {
  border-left: 3px solid #34d399;
}

.suggestion-item.ai-preview {
  border-top: 1px solid var(--border-subtle);
  margin-top: 2px;
  padding-top: 8px;
  color: var(--accent);
}

.suggestion-icon {
  font-size: 12px;
  opacity: 0.7;
}

.suggestion-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.suggestion-desc {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-ui);
}

.terminal-suggestion-popup::-webkit-scrollbar {
  width: 6px;
}

.terminal-suggestion-popup::-webkit-scrollbar-track {
  background: transparent;
}

.terminal-suggestion-popup::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 3px;
}
</style>
