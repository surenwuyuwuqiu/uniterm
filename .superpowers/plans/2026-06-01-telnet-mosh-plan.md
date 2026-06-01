# Telnet & Mosh 连接协议 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 SSH 连接基础上新增 Telnet 和 Mosh 两种终端连接协议，三者共享 `ConnectionConfig`，通过 `Type` 字段区分。

**Architecture:** 后端新增 `telnet_session.go` 和 `mosh_session.go`，均实现 `Session` 接口。Telnet 用 Go 标准库 `net.Dial` + 简易 IAC 协商；Mosh 引入 `github.com/unixshells/mosh-go` 纯 Go 库。前端复用 `TerminalTab`，仅在表单和菜单中增加类型入口。

**Tech Stack:** Go 1.25, Vue 3 + TypeScript, Wails v2, github.com/unixshells/mosh-go

---

## 文件结构

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `backend/session/telnet_session.go` | Telnet TCP 会话（IAC 协商、读写循环、NAWS） |
| 新建 | `backend/session/mosh_session.go` | Mosh UDP 会话（SSH 信令 + mosh-go SSP） |
| 修改 | `backend/session/manager.go` | 注册 `"telnet"` / `"mosh"` 工厂分支 |
| 修改 | `frontend/src/types/session.ts` | `ConnectionConfig.type` 扩展 `'telnet' \| 'mosh'` |
| 修改 | `frontend/src/components/ConnectionForm.vue` | 协议选择增加 Telnet/Mosh，动态显示表单字段 |
| 修改 | `frontend/src/components/Sidebar.vue` | 右键菜单增加 Connect Telnet / Connect Mosh |
| 修改 | `frontend/src/App.vue` | `onConnect()` 路由适配新类型 |
| 修改 | `frontend/src/i18n/index.ts` | 增加中英文翻译条目 |

---

### Task 1: 添加 mosh-go 依赖

- [ ] **Step 1: 运行 go get**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go get github.com/unixshells/mosh-go@latest
```

- [ ] **Step 2: 验证 go.mod 更新**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && grep mosh-go go.mod
```

Expected: `github.com/unixshells/mosh-go vX.Y.Z` 出现在 require 块中。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add go.mod go.sum && git commit -m "chore(deps): add github.com/unixshells/mosh-go"
```

---

### Task 2: 实现 TelnetSession

**Files:**
- Create: `backend/session/telnet_session.go`

- [ ] **Step 1: 创建 telnet_session.go**

```go
package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// Telnet protocol constants
	telnetIAC  = 255
	telnetWILL = 251
	telnetWONT = 252
	telnetDO   = 253
	telnetDONT = 254
	telnetSB   = 250
	telnetSE   = 240
	telnetNAWS = 31
)

type TelnetSession struct {
	*baseSession
	conn   net.Conn
	cancel context.CancelFunc
	quit   chan struct{}
	quitOnce sync.Once
}

func NewTelnetSession(id string) *TelnetSession {
	return &TelnetSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "telnet",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
}

func (s *TelnetSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)
	s.title = fmt.Sprintf("%s:%d", config.Host, config.Port)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("telnet dial: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.conn = conn
	s.cancel = cancel
	s.setStatus(StatusConnected)

	if cols, rows := s.GetPendingSize(); cols > 0 && rows > 0 {
		s.sendNAWS(cols, rows)
	}

	go s.readLoop(ctx)
	go s.runPostLoginScript(ctx, config.PostLoginScript)

	return nil
}

func (s *TelnetSession) readLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.conn.Read(buf)
		if n > 0 {
			filtered := s.filterIAC(buf[:n])
			if len(filtered) > 0 {
				s.emitData(filtered)
			}
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n\x1b[31m[read error: %v]\x1b[0m\r\n", err)))
			} else {
				s.emitData([]byte("\r\n\x1b[31mConnection closed by remote host. Press Enter to reconnect.\x1b[0m\r\n"))
			}
			s.Disconnect()
			return
		}
	}
}

func (s *TelnetSession) filterIAC(data []byte) []byte {
	var out []byte
	i := 0
	for i < len(data) {
		if data[i] == telnetIAC && i+1 < len(data) {
			cmd := data[i+1]
			switch cmd {
			case telnetWILL, telnetWONT, telnetDO, telnetDONT:
				// 3-byte negotiation: IAC CMD OPT
				if i+2 < len(data) {
					s.handleNegotiation(cmd, data[i+2])
					i += 3
					continue
				}
			case telnetSB:
				// Sub-negotiation: skip until IAC SE
				for j := i + 2; j < len(data)-1; j++ {
					if data[j] == telnetIAC && data[j+1] == telnetSE {
						i = j + 2
						break
					}
				}
				// If SE not found in this chunk, skip remaining
				i = len(data)
				continue
			case telnetIAC:
				// Escaped 0xFF → literal 0xFF
				out = append(out, telnetIAC)
				i += 2
				continue
			default:
				// Unknown IAC command, skip IAC + cmd
				i += 2
				continue
			}
		} else {
			out = append(out, data[i])
			i++
		}
	}
	return out
}

func (s *TelnetSession) handleNegotiation(cmd byte, opt byte) {
	switch cmd {
	case telnetWILL:
		// Reject most WILL offers by sending DONT
		s.reply(telnetDONT, opt)
	case telnetDO:
		// Reject most DO requests by sending WONT
		if opt == telnetNAWS {
			s.reply(telnetWILL, opt)
		} else {
			s.reply(telnetWONT, opt)
		}
	}
}

func (s *TelnetSession) reply(cmd byte, opt byte) {
	if s.conn == nil {
		return
	}
	s.conn.Write([]byte{telnetIAC, cmd, opt})
}

func (s *TelnetSession) sendNAWS(cols, rows int) {
	if s.conn == nil {
		return
	}
	// IAC SB NAWS <width-high> <width-low> <height-high> <height-low> IAC SE
	data := []byte{
		telnetIAC, telnetSB, telnetNAWS,
		byte(cols >> 8), byte(cols & 0xff),
		byte(rows >> 8), byte(rows & 0xff),
		telnetIAC, telnetSE,
	}
	s.conn.Write(data)
}

func (s *TelnetSession) runPostLoginScript(ctx context.Context, script string) {
	if strings.TrimSpace(script) == "" {
		return
	}
	// Wait briefly for connection to settle
	time.Sleep(1 * time.Second)
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		if s.conn != nil {
			s.conn.Write([]byte(line + "\r\n"))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *TelnetSession) Write(data []byte) error {
	if s.conn == nil {
		return fmt.Errorf("not connected")
	}
	_, err := s.conn.Write(data)
	return err
}

func (s *TelnetSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *TelnetSession) Resize(cols, rows int) error {
	s.SetPendingSize(cols, rows)
	if s.conn == nil {
		return fmt.Errorf("session not connected")
	}
	s.sendNAWS(cols, rows)
	return nil
}

func (s *TelnetSession) IsConnected() bool {
	return s.Status() == StatusConnected
}
```

- [ ] **Step 2: 验证编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/session/...
```

Expected: 编译通过（仅新增类型，无调用方报错）。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add backend/session/telnet_session.go && git commit -m "feat(session): add telnet connection support"
```

---

### Task 3: 注册 Telnet 到 SessionManager

**Files:**
- Modify: `backend/session/manager.go`

- [ ] **Step 1: 在工厂 switch 中增加 telnet 分支**

在 `backend/session/manager.go` 的 `Create` 方法 switch 块中，`case "monitor":` 之后（或 `case "ssh":` 附近）添加：

```
		case "telnet":
			s = NewTelnetSession(config.ID)
```

完整上下文（定位到 `case "monitor":` 行之后）：

```go
		case "monitor":
			s = NewMonitorSession(config.ID)

		case "telnet":
			s = NewTelnetSession(config.ID)

		default:
			return nil, fmt.Errorf("unsupported session type: %s", sessionType)
```

- [ ] **Step 2: 验证编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/session/...
```

Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add backend/session/manager.go && git commit -m "feat(session): register telnet in session manager factory"
```

---

### Task 4: 实现 MoshSession

**Files:**
- Create: `backend/session/mosh_session.go`

- [ ] **Step 1: 创建 mosh_session.go**

Mosh 连接流程：先用 SSH 连接远端启动 `mosh-server new -s`，解析输出的 `MOSH_KEY` 和 `MOSH_PORT`，然后用 mosh-go 建立 UDP SSP 传输。

```go
package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	mosh "github.com/unixshells/mosh-go"
)

type MoshSession struct {
	*baseSession
	moshClient *mosh.Client
	sshClient  *ssh.Client
	rw         io.ReadWriteCloser
	cancel     context.CancelFunc
	quit       chan struct{}
	quitOnce   sync.Once
}

func NewMoshSession(id string) *MoshSession {
	return &MoshSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "mosh",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
}

func (s *MoshSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)
	s.title = fmt.Sprintf("%s@%s (mosh)", config.User, config.Host)

	// Step 1: Establish SSH connection to start mosh-server
	authMethods := makeSSHAuthMethods(config)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := net.DialTimeout("tcp", addr, clientConfig.Timeout)
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("mosh ssh dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		conn.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("mosh ssh handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	// Step 2: Start mosh-server on the remote
	key, udpPort, err := s.startMoshServer(client)
	if err != nil {
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("mosh-server start: %w", err)
	}

	// Step 3: Establish Mosh UDP session
	ctx, cancel := context.WithCancel(context.Background())
	s.sshClient = client
	s.cancel = cancel

	moshConfig := &mosh.ClientConfig{
		Key:        key,
		RemoteAddr: fmt.Sprintf("%s:%d", config.Host, udpPort),
	}

	moshClient, err := mosh.NewClient(moshConfig)
	if err != nil {
		client.Close()
		cancel()
		s.setStatus(StatusError)
		return fmt.Errorf("mosh client: %w", err)
	}

	s.moshClient = moshClient
	s.setStatus(StatusConnected)

	if cols, rows := s.GetPendingSize(); cols > 0 && rows > 0 {
		moshClient.Resize(cols, rows)
	}

	go s.readLoop(ctx)
	go s.runPostLoginScript(ctx, config.PostLoginScript)

	return nil
}

func (s *MoshSession) startMoshServer(client *ssh.Client) (key string, udpPort int, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", 0, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", 0, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Start("mosh-server new -s"); err != nil {
		return "", 0, fmt.Errorf("start mosh-server: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	var output string
	for scanner.Scan() {
		line := scanner.Text()
		output += line + "\n"
	}

	session.Wait()

	// Parse MOSH_KEY and MOSH_PORT from output
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "MOSH_KEY=") {
			key = strings.TrimPrefix(line, "MOSH_KEY=")
		}
		if strings.HasPrefix(line, "MOSH_PORT=") {
			fmt.Sscanf(strings.TrimPrefix(line, "MOSH_PORT="), "%d", &udpPort)
		}
	}

	if key == "" || udpPort == 0 {
		return "", 0, fmt.Errorf("mosh-server output missing key or port: %s", output)
	}

	return key, udpPort, nil
}

func (s *MoshSession) readLoop(ctx context.Context) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := s.moshClient.Read(buf)
		if n > 0 {
			s.emitData(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n\x1b[31m[mosh error: %v]\x1b[0m\r\n", err)))
			} else {
				s.emitData([]byte("\r\n\x1b[31mMosh connection closed. Press Enter to reconnect.\x1b[0m\r\n"))
			}
			s.Disconnect()
			return
		}
	}
}

func (s *MoshSession) runPostLoginScript(ctx context.Context, script string) {
	if strings.TrimSpace(script) == "" {
		return
	}
	time.Sleep(2 * time.Second)
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		if s.moshClient != nil {
			s.moshClient.Write([]byte(line + "\r"))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *MoshSession) Write(data []byte) error {
	if s.moshClient == nil {
		return fmt.Errorf("not connected")
	}
	_, err := s.moshClient.Write(data)
	return err
}

func (s *MoshSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	if s.cancel != nil {
		s.cancel()
	}
	if s.moshClient != nil {
		s.moshClient.Close()
	}
	if s.sshClient != nil {
		s.sshClient.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *MoshSession) Resize(cols, rows int) error {
	s.SetPendingSize(cols, rows)
	if s.moshClient == nil {
		return fmt.Errorf("session not connected")
	}
	s.moshClient.Resize(cols, rows)
	return nil
}

func (s *MoshSession) IsConnected() bool {
	return s.Status() == StatusConnected
}

// makeSSHAuthMethods builds SSH auth methods from ConnectionConfig.
// Shared helper used by ssh_session.go, sftp_session.go, monitor_session.go, mosh_session.go.
func makeSSHAuthMethods(config ConnectionConfig) []ssh.AuthMethod {
	var authMethods []ssh.AuthMethod

	switch config.AuthType {
	case "password":
		authMethods = append(authMethods, ssh.Password(config.Password))
	case "key":
		key, err := os.ReadFile(config.KeyPath)
		if err != nil {
			authMethods = append(authMethods, ssh.Password(config.Password))
			break
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			authMethods = append(authMethods, ssh.Password(config.Password))
			break
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	case "agent":
		authMethods = append(authMethods, ssh.Password(config.Password))
	default:
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	return authMethods
}
```

**注意**: `mosh-go` 的具体 API（`mosh.Client`, `mosh.ClientConfig`, `NewClient`, `Read`, `Write`, `Resize`, `Close`）需在实际编译时对照库的导出类型进行微调。上述代码基于库的 README 描述的接口编写。如果 API 有差异（例如使用 `ServeRW` 模式），届时调整即可。

- [ ] **Step 2: 验证编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./backend/session/...
```

Expected: 编译通过。若 mosh-go API 不匹配，根据编译错误调整 struct/方法名。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add backend/session/mosh_session.go && git commit -m "feat(session): add mosh connection support"
```

---

### Task 5: 注册 Mosh 到 SessionManager

**Files:**
- Modify: `backend/session/manager.go`

- [ ] **Step 1: 在工厂 switch 中增加 mosh 分支**

在 `backend/session/manager.go` 的 `Create` 方法 switch 块中，`case "telnet":` 之后添加：

```go
		case "mosh":
			s = NewMoshSession(config.ID)
```

- [ ] **Step 2: 验证编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```

Expected: 全项目编译通过。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add backend/session/manager.go && git commit -m "feat(session): register mosh in session manager factory"
```

---

### Task 6: 重构 SSH 认证为共享 helper

**Files:**
- Modify: `backend/session/ssh_session.go:49-69`
- Create: `backend/session/ssh_auth.go`

- [ ] **Step 1: 创建 ssh_auth.go 并提取认证逻辑**

注意：`makeSSHAuthMethods` 已在 Task 4 的 `mosh_session.go` 中定义。此步将其移到独立文件以避免重复。

```go
// backend/session/ssh_auth.go
package session

import (
	"os"

	"golang.org/x/crypto/ssh"
)

func makeSSHAuthMethods(config ConnectionConfig) []ssh.AuthMethod {
	var authMethods []ssh.AuthMethod

	switch config.AuthType {
	case "password":
		authMethods = append(authMethods, ssh.Password(config.Password))
	case "key":
		key, err := os.ReadFile(config.KeyPath)
		if err != nil {
			authMethods = append(authMethods, ssh.Password(config.Password))
			break
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			authMethods = append(authMethods, ssh.Password(config.Password))
			break
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	case "agent":
		authMethods = append(authMethods, ssh.Password(config.Password))
	default:
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	return authMethods
}
```

同时从 `mosh_session.go` 中**删除**重复的 `makeSSHAuthMethods` 函数。

- [ ] **Step 2: 修改 ssh_session.go 使用共享 helper**

将 `backend/session/ssh_session.go` 第 49-69 行（`authMethods` 构建逻辑）替换为：

```go
	authMethods := makeSSHAuthMethods(config)
```

即删除原来的 switch 块，只保留这一行调用。

- [ ] **Step 3: 验证编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && go build ./...
```

Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add backend/session/ssh_auth.go backend/session/mosh_session.go backend/session/ssh_session.go && git commit -m "refactor(session): extract SSH auth methods to shared helper"
```

---

### Task 7: 扩展前端类型定义

**Files:**
- Modify: `frontend/src/types/session.ts:11`

- [ ] **Step 1: 扩展 ConnectionConfig.type 联合类型**

将 `frontend/src/types/session.ts` 第 11 行：

```ts
  type: 'ssh' | 'rdp' | 'vnc' | 'database'
```

改为：

```ts
  type: 'ssh' | 'telnet' | 'mosh' | 'rdp' | 'vnc' | 'database'
```

- [ ] **Step 2: 验证 TypeScript 编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npm run type-check 2>&1 || npx vue-tsc --noEmit 2>&1 || npx tsc --noEmit 2>&1
```

Expected: 无新增类型错误（`'telnet' | 'mosh'` 类型的扩散可能暴露下游 match 不完整，后续任务逐一修复）。

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add frontend/src/types/session.ts && git commit -m "feat(types): add telnet and mosh to ConnectionConfig type"
```

---

### Task 8: 修改 ConnectionForm 表单

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue`

- [ ] **Step 1: 协议选择按钮增加 Telnet / Mosh**

将 type radio-group（约第 26-31 行）改为：

```html
<el-form-item :label="t('conn.type')">
  <el-radio-group v-model="form.type">
    <el-radio-button label="ssh">SSH</el-radio-button>
    <el-radio-button label="telnet">Telnet</el-radio-button>
    <el-radio-button label="mosh">Mosh</el-radio-button>
    <el-radio-button label="rdp" v-if="isWindows">RDP</el-radio-button>
    <el-radio-button label="vnc">VNC</el-radio-button>
    <el-radio-button label="database">{{ t('db.database') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
```

- [ ] **Step 2: 调整字段可见性条件**

认证类型选择（约第 49 行）: 将 `v-if="form.type === 'ssh'"` 改为 `v-if="form.type === 'ssh' || form.type === 'mosh'"`

密码字段（约第 55 行）:

```html
<el-form-item v-if="(form.authType === 'password' || form.type === 'rdp' || form.type === 'vnc' || form.type === 'database' || form.type === 'mosh') && !(form.type === 'database' && form.dbType === 'rqlite')" :label="t('conn.password')">
```

密钥字段（约第 58 行）: 将 `v-if="form.authType === 'key' && form.type === 'ssh'"` 改为 `v-if="form.authType === 'key' && (form.type === 'ssh' || form.type === 'mosh')"`

PostLoginScript（约第 79 行）: 将 `v-if="form.type === 'ssh'"` 改为 `v-if="form.type === 'ssh' || form.type === 'telnet' || form.type === 'mosh'"`

用户名字段（约第 46-48 行）保持不变，telnet 场景下用户留空即可。

- [ ] **Step 3: 调整默认端口自动切换**

在 `watch(() => form.type, ...)` 中增加 telnet/mosh 默认端口逻辑：

在现有 `else if (newType === 'vnc' ...)` 块附近增加：

```ts
  else if (newType === 'telnet' && form.port === 22) form.port = 23
  else if (newType === 'mosh' && form.port !== 22) form.port = 22
```

- [ ] **Step 4: 更新 resetForm 的默认端口跟随类型**

在 `resetForm()` 中保持 `form.port = 22`（SSH/Mosh 共用），无需单独处理（用户切换类型时已有 watch 调整）。

- [ ] **Step 5: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add frontend/src/components/ConnectionForm.vue && git commit -m "feat(ui): add telnet/mosh options to connection form"
```

---

### Task 9: 修改 Sidebar 右键菜单

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 增加上下文菜单项**

将第 172-175 行的 context menu 改为（telnet/mosh 复用 SSH connect 的同时各自也可有独立入口，但按设计三者共享 `connect` emit）：

```html
<div v-if="selectedConn && (selectedConn.type === 'ssh' || selectedConn.type === 'telnet' || selectedConn.type === 'mosh')" class="menu-item" @click="doConnect">{{ connectLabel }}</div>
<div v-if="selectedConn && selectedConn.type === 'ssh'" class="menu-item" @click="doConnectSFTP">{{ t('sidebar.connectSftp') }}</div>
<div v-if="selectedConn && selectedConn.type === 'ssh'" class="menu-item" @click="doConnectMonitor">{{ t('sidebar.connectMonitor') }}</div>
```

- [ ] **Step 2: 更新 connectLabel computed**

`connectLabel` computed（约第 638 行）增加新类型的文本：

```ts
const connectLabel = computed(() => {
  if (selectedConn.value?.type === 'telnet') return t('sidebar.connectTelnet')
  if (selectedConn.value?.type === 'mosh') return t('sidebar.connectMosh')
  if (selectedConn.value?.type === 'ssh') return t('sidebar.connectSSH')
  return t('sidebar.connect')
})
```

- [ ] **Step 3: 更新回车/双击路由**

在 `onListKeydown`（约第 503-525 行）和 `onItemDblClick`（约第 607-618 行）中，telnet/mosh 走 `emit('connect', c)` 分支（与 SSH 相同），无需改动。

在 `onConnectFromForm`（约第 996-1013 行）中，telnet/mosh 走最后的 `else` → `emit('connect', config)` 分支，无需改动。

- [ ] **Step 4: 更新 emit 声明**

`defineEmits` 声明无需改动，telnet/mosh 复用 `connect` 事件。

- [ ] **Step 5: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add frontend/src/components/Sidebar.vue && git commit -m "feat(ui): add telnet/mosh to sidebar context menu"
```

---

### Task 10: 添加中英文翻译

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: 在中文翻译块中添加**

在中文 `'sidebar.connectMonitor': '服务器监控'` 附近添加：

```ts
'sidebar.connectTelnet': '连接 Telnet',
'sidebar.connectMosh': '连接 Mosh',
```

- [ ] **Step 2: 在英文翻译块中添加**

在英文 `'sidebar.connectMonitor': 'Server Monitor'` 附近添加：

```ts
'sidebar.connectTelnet': 'Connect Telnet',
'sidebar.connectMosh': 'Connect Mosh',
```

- [ ] **Step 3: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add frontend/src/i18n/index.ts && git commit -m "feat(i18n): add telnet/mosh translations"
```

---

### Task 11: 调通 App.vue 连接路由

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: 检查 onConnect 路由**

当前 `App.vue` 的 `onConnect` 函数（第 386-406 行）中：

```ts
async function onConnect(config: ConnectionConfig) {
  if (config.type === 'rdp') return onConnectRDP(config)
  if (config.type === 'vnc') return onConnectVNC(config)
  if (config.type === 'database') return onConnectDB(config)
  // ssh, telnet, mosh all fall through to here:
  connectionStore.add(config)
  const panel = panelStore.createPanel(config, 'ssh')
  // ...
```

telnet 和 mosh 会走到最后的通用分支，`panelStore.createPanel(config, 'ssh')` 中第二个参数 `'ssh'` 是显示用，无需区分。但建议改为按 config.type 传递，以保持一致性：

```ts
async function onConnect(config: ConnectionConfig) {
  if (config.type === 'rdp') return onConnectRDP(config)
  if (config.type === 'vnc') return onConnectVNC(config)
  if (config.type === 'database') return onConnectDB(config)
  connectionStore.add(config)
  const panel = panelStore.createPanel(config, config.type)
  const displayTitle = config.name || (config.type === 'telnet'
    ? `${config.host}:${config.port}`
    : `${config.user}@${config.host}`)
  panel.title = displayTitle
  const tab = tabStore.createTerminalTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession(config.type, config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}
```

- [ ] **Step 2: 更新 closeTab 中的 terminal 关闭逻辑**

`closeTab`（第 331-372 行）中 terminal 类型已是通用处理：

```ts
if (tab && tab.type === 'terminal') {
    const p = panelStore.getPanel(tab.panelId)
    if (p?.type === 'local' && p?.sessionId) {
      try { await CloseSession(p.sessionId) } catch (_) {}
    }
  }
```

telnet/mosh 的 panel type 不是 `'local'`（在 createPanel 时传的是 config.type），不会被此分支关闭。需要改为关闭所有 terminal panel 的 session（非 local 也关）：

```ts
  if (tab && tab.type === 'terminal') {
    const p = panelStore.getPanel(tab.panelId)
    if (p?.sessionId) {
      try { await CloseSession(p.sessionId) } catch (_) {}
    }
  }
```

- [ ] **Step 3: 验证前端编译**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm/frontend && npm run build 2>&1 || npx vue-tsc --noEmit 2>&1
```

Expected: 无编译/类型错误。

- [ ] **Step 4: Commit**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add frontend/src/App.vue && git commit -m "feat(app): wire telnet/mosh connection routing"
```

---

### Task 12: 端到端构建验证

- [ ] **Step 1: 完整构建**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && wails build
```

Expected: 构建成功，生成 `uniterm.exe`，大小增加约 1-2MB。

- [ ] **Step 2: 运行应用验证 Telnet**

启动应用 → 新建 Telnet 连接 → 填写 host/port（可用 `towel.blinkenlights.nl:23` 测试）→ 连接，确认终端正常显示 ASCII 动画。

- [ ] **Step 3: 运行应用验证 Mosh**

连接到安装了 mosh-server 的服务器 → 确认终端交互正常。

- [ ] **Step 4: Commit（如有构建产物配置变动）**

```bash
cd c:/Users/yowsa/Documents/workspace/uniterm && git add -A && git commit -m "chore(build): finalize telnet/mosh integration"
```
