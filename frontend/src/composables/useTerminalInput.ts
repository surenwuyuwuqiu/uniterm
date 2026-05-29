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
}

export function useTerminalInput(terminal: Terminal | null, options: UseTerminalInputOptions) {
  const lineBuffer = ref('')
  const cursorIndex = ref(0)
  const currentToken = ref('')
  const cursorPixelPos = ref<CursorPosition>({ x: 0, y: 0 })

  let outputBuffer = ''

  function extractCommandFromOutput(data: string): string | null {
    outputBuffer += data
    const lines = outputBuffer.split('\n')
    if (lines.length > 5) {
      outputBuffer = lines.slice(-5).join('\n')
    }

    const lastLine = lines[lines.length - 1]
    if (!lastLine) return null

    const match = lastLine.match(/(.+?[$#>\]])\s*(.+)/)
    if (match) {
      const command = match[2].trim()
      if (command && !command.includes('__AI_DONE_')) {
        return command
      }
    }
    return null
  }

  function updateToken() {
    const text = lineBuffer.value
    const idx = cursorIndex.value
    const beforeCursor = text.slice(0, idx)
    const tokens = beforeCursor.split(/\s+/)
    currentToken.value = tokens[tokens.length - 1] || ''
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
        cursorPixelPos.value = {
          x: cursorX * cellWidth,
          y: (cursorY + 1) * cellHeight + 4,
        }
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
    for (let i = 0; i < data.length; i++) {
      const char = data[i]
      const code = data.charCodeAt(i)
      if (char === '\r' || char === '\n') {
        lineBuffer.value = ''
        cursorIndex.value = 0
      } else if (code === 127 || char === '\b') {
        if (cursorIndex.value > 0) {
          lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value - 1) + lineBuffer.value.slice(cursorIndex.value)
          cursorIndex.value--
        }
      } else if (code === 27) {
        i++
        if (data[i] === '[') {
          i++
          while (i < data.length && (data[i] < 'A' || data[i] > 'Z') && (data[i] < 'a' || data[i] > 'z')) {
            i++
          }
        }
      } else if (code >= 32 && code <= 126) {
        lineBuffer.value = lineBuffer.value.slice(0, cursorIndex.value) + char + lineBuffer.value.slice(cursorIndex.value)
        cursorIndex.value++
      }
    }
    updateToken()
    updateCursorPosition()
  }

  function handleSessionData(data: string) {
    if (options.mode !== 'ssh') return
    const command = extractCommandFromOutput(data)
    if (command && options.onHistoryExtract) {
      options.onHistoryExtract(command)
    }
  }

  function clearBuffer() {
    lineBuffer.value = ''
    cursorIndex.value = 0
    currentToken.value = ''
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
  }
}
