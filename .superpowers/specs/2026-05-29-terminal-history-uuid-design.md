# 终端历史记录 UUID 与设置管理设计文档

**日期**: 2026-05-29  
**分支**: `feat/terminal-suggestions`  
**状态**: 已确认，待实现  

---

## 1. 目标

为 uniTerm 终端历史记录功能增加 UUID 标识，并支持在设置页面进行历史记录管理。

---

## 2. 需求确认

| 需求项 | 决策 |
|---|---|
| 数据结构 | `[]string` → `[]HistoryEntry{ID, Command}` |
| 旧数据兼容 | 无 UUID 则清空全部旧数据 |
| 最大历史数量 | 5000 → 500 |
| 相同命令去重 | 只保留最新的一条（覆盖旧 UUID） |
| 列表排序 | 按时间倒序，最新在前 |
| 保存策略 | 终端输入与删除操作共用同一个 500ms debounce |
| 设置页面计数 | 不显示 |

---

## 3. 数据结构设计

### 3.1 Go 后端

```go
// backend/store/terminal_history_store.go

const maxHistorySize = 500

type HistoryEntry struct {
    ID      string `json:"id"`
    Command string `json:"command"`
}

type TerminalHistoryData struct {
    Entries []HistoryEntry `json:"entries"`
}
```

### 3.2 前端 TypeScript

```typescript
// frontend/src/composables/useSuggestions.ts

export interface HistoryEntry {
  id: string
  command: string
}
```

---

## 4. API 变更

### 4.1 当前 API

```go
func (a *App) SaveTerminalHistory(commands []string) error
func (a *App) LoadTerminalHistory() ([]string, error)
```

### 4.2 变更后 API

```go
func (a *App) SaveTerminalHistory(entries []HistoryEntry) error
func (a *App) LoadTerminalHistory() ([]HistoryEntry, error)
func (a *App) DeleteTerminalHistoryEntry(ids []string) error  // 新增：批量删除
```

---

## 5. 后端存储逻辑

### 5.1 Save(entries []HistoryEntry)

1. 按 `Command` 去重，保留靠后的（最新的）
2. 超出 `maxHistorySize`（500）时，淘汰最前面的（最早的）
3. JSON 序列化后写入文件

### 5.2 Load() ([]HistoryEntry, error)

1. 读取 JSON 文件
2. 尝试 `json.Unmarshal` 到 `TerminalHistoryData`
3. 若解析失败（旧格式 `"commands"` 字段），**清空文件**并返回空切片
4. 若文件不存在，返回空切片

### 5.3 DeleteByIDs(ids []string) error

1. 先 `Load()` 当前数据
2. 过滤掉 `ids` 中包含的条目
3. 保存回文件

---

## 6. 前端缓存设计

### 6.1 缓存结构

```typescript
const MAX_HISTORY = 500

// key = command（便于去重），value = HistoryEntry
const historyCache = new Map<string, HistoryEntry>()
let historyLoaded = false
let saveDebounceTimer: ReturnType<typeof setTimeout> | null = null
```

### 6.2 核心方法

**addHistoryCommand(command: string)**
- 调用 `generateUUID()` 生成新 ID
- `historyCache.set(command, { id, command })`
- 若 `size > MAX_HISTORY`，删除第一个 key（最早插入的）
- 触发 debounce save（共用 500ms timer）

**removeHistoryCommandById(id: string)**
- 遍历 `historyCache`，找到 `entry.id === id` 的项
- 删除该项
- 触发 debounce save

**removeHistoryCommandsById(ids: string[])**
- 将 `ids` 转为 `Set` 做快速查找
- 遍历 `historyCache`，删除所有匹配项
- 触发 debounce save

**loadHistory(): Promise<HistoryEntry[]>**
- 若 `historyLoaded` 为 true，返回 `Array.from(historyCache.values())`
- 否则调用后端 `LoadTerminalHistory()`，填充 `historyCache`

### 6.3 搜索过滤（设置页面用）

```typescript
function filterHistory(query: string): HistoryEntry[] {
  const all = Array.from(historyCache.values())
  if (!query.trim()) return all.reverse()  // 最新在前
  const lower = query.toLowerCase()
  return all
    .filter(e => e.command.toLowerCase().includes(lower))
    .reverse()
}
```

---

## 7. 设置页面历史记录管理

### 7.1 新增分类

在 `SettingsTab.vue` 的分类导航中新增 `history`：

```typescript
{ key: 'history', label: t('settings.history'), icon: History }
```

### 7.2 列表功能

- **搜索框**：顶部实时过滤 `command`
- **列表项**：复选框 + 命令文本 + 右侧删除按钮（×）
- **表头**：全选复选框
- **底部**："批量删除"按钮（有选中项时可用）

### 7.3 交互逻辑

| 操作 | 行为 |
|---|---|
| 单条删除 | 点击 `×` → 调用 `removeHistoryCommandById(id)` → debounce save |
| 批量删除 | 选中多项 → 点击"批量删除" → 调用 `removeHistoryCommandsById(selectedIds)` → debounce save |
| 全选 | 点击表头复选框，切换全部选中/取消 |
| 搜索 | 实时过滤列表，不改变底层数据 |

### 7.4 数据加载

设置页面 `history` section 挂载时：
1. 调用 `useSuggestions().loadHistory()` 确保数据已加载
2. 监听 `historyCache` 变化（通过 `state` 或直接引用）更新列表

---

## 8. 终端提示框（TerminalSuggestion）调整

### 8.1 删除按钮传参变更

```vue
<!-- TerminalSuggestion.vue -->
<button class="delete-btn" @click.stop="onRemove(item.id)">×</button>
```

```typescript
// BaseTerminal.vue
@remove="(id: string) => suggestions.removeHistoryCommandById(id)"
```

### 8.2 列表渲染 key

历史建议项的 `key` 改为 `item.id`（原先用 `index`）：

```vue
<div v-for="item in historyItems" :key="item.id">
```

---

## 9. 新增/修改文件清单

### 后端

| 文件 | 操作 | 说明 |
|---|---|---|
| `backend/store/terminal_history_store.go` | 修改 | 数据结构、Save/Load 逻辑、新增 DeleteByIDs |
| `app.go` | 修改 | Wails 绑定方法签名变更，新增 DeleteTerminalHistoryEntry |

### 前端

| 文件 | 操作 | 说明 |
|---|---|---|
| `frontend/src/composables/useSuggestions.ts` | 修改 | 缓存结构改为 Map、MAX_HISTORY=500、删除方法改按 ID |
| `frontend/src/components/TerminalSuggestion.vue` | 修改 | 删除按钮传 id、列表 key 改用 id |
| `frontend/src/components/BaseTerminal.vue` | 修改 | @remove 绑定调整为传 id |
| `frontend/src/components/SettingsTab.vue` | 修改 | 新增 history 分类、历史记录管理 UI |
| `frontend/wailsjs/go/main/App.d.ts` | 自动生成 | Wails 绑定类型更新 |
| `frontend/wailsjs/go/main/App.js` | 自动生成 | Wails 绑定方法更新 |

---

## 10. 边界处理

| 场景 | 处理策略 |
|---|---|
| 旧格式 JSON（`{"commands": [...]}`） | `Load()` 解析失败 → 清空文件 → 返回空 |
| 文件不存在 | 返回空切片 |
| 删除时 ID 不存在 | 静默忽略 |
| 超出 500 条 | 删除最早插入的 |
| 设置页面打开时 cache 未加载 | 主动调用 `loadHistory()` |
| 快速连续删除 | 共用 debounce 500ms，合并为一次保存 |
| 终端输入和设置页面同时操作 | 共用同一 `historyCache`，数据一致 |

---

## 11. 实现顺序建议

1. **后端先行**：修改 `terminal_history_store.go` 数据结构和 API
2. **Wails 绑定**：运行 `wails generate bindings` 生成前端类型
3. **前端缓存**：修改 `useSuggestions.ts` 适配新结构
4. **提示框**：修改 `TerminalSuggestion.vue` 和 `BaseTerminal.vue` 的删除传参
5. **设置页面**：在 `SettingsTab.vue` 中新增 history 分类和管理功能
6. **端到端验证**：清理缓存后 `wails dev` 测试
