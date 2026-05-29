# Server Monitor Extra Tabs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three on-demand monitor tabs (Ports, Disks, Network) to the existing server monitor feature.

**Architecture:** Each tab triggers a one-shot SSH command via a new Wails-bound Go method when the tab is activated or the refresh button is clicked. Data is returned as typed structs and rendered in `el-table`. No auto-polling.

**Tech Stack:** Go 1.21+, Wails v2, Vue 3, Element Plus, TypeScript

---

### Task 1: Add Go structs for new monitor data types

**Files:**
- Modify: `backend/session/monitor_session.go`

- [ ] **Step 1: Add PortInfo, DiskInfo, NetCardInfo structs**

Insert the following structs after the existing `monitorState` struct (around line 22):

```go
type PortInfo struct {
	Protocol  string `json:"protocol"`
	LocalAddr string `json:"localAddr"`
	State     string `json:"state"`
	Process   string `json:"process"`
}

type DiskInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	Model      string `json:"model"`
	MountPoint string `json:"mountPoint"`
	Used       string `json:"used"`
	Total      string `json:"total"`
	Usage      int    `json:"usage"`
	Media      string `json:"media"`
}

type NetCardInfo struct {
	Name       string   `json:"name"`
	State      string   `json:"state"`
	MAC        string   `json:"mac"`
	Speed      string   `json:"speed"`
	Type       string   `json:"type"`
	BondMaster string   `json:"bondMaster"`
	BondSlaves []string `json:"bondSlaves"`
	IPAddrs    []string `json:"ipAddrs"`
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/session/monitor_session.go
git commit -m "feat(monitor): add PortInfo, DiskInfo, NetCardInfo structs"
```

---

### Task 2: Implement GetPorts() backend method

**Files:**
- Modify: `backend/session/monitor_session.go`

- [ ] **Step 1: Add GetPorts() method**

Insert after the existing `KillProcess()` method (around line 684):

```go
func (s *MonitorSession) GetPorts() ([]PortInfo, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	script := `exec 2>/dev/null
# Try ss first, fall back to netstat
if command -v ss >/dev/null 2>&1; then
    ss -tlnp | tail -n +2
else
    netstat -tlnp 2>/dev/null | tail -n +2
fi`
	out, err := session.Output(script)
	if err != nil {
		return nil, err
	}

	var ports []PortInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// ss output: State  Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
		// netstat output: Proto  Recv-Q  Send-Q  Local Address  Foreign Address  State  PID/Program name
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		var protocol, localAddr, state, process string

		// Detect format by checking first field
		if fields[0] == "tcp" || fields[0] == "udp" || fields[0] == "tcp6" || fields[0] == "udp6" {
			// netstat format
			protocol = fields[0]
			if len(fields) >= 6 {
				localAddr = fields[3]
				state = fields[5]
				if len(fields) >= 7 {
					process = fields[6]
				}
			}
		} else {
			// ss format: first field is State
			state = fields[0]
			if len(fields) >= 4 {
				localAddr = fields[3]
			}
			if len(fields) >= 5 {
				processField := fields[len(fields)-1]
				// Extract process name from users:(("name",pid=123,...))
				if idx := strings.Index(processField, `"`); idx >= 0 {
					endIdx := strings.Index(processField[idx+1:], `"`)
					if endIdx >= 0 {
						process = processField[idx+1 : idx+1+endIdx]
					}
				} else {
					process = processField
				}
			}
			// Infer protocol from address (IPv6 contains ::)
			if strings.Contains(localAddr, ":") && !strings.Contains(localAddr, ".") && !strings.HasPrefix(localAddr, "[::]") {
				protocol = "tcp6"
			} else if strings.HasPrefix(localAddr, "[::]") {
				protocol = "tcp6"
			} else {
				protocol = "tcp"
			}
		}

		if localAddr == "" {
			continue
		}

		ports = append(ports, PortInfo{
			Protocol:  protocol,
			LocalAddr: localAddr,
			State:     state,
			Process:   process,
		})
	}

	return ports, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/session/monitor_session.go
git commit -m "feat(monitor): implement GetPorts backend method"
```

---

### Task 3: Implement GetDisks() backend method

**Files:**
- Modify: `backend/session/monitor_session.go`

- [ ] **Step 1: Add GetDisks() method**

Insert after `GetPorts()`:

```go
func (s *MonitorSession) GetDisks() ([]DiskInfo, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	script := `exec 2>/dev/null
# lsblk with bytes for exact sizes
lsblk -b -o NAME,SIZE,TYPE,MOUNTPOINT,MODEL,ROTA 2>/dev/null || lsblk -o NAME,SIZE,TYPE,MOUNTPOINT,MODEL,ROTA 2>/dev/null
echo "---DF---"
df -h 2>/dev/null`

	out, err := session.Output(script)
	if err != nil {
		return nil, err
	}

	output := string(out)
	parts := strings.Split(output, "---DF---\n")
	lsblkPart := strings.TrimSpace(parts[0])
	var dfPart string
	if len(parts) > 1 {
		dfPart = strings.TrimSpace(parts[1])
	}

	// Build mountpoint -> usage map from df
	mountUsage := map[string]struct{ Used, Total string; Usage int }{}
	for _, line := range strings.Split(dfPart, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Filesystem") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mount := fields[5]
			usageStr := strings.TrimSuffix(fields[4], "%")
			usage, _ := strconv.Atoi(usageStr)
			mountUsage[mount] = struct{ Used, Total string; Usage int }{
				Used:  fields[2],
				Total: fields[1],
				Usage: usage,
			}
		}
	}

	var disks []DiskInfo
	// Parse lsblk output
	lines := strings.Split(lsblkPart, "\n")
	var headers []string
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if i == 0 {
			// Header line
			headers = fields
			continue
		}

		// Find NAME position (first column, may be indented)
		nameIdx := 0
		// Calculate indentation depth based on leading spaces/hyphens in the raw line
		rawName := fields[0]
		depth := 0
		trimmedName := rawName
		for strings.HasPrefix(trimmedName, "|") || strings.HasPrefix(trimmedName, "-") || strings.HasPrefix(trimmedName, "\\") || strings.HasPrefix(trimmedName, " ") {
			if strings.HasPrefix(trimmedName, " ") {
				trimmedName = strings.TrimPrefix(trimmedName, " ")
			} else {
				trimmedName = trimmedName[1:]
				if strings.HasPrefix(trimmedName, "-") {
					trimmedName = trimmedName[1:]
				}
			}
			depth++
		}
		// Reconstruct fields with trimmed name
		if len(fields) > 0 {
			fields[0] = trimmedName
		}

		// Map columns by header index
		colMap := map[string]string{}
		for j, h := range headers {
			if j < len(fields) {
				colMap[h] = fields[j]
			} else {
				colMap[h] = ""
			}
		}

		sizeBytes, _ := strconv.ParseUint(colMap["SIZE"], 10, 64)
		sizeStr := formatBytes(sizeBytes)

		media := "-"
		if colMap["ROTA"] == "1" {
			media = "HDD"
		} else if colMap["ROTA"] == "0" {
			media = "SSD"
		}

		mount := colMap["MOUNTPOINT"]
		usage := mountUsage[mount]

		disk := DiskInfo{
			Name:       strings.Repeat("  ", depth) + colMap["NAME"],
			Type:       colMap["TYPE"],
			Size:       sizeStr,
			Model:      colMap["MODEL"],
			MountPoint: mount,
			Media:      media,
		}
		if mount != "" {
			disk.Used = usage.Used
			disk.Total = usage.Total
			disk.Usage = usage.Usage
		}

		disks = append(disks, disk)
	}

	return disks, nil
}
```

- [ ] **Step 2: Add formatBytes helper**

Insert after the existing `roundFloat()` function (around line 689):

```go
func formatBytes(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	f := float64(bytes)
	for f >= k && i < len(sizes)-1 {
		f /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", f, sizes[i])
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/session/monitor_session.go
git commit -m "feat(monitor): implement GetDisks backend method"
```

---

### Task 4: Implement GetNetworkCards() backend method

**Files:**
- Modify: `backend/session/monitor_session.go`

- [ ] **Step 1: Add GetNetworkCards() method**

Insert after `GetDisks()`:

```go
func (s *MonitorSession) GetNetworkCards() ([]NetCardInfo, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	script := `exec 2>/dev/null
# Link info (JSON if available)
if ip -j link show >/dev/null 2>&1; then
    echo "---LINKJSON---"
    ip -j link show
else
    echo "---LINKTEXT---"
    ip link show
fi
echo "---ADDRJSON---"
ip -j addr show 2>/dev/null || echo "[]"
echo "---BOND---"
for f in /proc/net/bonding/*; do
    [ -f "$f" ] || continue
    echo "---BONDNAME---$(basename "$f")"
    cat "$f"
done`

	out, err := session.Output(script)
	if err != nil {
		return nil, err
	}

	output := string(out)
	sections := strings.Split(output, "---LINKJSON---\n")
	if len(sections) == 1 {
		sections = strings.Split(output, "---LINKTEXT---\n")
	}
	if len(sections) < 2 {
		return []NetCardInfo{}, nil
	}

	linkPart := strings.TrimSpace(sections[1])
	rest := ""
	if idx := strings.Index(linkPart, "---ADDRJSON---"); idx >= 0 {
		rest = linkPart[idx:]
		linkPart = strings.TrimSpace(linkPart[:idx])
	}

	// Parse bond info
	bondMasters := map[string][]string{} // bond name -> slave names
	bondSlaves := map[string]string{}    // slave name -> master name
	if rest != "" {
		bondSections := strings.Split(rest, "---BOND---\n")
		if len(bondSections) > 1 {
			for _, bs := range strings.Split(bondSections[1], "---BONDNAME---") {
				bs = strings.TrimSpace(bs)
				if bs == "" {
					continue
				}
				lines := strings.SplitN(bs, "\n", 2)
				bondName := strings.TrimSpace(lines[0])
				var slaves []string
				if len(lines) > 1 {
					for _, line := range strings.Split(lines[1], "\n") {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "Slave Interface:") {
							slave := strings.TrimSpace(strings.TrimPrefix(line, "Slave Interface:"))
							slaves = append(slaves, slave)
							bondSlaves[slave] = bondName
						}
					}
				}
				bondMasters[bondName] = slaves
			}
		}
	}

	// Parse addresses
	var addrData []map[string]interface{}
	if rest != "" {
		addrSections := strings.Split(rest, "---ADDRJSON---\n")
		if len(addrSections) > 1 {
			addrPart := strings.TrimSpace(addrSections[1])
			if idx := strings.Index(addrPart, "---BOND---"); idx >= 0 {
				addrPart = strings.TrimSpace(addrPart[:idx])
			}
			_ = json.Unmarshal([]byte(addrPart), &addrData)
		}
	}
	addrMap := map[string][]string{}
	for _, iface := range addrData {
		ifName, _ := iface["ifname"].(string)
		if ifName == "" {
			continue
		}
		addrs, _ := iface["addr_info"].([]interface{})
		for _, a := range addrs {
			ai, _ := a.(map[string]interface{})
			if ai == nil {
				continue
			}
			ip, _ := ai["local"].(string)
			if ip != "" {
				addrMap[ifName] = append(addrMap[ifName], ip)
			}
		}
	}

	// Parse link info
	var cards []NetCardInfo
	if strings.HasPrefix(linkPart, "[") {
		// JSON format
		var linkData []map[string]interface{}
		if err := json.Unmarshal([]byte(linkPart), &linkData); err == nil {
			for _, iface := range linkData {
				name, _ := iface["ifname"].(string)
				if name == "" {
					continue
				}
				state, _ := iface["operstate"].(string)
				mac, _ := iface["address"].(string)
				linkType, _ := iface["link_type"].(string)

				// Get speed
				speed := "-"
				if speedFile, err := session.Output(fmt.Sprintf("cat /sys/class/net/%s/speed 2>/dev/null || echo -1", name)); err == nil {
					speedVal, _ := strconv.Atoi(strings.TrimSpace(string(speedFile)))
					if speedVal > 0 {
						speed = fmt.Sprintf("%d Mbps", speedVal)
					}
				}

				// Determine type
				ifType := "Virtual"
				if linkType == "ether" {
					if _, isBond := bondMasters[name]; isBond {
						ifType = "Bond"
					} else {
						ifType = "Physical"
					}
				} else if linkType == "loopback" {
					ifType = "Loopback"
				}

				card := NetCardInfo{
					Name:       name,
					State:      state,
					MAC:        mac,
					Speed:      speed,
					Type:       ifType,
					BondMaster: bondSlaves[name],
					BondSlaves: bondMasters[name],
					IPAddrs:    addrMap[name],
				}
				cards = append(cards, card)
			}
		}
	} else {
		// Text format fallback
		var currentCard *NetCardInfo
		for _, line := range strings.Split(linkPart, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, " ") {
				// New interface line: "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> ..."
				if currentCard != nil {
					cards = append(cards, *currentCard)
				}
				parts := strings.SplitN(line, ":", 3)
				if len(parts) >= 2 {
					name := strings.TrimSpace(parts[1])
					currentCard = &NetCardInfo{
						Name:    name,
						State:   "UNKNOWN",
						Speed:   "-",
						Type:    "Virtual",
						IPAddrs: addrMap[name],
					}
					// Check flags for UP state
					if len(parts) >= 3 && strings.Contains(parts[2], "UP") {
						currentCard.State = "UP"
					}
					if _, isBond := bondMasters[name]; isBond {
						currentCard.Type = "Bond"
					} else if strings.Contains(line, "loopback") {
						currentCard.Type = "Loopback"
					} else if strings.Contains(line, "ether") {
						currentCard.Type = "Physical"
					}
					currentCard.BondMaster = bondSlaves[name]
					currentCard.BondSlaves = bondMasters[name]
				}
			} else if currentCard != nil {
				if strings.Contains(line, "link/ether") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						currentCard.MAC = fields[1]
					}
				}
				if strings.Contains(line, "state ") {
					stateIdx := strings.Index(line, "state ")
					if stateIdx >= 0 {
						stateVal := strings.Fields(line[stateIdx+6:])
						if len(stateVal) > 0 {
							currentCard.State = strings.ToUpper(stateVal[0])
						}
					}
				}
			}
		}
		if currentCard != nil {
			cards = append(cards, *currentCard)
		}
	}

	return cards, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/session/monitor_session.go
git commit -m "feat(monitor): implement GetNetworkCards backend method"
```

---

### Task 5: Add Wails binding functions

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add exported App methods**

Insert after the existing `KillProcess()` method (around line 648):

```go
func (a *App) GetPorts(sessionID string) ([]session.PortInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetPorts()
}

func (a *App) GetDisks(sessionID string) ([]session.DiskInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetDisks()
}

func (a *App) GetNetworkCards(sessionID string) ([]session.NetCardInfo, error) {
	ms, err := a.getMonitorSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ms.GetNetworkCards()
}
```

- [ ] **Step 2: Commit**

```bash
git add app.go
git commit -m "feat(monitor): add Wails bindings for GetPorts, GetDisks, GetNetworkCards"
```

---

### Task 6: Add i18n translations

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Add Chinese translations**

Insert after `'monitor.noProcessSelected': '未选择进程',` (around line 446):

```typescript
    'monitor.ports': '端口',
    'monitor.disks': '磁盘',
    'monitor.networkCards': '网卡',
    'monitor.refresh': '刷新',
    'monitor.noData': '暂无数据',
    'monitor.port.protocol': '协议',
    'monitor.port.localAddr': '本地地址',
    'monitor.port.state': '状态',
    'monitor.port.process': '进程',
    'monitor.disk.name': '名称',
    'monitor.disk.type': '类型',
    'monitor.disk.size': '容量',
    'monitor.disk.model': '型号',
    'monitor.disk.mountPoint': '挂载点',
    'monitor.disk.used': '已用',
    'monitor.disk.total': '总量',
    'monitor.disk.usage': '使用率',
    'monitor.disk.media': '介质',
    'monitor.net.name': '名称',
    'monitor.net.state': '状态',
    'monitor.net.mac': 'MAC地址',
    'monitor.net.speed': '速率',
    'monitor.net.type': '类型',
    'monitor.net.bond': 'Bond',
    'monitor.net.bondSlaves': '成员',
    'monitor.net.ipAddrs': 'IP地址',
```

- [ ] **Step 2: Add English translations**

Insert after `'monitor.noProcessSelected': 'No process selected',` (around line 961):

```typescript
    'monitor.ports': 'Ports',
    'monitor.disks': 'Disks',
    'monitor.networkCards': 'Network',
    'monitor.refresh': 'Refresh',
    'monitor.noData': 'No data',
    'monitor.port.protocol': 'Protocol',
    'monitor.port.localAddr': 'Local Address',
    'monitor.port.state': 'State',
    'monitor.port.process': 'Process',
    'monitor.disk.name': 'Name',
    'monitor.disk.type': 'Type',
    'monitor.disk.size': 'Size',
    'monitor.disk.model': 'Model',
    'monitor.disk.mountPoint': 'Mount Point',
    'monitor.disk.used': 'Used',
    'monitor.disk.total': 'Total',
    'monitor.disk.usage': 'Usage',
    'monitor.disk.media': 'Media',
    'monitor.net.name': 'Name',
    'monitor.net.state': 'State',
    'monitor.net.mac': 'MAC',
    'monitor.net.speed': 'Speed',
    'monitor.net.type': 'Type',
    'monitor.net.bond': 'Bond',
    'monitor.net.bondSlaves': 'Slaves',
    'monitor.net.ipAddrs': 'IP Addresses',
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/i18n/index.ts
git commit -m "feat(monitor): add i18n translations for ports, disks, network tabs"
```

---

### Task 7: Add frontend imports and data refs

**Files:**
- Modify: `frontend/src/components/MonitorTabContent.vue`

- [ ] **Step 1: Add imports**

Change the import line from:
```typescript
import { SetMonitorActiveTab, SetMonitorPaused, GetProcessDetail, KillProcess } from '../../wailsjs/go/main/App'
```
to:
```typescript
import { SetMonitorActiveTab, SetMonitorPaused, GetProcessDetail, KillProcess, GetPorts, GetDisks, GetNetworkCards } from '../../wailsjs/go/main/App'
```

- [ ] **Step 2: Add data refs**

Insert after the existing `processSummaryMem` ref (around line 279):

```typescript
// On-demand tab data
const portList = ref<any[]>([])
const diskList = ref<any[]>([])
const netCardList = ref<any[]>([])
const loadingPorts = ref(false)
const loadingDisks = ref(false)
const loadingNetCards = ref(false)
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MonitorTabContent.vue
git commit -m "feat(monitor): add frontend imports and data refs for new tabs"
```

---

### Task 8: Add tab buttons and pane templates

**Files:**
- Modify: `frontend/src/components/MonitorTabContent.vue`

- [ ] **Step 1: Add tab buttons**

Change the tab header div from:
```html
<div class="monitor-tabs-header">
  <div class="tab-item" :class="{ active: activeTab === 'performance' }" @click="activeTab = 'performance'">{{ t('monitor.performance') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'processes' }" @click="activeTab = 'processes'">{{ t('monitor.processes') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'system' }" @click="activeTab = 'system'">{{ t('monitor.system') }}</div>
</div>
```
to:
```html
<div class="monitor-tabs-header">
  <div class="tab-item" :class="{ active: activeTab === 'performance' }" @click="activeTab = 'performance'">{{ t('monitor.performance') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'processes' }" @click="activeTab = 'processes'">{{ t('monitor.processes') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'ports' }" @click="activeTab = 'ports'">{{ t('monitor.ports') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'disks' }" @click="activeTab = 'disks'">{{ t('monitor.disks') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'network' }" @click="activeTab = 'network'">{{ t('monitor.networkCards') }}</div>
  <div class="tab-item" :class="{ active: activeTab === 'system' }" @click="activeTab = 'system'">{{ t('monitor.system') }}</div>
</div>
```

- [ ] **Step 2: Add Ports pane**

Insert after the Processes pane closing `</div>` (around line 86):

```html
    <!-- Ports -->
    <div v-show="activeTab === 'ports'" class="tab-pane ports-pane">
      <div class="od-toolbar">
        <el-button size="small" :icon="RefreshRight" :loading="loadingPorts" @click="fetchPorts">
          {{ t('monitor.refresh') }}
        </el-button>
      </div>
      <el-table :data="portList" v-loading="loadingPorts" height="calc(100% - 36px)" size="small" class="od-table">
        <el-table-column prop="protocol" :label="t('monitor.port.protocol')" sortable width="90" />
        <el-table-column prop="localAddr" :label="t('monitor.port.localAddr')" sortable />
        <el-table-column prop="state" :label="t('monitor.port.state')" sortable width="100" />
        <el-table-column prop="process" :label="t('monitor.port.process')" sortable />
      </el-table>
    </div>

    <!-- Disks -->
    <div v-show="activeTab === 'disks'" class="tab-pane disks-pane">
      <div class="od-toolbar">
        <el-button size="small" :icon="RefreshRight" :loading="loadingDisks" @click="fetchDisks">
          {{ t('monitor.refresh') }}
        </el-button>
      </div>
      <el-table :data="diskList" v-loading="loadingDisks" height="calc(100% - 36px)" size="small" class="od-table">
        <el-table-column prop="name" :label="t('monitor.disk.name')" sortable>
          <template #default="{ row }">
            <span :style="{ paddingLeft: (row.name.match(/^ +/)?.[0].length || 0) * 6 + 'px' }">{{ row.name.trim() }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="type" :label="t('monitor.disk.type')" sortable width="90" />
        <el-table-column prop="size" :label="t('monitor.disk.size')" sortable width="100" />
        <el-table-column prop="model" :label="t('monitor.disk.model')" sortable />
        <el-table-column prop="mountPoint" :label="t('monitor.disk.mountPoint')" sortable />
        <el-table-column prop="used" :label="t('monitor.disk.used')" sortable width="90" />
        <el-table-column prop="total" :label="t('monitor.disk.total')" sortable width="90" />
        <el-table-column prop="usage" :label="t('monitor.disk.usage')" sortable width="100">
          <template #default="{ row }">{{ row.usage ? row.usage + '%' : '-' }}</template>
        </el-table-column>
        <el-table-column prop="media" :label="t('monitor.disk.media')" sortable width="80" />
      </el-table>
    </div>

    <!-- Network -->
    <div v-show="activeTab === 'network'" class="tab-pane network-pane">
      <div class="od-toolbar">
        <el-button size="small" :icon="RefreshRight" :loading="loadingNetCards" @click="fetchNetCards">
          {{ t('monitor.refresh') }}
        </el-button>
      </div>
      <el-table :data="netCardList" v-loading="loadingNetCards" height="calc(100% - 36px)" size="small" class="od-table">
        <el-table-column prop="name" :label="t('monitor.net.name')" sortable width="120" />
        <el-table-column prop="state" :label="t('monitor.net.state')" sortable width="90" />
        <el-table-column prop="mac" :label="t('monitor.net.mac')" sortable width="160" />
        <el-table-column prop="speed" :label="t('monitor.net.speed')" sortable width="120" />
        <el-table-column prop="type" :label="t('monitor.net.type')" sortable width="100" />
        <el-table-column prop="bondMaster" :label="t('monitor.net.bond')" sortable width="120" />
        <el-table-column prop="bondSlaves" :label="t('monitor.net.bondSlaves')" sortable width="180">
          <template #default="{ row }">{{ row.bondSlaves?.join(', ') || '-' }}</template>
        </el-table-column>
        <el-table-column prop="ipAddrs" :label="t('monitor.net.ipAddrs')" sortable>
          <template #default="{ row }">{{ row.ipAddrs?.join(', ') || '-' }}</template>
        </el-table-column>
      </el-table>
    </div>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MonitorTabContent.vue
git commit -m "feat(monitor): add ports, disks, network tab buttons and table panes"
```

---

### Task 9: Add fetch methods and tab activation logic

**Files:**
- Modify: `frontend/src/components/MonitorTabContent.vue`

- [ ] **Step 1: Add fetch methods**

Insert after the existing `confirmKill()` function (around line 491):

```typescript
async function fetchPorts() {
  loadingPorts.value = true
  try {
    portList.value = await GetPorts(props.sessionId)
  } catch (e: any) {
    ElMessage.error(e?.message || 'Failed to fetch ports')
    portList.value = []
  } finally {
    loadingPorts.value = false
  }
}

async function fetchDisks() {
  loadingDisks.value = true
  try {
    diskList.value = await GetDisks(props.sessionId)
  } catch (e: any) {
    ElMessage.error(e?.message || 'Failed to fetch disks')
    diskList.value = []
  } finally {
    loadingDisks.value = false
  }
}

async function fetchNetCards() {
  loadingNetCards.value = true
  try {
    netCardList.value = await GetNetworkCards(props.sessionId)
  } catch (e: any) {
    ElMessage.error(e?.message || 'Failed to fetch network cards')
    netCardList.value = []
  } finally {
    loadingNetCards.value = false
  }
}
```

- [ ] **Step 2: Add tab activation watcher**

Change the existing `watch(activeTab, ...)` from:
```typescript
watch(activeTab, (tab) => {
  SetMonitorActiveTab(props.sessionId, tab).catch(() => {})
})
```
to:
```typescript
watch(activeTab, (tab) => {
  SetMonitorActiveTab(props.sessionId, tab).catch(() => {})
  if (tab === 'ports' && portList.value.length === 0 && !loadingPorts.value) {
    fetchPorts()
  }
  if (tab === 'disks' && diskList.value.length === 0 && !loadingDisks.value) {
    fetchDisks()
  }
  if (tab === 'network' && netCardList.value.length === 0 && !loadingNetCards.value) {
    fetchNetCards()
  }
})
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/MonitorTabContent.vue
git commit -m "feat(monitor): add fetch methods and tab activation lazy-loading"
```

---

### Task 10: Add CSS styles for new panes

**Files:**
- Modify: `frontend/src/components/MonitorTabContent.vue`

- [ ] **Step 1: Add styles**

Insert before the closing `</style>` tag:

```css
/* On-demand tabs (ports, disks, network) */
.od-toolbar {
  display: flex;
  justify-content: flex-end;
  padding: 8px 12px;
  flex-shrink: 0;
}

.ports-pane,
.disks-pane,
.network-pane {
  flex-direction: column;
  padding: 0;
}

.od-table {
  flex: 1;
  min-height: 0;
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/MonitorTabContent.vue
git commit -m "feat(monitor): add CSS styles for on-demand tabs"
```

---

### Task 11: Generate Wails bindings and build

**Files:**
- Auto-generated: `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Generate bindings**

```bash
wails generate bindings
```

Expected: Bindings generated successfully. New types `PortInfo`, `DiskInfo`, `NetCardInfo` should appear in `frontend/wailsjs/go/models.ts`. New methods `GetPorts`, `GetDisks`, `GetNetworkCards` should appear in `frontend/wailsjs/go/main/App.d.ts`.

- [ ] **Step 2: Build frontend**

```bash
cd frontend && npm run build && cd ..
```

Expected: Build succeeds with no errors.

- [ ] **Step 3: Start dev mode to verify**

```bash
wails dev
```

Verify:
1. Open a monitor session
2. Click "端口" tab → port list loads after a brief delay
3. Click "磁盘" tab → disk list loads
4. Click "网卡" tab → network card list loads
5. Click refresh button on each tab → data reloads

- [ ] **Step 4: Commit generated bindings**

```bash
git add frontend/wailsjs/
git commit -m "chore: regenerate wails bindings for monitor extra tabs"
```

---

## Self-Review

**1. Spec coverage:**
- Ports tab with LISTEN ports ✓ (Task 2, 8)
- Disks tab with lsblk + df info ✓ (Task 3, 8)
- Network tab with physical NICs + bond info ✓ (Task 4, 8)
- Manual refresh button ✓ (Task 8)
- Open-once loading ✓ (Task 9, lazy-loading on tab switch)
- No auto-poll ✓ (not connected to ticker)

**2. Placeholder scan:** No TBD, TODO, or vague steps found.

**3. Type consistency:**
- `PortInfo`, `DiskInfo`, `NetCardInfo` structs match between Go and frontend usage
- Wails binding method signatures consistent with existing patterns (`sessionID string` first param)
- i18n keys match between `zh-CN` and `en` sections
