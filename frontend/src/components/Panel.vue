<template>
  <div
    class="panel"
    :class="{ 'panel-active': isActive }"
    draggable="true"
    @dragstart="emit('dragstart', $event)"
  >
    <div v-if="showHeader" class="panel-header" :class="{ 'ai-locked': isAILocked }" @dblclick.stop>
      <span class="panel-title">{{ panel.title }}</span>
      <div class="panel-header-actions">
        <button
          v-if="(panel.type === 'ssh' || panel.type === 'local') && workspaceId"
          class="panel-broadcast"
          :class="{ active: broadcastActive }"
          @click.stop="tabStore.toggleBroadcast(workspaceId)"
          :title="t('terminal.broadcastInput')"
        >
          <svg class="broadcast-icon" xmlns="http://www.w3.org/2000/svg" height="14" viewBox="0 -960 960 960" width="14" fill="currentColor"><path d="M600-160v-80H440v-200h-80v80H80v-240h280v80h80v-200h160v-80h280v240H600v-80h-80v320h80v-80h280v240H600Zm80-80h120v-80H680v80ZM160-440h120v-80H160v80Zm520-200h120v-80H680v80Zm0 400v-80 80ZM280-440v-80 80Zm400-200v-80 80Z"/></svg>
        </button>
        <button
          v-if="panel.type === 'ssh' || panel.type === 'local'"
          class="panel-ai-lock"
          :class="{ locked: isAILocked }"
          @click.stop="emit('toggleAiLock', panel.id)"
          :title="isAILocked ? t('terminal.aiLockedToPanel') : t('terminal.lockAIToPanel')"
        >
          <svg class="ai-lock-icon" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24"><path d="M0 0h24v24H0z" fill="none"/><path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16v-6a2 2 0 1 1 4 0v6m-4-3h4m4-5v8"/></svg>
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
}>()

const tabStore = useTabStore()
const panelStore = usePanelStore()
const sessionStore = useSessionStore()
const settingsStore = useSettingsStore()

const isAILocked = computed(() =>
  tabStore.aiLockedPanelId === props.panel.id
)

const baseTerminalRef = ref<InstanceType<typeof BaseTerminal> | null>(null)

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
}
.panel-active .panel-title {
  color: var(--text-primary);
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
