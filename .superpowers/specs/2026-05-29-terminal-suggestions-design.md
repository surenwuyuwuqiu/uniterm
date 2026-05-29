# 终端输入提示功能设计文档

**日期**: 2026-05-29  
**分支**: `feat/terminal-suggestions`  
**状态**: 设计中

---

## 1. 目标

在 uniTerm 的 SSH 终端中实现输入提示（Autocomplete）功能，支持：

1. **历史记录提示** — 前端从终端输出中实时提取历史命令
2. **AI 命令转写** — AI 根据当前输入生成完整命令，先预览再确认替换（仅 SSH）

---

## 2. 需求总结

| 需求项 | 决策 |
|---|---|
| 触发方式 | 输入时自动弹出（类似 IDE），光标在行尾时触发 |
| 历史记录来源 | 前端从终端输出中提取（WindTerm 方式），全局不分组 |
| AI 转写交互 | 两步确认：① 点击"AI 转写"生成结果并预览 ② 再次点击 AI 结果替换当前输入 |
| 键盘交互 | ↓/↑ 选择，Tab/Enter 确认，Esc 关闭 |
| 作用范围 | **仅 SSH 终端**，本地终端不提供提示功能 |

---

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         前端 (Vue + xterm.js)                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐  │
│  │ useTerminalInput │→│ useSuggestions  │→│ TerminalSuggestion│  │
│  │   (输入解析)      │  │   (建议管理)     │  │   (浮层渲染)      │  │
│  └─────────────────┘  └─────────────────┘  └──────────────────┘  │
│           ↑                    ↑                    ↑           │
│    xterm.onData             llm.ts             DOM 定位计算      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. 模块详细设计

### 4.1 useTerminalInput（前端 Composable）

**职责**: 拦截 xterm.js 输入事件，维护当前行输入 buffer，解析当前 token 和光标位置；同时从终端输出中提取历史命令。

**核心逻辑**:

```typescript
// 维护状态
interface InputState {
  lineBuffer: string      // 当前行从 prompt 后的完整输入
  cursorIndex: number     // 光标在 lineBuffer 中的位置
  currentToken: string    // 光标所在的 token（按空格分割）
  tokenType: 'command' | null
  cursorPixelPos: { x: number; y: number }  // 光标在屏幕上的像素位置
}

// 始终返回命令类型，因为目录提示已去掉
function detectTokenType(_token: string): 'command' | null {
  return 'command'
}
```

**历史命令提取**:
- 监听 `session:data` 事件，累积终端输出
- 使用简单的 prompt 检测正则（如匹配行尾的 `$`/`#`/`>`），提取 prompt 后的内容作为历史命令
- **过滤 AI 命令**：如果提取的内容包含 `__AI_DONE_` 标记（AI 助理执行命令的标记），跳过不记录
- 提取到新命令后，内存 `Set` 去重，直接调用 `SaveTerminalHistory` 实时持久化
- 启动时 `LoadTerminalHistory` 加载到内存，运行时查询纯内存操作
- 上限 5000 条，超出时淘汰最早的
- 仅当 `mode === 'ssh'` 时启用历史提取

**xterm 输入处理**:
- 监听 `terminal.onData`，累积字符到 `lineBuffer`
- 处理特殊字符：Backspace 删除、Enter 清空 buffer、Escape 序列忽略
- 通过 `(terminal as any)._core.buffer.x/y` 获取光标行列，结合字符尺寸计算像素位置

**关键约束**:
- 只在光标位于行尾时触发建议（避免行中间编辑时的干扰）
- 输入防抖 150ms 后再请求建议
- **仅 SSH 终端启用提示功能**，本地终端不启用

---

### 4.2 useSuggestions（前端 Composable）

**职责**: 根据当前 token 类型，调用对应数据源获取建议列表，管理缓存和状态。

```typescript
interface SuggestionItem {
  type: 'history' | 'ai-preview' | 'ai-result'
  label: string        // 显示文本
  value: string        // 实际替换值
  icon?: string        // 📜
  description?: string // 辅助说明（如"历史"）
}

interface SuggestionsState {
  visible: boolean
  items: SuggestionItem[]
  selectedIndex: number
  loading: boolean
}
```

**建议获取逻辑**:

```
用户输入变化（防抖后）
    │
    ▼
调用后端 `LoadTerminalHistory()` 加载历史
    │
    ├── 前缀匹配，标记 type='history'
    │
    └── 固定追加 SuggestionItem {
          type: 'ai-preview',
          label: 'AI 转写...',
          value: ''
        }
```

**存储策略**:
- 后端 `TerminalHistoryStore` 存 JSON 文件（`%APPDATA%/uniTerm/terminal-history.json`）
- 前端通过 Wails 绑定 `SaveTerminalHistory` / `LoadTerminalHistory` 读写
- 运行时前端内存维护 `Set<string>` 做快速前缀匹配
- 上限 5000 条，可被 sync 系统同步

---

### 4.3 TerminalSuggestion.vue（前端组件）

**职责**: 在终端容器内渲染绝对定位的提示浮层。

**模板结构**:

```vue
<template>
  <div
    v-show="visible"
    class="terminal-suggestion-popup"
    :style="popupStyle"
  >
    <div
      v-for="(item, index) in items"
      :key="index"
      class="suggestion-item"
      :class="{ selected: index === selectedIndex, 'ai-result': item.type === 'ai-result' }"
      @click="onSelect(index)"
      @mouseenter="selectedIndex = index"
    >
      <span class="suggestion-icon">{{ item.icon }}</span>
      <span class="suggestion-label">{{ item.label }}</span>
      <span v-if="item.description" class="suggestion-desc">{{ item.description }}</span>
    </div>
  </div>
</template>
```

**样式要点**:
- 位置：跟随 xterm 光标像素位置，在光标下方 4px 处
- 尺寸：最大高度 200px，超出可滚动
- 主题：与现有暗色主题一致，`var(--bg-surface)` 背景，`var(--border-subtle)` 边框
- AI 结果项特殊样式（如左侧绿色竖线标识）

**键盘事件**（在 `BaseTerminal.vue` 中通过 `terminal.attachCustomKeyEventHandler` 处理）:
- `ArrowDown` / `ArrowUp`：选择项上下移动
- `Tab` / `Enter`：确认当前选中项
- `Escape`：关闭提示框
- 当提示框可见时，这些按键事件被拦截，不传递给 xterm

---

### 4.4 后端存储：TerminalHistoryStore

**Go 实现**（参考现有 `AISessionStore` 模式）：

```go
package store

const terminalHistoryFileName = "terminal-history.json"

type TerminalHistoryStore struct {
    configDir string
}

type TerminalHistoryData struct {
    Commands []string `json:"commands"`
}

func (s *TerminalHistoryStore) Save(data TerminalHistoryData) error
func (s *TerminalHistoryStore) Load() (TerminalHistoryData, error)
```

**Wails 绑定**（在 `app.go` 中暴露）：

```go
func (a *App) SaveTerminalHistory(commands []string) error
func (a *App) LoadTerminalHistory() ([]string, error)
```

- 存储路径：`%APPDATA%/uniTerm/terminal-history.json`（Windows）或 `~/.config/uniTerm/terminal-history.json`（Linux/macOS）
- 上限 5000 条，超出时淘汰最早的

---

### 4.5 AI 转写交互流程（两步确认）

```
用户输入 "docker run -it ub"
    │
    ▼
提示框显示：
  📜 docker run -it ubuntu /bin/bash  （历史）
  AI 转写...                           （固定选项）
    │
    ▼ 用户点击/选择 "AI 转写..."
    ▼ 前端调用 llm.chat() 发送 prompt：
      system: "你是终端命令助手。根据当前输入补全为完整正确的命令。只返回命令本身，不要解释。"
      user: "当前输入: docker run -it ub"
    │
    ▼ AI 返回："docker run -it ubuntu:latest /bin/bash"
    ▼ 提示框更新为：
      📜 docker run -it ubuntu /bin/bash  （历史）
      docker run -it ubuntu:latest /bin/bash  （AI 结果，特殊样式）
        ↑ 标注为 type='ai-result'
    │
    ▼ 用户再次点击 AI 结果项
    ▼ 用 AI 结果替换当前整行输入
    ▼ 关闭提示框
```

**AI prompt 设计**:

```
system: 你是一个终端命令助手。用户正在 SSH 终端中输入命令。请根据当前输入上下文，
补全或改写为一个完整、正确的命令。只返回命令本身，不要添加解释、不要添加 markdown 代码块。

user: docker run -it ub
assistant: docker run -it ubuntu:latest /bin/bash
```

---

## 5. 数据流时序图

```
用户按键 ──→ BaseTerminal.onData ──→ useTerminalInput.updateBuffer()
                                              │
                                              ▼
                                       提取当前 token
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    ▼                                                     ▼
              后端 LoadTerminalHistory()                              type='ai-preview'
              前缀匹配筛选                                                 │
                    │                                                      ▼
                    ▼                                               用户选择后调用
              生成历史建议列表                                        llm.chat()
                    │                                                      │
                    └─────────────────────────┬────────────────────────────┘
                                              │
                                              ▼
                                    TerminalSuggestion.vue 渲染
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    ▼                         ▼                         ▼
                用户按 Tab                 用户按 Esc                 用户点击项
                    │                         │                         │
                    ▼                         ▼                         ▼
            替换当前 token              关闭提示框                 替换当前 token
            保持输入继续                                            保持输入继续
```

---

## 6. 新增/修改文件清单

### 前端

| 文件 | 操作 | 说明 |
|---|---|---|
| `frontend/src/composables/useTerminalInput.ts` | 新增 | 输入解析、光标位置计算 |
| `frontend/src/composables/useSuggestions.ts` | 新增 | 建议获取、缓存管理、状态 |
| `frontend/src/components/TerminalSuggestion.vue` | 新增 | 提示浮层 UI 组件 |
| `frontend/src/components/BaseTerminal.vue` | 修改 | 集成 useTerminalInput、useSuggestions、TerminalSuggestion；拦截键盘事件 |

### 后端

| 文件 | 操作 | 说明 |
|---|---|---|
| `backend/store/terminal_history_store.go` | 新增 | 终端历史命令本地文件存储 |
| `app.go` | 修改 | 新增 `SaveTerminalHistory`、`LoadTerminalHistory` Wails 绑定 |

---

## 7. 边界处理

| 场景 | 处理策略 |
|---|---|
| 光标不在行尾 | 不触发建议，避免干扰行中编辑 |
| SSH 未连接 | 提示功能静默失效，不影响正常输入 |
| 历史记录为空 | 只显示"AI 转写..."选项 |
| AI API 失败 | AI 结果项显示"AI 转写失败"，不影响历史建议 |
| 快速连续输入 | 防抖 150ms，取消未完成的请求 |
| 终端尺寸变化 | 关闭提示框，避免位置错位 |
| 切换 tab/panel | 关闭提示框，清空状态 |

---

## 8. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| xterm.js 内部 API 变化 | 光标位置计算失效 | 使用 try/catch 包装，失败时回退到终端左下角 |
| AI 调用延迟高 | 用户体验差 | AI 结果异步加载，先显示 loading 状态 |
| 自定义 prompt 导致历史提取失败 | 历史记录不准确 | 支持常见 prompt 模式，无法识别时不提取 |
| 终端输出量大 | 前端解析性能问题 | 只解析最后 N 行输出，超出部分丢弃 |
