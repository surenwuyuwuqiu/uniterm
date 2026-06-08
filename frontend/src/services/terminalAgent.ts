import { EventsOn } from '../../wailsjs/runtime'
import { SessionWrite } from '../../wailsjs/go/main/App'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'

export interface ExecuteResult {
  output: string
  exitCode: number
}

export async function executeCommand(command: string): Promise<ExecuteResult> {
  const tabStore = useTabStore()
  const panelStore = usePanelStore()

  // Check for AI-locked panel first
  const lockedPanelId = tabStore.getAILockedPanel()
  let panel = lockedPanelId ? panelStore.getPanel(lockedPanelId) : null

  if (!panel) {
    const activeTab = tabStore.activeTab
    if (activeTab?.type === 'terminal' || activeTab?.type === 'settings') {
      panel = panelStore.getPanel(activeTab.panelId)
    } else if (activeTab?.type === 'workspace' && activeTab.activePanelId) {
      panel = panelStore.getPanel(activeTab.activePanelId)
    }
  }

  if (!panel || !panel.sessionId) {
    throw new Error('No active terminal session')
  }

  const sessionId = panel.sessionId

  const marker = `__AI_DONE_${Date.now()}_${Math.random().toString(36).slice(2, 8)}__`
  const shellPath = panel.config?.shellPath
  const fullCommand = buildCommand(command, marker, shellPath)

  // Choose line ending based on shell type and platform
  const lowerShell = (shellPath || '').toLowerCase()
  let newline: string
  if (lowerShell.includes('powershell') || lowerShell.includes('pwsh')) {
    // PowerShell via ConPTY: \r executes without leaving a trailing \n that causes the >> prompt
    newline = '\r'
  } else if (lowerShell.includes('cmd')) {
    // CMD via ConPTY expects \r\n
    newline = '\r\n'
  } else if (lowerShell.includes('bash') || lowerShell.includes('sh')) {
    // Bash/Git Bash via ConPTY: \r\n is treated as Enter by the Windows console layer
    newline = '\r\n'
  } else {
    // Default / Unix: \n
    newline = '\n'
  }

  await SessionWrite(sessionId, fullCommand + newline)

  return new Promise((resolve) => {
    let output = ''
    let timeoutId: ReturnType<typeof setTimeout>
    let eventCount = 0
    let markerSeen = false
    let lastScanPos = 0

    const unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      eventCount++
      if (payload.id !== sessionId) return

      output += payload.data
      const clean = stripAnsi(output)

      const scanStart = Math.max(0, lastScanPos - marker.length)
      lastScanPos = clean.length
      let searchIdx = scanStart
      while ((searchIdx = clean.indexOf(marker, searchIdx)) !== -1) {
        searchIdx += marker.length
        if (!markerSeen) {
          markerSeen = true
          continue
        }
        clearTimeout(timeoutId)
        unsubscribe()
        const result = clean.slice(0, searchIdx - marker.length).trim()
        resolve({ output: result, exitCode: 0 })
        return
      }
    })

    timeoutId = setTimeout(() => {
      const cleanOutput = stripAnsi(output).trim()
      unsubscribe()
      const result = cleanOutput + '\n\n[Command timed out after 60s. The command may still be running. You can wait for it to complete or cancel it.]'
      resolve({ output: result, exitCode: -1 })
    }, 60000)
  })
}

function buildCommand(command: string, marker: string, shellPath?: string): string {
  const lower = (shellPath || '').toLowerCase()
  if (lower.includes('powershell') || lower.includes('pwsh')) {
    // PowerShell syntax
    return `${command};Write-Output "${marker}"`
  }
  if (lower.includes('cmd')) {
    // CMD syntax
    return `${command}&echo ${marker}`
  }
  // Default: bash / sh / zsh / fish
  return ` ${command};echo "${marker}"`
}

// Simple ANSI stripper for extracting readable text from terminal output
function stripAnsi(str: string): string {
  return str
    .replace(/\x1B\[[0-9;?]*[A-Za-z]/g, '')
    .replace(/\x1B][0-9;]*(?:\x07|\x1B\\)/g, '')
    .replace(/\x1B[()[\]#\^%@>=]/g, '')
    .replace(/\x1B[/!_]./g, '')
    .replace(/\x1B./g, '')
}
