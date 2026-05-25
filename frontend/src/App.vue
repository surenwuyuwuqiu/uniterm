<template>
  <div class="app-container">
    <AppHeader
      @new-connection="showConnectionForm = true"
      @new-local-terminal="createLocalTerminal"
      @new-local-terminal-with-shell="createLocalTerminalWithShell"
      @toggle-ai="aiStore.toggle"
      @toggle-sidebar="sidebarVisible = !sidebarVisible"
      @open-settings="openSettings"
    />
    <div class="main-content">
      <Sidebar :visible="sidebarVisible" @toggle="sidebarVisible = !sidebarVisible" @connect="onConnect" @connect-sftp="onConnectSftp" @connect-rdp="onConnectRDP" @connect-vnc="onConnectVNC" />
      <div class="tab-area">
        <TabBar />
        <template v-if="activeTab">
          <TerminalTabContent
            v-if="activeTab.type === 'terminal'"
            :key="activeTab.id"
            :tab="activeTab"
            @close="closeTab"
          />
          <SettingsTabContent
            v-else-if="activeTab.type === 'settings'"
          />
          <WorkspaceContent
            v-else-if="activeTab.type === 'workspace'"
            :tab="activeTab"
          />
          <SFTPTabContent
            v-else-if="activeTab.type === 'sftp'"
            :key="activeTab.id"
            :panel-id="activeTab.panelId"
          />
          <RDPTabContent
            v-else-if="activeTab.type === 'rdp'"
            :key="activeTab.id"
            :panel-id="activeTab.panelId"
            :config="getPanelConfig(activeTab.panelId)"
            :session-id="getPanelSessionId(activeTab.panelId)"
          />
          <VNCTabContent
            v-else-if="activeTab.type === 'vnc'"
            :key="activeTab.id"
            :panel-id="activeTab.panelId"
            :config="getPanelConfig(activeTab.panelId)"
            :session-id="getPanelSessionId(activeTab.panelId)"
          />
        </template>
      </div>
      <AISidebar />
    </div>
    <ConnectionForm v-model="showConnectionForm" @save="onSaveOnly" @connect="onConnect" />

    <!-- Input context menu -->
    <div
      v-show="inputMenuVisible"
      class="input-context-menu"
      :style="{ left: inputMenuPos.x + 'px', top: inputMenuPos.y + 'px' }"
      @click.stop
    >
      <div class="input-menu-item" @click="inputMenuCut">{{ t('input.cut') }}</div>
      <div class="input-menu-item" @click="inputMenuCopy">{{ t('input.copy') }}</div>
      <div class="input-menu-item" @click="inputMenuPaste">{{ t('input.paste') }}</div>
      <div class="input-menu-item" @click="inputMenuSelectAll">{{ t('input.selectAll') }}</div>
    </div>

    <SyncConflictDialog />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import AppHeader from './components/AppHeader.vue'
import Sidebar from './components/Sidebar.vue'
import TabBar from './components/TabBar.vue'
import TerminalTabContent from './components/TerminalTabContent.vue'
import SettingsTabContent from './components/SettingsTabContent.vue'
import WorkspaceContent from './components/WorkspaceContent.vue'
import SFTPTabContent from './components/SFTPTabContent.vue'
import RDPTabContent from './components/RDPTabContent.vue'
import VNCTabContent from './components/VNCTabContent.vue'
import ConnectionForm from './components/ConnectionForm.vue'
import AISidebar from './components/AISidebar.vue'
import SyncConflictDialog from './components/SyncConflictDialog.vue'
import { useConnectionStore } from './stores/connectionStore'
import { useTabStore } from './stores/tabStore'
import { usePanelStore } from './stores/panelStore'
import { useSessionStore } from './stores/sessionStore'
import { useAIStore } from './stores/aiStore'
import { useSettingsStore } from './stores/settingsStore'
import { useI18n } from './i18n'
import { CreateSession, CloseSession, RDPHide, RDPShow, RDPSetPosition, RDPSetFocus } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime'
import type { ConnectionConfig } from './types/session'

const connectionStore = useConnectionStore()
const tabStore = useTabStore()
const activeTab = computed(() => tabStore.activeTab)
const panelStore = usePanelStore()
const sessionStore = useSessionStore()
const aiStore = useAIStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
// ── RDP position sync ──
// Called explicitly on tab switch and overlay restore; no polling needed.

function getActiveRdpSessionId(): string | null {
  const tab = activeTab.value
  if (!tab || tab.type !== 'rdp') return null
  return panelStore.getPanel(tab.panelId)?.sessionId ?? null
}

function rdpSyncPosition() {
  if (rdpOverlayCount.value > 0) return
  const area = document.querySelector('.rdp-area') as HTMLElement | null
  if (!area) return
  const sid = getActiveRdpSessionId()
  if (!sid) return
  const rect = area.getBoundingClientRect()
  if (rect.width <= 0) return
  const dpr = window.devicePixelRatio || 1
  const sx = window.screenLeft ?? (window as any).screenX ?? 0
  const sy = window.screenTop ?? (window as any).screenY ?? 0
  const x = Math.round((sx + rect.left) * dpr)
  const y = Math.round((sy + rect.top) * dpr)

  const w = Math.round(rect.width * dpr)
  const h = Math.round(rect.height * dpr)
  RDPSetPosition(sid, x, y, w, h)
}

function rdpResetTracking() {
  nextTick(() => rdpSyncPosition())
}


// ── RDP overlay tracking: unified show/hide entry points ──
// ALL triggers (context menus, dialogs, drag, resize, external events)
// MUST call RDPHideForOverlay() to hide and RDPShowForOverlay() to restore.
// Reference-counted: nesting works correctly across multiple concurrent triggers.
const rdpOverlayCount = ref(0)
let rdpRestoreTimer: ReturnType<typeof setTimeout> | null = null

function RDPHideForOverlay() {
	rdpOverlayCount.value++
	if (rdpOverlayCount.value === 1) {
		const sid = getActiveRdpSessionId()
		if (sid) RDPHide(sid)
	}
}

function RDPShowForOverlay() {
	if (rdpOverlayCount.value > 0) rdpOverlayCount.value--
	if (rdpRestoreTimer) clearTimeout(rdpRestoreTimer)
	rdpRestoreTimer = setTimeout(() => {
		rdpRestoreTimer = null
		if (rdpOverlayCount.value === 0) {
			const tab = activeTab.value
			if (!tab || tab.type !== 'rdp') return
			const sid = panelStore.getPanel(tab.panelId)?.sessionId
			if (sid) {
				rdpResetTracking()
				nextTick(() => RDPShow(sid))
			}
		}
	}, 150)
}


const showConnectionForm = ref(false)
const sidebarVisible = ref(localStorage.getItem('sidebarVisible') !== 'false')

// Input context menu state
const inputMenuVisible = ref(false)
const inputMenuPos = ref({ x: 0, y: 0 })
let inputMenuTarget: HTMLInputElement | HTMLTextAreaElement | null = null

function closeInputMenu() {
  inputMenuVisible.value = false
  inputMenuTarget = null
}

function onInputContextMenu(e: Event) {
  const { x, y, target } = (e as CustomEvent).detail as {
    x: number; y: number; target: HTMLInputElement | HTMLTextAreaElement
  }
  window.dispatchEvent(new CustomEvent('global:close-context-menus'))
  inputMenuTarget = target
  const pos = fitMenuPosition(x, y, 120, 140)
  inputMenuPos.value = { x: parseInt(pos.left), y: parseInt(pos.top) }
  inputMenuVisible.value = true
}

function fitMenuPosition(x: number, y: number, menuW: number, menuH: number) {
  let left = x
  let top = y
  if (x + menuW > window.innerWidth) left = x - menuW
  if (y + menuH > window.innerHeight) top = y - menuH
  return { left: left + 'px', top: top + 'px' }
}

function inputMenuCut() {
  if (inputMenuTarget) {
    navigator.clipboard.writeText(getInputSelection(inputMenuTarget))
    setInputSelection(inputMenuTarget, '')
    inputMenuTarget.dispatchEvent(new Event('input', { bubbles: true }))
  }
  closeInputMenu()
}

function inputMenuCopy() {
  if (inputMenuTarget) {
    navigator.clipboard.writeText(getInputSelection(inputMenuTarget))
  }
  closeInputMenu()
}

function inputMenuPaste() {
  if (inputMenuTarget) {
    navigator.clipboard.readText().then(text => {
      setInputSelection(inputMenuTarget, text)
      inputMenuTarget?.dispatchEvent(new Event('input', { bubbles: true }))
    }).catch(() => {})
  }
  closeInputMenu()
}

function inputMenuSelectAll() {
  inputMenuTarget?.select()
  closeInputMenu()
}

function getInputSelection(el: HTMLInputElement | HTMLTextAreaElement): string {
  return el.value.substring(el.selectionStart ?? 0, el.selectionEnd ?? 0)
}

function setInputSelection(el: HTMLInputElement | HTMLTextAreaElement, text: string) {
  const start = el.selectionStart ?? 0
  const end = el.selectionEnd ?? 0
  el.value = el.value.substring(0, start) + text + el.value.substring(end)
  const pos = start + text.length
  el.setSelectionRange(pos, pos)
  el.focus()
}

function onWheel(e: WheelEvent) {
  if (e.ctrlKey) {
    e.preventDefault()
  }
}

onMounted(() => {
  connectionStore.load()
  aiStore.init()
  settingsStore.init()
  // Pre-load noVNC so VNC tab switches don't pay the dynamic import cost.
  import('@novnc/novnc').then((m: any) => {
    ;(window as any).__novnc_RFB = m.default || m
  }).catch(() => {})
  window.addEventListener('input:contextmenu', onInputContextMenu)
  window.addEventListener('global:close-context-menus', closeInputMenu)
  document.addEventListener('click', closeInputMenu)
  document.addEventListener('wheel', onWheel, { passive: false })
  // RDP blur/focus: notify Go side so it can manage focus on the native RDP window
  window.addEventListener('blur', () => {
    const sid = getActiveRdpSessionId()
    if (sid) RDPSetFocus(sid, false)
  })
  window.addEventListener('focus', () => {
    const sid = getActiveRdpSessionId()
    if (sid) RDPSetFocus(sid, true)
  })
  // RDP overlay tracking
  window.addEventListener('rdp:overlay-push', RDPHideForOverlay)
  window.addEventListener('rdp:overlay-pop', RDPShowForOverlay)
  window.addEventListener('split:resize-start', RDPHideForOverlay)
  window.addEventListener('split:resize-end', RDPShowForOverlay)
  window.addEventListener('rdp:sync-position', rdpResetTracking)
  // Go-side WndProc events: window move/resize start/end
  EventsOn('rdp:move-resize-start', () => RDPHideForOverlay())
  EventsOn('rdp:move-resize-end', () => RDPShowForOverlay())

})

onUnmounted(() => {
  window.removeEventListener('input:contextmenu', onInputContextMenu)
  window.removeEventListener('global:close-context-menus', closeInputMenu)
  document.removeEventListener('click', closeInputMenu)
  document.removeEventListener('wheel', onWheel)
  // RDP overlay tracking
  window.removeEventListener('rdp:overlay-push', RDPHideForOverlay)
  window.removeEventListener('rdp:overlay-pop', RDPShowForOverlay)
  window.removeEventListener('split:resize-start', RDPHideForOverlay)
  window.removeEventListener('split:resize-end', RDPShowForOverlay)
  window.removeEventListener('rdp:sync-position', rdpResetTracking)

})

function openSettings() {
  // Check if settings tab already exists
  const existingTab = tabStore.tabs.find(t => t.type === 'settings')
  if (existingTab) {
    tabStore.setActiveTab(existingTab.id)
    return
  }

  const panel = panelStore.createPanel(null, 'settings')
  panel.title = t('settings.title')
  const tab = tabStore.createSettingsTab(t('settings.title'), panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)
}

async function closeTab(tabId: string) {
  // Close session before removing panel to clean up Go-side resources
  const tab = tabStore.tabs.find(t => t.id === tabId)
  if (tab && tab.type === 'rdp') {
    const p = panelStore.getPanel(tab.panelId)
    if (p?.sessionId) {
      try { await CloseSession(p.sessionId) } catch (_) {}
    }
  }
  // Close VNC session
  if (tab && tab.type === 'vnc') {
    const p = panelStore.getPanel(tab.panelId)
    if (p?.sessionId) {
      try { await CloseSession(p.sessionId) } catch (_) {}
    }
    panelStore.disconnectVNCCache(tab.panelId)
    panelStore.removeVNCCache(tab.panelId)
  }
  // Local terminals must be explicitly closed to terminate the shell process
  if (tab && tab.type === 'terminal') {
    const p = panelStore.getPanel(tab.panelId)
    if (p?.type === 'local' && p?.sessionId) {
      try { await CloseSession(p.sessionId) } catch (_) {}
    }
  }
  const panelIds = tabStore.closeTab(tabId)
  panelIds.forEach(pid => panelStore.removePanel(pid))
}

function getPanelConfig(panelId: string): ConnectionConfig | null {
  return panelStore.getPanel(panelId)?.config || null
}

function getPanelSessionId(panelId: string): string | null {
  return panelStore.getPanel(panelId)?.sessionId || null
}

function onSaveOnly(config: ConnectionConfig) {
  connectionStore.add(config)
}

async function onConnect(config: ConnectionConfig) {
  if (config.type === 'rdp') return onConnectRDP(config)
  if (config.type === 'vnc') return onConnectVNC(config)
  connectionStore.add(config)
  const panel = panelStore.createPanel(config, 'ssh')
  const displayTitle = config.name || `${config.user}@${config.host}`
  panel.title = displayTitle
  const tab = tabStore.createTerminalTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession(config.type, config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}

function getShellLabel(path: string): string {
  if (!path) return 'Local'
  const lower = path.toLowerCase()
  if (lower.includes('pwsh')) return 'PowerShell'
  if (lower.includes('powershell')) return 'Windows PowerShell'
  if (lower.includes('bash')) return 'Git Bash'
  if (lower.includes('cmd')) return 'Command Prompt'
  return path.replace(/\\/g, '/').split('/').pop() || 'Local'
}

async function createLocalTerminalWithShell(shellPath: string) {
  await createLocalTerminal(shellPath)
}

async function createLocalTerminal(shellPath?: string) {
  const panel = panelStore.createPanel(null, 'local')
  const shellName = getShellLabel(shellPath)
  panel.title = shellName
  const tab = tabStore.createTerminalTab(shellName, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const config: ConnectionConfig = {
      id: '',
      name: shellName,
      type: 'local' as any,
      host: '',
      port: 0,
      user: '',
      authType: 'password' as any,
      shellPath: shellPath || undefined
    }
    panel.config = config
    const info = await CreateSession('local', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create local terminal:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}

async function onConnectSftp(config: ConnectionConfig) {
  const panel = panelStore.createPanel(config, 'sftp')
  const displayTitle = config.name || `${config.user}@${config.host}`
  panel.title = displayTitle
  const tab = tabStore.createSFPTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('sftp', config)
    panelStore.bindSession(panel.id, info.id)
  } catch (e) {
    console.error('Failed to create SFTP session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}

async function onConnectRDP(config: ConnectionConfig) {
  connectionStore.add(config)

  const displayTitle = config.name || `${config.user}@${config.host}`

  const panel = panelStore.createPanel(config, 'rdp')
  panel.title = displayTitle
  const tab = tabStore.createRDPTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('rdp', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create RDP session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}

async function onConnectVNC(config: ConnectionConfig) {
  connectionStore.add(config)

  const displayTitle = config.name || config.host

  const panel = panelStore.createPanel(config, 'vnc')
  panel.title = displayTitle
  const tab = tabStore.createVNCTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('vnc', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create VNC session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}

// Show/hide native RDP window on tab switch.
// Position updates are only sent to the active RDP session (see rdpSyncPosition),
// so background sessions stay at (32000,32000) and don't respond to drag.
watch(() => activeTab.value, (newTab, oldTab) => {
  if (oldTab?.type === 'rdp') {
    const p = panelStore.getPanel(oldTab.panelId)
    if (p?.sessionId) RDPHide(p.sessionId)
  }
  // Clear pending restore timer on tab switch
  if (rdpRestoreTimer) { clearTimeout(rdpRestoreTimer); rdpRestoreTimer = null }
  if (newTab?.type === 'rdp') {
    rdpResetTracking()
    const sid = panelStore.getPanel(newTab.panelId)?.sessionId
    if (sid) nextTick(() => RDPShow(sid))
  }
})

// Hide RDP when new-connection dialog opens (App.vue's ConnectionForm)
watch(showConnectionForm, (val) => {
  if (val) RDPHideForOverlay()
  else RDPShowForOverlay()
})

watch(sidebarVisible, () => {
  localStorage.setItem('sidebarVisible', String(sidebarVisible.value))
  RDPHideForOverlay()
  nextTick(() => RDPShowForOverlay())
})

watch(() => aiStore.visible, () => {
  RDPHideForOverlay()
  nextTick(() => RDPShowForOverlay())
})
</script>

<style scoped>
.app-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: var(--bg-base);
}

.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
  gap: 0;
  position: relative;
}

.tab-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg-base);
}

.input-context-menu {
  position: fixed;
  z-index: 9999;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  min-width: 120px;
  padding: 4px;
  backdrop-filter: blur(8px);
}

.input-menu-item {
  padding: 7px 14px;
  font-size: 12px;
  font-family: var(--font-ui);
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
  border-radius: var(--radius-sm);
  transition: all 0.1s ease;
}

.input-menu-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
</style>
