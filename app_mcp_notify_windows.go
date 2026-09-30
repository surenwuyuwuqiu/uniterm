//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	w32 "github.com/wailsapp/wails/v3/pkg/w32"
	"github.com/ys-ll/uniterm/backend/log"
	"github.com/ys-ll/uniterm/backend/mcp"
)

// notifyMCPApproval (Windows), with the app in the background:
//  1. taskbar flash via the Wails window's Flash() (button blinks until
//     activation),
//  2. a tray balloon (NOTIFYICONDATA NIF_INFO on the main HWND) carrying
//     the client + command preview — Windows 10+ renders it as a toast,
//  3. best-effort SetForegroundWindow raise (the documented foreground
//     exception doesn't apply — the requesting agent is another process —
//     so the flash + balloon carry the visibility; the raise lands when
//     the OS permits it).
func (a *App) notifyMCPApproval(req mcp.ApprovalRequest) {
	if a.window != nil {
		a.window.Flash(true)
	}

	title := "uniTerm · MCP 审批请求"
	body := fmt.Sprintf("%s 请求执行命令,请在 uniTerm 中确认", req.Client)
	if req.Command == "" {
		body = fmt.Sprintf("%s 请求建立新连接,请在 uniTerm 中确认", req.Client)
	} else if len(req.Command) <= 60 {
		body = fmt.Sprintf("%s: %s", req.Client, req.Command)
	} else {
		body = fmt.Sprintf("%s: %s…", req.Client, req.Command[:57])
	}
	if err := a.showWindowsBalloon(title, body); err != nil {
		log.Writef("mcp: balloon failed: %v", err)
	}
}

// showWindowsBalloon posts a Shell_NotifyIcon balloon on the main window's
// HWND. The icon entry is added (NIM_ADD) before the balloon and removed
// right after, so no stray tray icon remains. utf16 conversion truncates to
// the struct's fixed arrays (64/256 wchars).
func (a *App) showWindowsBalloon(title, body string) error {
	hwnd := windows.HWND(a.mainHwnd)
	if hwnd == 0 {
		hwnd = windows.HWND(a.findMainWindow())
	}
	if hwnd == 0 {
		return fmt.Errorf("no main window handle")
	}

	nid := w32.NOTIFYICONDATA{
		HWnd:   w32.HWND(hwnd),
		UID:    0x4d4350, // "MCP" — distinct from the real tray's uid
		UFlags: w32.NIF_MESSAGE | w32.NIF_ICON,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))

	// Register the notification icon the balloon hangs off.
	if !w32.ShellNotifyIcon(w32.NIM_ADD, &nid) {
		return fmt.Errorf("NIM_ADD failed")
	}

	// Balloon payload.
	copy(nid.SzInfoTitle[:], utf16FromString(title, len(nid.SzInfoTitle)))
	copy(nid.SzInfo[:], utf16FromString(body, len(nid.SzInfo)))
	nid.UFlags = w32.NIF_INFO
	nid.DwInfoFlags = w32.NIIF_WARNING | w32.NIIF_RESPECT_QUIET_TIME
	w32.ShellNotifyIcon(w32.NIM_MODIFY, &nid)

	// Remove the temporary icon once the toast has had time to spawn.
	go func() {
		// The balloon text lives in the struct we are about to reuse; the
		// delete call only needs identity (hwnd+uid), but keep a copy of the
		// struct so the async delete races nothing.
		del := w32.NOTIFYICONDATA{HWnd: nid.HWnd, UID: nid.UID, UFlags: 0}
		del.CbSize = uint32(unsafe.Sizeof(del))
		w32.ShellNotifyIcon(w32.NIM_DELETE, &del)
	}()
	return nil
}

// utf16FromString converts s into a fixed-length null-terminated UTF-16
// buffer (truncating to n-1 wchars for the terminator).
func utf16FromString(s string, n int) []uint16 {
	u := windows.StringToUTF16(s)
	if len(u) >= n {
		u = u[:n-1]
	}
	out := make([]uint16, n)
	copy(out, u)
	return out
}

// raiseMainWindow (Windows): best-effort foreground raise via the existing
// relaunch path.
func (a *App) raiseMainWindow() {
	a.bringMainWindowToFront()
}
