//go:build windows

package main

import (
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	GWLP_WNDPROC     = ^uintptr(3)
	WM_ENTERSIZEMOVE = 0x0231
	WM_EXITSIZEMOVE  = 0x0232
	WM_SYSCOMMAND    = 0x0112
	WM_SIZE          = 0x0005
	SC_MAXIMIZE      = 0xF030
	SC_MINIMIZE      = 0xF020
	SC_RESTORE       = 0xF120
)

func (a *App) findMainWindow() uintptr {
	pid := windows.GetCurrentProcessId()
	var result uintptr

	user32 := windows.NewLazySystemDLL("user32.dll")
	procEnumWindows := user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId := user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW := user32.NewProc("GetWindowTextW")

	cb := windows.NewCallback(func(hwnd windows.HWND, lParam uintptr) uintptr {
		var wndPid uint32
		procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wndPid)))
		if wndPid != pid {
			return 1 // continue
		}
		// Verify it has our window title so we don't pick up invisible helper windows.
		buf := make([]uint16, 256)
		procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 255)
		if windows.UTF16ToString(buf) == "uniTerm" {
			result = uintptr(hwnd)
			return 0 // stop
		}
		return 1 // continue
	})
	procEnumWindows.Call(cb, 0)
	return result
}

func (a *App) subclassMainWindow() {
	if a.mainHwnd == 0 {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtrW := user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW := user32.NewProc("CallWindowProcW")

	cb := windows.NewCallback(func(hwnd windows.HWND, msg uint32, wparam, lparam uintptr) uintptr {
		switch msg {
		case WM_ENTERSIZEMOVE:
			a.inSizeMove = true
			runtime.EventsEmit(a.ctx, "rdp:move-resize-start")
		case WM_EXITSIZEMOVE:
			a.inSizeMove = false
			runtime.EventsEmit(a.ctx, "rdp:move-resize-end")
		case WM_SYSCOMMAND:
			switch wparam {
			case SC_MAXIMIZE, SC_MINIMIZE, SC_RESTORE:
				runtime.EventsEmit(a.ctx, "rdp:move-resize-start")
			}
		case WM_SIZE:
			if !a.inSizeMove {
				runtime.EventsEmit(a.ctx, "rdp:move-resize-end")
			}
		}
		ret, _, _ := procCallWindowProcW.Call(a.originalWndProc, uintptr(hwnd), uintptr(msg), wparam, lparam)
		return ret
	})
	a.wndProcCb = cb

	orig, _, _ := procSetWindowLongPtrW.Call(a.mainHwnd, GWLP_WNDPROC, cb)
	a.originalWndProc = orig
}

func (a *App) unsubclassMainWindow() {
	if a.originalWndProc == 0 || a.mainHwnd == 0 {
		return
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtrW := user32.NewProc("SetWindowLongPtrW")
	procSetWindowLongPtrW.Call(a.mainHwnd, GWLP_WNDPROC, a.originalWndProc)
	a.originalWndProc = 0
}
