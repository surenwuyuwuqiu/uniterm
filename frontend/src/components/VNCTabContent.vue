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
    <div v-if="status === 'connected'" class="vnc-statusbar">
      <span class="vnc-status-dot" />
      <span>{{ t('vnc.connected') }}</span>
      <span class="vnc-status-sep">|</span>
      <span>{{ config?.host }}:{{ config?.port || 5900 }}</span>
    </div>

    <!-- Debug log panel -->
    <div v-if="debugLogs.length > 0" class="vnc-debug-panel">
      <div class="vnc-debug-title">诊断日志 (点击复制)</div>
      <pre class="vnc-debug-content" @click="copyDebugLogs">{{ debugLogs.join('\n') }}</pre>
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
const debugLogs = ref<string[]>([])

let rfb: any = null
let unsubStatus: (() => void) | null = null

function addDebug(msg: string) {
  const line = `[${new Date().toLocaleTimeString()}] ${msg}`
  console.log(msg)
  debugLogs.value.push(line)
  if (debugLogs.value.length > 50) debugLogs.value.shift()
}

function copyDebugLogs() {
  navigator.clipboard.writeText(debugLogs.value.join('\n')).catch(() => {})
}

async function connect() {
  if (!props.config) return
  status.value = 'connecting'
  addDebug('Calling CreateSession...')
  try {
    const info = await CreateSession('vnc', props.config)
    currentSessionId.value = info.id
    addDebug(`CreateSession returned, sessionId=${info.id}`)
  } catch (e: any) {
    addDebug(`CreateSession failed: ${e}`)
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

function initRFB(proxyAddr: string, password: string) {
  if (!vncContainer.value) {
    addDebug('vncContainer is null, cannot init RFB')
    return
  }

  addDebug('Loading noVNC module...')
  import('@novnc/novnc').then((module: any) => {
    addDebug(`noVNC module loaded, keys=${Object.keys(module).join(',')}`)
    const RFB = module.default || module
    addDebug(`RFB constructor type=${typeof RFB}, name=${RFB?.name}`)

    try {
      rfb = new RFB(vncContainer.value, proxyAddr, {
        credentials: { password: password || '' }
      })
      addDebug('RFB instance created successfully')
    } catch (e: any) {
      addDebug(`Failed to create RFB instance: ${e}`)
      status.value = 'error'
      return
    }

    rfb.addEventListener('connect', () => {
      addDebug('RFB connected event fired')
    })

    rfb.addEventListener('disconnect', (e: any) => {
      addDebug(`RFB disconnect event: clean=${e.detail?.clean}`)
      if (!e.detail.clean) {
        status.value = 'error'
      }
    })

    rfb.addEventListener('credentialsrequired', (e: any) => {
      addDebug(`RFB credentialsrequired: ${JSON.stringify(e.detail)}`)
      status.value = 'error'
    })

    rfb.addEventListener('securityfailure', (e: any) => {
      addDebug(`RFB securityfailure: ${JSON.stringify(e.detail)}`)
      status.value = 'error'
    })

    rfb.addEventListener('desktopname', (e: any) => {
      addDebug(`RFB desktopname: ${e.detail?.name}`)
    })

    rfb.addEventListener('bell', () => {
      addDebug('RFB bell')
    })

    rfb.addEventListener('clipboard', (e: any) => {
      const text = e.detail.text
      addDebug(`RFB clipboard received, length=${text?.length}`)
      navigator.clipboard.writeText(text).catch(() => {})
    })
  }).catch((e: any) => {
    addDebug(`Failed to load noVNC module: ${e}`)
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
  } else {
    connect()
  }

  unsubStatus = EventsOn('session:status', (data: any) => {
    addDebug(`session:status id=${data.id} status=${data.status} proxyAddr=${data.proxyAddr}`)
    if (data.id !== currentSessionId.value) return
    switch (data.status) {
      case 'connected':
        status.value = 'connected'
        if (data.proxyAddr && props.config?.password !== undefined) {
          addDebug(`Initializing RFB with proxyAddr=${data.proxyAddr}`)
          initRFB(data.proxyAddr, props.config.password)
        } else {
          addDebug(`Skip initRFB: proxyAddr=${data.proxyAddr}, password=${props.config?.password}`)
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
</script>

<style scoped>
.vnc-tab-content {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: #000;
  position: relative;
}
.vnc-area {
  flex: 1;
  background: #000;
  outline: none;
}
.vnc-area :deep(canvas) {
  display: block;
  width: 100%;
  height: 100%;
}
.vnc-overlay {
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
.vnc-error-text { color: #f56c6c; }
.vnc-statusbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: #1e1e1e;
  color: #999;
  font-size: 12px;
  flex-shrink: 0;
}
.vnc-status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: #67c23a;
}
.vnc-status-sep { color: #444; }

.vnc-debug-panel {
  position: absolute;
  bottom: 32px;
  left: 8px;
  right: 8px;
  max-height: 200px;
  background: rgba(0, 0, 0, 0.85);
  border: 1px solid #444;
  border-radius: 6px;
  z-index: 20;
  display: flex;
  flex-direction: column;
}
.vnc-debug-title {
  padding: 6px 10px;
  font-size: 11px;
  color: #aaa;
  background: rgba(40, 40, 40, 0.9);
  border-bottom: 1px solid #444;
  cursor: pointer;
  user-select: none;
}
.vnc-debug-content {
  padding: 8px 10px;
  margin: 0;
  font-size: 11px;
  font-family: 'Consolas', 'Courier New', monospace;
  color: #ccc;
  line-height: 1.5;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
