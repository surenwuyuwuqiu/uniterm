import { defineStore } from 'pinia'
import { reactive } from 'vue'
import type { Panel, PanelStatus, ConnectionConfig } from '../types/workspace'

export interface TransferTaskUI {
  id: string
  type: 'upload' | 'download'
  name: string
  percentage: number
  speed: string
  eta: string
  status: 'running' | 'paused' | 'done' | 'error' | 'cancelled'
  lastBytes: number
  lastTime: number
  total: number
}

const panelState = reactive<{
  panels: Map<string, Panel>
  transferTasks: Map<string, TransferTaskUI[]>
  proxyAddrs: Map<string, string>
}>({
  panels: new Map(),
  transferTasks: new Map(),
  proxyAddrs: new Map()
})

export const usePanelStore = defineStore('panel', () => {
  function createPanel(config: ConnectionConfig | null, type: Panel['type'] = 'ssh'): Panel {
    const id = `panel-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const panel: Panel = {
      id,
      tabId: '',
      type,
      sessionId: null,
      title: config ? `${config.host} ${config.user}` : 'New Panel',
      status: 'disconnected',
      config
    }
    panelState.panels.set(id, panel)
    return panel
  }

  function removePanel(id: string) {
    panelState.panels.delete(id)
  }

  function getPanel(id: string): Panel | undefined {
    return panelState.panels.get(id)
  }

  function bindSession(panelId: string, sessionId: string) {
    const p = panelState.panels.get(panelId)
    if (p) p.sessionId = sessionId
  }

  function updateStatus(panelId: string, status: PanelStatus) {
    const p = panelState.panels.get(panelId)
    if (p) p.status = status
  }

  function updateTitle(panelId: string, title: string) {
    const p = panelState.panels.get(panelId)
    if (p) p.title = title
  }

  function movePanelToTab(panelId: string, tabId: string) {
    const p = panelState.panels.get(panelId)
    if (p) p.tabId = tabId
  }

  function getTransferTasks(panelId: string): TransferTaskUI[] {
    if (!panelState.transferTasks.has(panelId)) {
      panelState.transferTasks.set(panelId, [])
    }
    return panelState.transferTasks.get(panelId)!
  }

  function setProxyAddr(panelId: string, addr: string) {
    panelState.proxyAddrs.set(panelId, addr)
  }

  function getProxyAddr(panelId: string): string | undefined {
    return panelState.proxyAddrs.get(panelId)
  }

  function removeProxyAddr(panelId: string) {
    panelState.proxyAddrs.delete(panelId)
  }

  return {
    panels: panelState.panels,
    transferTasks: panelState.transferTasks,
    proxyAddrs: panelState.proxyAddrs,
    getTransferTasks,
    createPanel,
    removePanel,
    getPanel,
    bindSession,
    updateStatus,
    updateTitle,
    movePanelToTab,
    setProxyAddr,
    getProxyAddr,
    removeProxyAddr
  }
})
