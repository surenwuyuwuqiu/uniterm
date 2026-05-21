<template>
  <div ref="containerRef" class="rdp-tab-content">
    <!-- Connecting state -->
    <div v-if="status === 'connecting'" class="rdp-overlay">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>{{ t('rdp.connecting', { host: config?.host || '...' }) }}</p>
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="rdp-overlay">
      <p class="rdp-error-text">{{ t('rdp.error') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('rdp.retry') }}</el-button>
    </div>

    <!-- Disconnected state -->
    <div v-else-if="status === 'disconnected'" class="rdp-overlay">
      <p>{{ t('rdp.disconnected') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('rdp.reconnect') }}</el-button>
    </div>

    <!-- Connected: placeholder div overlaid by native RDP popup window -->
    <div
      v-show="status === 'connected'"
      ref="rdpAreaRef"
      class="rdp-area"
      :class="{ 'rdp-fixed': sizeMode === 'fixed' }"
      :style="rdpAreaStyle"
    />

    <!-- Status bar -->
    <div v-if="status === 'connected'" class="rdp-statusbar">
      <span class="rdp-status-dot" />
      <span>{{ t('rdp.connected') }}</span>
      <span class="rdp-status-sep">|</span>
      <span>{{ config?.host }}:{{ config?.port || 3389 }}</span>
      <span v-if="sizeMode === 'fixed'" class="rdp-status-sep">|</span>
      <span v-if="sizeMode === 'fixed'">{{ t('rdp.resolution') }}: {{ config?.rdpFixedWidth }}×{{ config?.rdpFixedHeight }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'
import { CreateSession, CloseSession, RDPSetPosition, RDPHide } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const { t } = useI18n()

const props = defineProps<{
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null
}>()

const containerRef = ref<HTMLElement>()
const rdpAreaRef = ref<HTMLElement>()
const status = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
const currentSessionId = ref<string | null>(props.sessionId)
const sizeMode = computed(() => props.config?.rdpSizeMode || 'follow')

const rdpAreaStyle = computed(() => {
  if (sizeMode.value === 'fixed' && props.config?.rdpFixedWidth && props.config?.rdpFixedHeight) {
    return {
      width: props.config.rdpFixedWidth + 'px',
      height: props.config.rdpFixedHeight + 'px',
    }
  }
  return {}
})

// --- Positioning ---
// Convert viewport-relative div position to screen coordinates and send to backend.
let lastX = -1, lastY = -1, lastW = -1, lastH = -1

function syncRDPPosition() {
  if (!rdpAreaRef.value || !currentSessionId.value || status.value !== 'connected') return
  const rect = rdpAreaRef.value.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return
  const dpr = window.devicePixelRatio || 1
  const x = Math.round((window.screenLeft + rect.left) * dpr)
  const y = Math.round((window.screenTop + rect.top) * dpr)
  const w = Math.round(rect.width * dpr)
  const h = Math.round(rect.height * dpr)
  if (x === lastX && y === lastY && w === lastW && h === lastH) return
  lastX = x; lastY = y; lastW = w; lastH = h
  RDPSetPosition(currentSessionId.value, x, y, w, h)
}

async function connect() {
  if (!props.config) return
  status.value = 'connecting'
  try {
    const info = await CreateSession('rdp', props.config)
    currentSessionId.value = info.id
  } catch (e) {
    console.error('RDP connect error:', e)
    status.value = 'error'
  }
}

async function reconnect() {
  if (currentSessionId.value) {
    try { await CloseSession(currentSessionId.value) } catch (_) {}
    currentSessionId.value = null
  }
  await connect()
}

// --- Resize tracking ---

let resizeDebounceTimer: ReturnType<typeof setTimeout> | null = null

function onWindowResize() {
  syncRDPPosition()
  // Delayed re-sync to catch final layout after maximize/restore transitions
  if (resizeDebounceTimer) clearTimeout(resizeDebounceTimer)
  resizeDebounceTimer = setTimeout(() => syncRDPPosition(), 300)
}

// --- Events ---

EventsOn('session:status', (data: any) => {
  if (data.id !== currentSessionId.value) return
  switch (data.status) {
    case 'connected':
      status.value = 'connected'
      nextTick(() => requestAnimationFrame(() => syncRDPPosition()))
      break
    case 'disconnected':
      if (status.value !== 'error') status.value = 'disconnected'
      if (currentSessionId.value) RDPHide(currentSessionId.value)
      break
    case 'error':
      status.value = 'error'
      if (currentSessionId.value) RDPHide(currentSessionId.value)
      break
  }
})

EventsOn('session:data', (data: any) => {
  if (data.id === currentSessionId.value && data.data?.includes('[Connection failed')) {
    status.value = 'error'
  }
})

onMounted(() => {
  if (props.sessionId) {
    currentSessionId.value = props.sessionId
  }
  // Only use debounced window resize for maximize/restore.
  // ResizeObserver removed: fired too frequently during resize, flooding RDP.
  window.addEventListener('resize', onWindowResize)
  // If session already exists (tab switch back), show connected state immediately
  if (currentSessionId.value) {
    status.value = 'connected'
    nextTick(() => requestAnimationFrame(() => syncRDPPosition()))
  } else {
    status.value = 'connecting'
  }
})

watch(() => props.sessionId, (newId) => {
  if (newId && !currentSessionId.value) {
    currentSessionId.value = newId
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', onWindowResize)
  if (resizeDebounceTimer) clearTimeout(resizeDebounceTimer)
  // Only hide, don't close — session persists across tab switches.
  // Session cleanup is handled by the backend when the tab is closed or app exits.
  if (currentSessionId.value) {
    RDPHide(currentSessionId.value)
  }
})

defineExpose({ syncRDPPosition })
</script>

<style scoped>
.rdp-tab-content {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: #000;
  position: relative;
}
.rdp-area {
  flex: 1;
  background: #000;
}
.rdp-area.rdp-fixed {
  margin: 0 auto;
  flex: none;
}
.rdp-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #999;
  z-index: 10;
}
.rdp-error-text { color: #f56c6c; }
.rdp-statusbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: #1e1e1e;
  color: #999;
  font-size: 12px;
  flex-shrink: 0;
}
.rdp-status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: #67c23a;
}
.rdp-status-sep { color: #444; }
</style>
