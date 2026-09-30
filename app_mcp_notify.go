//go:build !darwin && !windows

package main

import (
	"github.com/ys-ll/uniterm/backend/mcp"
)

// notifyMCPApproval surfaces a pending MCP approval to the OS when the app
// window is not focused. Platform-specific (app_mcp_notify_darwin.go and
// friends); this non-darwin/windows base is a no-op — Linux can grow a
// notify-send implementation on demand.
func (a *App) notifyMCPApproval(req mcp.ApprovalRequest) {}

// raiseMainWindow brings the main window to the front for the notification
// click-through. Base no-op; platforms with a real impl wire the window.
func (a *App) raiseMainWindow() {}
