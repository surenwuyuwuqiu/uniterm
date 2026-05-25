<template>
  <div class="db-query-history">
    <div class="history-header">
      <span class="section-title">{{ t('db.queryHistory') }}</span>
      <button class="clear-btn" @click="onClear">{{ t('db.clearHistory') }}</button>
    </div>
    <div class="history-list">
      <div
        v-for="entry in history"
        :key="entry.id"
        class="history-item"
        :class="{ error: entry.error }"
        @click="$emit('replay', entry.sql)"
      >
        <div class="history-sql">{{ entry.sql }}</div>
        <div class="history-meta">
          <span>{{ formatTime(entry.executedAt) }}</span>
          <span v-if="entry.error" class="history-error">{{ entry.error }}</span>
          <span v-else-if="entry.rowCount !== undefined">{{ entry.rowCount }} {{ t('db.rows') }}</span>
          <span>{{ entry.durationMs }}ms</span>
        </div>
      </div>
      <div v-if="history.length === 0" class="empty">{{ t('db.noHistory') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from '../i18n'
import { GetQueryHistory, ClearQueryHistory } from '../../wailsjs/go/main/App'
import type { HistoryEntry } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
  refreshTrigger: number
}>()

defineEmits<{
  replay: [sql: string]
}>()

const history = ref<HistoryEntry[]>([])

onMounted(loadHistory)
watch(() => props.refreshTrigger, loadHistory)

async function loadHistory() {
  try {
    history.value = await GetQueryHistory(props.sessionId)
  } catch (e) {
    console.error('Failed to load history:', e)
  }
}

async function onClear() {
  try {
    await ClearQueryHistory(props.sessionId)
    history.value = []
  } catch (e) {
    console.error('Failed to clear history:', e)
  }
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  return d.toLocaleString()
}
</script>

<style scoped>
.db-query-history {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
}
.section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #888);
}
.clear-btn {
  border: none;
  background: none;
  color: var(--color-danger, #f56c6c);
  cursor: pointer;
  font-size: 12px;
}
.history-list {
  flex: 1;
  overflow: auto;
}
.history-item {
  padding: 6px 12px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-color, #eee);
}
.history-item:hover {
  background: var(--bg-hover, #f5f5f5);
}
.history-item.error {
  border-left: 3px solid var(--color-danger, #f56c6c);
}
.history-sql {
  font-family: monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.history-meta {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--text-secondary, #999);
  margin-top: 2px;
}
.history-error {
  color: var(--color-danger, #f56c6c);
}
.empty {
  padding: 12px;
  color: var(--text-secondary, #888);
  font-size: 12px;
  text-align: center;
}
</style>
