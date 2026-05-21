package session

import (
	"runtime"
	"fmt"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/ys-ll/uniterm/backend/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	atlDll              = windows.NewLazySystemDLL("atl.dll")
	procAtlAxWinInit    = atlDll.NewProc("AtlAxWinInit")
	procAtlAxGetControl = atlDll.NewProc("AtlAxGetControl")

	user32Dll              = windows.NewLazySystemDLL("user32.dll")
	procSetWindowPos       = user32Dll.NewProc("SetWindowPos")
	procDestroyWindow      = user32Dll.NewProc("DestroyWindow")
	procFindWindowW        = user32Dll.NewProc("FindWindowW")
	procPeekMessage        = user32Dll.NewProc("PeekMessageW")
	procTranslateMessage   = user32Dll.NewProc("TranslateMessage")
	procDispatchMessage    = user32Dll.NewProc("DispatchMessageW")
	procGetWindowRect      = user32Dll.NewProc("GetWindowRect")
	procGetClientRect      = user32Dll.NewProc("GetClientRect")
	procClientToScreen     = user32Dll.NewProc("ClientToScreen")
	procPostMessageW       = user32Dll.NewProc("PostMessageW")
	procIsIconic           = user32Dll.NewProc("IsIconic")
	procGetForegroundWindow = user32Dll.NewProc("GetForegroundWindow")
	procGetAncestor         = user32Dll.NewProc("GetAncestor")
)

const (
	WM_CLOSE       = 0x0010
	HWND_TOPMOST   = ^uintptr(0) // -1 as uintptr for syscall compatibility
	HWND_NOTOPMOST = ^uintptr(1) // -2 as uintptr for syscall compatibility
	SWP_SHOWWINDOW = 0x0040
	SWP_HIDEWINDOW = 0x0080
	SWP_NOMOVE     = 0x0002
	SWP_NOSIZE     = 0x0001
	SWP_NOACTIVATE = 0x0010
	SWP_NOZORDER   = 0x0004
	SWP_ASYNCWINDOWPOS = 0x4000 // non-blocking: avoids freezing RDP COM thread
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000
	WS_POPUP          = 0x80000000
	WS_CLIPSIBLINGS   = 0x04000000
	GA_ROOT           = 2
	PM_REMOVE         = 0x0001
	GWLP_HWNDPARENT   = ^uintptr(7) // -8 represented as uintptr for syscall compatibility
)

type RDPSession struct {
	baseSession
	parentHwnd uintptr
	hwnd       uintptr
	rdp        *ole.IDispatch
	config     ConnectionConfig
	mu         sync.Mutex
	shown     bool

	// Position tracking
	trackX, trackY       int
	refParentX, refParentY int
	refRdpX, refRdpY     int
	trackStop            chan struct{}
	trackGen             int       // incremented on each startTracking; stale goroutines exit
	wasMinimized         bool      // tracks parent minimize→restore transitions
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

// ClientAreaScreenRect returns the main window's client area in screen coordinates
// (physical pixels). Used by the frontend to position the RDP overlay precisely.
func (s *RDPSession) ClientAreaScreenRect() (x, y, w, h int) {
	if s.parentHwnd == 0 {
		return
	}
	var cr rect
	ret, _, _ := procGetClientRect.Call(s.parentHwnd, uintptr(unsafe.Pointer(&cr)))
	if ret == 0 {
		return
	}
	var origin point
	ret, _, _ = procClientToScreen.Call(s.parentHwnd, uintptr(unsafe.Pointer(&origin)))
	if ret == 0 {
		return
	}
	return int(origin.X), int(origin.Y), int(cr.Right), int(cr.Bottom)
}

func (s *RDPSession) SetParentHwnd(hwnd uintptr) {
	s.parentHwnd = hwnd
}

func (s *RDPSession) storeCredentials(host, user, password string) {
	cmd := exec.Command("cmdkey", "/generic:TERMSRV/"+host, "/user:"+user, "/pass:"+password)
	if err := cmd.Run(); err != nil {
		log.Writef("[RDP] cmdkey failed: %v", err)
	} else {
		log.Writef("[RDP] credentials stored for TERMSRV/%s", host)
	}
}

func (s *RDPSession) Connect(config ConnectionConfig) error {
	log.Writef("[RDP] starting connect to %s:%d as %s", config.Host, config.Port, config.User)

	defer func() {
		if r := recover(); r != nil {
			log.Writef("[RDP] PANIC in Connect: %v", r)
			s.setStatus(StatusError)
		}
	}()

	// Phase 1: quick state init (brief lock)
	s.mu.Lock()
	s.config = config
	s.title = fmt.Sprintf("%s@%s (RDP)", config.User, config.Host)
	s.setStatus(StatusConnecting)
	s.mu.Unlock()

	runtime.LockOSThread() // pin COM STA to a dedicated OS thread
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer func() {
		s.stopTracking()
		s.mu.Lock()
		hwnd := s.hwnd
		s.hwnd = 0
		s.mu.Unlock()
		if hwnd != 0 {
			procDestroyWindow.Call(hwnd)
		}
		s.mu.Lock()
		if s.rdp != nil {
			s.rdp.Release()
			s.rdp = nil
		}
		ole.CoUninitialize()
		s.mu.Unlock()
	}()

	if s.parentHwnd == 0 {
		title, _ := windows.UTF16PtrFromString("uniTerm")
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
		if hwnd == 0 {
			log.Writef("[RDP] ERROR: cannot find main window")
			s.setStatus(StatusError)
			return fmt.Errorf("cannot find main window")
		}
		s.parentHwnd = hwnd
	}

	ret, _, _ := procAtlAxWinInit.Call()
	if ret == 0 {
		log.Writef("[RDP] ERROR: AtlAxWinInit failed")
		s.setStatus(StatusError)
		return fmt.Errorf("AtlAxWinInit failed")
	}

	progID := s.findRdpProgID()
	if progID == "" {
		log.Writef("[RDP] ERROR: no RDP ActiveX control found")
		s.setStatus(StatusError)
		return fmt.Errorf("no RDP ActiveX control found")
	}

	width := config.RdpFixedWidth
	height := config.RdpFixedHeight
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}

	// Create WS_POPUP off-screen
	name, _ := windows.UTF16PtrFromString(progID)
	className, _ := windows.UTF16PtrFromString("AtlAxWin")

	createWindowEx := windows.NewLazySystemDLL("user32.dll").NewProc("CreateWindowExW")
	hwnd, _, _ := createWindowEx.Call(
		uintptr(WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(name)),
		uintptr(WS_POPUP|WS_CLIPSIBLINGS),
		32000, 32000,
		uintptr(width), uintptr(height),
		// No owner window: avoids modal-loop deadlock with RDP ActiveX.
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		log.Writef("[RDP] ERROR: CreateWindowExW failed")
		s.setStatus(StatusError)
		return fmt.Errorf("CreateWindowEx failed")
	}
		log.Writef("[RDP] hwnd=0x%x parentHwnd=0x%x", hwnd, s.parentHwnd)

	var unk *ole.IUnknown
	procAtlAxGetControl.Call(hwnd, uintptr(unsafe.Pointer(&unk)))
	if unk == nil {
		procDestroyWindow.Call(hwnd)
		s.setStatus(StatusError)
		return fmt.Errorf("AtlAxGetControl failed")
	}

	dispatch, err := unk.QueryInterface(ole.IID_IDispatch)
	unk.Release()
	if err != nil {
		procDestroyWindow.Call(hwnd)
		s.setStatus(StatusError)
		return fmt.Errorf("QI IDispatch: %w", err)
	}

	s.mu.Lock()
	s.hwnd = hwnd
	s.rdp = dispatch
	s.mu.Unlock()

	port := config.Port
	if port <= 0 {
		port = 3389
	}

	dispatch.PutProperty("Server", config.Host)
	dispatch.PutProperty("UserName", config.User)
	dispatch.PutProperty("Domain", "")
	dispatch.PutProperty("DesktopWidth", width)
	dispatch.PutProperty("DesktopHeight", height)
	dispatch.PutProperty("FullScreen", false)
	dispatch.PutProperty("AuthenticationLevel", 0)

	// AdvancedSettings2
	advObj, _ := dispatch.GetProperty("AdvancedSettings2")
	if advObj != nil {
		adv := advObj.ToIDispatch()
		if adv != nil {
			adv.PutProperty("RDPPort", port)
			adv.PutProperty("RedirectClipboard", true)
			adv.PutProperty("RedirectDrives", true)
			adv.PutProperty("DisplayConnectionBar", false)
			adv.PutProperty("EnableAutoReconnect", true)
			adv.PutProperty("AuthenticationLevel", 0)
			adv.PutProperty("EnableCredSspSupport", false)
			adv.PutProperty("WarnOnDirectConnect", false)
			adv.PutProperty("ContainerHandledFullScreen", true)
			log.Writef("[RDP] ContainerHandledFullScreen set on AdvancedSettings2")
			if config.Password != "" {
				adv.PutProperty("ClearTextPassword", config.Password)
			}
			adv.Release()
		}
	}

	// Suppress security prompts on all available AdvancedSettings versions
	for _, ver := range []int{9, 8, 7, 6, 5, 4, 3} {
		propName := fmt.Sprintf("AdvancedSettings%d", ver)
		advHigh, _ := dispatch.GetProperty(propName)
		if advHigh != nil {
			a := advHigh.ToIDispatch()
			if a != nil {
				a.PutProperty("AuthenticationLevel", 0)
				a.PutProperty("ContainerHandledFullScreen", true)
				a.PutProperty("EnableCredSspSupport", false)
				a.PutProperty("WarnOnDirectConnect", false)
				a.Release()
				log.Writef("[RDP] AdvancedSettings%d: AuthLevel=0, CredSsp=false", ver)
			}
		}
	}

	// SecuredSettings2: disable security layer negotiation
	secObj, _ := dispatch.GetProperty("SecuredSettings2")
	if secObj != nil {
		sec := secObj.ToIDispatch()
		if sec != nil {
			sec.PutProperty("NegotiateSecurityLayer", false)
			sec.Release()
			log.Writef("[RDP] SecuredSettings2: NegotiateSecurityLayer=false")
		}
	}

	// Suppress server certificate warning at OS level
	setAuthLevelOverride()

	// Store credentials in Windows Credential Manager
	if config.Password != "" {
		s.storeCredentials(config.Host, config.User, config.Password)
	}

	// Suppress credential dialogs via NonScriptable interface
	s.configureNonScriptable(config.Password)

	log.Writef("[RDP] calling Connect...")
	_, err = dispatch.CallMethod("Connect")
	if err != nil {
		log.Writef("[RDP] Connect failed: %v", err)
		s.mu.Lock()
		s.hwnd = 0
		s.rdp = nil
		s.mu.Unlock()
		dispatch.Release()
		procDestroyWindow.Call(hwnd)
		s.setStatus(StatusError)
		return fmt.Errorf("RDP Connect: %w", err)
	}

	log.Writef("[RDP] Connect succeeded")
	// Immediate show-and-position to avoid white/black screen.
	// Frontend will refine via RDPSetPosition shortly after.
	s.positionFromMainWindow(width, height)

	s.startTracking()
	s.setStatus(StatusConnected)

	s.runMessagePump()

	log.Writef("[RDP] COM thread exited")
	return nil
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type msg struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

func (s *RDPSession) runMessagePump() {
	log.Writef("[RDP] message pump started")
	var m msg
	pumpTick := 0
	noMsgCount := 0
	for {
		s.mu.Lock()
		done := s.hwnd == 0
		s.mu.Unlock()
		if done {
			break
		}

		ret, _, _ := procPeekMessage.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
			PM_REMOVE,
		)
		if ret != 0 {
			if m.Message == 0x0012 { // WM_QUIT
				log.Writef("[RDP-pump] WM_QUIT received, exiting")
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
			pumpTick++
			noMsgCount = 0
		} else {
			// No message available; sleep briefly to avoid busy-wait.
			// Check hwnd every ~1 second via heartbeat counter.
			time.Sleep(50 * time.Millisecond)
			noMsgCount++
			if noMsgCount%20 == 0 {
				log.Writef("[RDP-pump] heartbeat idle=%d, pumpMsgs=%d, hwnd=0x%x", noMsgCount, pumpTick, s.hwnd)
			}
		}
	}
	log.Writef("[RDP] message pump exited")
}


func (s *RDPSession) findRdpProgID() string {
	candidates := []string{
		"MsRdpClient12NotSafeForScripting",
		"MsRdpClient11NotSafeForScripting",
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

	clsidCandidates := []string{
		"{9059F30F-4EB1-4BD2-9FDC-36F43A218F4A}",
		"{54D38BF7-B1EF-4479-9674-1BD6EA465258}",
		"{C0EFA91A-EEB7-41C7-97FA-F0ED645EFB24}",
		"{301B94BA-5F25-4A12-9FFE-3B274E75C7DE}",
		"{5F681803-2900-4C43-A1CC-CF405404A676}",
		"{1FB464C8-09BB-4017-A2F5-EB742F04392F}",
	}
	ole32Dll := windows.NewLazySystemDLL("ole32.dll")
	procCLSIDFromString := ole32Dll.NewProc("CLSIDFromString")
	for _, clsidStr := range clsidCandidates {
		wideStr, _ := windows.UTF16PtrFromString(clsidStr)
		var clsid ole.GUID
		ret, _, _ := procCLSIDFromString.Call(
			uintptr(unsafe.Pointer(wideStr)),
			uintptr(unsafe.Pointer(&clsid)),
		)
		if ret == 0 {
			return clsidStr
		}
	}

	return ""
}

// setAuthLevelOverride sets the system-wide RDP authentication level to 0,
// which suppresses the server certificate warning dialog.
func setAuthLevelOverride() {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Terminal Server Client`,
		registry.SET_VALUE)
	if err != nil {
		// Key may not exist; create it
		k, _, err = registry.CreateKey(registry.CURRENT_USER,
			`Software\Microsoft\Terminal Server Client`,
			registry.SET_VALUE)
		if err != nil {
			return
		}
	}
	defer k.Close()
	k.SetDWordValue("AuthenticationLevelOverride", 0)
}

func (s *RDPSession) configureNonScriptable(password string) {
	if s.rdp == nil {
		return
	}
	nsGUIDs := []string{
		"{4F5331FB-42F5-48A2-9AFD-4743E3F6D3D7}", // IMsRdpClientNonScriptable5
		"{F50FA8AA-1C05-471B-9CB5-3BD7A6FD32BD}", // IMsRdpClientNonScriptable4
		"{B3378D90-0728-45C7-8ED7-B6159FB92219}", // IMsRdpClientNonScriptable3
	}
	unk, err := s.rdp.QueryInterface(ole.IID_IUnknown)
	if err != nil {
		log.Writef("[RDP] QI IUnknown for NonScriptable: %v", err)
		return
	}
	defer unk.Release()
	for i, guid := range nsGUIDs {
		nsGUID := ole.NewGUID(guid)
		nsUnk, err := unk.QueryInterface(nsGUID)
		if err != nil || nsUnk == nil {
			continue
		}
		nsUnk.PutProperty("AllowPromptingForCredentials", false)
		nsUnk.PutProperty("PromptForCredentials", false)
		nsUnk.PutProperty("PromptForCredentialsOnce", false)
		nsUnk.PutProperty("AuthenticationLevel", 0)
		nsUnk.PutProperty("EnableCredSspSupport", false)
		nsUnk.PutProperty("MarkRdpSettingsSecure", true)
		nsUnk.CallMethod("MarkRdpSettingsSecure", true)
		if password != "" {
			nsUnk.PutProperty("ClearTextPassword", password)
		}
		nsUnk.Release()
		log.Writef("[RDP] NonScriptable configured via index %d (prompts disabled, password=%v)", i, password != "")
		return
	}
	log.Writef("[RDP] NonScriptable not available on this control")
}

type point struct{ X, Y int32 }

// positionFromMainWindow calculates the RDP window position and initializes tracking.
func (s *RDPSession) positionFromMainWindow(width, height int) {
	if s.parentHwnd == 0 || s.hwnd == 0 {
		return
	}
	var cr rect
	ret, _, _ := procGetClientRect.Call(s.parentHwnd, uintptr(unsafe.Pointer(&cr)))
	if ret == 0 {
		log.Writef("[RDP] GetClientRect failed, fallback to GetWindowRect")
		var wr rect
		ret2, _, _ := procGetWindowRect.Call(s.parentHwnd, uintptr(unsafe.Pointer(&wr)))
		if ret2 == 0 {
			log.Writef("[RDP] GetWindowRect also failed")
			return
		}
		cr = rect{0, 0, wr.Right - wr.Left, wr.Bottom - wr.Top}
	}
	var origin point
	ret2, _, _ := procClientToScreen.Call(s.parentHwnd, uintptr(unsafe.Pointer(&origin)))
	if ret2 == 0 {
		origin = point{0, 0}
	}
	clientLeft := int(origin.X)
	clientTop := int(origin.Y)
	clientWidth := int(cr.Right - cr.Left)
	clientHeight := int(cr.Bottom - cr.Top)

	topReserve := 80
	bottomReserve := 32
	sideMargin := 4

	x := clientLeft + sideMargin
	y := clientTop + topReserve
	w := clientWidth - sideMargin*2
	h := clientHeight - topReserve - bottomReserve
	if w > width {
		w = width
	}
	if h > height {
		h = height
	}
	log.Writef("[RDP] backend positioning: client=(%d,%d %dx%d) rdp=(x=%d y=%d w=%d h=%d)",
		clientLeft, clientTop, clientWidth, clientHeight, x, y, w, h)

	s.shown = true
	procSetWindowPos.Call(s.hwnd, HWND_TOPMOST,
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		SWP_SHOWWINDOW|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)

	// Init tracking baseline
	s.refRdpX = x
	s.refRdpY = y
	s.trackX = x
	s.trackY = y
	var wr rect
	if ret, _, _ := procGetWindowRect.Call(s.parentHwnd, uintptr(unsafe.Pointer(&wr))); ret != 0 {
		s.refParentX = int(wr.Left)
		s.refParentY = int(wr.Top)
	}
	log.Writef("[RDP] backend position done, shown=%v hwnd=0x%x", s.shown, s.hwnd)
}

func (s *RDPSession) SetPosition(x, y, w, h int) {
	s.mu.Lock()
	hwnd := s.hwnd
	if hwnd == 0 {
		log.Writef("[RDP-SetPos] SKIP hwnd=0")
		s.mu.Unlock()
		return
	}
	wasHidden := !s.shown
	s.shown = true
	log.Writef("[RDP-SetPos] FROM FRONTEND x=%d y=%d w=%d h=%d wasHidden=%v", x, y, w, h, wasHidden)
	// Update tracking baseline from frontend's authoritative DOM-based position
	var wr rect
	if ret, _, _ := procGetWindowRect.Call(s.parentHwnd, uintptr(unsafe.Pointer(&wr))); ret != 0 {
		s.refParentX = int(wr.Left)
		s.refParentY = int(wr.Top)
	}
	s.refRdpX = x
	s.refRdpY = y
	s.trackX = x
	s.trackY = y
	s.mu.Unlock()

	// Include SWP_SHOWWINDOW in case window was hidden (e.g. after tab switch)
	procSetWindowPos.Call(hwnd, HWND_TOPMOST,
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		SWP_SHOWWINDOW|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)

		// Restart tracking if it was stopped by Hide().
		// Owned-window relationship handles z-order automatically.
		if wasHidden {
			s.startTracking()
		}
}

func (s *RDPSession) Show() {
	s.mu.Lock()
	if s.shown {
		s.mu.Unlock()
		return
	}
	hwnd := s.hwnd
	tX := s.trackX
	tY := s.trackY
	s.shown = true
	s.mu.Unlock()
	log.Writef("[RDP-Show] trackX=%d trackY=%d hwnd=0x%x", tX, tY, hwnd)
	if hwnd != 0 {
		procSetWindowPos.Call(hwnd, HWND_TOPMOST,
			uintptr(tX), uintptr(tY),
			0, 0,
			SWP_SHOWWINDOW|SWP_NOSIZE|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)
	}
	s.startTracking()
}

func (s *RDPSession) Hide() {
	log.Writef("[RDP-Hide] called")
	s.mu.Lock()
	if !s.shown {
		s.mu.Unlock()
		return
	}
	hwnd := s.hwnd
	s.shown = false
	s.mu.Unlock()
	s.stopTracking()
	if hwnd != 0 {
		procSetWindowPos.Call(hwnd, 0,
			32000, 32000,
			0, 0,
			SWP_NOSIZE|SWP_NOACTIVATE|SWP_NOZORDER|SWP_ASYNCWINDOWPOS)
	}
}

// trackParentMove checks if the parent window has moved and repositions the RDP window
// to follow. Also detects minimize/restore to hide/re-show RDP. Called from the tracking
// goroutine every ~16ms.
// trackParentMove checks the parent window state and hides/shows RDP accordingly.
// Position tracking during drag/resize is handled by the frontend via RDPHide/RDPSetPosition.
// This goroutine only handles minimize/restore detection which the frontend cannot see.
func (s *RDPSession) trackParentMove() {
	s.mu.Lock()
	hwnd := s.hwnd
	if hwnd == 0 || !s.shown {
		s.mu.Unlock()
		return
	}
	parent := s.parentHwnd
	tX := s.trackX
	tY := s.trackY
	s.mu.Unlock()

	isMin, _, _ := procIsIconic.Call(parent)
	if isMin != 0 {
		if !s.wasMinimized {
			s.wasMinimized = true
			log.Writef("[RDP-track] parent minimized, hiding RDP")
			procSetWindowPos.Call(hwnd, 0, 32000, 32000, 0, 0,
				SWP_NOSIZE|SWP_NOACTIVATE|SWP_NOZORDER|SWP_ASYNCWINDOWPOS)
		}
		return
	}
	if s.wasMinimized {
		s.wasMinimized = false
		log.Writef("[RDP-track] parent restored, showing RDP at (%d,%d)", tX, tY)
		procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(tX), uintptr(tY), 0, 0,
			SWP_SHOWWINDOW|SWP_NOSIZE|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)
		return
	}

	// Z-order maintenance: dynamic TOPMOST based on foreground
	fg, _, _ := procGetForegroundWindow.Call()
	fgRoot, _, _ := procGetAncestor.Call(fg, GA_ROOT)
	if fgRoot == parent {
		procSetWindowPos.Call(hwnd, HWND_TOPMOST, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)
	} else {
		procSetWindowPos.Call(hwnd, HWND_NOTOPMOST, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_ASYNCWINDOWPOS)
	}
}

func (s *RDPSession) startTracking() {
	s.mu.Lock()
	if s.trackStop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.trackStop = stop
	s.trackGen++
	gen := s.trackGen
	s.mu.Unlock()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Writef("[RDP-track] PANIC in tracking goroutine: %v", r)
			}
		}()
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				if s.trackGen != gen {
					s.mu.Unlock()
					return // stale goroutine, new tracking has started
				}
				s.mu.Unlock()
				s.trackParentMove()
			}
		}
	}()
}

func (s *RDPSession) stopTracking() {
	log.Writef("[RDP-track] stopTracking called")
	s.mu.Lock()
	if s.trackStop != nil {
		close(s.trackStop)
		s.trackStop = nil
	}
	s.mu.Unlock()
}

func (s *RDPSession) Disconnect() error {
	s.stopTracking()
	s.mu.Lock()
	hwnd := s.hwnd
	s.hwnd = 0
	s.mu.Unlock()

	if hwnd != 0 {
		procPostMessageW.Call(hwnd, WM_CLOSE, 0, 0)
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *RDPSession) Resize(cols, rows int) error {
	s.mu.Lock()
	if s.rdp != nil {
		s.rdp.PutProperty("DesktopWidth", cols)
		s.rdp.PutProperty("DesktopHeight", rows)
	}
	hwnd := s.hwnd
	shown := s.shown
	s.mu.Unlock()
	if hwnd != 0 && shown {
		procSetWindowPos.Call(hwnd, HWND_TOPMOST, 0, 0,
			uintptr(cols), uintptr(rows),
			SWP_NOACTIVATE)
	}
	return nil
}

func (s *RDPSession) Write(_ []byte) error {
	return nil
}

func (s *RDPSession) IsConnected() bool {
	return s.Status() == StatusConnected
}
