<template>
  <div class="db-query-editor">
    <div class="editor-area">
      <textarea
        v-model="sql"
        class="sql-editor"
        :placeholder="t('db.sqlPlaceholder')"
        @keydown="onKeydown"
        rows="8"
      />
      <div class="editor-actions">
        <button class="exec-btn" @click="onExecute">{{ t('db.execute') }}</button>
        <span class="shortcut-hint">Ctrl+Enter</span>
      </div>
    </div>

    <div v-if="error" class="error-msg">{{ error }}</div>

    <div v-if="execResult" class="result-info">
      {{ t('db.affectedRows') }}: {{ execResult.affected }}
    </div>

    <div v-if="queryResult" class="result-grid">
      <el-table
        :data="queryResult.rows"
        border
        size="small"
        max-height="400"
        style="width:100%"
        @cell-dblclick="onCellDblClick"
      >
        <el-table-column
          v-if="tableName && primaryKeys?.length"
          :label="t('db.actions')"
          width="60"
          fixed="right"
        >
          <template #default="{ row, $index }">
            <button class="action-btn danger" @click="onDeleteRow($index)">X</button>
          </template>
        </el-table-column>
        <el-table-column
          v-for="col in queryResult.columns"
          :key="col.name"
          :prop="col.name"
          :label="col.name"
          min-width="100"
          show-overflow-tooltip
        >
          <template #default="{ row, column, $index }">
            <div
              v-if="editingCell && editingCell.rowIndex === $index && editingCell.colName === column.property"
              class="cell-edit-wrap"
            >
              <input
                ref="cellInputEl"
                v-model="editingCell.value"
                class="cell-edit-input"
                @keydown.enter="onCellEditConfirm"
                @keydown.escape="onCellEditCancel"
                @blur="onCellEditCancel"
              />
            </div>
            <span v-else class="cell-value">{{ row[column.property] }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div class="result-count">{{ queryResult.rows.length }} {{ t('db.rows') }}</div>
    </div>

    <div v-if="queryResult && tableName && primaryKeys?.length" class="insert-row-bar">
      <button class="exec-btn" @click="startInsertRow">{{ t('db.insertRow') }}</button>
    </div>

    <div v-if="insertingRow" class="insert-row-form">
      <div class="insert-row-fields">
        <div v-for="col in insertColumns" :key="col" class="insert-field">
          <label>{{ col }}</label>
          <input v-model="insertValues[col]" class="insert-input" />
        </div>
      </div>
      <div class="insert-actions">
        <button class="exec-btn" @click="onInsertConfirm">{{ t('common.confirm') }}</button>
        <button class="cancel-btn" @click="onInsertCancel">{{ t('common.cancel') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { useI18n } from '../i18n'
import { ExecuteQuery, ExecuteStatement } from '../../wailsjs/go/main/App'
import type { QueryResult, ExecResult } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
  tableName?: string
  dbName?: string
  primaryKeys?: string[]
}>()

const emit = defineEmits<{
  cellUpdated: []
}>()

const sql = ref('')
const queryResult = ref<QueryResult | null>(null)
const execResult = ref<ExecResult | null>(null)
const error = ref('')

function onKeydown(e: KeyboardEvent) {
  if (e.ctrlKey && e.key === 'Enter') {
    e.preventDefault()
    onExecute()
  }
}

async function onExecute() {
  if (!sql.value.trim()) return
  error.value = ''
  queryResult.value = null
  execResult.value = null

  const trimmed = sql.value.trim()
  const isSelect = /^\s*SELECT\b/i.test(trimmed) ||
    /^\s*SHOW\b/i.test(trimmed) ||
    /^\s*DESCRIBE\b/i.test(trimmed) ||
    /^\s*EXPLAIN\b/i.test(trimmed) ||
    /^\s*PRAGMA\b/i.test(trimmed)

  try {
    if (isSelect) {
      queryResult.value = await ExecuteQuery(props.sessionId, trimmed)
    } else {
      execResult.value = await ExecuteStatement(props.sessionId, trimmed)
    }
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

// ── Inline cell editing ──

interface EditingCell {
  rowIndex: number
  colName: string
  originalValue: any
  value: string
}

const editingCell = ref<EditingCell | null>(null)
const cellInputEl = ref<HTMLInputElement | null>(null)

function onCellDblClick(row: any, column: any, _cell: HTMLElement, _event: MouseEvent) {
  if (!props.tableName || !props.primaryKeys || props.primaryKeys.length === 0) return

  const colName = column.property
  const originalValue = row[colName]
  editingCell.value = {
    rowIndex: queryResult.value!.rows.indexOf(row),
    colName,
    originalValue,
    value: originalValue ?? ''
  }
  nextTick(() => {
    cellInputEl.value?.focus()
    cellInputEl.value?.select()
  })
}

async function onCellEditConfirm() {
  if (!editingCell.value || !props.tableName || !props.primaryKeys) return

  const { rowIndex, colName, originalValue, value } = editingCell.value
  if (value === String(originalValue ?? '')) {
    editingCell.value = null
    return
  }

  const row = queryResult.value!.rows[rowIndex]
  const whereParts = props.primaryKeys.map(
    pk => `\`${pk}\` = '${String(row[pk] ?? '').replace(/'/g, "''")}'`
  )
  const whereClause = whereParts.join(' AND ')
  const updateSQL = `UPDATE \`${props.tableName}\` SET \`${colName}\` = '${value.replace(/'/g, "''")}' WHERE ${whereClause}`

  try {
    await ExecuteStatement(props.sessionId, updateSQL)
    queryResult.value!.rows[rowIndex][colName] = value
    error.value = ''
    emit('cellUpdated')
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
  editingCell.value = null
}

function onCellEditCancel() {
  editingCell.value = null
}

// ── Delete row ──

async function onDeleteRow(rowIndex: number) {
  if (!props.tableName || !props.primaryKeys || props.primaryKeys.length === 0) return

  const row = queryResult.value!.rows[rowIndex]
  const whereParts = props.primaryKeys.map(
    pk => `\`${pk}\` = '${String(row[pk] ?? '').replace(/'/g, "''")}'`
  )
  const whereClause = whereParts.join(' AND ')
  const deleteSQL = `DELETE FROM \`${props.tableName}\` WHERE ${whereClause}`

  try {
    await ExecuteStatement(props.sessionId, deleteSQL)
    queryResult.value!.rows.splice(rowIndex, 1)
    error.value = ''
    emit('cellUpdated')
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

// ── Insert row ──

const insertingRow = ref(false)
const insertValues = ref<Record<string, string>>({})
const insertColumns = ref<string[]>([])

function startInsertRow() {
  insertColumns.value = queryResult.value!.columns
    .map(c => c.name)
    .filter(c => !props.primaryKeys?.includes(c))
  insertValues.value = {}
  for (const col of insertColumns.value) {
    insertValues.value[col] = ''
  }
  insertingRow.value = true
}

async function onInsertConfirm() {
  if (!props.tableName) return

  const cols = Object.keys(insertValues.value).map(c => `\`${c}\``).join(', ')
  const vals = Object.values(insertValues.value)
    .map(v => `'${(v ?? '').replace(/'/g, "''")}'`)
    .join(', ')
  const insertSQL = `INSERT INTO \`${props.tableName}\` (${cols}) VALUES (${vals})`

  try {
    await ExecuteStatement(props.sessionId, insertSQL)
    const newRow: Record<string, any> = { ...insertValues.value }
    for (const pk of props.primaryKeys || []) {
      newRow[pk] = '(new)'
    }
    queryResult.value!.rows.push(newRow)
    error.value = ''
    insertingRow.value = false
    emit('cellUpdated')
  } catch (e: any) {
    error.value = e?.message || String(e)
  }
}

function onInsertCancel() {
  insertingRow.value = false
}
</script>

<style scoped>
.db-query-editor {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding: 8px;
  overflow: auto;
}
.editor-area { margin-bottom: 8px; }
.sql-editor {
  width: 100%;
  font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.5;
  background: var(--bg-secondary, #1e1e1e);
  color: var(--text-primary, #d4d4d4);
  border: 1px solid var(--border-color, #444);
  border-radius: 4px;
  padding: 8px;
  resize: vertical;
}
.editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}
.exec-btn {
  padding: 4px 16px;
  background: var(--color-primary, #409eff);
  color: #fff;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}
.shortcut-hint { font-size: 11px; color: var(--text-secondary, #888); }
.error-msg {
  color: var(--color-danger, #f56c6c);
  padding: 8px;
  background: rgba(245, 108, 108, 0.1);
  border-radius: 4px;
  margin-bottom: 8px;
  font-family: monospace;
  font-size: 13px;
}
.result-info { padding: 4px 0; font-size: 13px; color: var(--text-secondary, #888); margin-bottom: 8px; }
.result-grid { flex: 1; overflow: auto; }
.result-count { padding: 4px 0; font-size: 12px; color: var(--text-secondary, #888); }
.cell-value { cursor: default; }
.cell-edit-wrap { margin: -8px -12px; }
.cell-edit-input {
  width: 100%;
  padding: 4px 8px;
  border: 2px solid var(--color-primary, #409eff);
  border-radius: 2px;
  font-size: 13px;
  font-family: inherit;
  outline: none;
}
.action-btn {
  border: none;
  background: var(--color-primary, #409eff);
  color: #fff;
  padding: 2px 6px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 11px;
}
.action-btn.danger { background: var(--color-danger, #f56c6c); }
.insert-row-bar { padding: 4px 0; }
.insert-row-form {
  border: 1px solid var(--color-primary, #409eff);
  border-radius: 4px;
  padding: 8px;
  margin-top: 4px;
}
.insert-row-fields { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
.insert-field { display: flex; flex-direction: column; gap: 2px; }
.insert-field label { font-size: 11px; color: var(--text-secondary, #888); }
.insert-input {
  padding: 4px 8px;
  border: 1px solid var(--border-color, #444);
  border-radius: 3px;
  font-size: 13px;
  width: 140px;
}
.insert-actions { display: flex; gap: 8px; }
.cancel-btn {
  padding: 4px 16px;
  background: var(--bg-secondary, #eee);
  color: var(--text-primary, #333);
  border: 1px solid var(--border-color, #444);
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}
</style>
