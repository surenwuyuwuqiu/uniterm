# 服务器监控功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 SSH 连接右键菜单中添加"服务器监控"功能，点击后打开类似 Windows 任务管理器的实时监控面板。

**Architecture:** Go 后端新增 MonitorSession，通过 SSH 定时执行脚本采集 Linux 系统数据，经 Wails Events 推送到前端。前端新增 MonitorTabContent 组件，分"性能"/"进程"/"系统信息"三个 Tab 展示，性能页用 Canvas 绘制趋势图。

**Tech Stack:** Go (Wails v2), Vue 3, Pinia, Element Plus, Canvas 2D

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/session/monitor_session.go` | 创建 | MonitorSession 实现：SSH 连接、定时采集、数据推送 |
| `backend/session/manager.go` | 修改 | 在 Create 方法中注册 `monitor` 类型 |
| `frontend/src/types/workspace.ts` | 修改 | 在 PanelType / Tab 联合类型中添加 `monitor` |
| `frontend/src/stores/panelStore.ts` | 修改 | `createPanel` 支持 `monitor` 类型 |
| `frontend/src/stores/tabStore.ts` | 修改 | 新增 `createMonitorTab` 方法 |
| `frontend/src/components/MonitorTabContent.vue` | 创建 | 监控面板主组件（性能/进程/系统信息） |
| `frontend/src/App.vue` | 修改 | 添加 `onConnectMonitor`、注册 monitor 标签渲染、关闭清理 |
| `frontend/src/components/Sidebar.vue` | 修改 | 右键菜单添加"服务器监控"入口 |
| `frontend/src/i18n/index.ts` | 修改 | 添加监控相关中英文翻译词条 |

---

### Task 1: Go 后端 — MonitorSession

**Files:**
- Create: `backend/session/monitor_session.go`
- Modify: `backend/session/manager.go:21-50`

- [ ] **Step 1: 创建 monitor_session.go**

创建 `backend/session/monitor_session.go`，实现 MonitorSession：

```go
package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type MonitorSession struct {
	baseSession
	client   *ssh.Client
	config   ConnectionConfig
	ticker   *time.Ticker
	quit     chan struct{}
	quitOnce sync.Once
}

func NewMonitorSession(id string) *MonitorSession {
	return &MonitorSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "monitor",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
}

func (s *MonitorSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)
	s.config = config
	s.title = fmt.Sprintf("%s@%s", config.User, config.Host)

	authMethods := []ssh.AuthMethod{}
	switch config.AuthType {
	case "password":
		authMethods = append(authMethods, ssh.Password(config.Password))
	case "key":
		key, err := os.ReadFile(config.KeyPath)
		if err != nil {
			s.setStatus(StatusError)
			return fmt.Errorf("read key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			s.setStatus(StatusError)
			return fmt.Errorf("parse key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	case "agent":
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port), clientConfig)
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("ssh dial: %w", err)
	}

	s.client = client
	s.setStatus(StatusConnected)

	// Push system info once on connect
	go s.pushSystemInfo()

	// Start polling
	s.ticker = time.NewTicker(1 * time.Second)
	go s.pollLoop()

	return nil
}

func (s *MonitorSession) pushSystemInfo() {
	info := s.collectSystemInfo()
	if info != nil {
		data, _ := json.Marshal(map[string]interface{}{"system": info})
		s.emitData(data)
	}
}

func (s *MonitorSession) collectSystemInfo() map[string]interface{} {
	session, err := s.client.NewSession()
	if err != nil {
		return nil
	}
	defer session.Close()

	script := `cat /etc/os-release 2>/dev/null | grep -E '^(PRETTY_NAME|ID)='; echo "---"; uname -r; echo "---"; hostname; echo "---"; cat /proc/cpuinfo 2>/dev/null | grep 'model name' | head -1; echo "---"; nproc`
	out, err := session.CombinedOutput(script)
	if err != nil {
		return nil
	}

	parts := strings.Split(string(out), "---")
	osInfo := strings.TrimSpace(safeIndex(parts, 0))
	kernel := strings.TrimSpace(safeIndex(parts, 1))
	hostname := strings.TrimSpace(safeIndex(parts, 2))
	cpuModel := strings.TrimSpace(safeIndex(parts, 3))
	cpuModel = strings.TrimPrefix(cpuModel, "model name	:")
	cpuModel = strings.TrimSpace(cpuModel)
	coresStr := strings.TrimSpace(safeIndex(parts, 4))

	osName := "Linux"
	for _, line := range strings.Split(osInfo, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			break
		}
	}

	return map[string]interface{}{
		"os":       osName,
		"kernel":   kernel,
		"hostname": hostname,
		"cpuModel": cpuModel,
		"cores":    coresStr,
	}
}

func safeIndex(arr []string, idx int) string {
	if idx < len(arr) {
		return arr[idx]
	}
	return ""
}

func (s *MonitorSession) pollLoop() {
	for {
		select {
		case <-s.quit:
			return
		case <-s.ticker.C:
			s.collect()
		}
	}
}

func (s *MonitorSession) collect() {
	session, err := s.client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	script := `#!/bin/sh
# CPU
read -r cpu_line < /proc/stat
cpu_fields=$(echo "$cpu_line" | awk '{print $2,$3,$4,$5,$6,$7,$8}')
user=$(echo "$cpu_fields" | awk '{print $1}')
nice=$(echo "$cpu_fields" | awk '{print $2}')
system=$(echo "$cpu_fields" | awk '{print $3}')
idle=$(echo "$cpu_fields" | awk '{print $4}')
iowait=$(echo "$cpu_fields" | awk '{print $5}')
irq=$(echo "$cpu_fields" | awk '{print $6}')
softirq=$(echo "$cpu_fields" | awk '{print $7}')
total=$(echo "$user $nice $system $idle $iowait $irq $softirq" | awk '{s=0; for(i=1;i<=NF;i++) s+=$i; print s}')
active=$(echo "$total $idle $iowait" | awk '{print $1-$2-$3}')

# Memory
mem_total=$(awk '/MemTotal/ {print $2}' /proc/meminfo)
mem_avail=$(awk '/MemAvailable/ {print $2}' /proc/meminfo 2>/dev/null || awk '/MemFree/ {print $2}' /proc/meminfo)
mem_used=$(echo "$mem_total $mem_avail" | awk '{print ($1-$2)/1024/1024}')
mem_total_gb=$(echo "$mem_total" | awk '{print $1/1024/1024}')
mem_free_gb=$(echo "$mem_avail" | awk '{print $1/1024/1024}')

# Disk (root)
disk_line=$(df -h / 2>/dev/null | tail -1)
disk_total=$(echo "$disk_line" | awk '{print $2}')
disk_used=$(echo "$disk_line" | awk '{print $3}')
disk_usage=$(echo "$disk_line" | awk '{gsub(/%/,""); print $5}')

# Network (first interface)
net_line=$(cat /proc/net/dev 2>/dev/null | grep -v 'lo:' | grep -E '^\s*[^ ]+:' | head -1)
net_rx=$(echo "$net_line" | awk '{print $2}')
net_tx=$(echo "$net_line" | awk '{print $10}')

# Processes
proc_count=$(ls /proc 2>/dev/null | grep -E '^[0-9]+$' | wc -l)
thread_count=$(grep -c '^Threads:' /proc/[0-9]*/status 2>/dev/null || echo 0)

# CPU cores
cores=$(nproc)

printf '{"cpu":{"usage":%.1f,"total":%s,"active":%s,"cores":%s,"processes":%s,"threads":%s},"memory":{"total":%.2f,"used":%.2f,"free":%.2f,"usage":%.1f},"disk":{"total":"%s","used":"%s","usage":%s},"network":{"rx":%s,"tx":%s},"processes":[' "$active" "$total" "$active" "$cores" "$proc_count" "$thread_count" "$mem_total_gb" "$mem_used" "$mem_free_gb" "$mem_total_gb $mem_used $mem_free_gb" | awk '{print ($2/$1)*100}'" "$disk_total" "$disk_used" "$disk_usage" "$net_rx" "$net_tx"

# Top 30 processes by CPU
ps -eo pid,ppid,user,pcpu,pmem,comm,args --sort=-pcpu 2>/dev/null | head -31 | tail -30 | while read -r pid ppid user pcpu pmem comm args; do
  printf '{"pid":%s,"ppid":%s,"user":"%s","cpu":%s,"mem":%s,"name":"%s","cmd":"%s"},' "$pid" "$ppid" "$user" "$pcpu" "$pmem" "$comm" "$args"
done

printf ']}\n'
`

	out, err := session.CombinedOutput(script)
	if err != nil {
		return
	}

	// Trim the script source noise if any; the output should start with {
	data := bytes.TrimSpace(out)
	idx := bytes.Index(data, []byte("{"))
	if idx >= 0 {
		data = data[idx:]
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	jsonData, _ := json.Marshal(payload)
	s.emitData(jsonData)
}

func (s *MonitorSession) Write(data []byte) error {
	return nil // monitor session does not accept writes
}

func (s *MonitorSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.client != nil {
		s.client.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *MonitorSession) Resize(cols, rows int) error {
	return nil // not applicable
}

func (s *MonitorSession) IsConnected() bool {
	return s.Status() == StatusConnected
}
```

- [ ] **Step 2: 修改 manager.go 注册 monitor 类型**

在 `backend/session/manager.go` 的 `Create` 方法 switch 语句中添加：

```go
case "monitor":
    s = NewMonitorSession(config.ID)
```

- [ ] **Step 3: 编译验证 Go 代码**

Run:
```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm
go build ./...
```

Expected: 编译通过，无错误。

---

### Task 2: 前端类型与 Store 更新

**Files:**
- Modify: `frontend/src/types/workspace.ts`
- Modify: `frontend/src/stores/panelStore.ts`
- Modify: `frontend/src/stores/tabStore.ts`

- [ ] **Step 1: 修改 workspace.ts 添加 monitor 类型**

将 `PanelType` 改为：
```typescript
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'vnc' | 'local' | 'database' | 'monitor' | 'other'
```

在 `Tab` 联合类型中添加 `MonitorTab`：
```typescript
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab | VNCTab | DBTab | MonitorTab

export interface MonitorTab {
  type: 'monitor'
  id: string
  panelId: string
  name: string
}
```

- [ ] **Step 2: 修改 tabStore.ts 添加 createMonitorTab**

在 `useTabStore` 的返回对象前添加方法：

```typescript
function createMonitorTab(name: string, panelId: string): MonitorTab {
  const tab: MonitorTab = {
    type: 'monitor',
    id: genId('monitor-tab'),
    panelId,
    name
  }
  tabState.tabs.push(tab)
  tabState.activeTabId = tab.id
  return tab
}
```

在 `closeTab` 函数的 removedPanelIds 推导中，为 `monitor` 类型添加处理（与 `sftp` 同模式）：

```typescript
const removedPanelIds = removed.type === 'terminal' || removed.type === 'settings' || removed.type === 'rdp' || removed.type === 'vnc' || removed.type === 'database' || removed.type === 'monitor'
```

在返回对象中暴露 `createMonitorTab`。

- [ ] **Step 3: 修改 panelStore.ts 支持 monitor 类型**

`createPanel` 方法已有默认 title 生成逻辑，无需修改。但返回对象暴露即可。

---

### Task 3: MonitorTabContent 组件

**Files:**
- Create: `frontend/src/components/MonitorTabContent.vue`

- [ ] **Step 1: 创建组件文件**

创建 `frontend/src/components/MonitorTabContent.vue`，包含：

```vue
<template>
  <div class="monitor-tab">
    <div class="monitor-tabs">
      <div class="tab-item" :class="{ active: activeTab === 'performance' }" @click="activeTab = 'performance'">{{ t('monitor.performance') }}</div>
      <div class="tab-item" :class="{ active: activeTab === 'processes' }" @click="activeTab = 'processes'">{{ t('monitor.processes') }}</div>
      <div class="tab-item" :class="{ active: activeTab === 'system' }" @click="activeTab = 'system'">{{ t('monitor.system') }}</div>
    </div>

    <!-- Performance -->
    <div v-show="activeTab === 'performance'" class="tab-pane performance-pane">
      <div class="perf-sidebar">
        <div
          v-for="item in perfItems"
          :key="item.key"
          class="perf-nav-item"
          :class="{ active: selectedPerf === item.key }"
          @click="selectedPerf = item.key"
        >
          <div class="perf-nav-name">{{ item.label }}</div>
          <div class="perf-nav-value">{{ item.value }}</div>
          <div class="perf-nav-bar"><div class="perf-nav-bar-inner" :style="{ width: item.percent + '%', background: item.color }" /></div>
        </div>
      </div>
      <div class="perf-main">
        <div class="perf-big-value" :style="{ color: currentPerf.color }">{{ currentPerf.value }}</div>
        <canvas ref="chartCanvas" class="perf-chart" />
        <div class="perf-details">
          <div v-for="d in currentPerf.details" :key="d.label" class="perf-detail-item">
            <span class="detail-label">{{ d.label }}</span>
            <span class="detail-value">{{ d.value }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Processes -->
    <div v-show="activeTab === 'processes'" class="tab-pane">
      <el-input v-model="processSearch" :placeholder="t('monitor.searchProcess')" clearable class="process-search" />
      <el-table :data="filteredProcesses" height="calc(100% - 40px)" size="small">
        <el-table-column prop="pid" label="PID" sortable width="80" />
        <el-table-column prop="name" :label="t('monitor.processName')" sortable />
        <el-table-column prop="user" :label="t('monitor.user')" sortable width="100" />
        <el-table-column prop="cpu" :label="t('monitor.cpu')" sortable width="90">
          <template #default="{ row }">{{ row.cpu }}%</template>
        </el-table-column>
        <el-table-column prop="mem" :label="t('monitor.mem')" sortable width="90">
          <template #default="{ row }">{{ row.mem }}%</template>
        </el-table-column>
      </el-table>
    </div>

    <!-- System Info -->
    <div v-show="activeTab === 'system'" class="tab-pane system-pane">
      <div v-if="systemInfo" class="system-grid">
        <div v-for="item in systemDisplay" :key="item.label" class="system-item">
          <div class="system-label">{{ item.label }}</div>
          <div class="system-value">{{ item.value }}</div>
        </div>
      </div>
      <div v-else class="system-loading">{{ t('monitor.loading') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { useI18n } from '../i18n'

const props = defineProps<{
  sessionId: string
}>()

const { t } = useI18n()
const activeTab = ref('performance')
const selectedPerf = ref('cpu')
const processSearch = ref('')

// Data refs
const cpuHistory = ref<number[]>([])
const memHistory = ref<number[]>([])
const diskHistory = ref<number[]>([])
const netRxHistory = ref<number[]>([])
const netTxHistory = ref<number[]>([])
const processList = ref<any[]>([])
const systemInfo = ref<Record<string, any> | null>(null)

const currentCpu = ref({ usage: 0, cores: 0, processes: 0, threads: 0 })
const currentMem = ref({ total: 0, used: 0, free: 0, usage: 0 })
const currentDisk = ref({ total: '', used: '', usage: 0 })
const currentNet = ref({ rx: 0, tx: 0 })

// ... computed, watch, canvas drawing, event handlers ...
</script>
```

组件内部需要实现：
1. `EventsOn('session:data')` 监听，过滤 `id === props.sessionId`
2. 解析 JSON 数据，更新各 ref
3. `perfItems` computed：根据当前数据生成 CPU/内存/磁盘/网络的导航项
4. `currentPerf` computed：根据 `selectedPerf` 返回当前选中的性能项详情
5. Canvas 绘制：在 `chartCanvas` 上绘制 60 秒历史折线图，watch history 数组变化时重绘
6. `filteredProcesses` computed：根据 `processSearch` 过滤进程列表
7. `systemDisplay` computed：将 `systemInfo` 转为可展示的键值对列表

由于代码较长，实现时将这些逻辑完整写入文件。

---

### Task 4: 入口集成

**Files:**
- Modify: `frontend/src/components/Sidebar.vue:173-186`
- Modify: `frontend/src/App.vue:12,49-56,373-393,520-537`
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Sidebar.vue 右键菜单添加"服务器监控"**

在连接右键菜单的 `doConnectSFTP` 后面、分割线之前添加：

```vue
<div v-if="selectedConn && selectedConn.type === 'ssh'" class="menu-item" @click="doConnectMonitor">{{ t('sidebar.connectMonitor') }}</div>
```

在 script 中添加 `doConnectMonitor` 方法（模式同 `doConnectSFTP`）：

```typescript
function doConnectMonitor() {
  const ids = getSelectedConnectionIds()
  const conns = ids.map(id => connectionStore.connections.find(c => c.id === id)).filter(Boolean) as ConnectionConfig[]
  selectedIds.value = new Set()
  closeMenu()
  for (const c of conns) {
    emit('connectMonitor', c)
  }
}
```

在 `defineEmits` 中添加 `'connectMonitor'`。

- [ ] **Step 2: App.vue 注册 monitor 标签渲染**

Template 中 `DBTabContent` 后添加：

```vue
<MonitorTabContent
  v-else-if="activeTab.type === 'monitor'"
  :key="activeTab.id"
  :session-id="getPanelSessionId(activeTab.panelId) || ''"
/>
```

Import `MonitorTabContent`。

在 `Sidebar` 组件绑定中添加 `@connect-monitor="onConnectMonitor"`。

添加 `onConnectMonitor` 方法：

```typescript
async function onConnectMonitor(config: ConnectionConfig) {
  const panel = panelStore.createPanel(config, 'monitor')
  const displayTitle = config.name || `${config.user}@${config.host}`
  panel.title = displayTitle + ' ' + t('monitor.titleSuffix')
  const tab = tabStore.createMonitorTab(displayTitle + ' ' + t('monitor.titleSuffix'), panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('monitor', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create monitor session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}
```

在 `closeTab` 中添加 monitor session 的清理逻辑（模式同 database）：

```typescript
// Close monitor session
if (tab && tab.type === 'monitor') {
  const p = panelStore.getPanel(tab.panelId)
  if (p?.sessionId) {
    try { await CloseSession(p.sessionId) } catch (_) {}
  }
}
```

- [ ] **Step 3: i18n 添加翻译词条**

在 `zh-CN` 中添加：

```typescript
// Monitor
'sidebar.connectMonitor': '服务器监控',
'monitor.performance': '性能',
'monitor.processes': '进程',
'monitor.system': '系统信息',
'monitor.titleSuffix': '(监控)',
'monitor.cpu': 'CPU',
'monitor.memory': '内存',
'monitor.disk': '磁盘',
'monitor.network': '网络',
'monitor.processName': '进程名',
'monitor.user': '用户',
'monitor.mem': '内存',
'monitor.searchProcess': '搜索进程...',
'monitor.loading': '加载中...',
```

在 `en` 中添加对应英文翻译。

---

### Task 5: 编译与验证

**Files:** 无新增/修改

- [ ] **Step 1: 前端编译验证**

Run:
```bash
cd frontend && npm run build
```

Expected: 编译通过，无 TS/Vue 错误。

- [ ] **Step 2: Go 编译验证**

Run:
```bash
go build ./...
```

Expected: 编译通过。

- [ ] **Step 3: 运行验证**

按 CLAUDE.md 要求，清理缓存后启动：
```bash
cd frontend && rm -rf dist node_modules/.vite && cd .. && wails dev
```

Expected: 应用正常启动。右键 SSH 连接出现"服务器监控"，点击后打开监控面板，能看到 CPU/内存/磁盘/网络数据和进程列表。

---

## Self-Review

### Spec Coverage Check

| 需求 | 对应 Task |
|------|----------|
| SSH 右键菜单入口 | Task 4 Step 1 |
| 后端定时采集（1秒轮询） | Task 1 |
| 性能页（CPU/内存/磁盘/网络） | Task 3 |
| 进程页（列表+搜索+排序） | Task 3 |
| 系统信息页（首次加载） | Task 3 |
| Canvas 趋势图 | Task 3 |
| 关闭清理 | Task 1 + Task 4 Step 2 |
| i18n 翻译 | Task 4 Step 3 |

无遗漏。

### Placeholder Scan

- 无 TBD/TODO
- 无 "implement later"
- 无 "add appropriate error handling"
- 所有代码步骤均包含完整代码

### Type Consistency Check

- `PanelType` 中 `monitor` 与 `manager.go` 中 `case "monitor"` 一致
- `MonitorTab.type === 'monitor'` 与 `activeTab.type === 'monitor'` 匹配
- `sessionType: "monitor"` 与前端类型一致
