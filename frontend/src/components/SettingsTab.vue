<template>
  <div class="settings-tab" :key="settingsStore.settings.language">
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
              <el-select v-model="settingsStore.settings.language" size="small" @change="settingsStore.save()">
                <el-option :label="t('settings.langZhCN')" value="zh-CN" />
                <el-option :label="t('settings.langEn')" value="en" />
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

        </div>
      </div>

      <!-- Sync settings -->
      <div v-if="settingsStore.activeCategory === 'sync'" class="settings-section sync-settings">
        <div class="section-header">
          <h2>{{ t('settings.sync') }}</h2>
          <p class="section-desc">{{ t('settings.syncDesc') }}</p>
        </div>

        <div class="sync-warning">
          <AlertTriangle :size="16" />
          <span>{{ t('settings.syncWarning') }}</span>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncRepoUrl') }}</label>
            <p class="setting-desc">{{ t('settings.syncRepoUrlDesc') }}</p>
          </div>
          <div class="setting-control">
            <el-input
              v-model="syncStore.config.repoUrl"
              :placeholder="t('settings.syncRepoUrlPlaceholder')"
              size="default"
              style="width: 400px"
            />
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncAuthType') }}</label>
          </div>
          <div class="setting-control">
            <el-radio-group v-model="syncStore.config.authType">
              <el-radio value="ssh">SSH Key</el-radio>
              <el-radio value="token">Personal Access Token</el-radio>
            </el-radio-group>
          </div>
        </div>

        <div v-if="syncStore.config.authType === 'token'" class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncToken') }}</label>
          </div>
          <div class="setting-control">
            <el-input
              v-model="tokenInput"
              :type="showToken ? 'text' : 'password'"
              :placeholder="t('settings.syncTokenPlaceholder')"
              size="default"
              style="width: 300px"
            >
              <template #suffix>
                <el-button link @click="showToken = !showToken">
                  {{ showToken ? t('settings.syncHide') : t('settings.syncShow') }}
                </el-button>
              </template>
            </el-input>
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncAuto') }}</label>
            <p class="setting-desc">{{ t('settings.syncAutoDesc') }}</p>
          </div>
          <div class="setting-control">
            <el-switch v-model="syncStore.config.autoSync" />
          </div>
        </div>

        <div class="setting-item">
          <div class="setting-label">
            <label>{{ t('settings.syncLastTime') }}</label>
          </div>
          <div class="setting-control sync-time">
            {{ syncStore.lastSyncTime }}
          </div>
        </div>

        <div class="sync-actions">
          <el-button
            :loading="syncStore.testingConnection"
            @click="handleTestConnection"
          >
            {{ t('settings.syncTestConnection') }}
          </el-button>
          <el-button
            type="primary"
            :loading="syncStore.syncing"
            @click="handleSyncNow"
          >
            {{ t('settings.syncNow') }}
          </el-button>
        </div>

        <div v-if="syncStore.lastResult" class="sync-result">
          {{ syncStore.lastResult }}
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
        <el-form-item :label="t('settings.modelModel')">
          <el-input v-model="modelForm.model" />
        </el-form-item>
        <el-form-item :label="t('settings.modelApiKey')">
          <el-input v-model="modelForm.apiKey" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showModelForm = false">{{ t('settings.cancel') }}</el-button>
        <el-button type="primary" @click="saveModel">{{ t('settings.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { Settings, Monitor, MessageCircleMore, Info, RefreshCw, AlertTriangle, Pencil, Trash2 } from '@lucide/vue'
import { useSettingsStore } from '../stores/settingsStore'
import { useSyncStore } from '../stores/syncStore'
import { useI18n } from '../i18n'
import { TERMINAL_THEMES, FONT_OPTIONS } from '../types/settings'
import type { AIModelConfig } from '../types/settings'

const settingsStore = useSettingsStore()
const syncStore = useSyncStore()
const { t } = useI18n()

const appVersion = import.meta.env.VITE_VERSION || 'dev'

const tokenInput = ref('')
const showToken = ref(false)

async function handleTestConnection() {
  await syncStore.saveConfig(tokenInput.value)
  const err = await syncStore.testConnection()
  if (err) {
    ElMessage.error(t('settings.syncTestFailed', { error: err }))
  } else {
    ElMessage.success(t('settings.syncTestSuccess'))
  }
}

async function handleSyncNow() {
  await syncStore.saveConfig(tokenInput.value)
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

syncStore.loadConfig()

watch(() => settingsStore.openCategory, (cat) => {
  if (cat && (cat === 'basic' || cat === 'terminal' || cat === 'ai' || cat === 'sync' || cat === 'about')) {
    settingsStore.settingsStore.activeCategory = cat
    settingsStore.openCategory = null
  }
})

const categories = computed(() => {
  // Explicitly read language to ensure reactivity tracking
  void settingsStore.settings.language
  return [
    { key: 'basic', label: t('settings.basic'), icon: Settings },
    { key: 'terminal', label: t('settings.terminal'), icon: Monitor },
    { key: 'ai', label: t('settings.ai'), icon: MessageCircleMore },
    { key: 'sync', label: t('settings.sync'), icon: RefreshCw },
    { key: 'about', label: t('settings.about'), icon: Info },
  ]
})

const showModelForm = ref(false)
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

.sync-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-5);
  border-radius: 6px;
  margin-bottom: 20px;
  color: var(--el-color-warning-dark-2);
  font-size: 13px;
}

.sync-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

.sync-result {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.sync-time {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
</style>
