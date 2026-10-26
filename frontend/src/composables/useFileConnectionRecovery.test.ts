import { describe, it, expect, vi } from 'vitest'
import { reactive } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@wailsio/runtime', () => ({ Events: { On: vi.fn(() => () => {}) } }))
vi.mock('../../bindings/github.com/ys-ll/uniterm/app', () => ({ SaveConnections: vi.fn(), LoadConnections: vi.fn() }))
import { SaveConnections } from '../../bindings/github.com/ys-ll/uniterm/app'
import { useConnectionStore } from '../stores/connectionStore'
import { useSessionStore } from '../stores/sessionStore'
import { useFileConnectionRecovery } from './useFileConnectionRecovery'
import type { ConnectionConfig } from '../types/session'

const config: ConnectionConfig = { id: 'router', name: 'router', type: 'ssh', host: 'router', port: 22, user: 'root', authType: 'password' }
function setup(error = 'sftp client: unitermerr:sftp_unavailable|unexpected EOF|missing server') {
  setActivePinia(createPinia())
  const connections = useConnectionStore()
  connections.connections = [{ ...config }]
  const sessions = useSessionStore()
  sessions.removeSession('sftp-failed')
  sessions.removeSession('scp-retry')
  sessions.initSession('sftp-failed')
  sessions.sessions.get('sftp-failed')!.errorMessage = error
  const state = reactive({ sessionId: 'sftp-failed', config: { ...config }, retryScp: async () => {} })
  state.retryScp = vi.fn(async () => {
    state.config = { ...state.config, fileTransferProto: 'scp' }
    state.sessionId = 'scp-retry'
    sessions.initSession(state.sessionId)
  })
  return { ...useFileConnectionRecovery(state), state, sessions, connections }
}

describe('SFTP startup recovery', () => {
  it('retries only on click and remembers only after SCP connects', async () => {
    vi.mocked(SaveConnections).mockReset().mockResolvedValue(undefined)
    const view = setup()
    expect(view.state.retryScp).not.toHaveBeenCalled()
    expect(view.canRetry.value).toBe(true)
    await view.retry()
    expect(view.state.retryScp).toHaveBeenCalledTimes(1)
    expect(view.canRetry.value).toBe(false)
    await view.remember()
    expect(SaveConnections).not.toHaveBeenCalled()
    expect(view.connections.connections[0].fileTransferProto).toBeUndefined()
    view.sessions.updateStatus('scp-retry', 'connected')
    expect(view.connected.value).toBe(true)
    await view.remember()
    expect(SaveConnections).toHaveBeenCalledTimes(1)
    expect(view.connections.connections[0].fileTransferProto).toBe('scp')
    expect(view.connections.connections[0].type).toBe('ssh')
    expect(view.remembered.value).toBe(true)
  })

  it('offers no protocol switch on an authentication error', async () => {
    const view = setup('ssh dial: unable to authenticate')
    expect(view.canRetry.value).toBe(false)
    await view.retry()
    expect(view.state.retryScp).not.toHaveBeenCalled()
  })

  it('remembers a standalone SFTP connection as SCP without changing credentials', async () => {
    vi.mocked(SaveConnections).mockReset().mockResolvedValue(undefined)
    const view = setup()
    view.state.config.type = 'sftp'
    view.connections.connections[0].type = 'sftp'
    view.state.retryScp = async () => {
      view.state.config = { ...view.state.config, type: 'scp' }
      view.state.sessionId = 'scp-retry'
      view.sessions.initSession('scp-retry')
      view.sessions.updateStatus('scp-retry', 'connected')
    }
    await view.retry()
    await view.remember()
    expect(view.connections.connections[0]).toEqual({ ...config, type: 'scp' })
    expect(view.remembered.value).toBe(true)
  })

  it('does not claim to remember a preference when saving fails', async () => {
    vi.mocked(SaveConnections).mockReset().mockRejectedValue(new Error('save failed'))
    const view = setup()
    await view.retry()
    view.sessions.updateStatus('scp-retry', 'connected')
    await view.remember()
    expect(view.error.value).toBe('save failed')
    expect(view.connections.connections[0].fileTransferProto).toBeUndefined()
    expect(view.remembered.value).toBe(false)
  })
})
