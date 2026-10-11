# MCP Server (External AI Agents)

uniTerm can run as an MCP (Model Context Protocol) server, letting external AI agents — Claude Code, Codex, ZCode, Kimi, Gemini CLI, Trae, WorkBuddy, DeepSeek — **run commands, read output, and transfer files on the remote hosts uniTerm manages**.

The core principle: **credentials never leave uniTerm**. Agents never see a password or private key — they send requests to uniTerm, which executes them over its saved connections, with approval and audit at every step.

## How It Works

```
External agent (Claude Code / Codex / ...)
      │  MCP over Streamable HTTP + Bearer token
      ▼
uniTerm (127.0.0.1:61207/mcp)
      │  approval dialog → policy engine → audit log
      ▼
Saved SSH connections (credentials stay in the app)
```

- **Loopback-only listener** (127.0.0.1) — unreachable from the network
- **One Bearer token per client**; the plaintext is shown exactly once at creation, only a SHA-256 hash is stored
- Every call is logged to `<data-dir>/mcp-audit.log` (JSONL) with the client name, command, and exit code

## Getting Started

### 1. Enable the MCP server

Settings (`⌘,`) → **MCP Server**:

1. Turn on the "Enable MCP server" switch; the status line shows `Running · 127.0.0.1:61207`
2. Adjust as needed:
   - **Listen port** (default 61207; client configs must be regenerated after changing it)
   - **Command approval policy** (see below)

All tools are available by default; side effects are governed entirely by the approval policy.

![MCP Server Settings](/imgs/mcp_server_light.webp)

### 2. Generate a token and connect a client

1. Enter a recognizable token name (e.g. `claude-code`) → click "Generate token"
2. The setup wizard opens: **the token is shown only once** — copy it immediately
3. Switch to the tab of the client you use and copy its config snippet

::: warning One-time token display
The plaintext is never stored (the server keeps only a SHA-256 hash); it cannot be viewed again after the wizard closes. If lost, regenerate — generating with the same name rotates the token and invalidates the old one immediately, so client configs must be updated.
:::

### 3. Client configuration examples

**Claude Code** (run in any project directory):

```bash
claude mcp add --transport http uniterm http://127.0.0.1:61207/mcp \
  --header "Authorization: Bearer <your-token>"
```

**Codex** (append to `~/.codex/config.toml`):

```toml
[mcp_servers.uniterm]
url = "http://127.0.0.1:61207/mcp"
http_headers = { "Authorization" = "Bearer <your-token>" }
```

**ZCode** (merge into `mcp.servers` of `~/.zcode/cli/config.json`):

```json
{
  "mcp": {
    "servers": {
      "uniterm": {
        "type": "http",
        "url": "http://127.0.0.1:61207/mcp",
        "headers": { "Authorization": "Bearer <your-token>" }
      }
    }
  }
}
```

**Trae** (project-level `.trae/mcp.json` or user-level `~/.trae/mcp.json`):

```json
{
  "mcpServers": {
    "uniterm": {
      "url": "http://127.0.0.1:61207/mcp",
      "headers": { "Authorization": "Bearer <your-token>" }
    }
  }
}
```

**WorkBuddy** (connector dir `~/.workbuddy/connectors-marketplace/connectors/uniterm/mcp.json`, then enable in the app):

```json
{
  "mcpServers": {
    "uniterm": {
      "type": "streamableHttp",
      "url": "http://127.0.0.1:61207/mcp",
      "headers": { "Authorization": "Bearer <your-token>" }
    }
  }
}
```

**DeepSeek** (Cordis overlay snippet for dsh; save as `uniterm.cordis.yml` and start with `dsh web --patch <file>`; keep the token in the `UNITERM_MCP_TOKEN` env var):

```yaml
- id: mcp-uniterm
  name: '@deepseek-ai/dsh-mcp-client'
  config:
    serverName: uniterm
    transport: streamable-http
    url: http://127.0.0.1:61207/mcp
    headers:
      Authorization: !!js `Bearer ${process.env.UNITERM_MCP_TOKEN}`
    toolCallTimeoutMs: 130000
```

**Kimi** (run in any project directory):

```bash
kimi mcp add --transport http uniterm http://127.0.0.1:61207/mcp \
  --header "Authorization: Bearer <your-token>"
```

**Gemini** (merge into `mcpServers` of `~/.gemini/settings.json`):

```json
{
  "mcpServers": {
    "uniterm": {
      "type": "http",
      "url": "http://127.0.0.1:61207/mcp",
      "headers": { "Authorization": "Bearer <your-token>" }
    }
  }
}
```

## Using It

Once connected, just describe the task in natural language:

> Use uniterm to list my SSH connections, then run `nginx -t` on web-01 to check the config

The agent drives the whole flow itself: discover connections → open a session → execute → read results. Sessions the agent opens via `connect` appear as terminal tabs in uniTerm — **fully visible to you, and you can take over input at any time**.

## Tool Reference

| Tool | Group | Description |
|------|-------|-------------|
| `list_connections` | Discovery | List saved SSH connections (id, name, host, user). **Never returns credentials** |
| `list_sessions` | Discovery | List live sessions (id, title, status, cwd) |
| `connect` | Exec | Open a new SSH session for a saved connection (approval required) |
| `exec_command` | Exec | Run a command on a connected session: dedicated non-PTY channel, separate stdout/stderr, real exit code, never touches your interactive terminal |
| `get_command_output` | Exec | Poll output/status of long commands by command id (auto-async after 20s) |
| `interrupt_command` | Exec | Send SIGINT / SIGTERM / SIGKILL to a running command |
| `upload_file` | Files | Upload a local file to the remote host (streamed; content never passes through the model) |
| `download_file` | Files | Download a remote file (streamed) |
| `list_remote_dir` | Files | List a remote directory (capped at 500 entries) |
| `read_remote_file` | Files | Read remote file content (≤256KB, paged) |

**Typical flow**: find the target with `list_connections` / `list_sessions` → `connect` if no session exists → `exec_command` → poll long commands with `get_command_output` → move files with `upload_file` / `download_file`.

## Menu-driven jump hosts: mcpExecViaPty

Some jump hosts (bastion servers) offer no real SSH tunneling: after login they present an **interactive menu** (pick a host, type the target IP, ...) and only then drop you onto the target machine. The SSH transport of such a connection terminates on the jump host:

- `exec_command`'s exec channel lands on the **jump host** — commands never reach the target (issue #1108)
- File tools (SFTP/SCP companion sessions) likewise only see the jump host's filesystem

For such connections, turn on **MCP Exec via Terminal** in the connection's advanced settings, and `exec_command` rides a **companion terminal** instead:

1. On the first command, uniTerm opens a visible terminal tab cloned from the connection (no re-authentication; the connection's login automation / expect steps replay to get through the menu onto the target)
2. Commands run on that companion terminal's PTY — fully visible, and you can take over input at any time
3. The companion closes with the user session; if the jump host idle-logs it out, the next command recreates it

**Semantic differences** (a PTY is not an exec channel — the agent sees these through the tool description):

- stdout and stderr arrive **merged**; the exit code is parsed from a sentinel echo (best-effort on POSIX shells; non-POSIX shells or a killed session may time out with "exit state unknown")
- Commands run **serially** on a shared working directory and environment (`cd`, `export` persist between calls)
- Interactive commands (anything waiting on stdin) hang until the timeout — tell the agent to avoid them
- Interrupts are Ctrl+C only (SIGTERM/SIGKILL degrade to ^C)

**File tools refuse these connections** (they cannot reach the target behind the menu): to transfer files, use an SSH jump tunnel instead (the connection's SSH tunnel field pointing at the jump-host connection), or move data through `exec_command` (cat/base64).

::: warning Bastion-host considerations
Menu-driven bastions often **limit concurrent sessions per account** and require auditable sessions. The companion terminal is a real second login (SSH auth is skipped, but it occupies a menu session) — make sure your bastion's policy allows it. The companion tab is fully visible to the user, so auditing matches manual operation.
:::

## Approval Policy

| Policy | Behavior |
|--------|----------|
| Confirm every command (default) | Every `exec_command` / `connect` / transfer prompts for approval |
| Confirm write commands | Prompts for write-level commands and above (mv, systemctl restart, redirections, …) |
| Confirm dangerous commands | Prompts only for dangerous commands (rm -rf, mkfs, `curl \| sh`, writing /etc, …) |
| Bypass all | No dialogs — **but dangerous commands still prompt** |

The approval dialog shows: the requesting client, the target connection, and **the full command text**. You can:

- **Allow** — execute
- **Deny** — block the call
- **Deny with reason** — the reason is sent back to the agent, which can adjust its plan

::: warning Timeout = denial
If you don't respond within 110 seconds (switched away, away from the machine), the approval is auto-denied and the agent is told "user did not respond". This is deliberate: better to have the agent retry than to run a command without your knowledge.
:::

## Local Directory Restriction for Transfers

The **local side** of `upload_file` / `download_file` must be inside an allowed directory, taken from the SFTP panel's saved local path bookmarks (Settings → Storage, or paths saved in the file panel sidebar).

- Paths are resolved through symlinks — **escape attempts are rejected** (e.g. a symlink under `/tmp/allowed` pointing to `/etc` fails)
- An empty bookmark list rejects every transfer — on purpose: add the directory to bookmarks first
- Remote paths are unrestricted but governed by the approval policy

## Security Model at a Glance

| Layer | Mechanism |
|-------|-----------|
| Transport | Loopback-only listener; local processes still need a Bearer token |
| Auth | Per-client token, SHA-256 at rest, revocable, hot-reloaded |
| Authorization | Approval policy matrix (commands are risk-classified; the policy decides when to prompt) |
| Execution | Dedicated exec channel, never touches your interactive terminal; concurrency cap 8, 5-minute hard timeout per command |
| Audit | Full JSONL log (client / tool / command / exit code / approval outcome) |
| Credentials | Never leave the app — agents never see passwords, keys, or key contents |

## Troubleshooting

**`claude mcp list` shows connection failed?**
Verify the switch is on and the status line says "Running"; if you changed the port, update the URL in client configs too.

**Calls return 401?**
Wrong, revoked, or rotated token. Regenerate and update the client config.

**A command never executes?**
Most likely waiting for the approval dialog — switch back to uniTerm. It auto-denies after 110 seconds.

**exec_command says `session is not connected`?**
The agent needs to `connect` first; or open a terminal tab to that host yourself and let the agent find it via `list_sessions`.

**Transfers fail with "outside the allowed directories"?**
The local path is not in the SFTP local bookmarks. Add the directory and retry.

**Lost the token?**
It cannot be recovered (by design). Regenerate one and update client configs; the old token dies immediately.
