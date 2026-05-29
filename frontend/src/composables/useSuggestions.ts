import { ref } from 'vue'
import { SaveTerminalHistory, LoadTerminalHistory } from '../../wailsjs/go/main/App'
import { chat } from '../services/llm'

export interface SuggestionItem {
  type: 'history' | 'ai-preview' | 'ai-result'
  label: string
  value: string
  icon?: string
  description?: string
}

export interface SuggestionsState {
  visible: boolean
  items: SuggestionItem[]
  selectedIndex: number
  loading: boolean
}

const MAX_HISTORY = 5000

const historyCache = new Set<string>()
let historyLoaded = false

export function useSuggestions() {
  const state = ref<SuggestionsState>({
    visible: false,
    items: [],
    selectedIndex: 0,
    loading: false,
  })

  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let currentAbortController: AbortController | null = null

  async function loadHistory(): Promise<string[]> {
    if (historyLoaded) {
      return Array.from(historyCache)
    }
    try {
      const commands = await LoadTerminalHistory()
      commands.forEach(cmd => historyCache.add(cmd))
      historyLoaded = true
      return Array.from(historyCache)
    } catch {
      return []
    }
  }

  async function saveHistory(commands: string[]) {
    try {
      await SaveTerminalHistory(commands)
    } catch {
      // Silent fail
    }
  }

  function addHistoryCommand(command: string) {
    if (!command || command.includes('__AI_DONE_')) return
    if (historyCache.has(command)) {
      historyCache.delete(command)
    }
    historyCache.add(command)
    if (historyCache.size > MAX_HISTORY) {
      const arr = Array.from(historyCache)
      historyCache.clear()
      arr.slice(-MAX_HISTORY).forEach(cmd => historyCache.add(cmd))
    }
    saveHistory(Array.from(historyCache))
  }

  function getHistorySuggestions(prefix: string): SuggestionItem[] {
    if (!prefix) return []
    const lowerPrefix = prefix.toLowerCase()
    const matches: SuggestionItem[] = []
    for (const cmd of historyCache) {
      if (cmd.toLowerCase().startsWith(lowerPrefix)) {
        matches.push({
          type: 'history',
          label: cmd,
          value: cmd,
          description: '历史',
        })
      }
    }
    return matches.slice(0, 10)
  }

  async function generateAISuggestion(currentInput: string): Promise<void> {
    if (!currentInput.trim()) return
    state.value.loading = true
    try {
      let aiResult = ''
      await chat({
        system: '你是终端命令助手。用户正在 SSH 终端中输入命令。请根据当前输入上下文，补全或改写为一个完整、正确的命令。只返回命令本身，不要添加解释、不要添加 markdown 代码块。',
        messages: [{ role: 'user', content: `当前输入: ${currentInput}` }],
        onChunk: (chunk: string) => {
          aiResult += chunk
        },
      })
      const cleaned = aiResult.trim().replace(/^```[\w]*\n?/, '').replace(/\n?```$/, '')
      if (cleaned) {
        const items = state.value.items.filter(item => item.type !== 'ai-preview')
        items.push({
          type: 'ai-result',
          label: cleaned,
          value: cleaned,
          description: 'AI',
        })
        state.value.items = items
        state.value.selectedIndex = items.length - 1
      }
    } catch {
      const items = state.value.items.filter(item => item.type !== 'ai-preview')
      items.push({
        type: 'ai-result',
        label: 'AI 转写失败',
        value: '',
        description: 'AI',
      })
      state.value.items = items
    } finally {
      state.value.loading = false
    }
  }

  async function updateSuggestions(token: string) {
    if (debounceTimer) {
      clearTimeout(debounceTimer)
    }
    if (!token) {
      state.value.visible = false
      state.value.items = []
      return
    }
    debounceTimer = setTimeout(async () => {
      const historyItems = getHistorySuggestions(token)
      const items: SuggestionItem[] = [...historyItems]
      items.push({
        type: 'ai-preview',
        label: 'AI 转写...',
        value: '',
        description: 'AI',
      })
      state.value.items = items
      state.value.selectedIndex = 0
      state.value.visible = items.length > 0
    }, 150)
  }

  function selectNext() {
    if (state.value.items.length === 0) return
    state.value.selectedIndex = (state.value.selectedIndex + 1) % state.value.items.length
  }

  function selectPrev() {
    if (state.value.items.length === 0) return
    state.value.selectedIndex = (state.value.selectedIndex - 1 + state.value.items.length) % state.value.items.length
  }

  function getSelectedItem(): SuggestionItem | null {
    if (state.value.items.length === 0) return null
    return state.value.items[state.value.selectedIndex]
  }

  function close() {
    state.value.visible = false
    state.value.items = []
    state.value.selectedIndex = 0
  }

  function isVisible(): boolean {
    return state.value.visible
  }

  return {
    state,
    loadHistory,
    addHistoryCommand,
    updateSuggestions,
    generateAISuggestion,
    selectNext,
    selectPrev,
    getSelectedItem,
    close,
    isVisible,
  }
}
