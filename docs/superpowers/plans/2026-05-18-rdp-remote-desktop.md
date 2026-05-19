# RDP Remote Desktop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Windows RDP remote desktop as a new connection type and tab type, using the native MsRdpClient ActiveX control embedded as a child window in the Wails main window.

**Architecture:** Go backend creates an ActiveX host child window (via ATL `AtlAxWin`) of the Wails main window. Frontend calculates tab content area coordinates and sends them to Go, which positions the child window. Frontend `RDPTabContent.vue` provides Vue-level state management (connecting/connected/disconnected/error) while RDP rendering happens in the native HWND.

**Tech Stack:** Go (`go-ole` for COM IDispatch, `golang.org/x/sys/windows` for Win32), Vue 3 (new RDPTabContent component), atl.dll (Windows built-in ATL ActiveX hosting)

**Note:** No commits between tasks — user will review and commit when ready.

---

### Task 1: Add RDP fields to backend ConnectionConfig

**Files:**
- Modify: `backend/session/session.go:19-31`

- [ ] **Step 1: Add RDP-specific fields to ConnectionConfig struct**

```go
type ConnectionConfig struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	User     string  `json:"user"`
	AuthType string  `json:"authType"`
	// Password is stored in plaintext JSON. Will be migrated to OS keychain in a future iteration.
	Password string  `json:"password,omitempty"`
	KeyPath  string  `json:"keyPath,omitempty"`
	GroupId  *string `json:"groupId,omitempty"`

	// RDP-specific fields
	RdpSizeMode    string `json:"rdpSizeMode,omitempty"`    // "follow" | "fixed"
	RdpFixedWidth  int    `json:"rdpFixedWidth,omitempty"`
	RdpFixedHeight int    `json:"rdpFixedHeight,omitempty"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd backend/session && go build ./...
```

Expected: builds without errors.

---

### Task 2: Update frontend types for RDP

**Files:**
- Modify: `frontend/src/types/session.ts:8-19`
- Modify: `frontend/src/types/workspace.ts:1,37,62-67`

- [ ] **Step 1: Extend ConnectionConfig type in session.ts**

```ts
export interface ConnectionConfig {
  id: string
  name: string
  type: 'ssh' | 'rdp'
  host: string
  port: number
  user: string
  authType: 'password' | 'key' | 'agent'
  password?: string
  keyPath?: string
  groupId?: string
  // RDP-specific
  rdpSizeMode?: 'follow' | 'fixed'
  rdpFixedWidth?: number
  rdpFixedHeight?: number
}
```

- [ ] **Step 2: Add RDPTab type and extend PanelType in workspace.ts**

```ts
// Line 1 — change PanelType:
export type PanelType = 'ssh' | 'sftp' | 'settings' | 'rdp' | 'other'

// After SFTPTab (line 67), add:
export interface RDPTab {
  type: 'rdp'
  id: string
  panelId: string
  name: string
}

// Line 37 — change Tab union:
export type Tab = TerminalTab | SettingsTab | WorkspaceTab | SFTPTab | RDPTab
```

---

### Task 3: Add RDP i18n strings

**Files:**
- Modify: `frontend/src/i18n/index.ts`

- [ ] **Step 1: Add zh-CN translations**

In the `'zh-CN'` section, add:
```ts
// RDP
'conn.rdpSizeMode': '分辨率模式',
'conn.rdpFollowWindow': '跟随窗口',
'conn.rdpFixedSize': '固定分辨率',
'sidebar.connectRDP': '连接 RDP',
'rdp.connecting': '正在连接到 {host}...',
'rdp.connected': '已连接',
'rdp.disconnected': '已断开',
'rdp.error': '连接失败',
'rdp.reconnect': '重新连接',
'rdp.retry': '重试',
'rdp.resolution': '分辨率',
```

- [ ] **Step 2: Add en translations**

In the `'en'` section, add:
```ts
// RDP
'conn.rdpSizeMode': 'Resolution Mode',
'conn.rdpFollowWindow': 'Follow Window',
'conn.rdpFixedSize': 'Fixed Resolution',
'sidebar.connectRDP': 'Connect RDP',
'rdp.connecting': 'Connecting to {host}...',
'rdp.connected': 'Connected',
'rdp.disconnected': 'Disconnected',
'rdp.error': 'Connection failed',
'rdp.reconnect': 'Reconnect',
'rdp.retry': 'Retry',
'rdp.resolution': 'Resolution',
```

---

### Task 4: Create RDPSession backend with ActiveX hosting

**Files:**
- Create: `backend/session/rdp_session.go`

- [ ] **Step 1: Create rdp_session.go**

```go
package session

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	atlDll             = windows.NewLazySystemDLL("atl.dll")
	procAtlAxWinInit    = atlDll.NewProc("AtlAxWinInit")
	procAtlAxGetControl = atlDll.NewProc("AtlAxGetControl")

	user32Dll         = windows.NewLazySystemDLL("user32.dll")
	procSetWindowPos  = user32Dll.NewProc("SetWindowPos")
	procShowWindow    = user32Dll.NewProc("ShowWindow")
	procDestroyWindow = user32Dll.NewProc("DestroyWindow")
	procFindWindowW   = user32Dll.NewProc("FindWindowW")
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	HWND_TOP       = 0
	SWP_SHOWWINDOW = 0x0040
	SWP_NOMOVE     = 0x0002
)

type RDPSession struct {
	baseSession
	parentHwnd uintptr
	hwnd       uintptr
	rdp        *ole.IDispatch
	config     ConnectionConfig
	mu         sync.Mutex
}

func NewRDPSession(id string) *RDPSession {
	return &RDPSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "rdp",
			status:      StatusDisconnected,
		},
	}
}

func (s *RDPSession) SetParentHwnd(hwnd uintptr) {
	s.parentHwnd = hwnd
}

func (s *RDPSession) Connect(config ConnectionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	s.title = fmt.Sprintf("%s@%s (RDP)", config.User, config.Host)
	s.setStatus(StatusConnecting)

	// COM must be initialized on the calling thread for ActiveX
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)

	if s.parentHwnd == 0 {
		title, _ := windows.UTF16PtrFromString("uniTerm")
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
		if hwnd == 0 {
			s.setStatus(StatusError)
			return fmt.Errorf("cannot find main window")
		}
		s.parentHwnd = hwnd
	}

	// Initialize ATL AxWin
	ret, _, _ := procAtlAxWinInit.Call()
	if ret == 0 {
		s.setStatus(StatusError)
		return fmt.Errorf("AtlAxWinInit failed")
	}

	progID := s.findRdpProgID()
	if progID == "" {
		s.setStatus(StatusError)
		return fmt.Errorf("no RDP ActiveX control found")
	}

	// Set default size
	width := config.RdpFixedWidth
	height := config.RdpFixedHeight
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}

	// Create AtlAxWin child window hosting the RDP control
	name, _ := windows.UTF16PtrFromString(progID)
	className, _ := windows.UTF16PtrFromString("AtlAxWin")

	hwnd, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("CreateWindowExW").Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(name)),
		uintptr(windows.WS_CHILD|windows.WS_CLIPSIBLINGS),
		0, 0,
		uintptr(width), uintptr(height),
		s.parentHwnd,
		0, 0, 0,
	)
	if hwnd == 0 {
		s.setStatus(StatusError)
		return fmt.Errorf("CreateWindowEx failed for RDP control")
	}
	s.hwnd = hwnd

	// Get the ActiveX control's IUnknown
	var unk *ole.IUnknown
	procAtlAxGetControl.Call(hwnd, uintptr(unsafe.Pointer(&unk)))
	if unk == nil {
		procDestroyWindow.Call(hwnd)
		s.hwnd = 0
		s.setStatus(StatusError)
		return fmt.Errorf("AtlAxGetControl failed")
	}

	dispatch, err := unk.QueryInterface(ole.IID_IDispatch)
	unk.Release()
	if err != nil {
		procDestroyWindow.Call(hwnd)
		s.hwnd = 0
		s.setStatus(StatusError)
		return fmt.Errorf("QI IDispatch: %w", err)
	}
	s.rdp = dispatch

	// ── Configure RDP properties ──
	port := config.Port
	if port <= 0 {
		port = 3389
	}

	dispatch.PutProperty("Server", config.Host)
	dispatch.PutProperty("UserName", config.User)
	dispatch.PutProperty("Domain", "")
	dispatch.PutProperty("DesktopWidth", width)
	dispatch.PutProperty("DesktopHeight", height)

	// AdvancedSettings2
	advObj, err := dispatch.GetProperty("AdvancedSettings2")
	if err == nil && advObj != nil {
		adv := advObj.ToIDispatch()
		if adv != nil {
			adv.PutProperty("RDPPort", port)
			adv.PutProperty("RedirectClipboard", true)
			adv.PutProperty("RedirectDrives", true)
			adv.PutProperty("DisplayConnectionBar", true)
			adv.PutProperty("EnableAutoReconnect", true)
			adv.PutProperty("AuthenticationLevel", 2) // require NLA
			adv.Release()
		}
	}

	// Non-scriptable interface for password
	s.setClearTextPassword(config.Password)

	// Connect
	_, err = dispatch.CallMethod("Connect")
	if err != nil {
		dispatch.Release()
		procDestroyWindow.Call(hwnd)
		s.hwnd = 0
		s.setStatus(StatusError)
		return fmt.Errorf("RDP Connect: %w", err)
	}

	s.setStatus(StatusConnected)
	return nil
}

// findRdpProgID tries to find the best available RDP client ProgID.
func (s *RDPSession) findRdpProgID() string {
	candidates := []string{
		"MsRdpClient12NotSafeForScripting",
		"MsRdpClient10NotSafeForScripting",
		"MsRdpClient9NotSafeForScripting",
		"MsRdpClient8NotSafeForScripting",
		"MsTscAxNotSafeForScripting",
		"MsTscAx",
	}
	ole32 := windows.NewLazySystemDLL("ole32.dll")
	procCLSIDFromProgID := ole32.NewProc("CLSIDFromProgID")
	for _, id := range candidates {
		progID, _ := windows.UTF16PtrFromString(id)
		var clsid ole.GUID
		ret, _, _ := procCLSIDFromProgID.Call(
			uintptr(unsafe.Pointer(progID)),
			uintptr(unsafe.Pointer(&clsid)),
		)
		if ret == 0 {
			return id
		}
	}
	return ""
}

// setClearTextPassword sets the password via the non-scriptable interface.
func (s *RDPSession) setClearTextPassword(password string) {
	if password == "" || s.rdp == nil {
		return
	}
	// IMsRdpClientNonScriptable5 GUID
	nsGUID := ole.NewGUID("{4F5331FB-42F5-48A2-9AFD-4743E3F6D3D7}")
	unk, err := s.rdp.QueryInterface(ole.IID_IUnknown)
	if err != nil {
		return
	}
	nsUnk, err := unk.QueryInterface(nsGUID)
	unk.Release()
	if err != nil {
		return
	}
	nsDisp := nsUnk.ToIDispatch()
	if nsDisp != nil {
		nsDisp.PutProperty("ClearTextPassword", password)
		nsDisp.Release()
	}
	nsUnk.Release()
}

func (s *RDPSession) SetPosition(x, y, w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hwnd == 0 {
		return
	}
	procSetWindowPos.Call(s.hwnd, HWND_TOP,
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		SWP_SHOWWINDOW)
}

func (s *RDPSession) Show() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hwnd != 0 {
		procShowWindow.Call(s.hwnd, SW_SHOW)
		procSetWindowPos.Call(s.hwnd, HWND_TOP, 0, 0, 0, 0,
			SWP_SHOWWINDOW|SWP_NOMOVE|SWP_NOMOVE>>1) // SWP_NOSIZE
	}
}

func (s *RDPSession) Hide() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hwnd != 0 {
		procShowWindow.Call(s.hwnd, SW_HIDE)
	}
}

func (s *RDPSession) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rdp != nil {
		s.rdp.CallMethod("Disconnect")
		s.rdp.Release()
		s.rdp = nil
	}
	if s.hwnd != 0 {
		procDestroyWindow.Call(s.hwnd)
		s.hwnd = 0
	}
	s.setStatus(StatusDisconnected)
	ole.CoUninitialize()
	return nil
}

func (s *RDPSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rdp != nil {
		s.rdp.PutProperty("DesktopWidth", cols)
		s.rdp.PutProperty("DesktopHeight", rows)
	}
	if s.hwnd != 0 {
		procSetWindowPos.Call(s.hwnd, HWND_TOP, 0, 0,
			uintptr(cols), uintptr(rows),
			SWP_SHOWWINDOW|SWP_NOMOVE|SWP_NOMOVE>>1)
	}
	return nil
}

func (s *RDPSession) Write(data []byte) error {
	// Keyboard input is handled natively by the ActiveX control
	return nil
}

func (s *RDPSession) IsConnected() bool {
	return s.Status() == StatusConnected
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd backend && go build ./...
```

Expected: builds without errors. If `go-ole` is not a direct dependency, go will auto-add it to go.mod.

---

### Task 5: Register RDP session type in SessionManager

**Files:**
- Modify: `backend/session/manager.go:28-34`

- [ ] **Step 1: Add rdp case to Create switch**

```go
// In manager.go, inside Create(), after case "sftp":
case "rdp":
	s = NewRDPSession(config.ID)
```

- [ ] **Step 2: Verify compilation**

```bash
cd backend && go build ./...
```

---

### Task 6: Add RDP window management methods to app.go

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add imports and mainHwnd field**

Add to imports (if not present):
```go
import (
	"unsafe"
	"golang.org/x/sys/windows"
)
```

Add to App struct:
```go
type App struct {
	ctx             context.Context
	sessionManager  *session.SessionManager
	connectionStore *store.ConnectionStore
	aiConfigStore   *store.AIConfigStore
	settingsStore   *store.SettingsStore
	mainHwnd        uintptr
}
```

- [ ] **Step 2: Add main window discovery in startup()**

After `a.sessionManager = session.NewSessionManager()`:
```go
// Discover main window HWND for RDP child window embedding
a.mainHwnd = a.findMainWindow()
```

- [ ] **Step 3: Add helper and RDP methods**

After `SessionResize()`, add:
```go
func (a *App) findMainWindow() uintptr {
	title, _ := windows.UTF16PtrFromString("uniTerm")
	hwnd, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW").Call(
		0,
		uintptr(unsafe.Pointer(title)),
	)
	return hwnd
}

func (a *App) RDPSetPosition(sessionID string, x, y, w, h int) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.SetPosition(x, y, w, h)
	return nil
}

func (a *App) RDPShow(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.Show()
	return nil
}

func (a *App) RDPHide(sessionID string) error {
	if a.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	rdp, ok := s.(*session.RDPSession)
	if !ok {
		return fmt.Errorf("session is not RDP")
	}
	rdp.Hide()
	return nil
}
```

- [ ] **Step 4: Set parent HWND for RDP sessions in CreateSession**

In `CreateSession()`, after `s, err := a.sessionManager.Create(...)` and before the goroutine:
```go
// Set parent HWND for RDP sessions
if rdp, ok := s.(*session.RDPSession); ok {
	rdp.SetParentHwnd(a.mainHwnd)
}
```

- [ ] **Step 5: Verify compilation**

```bash
go build -o /dev/null ./...
```

---

### Task 7: Update ConnectionForm.vue for RDP

**Files:**
- Modify: `frontend/src/components/ConnectionForm.vue`

- [ ] **Step 1: Add RDP radio button**

Change type selector from:
```html
<el-radio-group v-model="form.type">
  <el-radio-button label="ssh">SSH</el-radio-button>
</el-radio-group>
```
To:
```html
<el-radio-group v-model="form.type">
  <el-radio-button label="ssh">SSH</el-radio-button>
  <el-radio-button label="rdp">RDP</el-radio-button>
</el-radio-group>
```

- [ ] **Step 2: Hide authType for RDP, show password always for RDP**

```html
<el-form-item v-if="form.type !== 'rdp'" :label="t('conn.authType')">
  <el-radio-group v-model="form.authType">
    <el-radio-button label="password">{{ t('conn.password') }}</el-radio-button>
    <el-radio-button label="key">{{ t('conn.keyPath') }}</el-radio-button>
  </el-radio-group>
</el-form-item>
<el-form-item v-if="form.authType === 'password' || form.type === 'rdp'" :label="t('conn.password')">
  <el-input v-model="form.password" type="password" show-password />
</el-form-item>
<el-form-item v-if="form.authType === 'key' && form.type !== 'rdp'" :label="t('conn.keyPath')">
  <el-input v-model="form.keyPath" :placeholder="t('conn.keyPathPlaceholder')" />
</el-form-item>
```

- [ ] **Step 3: Add RDP-specific fields after port field**

```html
<template v-if="form.type === 'rdp'">
  <el-form-item :label="t('conn.rdpSizeMode')">
    <el-radio-group v-model="form.rdpSizeMode">
      <el-radio-button label="follow">{{ t('conn.rdpFollowWindow') }}</el-radio-button>
      <el-radio-button label="fixed">{{ t('conn.rdpFixedSize') }}</el-radio-button>
    </el-radio-group>
  </el-form-item>
  <el-form-item v-if="form.rdpSizeMode === 'fixed'" :label="t('rdp.resolution')">
    <el-select v-model="rdpResolution" placeholder="1280×720">
      <el-option
        v-for="r in rdpResolutions"
        :key="r.label"
        :label="r.label"
        :value="r.label"
      />
    </el-select>
  </el-form-item>
</template>
```

- [ ] **Step 4: Update script — add resolution data and RDP defaults**

In `<script setup>`:

```ts
const rdpResolutions = [
  { label: '1280 × 720 (HD)', w: 1280, h: 720 },
  { label: '1920 × 1080 (Full HD)', w: 1920, h: 1080 },
  { label: '2560 × 1440 (QHD)', w: 2560, h: 1440 },
  { label: '1024 × 768 (XGA)', w: 1024, h: 768 },
  { label: '1600 × 1200 (UXGA)', w: 1600, h: 1200 },
  { label: '1680 × 1050 (WSXGA+)', w: 1680, h: 1050 },
]

const rdpResolution = ref('1280 × 720 (HD)')
```

Update the form reactive:
```ts
const form = reactive<ConnectionConfig>({
  id: '',
  name: '',
  type: 'ssh',
  host: '',
  port: 22,
  user: '',
  authType: 'password',
  password: '',
  keyPath: '',
  groupId: undefined,
  rdpSizeMode: 'follow',
  rdpFixedWidth: undefined,
  rdpFixedHeight: undefined
})
```

Update `resetForm()` similarly with RDP defaults.

- [ ] **Step 5: Add watchers for type switching and resolution**

```ts
// Auto-switch default port when changing type
watch(() => form.type, (newType) => {
  if (newType === 'rdp' && form.port === 22) form.port = 3389
  else if (newType === 'ssh' && form.port === 3389) form.port = 22
  if (newType === 'rdp') {
    form.authType = 'password'
    form.rdpSizeMode = form.rdpSizeMode || 'follow'
  }
})

// Sync resolution picker to form fields
watch(rdpResolution, (val) => {
  const found = rdpResolutions.find(r => r.label === val)
  if (found) {
    form.rdpFixedWidth = found.w
    form.rdpFixedHeight = found.h
  }
})
```

---

### Task 8: Create RDPTabContent.vue component

**Files:**
- Create: `frontend/src/components/RDPTabContent.vue`

- [ ] **Step 1: Create the component**

```vue
<template>
  <div ref="containerRef" class="rdp-tab-content">
    <!-- Connecting state -->
    <div v-if="status === 'connecting'" class="rdp-overlay">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>{{ t('rdp.connecting', { host: config?.host || '...' }) }}</p>
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="rdp-overlay">
      <p class="rdp-error-text">{{ t('rdp.error') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('rdp.retry') }}</el-button>
    </div>

    <!-- Disconnected state -->
    <div v-else-if="status === 'disconnected'" class="rdp-overlay">
      <p>{{ t('rdp.disconnected') }}</p>
      <el-button type="primary" @click="reconnect">{{ t('rdp.reconnect') }}</el-button>
    </div>

    <!-- Connected: placeholder div overlaid by native RDP HWND -->
    <div
      v-show="status === 'connected'"
      ref="rdpAreaRef"
      class="rdp-area"
      :class="{ 'rdp-fixed': sizeMode === 'fixed' }"
      :style="rdpAreaStyle"
    />

    <!-- Status bar -->
    <div v-if="status === 'connected'" class="rdp-statusbar">
      <span class="rdp-status-dot" />
      <span>{{ t('rdp.connected') }}</span>
      <span class="rdp-status-sep">|</span>
      <span>{{ config?.host }}:{{ config?.port || 3389 }}</span>
      <span v-if="sizeMode === 'fixed'" class="rdp-status-sep">|</span>
      <span v-if="sizeMode === 'fixed'">{{ t('rdp.resolution') }}: {{ config?.rdpFixedWidth }}×{{ config?.rdpFixedHeight }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from '../i18n'
import type { ConnectionConfig } from '../types/session'
import { CreateSession, CloseSession, RDPSetPosition, RDPShow, RDPHide } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const { t } = useI18n()

const props = defineProps<{
  panelId: string
  config: ConnectionConfig | null
  sessionId: string | null
}>()

const containerRef = ref<HTMLElement>()
const rdpAreaRef = ref<HTMLElement>()
const status = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
const currentSessionId = ref<string | null>(props.sessionId)
const sizeMode = computed(() => props.config?.rdpSizeMode || 'follow')

const rdpAreaStyle = computed(() => {
  if (sizeMode.value === 'fixed' && props.config?.rdpFixedWidth && props.config?.rdpFixedHeight) {
    return {
      width: props.config.rdpFixedWidth + 'px',
      height: props.config.rdpFixedHeight + 'px',
    }
  }
  return {}
})

function syncRDPPosition() {
  if (!rdpAreaRef.value || !currentSessionId.value || status.value !== 'connected') return
  const rect = rdpAreaRef.value.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
  RDPSetPosition(
    currentSessionId.value,
    Math.round(rect.left * dpr),
    Math.round(rect.top * dpr),
    Math.round(rect.width * dpr),
    Math.round(rect.height * dpr)
  )
}

async function connect() {
  if (!props.config) return
  status.value = 'connecting'

  try {
    const info = await CreateSession('rdp', props.config)
    currentSessionId.value = info.id

    EventsOn('session:status', (data: any) => {
      if (data.id !== currentSessionId.value) return
      switch (data.status) {
        case 'connected':
          status.value = 'connected'
          nextTick(() => { syncRDPPosition(); RDPShow(currentSessionId.value!) })
          break
        case 'disconnected':
          status.value = 'disconnected'
          RDPHide(currentSessionId.value!)
          break
        case 'error':
          status.value = 'error'
          RDPHide(currentSessionId.value!)
          break
      }
    })

    EventsOn('session:data', (data: any) => {
      if (data.id === currentSessionId.value && data.data?.includes('[Connection failed')) {
        status.value = 'error'
      }
    })
  } catch (e) {
    console.error('RDP connect error:', e)
    status.value = 'error'
  }
}

async function reconnect() {
  if (currentSessionId.value) {
    try { await CloseSession(currentSessionId.value) } catch (_) {}
    currentSessionId.value = null
  }
  await connect()
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  if (props.sessionId) currentSessionId.value = props.sessionId

  if (sizeMode.value === 'follow' && rdpAreaRef.value) {
    resizeObserver = new ResizeObserver(() => syncRDPPosition())
    resizeObserver.observe(rdpAreaRef.value)
  }
  window.addEventListener('resize', syncRDPPosition)

  // Session is created by App.vue's onConnectRDP; RDPTabContent just monitors events.
  if (!props.sessionId) {
    // Still waiting for parent to create session — start in connecting state.
    status.value = 'connecting'
  }
})

// React to sessionId being set after mount (parent creates session async)
watch(() => props.sessionId, (newId) => {
  if (newId && !currentSessionId.value) {
    currentSessionId.value = newId
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', syncRDPPosition)
  resizeObserver?.disconnect()
  if (currentSessionId.value) {
    RDPHide(currentSessionId.value)
    CloseSession(currentSessionId.value).catch(() => {})
  }
})

defineExpose({ syncRDPPosition })
</script>

<style scoped>
.rdp-tab-content {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: #000;
  position: relative;
}
.rdp-area {
  flex: 1;
  background: #000;
}
.rdp-area.rdp-fixed {
  margin: 0 auto;
  flex: none;
}
.rdp-overlay {
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
.rdp-error-text { color: #f56c6c; }
.rdp-statusbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: #1e1e1e;
  color: #999;
  font-size: 12px;
  flex-shrink: 0;
}
.rdp-status-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: #67c23a;
}
.rdp-status-sep { color: #444; }
</style>
```

---

### Task 9: Add createRDPTab to tabStore

**Files:**
- Modify: `frontend/src/stores/tabStore.ts`

- [ ] **Step 1: Update import to include RDPTab**

```ts
import type { Tab, TerminalTab, SettingsTab, WorkspaceTab, SFTPTab, RDPTab, PanelLayout, LayoutNode } from '../types/workspace'
```

- [ ] **Step 2: Add createRDPTab factory function**

After `createSFPTab`:
```ts
function createRDPTab(name: string, panelId: string): RDPTab {
  const tab: RDPTab = {
    type: 'rdp',
    id: genId('rdp-tab'),
    panelId,
    name
  }
  tabState.tabs.push(tab)
  tabState.activeTabId = tab.id
  return tab
}
```

- [ ] **Step 3: Update closeTab to handle RDP panel IDs**

In `closeTab`, change the removedPanelIds ternary:
```ts
const removedPanelIds = removed.type === 'terminal' || removed.type === 'settings' || removed.type === 'rdp'
  ? [removed.panelId]
  : removed.type === 'workspace'
    ? removed.panelIds
    : removed.type === 'sftp'
      ? [removed.panelId]
      : []
```

- [ ] **Step 4: Export createRDPTab in return statement**

Add `createRDPTab` to the returned object.

---

### Task 10: Update App.vue for RDP connection flow and tab rendering

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Import RDPTabContent**

```ts
import RDPTabContent from './components/RDPTabContent.vue'
```

- [ ] **Step 2: Add RDP tab rendering in template**

After the SFTPTabContent block, add:
```html
<RDPTabContent
  v-else-if="activeTab.type === 'rdp'"
  :key="activeTab.id"
  :panel-id="activeTab.panelId"
  :config="getPanelConfig(activeTab.panelId)"
  :session-id="getPanelSessionId(activeTab.panelId)"
/>
```

- [ ] **Step 3: Add helper functions**

```ts
function getPanelConfig(panelId: string): ConnectionConfig | null {
  return panelStore.getPanel(panelId)?.config || null
}

function getPanelSessionId(panelId: string): string | null {
  return panelStore.getPanel(panelId)?.sessionId || null
}
```

- [ ] **Step 4: Add RDP connect handler and route by type**

```ts
async function onConnectRDP(config: ConnectionConfig) {
  const displayTitle = config.name
    ? `${config.name} (RDP)`
    : `${config.user}@${config.host} (RDP)`

  const panel = panelStore.createPanel(config, 'rdp')
  panel.title = displayTitle
  const tab = tabStore.createRDPTab(displayTitle, panel.id)
  panelStore.movePanelToTab(panel.id, tab.id)

  try {
    const info = await CreateSession('rdp', config)
    panelStore.bindSession(panel.id, info.id)
    sessionStore.initSession(info.id)
  } catch (e) {
    console.error('Failed to create RDP session:', e)
    tabStore.closeTab(tab.id)
    panelStore.removePanel(panel.id)
  }
}
```

Modify `onConnect` to route by type:
```ts
async function onConnect(config: ConnectionConfig) {
  if (config.type === 'rdp') return onConnectRDP(config)
  // ... existing SSH logic unchanged
}
```

- [ ] **Step 5: Wire Sidebar @connect-rdp and add tab switch watcher**

Add `@connect-rdp="onConnectRDP"` to the Sidebar tag.

Add tab switch watcher for RDP native window show/hide:
```ts
watch(() => activeTab.value, (newTab, oldTab) => {
  if (oldTab?.type === 'rdp') {
    const p = panelStore.getPanel(oldTab.panelId)
    if (p?.sessionId) RDPHide(p.sessionId)
  }
  if (newTab?.type === 'rdp') {
    const p = panelStore.getPanel(newTab.panelId)
    if (p?.sessionId) nextTick(() => RDPShow(p.sessionId!))
  }
})
```

Add necessary imports: `RDPHide, RDPShow` from wailsjs bindings, `watch`, `nextTick` from Vue.

---

### Task 11: Add RDP context menu item to Sidebar

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: Add "Connect RDP" to context menu template**

After the `doConnectSFTP` line:
```html
<div class="menu-item" @click="doConnectRDP">{{ t('sidebar.connectRDP') }}</div>
```

- [ ] **Step 2: Update emit declaration**

```ts
const emit = defineEmits(['connect', 'connectSftp', 'connectRDP', 'toggle'])
```

- [ ] **Step 3: Add doConnectRDP function**

```ts
function doConnectRDP() {
  const ids = getSelectedConnectionIds()
  const conns = ids.map(id => connectionStore.connections.find(c => c.id === id)).filter(Boolean) as ConnectionConfig[]
  multiSelectedIds.value = new Set()
  closeMenu()
  for (const c of conns) {
    emit('connectRDP', c)
  }
}
```

---

### Task 12: Build and verify

- [ ] **Step 1: Clean cache and run wails dev**

```bash
cd frontend && rm -rf dist node_modules/.vite && cd .. && wails dev
```

Expected: app starts without errors. Check:
- ConnectionForm shows SSH | RDP type selector
- Switching to RDP shows RDP-specific fields (resolution mode, no auth type)
- Creating an RDP connection and connecting works
- RDP tab appears with connecting/connected states
- Native RDP window appears over the tab content area

- [ ] **Step 2: Test edge cases**

1. Edit an existing RDP connection
2. Duplicate an RDP connection
3. Close RDP tab while connected
4. Switch tabs while RDP is connected
5. Follow vs fixed resolution modes
6. Fixed resolution with different options

- [ ] **Step 3: Fix any issues**

Iterate on issues found during verification. No commits until user approves.
