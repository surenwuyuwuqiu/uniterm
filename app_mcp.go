package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ys-ll/uniterm/backend/log"
	"github.com/ys-ll/uniterm/backend/mcp"
	"github.com/ys-ll/uniterm/backend/session"
	"github.com/ys-ll/uniterm/backend/store"
)

// MCP server wiring: settings persistence (mcp.json), token management, the
// approval bridge (Wails events ↔ dialog component), audit logging, and the
// Env glue between backend/mcp and the session/connection stores.

// MCPTokenRecord is one persisted token (hash only).
type MCPTokenRecord struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// MCPTokensFile is the on-disk shape of mcp.json.
type MCPTokensFile struct {
	Tokens []MCPTokenRecord `json:"tokens"`
}

// MCPStatus reports runtime state to the frontend.
type MCPStatus struct {
	Running bool `json:"running"`
	Port    int  `json:"port"`
}

const mcpTokensFileName = "mcp.json"
const mcpAuditFileName = "mcp-audit.log"

func (a *App) mcpTokensPath() string {
	return filepath.Join(a.dataDir, mcpTokensFileName)
}

// mcpServer is lazily created once stores are ready.
func (a *App) ensureMCPServer() *mcp.Server {
	a.mcpOnce.Do(func() {
		env := mcp.Env{
			Sessions:        a.mcpSessionExec,
			Commands:        a.mcpCommandOwner,
			ListSessions:    a.mcpListSessions,
			ListConnections: a.mcpListConnections,
			Connect:         a.mcpConnect,
			Approve:         a.mcpApprove,
			Audit:           a.mcpAudit,
			ToolsEnabled:    a.mcpToolsEnabled,
			Policy:          a.mcpPolicy,
		}
		a.mcpServer = mcp.NewServer(env)
	})
	return a.mcpServer
}

// StartMCP brings the MCP endpoint up per settings and installs the active
// token set. Called from initStores (auto-start) and from SaveSettings when
// the user toggles the switch.
func (a *App) StartMCP() error {
	if a.settingsStore == nil {
		return fmt.Errorf("settings store not initialized")
	}
	srv := a.ensureMCPServer()
	settings, err := a.settingsStore.Load()
	if err != nil {
		return err
	}
	cfg := settings.MCP
	if cfg == nil || !cfg.Enabled {
		srv.Stop()
		return nil
	}
	srv.SetTokens(a.mcpLoadTokens())
	port := mcp.DefaultPort
	if cfg.Port > 0 {
		port = cfg.Port
	}
	return srv.Start(port)
}

// MCPStatus_ reports endpoint state to the settings UI.
func (a *App) GetMCPStatus() MCPStatus {
	if a.mcpServer == nil {
		return MCPStatus{}
	}
	return MCPStatus{Running: a.mcpServer.Running(), Port: a.mcpServer.Port()}
}

// GenerateMCPToken creates a new named token, persists its hash, returns the
// plaintext exactly once.
func (a *App) GenerateMCPToken(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("token name is required")
	}
	token, hash := mcp.GenerateToken()
	if err := a.mcpSaveToken(MCPTokenRecord{Name: name, Hash: hash}); err != nil {
		return "", err
	}
	return token, nil
}

// RevokeMCPToken removes one token by name.
func (a *App) RevokeMCPToken(name string) error {
	data, err := a.mcpReadTokens()
	if err != nil {
		return err
	}
	kept := data.Tokens[:0]
	for _, t := range data.Tokens {
		if t.Name != name {
			kept = append(kept, t)
		}
	}
	data.Tokens = kept
	return a.mcpWriteTokens(data)
}

// ListMCPTokens returns token names (no hashes).
func (a *App) ListMCPTokens() ([]string, error) {
	data, err := a.mcpReadTokens()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(data.Tokens))
	for _, t := range data.Tokens {
		names = append(names, t.Name)
	}
	return names, nil
}

func (a *App) mcpSaveToken(rec MCPTokenRecord) error {
	data, err := a.mcpReadTokens()
	if err != nil {
		return err
	}
	// Replace a token with the same name (regenerate = rotate).
	for i, t := range data.Tokens {
		if t.Name == rec.Name {
			data.Tokens[i] = rec
			return a.mcpWriteTokens(data)
		}
	}
	data.Tokens = append(data.Tokens, rec)
	return a.mcpWriteTokens(data)
}

func (a *App) mcpReadTokens() (MCPTokensFile, error) {
	data, err := os.ReadFile(a.mcpTokensPath())
	if err != nil {
		if os.IsNotExist(err) {
			return MCPTokensFile{Tokens: []MCPTokenRecord{}}, nil
		}
		return MCPTokensFile{}, err
	}
	var f MCPTokensFile
	if err := json.Unmarshal(data, &f); err != nil {
		return MCPTokensFile{}, err
	}
	if f.Tokens == nil {
		f.Tokens = []MCPTokenRecord{}
	}
	return f, nil
}

func (a *App) mcpWriteTokens(f MCPTokensFile) error {
	buf, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(a.mcpTokensPath(), buf, 0600); err != nil {
		return err
	}
	// Hot-reload the live server's token set.
	if a.mcpServer != nil {
		a.mcpServer.SetTokens(a.mcpLoadTokens())
	}
	return nil
}

// mcpLoadTokens maps the persisted records into the server's live map.
func (a *App) mcpLoadTokens() map[string]mcp.TokenInfo {
	data, err := a.mcpReadTokens()
	if err != nil {
		log.Writef("mcp: read tokens: %v", err)
		return map[string]mcp.TokenInfo{}
	}
	out := make(map[string]mcp.TokenInfo, len(data.Tokens))
	for _, t := range data.Tokens {
		out[t.Hash] = mcp.TokenInfo{Name: t.Name}
	}
	return out
}

// ── Env glue ─────────────────────────────────────────────────────

func (a *App) mcpSessionExec(sessionID string) (mcp.SSHExecutor, bool) {
	if a.sessionManager == nil {
		return nil, false
	}
	s, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, false
	}
	ssh, ok := s.(*session.SSHSession)
	if !ok {
		return nil, false
	}
	return ssh, true
}

func (a *App) mcpCommandOwner(commandID string) (string, mcp.SSHExecutor, bool) {
	// Commands are registered globally in the session package; resolve by
	// scanning live SSH sessions (tens at most).
	if a.sessionManager == nil {
		return "", nil, false
	}
	for _, s := range a.sessionManager.List() {
		if exec, ok := a.mcpSessionExec(s.ID); ok {
			if _, _, _, state, _ := exec.MCPLatestOutput(commandID, 0, 1); state != "unknown" {
				return s.ID, exec, true
			}
		}
	}
	return "", nil, false
}

func (a *App) mcpListSessions() []mcp.SessionSummary {
	if a.sessionManager == nil {
		return []mcp.SessionSummary{}
	}
	out := make([]mcp.SessionSummary, 0)
	for _, s := range a.sessionManager.List() {
		if s.Type != "ssh" {
			continue
		}
		sum := mcp.SessionSummary{
			ID:     s.ID,
			Type:   s.Type,
			Title:  s.Title,
			Status: string(s.Status),
		}
		if cwd := session.GetSessionCwd(s.ID); cwd != "" {
			sum.Cwd = cwd
		}
		out = append(out, sum)
	}
	return out
}

func (a *App) mcpListConnections() []mcp.ConnectionSummary {
	if a.connectionStore == nil {
		return []mcp.ConnectionSummary{}
	}
	data, err := a.connectionStore.Load()
	if err != nil {
		return []mcp.ConnectionSummary{}
	}
	out := make([]mcp.ConnectionSummary, 0, len(data.Connections))
	for _, c := range data.Connections {
		if c.Type != "ssh" {
			continue
		}
		out = append(out, mcp.ConnectionSummary{
			ID:   c.ID,
			Name: c.Name,
			Type: c.Type,
			Host: c.Host,
			Port: c.Port,
			User: c.User,
		})
	}
	return out
}

// mcpConnect opens a new visible SSH session for a saved connection, reusing
// the exact credential-resolution path the frontend uses.
func (a *App) mcpConnect(connectionID string) (string, error) {
	if a.connectionStore == nil {
		return "", fmt.Errorf("connection store not initialized")
	}
	data, err := a.connectionStore.Load()
	if err != nil {
		return "", err
	}
	var config session.ConnectionConfig
	found := false
	for _, c := range data.Connections {
		if c.ID == connectionID && c.Type == "ssh" {
			config = c
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("ssh connection %s not found", connectionID)
	}

	// Same resolution chain as CreateSession (app_terminal.go).
	if config.Password == "" && config.ID != "" {
		if pw, err := a.connectionStore.EnsurePassword(config.ID); err == nil && pw != "" {
			config.Password = pw
		}
	}
	if config.AuthType == "identity" {
		mc, err := a.materializeIdentity(config)
		if err != nil {
			return "", err
		}
		config = mc
	}
	mc, err := a.materializeProxy(config)
	if err != nil {
		return "", err
	}
	config = mc

	s, err := a.sessionManager.Create("ssh", config)
	if err != nil {
		return "", err
	}
	if setter, ok := s.(interface{ SetLogIdentity(string, string) }); ok {
		setter.SetLogIdentity(config.Name, config.Host)
	}
	ssh := s.(*session.SSHSession)
	ssh.SetOnDataCallback(func(data []byte) {
		a.emit("session:data", map[string]interface{}{
			"id":   s.ID(),
			"data": string(data),
		})
	})
	ssh.SetOnBinaryCallback(func(data []byte) {
		a.emit("session:binary", map[string]interface{}{
			"id":   s.ID(),
			"data": base64.StdEncoding.EncodeToString(data),
		})
	})
	s.SetOnStatusChangeCallback(func(status session.SessionStatus) {
		payload := map[string]interface{}{
			"id":     s.ID(),
			"status": status,
		}
		if status == session.StatusConnected {
			if remoteOS := ssh.RemoteOS(); remoteOS != "" {
				payload["remoteOS"] = remoteOS
			}
		}
		a.emit("session:status", payload)
	})
	// Tell the frontend to open a tab for the new session so the user can see
	// and control it. The terminal tab will attach via its session id.
	a.emit("mcp:session-created", map[string]interface{}{
		"sessionId": s.ID(),
		"name":      config.Name,
		"host":      config.Host,
	})

	// Connect in the background; the tool call already passed the approval
	// gate, and the user sees the tab while the handshake runs.
	configCopy := config
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Writef("mcp connect panic: %v", r)
			}
		}()
		if err := s.Connect(configCopy); err != nil {
			a.failSessionConnect(s, err)
		}
	}()
	return s.ID(), nil
}

// ── Approval bridge ──────────────────────────────────────────────

// mcpApprove emits mcp:approval-request and blocks for the verdict via
// ResolveMCPApproval (frontend dialog) or the timeout.
func (a *App) mcpApprove(req mcp.ApprovalRequest) error {
	ch := make(chan mcpApprovalVerdict, 1)
	a.mcpApprovalsMu.Lock()
	a.mcpApprovals[req.ID] = ch
	a.mcpApprovalsMu.Unlock()
	defer func() {
		a.mcpApprovalsMu.Lock()
		delete(a.mcpApprovals, req.ID)
		a.mcpApprovalsMu.Unlock()
	}()

	a.emit("mcp:approval-request", map[string]interface{}{
		"id":         req.ID,
		"client":     req.Client,
		"connection": req.Connection,
		"command":    req.Command,
		"createdAt":  req.CreatedAt,
	})

	// No-window guard: if the frontend never answers (window closed), the
	// timeout below denies. Frontend dialogs auto-dismiss on timeout too.
	select {
	case verdict := <-ch:
		if !verdict.Approved {
			return fmt.Errorf("denied by user: %s", verdict.Reason)
		}
		return nil
	case <-time.After(mcp.DefaultApprovalTimeout):
		return fmt.Errorf("approval timed out after %s (user unavailable)", mcp.DefaultApprovalTimeout)
	}
}

// mcpApprovalVerdict is the frontend's answer.
type mcpApprovalVerdict struct {
	Approved bool
	Reason   string
}

// ResolveMCPApproval is the Wails binding the frontend calls when the user
// answers the approval dialog.
func (a *App) ResolveMCPApproval(requestID string, approved bool, reason string) error {
	a.mcpApprovalsMu.Lock()
	ch, ok := a.mcpApprovals[requestID]
	a.mcpApprovalsMu.Unlock()
	if !ok {
		return fmt.Errorf("no pending approval %s", requestID)
	}
	select {
	case ch <- mcpApprovalVerdict{Approved: approved, Reason: reason}:
	default:
	}
	return nil
}

// ── Audit ────────────────────────────────────────────────────────

var mcpAuditMu sync.Mutex

func (a *App) mcpAudit(entry mcp.AuditEntry) {
	if a.dataDir == "" {
		return
	}
	mcpAuditMu.Lock()
	defer mcpAuditMu.Unlock()
	f, err := os.OpenFile(filepath.Join(a.dataDir, mcpAuditFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	buf, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = f.Write(append(buf, '\n'))
}

// ── Settings-backed callbacks ────────────────────────────────────

func (a *App) mcpSettings() store.MCPSettings {
	if a.settingsStore == nil {
		return store.DefaultMCPSettings()
	}
	settings, err := a.settingsStore.Load()
	if err != nil {
		return store.DefaultMCPSettings()
	}
	if settings.MCP == nil {
		return store.DefaultMCPSettings()
	}
	return *settings.MCP
}

func (a *App) mcpToolsEnabled() mcp.ToolGroups {
	cfg := a.mcpSettings()
	return mcp.ToolGroups{
		Discovery: true,
		Exec:      cfg.Tools.Exec,
		Terminal:  cfg.Tools.Terminal,
		Files:     cfg.Tools.Files,
	}
}

func (a *App) mcpPolicy() mcp.Policy {
	return mcp.Policy(a.mcpSettings().Policy)
}
