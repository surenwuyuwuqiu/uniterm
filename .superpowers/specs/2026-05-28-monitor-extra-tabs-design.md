# Server Monitor Extra Tabs Design

## Overview
Add three new on-demand monitor tabs to the existing server monitor: **Ports**, **Disks**, and **Network**.
Unlike Performance/Processes tabs which auto-poll every second, these tabs load data once when activated and provide a manual refresh button.

## Architecture

### Backend (Go)
Three new exported methods on `MonitorSession`, called directly from the frontend via Wails bindings:

- `GetPorts(sessionId string) ([]PortInfo, error)`
- `GetDisks(sessionId string) ([]DiskInfo, error)`
- `GetNetworkCards(sessionId string) ([]NetCardInfo, error)`

Each method opens a new SSH session, runs a shell command, parses the output, and returns a typed struct slice.

### Frontend (Vue)
- Add three new tab buttons to `MonitorTabContent.vue`: Ports (端口), Disks (磁盘), Network (网卡)
- Each tab pane contains:
  - A refresh button (top-right of the pane)
  - An `el-table` for tabular data
  - Empty/loading states
- Data is fetched via Wails bound methods when the tab is first clicked or when the refresh button is clicked.
- No `EventsOn` streaming; no auto-poll.

---

## Tab 1: Ports (端口)

**Goal:** Show all LISTENing TCP and UDP ports with the process that owns them.

**Command:**
```bash
ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null
```
Fallback chain: `ss` first, then `netstat`.

**Columns:**
| Column | Source |
|--------|--------|
| Protocol | `tcp` / `udp` (derived from ss/netstat output) |
| Local Address | e.g. `0.0.0.0:22` |
| State | `LISTEN` (or other if present) |
| Process | e.g. `sshd` + PID from `users:(("sshd",pid=1234,...))` |

**Parsing:** `ss` output fields are space-separated; the `users:` field contains a JSON-like string that needs regex extraction. If `ss` is unavailable, `netstat` output is parsed similarly.

**Go struct:**
```go
type PortInfo struct {
    Protocol string `json:"protocol"`
    LocalAddr string `json:"localAddr"`
    State string `json:"state"`
    Process string `json:"process"`
}
```

---

## Tab 2: Disks (磁盘)

**Goal:** Show block devices from `lsblk` and, for mounted devices, show capacity and usage from `df`.

**Command:**
```bash
lsblk -b -o NAME,SIZE,TYPE,MOUNTPOINT,MODEL,ROTA 2>/dev/null
```
If `-b` or `-o` flags fail (very old util-linux), fall back to plain `lsblk` and parse the default columns.

Then for each mountpoint found:
```bash
df -h <mountpoint> 2>/dev/null
```

Alternatively, run `df -h` once and build a map of mountpoint -> usage.

**Columns:**
| Column | Source |
|--------|--------|
| Name | `NAME` from lsblk |
| Type | `TYPE` (disk, part, lvm, etc.) |
| Size | Human-readable size |
| Model | `MODEL` (if available) |
| Mount Point | `MOUNTPOINT` |
| Used / Total | From `df` if mounted |
| Usage % | From `df` |
| Media | `HDD` if `ROTA=1`, `SSD` if `ROTA=0` |

**Go struct:**
```go
type DiskInfo struct {
    Name string `json:"name"`
    Type string `json:"type"`
    Size string `json:"size"`
    Model string `json:"model"`
    MountPoint string `json:"mountPoint"`
    Used string `json:"used"`
    Total string `json:"total"`
    Usage int `json:"usage"`
    Media string `json:"media"` // "HDD" / "SSD" / "-"
}
```

**Tree display:** `lsblk` output is hierarchical (indented by name). The frontend table should use `el-table` row indentation or a simple `padding-left` based on depth to show the parent-child relationship (e.g. `sda` -> `sda1`).

---

## Tab 3: Network (网卡)

**Goal:** Show physical network interfaces, their link state, and bond membership.

**Command:**
```bash
ip -j link show 2>/dev/null
```
If `ip -j` fails, fall back to `ip link show` and parse text output.

For bond information:
```bash
cat /proc/net/bonding/* 2>/dev/null
```
If bond directory does not exist, no bond info.

**Columns:**
| Column | Source |
|--------|--------|
| Name | Interface name (e.g. `eth0`, `bond0`) |
| State | `UP` / `DOWN` from `operstate` |
| MAC | `address` |
| Speed | `cat /sys/class/net/<iface>/speed` (Mbps, -1 if unknown) |
| Type | `Physical` if type is `ether`; `Bond` if name exists in bonding; `Virtual` otherwise |
| Bond | Bond master name (if interface is a bond slave) |
| Bond Slaves | List of slave interfaces (if this interface is a bond master) |
| IP Addresses | `ip -j addr show <iface>` |

**Go struct:**
```go
type NetCardInfo struct {
    Name string `json:"name"`
    State string `json:"state"`
    MAC string `json:"mac"`
    Speed string `json:"speed"`
    Type string `json:"type"` // "Physical" / "Bond" / "Virtual" / "Loopback"
    BondMaster string `json:"bondMaster"` // empty if not a slave
    BondSlaves []string `json:"bondSlaves"` // empty if not a master
    IPAddrs []string `json:"ipAddrs"`
}
```

**IP collection:** Run `ip -j addr show` once and map addresses to interfaces.

---

## UI Design

All three tabs share the same layout pattern:

```
+------------------+------------------+
|  Tab: Ports      |  [Refresh Icon]  |
+------------------+------------------+
|  el-table ...                       |
+-------------------------------------+
```

- **Refresh button:** `el-button` with `:icon="RefreshRight"` (or similar), size="small", placed top-right of the pane.
- **Loading:** `v-loading` on the table while fetching.
- **Empty:** `el-empty` if no data returned.
- **Error:** `ElMessage.error()` on fetch failure.

### i18n Keys (to add)
```
monitor.ports = Ports / 端口
monitor.disks = Disks / 磁盘
monitor.network = Network / 网卡
monitor.refresh = Refresh / 刷新
monitor.noData = No data / 暂无数据
monitor.port.protocol = Protocol / 协议
monitor.port.localAddr = Local Address / 本地地址
monitor.port.state = State / 状态
monitor.port.process = Process / 进程
monitor.disk.name = Name / 名称
monitor.disk.type = Type / 类型
monitor.disk.size = Size / 容量
monitor.disk.model = Model / 型号
monitor.disk.mountPoint = Mount Point / 挂载点
monitor.disk.used = Used / 已用
monitor.disk.total = Total / 总量
monitor.disk.usage = Usage / 使用率
monitor.disk.media = Media / 介质
monitor.net.name = Name / 名称
monitor.net.state = State / 状态
monitor.net.mac = MAC / MAC地址
monitor.net.speed = Speed / 速率
monitor.net.type = Type / 类型
monitor.net.bond = Bond / Bond
monitor.net.bondSlaves = Slaves / 成员
monitor.net.ipAddrs = IP Addresses / IP地址
```

---

## Error Handling

1. **Command not found:** If `ss` and `netstat` are both missing for Ports, return empty slice and log warning.
2. **Permission denied:** Some commands (e.g. `ss -p`) may require root to see all PIDs. Return whatever data is available; do not fail the whole request.
3. **JSON parse failure:** If `ip -j` produces invalid JSON, fall back to text parsing.
4. **SSH session failure:** Return error to frontend, which shows `ElMessage.error`.

---

## Files to Modify

| File | Action |
|------|--------|
| `backend/session/monitor_session.go` | Add `GetPorts`, `GetDisks`, `GetNetworkCards` methods and structs |
| `backend/session/manager.go` | Add exported wrapper functions for Wails binding |
| `frontend/src/components/MonitorTabContent.vue` | Add three tabs, tables, refresh buttons, data fetching logic |
| `frontend/src/i18n/index.ts` | Add all new i18n keys |
| `frontend/wailsjs/go/main/App.d.ts` | Auto-generated after `wails dev` |
| `frontend/wailsjs/go/models.ts` | Auto-generated after `wails dev` |

---

## Performance Considerations

- Each method opens a new SSH session. On high-latency links this may take 200-500ms. Use `v-loading` to indicate work in progress.
- Disk tab runs two commands (`lsblk` + `df`). Consider running `df -h` once and mapping by mountpoint rather than per-device.
- Network tab runs `ip -j link` + `ip -j addr` + bond file reads. Bond reads are cheap (local files).
