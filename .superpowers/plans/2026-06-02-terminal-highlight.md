# Terminal Text Highlight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add syntax highlighting to terminal output by injecting ANSI color codes around matched patterns (timestamps, IPs, keywords, URLs, paths, strings, numbers).

**Architecture:** Create `useHighlight.ts` composable with a `highlight(text: string): string` function that applies regex patterns with xterm-256 SGR color codes. Call it before `terminal.write()` in `BaseTerminal.vue`'s `session:data` handler.

**Tech Stack:** TypeScript, xterm.js 5.5, xterm-256 ANSI color palette

---

### Task 1: Create useHighlight.ts composable

**Files:**
- Create: `frontend/src/composables/useHighlight.ts`

- [ ] **Step 1: Write the highlight composable**

```typescript
// Regex patterns ordered longest-first so URLs/IPs match before plain numbers
const PATTERNS: { regex: RegExp; sgr: string }[] = [
  // URL (must match before IP)
  { regex: /https?:\/\/[^\s\x1b]+/gi, sgr: '\x1b[4;38;5;39m' },
  // IP address with optional port
  { regex: /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b/g, sgr: '\x1b[38;5;82m' },
  // File path (absolute or relative, with extension)
  { regex: /(?:\/|~\/)[\w.\/-]+\.\w+\b/g, sgr: '\x1b[38;5;177m' },
  // ISO timestamp
  { regex: /\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?\b/g, sgr: '\x1b[38;5;39m' },
  // HH:MM:SS time
  { regex: /\b\d{2}:\d{2}:\d{2}\b/g, sgr: '\x1b[38;5;39m' },
  // Quoted strings
  { regex: /"[^"]*"|'[^']*'/g, sgr: '\x1b[38;5;215m' },
  // ERROR / FAIL / CRITICAL / FATAL
  { regex: /\b(?:ERROR|FAIL(?:ED|URE)?|CRITICAL|FATAL)\b/g, sgr: '\x1b[38;5;203m' },
  // WARN / WARNING
  { regex: /\bWARN(?:ING)?\b/g, sgr: '\x1b[38;5;221m' },
  // INFO / SUCCESS / OK
  { regex: /\b(?:INFO|SUCCESS|OK)\b/g, sgr: '\x1b[38;5;75m' },
  // Plain number (last, so IPs/timestamps are already handled)
  { regex: /\b\d+\b/g, sgr: '\x1b[38;5;145m' },
]

const ANSI_RESET = '\x1b[0m'

// Track open SGR segments to avoid double-wrapping
function hasExistingSGR(text: string, start: number, end: number): boolean {
  // Check if there's an ANSI SGR already wrapping or adjacent to this position
  const before = text.substring(Math.max(0, start - 20), start)
  // If an SGR code was recently opened without reset, skip highlighting
  return /\x1b\[[34][0-9;]*m/.test(before) && !/\x1b\[0m/.test(before.slice(-10))
}

export function highlight(text: string): string {
  let result = text
  for (const { regex, sgr } of PATTERNS) {
    regex.lastIndex = 0
    const matches: { start: number; end: number }[] = []
    let m: RegExpExecArray | null
    while ((m = regex.exec(result)) !== null) {
      matches.push({ start: m.index, end: m.index + m[0].length })
    }
    // Apply highlights in reverse order to preserve indices
    for (let i = matches.length - 1; i >= 0; i--) {
      const { start, end } = matches[i]
      if (hasExistingSGR(result, start, end)) continue
      result = result.slice(0, start) + sgr + result.slice(start, end) + ANSI_RESET + result.slice(end)
    }
  }
  return result
}
```

- [ ] **Step 2: Verify no TypeScript errors**

Run: `cd frontend && npx vue-tsc --noEmit src/composables/useHighlight.ts 2>&1 || true`

---

### Task 2: Integrate highlight into BaseTerminal.vue

**Files:**
- Modify: `frontend/src/components/BaseTerminal.vue`

- [ ] **Step 1: Import useHighlight**

Add import near the top of the `<script setup>` block (around line 68):

```typescript
import { highlight } from '../composables/useHighlight'
```

- [ ] **Step 2: Apply highlight before terminal.write() in session:data**

Replace line 492:
```typescript
      terminal.write(payload.data)
```
With:
```typescript
      terminal.write(highlight(payload.data))
```

- [ ] **Step 3: Build frontend**

```bash
cd frontend && npm run build
```

- [ ] **Step 4: Verify full build**

```bash
cd .. && wails build -platform windows/amd64
```

---

### Task 3: Start wails dev and verify

- [ ] **Step 1: Start wails dev**

```bash
cd frontend && rm -rf dist node_modules/.vite && npm run build && cd .. && wails dev
```

- [ ] **Step 2: Verify visually**

Open a terminal session and check:
1. `ls -la` — numbers highlighted in gray
2. `echo "hello world"` — quoted string in orange
3. `ping 192.168.1.1` — IP in green
4. Log output with timestamps — timestamps in cyan
5. `cat /var/log/syslog` or any log with ERROR/WARN — keywords in red/yellow
6. URLs in terminal output — cyan underlined
7. File paths like `/etc/hosts` — purple
8. Verify existing terminal colors (e.g. `ls --color`) still work correctly
