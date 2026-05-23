<template>
  <div class="vnc-tab-content">
    <!-- Connecting state -->
    <div v-if="status === 'connecting'" class="vnc-overlay">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>{{ t('vnc.connecting', { host: config?.host || '...' }) }}</p>
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="vnc-overlay">
      <p class="vnc-error-text">{{ t('vnc.error') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('vnc.retry') }}</el-button>
    </div>

    <!-- Disconnected state -->
    <div v-else-if="status === 'disconnected'" class="vnc-overlay">
      <p>{{ t('vnc.disconnected') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('vnc.reconnect') }}</el-button>
    </div>

    <!-- Connected: noVNC Canvas mounts here -->
    <div
      v-show="status === 'connected'"
      ref="vncContainer"
      class="vnc-area"
      tabindex="0"
      @paste="onPaste"
    />

    <!-- Status bar -->
    <div v-show="status === 'connected'" class="vnc-statusbar">
      <span class="vnc-status-dot" />
      <span>{{ t('vnc.connected') }}</span>
      <span class="vnc-status-sep">|</span>
      <span>{{ config?.host }}:{{ config?.port || 5900 }}</span>
      <span class="vnc-status-sep">|</span>
      <span class="vnc-zoom-label">缩放</span>
      <el-select v-model="zoomMode" size="small" style="width: 90px">
        <el-option v-for="opt in zoomOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'
import { CreateSession, CloseSession } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const { t } = useI18n()

const props = defineProps<{
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null
}>()

const status = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
const currentSessionId = ref<string | null>(props.sessionId)
const vncContainer = ref<HTMLDivElement | null>(null)
const savedProxyAddr = ref<string>('')
const savedPassword = ref<string>('')
const zoomMode = ref<'auto' | number>('auto')
const zoomOptions: { label: string; value: 'auto' | number }[] = [
  { label: '自动缩放', value: 'auto' },
  { label: '50%', value: 50 },
  { label: '75%', value: 75 },
  { label: '100%', value: 100 },
  { label: '125%', value: 125 },
  { label: '150%', value: 150 },
]

let rfb: any = null
let unsubStatus: (() => void) | null = null
let resizeHandler: (() => void) | null = null

async function connect() {
  if (!props.config) return
  status.value = 'connecting'
  try {
    const info = await CreateSession('vnc', props.config)
    currentSessionId.value = info.id
  } catch (e: any) {
    console.error('VNC connect error:', e)
    status.value = 'error'
  }
}

async function reconnect() {
  if (currentSessionId.value) {
    try { await CloseSession(currentSessionId.value) } catch (_) {}
    currentSessionId.value = null
  }
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  await connect()
}

function applyZoom() {
  if (!rfb) return

  rfb.scaleViewport = false
  rfb.clipViewport = true

  if (zoomMode.value === 'auto') {
    const size = rfb._screenSize()
    rfb._display.autoscale(size.w, size.h)
  } else {
    rfb._display.scale = Number(zoomMode.value) / 100
  }
}

function initRFB(proxyAddr: string, password: string) {
  if (!vncContainer.value) return

  import('@novnc/novnc').then((module: any) => {
    const RFB = module.default || module
    try {
      rfb = new RFB(vncContainer.value, proxyAddr, {
        credentials: { password: password || '' }
      })
    } catch (e: any) {
      console.error('Failed to create RFB instance:', e)
      status.value = 'error'
      return
    }

    rfb.addEventListener('connect', () => {
      applyZoom()
      if (!resizeHandler) {
        resizeHandler = () => {
          if (zoomMode.value !== 'auto') {
            applyZoom()
          }
        }
        window.addEventListener('resize', resizeHandler)
      }
    })

    rfb.addEventListener('disconnect', (e: any) => {
      if (!e.detail.clean) {
        status.value = 'error'
      }
    })

    rfb.addEventListener('credentialsrequired', () => {
      status.value = 'error'
    })

    rfb.addEventListener('securityfailure', () => {
      status.value = 'error'
    })

    rfb.addEventListener('clipboard', (e: any) => {
      const text = e.detail.text
      navigator.clipboard.writeText(text).catch(() => {})
    })
  }).catch((e: any) => {
    console.error('Failed to load noVNC module:', e)
    status.value = 'error'
  })
}

function onPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text')
  if (text && rfb) {
    rfb.clipboardPasteFrom(text)
  }
}

onMounted(() => {
  if (props.sessionId) {
    currentSessionId.value = props.sessionId
  }
  if (currentSessionId.value) {
    status.value = 'connected'
    if (savedProxyAddr.value) {
      initRFB(savedProxyAddr.value, savedPassword.value)
    }
  } else {
    connect()
  }

  unsubStatus = EventsOn('session:status', (data: any) => {
    if (data.id !== currentSessionId.value) return
    switch (data.status) {
      case 'connected':
        status.value = 'connected'
        if (data.proxyAddr) {
          savedProxyAddr.value = data.proxyAddr
        }
        if (props.config) {
          savedPassword.value = props.config.password || ''
        }
        if (data.proxyAddr && props.config) {
          initRFB(data.proxyAddr, props.config.password || '')
        } else if (savedProxyAddr.value) {
          initRFB(savedProxyAddr.value, savedPassword.value)
        }
        break
      case 'disconnected':
        if (status.value !== 'error') status.value = 'disconnected'
        break
      case 'error':
        status.value = 'error'
        break
    }
  })
})

onUnmounted(() => {
  unsubStatus?.()
  if (resizeHandler) {
    window.removeEventListener('resize', resizeHandler)
    resizeHandler = null
  }
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  if (currentSessionId.value) {
    CloseSession(currentSessionId.value).catch(() => {})
  }
})

watch(() => props.sessionId, (newId) => {
  if (newId && !currentSessionId.value) {
    currentSessionId.value = newId
  }
})

watch(zoomMode, () => {
  if (status.value === 'connected') {
    applyZoom()
  }
})
</script>

<style scoped>
.vnc-tab-content {
  position: relative;
  width: 100%;
  height: 100%;
  background: #000;
}
.vnc-area {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 24px;
  background: #000;
  outline: none;
  overflow: hidden;
}
.vnc-area :deep(canvas) {
  display: block;
  image-rendering: pixelated;
}
.vnc-overlay {
  position: absolute;
  inset: 0;
  bottom: 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #999;
  z-index: 10;
}
.vnc-error-text { color: #f56c6c; }
.vnc-statusbar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 24px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  background: #1e1e1e;
  color: #999;
  font-size: 12px;
  box-sizing: border-box;
  z-index: 5;
}
.vnc-status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: #67c23a;
  flex-shrink: 0;
}
.vnc-status-sep { color: #444; }
.vnc-zoom-label {
  margin-left: auto;
  font-size: 11px;
}
.vnc-statusbar :deep(.el-select .el-input__wrapper) {
  padding: 0 4px;
  font-size: 11px;
}
.vnc-statusbar :deep(.el-select .el-input__inner) {
  height: 18px;
  line-height: 18px;
}
</style>
