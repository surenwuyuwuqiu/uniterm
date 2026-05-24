import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import type { AIMessage, AIConfig, ExecutionMode, AISession } from '../types/ai'
import { SaveAIConfig, LoadAIConfig, SaveAISessions, LoadAISessions } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import { t } from '../i18n'

const SYSTEM_PROMPT = `You are an AI assistant inside uniTerm, a terminal emulator. You can execute shell commands in the user's active terminal to help them complete tasks.

When you need to run a command, use the execute_command tool. The command will be executed in the active terminal session and you will receive its stdout/stderr output.

CRITICAL RULES:
- You can only send ONE execute_command tool call at a time. Never send multiple tool calls in a single response.
- Always explain what you are about to do before executing commands.
- Use commands appropriate for the current shell (the shell type will be provided in the context below).
- If a command might be destructive, warn the user.
- Chain multiple commands with && or ; when appropriate.
- If the output is too long, summarize the key findings.
- Commands have a 60-second timeout. If a command times out, you will see "[Command timed out after 60s...]". In that case, you can either wait (the command may still be running) or suggest canceling it with Ctrl+C.
- Do NOT send a new command if the previous one might still be running, unless you intend to cancel it first.
- At the START of EVERY response, you MUST read the "当前终端 (CURRENT SHELL)" line at the very top of this prompt. IGNORE any memory of what the previous shell was — only the CURRENT SHELL line matters. Do NOT assume the shell type based on previous turns.
- The user may switch terminal tabs at any time. Each terminal is an independent environment with its own working directory, environment variables, and file system state. Previous command results (like current directory, env vars, created files) may be COMPLETELY INVALID in the new terminal. ALWAYS reassess the environment before proceeding.
- When the terminal program type changes (e.g. PowerShell -> Git Bash, or CMD -> PowerShell), you MUST immediately switch to the NEW shell's command syntax. NEVER mix commands from different shell types in the same response. The current shell type is shown at the top of this prompt and takes ABSOLUTE PRECEDENCE over any previous turns or your memory.
- Do NOT invoke a different shell executable (e.g. powershell.exe, cmd.exe, bash.exe, pwsh.exe, wsl.exe) from within the current terminal to run commands. ALWAYS use the native syntax of the CURRENT shell only. For example, if the current shell is Git Bash, do NOT run "powershell.exe -Command ..." or "cmd /c ..."; use bash-native commands like "df -h" or "ls -la" instead. If a native equivalent does not exist, use the closest native workaround rather than spawning another shell.

RISK CLASSIFICATION:
Every execute_command call MUST include a "risk" field. Classify each command honestly:
- "read": only inspects/views data, no modifications at all (ls, cat, grep, head, tail, df, du, ps, top, find, pwd, whoami, git status/log/diff, docker ps/images/logs, npm list, pip list, go version/env, etc.)
- "write": modifies or creates data, but not system-destructive (echo > file, touch, mkdir, cp, mv, git add/commit/push, curl POST, npm install, pip install, apt install, brew install, etc.)
- "dangerous": potentially destructive or system-altering (rm, > overwrite important files, chmod, chown, shutdown, reboot, mkfs, dd, force push, kill -9, etc.)

For chained commands with && or ;, classify based on the MOST risky operation in the chain.

--- NEGATIVE EXAMPLES (STRICTLY FORBIDDEN) ---
You MUST NEVER do the following. These are common mistakes caused by relying on memory instead of reading the CURRENT SHELL line:
❌ In Git Bash, do NOT run: Get-CimInstance Win32_LogicalDisk
❌ In PowerShell, do NOT run: ls -la /mnt/c/
❌ In CMD, do NOT run: df -h
❌ In Git Bash, do NOT run: powershell.exe -Command "..."
❌ In PowerShell, do NOT run: bash -c "..."
If the native equivalent does not exist in the CURRENT shell, find a workaround using ONLY the CURRENT shell's syntax.

--- VIOLATION CONSEQUENCE ---
If you use command syntax from a shell type that does NOT match the CURRENT SHELL, the command will be treated as INVALID. You MUST stop, re-read the CURRENT SHELL line at the top, and rewrite the command using the correct syntax. Do NOT attempt to bypass this by spawning another interpreter (e.g. bash -c, powershell.exe -Command, cmd /c).`

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
        tool_calls: m.tool_calls || [],
        pendingTools: m.pendingTools || [],
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
  const lastPanelContext = ref<{ panelId: string; shellPath: string } | null>(null)

  function setLastPanelContext(panelId: string, shellPath: string) {
    lastPanelContext.value = { panelId, shellPath }
  }

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
        doSave()
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
        doSave()
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
        messages.value = s.messages.map(m => {
          const msg = { ...m }
          if (typeof msg._rawApiMsg === 'string' && msg._rawApiMsg) {
            try { msg._rawApiMsg = JSON.parse(msg._rawApiMsg) } catch { delete msg._rawApiMsg }
          }
          return reactive(msg) as AIMessage
        })
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
            tool_calls: m.tool_calls || [],
            pendingTools: m.pendingTools || [],
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
    doSave()
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
    doSave()
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
      doSave()
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

    // Final safety pass: enforce that every tool_use is immediately followed
    // by a user message containing its matching tool_result, and every
    // tool_result is immediately preceded by an assistant with its tool_use.
    // The Anthropic API rejects tool_use blocks that are not resolved in the
    // very next message.
    const cleaned: Array<Record<string, unknown>> = []
    for (let i = 0; i < result.length; i++) {
      const msg = result[i]

      if (msg.role === 'assistant' && Array.isArray(msg.content)) {
        const nextMsg = i + 1 < result.length ? result[i + 1] : null
        const blocks = (msg.content as Array<Record<string, unknown>>).filter((block) => {
          if (block.type === 'tool_use') {
            if (!nextMsg || nextMsg.role !== 'user' || !Array.isArray(nextMsg.content)) {
              return false
            }
            return (nextMsg.content as Array<Record<string, unknown>>).some(
              (nb) => nb.type === 'tool_result' && nb.tool_use_id === block.id
            )
          }
          return true
        })
        if (blocks.length === 0) continue
        cleaned.push({ ...msg, content: blocks })
      } else if (msg.role === 'user' && Array.isArray(msg.content)) {
        const prevMsg = i > 0 ? result[i - 1] : null
        const blocks = (msg.content as Array<Record<string, unknown>>).filter((block) => {
          if (block.type === 'tool_result') {
            if (!prevMsg || prevMsg.role !== 'assistant' || !Array.isArray(prevMsg.content)) {
              return false
            }
            return (prevMsg.content as Array<Record<string, unknown>>).some(
              (pb) => pb.type === 'tool_use' && pb.id === block.tool_use_id
            )
          }
          return true
        })
        if (blocks.length === 0) continue
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

  // Reload AI config when settings change via sync
  EventsOn('store:settings:changed', () => {
    initConfig()
  })

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
    init,
    lastPanelContext,
    setLastPanelContext,
    doSave
  }
})
