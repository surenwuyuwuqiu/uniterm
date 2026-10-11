package session

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MCP exec via companion PTY (issue #1108): menu-driven jump hosts terminate
// the SSH transport on the jump host, so a plain exec channel runs commands
// there, not on the target behind the menu. For connections that opt in
// (MCPExecViaPty), exec_command instead rides the companion PTY session —
// a visible tab cloned from the user's session, login automation replayed —
// so commands land exactly where the user's interactive shell is.
//
// Semantics differ from the exec-channel path and are documented in the tool
// description: commands run serially on ONE shared PTY (cd/export persist
// between calls; no concurrent commands), stdout and stderr arrive merged,
// and the exit code is parsed from a trailing sentinel echo — best-effort,
// because a PTY has no exit-status channel. When the sentinel never arrives
// (non-POSIX shells, an interactive command waiting for input) the command
// times out at the hard limit and returns what was captured, exit state
// unknown.

// The sentinel is echoed after the command:
//
//	__UNITERM_MCP__<token>__CODE__<exitcode>
//
// The token is unique per call, so stale output from a previous command can
// never terminate the next one. Both halves are shell-safe literal text.
const (
	mcpPtySentinelHead = "__UNITERM_MCP__"
	mcpPtySentinelTail = "__CODE__"

	// mcpPtySetupTimeout bounds the wait for the companion's post-login
	// automation before the first command.
	mcpPtySetupTimeout = 90 * time.Second

	// mcpPtyScanBack is how far before the current chunk the sentinel scan
	// reaches, so a sentinel straddling two chunks is still matched.
	mcpPtyScanBack = 64
)

// mcpPtySeq makes sentinel tokens unique across calls.
var mcpPtySeq atomic.Uint64

// mcpPtyRegistry maps companion session ids to their serial executors and
// PTY command ids back to their owning companion, mirroring mcpRegistry's
// global command lookup.
var mcpPtyRegistry = struct {
	mu        sync.Mutex
	executors map[string]*McpPtyExecutor
	byCommand map[string]string // commandID → companion session id
}{
	executors: make(map[string]*McpPtyExecutor),
	byCommand:  make(map[string]string),
}

// McpPtyExecutor runs MCP commands serially on one companion PTY session.
// One instance per companion; MCPExec holds the mutex for a command's whole
// runtime, so commands can never interleave on the shared PTY.
type McpPtyExecutor struct {
	sess *SSHSession
	mu   sync.Mutex
}

// GetOrCreateMcpPtyExecutor returns the executor bound to a companion
// session, creating it on first use.
func GetOrCreateMcpPtyExecutor(s *SSHSession) *McpPtyExecutor {
	mcpPtyRegistry.mu.Lock()
	defer mcpPtyRegistry.mu.Unlock()
	e, ok := mcpPtyRegistry.executors[s.ID()]
	if !ok {
		e = &McpPtyExecutor{sess: s}
		mcpPtyRegistry.executors[s.ID()] = e
	}
	return e
}

// RemoveMcpPtyExecutor drops a dead companion's registration so a recreated
// companion can bind cleanly.
func RemoveMcpPtyExecutor(sessionID string) {
	mcpPtyRegistry.mu.Lock()
	defer mcpPtyRegistry.mu.Unlock()
	delete(mcpPtyRegistry.executors, sessionID)
}

// LookupMcpPtyCommand resolves a commandId to its companion session id, for
// ownership attribution (polling and interrupt must reach the companion's
// executor, not whichever session the global registry scan trips over).
func LookupMcpPtyCommand(commandID string) (sessionID string, ok bool) {
	mcpPtyRegistry.mu.Lock()
	defer mcpPtyRegistry.mu.Unlock()
	sid, ok := mcpPtyRegistry.byCommand[commandID]
	return sid, ok
}

// mcpPtyPatternFor builds the sentinel-match regex for one call's head.
func mcpPtyPatternFor(head string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(head) + `(\-?\d+)`)
}

// MCPExec implements the mcp.SSHExecutor contract on the companion PTY:
// send command + sentinel, capture PTY output until the sentinel echo or the
// hard timeout. Like the exec-channel path, the sync budget only selects the
// return shape — the call still leaves after the command retires, because
// the capture tap must live exactly as long as the command.
func (e *McpPtyExecutor) MCPExec(cmd string, syncTimeoutMs, hardTimeoutMs, maxOutput int) (stdout, stderr []byte, exitCode int, commandID string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.sess.Status() != StatusConnected {
		return nil, nil, -1, "", fmt.Errorf("companion PTY session %s is not connected", e.sess.ID())
	}

	// First use: wait for the login automation (menu navigation) to finish —
	// commands sent before the last expect step landed would race the menu.
	if rerr := e.sess.MCPPtyWaitReady(mcpPtySetupTimeout); rerr != nil {
		return nil, nil, -1, "", fmt.Errorf("companion login automation not ready: %w", rerr)
	}

	token := strconv.FormatUint(mcpPtySeq.Add(1), 10)
	head := mcpPtySentinelHead + token + mcpPtySentinelTail
	// The sentinel echo arrives as `<head><exitcode>` followed by CR/LF and
	// the next prompt. The typed line's echo carries head+"$?" — `$` is not
	// a digit, so the pre-execution echo can never match.
	pattern := mcpPtyPatternFor(head)

	c := &mcpCommand{
		id:    "pty-" + token,
		done:  make(chan struct{}),
		state: "running",
	}
	mcpRegisterCommand(c)
	mcpPtyRegistry.mu.Lock()
	mcpPtyRegistry.byCommand[c.id] = e.sess.ID()
	mcpPtyRegistry.mu.Unlock()
	// Defer order (LIFO): untap first, then retire, then the byCommand
	// delete — a still-running async command keeps its tap and registry
	// entries until it retires, so polling keeps working; retire waits for
	// the runner goroutine so the tap never outlives the capture.
	defer func() {
		mcpPtyRegistry.mu.Lock()
		delete(mcpPtyRegistry.byCommand, c.id)
		mcpPtyRegistry.mu.Unlock()
		mcpRetireCommand(c)
	}()

	// finish terminates the command exactly once, from whichever of the
	// sentinel match / hard timeout / session teardown fires first.
	var doneOnce sync.Once
	finish := func(state string, code int, ferr error) {
		doneOnce.Do(func() {
			c.mu.Lock()
			if c.state == "running" {
				c.exitCode = code
				c.state = state
				c.err = ferr
			}
			c.mu.Unlock()
			close(c.done)
		})
	}

	// Session teardown (companion tab closed / transport down) ends the
	// capture instead of hanging until the hard timeout.
	quitCh := e.sess.quit
	go func() {
		select {
		case <-quitCh:
			finish("session_closed", -1, fmt.Errorf("companion session closed"))
		case <-c.done:
		}
	}()

	hardTimer := time.AfterFunc(time.Duration(hardTimeoutMs)*time.Millisecond, func() {
		finish("killed", -1, fmt.Errorf("command exceeded hard timeout (%dms) on the PTY companion — output returned, exit state unknown", hardTimeoutMs))
	})
	defer hardTimer.Stop()

	// Capture taps the emit path (decoded, pre-frontend chunks, exactly once
	// per chunk). The tap appends and scans inline on the readLoop goroutine:
	// no intermediate queue means no dropped chunks and no lost sentinel.
	untap := e.sess.TapOnData(func(data []byte) {
		c.mu.Lock()
		c.appendLocked(&c.stdout, data)
		out := c.stdout
		c.mu.Unlock()
		// Scan from just before this chunk so a straddling sentinel still
		// matches; appendLocked may have trimmed the head, so derive the
		// offset from the post-append length.
		scanFrom := len(out) - len(data) - mcpPtyScanBack
		if scanFrom < 0 {
			scanFrom = 0
		}
		m := pattern.FindSubmatchIndex(out[scanFrom:])
		if m == nil {
			return
		}
		code, _ := strconv.Atoi(string(out[scanFrom+m[2] : scanFrom+m[3]]))
		cleaned := mcpPtyTrimCapture(out[:scanFrom+m[0]], head)
		c.mu.Lock()
		c.stdout = cleaned
		c.mu.Unlock()
		finish("exited", code, nil)
	})
	defer untap()

	// `; echo` — not `&&` — so the sentinel fires even when the command fails
	// or is not found; that matches the exec channel's non-zero reporting.
	line := cmd + " ; echo " + head + "$?"
	if werr := e.sess.Write([]byte(line + "\r")); werr != nil {
		untap()
		finish("error", -1, fmt.Errorf("write to companion PTY: %w", werr))
		out, errOut, code, _, rerr := c.snapshot()
		return truncateOutput(out, maxOutput), truncateOutput(errOut, maxOutput), code, "", rerr
	}
	syncTimeout := time.Duration(syncTimeoutMs) * time.Millisecond
	select {
	case <-c.done:
		out, errOut, code, _, rerr := c.snapshot()
		return truncateOutput(out, maxOutput), truncateOutput(errOut, maxOutput), code, "", rerr
	case <-time.After(syncTimeout):
		// Still running: hand back the poll handle. The deferred untap/retire
		// keep the tap installed and the executor's mutex held until the
		// command retires, so polling keeps receiving output meanwhile.
		return nil, nil, -1, c.id, nil
	}
}

// mcpPtyTrimCapture cleans a finished command's capture: drop everything from
// the sentinel echo on (the next prompt follows it), then the leading echo of
// the typed command line when it can be located by its distinctive tail
// (head + "$?" — text the real output cannot contain because the token is
// per-call). Wrapped echoes that split the tail are left alone; the agent just
// sees the extra line.
func mcpPtyTrimCapture(out []byte, head string) []byte {
	out = []byte(strings.TrimRight(string(out), "\r\n"))
	typedTail := head + "$?"
	p := strings.Index(string(out), typedTail)
	if p >= 0 {
		rest := string(out[p+len(typedTail):])
		if i := strings.IndexAny(rest, "\r\n"); i >= 0 {
			out = []byte(rest[i:])
			out = []byte(strings.TrimLeft(string(out), "\r\n"))
		} else {
			// Only the echo was captured: no output at all.
			out = nil
		}
	}
	return out
}

// MCPLatestOutput reuses the shared command registry's poll contract.
func (e *McpPtyExecutor) MCPLatestOutput(commandID string, offset, max int) (stdout, stderr []byte, exitCode int, state string, err error) {
	return e.sess.MCPLatestOutput(commandID, offset, max)
}

// MCPInterrupt delivers a Ctrl+C into the PTY — the only signal path a PTY
// has. SIGTERM/SIGKILL degrade to it (the exec channel's ssh signal
// facility does not exist here), which the tool description calls out.
func (e *McpPtyExecutor) MCPInterrupt(commandID string, signal string) error {
	c, ok := mcpLookupCommand(commandID)
	if !ok {
		return fmt.Errorf("command %s not found or expired", commandID)
	}
	if state := func() string { c.mu.Lock(); defer c.mu.Unlock(); return c.state }(); state != "running" {
		return fmt.Errorf("command %s is not running (state=%s)", commandID, state)
	}
	if werr := e.sess.Write([]byte("\x03")); werr != nil {
		return fmt.Errorf("interrupt write failed: %w", werr)
	}
	return nil
}
