import { EventsOn } from '../../wailsjs/runtime'
import { SessionWrite } from '../../wailsjs/go/main/App'
import { getManagedTerminal } from '../services/terminalManager'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'

export interface ExecuteResult {
  output: string
  exitCode: number
  timedOut?: boolean
}

export interface WatchResult {
  output: string
  timedOut: boolean
}

// Split terminal output into display lines. Splits on newlines and, within a
// line, keeps only the text after the last carriage return so progress-bar
// style redraws (which overwrite the line with bare \r) collapse to their
// final state — the same way the text appears on screen.
function toDisplayLines(clean: string): string[] {
  return clean.split(/\r?\n/).map((line) => {
    const cr = line.lastIndexOf('\r')
    return cr >= 0 ? line.slice(cr + 1) : line
  })
}

// Watch session output and resolve when the command finishes.
//
// Completion is detected by the shell prompt reappearing: `promptLine` is the
// prompt captured immediately before the command was sent, and once that exact
// line shows up again at the bottom of the output the shell is back at the
// prompt and the command is done. No marker is injected into the shell, so the
// terminal shows nothing extra.
//
// When `promptLine` is empty (prompt could not be captured, or the prompt is
// dynamic and never reappears verbatim) detection is skipped entirely and the
// call resolves on timeout — the command already carries its own timeout.
export function watchOutput(
  sessionId: string,
  promptLine: string,
  timeoutMs: number
): { promise: Promise<WatchResult>; cleanup: () => void } {
  let timeoutId: ReturnType<typeof setTimeout>
  let unsubscribe: (() => void) | null = null
  let resolved = false
  let output = ''

  const cleanup = () => {
    clearTimeout(timeoutId)
    unsubscribe?.()
    resolved = true
  }

  const promise = new Promise<WatchResult>((resolve) => {
    unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      if (payload.id !== sessionId || resolved) return

      output += payload.data
      if (!promptLine) return

      const lines = toDisplayLines(stripAnsi(output))
      // Locate the last non-blank display line.
      let last = lines.length - 1
      while (last >= 0 && lines[last].trimEnd() === '') last--
      // Require at least the echoed command line before the prompt, so the
      // reappearing prompt — not the initial state — is what triggers.
      if (last < 1) return

      if (lines[last].trimEnd() === promptLine) {
        cleanup()
        const result = lines.slice(0, last).join('\n').trim()
        resolve({ output: result, timedOut: false })
      }
    })

    timeoutId = setTimeout(() => {
      cleanup()
      resolve({
        output: stripAnsi(output).trim(),
        timedOut: true,
      })
    }, timeoutMs)
  })

  return { promise, cleanup }
}

export function truncateOutput(
  text: string,
  headLines: number,
  tailLines: number
): string {
  const lines = text.split('\n')
  const total = lines.length
  const threshold = headLines + tailLines
  if (total <= threshold) return text

  const head = lines.slice(0, headLines).join('\n')
  const tail = lines.slice(total - tailLines).join('\n')
  const omitted = total - headLines - tailLines
  return `${head}\n\n─────── [截断: 共 ${total} 行, 已省略 ${omitted} 行] ────────\n调整 head_lines / tail_lines 参数可查看更多内容。\n\n${tail}`
}

function resolveActiveSession(): { sessionId: string; shellPath?: string } {
  const tabStore = useTabStore()
  const panelStore = usePanelStore()

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

  return { sessionId: panel.sessionId, shellPath: panel.config?.shellPath }
}

function getShellNewline(shellPath?: string): string {
  const lowerShell = (shellPath || '').toLowerCase()
  if (lowerShell.includes('powershell') || lowerShell.includes('pwsh')) {
    return '\r'
  } else if (lowerShell.includes('cmd')) {
    return '\r\n'
  } else if (lowerShell.includes('bash') || lowerShell.includes('sh')) {
    return '\r\n'
  } else {
    return '\n'
  }
}

// Read the current prompt line from the terminal buffer. Called right before a
// command is sent, when the cursor sits on the (freshly drawn) prompt with no
// input yet, so the cursor line's text is exactly the prompt string. Returns ''
// when unavailable, which disables prompt detection for that command.
function capturePromptLine(sessionId: string): string {
  const managed = getManagedTerminal(sessionId)
  const terminal = managed?.terminal
  if (!terminal) return ''
  const buffer = terminal.buffer.active
  const line = buffer.getLine(buffer.baseY + buffer.cursorY)
  if (!line) return ''
  return line.translateToString(true).trimEnd()
}

export async function executeCommand(
  command: string,
  timeoutMs: number = 60000,
  headLines: number = 50,
  tailLines: number = 300
): Promise<ExecuteResult> {
  const { sessionId, shellPath } = resolveActiveSession()
  const promptLine = capturePromptLine(sessionId)
  const fullCommand = buildCommand(command, shellPath)
  const newline = getShellNewline(shellPath)

  await SessionWrite(sessionId, fullCommand + newline)

  const { promise } = watchOutput(sessionId, promptLine, timeoutMs)
  const result = await promise

  if (result.timedOut) {
    const truncated = truncateOutput(result.output, headLines, tailLines)
    const timeoutSec = Math.round(timeoutMs / 1000)
    return {
      output: truncated
        + `\n\n⚠️ 命令在 ${timeoutSec}s 内未完成，可能仍在运行中。\n`
        + `请勿重复发送相同命令。\n`
        + `• 如果输出显示进度（百分比、文件名滚动等）→ 使用 collect_output 继续等待\n`
        + `• 如果输出显示密码/确认提示 → 使用 send_terminal_key 响应\n`
        + `• 如果命令卡住无响应 → 使用 interrupt_command 取消`,
      exitCode: -1,
      timedOut: true,
    }
  }

  return {
    output: truncateOutput(result.output, headLines, tailLines),
    exitCode: 0,
    timedOut: false,
  }
}

export interface StartResult {
  output: string
  started: boolean
}

export async function startCommand(command: string): Promise<StartResult> {
  const { sessionId, shellPath } = resolveActiveSession()
  const newline = getShellNewline(shellPath)

  await SessionWrite(sessionId, command + newline)

  // Collect output for 3 seconds, then return
  return new Promise((resolve) => {
    let output = ''
    const unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      if (payload.id !== sessionId) return
      output += payload.data
    })

    setTimeout(() => {
      unsubscribe()
      resolve({
        output: stripAnsi(output).trim(),
        started: true,
      })
    }, 3000)
  })
}

export interface CaptureResult {
  output: string
}

export function captureTerminal(tailLines: number = 200): CaptureResult {
  const { sessionId } = resolveActiveSession()

  const managed = getManagedTerminal(sessionId)
  if (!managed || !managed.terminal) {
    return { output: '' }
  }

  const terminal = managed.terminal
  const buffer = terminal.buffer.active
  const totalLines = buffer.length

  if (totalLines === 0) {
    return { output: '' }
  }

  // Find the last non-blank line — skip trailing empty space at the bottom of the terminal
  let lastContentLine = totalLines - 1
  while (lastContentLine >= 0) {
    const line = buffer.getLine(lastContentLine)
    if (line && line.translateToString().trim() !== '') break
    lastContentLine--
  }

  if (lastContentLine < 0) {
    return { output: '' }
  }

  // Capture up to tailLines lines, ending at the last non-blank line
  const startLine = Math.max(0, lastContentLine - tailLines + 1)
  const lines: string[] = []

  for (let i = startLine; i <= lastContentLine; i++) {
    const line = buffer.getLine(i)
    if (line) lines.push(line.translateToString())
  }

  return { output: lines.join('\n') }
}

export interface CollectResult {
  output: string
  timedOut: boolean
}

export async function collectOutput(
  timeoutMs: number = 30000,
  headLines: number = 100,
  tailLines: number = 300
): Promise<CollectResult> {
  const { sessionId } = resolveActiveSession()

  return new Promise((resolve) => {
    let output = ''
    const unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
      if (payload.id !== sessionId) return
      output += payload.data
    })

    setTimeout(() => {
      unsubscribe()
      resolve({
        output: truncateOutput(stripAnsi(output).trim(), headLines, tailLines),
        timedOut: true,
      })
    }, timeoutMs)
  })
}

interface SendKeyResult {
  output: string
}

export async function sendTerminalKey(
  input?: string,
  control?: 'ctrl_c' | 'ctrl_d' | 'enter',
  sendEnter: boolean = true
): Promise<SendKeyResult> {
  const { sessionId, shellPath } = resolveActiveSession()

  let data: string
  if (control) {
    if (control === 'ctrl_c') {
      data = '\x03'
    } else if (control === 'ctrl_d') {
      data = '\x04'
    } else if (control === 'enter') {
      data = '\n'
    } else {
      data = ''
    }
  } else if (input !== undefined && input !== '') {
    data = input
  } else {
    throw new Error('Either input or control must be provided')
  }

  // Append shell-appropriate newline when send_enter is true and input was provided
  if (sendEnter && !control && input !== undefined && input !== '') {
    data += getShellNewline(shellPath)
  }

  await SessionWrite(sessionId, data)

  // For ctrl_c / ctrl_d: passively capture shell response for a short time.
  // No marker injection — avoids corrupting interactive program input.
  if (control === 'ctrl_c' || control === 'ctrl_d') {
    return new Promise((resolve) => {
      let output = ''
      const unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
        if (payload.id !== sessionId) return
        output += payload.data
      })
      setTimeout(() => {
        unsubscribe()
        resolve({ output: stripAnsi(output).trim() || '(input sent)' })
      }, 1000)
    })
  }

  return { output: '(input sent)' }
}

// Build the string sent to the shell. No completion marker is appended — the
// AI executor detects completion by watching for the shell prompt to reappear
// (see watchOutput). This keeps the terminal clean and, for POSIX shells,
// avoids corrupting multi-line input such as here-documents. A single leading
// space keeps the command out of shell history (HISTCONTROL=ignorespace).
function buildCommand(command: string, shellPath?: string): string {
  const lower = (shellPath || '').toLowerCase()
  if (lower.includes('powershell') || lower.includes('pwsh') || lower.includes('cmd')) {
    return command
  }
  // bash / sh / zsh / fish
  return ` ${command}`
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
