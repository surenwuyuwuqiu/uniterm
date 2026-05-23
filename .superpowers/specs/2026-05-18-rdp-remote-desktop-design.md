# RDP 远程桌面功能设计

## 概述

为 uniTerm 新增 Windows RDP（远程桌面协议）支持，作为一种新的连接类型和 Tab 类型。

## 方案选型

**使用 Windows 原生 ActiveX 控件 `MsTscAx`，通过 Go COM 互操作嵌入到 Wails 窗口。**

- 零新增 Go 依赖（`go-ole` 已是间接依赖）
- 应用本身为 Windows-only（只编译 `windows/amd64`），无需跨平台
- 系统原生 RDP 兼容性，天然支持 NLA、剪贴板、磁盘映射

## 认证

仅支持密码认证。NLA 由 ActiveX 控件默认启用，无需额外配置。

## 功能范围

- 远程桌面画面显示 + 键盘鼠标输入
- 剪贴板共享（本地 ↔ 远程复制粘贴文本）
- 磁盘映射（本地驱动器挂载到远程）
- 分辨率模式：跟随窗口自动调整 / 固定分辨率

## 连接配置

`ConnectionConfig.type` 新增 `'rdp'`。

RDP 专属字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `host` | string | 远程主机地址 |
| `port` | number | 默认 3389 |
| `user` | string | 用户名 |
| `password` | string | 密码 |
| `rdpSizeMode` | `'follow' \| 'fixed'` | 分辨率模式（默认 `follow`） |
| `rdpFixedWidth` | number? | 固定宽度，仅 fixed 模式 |
| `rdpFixedHeight` | number? | 固定高度，仅 fixed 模式 |

固定分辨率预设选项：1280×720、1920×1080、2560×1440、1024×768、1600×1200、1680×1050。

后端 `ConnectionConfig` 同样新增对应字段，Go 结构体用 `int` 存储分辨率值。

## 架构

```
Wails 主窗口
├── WebView2（Vue 前端）
│   ├── Sidebar / ConnectionForm
│   ├── TabBar（RDP Tab 与 SSH/SFTP Tab 并列）
│   └── Tab 内容区
│       ├── RDPTabContent.vue（占位 + 状态提示）
│       │   └── [Go 创建的 ActiveX HWND 子窗口覆盖在此区域]
│       ├── TerminalTabContent（SSH）
│       └── SFTPTabContent
```

Go 后端通过 `SetParent` 将 ActiveX 的 HWND 设为 Wails 窗口的子窗口，由前端监听 resize/Tab 切换事件并调用 Go 方法（`SetRDPPosition` 等）控制显示状态。

## 新增文件

### `backend/session/rdp_session.go`

RDP 会话实现，实现 `Session` 接口：

```
RDPSession struct:
  BaseSession
  hwnd      uintptr        // ActiveX 子窗口句柄
  rdpObj    *ole.IDispatch // MsRdpClient 实例
  config    ConnectionConfig

Connect():
  1. 创建子 HWND（CreateWindowEx + WS_CHILD）
  2. CoCreateInstance(MsTscAx)
  3. 设置属性：Server, UserName, ClearTextPassword,
     DesktopWidth, DesktopHeight, RedirectClipboard,
     RedirectDrives, DisplayConnectionBar, EnableAutoReconnect
  4. 调用 IMsRdpClient.Connect()
  5. 设置 OnConnected/OnDisconnected/OnLoginComplete 事件回调

Disconnect():
  1. IMsRdpClient.Disconnect()
  2. DestroyWindow(hwnd)
  3. 释放 COM 对象

Resize(w, h): 调整 DesktopWidth/Height + SetWindowPos

SetPosition(x, y, w, h): SetWindowPos 移动/缩放子窗口

Show(): ShowWindow(SW_SHOW)
Hide(): ShowWindow(SW_HIDE)
```

COM 事件通过连接点（IConnectionPointContainer）注册回调，状态变化通过 Wails event 通知前端。

### `frontend/src/components/RDPTabContent.vue`

```
状态:
  connecting  → spinner + 目标地址文字
  connected   → 显示区域（填充或居中）+ 底部状态栏
  disconnected → 断开提示 + "重新连接"按钮
  error       → 错误信息 + "重试"按钮

根据 rdpSizeMode:
  follow → div 100% 填充，resize 时调用 SetRDPPosition
  fixed  → div 固定尺寸居中，超出滚动

生命周期:
  onMounted   → 监听 resize，初始化位置
  onUnmounted → 调用 CloseSession
  watch isActive → 切换显示/隐藏
```

## 修改文件

### 前端

| 文件 | 改动 |
|---|---|
| `types/session.ts` | `ConnectionConfig.type` 改为 `'ssh' \| 'rdp'`，增加 rdp 字段 |
| `types/workspace.ts` | `PanelType` 增加 `'rdp'`，新增 `RDPTab` 接口，`Tab` 联合类型新增 `RDPTab` |
| `components/ConnectionForm.vue` | 类型选择器增加 RDP 按钮；`type === 'rdp'` 时显示主机/端口/用户名/密码/分辨率模式/固定分辨率选择；隐藏 SSH 专属字段（authType/keyPath） |
| `components/RDPTabContent.vue` | 新增，上述完整组件 |
| `stores/tabStore.ts` | 新增 `createRDPTab()` 工厂函数 |
| `stores/panelStore.ts` | `createPanel` 支持 `'rdp'` 面板类型 |
| `App.vue` | `onConnect` 按连接 type 分流，RDP 走 `onConnectRDP` 逻辑 |
| `i18n/` | 新增 RDP 相关文案的 zh/en 翻译 |

### 后端

| 文件 | 改动 |
|---|---|
| `session/rdp_session.go` | 新增，实现 `Session` 接口 |
| `session/manager.go` | `Create()` switch 增加 `case "rdp"` |
| `session/session.go` | `ConnectionConfig` 增加 `RdpSizeMode`、`RdpFixedWidth`、`RdpFixedHeight` |
| `app.go` | 新增 `SetRDPPosition`、`SetRDPVisibility` 等 Wails 绑定方法 |

## RDP Tab 与 Workspace 关系

RDP Tab 是独占型 Tab，**不支持**合并到 Workspace 的分割面板中。RDP 画面由原生 HWND 渲染，无法像 xterm.js 那样被 Vue 的 flex/grid 布局自由分割。

在 TabBar 右键菜单中，RDP Tab 不显示"合并到工作区"选项。

## 编译注意事项

- RDP ActiveX (MsTscAx.dll) 是 Windows 内置组件，无需额外安装
- `go-ole` 是纯 Go syscall 实现，COM 调用不依赖 CGO
- 只能在 Windows 上编译和运行

## 待技术验证

以下点需在实现计划阶段确认：

1. **获取 Wails 主窗口 HWND** — 需要找到将 ActiveX 子窗口挂载到 Wails 窗口的可靠方法。可能途径：`runtime.WindowSetRGBA` 相关 API、`FindWindow` + 窗口标题、或 Wails v2 内部的 application 方法。
2. **COM 事件回调** — `IMsRdpClient` 的 `OnConnected`/`OnDisconnected` 事件通过连接点（IConnectionPointContainer）注册，需验证 `go-ole` 能否正确处理 IDispatch 事件 sink。
