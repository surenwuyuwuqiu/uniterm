<template>
  <div class="base-terminal">
    <div ref="terminalRef" class="terminal-area" @contextmenu="menu.onContextMenu"></div>

    <!-- Search bar -->
    <div v-show="searchVisible" class="terminal-search-bar">
      <input
        ref="searchInputRef"
        v-model="searchText"
        class="search-input"
        :placeholder="t('terminal.searchPlaceholder')"
        @input="onSearchInput"
        @keydown.enter.prevent="onSearchNext"
        @keydown.escape="closeSearch"
      />
      <span class="search-count" v-if="searchText">{{ searchResultIndex + 1 }}/{{ searchResultCount || 0 }}</span>
      <button class="search-btn" @click="onSearchPrev" :title="t('terminal.searchPrev')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m18 15-6-6-6 6"/></svg>
      </button>
      <button class="search-btn" @click="onSearchNext" :title="t('terminal.searchNext')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 9 6 6 6-6"/></svg>
      </button>
      <button class="search-btn" @click="closeSearch" :title="t('terminal.searchClose')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
      </button>
    </div>

    <!-- Terminal context menu -->
    <div
      v-show="menu.menuVisible.value"
      class="context-menu"
      :style="menu.menuStyle.value"
      @click.stop
    >
      <div class="menu-item" :class="{ disabled: !menu.hasSelection.value }" @click="menu.askAI">
        {{ t('terminal.askAI') }}
      </div>
      <div class="menu-item" :class="{ disabled: !menu.hasSelection.value }" @click="menu.copySelection">
        {{ t('terminal.copy') }}
      </div>
      <div class="menu-item" :class="{ disabled: !menu.hasSelection.value }" @click="menu.copyAndPaste">
        {{ t('terminal.copyAndPaste') }}
      </div>
      <div class="menu-item" @click="menu.pasteFromClipboard">{{ t('terminal.paste') }}</div>
    </div>

    <!-- Terminal suggestions popup -->
    <TerminalSuggestion
      :visible="suggestions.state.value.visible"
      :items="suggestions.state.value.items"
      :selected-index="suggestions.state.value.selectedIndex"
      :cursor-x="terminalInput?.cursorPixelPos.value.x ?? 0"
      :cursor-y="terminalInput?.cursorPixelPos.value.y ?? 0"
      @select="(idx: number) => applySuggestion(suggestions.state.value.items[idx])"
      @hover="(idx: number) => suggestions.state.value.selectedIndex = idx"
      @remove="(id: string) => suggestions.removeHistoryCommandById(id)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { SessionWrite, SessionResize } from '../../wailsjs/go/main/App'
import { EventsOn, BrowserOpenURL } from '../../wailsjs/runtime'
import { useSettingsStore } from '../stores/settingsStore'
import { useSessionStore } from '../stores/sessionStore'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'
import { useTerminalMenu } from '../composables/useTerminalMenu'
import { useI18n } from '../i18n'
import { getXtermTheme } from '../composables/useTerminal'
import { useTerminalInput } from '../composables/useTerminalInput'
import { useSuggestions } from '../composables/useSuggestions'
import TerminalSuggestion from './TerminalSuggestion.vue'

const props = defineProps<{
  mode: 'ssh' | 'sftp' | 'local'
  sessionId: string | null | undefined
  onSessionStatus?: (status: string) => void
  broadcastActive?: boolean
  workspaceId?: string
  panelId?: string
}>()

const settingsStore = useSettingsStore()
const sessionStore = useSessionStore()
const tabStore = useTabStore()
const panelStore = usePanelStore()
const { t } = useI18n()

const terminalRef = ref<HTMLDivElement>()
const searchInputRef = ref<HTMLInputElement>()
const searchVisible = ref(false)
const suggestions = useSuggestions()
let terminalInput: ReturnType<typeof useTerminalInput> | null = null
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let searchAddon: SearchAddon | null = null
let resizeObserver: ResizeObserver | null = null
let intersectionObserver: IntersectionObserver | null = null
let unsubscribe: (() => void) | null = null
let statusUnsubscribe: (() => void) | null = null
let onDocumentMouseUp: (() => void) | null = null
let onDocumentMouseDown: ((e: MouseEvent) => void) | null = null

let resizeTimer: ReturnType<typeof setTimeout> | null = null
let isResizing = false
let splitResizing = false
let suppressResizeUntil = 0
let retryOnEnter = false

// Search state
const searchText = ref('')
const searchResultIndex = ref(0)
const searchResultCount = ref(0)

// SFTP line buffer
let inputBuffer = ''

function getTerminalOptions() {
  const ts = settingsStore.settings.terminal
  const themeName = ts.theme || 'dark'
  return {
    fontSize: ts.fontSize || 13,
    fontFamily: ts.fontFamily || 'Consolas, "Courier New", monospace',
    theme: getXtermTheme(themeName),
    cursorBlink: true,
    rightClickSelectsWord: false,
    scrollback: ts.maxHistoryLines || 2500,
    allowProposedApi: true
  }
}

function getSelection(): string {
  return terminal?.getSelection() || ''
}

async function applySuggestion(item: ReturnType<typeof suggestions.getSelectedItem>) {
  if (!item || !terminal || !terminalInput) return

  if (item.type === 'ai-preview') {
    // Step 1: Generate AI suggestion
    await suggestions.generateAISuggestion(terminalInput.lineBuffer.value)
    return
  }

  const currentLine = terminalInput.lineBuffer.value
  const currentToken = terminalInput.currentToken.value
  const sid = props.sessionId

  if (item.type === 'ai-result' || item.type === 'history') {
    // Replace entire line with Ctrl+U. Using backspaces only works when the
    // replacement is exactly the currentToken; for multi-token input (e.g.
    // "git che" → "git checkout") backspaces leave the earlier text behind.
    if (sid && currentLine) {
      SessionWrite(sid, '\x15')
      SessionWrite(sid, item.value)
    }
    terminalInput.lineBuffer.value = item.value
    terminalInput.cursorIndex.value = item.value.length
    terminalInput.currentToken.value = ''
  }

  suggestions.close()
}

function resize() {
  if (props.mode === 'ssh' || props.mode === 'local') {
    const sid = props.sessionId
    if (!terminal || !fitAddon || !sid) return
    const el = terminalRef.value
    if (!el) return

    const rect = el.getBoundingClientRect()
    let cellWidth = 0
    let cellHeight = 0
    try {
      const core = (terminal as any)._core
      const dims = core?._renderService?.dimensions
      if (dims) {
        cellWidth = dims.css?.cell?.width || 0
        cellHeight = dims.css?.cell?.height || 0
      }
    } catch {
      cellWidth = 0
      cellHeight = 0
    }

    if (cellWidth === 0 || cellHeight === 0) {
      fitAddon.fit()
      if (terminal.cols <= 0 || terminal.rows <= 0) return
      SessionResize(sid, terminal.cols, terminal.rows).catch(() => {})
      return
    }

    // Subtract scrollbar width so the canvas doesn't overlap the scrollbar.
    const scrollbarWidth = (terminal as any)._core?.viewport?.scrollBarWidth || 0
    const cols = Math.floor((rect.width - scrollbarWidth) / cellWidth)
    const rows = Math.floor(rect.height / cellHeight)
    const newCols = Math.max(2, cols)
    const newRows = Math.max(1, rows)

    if (terminal.cols !== newCols || terminal.rows !== newRows) {
      terminal.resize(newCols, newRows)
      SessionResize(sid, newCols, newRows).catch(() => {})
    }
  } else {
    fitAddon?.fit()
  }
}

function write(data: string) {
  terminal?.write(data)
}

function focus() {
  terminal?.focus()
}

function setRetryOnEnter(value: boolean) {
  retryOnEnter = value
}

function onWindowResize() {
  const el = terminalRef.value
  if (!el) return
  if (!isResizing) {
    isResizing = true
    el.classList.add('resizing')
  }
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    isResizing = false
    el.classList.remove('resizing')
    resize()
  }, 400)
}

function onSplitResizeStart() {
  splitResizing = true
}

function onSplitResizeEnd() {
  splitResizing = false
  if (resizeTimer) {
    clearTimeout(resizeTimer)
    resizeTimer = null
  }
  suppressResizeUntil = Date.now() + 200
  nextTick(() => {
    setTimeout(() => {
      void terminalRef.value?.offsetWidth
      resize()
    }, 0)
  })
}

onMounted(() => {
  if (!terminalRef.value) return

  terminal = new Terminal(getTerminalOptions())
  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)

  // Web links addon
  let hoverEl: HTMLDivElement | null = null
  const webLinksAddon = new WebLinksAddon(
    (event, uri) => {
      if (event.ctrlKey || event.metaKey) {
        BrowserOpenURL(uri)
      }
    },
    {
      hover(event, _text, _location) {
        if (!hoverEl) {
          hoverEl = document.createElement('div')
          hoverEl.className = 'xterm-link-tooltip'
          terminal!.element!.appendChild(hoverEl)
        }
        const rect = terminal!.element!.getBoundingClientRect()
        hoverEl.textContent = 'Ctrl + Click to open'
        hoverEl.style.left = (event.clientX - rect.left + 12) + 'px'
        hoverEl.style.top = (event.clientY - rect.top - 28) + 'px'
        hoverEl.style.display = 'block'
      },
      leave() {
        if (hoverEl) {
          hoverEl.style.display = 'none'
        }
      }
    }
  )
  terminal.loadAddon(webLinksAddon)

  // Search addon
  searchAddon = new SearchAddon()
  terminal.loadAddon(searchAddon)
  searchAddon.onDidChangeResults((e) => {
    searchResultIndex.value = e.resultIndex
    searchResultCount.value = e.resultCount
  })

  terminal.open(terminalRef.value)
  void terminalRef.value.offsetHeight
  fitAddon.fit()

  // Initialize terminal input handling for SSH
  if (props.mode === 'ssh') {
    const smartOn = settingsStore.settings.terminal.smartCompletion ?? true
    terminalInput = useTerminalInput(terminal, {
      mode: props.mode,
      sessionId: props.sessionId,
      enableHistory: smartOn,
      onHistoryExtract: (command: string) => {
        suggestions.addHistoryCommand(command)
      },
      onResetSuppress: () => {
        suggestions.resetSuppress()
      },
    })
    // Load history on startup only when smart completion is enabled
    if (smartOn) {
      suggestions.loadHistory()
    }
  }

  if (props.mode === 'ssh' || props.mode === 'local') {
    // Restore terminal content from session buffer
    const sid = props.sessionId
    if (sid) {
      const history = sessionStore.getData(sid)
      if (history) {
        terminal.write(history)
      }
    }
    // Force initial resize with retries — needed because cell dimensions
    // may not be available immediately.
    ;[100, 200, 400, 800, 1500].forEach(d => setTimeout(() => {
      if (!terminal || terminal.cols <= 0 || terminal.rows <= 0) {
        fitAddon?.fit()
      }
      const sessionId = props.sessionId
      if (sessionId && terminal && terminal.cols > 0 && terminal.rows > 0) {
        SessionResize(sessionId, terminal.cols, terminal.rows).catch(() => {})
      }
    }, d))
  }

  // Input handling
  terminal.onData((data) => {
    if (props.mode === 'ssh' || props.mode === 'local') {
      if (retryOnEnter && (data === '\r' || data === '\n')) {
        retryOnEnter = false
        if (props.onSessionStatus) {
          props.onSessionStatus('retry')
        }
        return
      }

      // Handle suggestions input (skip in alternate screen apps like vim/k9s)
      if (terminalInput && !terminalInput.isInAlternateScreen() && (props.mode === 'ssh' || props.mode === 'local')) {
        terminalInput.handleInput(data)

        // When suggestions are visible, intercept certain keys synchronously
        if (suggestions.isVisible()) {
          if (data === '\t') {
            const selected = suggestions.getSelectedItem()
            if (selected) {
              applySuggestion(selected)
              return
            }
          }
          if (data === '\r' || data === '\n') {
            const selected = suggestions.getSelectedItem()
            if (selected) {
              applySuggestion(selected)
              return
            }
          }
          if (data === '\x1b') {
            suggestions.close()
            return
          }
        }

        // Defer suggestion update/close so SessionWrite is not blocked
        setTimeout(() => {
          if (!terminalInput) return
          const smartOn = settingsStore.settings.terminal.smartCompletion ?? true
          if (!smartOn) {
            suggestions.close()
            return
          }
          if (terminalInput.isAtLineEnd() && terminalInput.currentToken.value) {
            suggestions.updateSuggestions(terminalInput.currentToken.value)
          } else {
            suggestions.close()
          }
        }, 0)
      } else if (terminalInput?.isInAlternateScreen()) {
        suggestions.close()
      }

      const sid = props.sessionId
      if (sid) {
        if (props.broadcastActive && props.workspaceId) {
          const tab = tabStore.tabs.find(t => t.id === props.workspaceId)
          if (tab && tab.type === 'workspace') {
            for (const pid of tab.panelIds) {
              const p = panelStore.getPanel(pid)
              if (p?.sessionId && (p.type === 'ssh' || p.type === 'local')) {
                SessionWrite(p.sessionId, data)
              }
            }
            return
          }
        }
        SessionWrite(sid, data)
      }
    } else {
      // SFTP line buffering
      for (let i = 0; i < data.length; i++) {
        const char = data[i]
        const code = data.charCodeAt(i)
        if (char === '\r' || char === '\n') {
          if (inputBuffer) {
            const sid = props.sessionId
            if (sid) {
              for (let j = 0; j < inputBuffer.length; j++) {
                terminal!.write('\b \b')
              }
              SessionWrite(sid, inputBuffer)
            }
            inputBuffer = ''
          }
        } else if (code === 127 || char === '\b') {
          if (inputBuffer.length > 0) {
            inputBuffer = inputBuffer.slice(0, -1)
            terminal!.write('\b \b')
          }
        } else if (code >= 32 && code <= 126) {
          inputBuffer += char
          terminal!.write(char)
        }
      }
    }
  })

  // Selection action: copy on mouse up
  onDocumentMouseUp = () => {
    if (settingsStore.settings.terminal.selectionAction === 'copy') {
      const text = terminal?.getSelection()
      if (text) {
        navigator.clipboard.writeText(text)
      }
    }
  }
  document.addEventListener('mouseup', onDocumentMouseUp)

  // Close suggestion popup when clicking outside
  onDocumentMouseDown = (event: MouseEvent) => {
    if (!suggestions.isVisible()) return
    const popupEl = document.querySelector('.terminal-suggestion-popup')
    if (popupEl && !popupEl.contains(event.target as Node)) {
      suggestions.close()
    }
  }
  document.addEventListener('mousedown', onDocumentMouseDown)

  // Session data
  unsubscribe = EventsOn('session:data', (payload: { id: string; data: string }) => {
    if (payload.id !== props.sessionId || !terminal) return
    if (props.mode === 'sftp') {
      const cleaned = payload.data.replace(/\x1b\]633;S[^\x07]*\x07/g, '')
      if (cleaned) {
        terminal.write(cleaned)
      }
    } else {
      // Extract history commands from SSH output
      if (props.mode === 'ssh' && terminalInput) {
        terminalInput.handleSessionData(payload.data)
        // Close suggestions if we entered an alternate screen app (vim, k9s, etc.)
        if (terminalInput.isInAlternateScreen()) {
          suggestions.close()
        }
      }
      terminal.write(payload.data)
      if (props.mode === 'ssh' && props.onSessionStatus) {
        // onSessionData is handled by the consumer via EventsOn if needed
      }
    }
  })

  // SSH/Local: session status events
  if (props.mode === 'ssh' || props.mode === 'local') {
    retryOnEnter = false
    statusUnsubscribe = EventsOn('session:status', (payload: { id: string; status: string }) => {
      if (payload.id !== props.sessionId) return
      if (payload.status === 'connected') {
        retryOnEnter = false
        if (props.onSessionStatus) {
          props.onSessionStatus(payload.status)
        }
        resize()
      } else if (payload.status === 'error') {
        retryOnEnter = true
        if (props.onSessionStatus) {
          props.onSessionStatus(payload.status)
        }
        terminal?.write('\r\n\x1b[31mConnection failed. Press Enter to retry.\x1b[0m\r\n')
      } else if (payload.status === 'disconnected') {
        retryOnEnter = true
        if (props.onSessionStatus) {
          props.onSessionStatus(payload.status)
        }
      } else {
        if (props.onSessionStatus) {
          props.onSessionStatus(payload.status)
        }
      }
    })
  }

  window.addEventListener('resize', onWindowResize)
  window.addEventListener('split:resize-start', onSplitResizeStart)
  window.addEventListener('split:resize-end', onSplitResizeEnd)
  window.addEventListener('terminal:open-search', openSearch)

  // Ctrl+F to open search
  terminal.attachCustomKeyEventHandler((e) => {
    if (e.ctrlKey && e.key === 'f' && e.type === 'keydown') {
      openSearch()
      return false
    }

    // Suggestion navigation (only on keydown, ignore keyup)
    if (suggestions.isVisible() && e.type === 'keydown') {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        suggestions.selectNext()
        return false
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        suggestions.selectPrev()
        return false
      }
      if (e.key === 'Tab') {
        const selected = suggestions.getSelectedItem()
        if (selected) {
          e.preventDefault()
          applySuggestion(selected)
          return false
        }
      }
      if (e.key === 'Enter') {
        // Only apply suggestion if user explicitly selected one with arrow keys
        const selected = suggestions.getSelectedItem()
        if (selected) {
          e.preventDefault()
          applySuggestion(selected)
          return false
        }
        // No selection: let xterm handle Enter normally (terminal command execution)
      }
      if (e.key === 'Escape') {
        suggestions.close()
        return false
      }
    }

    return true
  })

  resizeObserver = new ResizeObserver(() => {
    if (isResizing || splitResizing || Date.now() < suppressResizeUntil) return
    const el = terminalRef.value
    if (!el) return
    if (resizeTimer) clearTimeout(resizeTimer)
    resizeTimer = setTimeout(() => resize(), 150)
  })
  resizeObserver.observe(terminalRef.value)

  if (props.mode === 'ssh') {
    intersectionObserver = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          resize()
        }
      })
    })
    intersectionObserver.observe(terminalRef.value)
  }
})

// Watch sessionId changes to rebind session data
watch(() => props.sessionId, (newId) => {
  if (newId && terminal && (props.mode === 'ssh' || props.mode === 'local')) {
    const history = sessionStore.getData(newId)
    if (history) {
      terminal.write(history)
    }
    const delays = [200, 400, 600, 800, 1000, 1500, 2000]
    delays.forEach((delay) => {
      setTimeout(() => resize(), delay)
    })
  }
})

// ── Search ──
const searchDecoOptions = {
  matchBackground: '#515c6e',
  matchBorder: '#22d3ee',
  matchOverviewRuler: '#22d3ee',
  activeMatchBackground: '#22d3ee44',
  activeMatchBorder: '#22d3ee',
  activeMatchColorOverviewRuler: '#22d3ee',
}

function openSearch() {
  searchVisible.value = true
  nextTick(() => {
    searchInputRef.value?.focus()
    if (searchText.value) {
      searchInputRef.value?.select()
      if (searchAddon) {
        searchAddon.findNext(searchText.value, { decorations: searchDecoOptions })
      }
    }
  })
}

function closeSearch() {
  searchVisible.value = false
  searchText.value = ''
  searchResultIndex.value = 0
  searchResultCount.value = 0
  searchAddon?.clearDecorations()
}

function onSearchInput() {
  if (!searchAddon || !searchText.value) {
    searchResultIndex.value = 0
    searchResultCount.value = 0
    searchAddon?.clearDecorations()
    return
  }
  searchAddon.findNext(searchText.value, { incremental: true, decorations: searchDecoOptions })
}

function onSearchNext() {
  if (!searchAddon || !searchText.value) return
  searchAddon.findNext(searchText.value, { decorations: searchDecoOptions })
}

function onSearchPrev() {
  if (!searchAddon || !searchText.value) return
  searchAddon.findPrevious(searchText.value, { decorations: searchDecoOptions })
}

// Watch terminal settings changes
watch(() => settingsStore.settings.terminal, (ts) => {
  if (!terminal) return
  if (ts.fontSize) terminal.options.fontSize = ts.fontSize
  if (ts.fontFamily) terminal.options.fontFamily = ts.fontFamily
  if (ts.maxHistoryLines) terminal.options.scrollback = ts.maxHistoryLines
  if (ts.theme) terminal.options.theme = getXtermTheme(ts.theme)
  resize()
}, { deep: true })

onUnmounted(() => {
  resizeObserver?.disconnect()
  intersectionObserver?.disconnect()
  terminal?.dispose()
  unsubscribe?.()
  statusUnsubscribe?.()
  if (onDocumentMouseUp) {
    document.removeEventListener('mouseup', onDocumentMouseUp)
    onDocumentMouseUp = null
  }
  if (onDocumentMouseDown) {
    document.removeEventListener('mousedown', onDocumentMouseDown)
    onDocumentMouseDown = null
  }
  window.removeEventListener('resize', onWindowResize)
  window.removeEventListener('split:resize-start', onSplitResizeStart)
  window.removeEventListener('split:resize-end', onSplitResizeEnd)
  window.removeEventListener('terminal:open-search', openSearch)
  suggestions.close()
})

// Paste handling
function pasteToTerminal(text: string) {
  if (props.mode === 'sftp' && terminal) {
    for (const char of text) {
      const code = char.charCodeAt(0)
      if (code >= 32 && code <= 126) {
        inputBuffer += char
        terminal.write(char)
      }
    }
  }
}

async function pasteToSession(text: string) {
  if (props.mode === 'ssh' || props.mode === 'local') {
    const sid = props.sessionId
    if (sid) {
      SessionWrite(sid, text)
    }
  }
}

const menu = useTerminalMenu({
  getSelection,
  onPaste: async (text) => {
    if (props.mode === 'ssh' || props.mode === 'local') {
      await pasteToSession(text)
    } else {
      pasteToTerminal(text)
    }
  },
  onAskAI: (text) => {
    window.dispatchEvent(new CustomEvent('ai:ask', { detail: text }))
  },
})

defineExpose({
  getSelection,
  resize,
  focus,
  write,
  setRetryOnEnter,
})
</script>

<style scoped>
.base-terminal {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}
.terminal-area {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* Search bar */
.terminal-search-bar {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  gap: 4px;
  background: rgba(20, 23, 29, 0.88);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: var(--radius-md);
  padding: 4px 6px;
  z-index: 50;
}
.search-input {
  width: 160px;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-family: var(--font-ui);
  font-size: 12px;
  padding: 2px 4px;
}
.search-input::placeholder {
  color: var(--text-muted);
}
.search-count {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  min-width: 32px;
  text-align: center;
}
.search-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s;
}
.search-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}
.terminal-area :deep(.xterm) {
  width: 100%;
  height: 100%;
  display: block;
}
.terminal-area :deep(.xterm),
.terminal-area :deep(.xterm-viewport) {
  background: var(--bg-base);
}
.terminal-area :deep(.xterm-viewport) {
  overflow-y: scroll !important;
}
.terminal-area :deep(.xterm-viewport::-webkit-scrollbar) {
  width: 8px;
}
.terminal-area :deep(.xterm-viewport::-webkit-scrollbar-track) {
  background: var(--bg-elevated);
}
.terminal-area :deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: var(--scrollbar-thumb);
  border-radius: 10px;
}
.terminal-area :deep(.xterm-viewport::-webkit-scrollbar-thumb:hover) {
  background: var(--scrollbar-thumb-hover);
}

.context-menu {
  position: fixed;
  z-index: 9999;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  min-width: 120px;
  padding: 4px;
  backdrop-filter: blur(8px);
}

.menu-item {
  padding: 7px 14px;
  font-size: 12px;
  font-family: var(--font-ui);
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
  border-radius: var(--radius-sm);
  transition: all 0.1s ease;
}

.menu-item:hover:not(.disabled) {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.menu-item.disabled {
  color: var(--text-disabled);
  cursor: default;
}
</style>
