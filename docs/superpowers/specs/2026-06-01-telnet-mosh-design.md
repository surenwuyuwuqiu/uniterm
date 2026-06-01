# Telnet & Mosh 连接协议支持 — 设计文档

**日期**: 2026-06-01  
**分支**: `feature/mosh-telnet`

---

## 1. 概述

在现有 SSH 连接基础上，新增 Telnet 和 Mosh 两种终端连接协议。三种协议共享 `ConnectionConfig` 结构，通过 `Type` 字段区分，减少 API 层面的概念膨胀。

---

## 2. 协议与实现路线

| 协议 | 实现方式 | 新增依赖 |
|------|---------|---------|
| **Telnet** | Go 标准库 `net.Dial` + 简易 IAC 协商过滤 | 无 |
| **Mosh** | 引入 `github.com/unixshells/mosh-go` 纯 Go 库 | `mosh-go` + charmbracelet 系列（预估 +1-2MB 二进制） |

两个新会话类型均实现 `Session` 接口，嵌入 `baseSession`。

---

## 3. ConnectionConfig 字段共享

三个协议共享同一套字段，通过 `Type` 路由：

```
Type = "ssh"     → SSH 连接，使用全部认证字段
Type = "telnet"  → TCP 直连，仅使用 Host / Port / PostLoginScript
Type = "mosh"    → SSH 信令 + UDP SSP，使用全部 SSH 字段，远端需有 mosh-server
```

| 字段 | SSH | Telnet | Mosh |
|------|-----|--------|------|
| Type | `"ssh"` | `"telnet"` | `"mosh"` |
| Host | ✓ | ✓ | ✓ |
| Port | ✓ (default 22) | ✓ (default 23) | ✓ (default 22) |
| User | ✓ | — | ✓ |
| AuthType | ✓ | — | ✓ |
| Password | ✓ | — | ✓ |
| KeyPath | ✓ | — | ✓ |
| PostLoginScript | ✓ | ✓ | ✓ |

`ConnectionConfig` 本身**不新增字段**。各 session 实现按 `Type` 自行决定从 config 中读取哪些字段参与连接（telnet 忽略认证字段，mosh/ssh 使用全部字段）。

---

## 4. 后端改动

### 4.1 新文件

**`backend/session/telnet_session.go`**

```
TelnetSession struct {
    *baseSession
    conn   net.Conn
    cancel context.CancelFunc
}
```

- `Connect(config)` → `net.Dial("tcp", host:port)`，启动读循环 goroutine
- 读循环做简易 IAC 协商处理：响应 WILL/WON'T/DO/DON'T，过滤协商字节，仅向 emmit data 输出纯数据
- `Write(data)` → `conn.Write(data)`（直接透传，不做协商编码）
- `Disconnect()` → `conn.Close()` + `cancel()`
- `Resize()` → 发送 NAWS 协商（IAC SB NAWS width height IAC SE）

**`backend/session/mosh_session.go`**

```
MoshSession struct {
    *baseSession
    moshClient *mosh.Client
    cancel     context.CancelFunc
}
```

- `Connect(config)` →
  1. 先用 `golang.org/x/crypto/ssh` 建立 SSH 连接（复用现有 ssh_session 的认证逻辑提取为 helper）
  2. 通过 SSH 会话启动远端 `mosh-server new -s`
  3. 解析 mosh-server 输出的 `MOSH_KEY` 和 `MOSH_PORT`
  4. 使用 `mosh-go` 库建立 UDP SSP 会话
  5. 将 mosh-go 的输出接入 `emitData`
- `Write(data)` → mosh client 写入
- `Resize()` → 通过 mosh 协议发送窗口大小变更
- `Disconnect()` → 关闭 mosh 会话 + SSH 通道 + `cancel()`

### 4.2 改动文件

**`backend/session/manager.go`** — `Create()` 方法：

```go
case "telnet":
    return &TelnetSession{baseSession: newBaseSession(sessionType)}, nil
case "mosh":
    return &MoshSession{baseSession: newBaseSession(sessionType)}, nil
```

**`backend/session/session.go`** — 不需要改结构体，无需新增字段。

**`app.go`** — 不需要改，`CreateSession` 已通用。

### 4.3 SSH 认证逻辑复用

将 `ssh_session.go` 中的 SSH 客户端创建和认证逻辑提取为内部 helper 函数（`ssh_auth.go`）：

```go
func createSSHClient(config ConnectionConfig) (*ssh.Client, error)
```

供 `ssh_session.go`、`sftp_session.go`、`monitor_session.go`、`mosh_session.go` 共同使用。

---

## 5. 前端改动

### 5.1 类型扩展

**`frontend/src/types/session.ts`**：

```ts
export type ConnectionType = 'ssh' | 'rdp' | 'vnc' | 'database' | 'telnet' | 'mosh'
```

### 5.2 表单

**`frontend/src/components/ConnectionForm.vue`**：

- 协议下拉框增加 "Telnet" / "Mosh" 选项
- 选中 Telnet 时：只显示 Host / Port / PostLoginScript，隐藏认证相关字段
- 选中 Mosh 时：完整显示 SSH 认证字段（和 SSH 表单一致）
- 默认端口随协议切换：SSH=22, Telnet=23, Mosh=22

### 5.3 路由

**`frontend/src/components/Sidebar.vue`**：右键菜单增加 "Connect Telnet" / "Connect Mosh"

**`frontend/src/App.vue`**：telnet/mosh 连接处理复用 `onConnect()` 逻辑，走 `CreateSession(type, config)` → 创建 `TerminalTab`

---

## 6. 复用已有的部分

- `TerminalTab`（不新增 tab 类型）
- `PanelType.terminal`（不新增 panel 类型）
- `ConnectionConfig`（不新增字段）
- `SessionManager` 工厂模式
- `App.CreateSession` 绑定
- `connectionStore` / `sessionStore` Pinia stores

---

## 7. 错误处理

- Telnet 连接失败：TCP dial error → `setStatus(SessionError)` → 前端展示错误
- Mosh SSH 信令失败：复用 SSH 认证错误处理
- Mosh mosh-server 未安装：解析输出时检测 `command not found`，返回明确错误信息
- Mosh UDP 无法建立：mosh-go 库超时 → 返回错误

---

## 8. 测试要点

- Telnet 连接到公共 Telnet 服务（如 `towel.blinkenlights.nl:23`）
- Mosh 连接到安装了 mosh-server 的 Linux 服务器
- PostLoginScript 在 telnet 下的自动发送
- 窗口 resize 在 telnet（NAWS）和 mosh 下正确传递
- 会话断开后的资源清理（conn + context）
