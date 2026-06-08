import { chat, AVAILABLE_TOOLS } from './llm'
import { executeCommand } from './terminalAgent'
import { useAIStore } from '../stores/aiStore'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'

function getActivePanel() {
  const tabStore = useTabStore()
  const panelStore = usePanelStore()

  // Check for AI-locked panel first
  const lockedPanelId = tabStore.getAILockedPanel()
  if (lockedPanelId) {
    return panelStore.getPanel(lockedPanelId)
  }

  // Fall back to active panel based on tab type
  const activeTab = tabStore.activeTab
  if (!activeTab) return undefined

  if (activeTab.type === 'terminal' || activeTab.type === 'settings') {
    return panelStore.getPanel(activeTab.panelId)
  }

  if (activeTab.type === 'workspace' && activeTab.activePanelId) {
    return panelStore.getPanel(activeTab.activePanelId)
  }

  return undefined
}

function hasActiveSession(): boolean {
  const panel = getActivePanel()
  return !!panel?.sessionId
}

type RiskLevel = 'read' | 'write' | 'dangerous'

function getRisk(tu: { name: string; input: Record<string, unknown> }): RiskLevel {
  if (tu.name !== 'execute_command') return 'write'
  const risk = tu.input.risk as string | undefined
  if (risk === 'read' || risk === 'write' || risk === 'dangerous') return risk
  return 'write' // conservative fallback
}

function shouldConfirm(risk: RiskLevel): boolean {
  const store = useAIStore()
  switch (store.mode) {
    case 'confirm_all': return true
    case 'confirm_write': return risk !== 'read'
    case 'confirm_dangerous': return risk === 'dangerous'
    case 'bypass': return false
    default: return risk !== 'read'
  }
}

function getShellName(path?: string): string {
  if (!path) return 'Unknown'
  const lower = path.toLowerCase()
  if (lower.includes('pwsh')) return 'PowerShell'
  if (lower.includes('powershell')) return 'Windows PowerShell'
  if (lower.includes('bash')) return 'Bash'
  if (lower.includes('zsh')) return 'Zsh'
  if (lower.includes('fish')) return 'Fish'
  if (lower.includes('cmd')) return 'CMD'
  if (lower.includes('sh')) return 'Sh'
  return path.split(/[\\/]/).pop() || 'Unknown'
}

function buildSystemPrompt(): string {
  const store = useAIStore()
  const activePanel = getActivePanel()

  let base = store.systemPrompt
  if (!activePanel) return base

  const parts: string[] = []
  parts.push(`Current active terminal panel: "${activePanel.title}" (id: ${activePanel.id}, type: ${activePanel.type})`)

  const shellPath = activePanel.config?.shellPath
  if (shellPath) {
    parts.push(`Current shell: ${getShellName(shellPath)} (${shellPath})`)
  } else if (activePanel.type === 'ssh') {
    parts.push(`Current shell: remote SSH session (Unix-like)`)
  }

  if (activePanel.type === 'ssh' && activePanel.config) {
    parts.push(`Connected to: ${activePanel.config.user}@${activePanel.config.host}:${activePanel.config.port}`)
  }

  // Detect terminal switch and inject explicit notice
  const lastCtx = store.lastPanelContext
  let switchNotice = ''
  if (lastCtx && lastCtx.panelId !== activePanel.id) {
    const prev = lastCtx.shellPath ? getShellName(lastCtx.shellPath) : 'another terminal'
    const curr = shellPath ? getShellName(shellPath) : (activePanel.type === 'ssh' ? 'SSH' : 'local terminal')
    switchNotice = `\n\n【TERMINAL SWITCHED】The user has switched from "${prev}" to "${curr}". This is a DIFFERENT terminal environment — previous results (current directory, env vars, files created) may be COMPLETELY INVALID here. You MUST use command syntax appropriate for the NEW terminal type ONLY. Do NOT mix commands from different shell types. Reassess the environment from scratch if needed.`
  }

  const isWindowsShell = shellPath && (
    shellPath.toLowerCase().includes('powershell') ||
    shellPath.toLowerCase().includes('pwsh') ||
    shellPath.toLowerCase().includes('cmd')
  )

  if (isWindowsShell) {
    base += '\n\nIMPORTANT: The active terminal is a Windows shell. Use Windows command syntax, not Unix syntax.\n- For PowerShell: use cmdlets like Get-ChildItem, Set-Location, Get-Content, etc.\n- For CMD: use dir, cd, type, etc.'
  } else if (activePanel.type === 'ssh' || activePanel.type === 'local') {
    base += '\n\nThe active terminal is a Unix-like shell. Use standard Unix syntax (ls, cat, grep, find, etc.).'
  }

  // Build a highly-visible current-shell banner at the very top
  const shellName = shellPath
    ? getShellName(shellPath)
    : (activePanel.type === 'ssh' ? 'SSH (Unix-like)' : 'Unknown')
  const syntaxStyle = isWindowsShell
    ? (shellPath?.toLowerCase().includes('cmd') ? 'Windows CMD (dir, cd, type, wmic)' : 'Windows PowerShell (cmdlets like Get-ChildItem, Set-Location)')
    : 'Unix (ls, cat, grep, df, find)'
  const shellBanner = `========================================\n当前终端 (CURRENT SHELL): ${shellName}\n终端ID (PANEL ID): ${activePanel.id}\n========================================`

  return shellBanner + switchNotice + '\n\n' + base + '\n\n--- Current Context ---\n' + parts.join('\n') + '\n---'
}

export async function runAgent(userInput: string) {
  const store = useAIStore()

  if (!hasActiveSession()) {
    if (userInput) {
      store.addMessage({
        id: `msg-${Date.now()}`,
        role: 'user',
        content: userInput
      })
    }
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'tool',
      content: '请先在主窗口中打开一个终端会话，这样我才能执行命令。'
    })
    return
  }

  // Auto-reject any pending command from previous turn
  if (userInput && store.pendingCommand) {
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'tool',
      content: 'User started a new conversation. Previous command was cancelled.',
      tool_call_id: store.pendingCommand.toolId
    })
    store.clearPendingCommand()
  }

  store.resetStop()
  store.isRunning = true

  // Record current panel context so buildSystemPrompt can detect terminal switches
  const activePanel = getActivePanel()
  if (activePanel) {
    store.setLastPanelContext(activePanel.id, activePanel.config?.shellPath || '')
  }

  if (userInput) {
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'user',
      content: userInput
    })
  }

  let turnCount = 0
  const maxTurns = 20

  while (turnCount < maxTurns) {
    turnCount++

    if (store.stopRequested) {
      store.isRunning = false
      return
    }

    const assistantMsg = store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'assistant',
      content: ''
    })

    const toolUses: Array<{ id: string; name: string; input: Record<string, unknown> }> = []

    const chatOptions: any = {
      system: buildSystemPrompt(),
      messages: store.conversation,
      tools: AVAILABLE_TOOLS,
      onChunk: (chunk: string) => {
        if (store.stopRequested) return
        assistantMsg.content += chunk
      },
      onToolUse: (tu: { id: string; name: string; input: Record<string, unknown> }) => {
        if (store.stopRequested) return
        toolUses.push(tu)
      }
    }
    try {
      await chat(chatOptions)
      // Preserve raw API message blocks for conversation history
      if (chatOptions._rawApiMsg) {
        assistantMsg._rawApiMsg = chatOptions._rawApiMsg
      }
      // Save immediately so the complete assistant content and raw API message are persisted
      store.doSave()
    } catch (e: any) {
      const errMsg = e.message ?? String(e)
      // Convert the failed assistant placeholder to a display-only tool message.
      // This keeps the error visible in the UI without polluting the API conversation.
      assistantMsg.role = 'tool'
      assistantMsg.content = `[Error: ${errMsg}]`
      delete assistantMsg._rawApiMsg
      delete assistantMsg.tool_calls
      store.setDebugInfo(store.conversation, errMsg)
      store.isRunning = false
      return
    }

    if (store.stopRequested) {
      // Cancel any tool calls that were received but not executed
      const cancelledIds = new Set<string>()
      const rawContent = assistantMsg._rawApiMsg?.content
      if (Array.isArray(rawContent)) {
        for (const block of rawContent) {
          if (block.type === 'tool_use' && !cancelledIds.has(block.id)) {
            store.addMessage({
              id: `msg-${Date.now()}`,
              role: 'tool',
              content: 'Command was cancelled by user (Stop).',
              tool_call_id: block.id
            })
            cancelledIds.add(block.id)
          }
        }
      }
      store.isRunning = false
      return
    }

    // Enforce single tool call
    if (toolUses.length > 1) {
      toolUses.splice(1)
    }

    // Store tool calls in the message for UI confirmation
    if (toolUses.length > 0) {
      assistantMsg.tool_calls = toolUses.map(tu => ({
        id: tu.id,
        type: 'function' as const,
        function: {
          name: tu.name,
          arguments: JSON.stringify(tu.input)
        }
      }))
    }

    if (!assistantMsg.content && toolUses.length === 0) {
      assistantMsg.content = '[No response received from the model. Check your API settings and network connection.]'
      store.isRunning = false
      return
    }

    if (toolUses.length === 0) {
      store.isRunning = false
      return
    }

    // Process exactly one tool call
    const tu = toolUses[0]
    if (tu.name === 'execute_command') {
      const command = tu.input.command as string

      const risk = getRisk(tu)

      if (shouldConfirm(risk)) {
        store.setPendingCommand({
          messageId: assistantMsg.id,
          toolId: tu.id,
          command,
          risk,
          dangerous: risk === 'dangerous'
        })
        // Keep tool_calls for UI display of IN/OUT boxes
        assistantMsg.tool_calls = [{
          id: tu.id,
          type: 'function' as const,
          function: {
            name: tu.name,
            arguments: JSON.stringify(tu.input)
          }
        }]
        store.isRunning = false
        return
      }

      // Auto-execute
      try {
        const result = await executeCommand(command)
        store.addMessage({
          id: `msg-${Date.now()}`,
          role: 'tool',
          content: result.output,
          tool_call_id: tu.id
        })
      } catch (e: any) {
        store.addMessage({
          id: `msg-${Date.now()}`,
          role: 'tool',
          content: `[Error executing command: ${e.message ?? e}]`,
          tool_call_id: tu.id
        })
      }
    }
  }

  // Max turns reached - prompt user to continue
  if (turnCount >= maxTurns) {
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'assistant',
      content: `已达到最大对话轮次限制（${maxTurns}轮）。点击"继续"或发送任意消息以继续。`,
      needsContinue: true
    })
  }

  store.isRunning = false
  store.doSave()
}

export async function continueAgent() {
  const store = useAIStore()
  const lastMsg = store.messages[store.messages.length - 1]
  if (lastMsg?.needsContinue) {
    store.messages.pop()
  }
  await runAgent('')
}

export async function approveTool(_messageId: string) {
  const store = useAIStore()
  const cmd = store.pendingCommand
  if (!cmd) return

  if (!hasActiveSession()) {
    store.clearPendingCommand()
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'tool',
      content: '请先打开一个终端会话，再执行此命令。'
    })
    return
  }

  store.clearPendingCommand()
  store.isRunning = true


  try {
    const result = await executeCommand(cmd.command)
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'tool',
      content: result.output,
      tool_call_id: cmd.toolId
    })
  } catch (e: any) {
    store.addMessage({
      id: `msg-${Date.now()}`,
      role: 'tool',
      content: `[Error executing command: ${e.message ?? e}]`,
      tool_call_id: cmd.toolId
    })
  }
  await runAgent('')
}

export function rejectTool(_messageId: string) {
  const store = useAIStore()
  const cmd = store.pendingCommand
  if (!cmd) return

  store.clearPendingCommand()

  store.addMessage({
    id: `msg-${Date.now()}`,
    role: 'tool',
    content: 'User rejected this command.',
    tool_call_id: cmd.toolId
  })

  setTimeout(() => runAgent(''), 0)
}
