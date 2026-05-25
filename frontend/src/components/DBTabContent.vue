<template>
  <div class="db-tab-content">
    <div class="db-main">
      <div class="db-left" :style="{ width: leftWidth + 'px' }">
        <DBTreePanel
          :session-id="sessionId"
          @select-table="onSelectTable"
          @select-database="onSelectDatabase"
        />
      </div>
      <div class="db-resizer" @mousedown="onResizeStart" />
      <div class="db-right">
        <div class="db-right-top">
          <div class="db-tabs">
            <button
              class="db-tab"
              :class="{ active: activeTab === 'structure' }"
              @click="activeTab = 'structure'"
              :disabled="!selectedTable"
            >
              {{ t('db.tableStructure') }}
            </button>
            <button
              class="db-tab"
              :class="{ active: activeTab === 'query' }"
              @click="activeTab = 'query'"
            >
              {{ t('db.sqlQuery') }}
            </button>
          </div>
          <div class="db-right-top-content">
            <DBTableStructure
              v-if="activeTab === 'structure'"
              :session-id="sessionId"
              :db-name="selectedDb"
              :table-name="selectedTable"
              @refresh="onRefresh"
            />
            <DBQueryEditor
              v-else
              :session-id="sessionId"
              :table-name="selectedTable"
              :db-name="selectedDb"
              :primary-keys="primaryKeys"
              @cell-updated="onRefresh"
            />
          </div>
        </div>
        <div class="db-right-bottom">
          <DBQueryHistory
            :session-id="sessionId"
            :refresh-trigger="historyRefresh"
            @replay="onReplay"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../i18n'
import { GetTableSchema } from '../../wailsjs/go/main/App'
import DBTreePanel from './DBTreePanel.vue'
import DBTableStructure from './DBTableStructure.vue'
import DBQueryEditor from './DBQueryEditor.vue'
import DBQueryHistory from './DBQueryHistory.vue'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
}>()

const activeTab = ref<'structure' | 'query'>('query')
const selectedDb = ref('')
const selectedTable = ref('')
const primaryKeys = ref<string[]>([])
const historyRefresh = ref(0)

const leftWidth = ref(220)
let resizeStartX = 0
let resizeStartWidth = 0

function onSelectDatabase(dbName: string) {
  selectedDb.value = dbName
}

async function onSelectTable(dbName: string, tableName: string) {
  selectedDb.value = dbName
  selectedTable.value = tableName
  try {
    const schema = await GetTableSchema(props.sessionId, dbName, tableName)
    primaryKeys.value = schema.columns.filter(c => c.isPrimary).map(c => c.name)
  } catch {
    primaryKeys.value = []
  }
  activeTab.value = 'structure'
}

function onRefresh() {
  historyRefresh.value++
}

function onReplay(sql: string) {
  activeTab.value = 'query'
}

function onResizeStart(e: MouseEvent) {
  resizeStartX = e.clientX
  resizeStartWidth = leftWidth.value
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
}

function onResizeMove(e: MouseEvent) {
  const dx = e.clientX - resizeStartX
  leftWidth.value = Math.max(150, Math.min(500, resizeStartWidth + dx))
}

function onResizeEnd() {
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
}
</script>

<style scoped>
.db-tab-content {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.db-main {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.db-left {
  flex-shrink: 0;
  border-right: 1px solid var(--border-color, #444);
  overflow: auto;
}
.db-resizer {
  width: 4px;
  cursor: col-resize;
  background: transparent;
  flex-shrink: 0;
}
.db-resizer:hover {
  background: var(--color-primary, #409eff);
}
.db-right {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.db-right-top {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.db-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-color, #444);
  padding: 0 8px;
  flex-shrink: 0;
}
.db-tab {
  padding: 6px 16px;
  border: none;
  background: none;
  color: var(--text-secondary, #888);
  cursor: pointer;
  font-size: 13px;
  border-bottom: 2px solid transparent;
}
.db-tab.active {
  color: var(--text-primary, #333);
  border-bottom-color: var(--color-primary, #409eff);
}
.db-tab:disabled {
  opacity: 0.4;
  cursor: default;
}
.db-right-top-content {
  flex: 1;
  overflow: hidden;
}
.db-right-bottom {
  height: 180px;
  border-top: 1px solid var(--border-color, #444);
  overflow: auto;
  flex-shrink: 0;
}
</style>
