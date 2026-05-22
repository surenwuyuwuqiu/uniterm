# VNC 远程桌面功能设计

## 概述

为 uniTerm 新增 VNC（Virtual Network Computing）远程桌面支持，作为继 SSH、SFTP、RDP 之后的第四种连接类型。VNC 采用纯前端渲染方案（noVNC + Canvas），Go 后端仅负责 TCP↔WebSocket 桥接，天然跨平台（Windows/macOS/Linux）。

## 方案选型

**使用 `@novnc/novnc` npm 包作为前端 RFB 客户端，Go 后端自研轻量级 WebSocket→TCP 桥接（VNCProxy）。**

- 前端直接处理 RFB 协议、Canvas 渲染、鼠标/键盘输入，Go 后端零协议解析负担
- 不依赖原生窗口，与现有 Vue/Wails 架构完全融合，UI 风格统一
- 每个会话独立桥接端口，会话隔离清晰

对比 RDP 方案：

| 维度 | RDP | VNC（本方案） |
|---|---|---|
| 渲染方式 | Windows 原生 ActiveX HWND 子窗口 | 前端 noVNC Canvas |
| 跨平台 | Windows only | Windows / macOS / Linux |
| Go 后端职责 | COM 互操作 + Win32 API | TCP↔WebSocket 桥接 |
| 前端职责 | 状态占位 + 位置同步 | 完整 RFB 协议 + 渲染 |
| 剪贴板 | ActiveX 内置 | noVNC `clipboard` 事件 + navigator.clipboard |

## 认证

仅支持密码认证（VNC Authentication / VNC Password）。noVNC 的 `RFB` 构造函数支持通过 `credentials: { password }` 预填密码，也可以在 `credentialsrequired` 事件中动态输入。

## 功能范围（MVP）

- 远程桌面画面显示（Canvas 渲染）
- 鼠标点击、移动、滚轮输入
- 键盘输入
- 剪贴板双向同步（本地 ↔ 远程）
- 连接状态管理（connecting / connected / disconnected / error）

**本期不做**：画面缩放（scaleViewport）、全屏切换、多编码协商优化、文件传输（VNC 本身不支持文件传输，需额外协议如RFB扩展或SFTP）。缩放和全屏可通过后续简单配置 `rfb.scaleViewport` 快速添加。

## 连接配置

`ConnectionConfig.type` 新增 `'vnc'`。

VNC 配置字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `host` | string | 远程主机地址 |
| `port` | number | 默认 5900 |
| `password` | string | VNC 密码 |

后端 `session.ConnectionConfig` 复用现有结构体，VNC 仅使用 `Host`、`Port`、`Password` 三个字段，无需新增专用字段。

## 架构

```
Wails 主窗口
├── WebView2（Vue 前端）
│   ├── Sidebar / ConnectionForm（新增 VNC 类型选项）
│   ├── TabBar（VNC Tab 与 SSH/SFTP/RDP Tab 并列）
│   └── Tab 内容区
│       ├── VNCTabContent.vue（noVNC Canvas + 状态 UI）
│       │   └── RFB ──WebSocket──→ VNCProxy
│       ├── TerminalTabContent（SSH）
│       ├── SFTPTabContent
│       └── RDPTabContent（RDP，Windows only）
```

Go 后端为每个 VNC 会话启动一个独立的 `VNCProxy` 实例，监听 `127.0.0.1:0`（随机端口），只接受来自前端 noVNC 的 WebSocket 连接，将数据透传到 VNC 服务器的 TCP 端口。

## 新增文件

### `backend/session/vnc_session.go`

VNC 会话实现，实现 `Session` 接口：

```go
VNCSession struct:
  baseSession
  proxy     *VNCProxy  // TCP↔WebSocket 桥接器
  proxyAddr string     // 本地 WebSocket 监听地址，如 "ws://127.0.0.1:54321"

Connect():
  1. 设置状态 connecting
  2. 创建 VNCProxy，目标地址 = config.Host:config.Port（默认 5900）
  3. 启动本地 WebSocket 监听（127.0.0.1:0）
  4. proxyAddr 通过 session:status 事件发送给前端
  5. 等待前端 WebSocket 连接或超时（30秒）
  6. 连接成功后状态变为 connected

Disconnect():
  1. 关闭 VNCProxy（listener + wsConn + tcpConn）
  2. 等待 goroutine 退出
  3. 状态变为 disconnected

Write(data []byte): 空实现（VNC 数据不走此方法）
Resize(cols, rows): 空实现（VNC 桌面尺寸由服务器或 noVNC 的 resizeSession 处理）
```

### `backend/session/vnc_proxy.go`

纯 Go 实现的 WebSocket→TCP 桥接器（约 150 行），每个会话独立实例：

```go
VNCProxy struct:
  listener   net.Listener    // 本地 WS 监听
  target     string          // VNC 服务器地址
  wsUpgrader websocket.Upgrader
  stopCh     chan struct{}
  wg         sync.WaitGroup

Start():
  1. net.Listen("tcp", "127.0.0.1:0")
  2. http.Serve(listener, WS handler)

WS handler:
  1. Upgrade HTTP → WebSocket
  2. net.Dial("tcp", target) 连接 VNC 服务器
  3. 启动两个 goroutine 双向复制：
     - ws → tcp: websocket 帧读取 → TCP 写入
     - tcp → ws: TCP 读取 → websocket 二进制帧发送
  4. 任一方向断开时，两边都关闭

Stop():
  1. close(stopCh)
  2. listener.Close()
  3. wg.Wait()
```

安全设计：
- 只绑定 `127.0.0.1`，拒绝外部连接
- 每个会话独立端口，会话关闭后端口释放，避免多会话数据混淆
- 不验证 WebSocket Origin（因为只绑定本地回环）

### `frontend/src/components/VNCTabContent.vue`

```
状态:
  connecting  → spinner + "正在连接到 {host}..."
  connected   → noVNC Canvas 区域（全填充）
  disconnected → 断开提示 + "重新连接"按钮
  error       → 错误信息 + "重试"按钮

Props:
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null

核心逻辑:
  onMounted:
    - 监听 session:status 事件
    - 状态为 connected 时，从事件 payload 获取 proxyAddr
    - import RFB from '@novnc/novnc/core/rfb.js'
    - new RFB(container, proxyAddr, { credentials: { password } })
    - 绑定 RFB 事件：connect、disconnect、credentialsrequired、clipboard

  onUnmounted:
    - rfb.disconnect()
    - CloseSession(sessionId)

剪贴板同步:
  - 服务端→客户端: RFB 'clipboard' 事件 → navigator.clipboard.writeText()
  - 客户端→服务端: 监听容器的 `paste` 事件 → e.clipboardData.getData('text') → rfb.clipboardPasteFrom()

`paste` 事件由浏览器在用户按 Ctrl+V / Cmd+V / 右键粘贴时自然触发，自带剪贴板权限，无需额外申请。noVNC 内部的键盘事件处理不会拦截浏览器的原生 paste 事件。
```

样式与 `RDPTabContent.vue` 保持一致：黑色背景、状态栏（已连接/主机/端口）、绿色状态点。

## 修改文件

### 前端

| 文件 | 改动 |
|---|---|
| `frontend/package.json` | 新增依赖 `"@novnc/novnc": "^1.5.0"` |
| `frontend/src/types/session.ts` | `ConnectionConfig.type` 扩展为 `'ssh' \| 'rdp' \| 'vnc'` |
| `frontend/src/types/workspace.ts` | `PanelType` 增加 `'vnc'`，新增 `VNCTab` 接口，`Tab` 联合类型新增 `VNCTab` |
| `frontend/src/components/ConnectionForm.vue` | 类型选择器增加 VNC 按钮（所有平台都显示）；`type === 'vnc'` 时显示主机/端口/密码；默认端口 5900 |
| `frontend/src/components/VNCTabContent.vue` | 新增，上述完整组件 |
| `frontend/src/components/Sidebar.vue` | 连接项右键菜单增加"连接 VNC"选项 |
| `frontend/src/stores/tabStore.ts` | 新增 `createVNCTab()` 工厂函数 |
| `frontend/src/stores/panelStore.ts` | `createPanel` 支持 `'vnc'` 面板类型 |
| `frontend/src/App.vue` | `v-else-if="activeTab.type === 'vnc'"` 分支渲染 `<VNCTabContent>`；`closeTab` 时 VNC 类型也调用 `CloseSession`；`onConnect` / `onConnectVNC` 分流 |
| `frontend/src/i18n/index.ts` | 新增 VNC 相关文案的 zh/en 翻译 |

### 后端

| 文件 | 改动 |
|---|---|
| `backend/session/vnc_session.go` | 新增，实现 `Session` 接口 |
| `backend/session/vnc_proxy.go` | 新增，WebSocket↔TCP 桥接 |
| `backend/session/manager.go` | `Create()` switch 增加 `case "vnc": s = NewVNCSession(config.ID)` |
| `backend/session/session.go` | `ConnectionConfig` 无需修改（复用 Host/Port/Password） |
| `app.go` | `CreateSession` 中 `SetOnStatusChangeCallback` 里增加 VNC 类型判断，状态为 `connected` 时附加 `proxyAddr` 字段到事件 payload |

### VNC Tab 与 Workspace 关系

VNC Tab 是**独占型 Tab**，不支持合并到 Workspace 的分割面板中。原因：noVNC 的 Canvas 渲染在一个 DOM 容器中，虽然技术上可以分割，但当前 Workspace 的分割面板系统主要针对 xterm.js 终端设计，VNC 的鼠标/键盘焦点管理在分割场景下会变得复杂。

在 TabBar 右键菜单中，VNC Tab 不显示"合并到工作区"选项。

## 数据流：连接生命周期

```
用户点击"连接 VNC"
    │
    ▼
ConnectionForm 提交 config {type:'vnc', host, port, password}
    │
    ▼
App.vue 调用 CreateSession('vnc', config) ──→ Go SessionManager
    │                                              │
    │                                              ▼
    │                                      创建 VNCSession
    │                                              │
    │                                              ▼
    │                                      VNCSession.Connect()
    │                                              │
    │                                              ▼
    │                                      启动 VNCProxy
    │                                      监听 127.0.0.1:随机端口
    │                                              │
    ▼                                              │
前端监听 session:status ◄────────────────── 发射事件
{ id, status:'connected', proxyAddr:'ws://127.0.0.1:54321' }
    │
    ▼
VNCTabContent 收到 connected
    │
    ▼
new RFB(container, 'ws://127.0.0.1:54321', { credentials })
    │
    ▼
RFB 连接 WebSocket ──→ VNCProxy ──→ TCP ──→ VNC Server
    │                                              │
    │◄─────────────────────────────────────────────│
    │         RFB 协议握手 + 帧数据 + 输入事件
    │
用户操作鼠标/键盘
    │
    ▼
noVNC 捕获输入 → WebSocket 发送 → VNCProxy 转发 → VNC Server
```

## 剪贴板双向同步

### 服务端 → 客户端（VNC → 本地）

noVNC 的 RFB 对象在收到服务器的 `ServerCutText` 消息后会触发 `clipboard` 事件：

```js
rfb.addEventListener('clipboard', (e) => {
  const text = e.detail.text
  navigator.clipboard.writeText(text).catch(() => {})
})
```

### 客户端 → 服务端（本地 → VNC）

浏览器对 `navigator.clipboard.readText()` 有安全限制（必须通过用户手势触发）。实现方式：

```js
// 监听粘贴快捷键（Ctrl+V / Cmd+V）
vncContainer.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'v') {
    e.preventDefault()
    navigator.clipboard.readText().then(text => {
      rfb.clipboardPasteFrom(text)
    }).catch(() => {})
  }
})
```

不采用 Clipboard API 的 `navigator.clipboard.read()` 自动监听方案（权限不稳定、兼容性差）。

## 错误处理

| 场景 | 检测方式 | 处理方式 |
|---|---|---|
| VNC 服务器不可达 | TCP Dial 超时 | VNCProxy 连接失败 → 关闭 proxy → VNCSession 状态变为 error |
| 密码错误 | RFB 安全握手失败 | noVNC 触发 `securityfailure` 事件 → 前端显示 error 状态 |
| WebSocket 连接失败 | 前端 5 秒超时检测 | 未收到 `connect` 事件 → 显示 error + 重试按钮 |
| 网络断开 | TCP 连接 EOF / WS close | 双向 goroutine 退出 → VNCProxy 关闭 → noVNC 触发 `disconnect` 事件 |
| Proxy 端口冲突 | `net.Listen("tcp", "127.0.0.1:0")` | 由 OS 分配随机端口，理论上不会冲突 |
| 会话关闭时资源泄漏 | `VNCSession.Disconnect()` | 关闭 listener → close wsConn → close tcpConn → `wg.Wait()` 等待 goroutine 退出 |

## 测试策略

- **单元测试**：`VNCProxy` 的双向转发逻辑。用 `net.Pipe()` 或本地 TCP echo 服务器模拟 VNC，验证 WS→TCP 和 TCP→WS 数据一致性
- **集成测试**：启动 VNCProxy，用 Go 的 `gorilla/websocket` 客户端连接，发送二进制帧，验证透传到目标 TCP
- **手动测试**：
  1. 启动 TightVNC / TigerVNC 服务器（本地虚拟机或 Docker）
  2. 通过 uniTerm 连接，验证画面显示、鼠标、键盘
  3. 测试剪贴板：从本地复制文本粘贴到 VNC 桌面，反之亦然
  4. 测试连接断开/重连
  5. 测试多标签同时打开多个 VNC 连接

## 编译与部署注意事项

- `@novnc/novnc` 是纯 ES6 模块，Vite 可以直接打包，无需额外配置
- Go 的 WebSocket 库推荐使用标准库 `golang.org/x/net/websocket` 或 `github.com/gorilla/websocket`（后者更成熟）。由于 Wails 项目已依赖 gorilla/websocket（通过 Wails 自身），建议直接使用 `nhooyr/websocket` 或 `gorilla/websocket` 保持依赖简洁
- VNCProxy 只绑定 `127.0.0.1`，不存在安全风险
- 跨平台编译：Wails 支持 `wails build` 跨平台编译到 Windows/macOS/Linux（macOS 需要在 macOS 环境或 CI 上构建）

## 依赖

| 包 | 版本 | 用途 |
|---|---|---|
| `@novnc/novnc` | ^1.5.0 | 前端 RFB 客户端 |
| `github.com/gorilla/websocket` | v1.5.x | Go WebSocket 服务端（如项目未引入） |

如果项目已有 WebSocket 相关依赖，优先复用现有依赖，避免重复引入。