# 终端历史记录 UUID 与设置管理实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将终端历史记录从 `[]string` 升级为带 UUID 的 `[]HistoryEntry`，支持设置页面历史记录管理。

**Architecture:** 后端 Go 存储层变更数据结构并提供批量删除 API；前端 `useSuggestions` 缓存改为 `Map<string, HistoryEntry>`（key=command 便于去重）；设置页面新增独立分类用于历史记录列表展示、搜索、单条/批量删除。

**Tech Stack:** Go, Wails v2, Vue 3, TypeScript, Element Plus

---

## 文件结构

| 文件 | 操作 | 职责 |
|---|---|---|
| `backend/store/terminal_history_store.go` | 修改 | Go 层：HistoryEntry 结构、Save/Load/DeleteByIDs |
| `app.go` | 修改 | Wails 绑定：SaveTerminalHistory / LoadTerminalHistory 签名变更，新增 DeleteTerminalHistoryEntry |
| `frontend/src/composables/useSuggestions.ts` | 修改 | 前端缓存：Map<string, HistoryEntry>、UUID 生成、按 ID 删除、批量删除 |
| `frontend/src/components/TerminalSuggestion.vue` | 修改 | 提示框：列表 key 改用 id、删除按钮传 id |
| `frontend/src/components/BaseTerminal.vue` | 修改 | 绑定：@remove 回调改为接收 id |
| `frontend/src/components/SettingsTab.vue` | 修改 | 设置页面：新增 history 分类、历史记录列表 UI |
| `frontend/wailsjs/go/main/App.d.ts` | 自动生成 | Wails 绑定 TypeScript 类型 |
| `frontend/wailsjs/go/main/App.js` | 自动生成 | Wails 绑定 JS 方法 |

---

## Task 1: 后端存储层数据结构变更

**Files:**
- Modify: `backend/store/terminal_history_store.go`

- [ ] **Step 1: 修改常量和结构体**

  将 `maxHistorySize` 从 `5000` 改为 `500`，新增 `HistoryEntry`，修改 `TerminalHistoryData`。

  ```go
  const terminalHistoryFileName = "terminal-history.json"
  const maxHistorySize = 500

  type TerminalHistoryStore struct {
      configDir string
  }

  type HistoryEntry struct {
      ID      string `json:"id"`
      Command string `json:"command"`
  }

  type TerminalHistoryData struct {
      Entries []HistoryEntry `json:"entries"`
  }
  ```

- [ ] **Step 2: 修改 Save 方法**

  接收 `[]HistoryEntry`，按 `Command` 去重（保留最后一个/最新的），截断到 500 条。

  ```go
  func (s *TerminalHistoryStore) Save(entries []HistoryEntry) error {
      // Deduplicate by Command: keep last occurrence
      seen := make(map[string]bool)
      result := make([]HistoryEntry, 0, len(entries))
      for i := len(entries) - 1; i >= 0; i-- {
          entry := entries[i]
          if entry.Command == "" || seen[entry.Command] {
              continue
          }
          seen[entry.Command] = true
          result = append([]HistoryEntry{entry}, result...)
      }
      // Trim to max
      if len(result) > maxHistorySize {
          result = result[len(result)-maxHistorySize:]
      }
      data := TerminalHistoryData{Entries: result}
      jsonData, err := json.MarshalIndent(data, "", "  ")
      if err != nil {
          return err
      }
      return os.WriteFile(s.filePath(), jsonData, 0600)
  }
  ```

- [ ] **Step 3: 修改 Load 方法**

  返回 `[]HistoryEntry`。若 JSON 无 `entries` 字段（旧格式），清空文件并返回空切片。

  ```go
  func (s *TerminalHistoryStore) Load() ([]HistoryEntry, error) {
      fileData, err := os.ReadFile(s.filePath())
      if err != nil {
          if os.IsNotExist(err) {
              return []HistoryEntry{}, nil
          }
          return nil, err
      }
      var data TerminalHistoryData
      if err := json.Unmarshal(fileData, &data); err != nil {
          // Old format or corrupt: clear file
          _ = os.Remove(s.filePath())
          return []HistoryEntry{}, nil
      }
      // Defensive: if unmarshaled but Entries is nil/empty and file had content,
      // treat as old format (old format had "commands" not "entries")
      if len(data.Entries) == 0 && len(fileData) > 10 {
          var oldFormat struct {
              Commands []string `json:"commands"`
          }
          if err := json.Unmarshal(fileData, &oldFormat); err == nil && len(oldFormat.Commands) > 0 {
              _ = os.Remove(s.filePath())
              return []HistoryEntry{}, nil
          }
      }
      return data.Entries, nil
  }
  ```

- [ ] **Step 4: 新增 DeleteByIDs 方法**

  ```go
  func (s *TerminalHistoryStore) DeleteByIDs(ids []string) error {
      entries, err := s.Load()
      if err != nil {
          return err
      }
      idSet := make(map[string]bool)
      for _, id := range ids {
          idSet[id] = true
      }
      filtered := make([]HistoryEntry, 0, len(entries))
      for _, entry := range entries {
          if !idSet[entry.ID] {
              filtered = append(filtered, entry)
          }
      }
      return s.Save(filtered)
  }
  ```

- [ ] **Step 5: Commit**

  ```bash
  git add backend/store/terminal_history_store.go
  git commit -m "feat(history): add UUID to HistoryEntry, change max to 500, add DeleteByIDs"
  ```

---

## Task 2: Wails 绑定方法签名变更

**Files:**
- Modify: `app.go`

- [ ] **Step 1: 修改 SaveTerminalHistory 和 LoadTerminalHistory**

  将 `commands []string` 改为 `entries []store.HistoryEntry`，返回类型同步变更。

  ```go
  func (a *App) SaveTerminalHistory(entries []store.HistoryEntry) error {
      if a.terminalHistoryStore == nil {
          return fmt.Errorf("terminal history store not initialized")
      }
      return a.terminalHistoryStore.Save(entries)
  }

  func (a *App) LoadTerminalHistory() ([]store.HistoryEntry, error) {
      if a.terminalHistoryStore == nil {
          return []store.HistoryEntry{}, fmt.Errorf("terminal history store not initialized")
      }
      return a.terminalHistoryStore.Load()
  }
  ```

- [ ] **Step 2: 新增 DeleteTerminalHistoryEntry**

  ```go
  func (a *App) DeleteTerminalHistoryEntry(ids []string) error {
      if a.terminalHistoryStore == nil {
          return fmt.Errorf("terminal history store not initialized")
      }
      return a.terminalHistoryStore.DeleteByIDs(ids)
  }
  ```

- [ ] **Step 3: Commit**

  ```bash
  git add app.go
  git commit -m "feat(history): update Wails bindings for HistoryEntry with UUID, add DeleteTerminalHistoryEntry"
  ```

---

## Task 3: 重新生成 Wails 前端绑定

**Files:**
- Auto-generate: `frontend/wailsjs/go/main/App.d.ts`
- Auto-generate: `frontend/wailsjs/go/main/App.js`

- [ ] **Step 1: 运行绑定生成命令**

  ```bash
  wails generate bindings
  ```

  验证 `frontend/wailsjs/go/main/App.d.ts` 中出现 `HistoryEntry` 类型和新的方法签名：
  - `SaveTerminalHistory(arg1: store.HistoryEntry[]): Promise<void>`
  - `LoadTerminalHistory(): Promise<store.HistoryEntry[]>`
  - `DeleteTerminalHistoryEntry(arg1: string[]): Promise<void>`

- [ ] **Step 2: Commit**

  ```bash
  git add frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js
  git commit -m "chore(bindings): regenerate Wails bindings for HistoryEntry UUID APIs"
  ```

---

## Task 4: 前端缓存结构变更

**Files:**
- Modify: `frontend/src/composables/useSuggestions.ts`

- [ ] **Step 1: 修改接口和常量**

  ```typescript
  import { SaveTerminalHistory, LoadTerminalHistory, DeleteTerminalHistoryEntry } from '../../wailsjs/go/main/App'

  export interface HistoryEntry {
    id: string
    command: string
  }

  const MAX_HISTORY = 500
  const MAX_COMMAND_LENGTH = 200

  const historyCache = new Map<string, HistoryEntry>() // key = command
  let historyLoaded = false
  ```

- [ ] **Step 2: 新增 UUID 生成函数**

  在文件顶部（import 之后）添加：

  ```typescript
  function generateUUID(): string {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = Math.random() * 16 | 0
      const v = c === 'x' ? r : (r & 0x3 | 0x8)
      return v.toString(16)
    })
  }
  ```

- [ ] **Step 3: 修改 loadHistory**

  ```typescript
  async function loadHistory(): Promise<HistoryEntry[]> {
    if (historyLoaded) {
      return Array.from(historyCache.values())
    }
    try {
      const entries = await LoadTerminalHistory()
      historyCache.clear()
      entries.forEach(entry => {
        if (entry.command) {
          historyCache.set(entry.command, entry)
        }
      })
      historyLoaded = true
      return Array.from(historyCache.values())
    } catch {
      return []
    }
  }
  ```

- [ ] **Step 4: 修改 saveHistory**

  ```typescript
  async function saveHistory(entries: HistoryEntry[]) {
    try {
      await SaveTerminalHistory(entries)
    } catch (e) {
      console.error('Failed to save terminal history:', e)
    }
  }
  ```

- [ ] **Step 5: 修改 addHistoryCommand**

  生成 UUID，按 command 覆盖去重，超出 MAX_HISTORY 删除最早的。

  ```typescript
  function addHistoryCommand(command: string) {
    if (shouldSkipCommand(command)) return
    historyCache.set(command, { id: generateUUID(), command })
    if (historyCache.size > MAX_HISTORY) {
      const firstKey = historyCache.keys().next().value
      if (firstKey !== undefined) {
        historyCache.delete(firstKey)
      }
    }
    if (saveDebounceTimer) {
      clearTimeout(saveDebounceTimer)
    }
    saveDebounceTimer = setTimeout(() => {
      saveDebounceTimer = null
      saveHistory(Array.from(historyCache.values()))
    }, 500)
  }
  ```

- [ ] **Step 6: 修改 removeHistoryCommand 为按 id 删除**

  删除旧的 `removeHistoryCommand(command: string)`，替换为按 id 删除，同时提供按 command 删除的便捷方法（内部转 id）。

  ```typescript
  function removeHistoryCommandById(id: string) {
    for (const [cmd, entry] of historyCache) {
      if (entry.id === id) {
        historyCache.delete(cmd)
        break
      }
    }
    // Update visible items
    state.value.items = state.value.items.filter(item => {
      if (item.type !== 'history') return true
      return !Array.from(historyCache.values()).some(e => e.command === item.value)
    })
    if (state.value.selectedIndex >= state.value.items.length) {
      state.value.selectedIndex = state.value.items.length - 1
    }
    if (state.value.items.every(item => item.type !== 'history')) {
      state.value.visible = false
    }
    if (saveDebounceTimer) {
      clearTimeout(saveDebounceTimer)
    }
    saveDebounceTimer = setTimeout(() => {
      saveDebounceTimer = null
      saveHistory(Array.from(historyCache.values()))
    }, 500)
  }

  function removeHistoryCommandsById(ids: string[]) {
    const idSet = new Set(ids)
    for (const [cmd, entry] of historyCache) {
      if (idSet.has(entry.id)) {
        historyCache.delete(cmd)
      }
    }
    state.value.items = state.value.items.filter(item => {
      if (item.type !== 'history') return true
      return !Array.from(historyCache.values()).some(e => e.command === item.value)
    })
    if (state.value.selectedIndex >= state.value.items.length) {
      state.value.selectedIndex = state.value.items.length - 1
    }
    if (state.value.items.every(item => item.type !== 'history')) {
      state.value.visible = false
    }
    if (saveDebounceTimer) {
      clearTimeout(saveDebounceTimer)
    }
    saveDebounceTimer = setTimeout(() => {
      saveDebounceTimer = null
      saveHistory(Array.from(historyCache.values()))
    }, 500)
  }
  ```

- [ ] **Step 7: 修改 getHistorySuggestions**

  `HistoryEntry` 的 `command` 字段替代之前的 `string`。

  ```typescript
  function getHistorySuggestions(prefix: string): SuggestionItem[] {
    if (!prefix) return []
    const lowerPrefix = prefix.toLowerCase()
    const matches: SuggestionItem[] = []
    const entries = Array.from(historyCache.values()).reverse()

    // First pass: exact prefix matches
    for (const entry of entries) {
      if (entry.command.length > MAX_COMMAND_LENGTH) continue
      if (entry.command.toLowerCase().startsWith(lowerPrefix)) {
        const indices: number[] = []
        for (let i = 0; i < lowerPrefix.length && i < entry.command.length; i++) {
          indices.push(i)
        }
        matches.push({
          type: 'history',
          label: entry.command,
          value: entry.command,
          description: '历史',
          matchIndices: indices,
        })
      }
    }

    // Second pass: fuzzy matches
    for (const entry of entries) {
      if (entry.command.length > MAX_COMMAND_LENGTH) continue
      if (matches.some(m => m.value === entry.command)) continue
      const indices = getFuzzyMatchIndices(entry.command, lowerPrefix)
      if (indices.length === lowerPrefix.length) {
        matches.push({
          type: 'history',
          label: entry.command,
          value: entry.command,
          description: '历史',
          matchIndices: indices,
        })
      }
    }

    return matches.slice(0, 10)
  }
  ```

- [ ] **Step 8: 更新 return 对象**

  ```typescript
  return {
    state,
    loadHistory,
    addHistoryCommand,
    removeHistoryCommandById,
    removeHistoryCommandsById,
    updateSuggestions,
    generateAISuggestion,
    selectNext,
    selectPrev,
    getSelectedItem,
    close,
    suppress,
    isVisible,
    resetSuppress,
  }
  ```

- [ ] **Step 9: Commit**

  ```bash
  git add frontend/src/composables/useSuggestions.ts
  git commit -m "feat(suggestions): migrate history cache to Map<string, HistoryEntry> with UUID"
  ```

---

## Task 5: 终端提示框调整

**Files:**
- Modify: `frontend/src/components/TerminalSuggestion.vue`

- [ ] **Step 1: 修改删除按钮传参**

  将 `@click.stop="onRemove(item.value)"` 改为 `@click.stop="onRemove(item.id)"`。但 `SuggestionItem` 没有 `id` 字段，需要把 `HistoryEntry` 的 id 带到列表项中。

  由于 `SuggestionItem` 目前结构里没有 id，最简做法是在 TerminalSuggestion 里通过 `item.value`（即 command）反向查找 id？不，更直接的做法是给 `SuggestionItem` 加一个可选的 `id` 字段。

  **先在 useSuggestions.ts 的 SuggestionItem 接口中加 id：**

  ```typescript
  export interface SuggestionItem {
    type: 'history' | 'ai-preview' | 'ai-result'
    label: string
    value: string
    icon?: string
    description?: string
    matchIndices?: number[]
    id?: string  // For history items only
  }
  ```

  **然后在 getHistorySuggestions 中填充 id：**

  ```typescript
  matches.push({
    type: 'history',
    label: entry.command,
    value: entry.command,
    id: entry.id,  // <-- 新增
    description: '历史',
    matchIndices: indices,
  })
  ```

  **最后修改 TerminalSuggestion.vue：**

  ```vue
  <button class="delete-btn" @click.stop="onRemove(item.id)">×</button>
  ```

  以及 emit 类型：

  ```typescript
  const emit = defineEmits<{
    select: [index: number]
    hover: [index: number]
    remove: [id: string]  // 改为 string (id)
  }>()

  function onRemove(id: string) {
    emit('remove', id)
  }
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add frontend/src/composables/useSuggestions.ts frontend/src/components/TerminalSuggestion.vue
  git commit -m "feat(ui): pass id instead of command for history removal in suggestion popup"
  ```

---

## Task 6: BaseTerminal 绑定调整

**Files:**
- Modify: `frontend/src/components/BaseTerminal.vue`

- [ ] **Step 1: 修改 @remove 绑定**

  ```vue
  @remove="(id: string) => suggestions.removeHistoryCommandById(id)"
  ```

- [ ] **Step 2: Commit**

  ```bash
  git add frontend/src/components/BaseTerminal.vue
  git commit -m "feat(terminal): update suggestion removal binding to pass id"
  ```

---

## Task 7: 设置页面历史记录管理

**Files:**
- Modify: `frontend/src/components/SettingsTab.vue`

- [ ] **Step 1: 导入 useSuggestions 和 History icon**

  ```typescript
  import { useSuggestions } from '../composables/useSuggestions'
  import { History } from '@lucide/vue'
  ```

- [ ] **Step 2: 新增 history 分类**

  ```typescript
  const categories = computed(() => {
    void settingsStore.settings.language
    return [
      { key: 'basic', label: t('settings.basic'), icon: Settings },
      { key: 'terminal', label: t('settings.terminal'), icon: Monitor },
      { key: 'ai', label: t('settings.ai'), icon: MessageCircleMore },
      { key: 'sync', label: t('settings.sync'), icon: RefreshCw },
      { key: 'history', label: t('settings.history'), icon: History },
      { key: 'about', label: t('settings.about'), icon: Info },
    ]
  })
  ```

  同时更新 watch 中的白名单：

  ```typescript
  watch(() => settingsStore.openCategory, (cat) => {
    if (cat && (cat === 'basic' || cat === 'terminal' || cat === 'ai' || cat === 'sync' || cat === 'history' || cat === 'about')) {
      settingsStore.activeCategory = cat
      settingsStore.openCategory = null
    }
  })
  ```

- [ ] **Step 3: 新增 history section 的响应式状态**

  ```typescript
  const suggestions = useSuggestions()
  const historySearch = ref('')
  const historySelectedIds = ref<Set<string>>(new Set())

  const historyEntries = computed(() => {
    const all = Array.from(suggestions.loadHistory())
    const query = historySearch.value.trim().toLowerCase()
    if (!query) return all.reverse()
    return all.filter(e => e.command.toLowerCase().includes(query)).reverse()
  })

  const isAllHistorySelected = computed(() => {
    if (historyEntries.value.length === 0) return false
    return historyEntries.value.every(e => historySelectedIds.value.has(e.id))
  })

  function toggleSelectAllHistory() {
    if (isAllHistorySelected.value) {
      historySelectedIds.value.clear()
    } else {
      historyEntries.value.forEach(e => historySelectedIds.value.add(e.id))
    }
  }

  function toggleHistorySelection(id: string) {
    if (historySelectedIds.value.has(id)) {
      historySelectedIds.value.delete(id)
    } else {
      historySelectedIds.value.add(id)
    }
  }

  async function deleteSelectedHistory() {
    const ids = Array.from(historySelectedIds.value)
    if (ids.length === 0) return
    suggestions.removeHistoryCommandsById(ids)
    historySelectedIds.value.clear()
  }

  function deleteHistoryItem(id: string) {
    suggestions.removeHistoryCommandById(id)
    historySelectedIds.value.delete(id)
  }
  ```

  **注意**：`suggestions.loadHistory()` 在 computed 中每次都会调用，但由于内部有 `historyLoaded` 缓存，实际只会在首次调用时走 Wails。但 computed 不应该有副作用，更好的做法是：

  ```typescript
  const historyList = ref<HistoryEntry[]>([])

  async function refreshHistory() {
    historyList.value = await suggestions.loadHistory()
  }

  const historyEntries = computed(() => {
    const query = historySearch.value.trim().toLowerCase()
    if (!query) return [...historyList.value].reverse()
    return historyList.value.filter(e => e.command.toLowerCase().includes(query)).reverse()
  })
  ```

  在挂载 history section 时调用 `refreshHistory()`。

  为了简化，用 `watch` 在 `activeCategory === 'history'` 时触发加载：

  ```typescript
  watch(() => settingsStore.activeCategory, async (cat) => {
    if (cat === 'history') {
      historyList.value = await suggestions.loadHistory()
    }
  })
  ```

- [ ] **Step 4: 添加 history section 模板**

  在 `SettingsTab.vue` 的 template 中，about section 之前插入：

  ```vue
  <!-- 历史记录管理 -->
  <div v-if="settingsStore.activeCategory === 'history'" class="settings-section">
    <h2 class="section-title">{{ t('settings.history') }}</h2>

    <!-- Search -->
    <div class="history-search-bar">
      <el-input
        v-model="historySearch"
        :placeholder="t('settings.historySearchPlaceholder')"
        prefix-icon="Search"
        clearable
        size="small"
      />
    </div>

    <!-- History list -->
    <div class="history-list-container">
      <div class="history-list-header">
        <el-checkbox
          :model-value="isAllHistorySelected"
          :indeterminate="historySelectedIds.size > 0 && !isAllHistorySelected"
          @change="toggleSelectAllHistory"
        />
        <span class="history-header-label">{{ t('settings.historyCommand') }}</span>
      </div>

      <div class="history-list-body">
        <div
          v-for="entry in historyEntries"
          :key="entry.id"
          class="history-item"
        >
          <el-checkbox
            :model-value="historySelectedIds.has(entry.id)"
            @change="toggleHistorySelection(entry.id)"
          />
          <span class="history-command">{{ entry.command }}</span>
          <el-button
            link
            size="small"
            type="danger"
            @click="deleteHistoryItem(entry.id)"
          >
            <el-icon><Trash2 :size="14" /></el-icon>
          </el-button>
        </div>

        <div v-if="historyEntries.length === 0" class="history-empty">
          {{ t('settings.historyEmpty') }}
        </div>
      </div>

      <!-- Batch actions -->
      <div v-if="historySelectedIds.size > 0" class="history-batch-actions">
        <el-button size="small" type="danger" @click="deleteSelectedHistory">
          {{ t('settings.historyBatchDelete', { count: historySelectedIds.size }) }}
        </el-button>
      </div>
    </div>
  </div>
  ```

  **样式**（加在 `<style scoped>` 末尾）：

  ```css
  .history-search-bar {
    margin-bottom: 12px;
  }
  .history-search-bar .el-input {
    width: 100%;
  }
  .history-list-container {
    background: var(--bg-surface);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .history-list-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--bg-hover);
    border-bottom: 1px solid var(--border-subtle);
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
  }
  .history-header-label {
    flex: 1;
  }
  .history-list-body {
    max-height: 400px;
    overflow-y: auto;
  }
  .history-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 14px;
    border-bottom: 1px solid var(--border-subtle);
    font-family: var(--font-mono);
    font-size: 12px;
    transition: background 0.1s ease;
  }
  .history-item:last-child {
    border-bottom: none;
  }
  .history-item:hover {
    background: var(--bg-hover);
  }
  .history-command {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-primary);
  }
  .history-empty {
    padding: 24px;
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }
  .history-batch-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: 10px 14px;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-hover);
  }
  ```

  **注意**：需要在 script imports 中加入 `Trash2`：

  ```typescript
  import { Settings, Monitor, MessageCircleMore, Info, RefreshCw, Pencil, Trash2, History } from '@lucide/vue'
  ```

- [ ] **Step 5: Commit**

  ```bash
  git add frontend/src/components/SettingsTab.vue
  git commit -m "feat(settings): add history management section with search, select, and batch delete"
  ```

---

## Task 8: 端到端验证

**Files:** N/A（验证任务）

- [ ] **Step 1: 清理前端缓存并启动**

  ```bash
  cd frontend && rm -rf dist node_modules/.vite && cd .. && wails dev
  ```

- [ ] **Step 2: 验证终端输入提示**

  1. SSH 连接到一个服务器
  2. 输入几个命令，观察悬浮提示框是否正常显示历史建议
  3. 点击悬浮框中某条历史右侧的 `×`，确认该条历史被删除且不再出现
  4. 关闭并重启应用，确认历史记录持久化正常

- [ ] **Step 3: 验证设置页面历史管理**

  1. 打开设置页面，切换到"历史记录"分类
  2. 确认列表展示所有历史命令（最新在前）
  3. 在搜索框输入关键词，确认实时过滤
  4. 勾选几条记录，点击"批量删除"，确认选中项被移除
  5. 点击单条记录右侧的删除按钮，确认即时移除
  6. 关闭并重启应用，确认删除操作已持久化

- [ ] **Step 4: 验证旧数据清空**

  1. 手动构造一个旧格式文件 `%APPDATA%/uniTerm/terminal-history.json`：
     ```json
     {"commands": ["ls", "pwd", "git status"]}
     ```
  2. 重启应用
  3. 确认旧文件被清空，设置页面历史记录为空

---

## Self-Review Checklist

### 1. Spec Coverage

| Spec 需求 | 对应 Task |
|-----------|-----------|
| 每条历史记录加 UUID | Task 1 (Go), Task 4 (前端 generateUUID) |
| 旧数据不兼容则清空 | Task 1 (Load 方法) |
| 最大历史数量 5000→500 | Task 1 (常量), Task 4 (MAX_HISTORY) |
| 设置页面历史记录管理 | Task 7 |
| 列表展示、搜索、单条删除、批量复选、批量删除 | Task 7 |
| 去重保留最新 | Task 1 (Save), Task 4 (addHistoryCommand Map.set 覆盖) |
| 最新在前 | Task 7 (reverse()) |
| 共用 debounce | Task 4 (remove 复用 saveDebounceTimer) |

### 2. Placeholder Scan

- ✅ 无 TBD/TODO
- ✅ 无 "add appropriate error handling"
- ✅ 无 "similar to Task N"
- ✅ 所有代码块完整

### 3. Type Consistency

- ✅ `HistoryEntry` 结构：Go `{ID, Command}` ↔ TS `{id, command}`
- ✅ `SaveTerminalHistory` 参数：`[]store.HistoryEntry`
- ✅ `LoadTerminalHistory` 返回：`[]store.HistoryEntry`
- ✅ `DeleteTerminalHistoryEntry` 参数：`ids []string`
- ✅ 前端 `SuggestionItem.id?: string` 与 `HistoryEntry.id` 对应
- ✅ `removeHistoryCommandById(id: string)` 与 emit `remove: [id: string]` 对应
