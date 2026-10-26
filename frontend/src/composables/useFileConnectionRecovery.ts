import { computed, ref } from 'vue'
import { useI18n } from '../i18n'
import { useSessionStore } from '../stores/sessionStore'
import { useConnectionStore } from '../stores/connectionStore'
import { canRetryWithScp } from '../utils/fileTransferUtils'
import type { ConnectionConfig } from '../types/session'

export function useFileConnectionRecovery(props: {
  sessionId?: string | null
  config?: ConnectionConfig | null
  retryScp: () => Promise<unknown>
}) {
  const { t } = useI18n()
  const sessions = useSessionStore()
  const connections = useConnectionStore()
  const busy = ref(false)
  const switched = ref(false)
  const remembered = ref(false)
  const localError = ref('')
  const error = computed(() => localError.value || (props.sessionId ? sessions.sessions.get(props.sessionId)?.errorMessage : '') || '')
  const connected = computed(() => !!props.sessionId && sessions.getStatus(props.sessionId) === 'connected')
  const canRetry = computed(() => canRetryWithScp(props.config, error.value))
  const savedConnection = computed(() => connections.connections.find(c => c.id === props.config?.id && ['ssh', 'sftp', 'scp'].includes(c.type)))

  async function retry() {
    if (busy.value || !canRetry.value) return
    busy.value = true
    localError.value = ''
    switched.value = true
    try {
      await props.retryScp()
    } catch (e) {
      localError.value = e instanceof Error ? e.message : String(e)
    } finally {
      busy.value = false
    }
  }

  async function remember() {
    const saved = savedConnection.value
    if (!saved || !switched.value || !connected.value) return
    try {
      await connections.rememberScp(saved.id)
      remembered.value = true
    } catch (e) {
      localError.value = e instanceof Error ? e.message : String(e)
    }
  }

  return { t, busy, error, switched, connected, canRetry, savedConnection, remembered, retry, remember }
}
