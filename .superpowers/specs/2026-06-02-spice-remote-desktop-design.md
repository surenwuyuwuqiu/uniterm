# SPICE 远程桌面功能设计

## 概述

为 uniTerm 新增 SPICE（Simple Protocol for Independent Computing Environments）远程桌面支持。SPICE 是 KVM/QEMU 虚拟化的原生远程显示协议，采用与 VNC 相同的纯前端渲染方案（spice-html5 + Canvas），Go 后端仅负责 TCP↔WebSocket 桥接，天然跨平台（Windows/macOS/Linux）。

## 方案选型

**使用 spice-html5 作为前端 SPICE 客户端，Go 后端自研轻量级 WebSocket→TCP 桥接（SPICEProxy）。**

理由：
- spice-html5 是唯一可用的跨平台 SPICE JS 客户端，由 SPICE 官方维护
- 前端直接处理 SPICE 协议、Canvas 渲染、键盘/鼠标/音频，Go 后端零协议解析负担
- 不依赖原生窗口，与现有 Vue/Wails 架构完全融合，UI 风格统一
- 架构与 VNC（VNCTabContent + VNCProxy）一致，实现模式已有前例

### 候选方案对比

| 方案 | 跨平台 | 嵌入 WebView | 体积增量 | 功能完整度 |
|------|--------|-------------|---------|-----------|
| **spice-html5** | ✅ | ✅ | ~80KB | 显示/键鼠/剪贴板/音频播放/分辨率调整 |
| spice-gtk (CGo 嵌入) | ✅ | ❌ 需 GTK 控件 | 50MB+ | 完整，含 USB 重定向 |
| WebRTC + streaming-agent | ✅ | ✅ | 大 | 性能最优，需在 VM 内部署 agent |

spice-html5 是唯一适合 Wails + WebView 架构的方案。spice-gtk 需要 GTK 栈无法嵌入 WebView，WebRTC 方案需要在虚拟机内部署 agent 且实施复杂度过高。

## SPICE 协议简介

SPICE 与 VNC/RFB 的关键区别：

- **多通道架构**：显示、输入、光标、音频播放、音频录制、USB 等通道在同一 TCP 连接上复用
- **客户端渲染**：SPICE 发送 2D 绘图命令（QXL 设备驱动生成），客户端执行渲染，比像素传输更高效
- **自适应编码**：支持 quic、lz、glz、jpeg、zlib 等多种压缩算法
- **音频支持**：原生 PCM 音频流，通过 SPICE 播放通道传输

spice-html5 支持：显示通道（quic/lz/glz）、输入通道、光标通道、音频播放通道、agent 通道（剪贴板/分辨率）。不支持：USB 通道、音频录制通道、智能卡通道。

## 功能范围

受 spice-html5 能力限制，本次实现以下功能：

- ✅ 远程桌面画面显示（Canvas 渲染，支持 quic/lz/glz 解码）
- ✅ 键盘输入
- ✅ 鼠标点击、移动、滚轮输入
- ✅ 剪贴板双向同步（通过 SPICE agent 通道）
- ✅ 音频播放（Web Audio API，PCM 流）
- ✅ 分辨率自适应调整
- ✅ 连接状态管理（connecting / connected / disconnected / error）

**不做**：USB 重定向（spice-html5 不支持）、音频录制（spice-html5 不支持）、智能卡、多显示器。

## 连接配置

`ConnectionConfig.type` 新增 `'spice'`。

SPICE 配置字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `host` | string | 远程主机地址 |
| `port` | number | 默认 5900，小于 100 时按 libvirt 规则 +5900 |
| `password` | string | SPICE 密码 |

后端 `session.ConnectionConfig` 复用现有结构体，SPICE 仅使用 `Host`、`Port`、`Password` 三个字段，端口规则与 VNC 一致（libvirt 兼容：port < 100 时实际端口 = port + 5900）。

## 架构

```
Wails 主窗口
├── WebView2（Vue 前端）
│   ├── Sidebar / ConnectionForm（新增 SPICE 类型选项）
│   ├── TabBar（SPICE Tab 与 SSH/SFTP/RDP/VNC Tab 并列）
│   └── Tab 内容区
│       ├── SPICETabContent.vue（spice-html5 Canvas + 状态 UI）
│       │   └── SpiceMainConn ──WebSocket──→ SPICEProxy
│       ├── VNCTabContent（VNC）
│       ├── TerminalTabContent（SSH）
│       ├── SFTPTabContent
│       └── RDPTabContent（RDP，Windows only）
```

Go 后端为每个 SPICE 会话启动一个独立的 `SPICEProxy` 实例，监听 `127.0.0.1:0`（随机端口），只接受来自前端 spice-html5 的 WebSocket 连接，将数据透传到 SPICE 服务器的 TCP 端口。

## 新增文件

### `backend/session/spice_session.go`

SPICE 会话实现，实现 `Session` 接口：

```go
type SPICESession struct {
    baseSession
    proxy     *SPICEProxy  // TCP↔WebSocket 桥接器
    proxyAddr string       // 本地 WebSocket 监听地址，如 "ws://127.0.0.1:54321"
}

func NewSPICESession(id string) *SPICESession

Connect(config ConnectionConfig):
  1. 设置状态 connecting
  2. 计算目标地址：host:port（port 规则同 VNC，默认 5900）
  3. 创建 SPICEProxy，目标地址
  4. 启动本地 WebSocket 监听（127.0.0.1:0）
  5. proxyAddr 通过 session:status 事件发送给前端
  6. 状态变为 connected（实际 SPICE 握手由前端 spice-html5 完成）

Disconnect():
  1. 关闭 SPICEProxy（listener + wsConn + tcpConn）
  2. 等待 goroutine 退出
  3. 状态变为 disconnected

Write(data []byte): 空实现（SPICE 数据不走此方法）
Resize(cols, rows): 空实现（SPICE 分辨率由 spice-html5/spice agent 协商处理）
```

### `backend/session/spice_proxy.go`

纯 Go 实现的 WebSocket→TCP 桥接器，与 `VNCProxy` 架构完全一致，仅日志标识不同：

```go
type SPICEProxy struct {
    listener   net.Listener
    target     string
    stopCh     chan struct{}
    stopOnce   sync.Once
    wg         sync.WaitGroup
    mu         sync.Mutex
    wsConn     *websocket.Conn
    tcpConn    net.Conn
}

func NewSPICEProxy(target string) *SPICEProxy

Start() (string, error):
  1. net.Listen("tcp", "127.0.0.1:0")
  2. http.Serve(listener, WS handler)

WS handler:
  1. Upgrade HTTP → WebSocket
  2. 限制仅一个 WebSocket 连接（同 VNCProxy）
  3. net.Dial("tcp", target) 连接 SPICE 服务器
  4. 启动两个 goroutine 双向复制：
     - ws → tcp: WebSocket 读取 → TCP 写入
     - tcp → ws: TCP 读取 → WebSocket 二进制帧发送
  5. 任一方向断开时，两边都关闭

Stop():
  1. close(stopCh)
  2. listener.Close()
  3. wsConn.Close() / tcpConn.Close()
  4. wg.Wait()
```

安全设计：
- 只绑定 `127.0.0.1`，拒绝外部连接
- 每个会话独立端口，会话关闭后端口释放
- 不验证 WebSocket Origin（只绑定本地回环）

### `frontend/src/components/SPICETabContent.vue`

```
状态:
  connecting  → spinner + "正在连接到 {host}..."
  connected   → spice-html5 Canvas 区域（全填充）
  disconnected → 断开提示 + "重新连接"按钮
  error       → 错误信息 + "重试"按钮

Props:
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null

核心逻辑:
  onMounted:
    - 检查缓存（panelStore.getSPICECache），有则恢复
    - 监听 session:status 事件
    - 状态为 connected 时，从事件 payload 获取 proxyAddr
    - 加载 spice-html5：
      import('spice-html5').then(module => {
        const sc = new module.SpiceMainConn({
          uri: proxyAddr,
          password: config.password,
          ...
        })
      })
    - spice-html5 在 container 内自动创建 Canvas
    - 绑定 spice-html5 事件：连接状态、剪贴板

  onUnmounted:
    - 缓存 DOM + spice 对象（panelStore.setSPICECache），类似 VNC 的零延迟标签切换
    - 或 disconnect 并 CloseSession(sessionId)

剪贴板同步:
  - 服务端→客户端: spice-html5 agent 通道接收剪贴板 → 写入本地剪贴板
  - 客户端→服务端: 监听 paste 事件 / Ctrl+Shift+V → 读取本地剪贴板 → 发送到 SPICE agent

spice-html5 加载方式:
  由于 spice-html5 没有标准的 npm 包（托管在 freedesktop.org），两种加载策略：
  方案 A: npm install spice-html5（如果有 npm 发布）→ Vite 自动打包
  方案 B: 将 spice-html5 的 JS 文件作为静态资源放在 frontend/public/ 下，通过 <script> 标签或动态 import 加载
  
  优先尝试方案 A，若无 npm 发布则使用方案 B（手动下载 spice.js 到 public/）。
```

样式与 `VNCTabContent.vue` 保持一致：黑色背景、状态栏（已连接/主机/端口）、绿色状态点。

## 修改文件

### 前端

| 文件 | 改动 |
|---|---|
| `frontend/package.json` | 新增 spice-html5 依赖（或手动管理） |
| `frontend/src/types/session.ts` | `ConnectionConfig.type` 扩展增加 `'spice'` |
| `frontend/src/types/workspace.ts` | `PanelType` 增加 `'spice'`，新增 `SPICETab` 接口 |
| `frontend/src/components/ConnectionForm.vue` | 类型选择器增加 SPICE 按钮；`type === 'spice'` 时显示主机/端口/密码；默认端口 5900 |
| `frontend/src/components/SPICETabContent.vue` | 新增完整组件 |
| `frontend/src/components/Sidebar.vue` | 连接项右键菜单增加"连接 SPICE"选项 |
| `frontend/src/stores/tabStore.ts` | 新增 `createSPICETab()` 工厂函数 |
| `frontend/src/stores/panelStore.ts` | `createPanel` 支持 `'spice'` 面板类型；新增 SPICE 缓存方法 |
| `frontend/src/App.vue` | `v-else-if="activeTab.type === 'spice'"` 分支渲染 `<SPICETabContent>` |
| `frontend/src/i18n/index.ts` | 新增 SPICE 相关文案的 zh/en 翻译 |

### 后端

| 文件 | 改动 |
|---|---|
| `backend/session/spice_session.go` | 新增，实现 `Session` 接口 |
| `backend/session/spice_proxy.go` | 新增，WebSocket↔TCP 桥接 |
| `backend/session/manager.go` | `Create()` switch 增加 `case "spice": s = NewSPICESession(config.ID)` |
| `backend/session/session.go` | `ConnectionConfig` 无需修改（复用 Host/Port/Password） |
| `app.go` | `CreateSession` 中 `SetOnStatusChangeCallback` 里增加 SPICE 类型判断，状态为 `connected` 时附加 `proxyAddr` 字段到事件 payload |

### SPICE Tab 与 Workspace 关系

SPICE Tab 与 VNC Tab 相同，是 **独占型 Tab**，不支持合并到 Workspace 的分割面板中。spice-html5 的 Canvas 渲染和 SPICE 输入事件管理在分割场景下焦点处理复杂。

## 数据流：连接生命周期

```
用户点击"连接 SPICE"
    │
    ▼
ConnectionForm 提交 config {type:'spice', host, port, password}
    │
    ▼
App.vue 调用 CreateSession('spice', config) ──→ Go SessionManager
    │                                              │
    │                                              ▼
    │                                      创建 SPICESession
    │                                              │
    │                                              ▼
    │                                      SPICESession.Connect()
    │                                              │
    │                                              ▼
    │                                      启动 SPICEProxy
    │                                      监听 127.0.0.1:随机端口
    │                                              │
    ▼                                              │
前端监听 session:status ◄────────────────── 发射事件
{ id, status:'connected', proxyAddr:'ws://127.0.0.1:54321' }
    │
    ▼
SPICETabContent 收到 connected
    │
    ▼
new SpiceMainConn({uri: 'ws://127.0.0.1:54321', password: '...'})
    │
    ▼
spice-html5 连接 WebSocket ──→ SPICEProxy ──→ TCP ──→ SPICE Server
    │                                                    │
    │◄───────────────────────────────────────────────────│
    │     SPICE 协议握手 + 通道建立 + 显示/音频/输入数据
    │
```

## 剪贴板双向同步

### 服务端 → 客户端（SPICE → 本地）

spice-html5 通过 SPICE agent 通道接收剪贴板数据后，通过 Clipboard API 写入系统剪贴板。

### 客户端 → 服务端（本地 → SPICE）

监听键盘快捷键（Ctrl+Shift+V 或 Ctrl+V），通过 `navigator.clipboard.readText()` 或 Wails Runtime `ClipboardGetText()` 读取剪贴板，再通过 SPICE agent 通道发送到服务端。

## 音频播放

spice-html5 使用 Web Audio API 播放 SPICE 服务器发送的 PCM 音频流。无需额外配置，用户在连接建立后自动启用音频。前端需在连接时处理浏览器自动播放策略（autoplay policy），可能需要用户首次交互后解锁音频上下文。

## 错误处理

| 场景 | 检测方式 | 处理方式 |
|---|---|---|
| SPICE 服务器不可达 | TCP Dial 超时 | SPICEProxy 连接失败 → 关闭 proxy → SPICESession 状态变为 error |
| 密码错误 | SPICE 认证握手失败 | spice-html5 触发断开事件 → 前端显示 error 状态 |
| WebSocket 连接失败 | 前端超时检测 | 未在规定时间内收到连接确认 → 显示 error + 重试按钮 |
| 网络断开 | TCP 连接 EOF / WS close | 双向 goroutine 退出 → SPICEProxy 关闭 → spice-html5 检测断开 |
| 会话关闭时资源泄漏 | `SPICESession.Disconnect()` | 关闭 listener → close wsConn → close tcpConn → `wg.Wait()` |
| 音频自动播放被阻止 | 浏览器 autoplay 策略 | 无需特殊处理，spice-html5 会在首次用户交互后恢复音频 |

## spice-html5 加载策略

spice-html5 在 npm 上的发布状态不确定。两种加载方式：

1. **npm 包方式（优先）**：若 `spice-html5` 在 npm registry 上可用，`npm install spice-html5`，Vite 自动打包。前端通过 `import SpiceMainConn from 'spice-html5'` 加载。

2. **静态文件方式（兜底）**：从 [spice-html5 GitLab](https://gitlab.freedesktop.org/spice/spice-html5) 下载 `spice.js`、`spice.css` 等文件到 `frontend/public/spice-html5/`，在 `SPICETabContent.vue` 中动态创建 `<script>` 标签加载。全局命名空间 `SpiceMainConn`。

实施时先确认 npm 可用性，选择对应方式。

## 测试策略

- **手动测试**：
  1. 在 KVM/QEMU 虚拟机上启用 SPICE 服务（qemu -spice port=5900,password=xxx）
  2. 通过 uniTerm 连接，验证画面显示、鼠标、键盘
  3. 测试剪贴板：从本地复制文本粘贴到 SPICE 桌面，反之亦然
  4. 测试音频：虚拟机内播放音频，确认本地能听到
  5. 测试连接断开/重连
  6. 测试多标签同时打开多个 SPICE 连接

- **单元测试**（可选）：
  - `SPICEProxy` 的双向转发逻辑，用本地 TCP echo 服务器模拟目标，验证 WS→TCP 和 TCP→WS 数据一致性

## 依赖

| 包 | 用途 |
|---|---|
| `spice-html5` | 前端 SPICE 客户端（npm 或手动引入） |
| `github.com/gorilla/websocket` | Go WebSocket 服务端（项目已有） |

无需新增 Go 依赖。
