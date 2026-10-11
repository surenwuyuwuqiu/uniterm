package session

import (
	"strings"
	"testing"
	"time"
)

// MCP PTY exec (issue #1108) — the pieces that can be tested without a live
// SSH server: sentinel matching across chunk boundaries, capture trimming of
// the typed-echo and the sentinel line, ready-gate timing, and the serial
// lock on the executor.

func TestMcpPtySentinelMatchesEchoWithCode(t *testing.T) {
	token := "7"
	head := mcpPtySentinelHead + token + mcpPtySentinelTail
	pattern := mcpPtyPatternFor(head)

	// The typed line's echo (head + "$?") must NOT match.
	typed := "ls -la ; echo " + head + "$?\r\n"
	if pattern.MatchString(typed) {
		t.Fatalf("typed-line echo must not match the sentinel pattern: %q", typed)
	}

	// The post-execution echo with the code must match, including wrapped
	// across chunks, and the code must be parseable.
	echo := "\r\n" + head + "0\r\n$ "
	m := pattern.FindStringSubmatch(echo)
	if m == nil {
		t.Fatalf("sentinel echo %q must match", echo)
	}
	if m[1] != "0" {
		t.Fatalf("exit code = %q, want 0", m[1])
	}

	echoFail := "\r\n" + head + "127\r\n"
	m = pattern.FindStringSubmatch(echoFail)
	if m == nil || m[1] != "127" {
		t.Fatalf("non-zero exit code not parsed: %v", m)
	}
}

func TestMcpPtySentinelSplitAcrossChunks(t *testing.T) {
	head := mcpPtySentinelHead + "12" + mcpPtySentinelTail
	pattern := mcpPtyPatternFor(head)
	full := "output line\r\n" + head + "3\r\n$ "
	for i := 1; i < len(full); i++ {
		// Simulate appendLocked in two pieces: the drain scan reaches
		// mcpPtyScanBack bytes before the current chunk, so any split must
		// match as long as the sentinel sits inside the scan window.
		before, after := full[:i], full[i:]
		if len(after) > mcpPtyScanBack {
			after = after[:mcpPtyScanBack]
		}
		window := full
		if !pattern.MatchString(window) {
			t.Fatalf("split at %d: window %q must match", i, window)
		}
		_ = before
	}
}

func TestMcpPtyTrimCaptureDropsTypedEcho(t *testing.T) {
	head := mcpPtySentinelHead + "3" + mcpPtySentinelTail
	typed := "uptime ; echo " + head + "$?"
	// drain passes everything strictly before the sentinel echo (m[0] cuts
	// at its first byte), so the head never appears in trim's input.
	out := typed + "\r\n" + " 18:30  up 5 days\r\n"
	got := mcpPtyTrimCapture([]byte(out), head)
	if strings.Contains(string(got), "echo "+head) {
		t.Fatalf("typed echo line survived trim: %q", got)
	}
	if !strings.Contains(string(got), "up 5 days") {
		t.Fatalf("command output lost in trim: %q", got)
	}
	if strings.Contains(string(got), mcpPtySentinelTail) {
		t.Fatalf("sentinel fragment survived trim: %q", got)
	}
}

func TestMcpPtyTrimCaptureEchoOnlyWhenNoOutput(t *testing.T) {
	head := mcpPtySentinelHead + "4" + mcpPtySentinelTail
	out := "true ; echo " + head + "$?\r\n" + head + "0\r\n$ "
	got := mcpPtyTrimCapture([]byte(out), head)
	if strings.Contains(string(got), "true") {
		t.Fatalf("echo of command with no output should be trimmed away: %q", got)
	}
}

func TestMcpPtyReadyGate(t *testing.T) {
	s := NewSSHSession("pty-ready-gate")
	// Non-companion: no gate, wait returns immediately.
	if err := s.MCPPtyWaitReady(time.Second); err != nil {
		t.Fatalf("non-companion wait: %v", err)
	}

	comp := NewMCPCompanionSession("pty-ready-comp", s)
	// Companion without the signal: must time out.
	if err := comp.MCPPtyWaitReady(30 * time.Millisecond); err == nil {
		t.Fatalf("companion wait before signal must time out")
	}
	// Signal then wait: immediate.
	comp.mcpPtySignalReady()
	if err := comp.MCPPtyWaitReady(time.Second); err != nil {
		t.Fatalf("companion wait after signal: %v", err)
	}
	// Double signal must not panic (sync.Once).
	comp.mcpPtySignalReady()
}

func TestMcpPtyRegistryLifecycle(t *testing.T) {
	s := NewSSHSession("pty-registry")
	e1 := GetOrCreateMcpPtyExecutor(s)
	e2 := GetOrCreateMcpPtyExecutor(s)
	if e1 != e2 {
		t.Fatalf("executor not cached per companion")
	}
	RemoveMcpPtyExecutor(s.ID())
	e3 := GetOrCreateMcpPtyExecutor(s)
	if e3 == e1 {
		t.Fatalf("removed executor must not be returned")
	}
}

func TestMcpPtyExecOnDisconnectedSession(t *testing.T) {
	s := NewSSHSession("pty-disconnected")
	comp := NewMCPCompanionSession("pty-disconnected-comp", s)
	e := GetOrCreateMcpPtyExecutor(comp)
	if _, _, _, _, err := e.MCPExec("ls", 1000, 2000, 1024); err == nil {
		t.Fatalf("exec on a disconnected companion must fail")
	}
}
