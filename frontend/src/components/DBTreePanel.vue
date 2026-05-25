<template>
  <div class="db-tree-panel">
    <div class="panel-header">{{ t('db.databases') }}</div>
    <div class="tree-content">
      <el-tree
        :data="treeData"
        :props="treeProps"
        node-key="id"
        :loading="loading"
        highlight-current
        @node-click="onNodeClick"
        default-expand-all
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from '../i18n'
import { GetDatabases, GetTables } from '../../wailsjs/go/main/App'
import type { TableInfo } from '../types/database'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  selectTable: [dbName: string, tableName: string]
  selectDatabase: [dbName: string]
}>()

interface TreeNode {
  id: string
  label: string
  children?: TreeNode[]
}

const treeData = ref<TreeNode[]>([])
const loading = ref(false)
const treeProps = { children: 'children', label: 'label' }

onMounted(async () => {
  loading.value = true
  try {
    const dbs = await GetDatabases(props.sessionId)
    for (const db of dbs) {
      const tables = await GetTables(props.sessionId, db)
      treeData.value.push({
        id: `db:${db}`,
        label: db,
        children: tables.map((t: TableInfo) => ({
          id: `table:${db}:${t.name}`,
          label: t.name,
        }))
      })
    }
  } catch (e) {
    console.error('Failed to load databases:', e)
  } finally {
    loading.value = false
  }
})

function onNodeClick(data: TreeNode) {
  if (data.id.startsWith('table:')) {
    const [, db, table] = data.id.split(':')
    emit('selectTable', db, table)
  } else if (data.id.startsWith('db:')) {
    const db = data.id.slice(3)
    emit('selectDatabase', db)
  }
}
</script>

<style scoped>
.db-tree-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.panel-header {
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #888);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.tree-content {
  flex: 1;
  overflow: auto;
}
</style>
