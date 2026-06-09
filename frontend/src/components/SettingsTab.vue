<template>
  <div class="settings-tab">
    <div class="settings-sidebar">
      <div
        v-for="cat in categories"
        :key="cat.key"
        class="settings-category"
        :class="{ active: settingsStore.activeCategory === cat.key }"
        @click="settingsStore.activeCategory = cat.key"
      >
        <el-icon class="category-icon"><component :is="cat.icon" /></el-icon>
        <span class="category-label">{{ cat.label }}</span>
      </div>
    </div>

    <div class="settings-panel">
      <!-- 基础设置 -->
      <div v-if="settingsStore.activeCategory === 'basic'" class="settings-section">
        <h2 class="section-title">{{ t('settings.basic') }}</h2>

        <div class="settings-group">
          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.theme') }}</div>
              <div class="setting-desc">{{ t('settings.themeDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select v-model="settingsStore.settings.theme" size="small" @change="settingsStore.save()">
                <el-option :label="t('settings.themeDark')" value="dark" />
                <el-option :label="t('settings.themeDeepBlue')" value="deep-blue" />
                <el-option :label="t('settings.themeLight')" value="light" />
                <el-option :label="t('settings.themeSystem')" value="system" />
              </el-select>
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.language') }}</div>
              <div class="setting-desc">{{ t('settings.languageDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select :model-value="settingsStore.settings.language" size="small" @change="settingsStore.updateLanguage">
                <el-option
                  v-for="lang in LANGUAGE_OPTIONS"
                  :key="lang.value"
                  :label="lang.native"
                  :value="lang.value"
                />
                <el-option :label="t('settings.langSystem')" value="system" />
              </el-select>
            </div>
          </div>
        </div>
      </div>

      <!-- 终端配置 -->
      <div v-if="settingsStore.activeCategory === 'terminal'" class="settings-section">
        <h2 class="section-title">{{ t('settings.terminal') }}</h2>

        <div class="settings-group">
          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.colorScheme') }}</div>
              <div class="setting-desc">{{ t('settings.colorSchemeDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select v-model="settingsStore.settings.terminal.theme" size="small" @change="settingsStore.save()">
                <el-option
                  v-for="th in TERMINAL_THEMES"
                  :key="th.value"
                  :label="th.label"
                  :value="th.value"
                />
              </el-select>
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.font') }}</div>
              <div class="setting-desc">{{ t('settings.fontDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select v-model="settingsStore.settings.terminal.fontFamily" size="small" @change="settingsStore.save()">
                <el-option
                  v-for="f in FONT_OPTIONS"
                  :key="f.value"
                  :label="f.label"
                  :value="f.value"
                />
              </el-select>
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.fontSize') }}</div>
              <div class="setting-desc">{{ t('settings.fontSizeDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-input-number
                v-model="settingsStore.settings.terminal.fontSize"
                :min="8"
                :max="32"
                size="small"
                @change="settingsStore.save()"
              />
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.selectionAction') }}</div>
              <div class="setting-desc">{{ t('settings.selectionActionDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select v-model="settingsStore.settings.terminal.selectionAction" size="small" @change="settingsStore.save()">
                <el-option :label="t('settings.selectionNone')" value="none" />
                <el-option :label="t('settings.selectionCopy')" value="copy" />
              </el-select>
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.rightClick') }}</div>
              <div class="setting-desc">{{ t('settings.rightClickDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-select v-model="settingsStore.settings.terminal.rightClickAction" size="small" @change="settingsStore.save()">
                <el-option :label="t('settings.rightClickMenu')" value="menu" />
                <el-option :label="t('settings.rightClickPaste')" value="paste" />
              </el-select>
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.maxHistory') }}</div>
              <div class="setting-desc">{{ t('settings.maxHistoryDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-input-number
                v-model="settingsStore.settings.terminal.maxHistoryLines"
                :min="100"
                :max="50000"
                :step="100"
                size="small"
                @change="settingsStore.save()"
              />
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.smartCompletion') }}</div>
              <div class="setting-desc">{{ t('settings.smartCompletionDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-switch v-model="settingsStore.settings.terminal.smartCompletion" @change="settingsStore.save()" />
            </div>
          </div>

          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.highlight') }}</div>
              <div class="setting-desc">{{ t('settings.highlightDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-switch :model-value="settingsStore.settings.terminal.highlightEnabled ?? true" @update:model-value="(v: boolean) => { settingsStore.settings.terminal.highlightEnabled = v; settingsStore.save() }" />
            </div>
          </div>

        </div>
      </div>

      <!-- Sync settings -->
      <div v-if="settingsStore.activeCategory === 'sync'" class="settings-section sync-settings">
        <h2 class="section-title">{{ t('settings.sync') }}</h2>
        <p class="section-desc">{{ t('settings.syncDesc') }}</p>

        <!-- Empty state: no repo configured -->
        <div v-if="!syncStore.config.repoUrl" class="sync-card">
          <div class="sync-card-header">{{ t('settings.syncRepoCard') }}</div>
          <div class="sync-card-body empty-state">
            <p class="empty-text">{{ t('settings.syncEmptyDesc') }}</p>
            <el-button type="primary" @click="syncStore.showAddRepo = true">
              {{ t('settings.syncAddRepo') }}
            </el-button>
          </div>
        </div>

        <!-- Configured state -->
        <template v-else>
          <!-- Repo config card -->
          <div class="sync-card">
            <div class="sync-card-header">
              <span>{{ t('settings.syncRepoCard') }}</span>
              <el-button size="small" text @click="openEditRepo">{{ t('settings.syncEdit') }}</el-button>
            </div>
            <div class="sync-card-body">
              <div class="repo-info">
                <div class="repo-info-row">
                  <span class="repo-label">{{ t('settings.syncRepoUrl') }}</span>
                  <span class="repo-value">{{ syncStore.config.repoUrl }}</span>
                </div>
                <div class="repo-info-row">
                  <span class="repo-label">{{ t('settings.syncUsername') }}</span>
                  <span class="repo-value">{{ syncStore.config.username }}</span>
                </div>
              </div>
              <div class="repo-actions">
                <el-button size="small" @click="syncStore.showChangePassword = true">{{ t('settings.syncChangePassword') }}</el-button>
                <el-button size="small" @click="syncStore.showDeleteRepo = true">{{ t('settings.syncDeleteRepo') }}</el-button>
              </div>
            </div>
          </div>

          <!-- Sync card -->
          <div class="sync-card">
            <div class="sync-card-header">{{ t('settings.syncSyncCard') }}</div>
            <div class="sync-card-body">
              <div class="sync-status">
                <div class="sync-status-row">
                  <span class="sync-label">{{ t('settings.syncLastSync') }}</span>
                  <span class="sync-value">{{ syncStore.formatSyncTime() }}</span>
                  <span v-if="syncStore.config.lastSyncStatus === 'success'" class="sync-tag success">{{ t('settings.syncStatusSuccess') }}</span>
                  <span v-else-if="syncStore.config.lastSyncStatus === 'failed'" class="sync-tag failed">{{ t('settings.syncStatusFailed') }}</span>
                </div>
                <div v-if="syncStore.config.lastSyncStatus === 'failed' && syncStore.config.lastSyncError" class="sync-status-row sync-error">
                  <span class="sync-label">{{ t('settings.syncReason') }}</span>
                  <span class="sync-value error-text">{{ syncStore.config.lastSyncError }}</span>
                </div>
              </div>
              <div class="sync-actions-row">
                <el-button
                  type="primary"
                  :loading="syncStore.syncing"
                  @click="handleSyncNow"
                >
                  {{ t('settings.syncNow') }}
                </el-button>
              </div>
              <div class="sync-auto-row">
                <span class="sync-auto-label">{{ t('settings.syncAuto') }}</span>
                <span class="sync-auto-desc">{{ t('settings.syncAutoDesc') }}</span>
                <el-switch v-model="syncStore.config.autoSync" @change="handleAutoSyncToggle" />
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- 历史记录管理 -->
      <div v-if="settingsStore.activeCategory === 'history'" class="settings-section">
        <h2 class="section-title">{{ t('settings.history') }}</h2>

        <!-- Search -->
        <div class="history-search-bar">
          <el-input
            v-model="historySearch"
            :placeholder="t('settings.historySearchPlaceholder')"
            clearable
            size="small"
          >
            <template #prefix>
              <el-icon><Search :size="14" /></el-icon>
            </template>
          </el-input>
        </div>

        <!-- History list -->
        <div class="history-list-container">
          <div class="history-list-header">
            <el-checkbox
              :model-value="isAllHistorySelected"
              :indeterminate="historySelectedIds.size > 0 && !isAllHistorySelected"
              @change="toggleSelectAllHistory"
            />
            <span class="history-header-label">{{ t('settings.historyCommand') }}</span>
          </div>

          <div class="history-list-body">
            <div
              v-for="entry in historyEntries"
              :key="entry.id"
              class="history-item"
            >
              <el-checkbox
                :model-value="historySelectedIds.has(entry.id)"
                @change="toggleHistorySelection(entry.id)"
              />
              <span class="history-command">{{ entry.command }}</span>
              <el-button
                link
                size="small"
                type="danger"
                @click="deleteHistoryItem(entry.id)"
              >
                <el-icon><Trash2 :size="14" /></el-icon>
              </el-button>
            </div>

            <div v-if="historyEntries.length === 0" class="history-empty">
              {{ t('settings.historyEmpty') }}
            </div>
          </div>

          <!-- Batch actions -->
          <div v-if="historySelectedIds.size > 0" class="history-batch-actions">
            <el-button size="small" type="danger" @click="deleteSelectedHistory">
              {{ t('settings.historyBatchDelete', { count: historySelectedIds.size }) }}
            </el-button>
          </div>
        </div>
      </div>

      <!-- 关于 -->
      <div v-if="settingsStore.activeCategory === 'about'" class="settings-section">
        <h2 class="section-title">{{ t('settings.about') }}</h2>
        <div class="about-content">
          <div class="about-appname">uniTerm</div>
          <p class="about-desc">{{ t('settings.aboutDesc') }}</p>
          <div class="about-version">{{ t('settings.version') }}: {{ appVersion }}</div>
        </div>
      </div>

      <!-- AI助理设置 -->
      <div v-if="settingsStore.activeCategory === 'ai'" class="settings-section">
        <h2 class="section-title">{{ t('settings.ai') }}</h2>

        <div class="settings-group">
          <div class="setting-card">
            <div class="setting-info">
              <div class="setting-title">{{ t('settings.modelList') }}</div>
              <div class="setting-desc">{{ t('settings.modelListDesc') }}</div>
            </div>
            <div class="setting-control">
              <el-button size="small" @click="showModelForm = true">+ {{ t('settings.addModel') }}</el-button>
            </div>
          </div>

          <div
            v-for="model in settingsStore.settings.ai.models"
            :key="model.id"
            class="model-card"
            :class="{ active: model.id === settingsStore.settings.ai.activeModelId }"
          >
            <div class="model-main">
              <el-radio
                :model-value="settingsStore.settings.ai.activeModelId"
                :label="model.id"
                @change="settingsStore.setActiveModel(model.id)"
              >
                <span class="model-name">{{ model.name }}</span>
              </el-radio>
              <span class="model-detail">{{ model.model }} @ {{ model.baseURL }}</span>
            </div>
            <div class="model-actions">
              <el-button link size="small" @click="editModel(model)">
                <el-icon><Pencil :size="14" /></el-icon>
              </el-button>
              <el-button link size="small" type="danger" @click="settingsStore.removeModel(model.id)">
                <el-icon><Trash2 :size="14" /></el-icon>
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Model Form Dialog -->
    <el-dialog v-model="showModelForm" :title="editingModel ? t('settings.editModel') : t('settings.newModel')" width="400px">
      <el-form label-width="80px">
        <el-form-item :label="t('settings.modelName')">
          <el-input v-model="modelForm.name" />
        </el-form-item>
        <el-form-item :label="t('settings.modelBaseURL')">
          <el-input v-model="modelForm.baseURL" />
        </el-form-item>
        <el-form-item :label="t('settings.modelApiKey')">
          <el-input v-model="modelForm.apiKey" type="password" show-password />
        </el-form-item>
        <el-form-item :label="t('settings.modelModel')">
          <div class="model-fetch-row">
            <el-autocomplete
              v-model="modelForm.model"
              :fetch-suggestions="(qs, cb) => cb(qs ? modelSuggestions.filter(s => s.value.toLowerCase().includes(qs.toLowerCase())) : modelSuggestions)"
              class="model-autocomplete"
            />
            <el-button size="small" :loading="modelFetching" @click="fetchModelList">
              {{ t('settings.fetchModels') }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showModelForm = false">{{ t('settings.cancel') }}</el-button>
        <el-button type="primary" @click="saveModel">{{ t('settings.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Sync dialogs -->
    <AddRepoDialog />
    <EditRepoDialog />
    <ChangePasswordDialog />
    <DeleteRepoDialog />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { Settings, Monitor, MessageCircleMore, Info, RefreshCw, Pencil, Trash2, History, Search } from '@lucide/vue'
import { ElMessage } from 'element-plus'
import { FetchModels } from '../../wailsjs/go/main/App'
import { useSettingsStore } from '../stores/settingsStore'
import { useSyncStore } from '../stores/syncStore'
import { useI18n } from '../i18n'
import { useSuggestions } from '../composables/useSuggestions'
import type { HistoryEntry } from '../composables/useSuggestions'
import { TERMINAL_THEMES, FONT_OPTIONS, LANGUAGE_OPTIONS } from '../types/settings'
import type { AIModelConfig } from '../types/settings'
import AddRepoDialog from './AddRepoDialog.vue'
import EditRepoDialog from './EditRepoDialog.vue'
import ChangePasswordDialog from './ChangePasswordDialog.vue'
import DeleteRepoDialog from './DeleteRepoDialog.vue'

const settingsStore = useSettingsStore()
const syncStore = useSyncStore()
const { t } = useI18n()

const appVersion = import.meta.env.VITE_VERSION || 'dev'

const suggestions = useSuggestions()
const historySearch = ref('')
const historySelectedIds = ref<Set<string>>(new Set())
const historyList = ref<HistoryEntry[]>([])

const historyEntries = computed(() => {
  const query = historySearch.value.trim().toLowerCase()
  if (!query) return [...historyList.value].reverse()
  return historyList.value.filter(e => e.command.toLowerCase().includes(query)).reverse()
})

const isAllHistorySelected = computed(() => {
  if (historyEntries.value.length === 0) return false
  return historyEntries.value.every(e => historySelectedIds.value.has(e.id))
})

async function refreshHistory() {
  historyList.value = await suggestions.loadHistory()
}

function toggleSelectAllHistory() {
  if (isAllHistorySelected.value) {
    historySelectedIds.value.clear()
  } else {
    historyEntries.value.forEach(e => historySelectedIds.value.add(e.id))
  }
}

function toggleHistorySelection(id: string) {
  if (historySelectedIds.value.has(id)) {
    historySelectedIds.value.delete(id)
  } else {
    historySelectedIds.value.add(id)
  }
}

async function deleteSelectedHistory() {
  const ids = Array.from(historySelectedIds.value)
  if (ids.length === 0) return
  suggestions.removeHistoryCommandsById(ids)
  historySelectedIds.value.clear()
  // Remove from local list immediately for responsive UI
  historyList.value = historyList.value.filter(e => !ids.includes(e.id))
}

function deleteHistoryItem(id: string) {
  suggestions.removeHistoryCommandById(id)
  historySelectedIds.value.delete(id)
  // Remove from local list immediately for responsive UI
  historyList.value = historyList.value.filter(e => e.id !== id)
}

watch(() => settingsStore.activeCategory, async (cat) => {
  if (cat === 'history') {
    await refreshHistory()
  }
})

function openEditRepo() {
  syncStore.showEditRepo = true
}

async function handleSyncNow() {
  const result = await syncStore.doSync()
  if (!result) {
    ElMessage.error(syncStore.lastResult || t('settings.syncFailed'))
    return
  }
  if (result.direction === 3) {
    return  // conflict — handled by SyncConflictDialog
  }
  ElMessage.success(result.message || t('settings.syncSuccess'))
}

async function handleAutoSyncToggle() {
  try {
    await syncStore.saveConfig()
  } catch (e) {
    console.error('Failed to save auto sync toggle:', e)
  }
}

syncStore.loadConfig()

watch(() => settingsStore.openCategory, (cat) => {
  if (cat && (cat === 'basic' || cat === 'terminal' || cat === 'ai' || cat === 'sync' || cat === 'history' || cat === 'about')) {
    settingsStore.activeCategory = cat
    settingsStore.openCategory = null
  }
})

const categories = computed(() => {
  // Explicitly read language and smartCompletion to ensure reactivity tracking
  void settingsStore.settings.language
  const smartOn = settingsStore.settings.terminal.smartCompletion ?? true
  const cats = [
    { key: 'basic', label: t('settings.basic'), icon: Settings },
    { key: 'terminal', label: t('settings.terminal'), icon: Monitor },
    { key: 'ai', label: t('settings.ai'), icon: MessageCircleMore },
    { key: 'sync', label: t('settings.sync'), icon: RefreshCw },
    { key: 'about', label: t('settings.about'), icon: Info },
  ]
  if (smartOn) {
    cats.splice(4, 0, { key: 'history', label: t('settings.history'), icon: History })
  }
  return cats
})

const showModelForm = ref(false)
const modelSuggestions = ref<Array<{ value: string }>>([])
const modelFetching = ref(false)
const editingModel = ref<AIModelConfig | null>(null)
const modelForm = reactive({
  id: '',
  name: '',
  baseURL: '',
  model: '',
  apiKey: '',
})

function editModel(model: AIModelConfig) {
  editingModel.value = model
  modelSuggestions.value = []
  Object.assign(modelForm, { ...model })
  showModelForm.value = true
}

function saveModel() {
  if (editingModel.value) {
    settingsStore.updateModel(editingModel.value.id, { ...modelForm })
  } else {
    settingsStore.addModel({
      id: `model-${Date.now()}`,
      name: modelForm.name || 'Unnamed',
      baseURL: modelForm.baseURL,
      model: modelForm.model,
      apiKey: modelForm.apiKey
    })
  }
  showModelForm.value = false
  editingModel.value = null
  resetModelForm()
}

function resetModelForm() {
  modelForm.id = ''
  modelForm.name = ''
  modelForm.baseURL = ''
  modelForm.model = ''
  modelForm.apiKey = ''
  modelSuggestions.value = []
}

async function fetchModelList() {
  if (!modelForm.apiKey || !modelForm.baseURL) {
    ElMessage.warning(t('settings.fetchModelsHint'))
    return
  }
  modelFetching.value = true
  modelSuggestions.value = []
  try {
    const models = await FetchModels(modelForm.apiKey, modelForm.baseURL)
    modelSuggestions.value = (models || []).map(m => ({
      value: m.display_name || m.id
    }))
    ElMessage.success(t('settings.fetchModelsSuccess', { count: modelSuggestions.value.length }))
  } catch (e: any) {
    ElMessage.error(t('settings.fetchModelsFailed'))
  } finally {
    modelFetching.value = false
  }
}

function getShellLabel(path: string): string {
  const lower = path.toLowerCase()
  if (lower.includes('pwsh')) return 'PowerShell'
  if (lower.includes('powershell')) return 'Windows PowerShell'
  if (lower.includes('bash')) return 'Git Bash'
  if (lower.includes('cmd')) return 'Command Prompt'
  return path.split(/[\\/]/).pop() || path
}
</script>

<style scoped>
.settings-tab {
  display: flex;
  width: 100%;
  max-width: 960px;
  height: 100%;
  margin: 0 auto;
  background: var(--bg-base);
  color: var(--text-primary);
}

.settings-sidebar {
  width: 180px;
  flex-shrink: 0;
  padding: 16px 0;
  border-right: 1px solid var(--border-hover);
}

.settings-category {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  margin: 0 8px;
  font-size: 13px;
  font-family: var(--font-ui);
  cursor: pointer;
  user-select: none;
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  transition: all 0.12s ease;
  border-left: 3px solid transparent;
}

.settings-category:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.settings-category.active {
  background: var(--accent-subtle);
  color: var(--accent);
  border-left-color: var(--accent);
}

.category-icon {
  font-size: 16px;
}

.category-label {
  line-height: 1;
}

.settings-panel {
  flex: 1;
  padding: 24px 32px;
  overflow-y: auto;
  min-width: 0;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  font-family: var(--font-ui);
  margin: 0 0 20px 0;
  color: var(--text-primary);
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all 0.12s ease;
}

.setting-card:hover {
  border-color: var(--border-hover);
}

.setting-info {
  flex: 1;
  min-width: 0;
}

.setting-title {
  font-size: 13px;
  font-weight: 500;
  font-family: var(--font-ui);
  color: var(--text-primary);
  margin-bottom: 2px;
}

.setting-desc {
  font-size: 11px;
  font-family: var(--font-ui);
  color: var(--text-muted);
  line-height: 1.4;
}

.setting-control {
  flex-shrink: 0;
  min-width: 210px;
}

.setting-control .el-select,
.setting-control .el-input-number {
  width: 100%;
}

/* Model cards */
.model-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 18px;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all 0.12s ease;
}

.model-card:hover {
  border-color: var(--border-hover);
}

.model-card.active {
  border-color: var(--accent);
  background: var(--accent-subtle);
}

.model-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.model-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.model-detail {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  margin-left: 24px;
}

.model-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.about-content {
  text-align: left;
  padding: 20px 0;
}
.about-appname {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 12px;
}
.about-desc {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0 0 24px 0;
  line-height: 1.6;
  max-width: 400px;
}
.about-version {
  font-size: 12px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.section-desc {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
}

.sync-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  margin-bottom: 16px;
  overflow: hidden;
}

.sync-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  font-weight: 600;
  font-family: var(--font-ui);
  color: var(--text-primary);
  padding: 8px 12px 8px 18px;
  background: var(--bg-hover);
  border-bottom: 1px solid var(--border-subtle);
}

.sync-card-body {
  padding: 16px 18px;
}

.sync-card-body.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 28px 18px;
}

.empty-text {
  font-size: 13px;
  color: var(--text-muted);
  margin: 0;
}

/* Repo config */
.repo-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 12px;
}

.repo-info-row {
  display: flex;
  gap: 12px;
  font-size: 13px;
}

.repo-label {
  color: var(--text-muted);
  min-width: 70px;
  flex-shrink: 0;
}

.repo-value {
  color: var(--text-primary);
  font-family: var(--font-mono);
  word-break: break-all;
}

.repo-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-5);
  border-radius: 6px;
  margin-bottom: 14px;
  color: var(--el-color-warning-dark-2);
  font-size: 12px;
  line-height: 1.5;
}

.repo-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.repo-actions-left {
  display: flex;
  gap: 8px;
}

/* Sync status */
.sync-status {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
}

.sync-status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.sync-label {
  color: var(--text-muted);
  min-width: 70px;
  flex-shrink: 0;
}

.sync-value {
  color: var(--text-primary);
}

.sync-tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 500;
}

.sync-tag.success {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success-dark-2);
}

.sync-tag.failed {
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger-dark-2);
}

.sync-error {
  align-items: flex-start;
}

.error-text {
  color: var(--el-color-danger);
}

.sync-actions-row {
  margin-bottom: 14px;
}

.sync-auto-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--border-subtle);
}

.sync-auto-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.sync-auto-desc {
  font-size: 12px;
  color: var(--text-muted);
  flex: 1;
}

/* History management */
.history-search-bar {
  margin-bottom: 12px;
}
.history-search-bar .el-input {
  width: 100%;
}
.history-list-container {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.history-list-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--bg-hover);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}
.history-header-label {
  flex: 1;
}
.history-list-body {
  max-height: 400px;
  overflow-y: auto;
}
.history-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border-subtle);
  font-family: var(--font-mono);
  font-size: 12px;
  transition: background 0.1s ease;
}
.history-item:last-child {
  border-bottom: none;
}
.history-item:hover {
  background: var(--bg-hover);
}
.history-command {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-primary);
}
.history-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}
.history-batch-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 10px 14px;
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-hover);
}

.model-fetch-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.model-autocomplete {
  flex: 1;
}
</style>
