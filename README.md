<div align="center">
  <img src="build/appicon.png" alt="uniTerm" width="128" height="128" />
  <h1>uniTerm</h1>
  <p>A modern cross-platform terminal emulator with built-in AI assistant.</p>
</div>

[简体中文](README_zh-CN.md)

## Features

- **SSH Client** — Connect to remote servers via password or private key authentication, with multi-tab terminal session management. 5 color schemes, 6 monospace fonts, adjustable font size and scrollback history, configurable selection behavior and right-click action.
- **SFTP File Manager** — Dual-pane browser for local and remote files. Upload, download, drag-and-drop, delete, rename, and more. Transfer tasks are tracked per tab with pause, resume, and cancel support.
- **AI Assistant** — Sidebar chat with Anthropic-compatible LLMs that execute shell commands directly in the terminal. Three execution modes (confirm all, confirm dangerous, bypass) with persistent conversation history.
- **Workspace & Split Panes** — Merge multiple terminal tabs into a workspace with horizontal or vertical split layouts. Drag panel edges or title bars to resize and rearrange freely.
- **Connection Manager** — Save, search, edit, and duplicate server connections. Group and organize with drag-and-drop, multi-select or range-select for batch connect, batch delete, and more.
- **Themes & i18n** — Dark, Deep Blue, and Light themes with system auto-detect. Simplified Chinese and English UI.
- **Cross-Platform** — Built on Wails v2, runs on Windows, macOS, and Linux.

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
│   ├── session/                  # SSH/SFTP session management
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
