<template>
  <div class="db-table-structure">
    <div v-if="!tableName" class="placeholder">{{ t('db.selectTableHint') }}</div>
    <template v-else>
      <div class="section">
        <div class="section-title">{{ t('db.columns') }}</div>
        <el-table :data="schema?.columns || []" border size="small" style="width:100%">
          <el-table-column prop="name" :label="t('db.colName')" />
          <el-table-column prop="type" :label="t('db.colType')" />
          <el-table-column :label="t('db.colNullable')" width="80">
            <template #default="{ row }">
              {{ row.nullable ? 'YES' : 'NO' }}
            </template>
          </el-table-column>
          <el-table-column prop="defaultVal" :label="t('db.colDefault')" />
          <el-table-column :label="t('db.colPrimary')" width="70">
            <template #default="{ row }">
              <span v-if="row.isPrimary">PK</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('db.actions')" width="120">
            <template #default="{ row }">
              <button class="action-btn" @click="startEditColumn(row)">{{ t('db.edit') }}</button>
              <button class="action-btn danger" @click="onDropColumn(row.name)">{{ t('db.drop') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="section">
        <div class="section-title">{{ t('db.indexes') }}</div>
        <el-table :data="schema?.indexes || []" border size="small" style="width:100%">
          <el-table-column prop="name" :label="t('db.idxName')" />
          <el-table-column :label="t('db.idxColumns')">
            <template #default="{ row }">
              {{ row.columns?.join(', ') }}
            </template>
          </el-table-column>
          <el-table-column :label="t('db.idxUnique')" width="80">
            <template #default="{ row }">
              {{ row.unique ? 'YES' : 'NO' }}
            </template>
          </el-table-column>
          <el-table-column :label="t('db.actions')" width="80">
            <template #default="{ row }">
              <button class="action-btn danger" @click="onDropIndex(row.name)">{{ t('db.drop') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-dialog v-model="editDialogVisible" :title="t('db.editColumn')" width="400px">
        <el-form v-if="editingColumn" label-width="100px">
          <el-form-item :label="t('db.colName')">
            <el-input v-model="editingColumn.name" disabled />
          </el-form-item>
          <el-form-item :label="t('db.colType')">
            <el-input v-model="editColumnType" />
          </el-form-item>
          <el-form-item :label="t('db.colNullable')">
            <el-switch v-model="editColumnNullable" />
          </el-form-item>
          <el-form-item :label="t('db.colDefault')">
            <el-input v-model="editColumnDefault" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="editDialogVisible = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" @click="onSaveColumnEdit">{{ t('common.save') }}</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { GetTableSchema, AlterTable } from '../../wailsjs/go/main/App'
import type { SchemaResult, ColumnInfo } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
  tableName: string
  dbName: string
}>()

const emit = defineEmits<{
  refresh: []
}>()

const schema = ref<SchemaResult | null>(null)
const editDialogVisible = ref(false)
const editingColumn = ref<ColumnInfo | null>(null)
const editColumnType = ref('')
const editColumnNullable = ref(false)
const editColumnDefault = ref('')

watch(() => props.tableName, async (name) => {
  if (!name) return
  await loadSchema()
})

async function loadSchema() {
  if (!props.tableName) return
  try {
    schema.value = await GetTableSchema(props.sessionId, props.dbName, props.tableName)
  } catch (e) {
    console.error('Failed to load schema:', e)
  }
}

function startEditColumn(col: ColumnInfo) {
  editingColumn.value = col
  editColumnType.value = col.type
  editColumnNullable.value = col.nullable
  editColumnDefault.value = col.defaultVal
  editDialogVisible.value = true
}

async function onSaveColumnEdit() {
  if (!editingColumn.value) return
  const col = editingColumn.value
  const sql = `ALTER TABLE \`${props.tableName}\` MODIFY COLUMN \`${col.name}\` ${editColumnType.value}${editColumnNullable.value ? ' NULL' : ' NOT NULL'}${editColumnDefault.value ? ` DEFAULT '${editColumnDefault.value}'` : ''}`
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    editDialogVisible.value = false
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to alter column:', e)
  }
}

async function onDropColumn(colName: string) {
  const sql = `ALTER TABLE \`${props.tableName}\` DROP COLUMN \`${colName}\``
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to drop column:', e)
  }
}

async function onDropIndex(idxName: string) {
  const sql = `DROP INDEX \`${idxName}\` ON \`${props.tableName}\``
  try {
    await AlterTable(props.sessionId, props.dbName, props.tableName, sql)
    await loadSchema()
    emit('refresh')
  } catch (e) {
    console.error('Failed to drop index:', e)
  }
}
</script>

<style scoped>
.db-table-structure {
  height: 100%;
  overflow: auto;
  padding: 8px;
}
.placeholder {
  color: var(--text-secondary, #888);
  text-align: center;
  padding: 40px 0;
}
.section {
  margin-bottom: 16px;
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary, #333);
}
.action-btn {
  border: none;
  background: var(--color-primary, #409eff);
  color: #fff;
  padding: 2px 8px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 12px;
  margin-right: 4px;
}
.action-btn.danger {
  background: var(--color-danger, #f56c6c);
}
</style>
