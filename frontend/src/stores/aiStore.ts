import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import type { AIMessage, AIConfig, ExecutionMode, AISession } from '../types/ai'
import { SaveAIConfig, LoadAIConfig, SaveAISessions, LoadAISessions } from '../../wailsjs/go/main/App'
import { t } from '../i18n'

const SYSTEM_PROMPT = `You are an AI assistant inside uniTerm, a terminal emulator. You can execute shell commands in the user's active terminal to help them complete tasks.

When you need to run a command, use the execute_command tool. The command will be executed in the active terminal session and you will receive its stdout/stderr output.

CRITICAL RULES:
- You can only send ONE execute_command tool call at a time. Never send multiple tool calls in a single response.
- Always explain what you are about to do before executing commands.
- Prefer using standard Unix tools (ls, cat, grep, find, etc.).
- For file editing, use sed, awk, or echo with redirection.
- If a command might be destructive, warn the user.
- Chain multiple commands with && or ; when appropriate.
- If the output is too long, summarize the key findings.
- Commands have a 60-second timeout. If a command times out, you will see "[Command timed out after 60s...]". In that case, you can either wait (the command may still be running) or suggest canceling it with Ctrl+C.
- Do NOT send a new command if the previous one might still be running, unless you intend to cancel it first.

RISK CLASSIFICATION:
Every execute_command call MUST include a "risk" field. Classify each command honestly:
- "read": only inspects/views data, no modifications at all (ls, cat, grep, head, tail, df, du, ps, top, find, pwd, whoami, git status/log/diff, docker ps/images/logs, npm list, pip list, go version/env, etc.)
- "write": modifies or creates data, but not system-destructive (echo > file, touch, mkdir, cp, mv, git add/commit/push, curl POST, npm install, pip install, apt install, brew install, etc.)
- "dangerous": potentially destructive or system-altering (rm, > overwrite important files, chmod, chown, shutdown, reboot, mkfs, dd, force push, kill -9, etc.)

For chained commands with && or ;, classify based on the MOST risky operation in the chain.`

const DEFAULT_CONFIG: AIConfig = {
  apiKey: '',
  baseURL: 'https://api.openai.com/v1',
  model: 'gpt-4o'
}

async function loadSessionsFromBackend(): Promise<{ sessions: AISession[], currentSessionId: string | null }> {
  try {
    const data = await LoadAISessions() as any
    const sessions: AISession[] = (data.sessions || []).map((s: any) => ({
      id: s.id,
      name: s.name,
      createdAt: s.createdAt,
      updatedAt: s.updatedAt,
      messages: (s.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        tool_call_id: m.tool_call_id,
        _rawApiMsg: m._rawApiMsg ? JSON.parse(m._rawApiMsg) : undefined,
      }))
    }))
    return { sessions, currentSessionId: data.currentSessionId || null }
  } catch {
    return { sessions: [], currentSessionId: null }
  }
}

export const useAIStore = defineStore('ai', () => {
  const visible = ref(true)
  const messages = ref<AIMessage[]>([])
  const mode = ref<ExecutionMode>('confirm_dangerous')
  const config = ref<AIConfig>({ ...DEFAULT_CONFIG })
  const isRunning = ref(false)
  const stopRequested = ref(false)
  const sessions = ref<AISession[]>([])
  const currentSessionId = ref<string | null>(null)
  const lastDebugInfo = ref<{ request: string; error: string } | null>(null)
  const initialized = ref(false)
  const pendingCommand = ref<{
    messageId: string
    toolId: string
    command: string
    risk: string
    dangerous: boolean
  } | null>(null)

  function setDebugInfo(request: unknown, error: string) {
    try {
      lastDebugInfo.value = {
        request: JSON.stringify(request, null, 2),
        error
      }
    } catch {
      lastDebugInfo.value = {
        request: String(request),
        error
      }
    }
  }

  function clearDebugInfo() {
    lastDebugInfo.value = null
  }

  function setPendingCommand(cmd: { messageId: string; toolId: string; command: string; risk: string; dangerous: boolean }) {
    pendingCommand.value = cmd
  }

  function clearPendingCommand() {
    pendingCommand.value = null
  }

  function toggle() {
    visible.value = !visible.value
  }

  function addMessage(msg: AIMessage): AIMessage {
    const r = reactive({ ...msg }) as AIMessage
    messages.value.push(r)
    if (currentSessionId.value) {
      const s = sessions.value.find(s => s.id === currentSessionId.value)
      if (s) {
        s.messages.push(r)
        s.updatedAt = Date.now()
        if (msg.role === 'user' && s.name === t('ai.newSession')) {
          const trimmed = msg.content.trim()
          if (trimmed) {
            s.name = trimmed.length > 20 ? trimmed.slice(0, 20) + '...' : trimmed
          }
        }
        scheduleSave()
      }
    }
    return r
  }

  function clearMessages() {
    messages.value = []
    if (currentSessionId.value) {
      const s = sessions.value.find(s => s.id === currentSessionId.value)
      if (s) {
        s.messages = []
        s.updatedAt = Date.now()
        scheduleSave()
      }
    }
  }

  async function init() {
    await initConfig()
    const data = await loadSessionsFromBackend()
    sessions.value = data.sessions
    currentSessionId.value = data.currentSessionId
    initialized.value = true

    // Restore current session or create a new one
    if (currentSessionId.value) {
      const s = sessions.value.find(s => s.id === currentSessionId.value)
      if (s) {
        messages.value = s.messages.map(m => reactive({ ...m }) as AIMessage)
      } else {
        createSession()
      }
    } else {
      createSession()
    }
  }

  async function initConfig() {
    try {
      const loaded = await LoadAIConfig()
      if (loaded.apiKey || loaded.baseURL || loaded.model) {
        config.value = {
          apiKey: loaded.apiKey || DEFAULT_CONFIG.apiKey,
          baseURL: loaded.baseURL || DEFAULT_CONFIG.baseURL,
          model: loaded.model || DEFAULT_CONFIG.model,
        }
      }
    } catch {
      // ignore, use defaults
    }
  }

  async function saveConfig() {
    try {
      await SaveAIConfig({
        apiKey: config.value.apiKey,
        baseURL: config.value.baseURL,
        model: config.value.model,
      })
    } catch {
      // ignore save errors
    }
  }

  function setConfig(updates: Partial<AIConfig>) {
    config.value = { ...config.value, ...updates }
  }

  let saveTimer: ReturnType<typeof setTimeout> | null = null

  function scheduleSave() {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => doSave(), 300)
  }

  async function doSave() {
    try {
      const data = {
        sessions: sessions.value.map(s => ({
          id: s.id,
          name: s.name,
          createdAt: s.createdAt,
          updatedAt: s.updatedAt,
          messages: s.messages.map(m => ({
            id: m.id,
            role: m.role,
            content: m.content,
            tool_call_id: m.tool_call_id || '',
            _rawApiMsg: m._rawApiMsg ? JSON.stringify(m._rawApiMsg) : '',
          }))
        })),
        currentSessionId: currentSessionId.value || '',
      }
      await SaveAISessions(data as any)
    } catch {
      // ignore save errors
    }
  }

  function createSession(name?: string) {
    const now = Date.now()
    const session: AISession = {
      id: `session-${now}`,
      name: name || t('ai.newSession'),
      createdAt: now,
      updatedAt: now,
      messages: []
    }
    sessions.value.unshift(session)
    currentSessionId.value = session.id
    messages.value = []
    scheduleSave()
  }

  function switchSession(sessionId: string) {
    const s = sessions.value.find(s => s.id === sessionId)
    if (!s) return
    currentSessionId.value = sessionId
    messages.value = s.messages.map(m => reactive({ ...m }) as AIMessage)
  }

  function deleteSession(sessionId: string) {
    const idx = sessions.value.findIndex(s => s.id === sessionId)
    if (idx === -1) return
    sessions.value.splice(idx, 1)
    scheduleSave()
    if (currentSessionId.value === sessionId) {
      if (sessions.value.length > 0) {
        switchSession(sessions.value[0].id)
      } else {
        createSession()
      }
    }
  }

  function renameSession(sessionId: string, name: string) {
    const s = sessions.value.find(s => s.id === sessionId)
    if (s) {
      s.name = name
      scheduleSave()
    }
  }

  function stop() {
    stopRequested.value = true
    isRunning.value = false
  }

  function resetStop() {
    stopRequested.value = false
  }

  // Build Anthropic-native message array (system is separate top-level field)
  const conversation = computed(() => {
    const MAX_CTX_MSGS = 50

    // Take only recent messages to stay within context window
    let recentMsgs = messages.value.slice(-MAX_CTX_MSGS)

    // Don't start the conversation with an orphaned tool_result whose matching
    // tool_use was truncated out of the window. Strip leading tool messages
    // until we hit a user or assistant message.
    while (recentMsgs.length > 0 && recentMsgs[0].role === 'tool') {
      recentMsgs.shift()
    }

    // Collect all resolved tool_use IDs from tool_result messages
    const resolvedIds = new Set<string>()
    for (const m of recentMsgs) {
      if (m.role === 'tool' && m.tool_call_id) {
        resolvedIds.add(m.tool_call_id)
      }
    }

    const result: Array<Record<string, unknown>> = []

    for (const m of recentMsgs) {
      if (m.id.startsWith('dbg-')) continue
      if (m.needsContinue) continue  // UI-only prompts, not part of LLM conversation

      // Tool messages: ones with tool_call_id are real tool_results for the API;
      // ones without are display-only system errors and must not be sent.
      if (m.role === 'tool') {
        if (m.tool_call_id) {
          result.push({
            role: 'user',
            content: [{ type: 'tool_result', tool_use_id: m.tool_call_id, content: m.content }]
          })
        }
        continue
      }

      // Skip assistant messages that are API error placeholders from before the fix
      if (m.role === 'assistant' && typeof m.content === 'string' && m.content.includes('[Error:')) {
        continue
      }

      // Assistant with raw API blocks: filter dangling tool_use blocks without matching tool_result
      if (m._rawApiMsg) {
        const raw = m._rawApiMsg as Record<string, unknown>
        const content = raw.content
        if (Array.isArray(content)) {
          const filtered = (content as Array<Record<string, unknown>>).filter((block: Record<string, unknown>) => {
            if (block.type === 'tool_use') {
              return resolvedIds.has(block.id as string)
            }
            return true
          })
          if (filtered.length === 0 && !m.content && !(m.pendingTools?.length || pendingCommand.value?.messageId === m.id)) continue
          result.push({ ...raw, content: filtered })
        } else {
          result.push({ ...raw })
        }
        continue
      }

      // Assistant with legacy tool_calls: filter dangling ones, build content blocks
      if (m.role === 'assistant' && m.tool_calls?.length) {
        const resolved = m.tool_calls.filter(tc => resolvedIds.has(tc.id))
        if (!m.content && resolved.length === 0 && !(m.pendingTools?.length || pendingCommand.value?.messageId === m.id)) continue

        const blocks: Array<Record<string, unknown>> = []
        if (m.content) {
          blocks.push({ type: 'text', text: m.content })
        }
        for (const tc of resolved) {
          let input: Record<string, unknown> = {}
          try { input = JSON.parse(tc.function.arguments) } catch { /* passthrough */ }
          blocks.push({ type: 'tool_use', id: tc.id, name: tc.function.name, input })
        }
        result.push({ role: 'assistant', content: blocks })
        continue
      }

      // Skip empty assistant messages (no content, no tool calls, no raw api msg, no pending tools)
      if (m.role === 'assistant' && !m.content && !(m.pendingTools?.length || pendingCommand.value?.messageId === m.id)) continue

      result.push({ role: m.role, content: m.content })
    }

    // Final safety pass: remove any dangling tool_use blocks that escaped filtering
    // and orphaned tool_results whose matching tool_use is missing.
    // This handles edge cases from truncated history or pre-fix conversations.
    const toolResultIds = new Set<string>()
    for (const msg of result) {
      if (msg.role === 'user' && Array.isArray(msg.content)) {
        for (const block of msg.content as Array<Record<string, unknown>>) {
          if (block.type === 'tool_result') {
            toolResultIds.add(block.tool_use_id as string)
          }
        }
      }
    }

    const toolUseIds = new Set<string>()
    for (const msg of result) {
      if (msg.role === 'assistant' && Array.isArray(msg.content)) {
        for (const block of msg.content as Array<Record<string, unknown>>) {
          if (block.type === 'tool_use') {
            toolUseIds.add(block.id as string)
          }
        }
      }
    }

    const cleaned: Array<Record<string, unknown>> = []
    for (const msg of result) {
      if (msg.role === 'assistant' && Array.isArray(msg.content)) {
        const blocks = (msg.content as Array<Record<string, unknown>>).filter((block) => {
          if (block.type === 'tool_use') {
            return toolResultIds.has(block.id as string)
          }
          return true
        })
        if (blocks.length === 0) continue
        cleaned.push({ ...msg, content: blocks })
      } else if (msg.role === 'user' && Array.isArray(msg.content)) {
        const blocks = (msg.content as Array<Record<string, unknown>>).filter((block) => {
          if (block.type === 'tool_result') {
            return toolUseIds.has(block.tool_use_id as string)
          }
          return true
        })
        // Only skip the message if ALL tool_results were orphaned (nothing left).
        // If valid tool_results remain, keep the message even if it's tool-only.
        if (blocks.length === 0) {
          continue
        }
        cleaned.push({ ...msg, content: blocks })
      } else {
        cleaned.push(msg)
      }
    }

    // Additional validation: ensure no consecutive user messages ( Anthropic rejects this )
    const deduped: Array<Record<string, unknown>> = []
    for (const msg of cleaned) {
      if (msg.role === 'user' && deduped.length > 0 && deduped[deduped.length - 1].role === 'user') {
        const prev = deduped[deduped.length - 1]
        const prevBlocks = Array.isArray(prev.content) ? prev.content : [{ type: 'text', text: prev.content }]
        const msgBlocks = Array.isArray(msg.content) ? msg.content : [{ type: 'text', text: msg.content }]
        prev.content = [...prevBlocks, ...msgBlocks]
      } else {
        deduped.push(msg)
      }
    }

    return deduped
  })

  const systemPrompt = computed(() => SYSTEM_PROMPT)

  return {
    visible,
    toggle,
    messages,
    addMessage,
    clearMessages,
    mode,
    config,
    isRunning,
    saveConfig,
    initConfig,
    setConfig,
    conversation,
    systemPrompt,
    stopRequested,
    stop,
    resetStop,
    sessions,
    currentSessionId,
    createSession,
    switchSession,
    deleteSession,
    renameSession,
    lastDebugInfo,
    setDebugInfo,
    clearDebugInfo,
    pendingCommand,
    setPendingCommand,
    clearPendingCommand,
    initialized,
    init
  }
})
