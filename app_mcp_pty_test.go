package main

import (
	"strings"
	"testing"

	"github.com/ys-ll/uniterm/backend/mcp"
	"github.com/ys-ll/uniterm/backend/session"
)

// MCP PTY exec routing (issue #1108): mcpSessionExec must route opt-in
// connections to the companion PTY executor and leave opt-out connections on
// the exec channel, without spinning companions up on the read-only poll
// path.

func newMcpPtyTestApp() *App {
	a := &App{
		mcpFileSessions:    map[string]string{},
		mcpTransferWaiters: map[string]chan mcpTransferOutcome{},
		mcpPtySessions:     map[string]string{},
	}
	return a
}

func newTestSSHSessionWith(id string, viaPty bool) *session.SSHSession {
	s := session.NewSSHSession(id)
	s.SetTestConnectionConfig(session.ConnectionConfig{
		ID:            id,
		Type:          "ssh",
		Name:          "jump-conn",
		Host:          "1.2.3.4",
		MCPExecViaPty: viaPty,
	})
	return s
}

func TestMcpSessionExecOptOutKeepsExecChannel(t *testing.T) {
	a := newMcpPtyTestApp()
	a.sessionManager = session.NewSessionManager()
	s := newTestSSHSessionWith("s-plain", false)
	a.sessionManager.Add(s)

	exec, ok := a.mcpSessionExec("s-plain")
	if !ok {
		t.Fatalf("opt-out session must resolve")
	}
	if _, isPty := exec.(*session.McpPtyExecutor); isPty {
		t.Fatalf("opt-out session must keep the exec-channel executor")
	}
	if exec != mcp.SSHExecutor(s) {
		t.Fatalf("opt-out executor must be the session itself")
	}
}

func TestMcpSessionExecOptInRoutesToCompanion(t *testing.T) {
	a := newMcpPtyTestApp()
	a.sessionManager = session.NewSessionManager()
	s := newTestSSHSessionWith("s-jump", true)
	a.sessionManager.Add(s)

	// Status is Disconnected: companion creation must fail cleanly (not
	// panic, not silently return the session's own exec channel).
	exec, ok := a.mcpSessionExec("s-jump")
	if ok {
		t.Fatalf("opt-in session with dead source must not resolve")
	}
	_ = exec
}

func TestMcpCommandOwnerReadOnlyNeverCreatesCompanion(t *testing.T) {
	a := newMcpPtyTestApp()
	a.sessionManager = session.NewSessionManager()
	s := newTestSSHSessionWith("s-owner", true)
	a.sessionManager.Add(s)

	// Polling an unknown command on an opt-in connection with no companion
	// yet must NOT create one.
	if _, _, ok := a.mcpCommandOwner("no-such-command"); ok {
		t.Fatalf("unknown command must not resolve")
	}
	a.mcpPtyMu.Lock()
	_, created := a.mcpPtySessions["s-owner"]
	a.mcpPtyMu.Unlock()
	if created {
		t.Fatalf("read-only poll path must not create a companion session")
	}
}

func TestMcpFileToolsRefusePtyOptInConnections(t *testing.T) {
	a := newMcpPtyTestApp()
	a.sessionManager = session.NewSessionManager()
	s := newTestSSHSessionWith("s-file", true)
	a.sessionManager.Add(s)

	_, err := a.mcpFileSession("s-file")
	if err == nil {
		t.Fatalf("file tools must refuse mcpExecViaPty connections")
	}
	if !strings.Contains(err.Error(), "jump host") {
		t.Fatalf("refusal must explain the jump-host reason, got: %v", err)
	}
}
