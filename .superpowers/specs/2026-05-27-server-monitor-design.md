# 服务器监控功能设计文档

## 背景

在 SSH 连接的右键菜单中新增"服务器监控"功能，点击后打开类似 Windows 任务管理器的实时监控面板，展示远程 Linux 服务器的 CPU、内存、磁盘、网络、进程等资源使用情况。

## 目标

- 右键 SSH 连接即可打开服务器实时监控
- UI 参考 Windows 任务管理器，分 Tab 展示
- 仅支持 Linux 服务器（通过 SSH 执行采集脚本）
- 纯 Canvas 绘制趋势图，不引入图表库

## 架构概述

新增 `monitor` 会话类型，整体架构沿用现有 Wails 模式：

```
Sidebar 右键菜单
    ↓ 点击"服务器监控"
App.vue onConnectMonitor()
    ↓
Go: CreateSession("monitor", config)
    ↓ 内部新建 SSH 连接
Go: MonitorSession 定时执行采集脚本
    ↓ Wails EventsEmit
Vue: MonitorTabContent 接收数据并渲染
    ↓ 关闭标签
Go: CloseSession 断开 SSH 连接
```

与现有终端 SSH 的区别：MonitorSession 不创建 PTY、不进入 Shell，而是通过 `client.NewSession()` 周期性执行命令获取 JSON 输出。

## 后端设计

### MonitorSession 结构

```go
type MonitorSession struct {
    baseSession
    client   *ssh.Client
    config   ConnectionConfig
    ticker   *time.Ticker
    quit     chan struct{}
}
```

### 数据采集命令

通过 `ssh.Client.NewSession()` 执行一个内嵌的 shell 脚本，脚本输出单行 JSON：

```json
{
  "cpu": {"usage": 23.5, "cores": 8, "processes": 120, "threads": 450, "handles": 3000},
  "memory": {"total": 16, "used": 8.5, "free": 7.5, "usage": 53.1},
  "disk": {"total": 500, "used": 200, "free": 300, "usage": 40.0, "readRate": 1024, "writeRate": 512},
  "network": {"rxRate": 1048576, "txRate": 524288},
  "processes": [...],
  "system": {"os": "Ubuntu 22.04", "hostname": "srv1", "uptime": "5d 3h"}
}
```

### 执行策略

- 首次连接成功（`StatusConnected`）后：立即执行一次 `system` 采集命令，通过 `session:data` 事件推送系统信息，前端收到 `system` 字段后缓存不再重复请求
- 定时轮询：`ticker = time.NewTicker(1 * time.Second)`，每次新建 SSH session 执行监控脚本
- 数据推送：通过 `runtime.EventsEmit` 发送 `session:data` 事件，payload 带 `id` 和 JSON 字符串
- 关闭时：停止 ticker，断开 SSH client

## 前端组件设计

### MonitorTabContent.vue 整体结构

```vue
<template>
  <div class="monitor-tab">
    <!-- 顶部 Tab 导航 -->
    <div class="monitor-tabs">
      <div class="tab-item" :class="{ active: activeTab === 'performance' }" @click="activeTab = 'performance'">性能</div>
      <div class="tab-item" :class="{ active: activeTab === 'processes' }" @click="activeTab = 'processes'">进程</div>
      <div class="tab-item" :class="{ active: activeTab === 'system' }" @click="activeTab = 'system'">系统信息</div>
    </div>

    <!-- 性能页 -->
    <div v-show="activeTab === 'performance'" class="tab-pane performance-pane">
      <!-- 左侧导航：CPU / 内存 / 磁盘 / 网络 -->
      <!-- 右侧详情：当前选中项的 Canvas 趋势图 + 关键指标文本 -->
    </div>

    <!-- 进程页 -->
    <div v-show="activeTab === 'processes'" class="tab-pane">
      <!-- 搜索框 + 排序表头 + 进程列表 -->
    </div>

    <!-- 系统信息页 -->
    <div v-show="activeTab === 'system'" class="tab-pane">
      <!-- 静态文本展示，首次加载后缓存 -->
    </div>
  </div>
</template>
```

### 性能页布局（左右分区）

- **左侧窄栏（~180px）**：CPU、内存、磁盘、网络四个可点击项，每项显示当前使用率数字和小条形色块
- **右侧主区**：选中某项后展示
  - 大字当前使用率（如 `23%`）
  - Canvas 折线图（60 秒历史，每秒一个点）
  - 下方网格排列关键指标文本（如 CPU 的"进程数: 120"、"线程数: 450"等）

### 进程页

- Element Plus `el-table` 展示进程列表
- 列：PID、名称、用户、CPU%、内存%、状态
- 支持点击表头排序、顶部搜索框过滤

### 数据接收

- 组件 `onMounted` 通过 `EventsOn('session:data')` 过滤 `id === sessionId` 的消息
- 解析 JSON 后更新 `ref` 数据，Vue 响应式驱动 UI 更新

## 数据流

```
Go MonitorSession.poll()
  → ssh.Session.Run(monitorScript)
  ← JSON stdout
  → runtime.EventsEmit(ctx, "session:data", {id, data: json})

Vue MonitorTabContent
  → EventsOn("session:data", handler)
  → 过滤 id === props.sessionId
  → JSON.parse(data)
  → 更新 cpuHistory / memHistory / diskHistory / netHistory / processList / systemInfo
  → Canvas 重绘 / el-table 刷新
```

## 错误处理

| 场景 | 处理方式 |
|------|---------|
| SSH 连接失败 | `session:status` 推送 `error`，前端显示红色错误提示 |
| 采集脚本执行失败（如服务器不是 Linux） | 跳过本次轮询，记录日志，前端显示"采集失败" |
| JSON 解析失败 | 丢弃该帧数据，不影响后续轮询 |
| 网络中断 | SSH client 自动断开，`session:status` 变为 `disconnected` |
| 标签页关闭 | App.vue 调用 `CloseSession`，Go 端停止 ticker 并断开 SSH |

## 资源清理

- 标签页关闭时，`App.vue` 的 `closeTab` 调用 `CloseSession(sessionId)`
- Go 端：`MonitorSession.Disconnect()` 停止 ticker -> 关闭 SSH client
- 前端组件 `onUnmounted` 注销 `EventsOn` 监听

## 变更清单

| 模块 | 变更内容 |
|------|---------|
| Go 后端 | 新增 `backend/session/monitor_session.go`，`manager.go` 注册 `monitor` 类型 |
| 前端类型 | `workspace.ts` / `panelStore.ts` / `tabStore.ts` 新增 `monitor` 类型 |
| 前端组件 | 新增 `MonitorTabContent.vue`（性能/进程/系统信息三页） |
| 前端入口 | `Sidebar.vue` 右键菜单添加"服务器监控" |
| 前端渲染 | `App.vue` 注册 `monitor` 标签页渲染 |
| i18n | 添加监控相关翻译词条 |

---

# 系统信息扩充与排版优化（补充设计）

## 设计目标

1. 系统信息页扩充更多静态属性
2. 性能监控页补充系统负载和内存缓存指标
3. 系统信息页重新排版为分组卡片式布局

## 信息分类原则

- **系统信息页**：只展示静态/半静态属性，连接时采集一次
- **性能监控页**：展示实时变化的动态指标，每秒轮询

## 系统信息页扩充内容

### 信息分组

| 系统信息 | 硬件信息 | 网络信息 |
|---------|---------|---------|
| 操作系统 | CPU 型号 | 内网 IP |
| 发行版版本号 | 核心数 | |
| 内核版本 | 系统架构 | |
| 主机名 | CPU 频率 | |
| 时区 | 内存总量 | |
| | 磁盘总量 | |

### 采集命令

```bash
# 系统信息组
cat /etc/os-release | grep -E '^(PRETTY_NAME|VERSION_ID)='
uname -r
hostname
date +%Z

# 硬件信息组
cat /proc/cpuinfo | grep 'model name' | head -1
nproc
uname -m
cat /proc/cpuinfo | grep 'cpu MHz' | head -1
awk '/MemTotal/{print $2}' /proc/meminfo
df -h / | awk 'NR==2{print $2}'

# 网络信息组
ip route get 1.1.1.1 | grep -oP 'src \K\S+'
```

### 前端排版（简洁列表式）

- 每组一个标题（如"系统信息"、"硬件信息"、"网络信息"），标题用较大字号、加粗
- 标题下方是 key-value 列表，每行一个：左侧灰色小字 label，右侧白色等宽字体 value
- 组与组之间用适当间距分隔
- 单栏布局，整体简洁无卡片背景

## 性能监控页补充内容

### CPU 详情区补充

- **系统负载**：1min / 5min / 15min
- 采集：`cat /proc/loadavg | awk '{print $1,$2,$3}'`

### 内存详情区补充

- **内存详情**：已用 / 可用 / 缓存 (cache) / buffers
- 采集：`/proc/meminfo` 中的 `Cached` 和 `Buffers` 字段

## 趋势图纵轴范围

| 指标 | yMin | yMax |
|------|------|------|
| CPU 使用率 | 0 | 100（固定） |
| 内存使用率 | 0 | 100（固定） |
| 磁盘使用率 | 0 | 100（固定） |
| 网络速率 | 0 | 动态计算 |
