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
  // RDPSetPosition shows the RDP at the given position (SWP_SHOWWINDOW)
  RDPSetPosition(currentSessionId.value, x, y, w, h)
}

// --- Layout change: sync position directly ---
// syncRDPPosition skips if position hasn't changed (lastX/lastY check),
// so calling it directly on every event is cheap.

// --- Window resize ---
function onWindowResize() {
  syncRDPPosition()
}

// --- Window move detection ---
// Window resize events don't fire for title-bar drag moves, so poll screen pos.

let movePollTimer: ReturnType<typeof setInterval> | null = null
let lastScreenX = 0
let lastScreenY = 0

function startMovePolling() {
  lastScreenX = window.screenLeft ?? window.screenX ?? 0
  lastScreenY = window.screenTop ?? window.screenY ?? 0
  movePollTimer = setInterval(() => {
    const sx = window.screenLeft ?? window.screenX ?? 0
    const sy = window.screenTop ?? window.screenY ?? 0
    if (sx !== lastScreenX || sy !== lastScreenY) {
      lastScreenX = sx
      lastScreenY = sy
      syncRDPPosition()
    }
  }, 16)
}

function stopMovePolling() {
  if (movePollTimer) { clearInterval(movePollTimer); movePollTimer = null }
}

// --- Sidebar/panel resize detection ---
// ResizeObserver tracks the rdpArea div; fires when sidebars change width.

let resizeObserver: ResizeObserver | null = null

function startResizeObserver() {
  if (!rdpAreaRef.value) return
  resizeObserver = new ResizeObserver(() => {
    syncRDPPosition()
  })
  resizeObserver.observe(rdpAreaRef.value)
}

function stopResizeObserver() {
  if (resizeObserver) { resizeObserver.disconnect(); resizeObserver = null }
}

// --- Connection ---

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
  window.addEventListener('resize', onWindowResize)
  startMovePolling()
  nextTick(() => {
    startResizeObserver()
  })
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
  stopMovePolling()
  stopResizeObserver()
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
