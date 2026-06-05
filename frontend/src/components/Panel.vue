<template>
  <div
    class="panel"
    :class="{ 'panel-active': isActive }"
    draggable="true"
    @dragstart="emit('dragstart', $event)"
  >
    <div v-if="showHeader" class="panel-header" :class="{ 'ai-locked': isAILocked }" @dblclick.stop>
      <span v-if="!editing" class="panel-title" @dblclick.stop="startEdit">{{ panel.title }}</span>
      <input
        v-else
        ref="editInputRef"
        v-model="editName"
        class="panel-title-input"
        @keydown.enter="confirmEdit"
        @keydown.escape="cancelEdit"
        @blur="confirmEdit"
        @click.stop
      />
      <div class="panel-header-actions">
        <button
          v-if="(panel.type === 'ssh' || panel.type === 'local') && workspaceId"
          class="panel-broadcast"
          :class="{ active: broadcastActive }"
          @click.stop="tabStore.toggleBroadcast(workspaceId)"
          :title="t('terminal.broadcastInput')"
        >
          <Radio :size="14" />
        </button>
        <button
          v-if="panel.type === 'ssh' || panel.type === 'local'"
          class="panel-ai-lock"
          :class="{ locked: isAILocked }"
          @click.stop="emit('toggleAiLock', panel.id)"
          :title="isAILocked ? t('terminal.aiLockedToPanel') : t('terminal.lockAIToPanel')"
        >
          <Sparkles :size="14" />
        </button>
        <button
          v-if="panel.type === 'ssh' || panel.type === 'local'"
          class="panel-duplicate"
          @click.stop="emit('duplicate', panel.id)"
          :title="t('terminal.duplicate')"
        >
          <Copy :size="14" />
        </button>
        <button class="panel-close" @click.stop="emit('close', panel.id)">×</button>
      </div>
    </div>
    <BaseTerminal
      ref="baseTerminalRef"
      :mode="panel.type === 'local' ? 'local' : 'ssh'"
      :session-id="panel.sessionId"
      :on-session-status="onSessionStatus"
      :broadcast-active="broadcastActive"
      :workspace-id="workspaceId"
      :panel-id="panel.id"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, nextTick } from 'vue'
import { Radio, Sparkles, Copy } from '@lucide/vue'
import BaseTerminal from './BaseTerminal.vue'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'
import { useSessionStore } from '../stores/sessionStore'
import { useSettingsStore } from '../stores/settingsStore'
import { CreateSession } from '../../wailsjs/go/main/App'
import { useI18n } from '../i18n'
import type { Panel } from '../types/workspace'
import type { ConnectionConfig } from '../types/session'

const { t } = useI18n()

const props = defineProps<{
  panel: Panel
  showHeader: boolean
  isActive: boolean
  broadcastActive?: boolean
  workspaceId?: string
}>()

const emit = defineEmits<{
  close: [panelId: string]
  dragstart: [e: DragEvent]
  toggleAiLock: [panelId: string]
  duplicate: [panelId: string]
  rename: [panelId: string, newName: string]
}>()

const tabStore = useTabStore()
const panelStore = usePanelStore()
const sessionStore = useSessionStore()
const settingsStore = useSettingsStore()

const isAILocked = computed(() =>
  tabStore.aiLockedPanelId === props.panel.id
)

const baseTerminalRef = ref<InstanceType<typeof BaseTerminal> | null>(null)

const editing = ref(false)
const editName = ref('')
const editInputRef = ref<HTMLInputElement>()

function startEdit() {
  editName.value = props.panel.title
  editing.value = true
  nextTick(() => {
    editInputRef.value?.focus()
    editInputRef.value?.select()
  })
}

function confirmEdit() {
  if (!editing.value) return
  editing.value = false
  const newName = editName.value.trim()
  if (newName && newName !== props.panel.title) {
    emit('rename', props.panel.id, newName)
  }
}

function cancelEdit() {
  editing.value = false
}

function onSessionStatus(status: string) {
  if (status === 'retry') {
    retryConnection()
  }
}

async function retryConnection() {
  if (props.panel.type === 'local') {
    // Local terminal: reconnect with the same shell used when created
    baseTerminalRef.value?.write('\r\n\x1b[33mRestarting local shell...\x1b[0m\r\n')
    try {
      const shellPath = props.panel.config?.shellPath || ''
      const config = { ...props.panel.config, type: 'local', shellPath } as ConnectionConfig
      const info = await CreateSession('local', config)
      panelStore.bindSession(props.panel.id, info.id)
      sessionStore.initSession(info.id)
    } catch (e: any) {
      baseTerminalRef.value?.write(`\r\n\x1b[31mFailed to start local shell: ${e}\x1b[0m\r\n`)
      baseTerminalRef.value?.setRetryOnEnter(true)
    }
    return
  }
  if (!props.panel.config) return
  baseTerminalRef.value?.write('\r\n\x1b[33mReconnecting...\x1b[0m\r\n')
  try {
    const info = await CreateSession(props.panel.config.type, props.panel.config)
    panelStore.bindSession(props.panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e: any) {
    baseTerminalRef.value?.write(`\r\n\x1b[31mReconnect failed: ${e}\x1b[0m\r\n`)
    baseTerminalRef.value?.setRetryOnEnter(true)
  }
}

// Watch panel sessionId changes and retry resize
watch(() => props.panel.sessionId, (newId) => {
  if (newId) {
    const delays = [200, 400, 600, 800, 1000, 1500, 2000]
    delays.forEach((delay) => {
      setTimeout(() => baseTerminalRef.value?.resize(), delay)
    })
  }
})

watch(() => props.isActive, (active) => {
  if (active) {
    nextTick(() => baseTerminalRef.value?.focus())
  }
})
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: var(--bg-base);
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
  cursor: grab;
}
.panel-header:active {
  cursor: grabbing;
}
.panel-active .panel-header {
  background: var(--bg-elevated);
  border-bottom-color: var(--accent-dim);
}
.panel-header.ai-locked {
  border-left: 3px solid var(--warning, #f59e0b);
  box-shadow: inset 0 0 12px rgba(245, 158, 11, 0.12);
}
.panel-title {
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: text;
}
.panel-active .panel-title {
  color: var(--text-primary);
}
.panel-title-input {
  font-size: 12px;
  font-family: inherit;
  color: var(--text-primary);
  background: var(--bg-base);
  border: 1px solid var(--accent-dim);
  border-radius: var(--radius-sm);
  padding: 2px 6px;
  width: 120px;
  outline: none;
}
.panel-header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.panel-broadcast {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 4px;
  border-radius: 3px;
  line-height: 1;
}
.panel-broadcast:hover {
  background: var(--bg-hover);
}
.panel-broadcast.active {
  color: var(--accent, #22d3ee);
  background: var(--accent-subtle);
}
.broadcast-icon {
  display: inline-block;
  line-height: 1;
}
.panel-ai-lock {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
}
.ai-lock-icon {
  display: block;
}
.panel-ai-lock:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.panel-ai-lock.locked {
  color: var(--warning, #f59e0b);
}
.panel-duplicate {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 3px;
  display: inline-flex;
  align-items: center;
}
.panel-duplicate:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
.panel-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  cursor: pointer;
  font-size: 14px;
  transition: all 0.12s ease;
}
.panel-close:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
</style>
