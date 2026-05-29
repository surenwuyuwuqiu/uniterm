<div align="center">
  <img src="build/appicon.png" alt="uniTerm" width="128" height="128" />
  <h1>uniTerm</h1>
  <p>A modern cross-platform terminal emulator with a built-in autonomous AI Agent — capable of independently planning and executing multi-turn shell commands, like Claude Code for your terminal.</p>
</div>

[简体中文](README_zh-CN.md)

## Features

### AI Assistant

Autonomous AI Agent that works like Claude Code — independently plans and executes multi-turn shell commands directly in your terminal.

- **Autonomous Multi-Turn Execution** — The AI Agent can plan, execute, observe results, and iterate across multiple rounds of shell commands without manual intervention.
- **LLM Integration** — Sidebar chat with Anthropic-compatible API, supporting Claude and other compliant models.
- **Flexible Execution Modes** — Confirm all, confirm dangerous only, or bypass — you control how much oversight the AI Agent needs.
- **Persistent Conversations** — Chat history is saved per session, so conversations survive app restarts.
- **Terminal Integration** — AI commands execute directly in the active terminal tab, with optional pinning to a specific tab or following your active one. Collaborate side-by-side in split panes, each with its own terminal context.
- **Smart Completion** — While typing in SSH terminals, get real-time suggestions from your command history and AI-powered command rewrites.

### Full-Featured Terminal

- **SSH Client** — Connect via password or private key authentication. Multi-tab management with 5 color schemes, 6 monospace fonts, adjustable font size and scrollback, configurable selection behavior and right-click actions.
- **SFTP File Manager** — Dual-pane browser for local and remote files. Upload, download, drag-and-drop, delete, rename, and more. Transfers tracked per tab with pause, resume, and cancel support.
- **Local Terminal** — Full-featured local terminal supporting PowerShell, CMD, Git Bash, and other shells with the same font, color, and behavior settings as SSH sessions.
- **RDP** — Support Windows Remote Desktop connections.
- **VNC** — Support VNC connections.
- **Database Client** — Connect to MySQL, PostgreSQL, and rqlite databases. Execute SQL queries, browse table structures, and edit data rows inline — all from a unified interface.

### Customization

- **Connection Manager** — Save, search, edit, group, and duplicate server connections. Drag-and-drop organization, multi-select or range-select for batch connect, batch delete, and more.
- **Workspace & Split Panes** — Merge terminal tabs into a workspace with horizontal or vertical splits. Drag panel edges or title bars to resize and rearrange freely.
- **Cloud Sync** — Build your own private sync repository via GitHub, GitLab, or Gitee. All configurations (connections, AI model keys, app settings) are encrypted with AES-256-GCM before syncing. Supports automatic sync, conflict resolution, master password changes, and repository management.
- **Themes** — Dark, Deep Blue, and Light themes with automatic system theme detection.
- **Internationalization** — Simplified Chinese and English UI, built with a clean i18n architecture ready for more languages.
- **Cross-Platform** — Built on Wails v2, runs natively on Windows, macOS, and Linux.

## Quick Workflows

### SSH Connection

1. Click **New Connection** in the Connection Manager
2. Fill in host, port, and authentication (password or private key)
3. Click **Connect** to open an SSH terminal session

### AI Assistant

1. Go to Settings and configure your **AI provider** (API endpoint, model, and key)
2. Open a terminal tab (SSH or local)
3. Open the AI sidebar chat — type your task, and the AI Agent executes commands directly in your terminal

### SFTP File Transfer

1. In the Connection Manager, **right-click** an SSH connection
2. Select **Connect SFTP**
3. Browse, upload, download, and drag-and-drop files in the dual-pane file manager

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop Framework | Wails v2 |
| Backend | Go |
| Frontend | Vue 3 + Pinia + Element Plus |
| Terminal | xterm.js |
| AI Protocol | Anthropic Messages API |

## Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Node.js](https://nodejs.org/) 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2

### Platform-specific

- **Windows**: WebView2 runtime (included in Windows 10+)
- **macOS**: Xcode Command Line Tools
- **Linux**: `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`

## Getting Started

```bash
git clone https://github.com/ys-ll/uniterm.git
cd uniTerm
cd frontend && npm install && cd ..
wails dev                   # Development
wails build                 # Production build
```

## Project Structure

```
uniTerm/
├── main.go                       # Entry point
├── app.go                        # Wails bindings, LLM API proxy, SFTP API
├── backend/
│   ├── session/                  # SSH/SFTP/database session management
│   ├── database/                 # SQL execution, schema introspection, DSN builders
│   ├── store/                    # Persistent config (connections, AI, settings)
│   └── log/                      # File-based logging
├── frontend/
│   └── src/
│       ├── components/           # Vue components
│       ├── composables/          # Terminal composables
│       ├── stores/               # Pinia stores
│       ├── services/             # AI agent loop, LLM client
│       ├── i18n/                 # Translations
│       └── types/                # TypeScript type definitions
└── wails.json
```

## License

Apache 2.0
