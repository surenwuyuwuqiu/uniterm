# Changelog

## v2026.06.08-alpha

- **new** SPICE remote desktop protocol support.
- **new** Panel duplicate, rename, drag image preview, and title synchronization.
- **new** Drag active terminal tab to adjacent tab with workspace merge.
- **improve** SSH keepalive changed from global request to session channel request (`keepalive@openssh.com`), matching OpenSSH `ServerAliveInterval` behavior; interval adjusted from 30s to 60s, max failures from 2 to 3.
- **improve** On Windows, prefer Git Bash over WSL and fix WSL bash argument passing.
- **improve** Suggestion popup position fixed in multi-panel workspace; SFTP scroll behavior improved.
- **bugfix** Fixed terminal size not updating after SSH reconnect. New sessions default to 80×24 PTY; now forces a `SessionResize` with the current terminal dimensions when reconnected, so apps like vim/k9s display at the correct size.
- **bugfix** Fixed `clear` command destroying scrollback history. Replaces ED2 (clear screen) with newline scrolling + home, pushing viewport content into scrollback before clearing.
- **bugfix** Fixed text highlighting disappearing after tab switch. Restored history now applies `highlight()` based on the current `highlightEnabled` setting.
- **bugfix** Fixed copy-on-select overwriting clipboard when switching panels or returning from another app. Copy now only triggers when the mouse selection actually started inside the same terminal.
- **bugfix** Fixed dropping panel/tab onto empty tab bar area not working in certain layouts.

## v2026.06.02-alpha

- **new** Telnet and Mosh connection protocol support. Telnet provides IAC negotiation (binary mode, terminal type, window size); Mosh uses UDP-based SSP protocol for low-latency mobile connections.
- **new** Terminal text highlighting. Automatically highlights timestamps, IP addresses, URLs, file paths, keywords (ERROR/WARN/INFO), quoted strings, numbers, and punctuation in terminal output. Toggle in settings; lines containing ESC are skipped to avoid TUI interference.
- **new** xterm.js Unicode11 addon for correct emoji and wide character rendering (e.g. k9s dog icon).
- **improve** Merged tab bar into titlebar as a single row, saving ~40px vertical space. All buttons icon-only, new connection + local terminal merged into `+` dropdown, window controls styled consistently.
- **improve** New/Edit connection dialog restructured into two-level category selection (Terminal / Remote Desktop / Database) with radio-button toggle for protocol sub-type.
- **improve** Unified control sizing across the entire UI: 28px height, 12px font, consistent border-radius, background, and border colors for all controls (el-input, el-button, el-select, el-radio-button, el-switch, el-checkbox, etc.).
- **improve** Smart completion UX fixes: popup flips above/below intelligently to avoid covering input; mouse hover only activates after movement to prevent accidental selection; password (hidden) input is not saved to history and does not trigger suggestions.
- **improve** Terminal tabs restyled as buttons with accent border + background for active state, AI lock + active effects combined.
- **improve** AI sidebar defaults to new session on restart; empty sessions are not saved; max 15 sessions retained.
- **bugfix** Fixed terminal history capture logic. Scans visible buffer area (bottom to top) instead of relying on cursorY, which is unreliable after buffer scrolling.
- **bugfix** Fixed race condition where suggestion popup remained open after Enter (debounce timer cancellation + empty token check).
- **bugfix** Fixed WebView2 conflict causing input failure when opening multiple processes on Windows 11. UserDataFolder now uses a per-process PID-isolated path.
- **bugfix** Fixed garbled text on tab switch. xterm.js OSC color queries sent via onData were echoed back by the server as scrambled text; added OSC filtering to resolve.

## v2026.05.29-alpha

- **new** Terminal smart completion. Real-time popup with history command and AI rewrite suggestions while typing in SSH terminals. Settings page adds a command history management section with search, select-all, and batch delete.
- **new** Server monitor. Real-time monitoring for connected servers. Supports performance metrics (CPU/memory/disk/network), process list with details, listening ports, disk usage and mount info, network interfaces with bond/bridge detection.
- **new** SSH post-login script execution. Configure a script to run automatically after SSH connection; supports idle detection to avoid executing during manual interaction.
- **new** SSH keepalive to prevent idle disconnect. Sends periodic keepalive packets and shows a reconnect prompt when the connection drops.
- **improve** Sidebar splitter visibility and terminal scrollbar contrast improved for easier interaction.
- **bugfix** Fixed MySQL multi-database race condition in database query capabilities.
- **bugfix** Unified connection type label rendering between grouped and ungrouped views in the sidebar.

## v2026.05.27-alpha

- **new** Database connection and query. Supports MySQL, PostgreSQL, and rqlite. Provides SQL query execution, table schema browsing, CRUD on data rows, and tree navigation of databases/tables.
- **new** Terminal search bar. Press Ctrl+F to open the search bar; highlights matches and counts results using @xterm/addon-search.
- **new** Connection sidebar and AI sidebar visibility states are now persisted to localStorage, restoring expand/collapse state after restart.
- **improve** Scrollbar width increased from 5px to 8px for easier grabbing.
- **improve** AI sidebar maximize button now shows a shrink icon when expanded for clearer indication.
- **improve** Added transparent padding around window edges so the terminal can still be resized by dragging even when it fills the edge.
- **improve** Workspace tabs now display the LayoutDashboard icon.
- **bugfix** Fixed garbled text appearing when switching tabs. Session buffer truncation could fall in the middle of escape sequences (DA2, OSC color queries, etc.), leaving fragments without the \x1b prefix that xterm.js rendered as garbage. Fix: scan for \x1b before the first \n to determine a safe restart boundary.

## v2026.05.25-alpha

- **bugfix** Editing and saving a connection caused passwords for other connections to be lost. Fixed an issue in the Go backend Save method where underlying array sharing inadvertently cleared passwords/APIKeys.

## v2026.05.24-alpha

- **new** Cloud config sync. Build a private cloud sync repository based on GitHub, GitLab, or Gitee private repos. All configurations (connections, AI model keys, app settings) are encrypted with AES-256-GCM before being saved remotely. Supports auto-sync, conflict resolution, master password change, and repo binding management.

## v2026.05.23-alpha

- **new** VNC remote desktop. Connect to VNC servers (TigerVNC, TightVNC, QEMU, etc.) via noVNC with a built-in WebSocket↔TCP proxy bridge. DOM remains alive across tab switches for zero-latency screen recovery. Supports auto-resize toggle and bidirectional clipboard sharing (Ctrl+Shift+V to paste local clipboard).
- **new** Local terminal support. Open a local shell directly (Windows PowerShell/CMD, macOS/Linux bash/zsh) without an SSH connection.
- **new** AI Shell awareness. AI can detect the current terminal's shell type to generate more accurate commands.
- **improve** Connection list now shows ports. Sidebar entries changed from `user@host` to `user@host:port`, displaying `host:port` when the username is empty.
- **improve** Username field is hidden when creating a new VNC connection (VNC authentication only requires a password).
- **improve** Icon unification.
- **bugfix** RDP windows now correctly hide/show when menus, dialogs, or window dragging occur, avoiding obstruction.
- **bugfix** Settings page state persistence issue fixed.
- **bugfix** Password input visibility toggle fixed.
- **bugfix** Windows 11 security dialog suppression fixed.

## v2026.05.22-alpha

- **new** RDP remote desktop. Connect to Windows Remote Desktop via the Microsoft RDP ActiveX control. Native security dialogs are fully suppressed; the RDP window is seamlessly embedded into uniTerm tabs and smoothly follows window dragging and resizing.
- **new** AI sidebar maximize button. Expands the AI assistant panel to fill the entire main area; click again to restore original width.
- **new** AI message copy functionality. Added copy buttons next to tool-call IN/OUT expand boxes with a checkmark feedback on click. Added a "Copy as Markdown" button in the top-right corner of messages that expands on hover.
- **new** Broadcast input. Added a broadcast button to the workspace panel header to send keyboard input to all terminals in the workspace simultaneously.
- **new** About section in Settings page showing the app version.
- **new** Added "Write-operation confirmation" level to AI command execution confirmation. On top of the existing three levels (Off / Dangerous only / All), added a "Dangerous + Write" option that also requires confirmation for write operations like rm and mv, filling the granularity gap between Dangerous and All.
- **new** Added "New Connection" to the connection group context menu; the group is automatically preselected when creating.
- **improve** Unified sidebar selection state. Merged `selectedId` and `multiSelectedIds` into a single `selectedIds` set for consistent highlight logic; fixed the issue where previous selection was not cleared on right-click.
- **improve** Right-clicking a connection now automatically deselects others and selects only the current one if it wasn't already in the selection.
- **improve** Tab bar improvements: supports horizontal scrolling with mouse wheel, and dropdown selections auto-scroll into view.
- **improve** System prompt message role changed from `assistant` to `tool` to avoid polluting the LLM conversation context.
- **improve** AI session storage migrated from localStorage to Go backend file storage.
- **improve** README redesign: added Chinese version, landing page, categorized feature showcase, and screenshot carousel.
- **bugfix** Fixed garbled text when switching tabs (strip incomplete escape sequences).
- **bugfix** Fixed panel active state not syncing terminal focus in workspaces.
- **bugfix** Fixed edit button incorrectly grayed out when a single connection is selected.
- **bugfix** Fixed "New Connection..." virtual item not auto-selected when search yields no matches.
- **bugfix** Fixed tab title displaying with host suffix (should show connection name only).

## v2026.05.17-alpha

- **new** Connection grouping. Supports creating, renaming, and deleting connection groups. Connections can be collapsed by group; drag-and-drop to change group assignment; ungrouped connections are automatically placed in a "(No Group)" virtual group.
- **new** Batch connection selection and actions. Supports Ctrl+click multi-select and Shift+click range select. Context menu supports batch connect, batch SFTP connect, batch copy, and batch delete. Enter key also supports batch open.
- **new** A "New Connection..." virtual item appears at the bottom of the list while typing in the search bar. Double-click or press Enter to prefill the host field and open the new connection dialog.
- **new** Confirmation prompt before deleting connections, showing the number of items to be deleted.
- **change** Light theme redesigned with a Windows-style neutral gray palette, using blue accent color instead of the previous warm yellow.
- **improve** AI session names, workspace names, AI lock button tooltips, and more now support Chinese/English internationalization.
- **bugfix** Fixed AI assistant settings button navigating to General Settings instead of AI Settings.
- **bugfix** Fixed right-click menu appearing on the ".." parent directory row in the SFTP file list.
- **bugfix** Fixed selected count being one less than actual when multi-selecting connections.

## v2026.05.16-alpha

- **new** SFTP file manager with dual-pane browsing of local and remote files. Supports upload, download, rename, delete, and permission changes. Transfer tasks are tracked independently per tab.

## v2026.05.15-alpha

- **new** Workspace panel system. Merge multiple terminal tabs into a workspace, displayed side-by-side or stacked within the same window. Drag panel headers to freely adjust panel position and size; drag tabs to panel edges to auto-create new splits.
- **new** Custom window title bar adapted for Windows and macOS platform window controls.
- **new** Terminal http/https links are auto-detected and underlined. Hover to see tooltip; Ctrl+click to open in default browser.
- **new** Windows installer now detects running processes and prompts to close the running application before installation.
- **improve** Dark theme contrast improved; both Default and Deep Blue color schemes have clearer background layers and more readable text.
- **bugfix** Selected text auto-copy to clipboard now works even if the mouse is released outside the terminal.
- **bugfix** Fixed errors caused by missing tool responses in AI multi-turn conversations.
- **bugfix** Fixed blank areas in child panels and unmovable split bars when panel splitting occurs.
- **bugfix** Fixed abnormal terminal content display when window or panel size changes.
- **bugfix** Fixed Ctrl+scroll wheel causing unexpected full-window zoom.

## v2026.05.13-alpha

- **new** Free panel splitting. Split windows left/right or top/bottom to open multiple terminals simultaneously. Drag borders to resize panels; tabs can also be dragged across panels.
- **new** AI window lock button. The "AI" button on each terminal tab locks AI to that terminal. After locking, AI command execution targets only that terminal; switching to other tabs won't send commands to the wrong place.
- **improve** Increased AI max consecutive interactions per conversation from 10 to 20. A "Continue" button appears when the limit is reached to continue the conversation; added history message length control to prevent sluggishness from overly long context.
- **new** Connection list supports up/down arrow key navigation; press Enter to connect directly without double-clicking.
- **change** Windows releases now provide installer (.exe) only, no additional zip archives. macOS releases now provide DMG only, no additional zip archives.
- **improve** Terminal default scrollback lines reduced from 5000 to 2500 to reduce memory usage.
- **bugfix** Fixed terminal content not resizing when the window or panel shrinks.
