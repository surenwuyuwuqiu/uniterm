import { ref } from 'vue'
import type { Terminal } from '@xterm/xterm'

export interface CursorPosition {
  x: number
  y: number
}

export interface UseTerminalInputOptions {
  mode: 'ssh' | 'sftp' | 'local'
  sessionId: string | null | undefined
  onHistoryExtract?: (command: string) => void
  onResetSuppress?: () => void
  enableHistory?: boolean
}

export function useTerminalInput(terminal: Terminal | null, options: UseTerminalInputOptions) {
  const lineBuffer = ref('')
  const cursorIndex = ref(0)
  const currentToken = ref('')
  const cursorPixelPos = ref<CursorPosition>({ x: 0, y: 0 })

  let inAlternateScreen = false
  let cursorPosTimer: ReturnType<typeof setTimeout> | null = null

  function stripAnsi(str: string): string {
    return str
      // OSC sequences: ESC ] ... BEL or ESC ] ... ESC \
      .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '')
      // CSI sequences: ESC [ params final-byte
      .replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '')
      // Single-char FE escapes: ESC @ to ESC _, ESC ` to ESC ~
      .replace(/\x1b[@-Z\-_]/g, '')
      // Character set designation: ESC ( B, ESC ) B, etc.
      .replace(/\x1b[()[\]{}][0-9A-Za-z]/g, '')
  }

  const MAX_COMMAND_LENGTH = 200

  function getCurrentCommandFromTerminal(): string | null {
    if (!terminal) return null
    try {
      const buffer = (terminal as any).buffer?.active
      if (!buffer) return null
      const line = buffer.getLine(buffer.cursorY)
      if (!line) return null
      const lineText = line.translateToString().trim()
      // Strip ANSI escape sequences before matching
      const cleanText = stripAnsi(lineText)
      // Match prompt followed by command; use (?:\s+|$) so empty commands after prompt are handled too
      const match = cleanText.match(/(.+?[$#>\]])(?:\s+|$)(.*)/)
      if (match) {
        const command = match[2].trim()
        if (command && !command.includes('__AI_DONE_') && command.length <= MAX_COMMAND_LENGTH) {
          return command
        }
      }
    } catch {
      // Ignore errors
    }
    return null
  }

  function updateToken() {
    const text = lineBuffer.value
    const idx = cursorIndex.value
    const beforeCursor = text.slice(0, idx)
    // Use the entire command before cursor for suggestion matching,
    // so "git status" matches history entries like "git status --short".
    currentToken.value = beforeCursor.trim()
  }

  function updateCursorPosition() {
    if (!terminal) {
      cursorPixelPos.value = { x: 0, y: 0 }
      return
    }
    try {
      const core = (terminal as any)._core
      if (!core) return
      const buffer = core.buffer
      const renderer = core._renderService
      if (!buffer || !renderer) return
      const cursorX = buffer.x
      const cursorY = buffer.y
      const dims = renderer.dimensions
      if (dims && dims.css && dims.css.cell) {
        const cellWidth = dims.css.cell.width || 9
        const cellHeight = dims.css.cell.height || 17
        const x = cursorX * cellWidth
        const belowY = (cursorY + 1) * cellHeight + 16
        cursorPixelPos.value = { x, y: belowY }
      }
    } catch {
      const el = terminal.element
      if (el) {
        const rect = el.getBoundingClientRect()
        cursorPixelPos.value = { x: 0, y: rect.height }
      }
    }
  }

  function isAtLineEnd(): boolean {
    return cursorIndex.value >= lineBuffer.value.length
  }

  function handleInput(data: string) {
    if (options.mode !== 'ssh') return
    if (inAlternateScreen) return
    for (let i = 0; i < data.length; i++) {
      const char = data[i]
      const code = data.charCodeAt(i)
      if (char === '\r' || char === '\n') {
        // Save command to history before clearing.
        // Prefer terminal buffer (includes tab completion), fallback to lineBuffer.
        if (options.enableHistory !== false) {
          let command = getCurrentCommandFromTerminal()
          if (!command) {
            command = lineBuffer.value.trim()
          }
          if (command && options.onHistoryExtract) {
            options.onHistoryExtract(command)
          }
        }
        lineBuffer.value = ''
        cursorIndex.value = 0
        // Reset suggestion suppress on new command
        if (options.onResetSuppress) {
          options.onResetSuppress()
        }
      } else if (code === 127 || char === '\b') {
        if (cursorIndex.value > 0) {
          lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value - 1) + lineBuffer.value.slice(cursorIndex.value)
          cursorIndex.value--
        }
      } else if (code === 1) {
        // Ctrl+A — beginning of line
        cursorIndex.value = 0
      } else if (code === 5) {
        // Ctrl+E — end of line
        cursorIndex.value = lineBuffer.value.length
      } else if (code === 11) {
        // Ctrl+K — delete from cursor to end of line
        lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value)
      } else if (code === 21) {
        // Ctrl+U — delete from beginning to cursor
        lineBuffer.value = lineBuffer.value.slice(cursorIndex.value)
        cursorIndex.value = 0
      } else if (code === 27) {
        i++
        if (data[i] === '[') {
          i++
          let param = ''
          while (i < data.length && ((data[i] >= '0' && data[i] <= '9') || data[i] === ';')) {
            param += data[i]
            i++
          }
          const finalChar = data[i]
          if (finalChar === 'D') {
            // Left arrow
            if (cursorIndex.value > 0) cursorIndex.value--
          } else if (finalChar === 'C') {
            // Right arrow
            if (cursorIndex.value < lineBuffer.value.length) cursorIndex.value++
          } else if (finalChar === 'H' && param === '') {
            // Home
            cursorIndex.value = 0
          } else if (finalChar === 'F' && param === '') {
            // End
            cursorIndex.value = lineBuffer.value.length
          } else if (finalChar === '~') {
            if (param === '1' || param === '7') {
              // Home (alternate)
              cursorIndex.value = 0
            } else if (param === '4' || param === '8') {
              // End (alternate)
              cursorIndex.value = lineBuffer.value.length
            } else if (param === '3') {
              // Delete
              if (cursorIndex.value < lineBuffer.value.length) {
                lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value) + lineBuffer.value.slice(cursorIndex.value + 1)
              }
            }
          }
        }
      } else if (code >= 32) {
        // Support all printable characters including CJK
        lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value) + char + lineBuffer.value.slice(cursorIndex.value)
        cursorIndex.value++
      }
    }
    updateToken()
    // Defer cursor position update to next tick to avoid blocking rapid input
    // (getBoundingClientRect() inside updateCursorPosition forces synchronous layout)
    if (cursorPosTimer) {
      clearTimeout(cursorPosTimer)
    }
    cursorPosTimer = setTimeout(() => {
      cursorPosTimer = null
      updateCursorPosition()
    }, 0)
  }

  function handleSessionData(data: string) {
    if (options.mode !== 'ssh') return

    // Detect alternate screen buffer enter/exit (vim, k9s, less, etc.)
    if (data.includes('\x1b[?1049h') || data.includes('\x1b[?47h')) {
      inAlternateScreen = true
      return
    }
    if (data.includes('\x1b[?1049l') || data.includes('\x1b[?47l')) {
      inAlternateScreen = false
    }
  }

  function clearBuffer() {
    lineBuffer.value = ''
    cursorIndex.value = 0
    currentToken.value = ''
  }

  function isInAlternateScreen(): boolean {
    return inAlternateScreen
  }

  return {
    lineBuffer,
    cursorIndex,
    currentToken,
    cursorPixelPos,
    isAtLineEnd,
    handleInput,
    handleSessionData,
    clearBuffer,
    isInAlternateScreen,
  }
}
