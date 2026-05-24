import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  SyncGetConfig,
  SyncSaveConfig,
  SyncNow,
  SyncResolveConflict,
  SyncTestConnection,
  SyncGetLastSyncTime,
} from '../../wailsjs/go/main/App'
import { sync } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime'

export interface SyncConfig {
  repoUrl: string
  branch: string
  authType: 'ssh' | 'token'
  autoSync: boolean
  lastSyncAt: string
}

export interface SyncResult {
  direction: number // 0=none, 1=push, 2=pull, 3=conflict
  message: string
  conflict?: SyncConflict
}

export interface SyncConflict {
  localTime: string
  remoteTime: string
}

export const useSyncStore = defineStore('sync', () => {
  const config = ref<SyncConfig>({
    repoUrl: '',
    branch: 'main',
    authType: 'ssh',
    autoSync: false,
    lastSyncAt: '',
  })
  const lastSyncTime = ref('从未同步')
  const syncing = ref(false)
  const testingConnection = ref(false)
  const conflict = ref<SyncConflict | null>(null)
  const lastResult = ref('')

  async function loadConfig() {
    try {
      const cfg = await SyncGetConfig()
      config.value = {
        repoUrl: cfg.repoUrl || '',
        branch: cfg.branch || 'main',
        authType: (cfg.authType as 'ssh' | 'token') || 'ssh',
        autoSync: cfg.autoSync || false,
        lastSyncAt: cfg.lastSyncAt || '',
      }
    } catch (e) {
      console.error('Load sync config failed:', e)
    }
    try {
      lastSyncTime.value = await SyncGetLastSyncTime()
    } catch (_) {}
  }

  async function saveConfig(token: string = '') {
    try {
      const cfg = new sync.SyncConfig()
      cfg.repoUrl = config.value.repoUrl
      cfg.branch = config.value.branch
      cfg.authType = config.value.authType
      cfg.autoSync = config.value.autoSync
      cfg.lastSyncAt = config.value.lastSyncAt
      await SyncSaveConfig(cfg, token)
    } catch (e) {
      console.error('Save sync config failed:', e)
      throw e
    }
  }

  async function doSync(): Promise<SyncResult | null> {
    syncing.value = true
    try {
      const result = await SyncNow()
      // Wails returns Go struct with PascalCase fields (no json tags on SyncResult)
      if (result.Direction === 3) {
        conflict.value = result.Conflict
          ? { localTime: result.Conflict.LocalTime, remoteTime: result.Conflict.RemoteTime }
          : null
      } else {
        conflict.value = null
      }
      lastResult.value = result.Message || ''
      await updateLastSyncTime()
      return {
        direction: result.Direction,
        message: result.Message || '',
        conflict: result.Conflict
          ? { localTime: result.Conflict.LocalTime, remoteTime: result.Conflict.RemoteTime }
          : undefined,
      }
    } catch (e: any) {
      lastResult.value = e?.message || String(e)
      return null
    } finally {
      syncing.value = false
    }
  }

  async function resolveConflict(useLocal: boolean): Promise<SyncResult | null> {
    syncing.value = true
    try {
      const result = await SyncResolveConflict(useLocal)
      conflict.value = null
      lastResult.value = result.Message || ''
      await updateLastSyncTime()
      return {
        direction: result.Direction,
        message: result.Message || '',
      }
    } catch (e: any) {
      lastResult.value = e?.message || String(e)
      return null
    } finally {
      syncing.value = false
    }
  }

  async function testConnection(): Promise<string | null> {
    testingConnection.value = true
    try {
      await SyncTestConnection()
      return null
    } catch (e: any) {
      return e?.message || String(e)
    } finally {
      testingConnection.value = false
    }
  }

  async function updateLastSyncTime() {
    try {
      lastSyncTime.value = await SyncGetLastSyncTime()
    } catch (_) {}
  }

  // Listen for conflict events from auto-sync
  EventsOn('sync:conflict', (data: SyncConflict) => {
    conflict.value = data
  })

  return {
    config,
    lastSyncTime,
    syncing,
    testingConnection,
    conflict,
    lastResult,
    loadConfig,
    saveConfig,
    doSync,
    resolveConflict,
    testConnection,
  }
})
