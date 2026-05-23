# VNC 远程桌面 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 uniTerm 新增 VNC 远程桌面连接功能，使用 noVNC 前端渲染 + Go WebSocket↔TCP 桥接，支持跨平台（Windows/macOS/Linux）和剪贴板双向同步。

**Architecture:** Go 后端为每个 VNC 会话启动一个独立的 `VNCProxy`（WebSocket→TCP 桥接，监听 127.0.0.1:随机端口）。前端通过 `@novnc/novnc` 的 `RFB` 类连接本地 WebSocket，处理 RFB 协议、Canvas 渲染和输入事件。会话生命周期和 UI 状态管理复用现有 SSH/RDP 的模式。

**Tech Stack:** Go 1.23, Vue 3, Wails v2, `@novnc/novnc`, `github.com/gorilla/websocket`

---

## File Structure

### Backend (Go)

| File | Action | Responsibility |
|---|---|---|
| `backend/session/vnc_proxy.go` | Create | WebSocket↔TCP 桥接器：每个会话独立实例，监听本地随机端口，双向 goroutine 转发数据 |
| `backend/session/vnc_session.go` | Create | 实现 `Session` 接口：管理 VNCProxy 生命周期，通过 `session:status` 事件向前端发送 `proxyAddr` |
| `backend/session/manager.go` | Modify | `Create()` switch 增加 `case "vnc"` |
| `app.go` | Modify | `CreateSession` 的 `SetOnStatusChangeCallback` 中为 VNC 会话附加 `proxyAddr` |

### Frontend (Vue)

| File | Action | Responsibility |
|---|---|---|
| `frontend/package.json` | Modify | 添加 `"@novnc/novnc": "^1.5.0"` 依赖 |
| `frontend/src/types/session.ts` | Modify | `ConnectionConfig.type` 扩展为 `'ssh' \| 'rdp' \| 'vnc'` |
| `frontend/src/types/workspace.ts` | Modify | `PanelType` 增加 `'vnc'`，新增 `VNCTab` 接口，`Tab` 联合类型扩展 |
| `frontend/src/components/VNCTabContent.vue` | Create | VNC 连接内容组件：状态 UI + noVNC RFB 初始化 + 剪贴板同步 |
| `frontend/src/components/ConnectionForm.vue` | Modify | 类型选择加 VNC 按钮；VNC 时显示主机/端口/密码，默认端口 5900 |
| `frontend/src/components/Sidebar.vue` | Modify | 右键菜单加"连接 VNC"；双击 VNC 类型连接时走 VNC 流程 |
| `frontend/src/stores/tabStore.ts` | Modify | 新增 `createVNCTab()` 工厂函数；`closeTab` 中 `removedPanelIds` 收集包含 VNC |
| `frontend/src/stores/panelStore.ts` | Modify | `createPanel` 已支持任意 `PanelType`，无需改动（参数类型是 `Panel['type']`） |
| `frontend/src/App.vue` | Modify | 渲染 `vnc` tab 的 `VNCTabContent`；`closeTab` 清理 VNC session；`onConnectVNC` 方法 |
| `frontend/src/components/TabBar.vue` | Modify | TabItem v-if 包含 `'vnc'`；关闭 VNC tab 时调用 `CloseSession`（无需 RDPHide） |
| `frontend/src/i18n/index.ts` | Modify | 新增 VNC 相关文案（zh/en） |

---

## Task 1: Backend — VNCProxy (WebSocket↔TCP Bridge)

**Files:**
- Create: `backend/session/vnc_proxy.go`

- [ ] **Step 1: Write VNCProxy implementation**

```go
package session

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// VNCProxy bridges WebSocket (frontend noVNC) to TCP (VNC server).
// One instance per VNC session, bound to a random local port.
type VNCProxy struct {
	listener net.Listener
	target   string
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	wsConn   *websocket.Conn
	tcpConn  net.Conn
}

func NewVNCProxy(target string) *VNCProxy {
	return &VNCProxy{
		target: target,
		stopCh: make(chan struct{}),
	}
}

// Start begins listening on a random local port and returns the WebSocket URL.
func (p *VNCProxy) Start() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("vnc proxy listen: %w", err)
	}
	p.listener = ln

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		p.handleWebSocket(ws)
	})

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		_ = http.Serve(ln, mux)
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return fmt.Sprintf("ws://127.0.0.1:%d", addr.Port), nil
}

func (p *VNCProxy) handleWebSocket(ws *websocket.Conn) {
	p.mu.Lock()
	p.wsConn = ws
	p.mu.Unlock()

	tcp, err := net.Dial("tcp", p.target)
	if err != nil {
		ws.Close()
		return
	}

	p.mu.Lock()
	p.tcpConn = tcp
	p.mu.Unlock()

	p.wg.Add(2)

	go func() {
		defer p.wg.Done()
		defer tcp.Close()
		for {
			select {
			case <-p.stopCh:
				return
			default:
			}
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage {
				if _, err := tcp.Write(data); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer p.wg.Done()
		defer ws.Close()
		buf := make([]byte, 32768)
		for {
			select {
			case <-p.stopCh:
				return
			default:
			}
			n, err := tcp.Read(buf)
			if err != nil {
				if err != io.EOF {
					return
				}
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()
}

// Stop closes all connections and waits for goroutines to exit.
func (p *VNCProxy) Stop() {
	close(p.stopCh)
	p.mu.Lock()
	if p.wsConn != nil {
		p.wsConn.Close()
	}
	if p.tcpConn != nil {
		p.tcpConn.Close()
	}
	p.mu.Unlock()
	if p.listener != nil {
		p.listener.Close()
	}
	p.wg.Wait()
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend/session && go build vnc_proxy.go`
Expected: No output (compiles successfully). Note: `vnc_proxy.go` alone won't compile because it's a package file, but `go build ./...` from repo root should work after all files are in place.

---

## Task 2: Backend — VNCSession

**Files:**
- Create: `backend/session/vnc_session.go`

- [ ] **Step 1: Write VNCSession implementation**

```go
package session

import (
	"fmt"
	"time"
)

type VNCSession struct {
	baseSession
	proxy     *VNCProxy
	proxyAddr string
}

func NewVNCSession(id string) *VNCSession {
	return &VNCSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "vnc",
			status:      StatusDisconnected,
		},
	}
}

func (s *VNCSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)

	target := fmt.Sprintf("%s:%d", config.Host, config.Port)
	if config.Port <= 0 {
		target = fmt.Sprintf("%s:5900", config.Host)
	}

	s.title = fmt.Sprintf("%s (VNC)", config.Host)

	proxy := NewVNCProxy(target)
	addr, err := proxy.Start()
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("vnc proxy start: %w", err)
	}

	s.proxy = proxy
	s.proxyAddr = addr

	// Set connected immediately so frontend gets proxyAddr.
	// The actual VNC handshake happens between noVNC and the VNC server
	// through the proxy; we don't wait for it here.
	s.setStatus(StatusConnected)

	// Keep the session alive until Disconnect() is called.
	// The proxy goroutines handle the actual data flow.
	return nil
}

func (s *VNCSession) Disconnect() error {
	if s.proxy != nil {
		s.proxy.Stop()
		s.proxy = nil
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *VNCSession) IsConnected() bool {
	return s.Status() == StatusConnected
}

func (s *VNCSession) Resize(cols, rows int) error {
	// VNC desktop size is managed by noVNC's resizeSession or the server.
	return nil
}

func (s *VNCSession) Write(data []byte) error {
	// VNC data flows through WebSocket, not this method.
	return nil
}

func (s *VNCSession) ProxyAddr() string {
	return s.proxyAddr
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./backend/session/...`
Expected: No output (compiles successfully).

---

## Task 3: Backend — Integrate VNC into SessionManager and App

**Files:**
- Modify: `backend/session/manager.go`
- Modify: `app.go`

- [ ] **Step 1: Add "vnc" case to SessionManager**

In `backend/session/manager.go`, find the switch in `Create()` and add:

```go
case "vnc":
    s = NewVNCSession(config.ID)
```

- [ ] **Step 2: Add proxyAddr to session:status event for VNC**

In `app.go`, find the `SetOnStatusChangeCallback` block in `CreateSession()` and modify the payload building:

```go
s.SetOnStatusChangeCallback(func(status session.SessionStatus) {
    payload := map[string]interface{}{
        "id":     s.ID(),
        "status": status,
    }
    if status == session.StatusConnected {
        if rdp, ok := s.(*session.RDPSession); ok {
            cx, cy, cw, ch := rdp.ClientAreaScreenRect()
            payload["clientX"] = cx
            payload["clientY"] = cy
            payload["clientW"] = cw
            payload["clientH"] = ch
        }
        // NEW: Attach proxyAddr for VNC sessions
        if vnc, ok := s.(*session.VNCSession); ok {
            payload["proxyAddr"] = vnc.ProxyAddr()
        }
    }
    runtime.EventsEmit(a.ctx, "session:status", payload)
})
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: No output (compiles successfully).

- [ ] **Step 4: Commit backend changes**

```bash
git add backend/session/vnc_proxy.go backend/session/vnc_session.go backend/session/manager.go app.go
git commit -m "feat(vnc): add VNCProxy bridge and VNCSession backend"
```

---

## Task 4: Frontend — Type Definitions

**Files:**
- Modify: `frontend/src/types/session.ts`
- Modify: `frontend/src/types/workspace.ts`

- [ ] **Step 1: Extend ConnectionConfig.type in session.ts**

```typescript
export interface ConnectionConfig {
  id: string
  name: string
  type: 'ssh' | 'rdp' | 'vnc'
  host: string
  port: number
  user: string
  authType: 'password' | 'key' | 'agent'
  password?: string
  keyPath?: string
  groupId?: string
  // RDP-specific
  rdpFixedWidth?: number
  rdpFixedHeight?: number
  rdpSmartSizing?: boolean
}
```

- [ ] **Step 2: Add VNCTab and extend PanelType in workspace.ts**

```typescript
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc'
```

Add `VNCTab` interface:

```typescript
export interface VNCTab {
  type: 'vnc'
  id: string
  panelId: string
  name: string
}
```

Update `Tab` union type:

```typescript
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab | VNCTab
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/types/session.ts frontend/src/types/workspace.ts
git commit -m "feat(vnc): extend types for VNC connection support"
```

---

## Task 5: Frontend — Install noVNC Dependency

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Add @novnc/novnc to package.json**

Add to `dependencies`:

```json
"@novnc/novnc": "^1.5.0"
```

- [ ] **Step 2: Install and verify build**

Run:
```bash
cd frontend && npm install
cd .. && wails dev
```
Expected: Dev server starts without errors. Cancel after confirming (`Ctrl+C`).

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "feat(vnc): add @novnc/novnc dependency"
```

---

## Task 6: Frontend — VNCTabContent Component

**Files:**
- Create: `frontend/src/components/VNCTabContent.vue`

- [ ] **Step 1: Write VNCTabContent.vue**

```vue
<template>
  <div class="vnc-tab-content">
    <!-- Connecting state -->
    <div v-if="status === 'connecting'" class="vnc-overlay">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>{{ t('vnc.connecting', { host: config?.host || '...' }) }}</p>
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="vnc-overlay">
      <p class="vnc-error-text">{{ t('vnc.error') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('vnc.retry') }}</el-button>
    </div>

    <!-- Disconnected state -->
    <div v-else-if="status === 'disconnected'" class="vnc-overlay">
      <p>{{ t('vnc.disconnected') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('vnc.reconnect') }}</el-button>
    </div>

    <!-- Connected: noVNC Canvas mounts here -->
    <div
      v-show="status === 'connected'"
      ref="vncContainer"
      class="vnc-area"
      tabindex="0"
      @paste="onPaste"
    />

    <!-- Status bar -->
    <div v-if="status === 'connected'" class="vnc-statusbar">
      <span class="vnc-status-dot" />
      <span>{{ t('vnc.connected') }}</span>
      <span class="vnc-status-sep">|</span>
      <span>{{ config?.host }}:{{ config?.port || 5900 }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'
import { CreateSession, CloseSession } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const { t } = useI18n()

const props = defineProps<{
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null
}>()

const status = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
const currentSessionId = ref<string | null>(props.sessionId)
const vncContainer = ref<HTMLDivElement | null>(null)

let rfb: any = null
let unsubStatus: (() => void) | null = null

async function connect() {
  if (!props.config) return
  status.value = 'connecting'
  try {
    const info = await CreateSession('vnc', props.config)
    currentSessionId.value = info.id
  } catch (e) {
    console.error('VNC connect error:', e)
    status.value = 'error'
  }
}

async function reconnect() {
  if (currentSessionId.value) {
    try { await CloseSession(currentSessionId.value) } catch (_) {}
    currentSessionId.value = null
  }
  // Clean up existing RFB instance
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  await connect()
}

function initRFB(proxyAddr: string, password: string) {
  if (!vncContainer.value) return

  import('@novnc/novnc/core/rfb.js').then((module: any) => {
    const RFB = module.default || module
    rfb = new RFB(vncContainer.value, proxyAddr, {
      credentials: { password }
    })

    rfb.addEventListener('connect', () => {
      console.log('[VNC] RFB connected')
    })

    rfb.addEventListener('disconnect', (e: any) => {
      if (!e.detail.clean) {
        status.value = 'error'
      }
    })

    rfb.addEventListener('credentialsrequired', () => {
      status.value = 'error'
    })

    rfb.addEventListener('securityfailure', () => {
      status.value = 'error'
    })

    rfb.addEventListener('clipboard', (e: any) => {
      const text = e.detail.text
      navigator.clipboard.writeText(text).catch(() => {})
    })
  })
}

function onPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text')
  if (text && rfb) {
    rfb.clipboardPasteFrom(text)
  }
}

onMounted(() => {
  if (props.sessionId) {
    currentSessionId.value = props.sessionId
  }
  if (currentSessionId.value) {
    status.value = 'connected'
  } else {
    connect()
  }

  unsubStatus = EventsOn('session:status', (data: any) => {
    if (data.id !== currentSessionId.value) return
    switch (data.status) {
      case 'connected':
        status.value = 'connected'
        if (data.proxyAddr && props.config?.password !== undefined) {
          initRFB(data.proxyAddr, props.config.password)
        }
        break
      case 'disconnected':
        if (status.value !== 'error') status.value = 'disconnected'
        break
      case 'error':
        status.value = 'error'
        break
    }
  })
})

onUnmounted(() => {
  unsubStatus?.()
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  if (currentSessionId.value) {
    CloseSession(currentSessionId.value).catch(() => {})
  }
})

watch(() => props.sessionId, (newId) => {
  if (newId && !currentSessionId.value) {
    currentSessionId.value = newId
  }
})
</script>

<style scoped>
.vnc-tab-content {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: #000;
  position: relative;
}
.vnc-area {
  flex: 1;
  background: #000;
  outline: none;
}
.vnc-area :deep(canvas) {
  display: block;
  width: 100%;
  height: 100%;
}
.vnc-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #999;
  z-index: 10;
}
.vnc-error-text { color: #f56c6c; }
.vnc-statusbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: #1e1e1e;
  color: #999;
  font-size: 12px;
  flex-shrink: 0;
}
.vnc-status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: #67c23a;
}
.vnc-status-sep { color: #444; }
</style>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/VNCTabContent.vue
git commit -m "feat(vnc): add VNCTabContent component with noVNC integration"
```

---

## Task 7: Frontend — ConnectionForm VNC Support

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue`

- [ ] **Step 1: Add VNC to type selector**

Find the type radio-group and add VNC button (all platforms):

```vue
<el-radio-group v-model="form.type">
  <el-radio-button label="ssh">SSH</el-radio-button>
  <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
  <el-radio-button label="vnc">VNC</el-radio-button>
</el-radio-group>
```

- [ ] **Step 2: Auto-switch port for VNC**

In the `watch(() => form.type)` handler, add VNC port logic:

```typescript
watch(() => form.type, (newType) => {
  if (newType === 'rdp' && form.port === 22) form.port = 3389
  else if (newType === 'ssh' && form.port === 3389) form.port = 22
  else if (newType === 'vnc' && form.port === 22) form.port = 5900
  else if (newType === 'ssh' && form.port === 5900) form.port = 22
  if (newType === 'rdp' || newType === 'vnc') {
    form.authType = 'password'
  }
})
```

- [ ] **Step 3: Show password field for VNC**

The existing condition for password already covers RDP:
```vue
<el-form-item v-if="form.authType === 'password' || form.type === 'rdp'" :label="t('conn.password')">
```

Change to also include VNC:
```vue
<el-form-item v-if="form.authType === 'password' || form.type === 'rdp' || form.type === 'vnc'" :label="t('conn.password')">
```

- [ ] **Step 4: Hide authType selector for VNC**

The authType selector is already hidden for RDP (`v-if="form.type !== 'rdp'"`). Change to:
```vue
<el-form-item v-if="form.type !== 'rdp' && form.type !== 'vnc'" :label="t('conn.authType')">
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ConnectionForm.vue
git commit -m "feat(vnc): add VNC type to ConnectionForm"
```

---

## Task 8: Frontend — App.vue VNC Integration

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Import VNCTabContent**

Add import:
```typescript
import VNCTabContent from './components/VNCTabContent.vue'
```

- [ ] **Step 2: Add VNC tab rendering**

In the template, after the RDPTabContent branch, add:
```vue
<VNCTabContent
  v-else-if="activeTab.type === 'vnc'"
  :key="activeTab.id"
  :panel-id="activeTab.panelId"
  :config="getPanelConfig(activeTab.panelId)"
  :session-id="getPanelSessionId(activeTab.panelId)"
/>
```

- [ ] **Step 3: Add VNC session cleanup in closeTab**

In `closeTab()`, find the RDP cleanup block and add VNC:

```typescript
// Close RDP session before removing panel to clean up Go-side resources
const tab = tabStore.tabs.find(t => t.id === tabId)
if (tab && tab.type === 'rdp') {
  const p = panelStore.getPanel(tab.panelId)
  if (p?.sessionId) {
    try { await CloseSession(p.sessionId) } catch (_) {}
  }
}
// NEW: Close VNC session
if (tab && tab.type === 'vnc') {
  const p = panelStore.getPanel(tab.panelId)
  if (p?.sessionId) {
    try { await CloseSession(p.sessionId) } catch (_) {}
  }
}
```

- [ ] **Step 4: Add onConnectVNC method**

After `onConnectRDP`, add:

```typescript
async function onConnectVNC(config: ConnectionConfig) {
  connectionStore.add(config)

  const displayTitle = config.name
    ? `${config.name} (VNC)`
    : `${config.host} (VNC)`

  const panel = panelStore.createPanel(config, 'vnc')
  panel.title = displayTitle
  const tab = tabStore.createVNCTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('vnc', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create VNC session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}
```

- [ ] **Step 5: Route VNC connections in onConnect**

In `onConnect()`, add VNC routing before the existing SSH logic:

```typescript
async function onConnect(config: ConnectionConfig) {
  if (config.type === 'rdp') return onConnectRDP(config)
  if (config.type === 'vnc') return onConnectVNC(config)
  // existing SSH logic...
}
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.vue
git commit -m "feat(vnc): integrate VNC tab into App.vue"
```

---

## Task 9: Frontend — Sidebar VNC Menu Support

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: Add connectVnc emit**

Add to defineEmits:
```typescript
const emit = defineEmits(['connect', 'connectSftp', 'connectRdp', 'connectVnc', 'toggle'])
```

- [ ] **Step 2: Add VNC context menu item**

In the context menu template, update the conditional items:

```vue
<div v-if="!selectedConn || (selectedConn.type !== 'rdp' && selectedConn.type !== 'vnc')" class="menu-item" @click="doConnect">{{ t('sidebar.connect') }}</div>
<div v-if="!selectedConn || (selectedConn.type !== 'rdp' && selectedConn.type !== 'vnc')" class="menu-item" @click="doConnectSFTP">{{ t('sidebar.connectSftp') }}</div>
<div v-if="selectedConn && selectedConn.type === 'rdp'" class="menu-item" @click="doConnectRDP">{{ t('sidebar.connectRDP') }}</div>
<div v-if="selectedConn && selectedConn.type === 'vnc'" class="menu-item" @click="doConnectVNC">{{ t('sidebar.connectVNC') }}</div>
```

- [ ] **Step 3: Add doConnectVNC handler**

After `doConnectRDP`:

```typescript
function doConnectVNC() {
  const ids = getSelectedConnectionIds()
  const conns = ids.map(id => connectionStore.connections.find(c => c.id === id)).filter(Boolean) as ConnectionConfig[]
  selectedIds.value = new Set()
  closeMenu()
  for (const c of conns) {
    emit('connectVnc', c)
  }
}
```

- [ ] **Step 4: Handle VNC double-click**

In `onItemDblClick`:

```typescript
function onItemDblClick(conn: ConnectionConfig) {
  selectedIds.value = new Set()
  if (conn.type === 'rdp') {
    emit('connectRdp', conn)
  } else if (conn.type === 'vnc') {
    emit('connectVnc', conn)
  } else {
    emit('connect', conn)
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/Sidebar.vue
git commit -m "feat(vnc): add VNC connection menu in Sidebar"
```

---

## Task 10: Frontend — TabStore and TabBar VNC Support

**Files:**
- Modify: `frontend/src/stores/tabStore.ts`
- Modify: `frontend/src/components/TabBar.vue`

- [ ] **Step 1: Add createVNCTab to tabStore**

After `createRDPTab`:

```typescript
function createVNCTab(name: string, panelId: string): VNCTab {
  const tab: VNCTab = {
    type: 'vnc',
    id: genId('vnc-tab'),
    panelId,
    name
  }
  tabState.tabs.push(tab)
  tabState.activeTabId = tab.id
  return tab
}
```

Add to return object:
```typescript
createVNCTab,
```

- [ ] **Step 2: Include VNC in closeTab removedPanelIds**

Find the `removedPanelIds` ternary in `closeTab` and add VNC:

```typescript
const removedPanelIds = removed.type === 'terminal' || removed.type === 'settings' || removed.type === 'rdp' || removed.type === 'vnc'
  ? [removed.panelId]
  : removed.type === 'workspace'
    ? removed.panelIds
    : removed.type === 'sftp'
      ? [removed.panelId]
      : []
```

- [ ] **Step 3: Include VNC in TabBar TabItem v-if**

In `TabBar.vue`, update:
```vue
<TabItem
  v-if="tab.type === 'terminal' || tab.type === 'settings' || tab.type === 'sftp' || tab.type === 'rdp' || tab.type === 'vnc'"
```

- [ ] **Step 4: Add VNC cleanup in TabBar close handler**

In TabBar's close handler, after RDP cleanup, add VNC:

```typescript
// Clean up VNC session
if (tab && tab.type === 'vnc') {
  const vncPanel = panelStore.getPanel(tab.panelId)
  if (vncPanel?.sessionId) {
    try { await CloseSession(vncPanel.sessionId) } catch (_) {}
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/tabStore.ts frontend/src/components/TabBar.vue
git commit -m "feat(vnc): add VNC tab support to tabStore and TabBar"
```

---

## Task 11: Frontend — i18n Translations

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Add zh-CN VNC strings**

In zh-CN messages, after the RDP section:

```typescript
// VNC
'sidebar.connectVNC': '连接 VNC',
'vnc.connecting': '正在连接到 {host}...',
'vnc.connected': '已连接',
'vnc.disconnected': '已断开',
'vnc.error': '连接失败',
'vnc.reconnect': '重新连接',
'vnc.retry': '重试',
```

- [ ] **Step 2: Add en VNC strings**

In en messages, after the RDP section:

```typescript
// VNC
'sidebar.connectVNC': 'Connect VNC',
'vnc.connecting': 'Connecting to {host}...',
'vnc.connected': 'Connected',
'vnc.disconnected': 'Disconnected',
'vnc.error': 'Connection failed',
'vnc.reconnect': 'Reconnect',
'vnc.retry': 'Retry',
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/i18n/index.ts
git commit -m "feat(vnc): add VNC i18n translations"
```

---

## Task 12: Build and Manual Test

**Files:**
- All of the above

- [ ] **Step 1: Build frontend**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors.

- [ ] **Step 2: Build Go backend**

Run: `go build ./...`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Run dev mode and verify**

Run: `wails dev`
Test checklist:
1. Open ConnectionForm, select VNC type, verify port defaults to 5900
2. Save a VNC connection, verify it appears in sidebar
3. Right-click VNC connection, verify "连接 VNC" menu item
4. Double-click VNC connection, verify VNC tab opens with "connecting" spinner
5. If a VNC server is available, verify connection succeeds and Canvas renders
6. Close VNC tab, verify no errors
7. Switch between VNC and other tabs, verify no visual glitches

- [ ] **Step 4: Final commit**

```bash
git commit -m "feat(vnc): complete VNC remote desktop feature"
```

---

## Self-Review

### Spec Coverage Check

| Spec Requirement | Implementing Task |
|---|---|
| WebSocket↔TCP 桥接 (VNCProxy) | Task 1 |
| VNCSession 实现 Session 接口 | Task 2 |
| SessionManager 增加 vnc case | Task 3 |
| app.go 附加 proxyAddr 到事件 | Task 3 |
| @novnc/novnc 依赖 | Task 5 |
| 前端类型扩展 | Task 4 |
| VNCTabContent 组件 | Task 6 |
| ConnectionForm VNC 支持 | Task 7 |
| App.vue VNC 集成 | Task 8 |
| Sidebar VNC 菜单 | Task 9 |
| tabStore / TabBar VNC 支持 | Task 10 |
| i18n 文案 | Task 11 |
| 剪贴板双向同步 | Task 6 (VNCTabContent.vue) |
| 跨平台支持 | 架构本身保证（纯前端渲染 + Go 桥接） |

### Placeholder Scan

- No TBD/TODO/fill-in-details found ✓
- All steps contain actual code ✓
- No "similar to Task N" shortcuts ✓

### Type Consistency Check

- `ConnectionConfig.type`: `'ssh' | 'rdp' | 'vnc'` — consistent across session.ts and all usages ✓
- `PanelType`: `'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc'` — consistent ✓
- `VNCTab.type`: `'vnc'` — matches PanelType ✓
- `createVNCTab` returns `VNCTab` — type matches `Tab` union ✓
- `SessionManager.Create` case `"vnc"` — matches ConnectionConfig.type ✓
- `CreateSession('vnc', config)` — type string consistent ✓

### Gaps Found

None. All spec requirements are covered.
