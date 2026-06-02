# Terminal Text Highlight Design

## Overview

为终端输出增加语法高亮，自动识别时间戳、IP 地址、ERROR/WARN 关键词、字符串、URL、文件路径、数字等模式，用 ANSI 颜色转义码包裹以改变文字颜色。

## Implementation: ANSI Injection

拦截 `terminal.write()` 的数据流，用正则匹配文本模式，包裹 ANSI 颜色码后写入终端。不依赖 xterm.js 内部 API。

```
服务端数据流 → HighlightFilter.process(data) → terminal.write(highlightedData)
```

## Color Scheme (xterm 256-color, cross-theme safe)

| Pattern | Color | SGR Code |
|---------|-------|----------|
| Timestamp | Cyan | `38;5;39` |
| IP Address | Green | `38;5;82` |
| ERROR/FAIL/CRITICAL | Red | `38;5;203` |
| WARN/WARNING | Yellow | `38;5;221` |
| INFO/SUCCESS/OK | Blue | `38;5;75` |
| Quoted String | Orange | `38;5;215` |
| File Path | Purple | `38;5;177` |
| URL | Cyan + Underline | `4;38;5;39` |
| Number | Gray | `38;5;145` |

## Regex Rules (ordered, longest first to avoid short-match stealing)

1. **URL** — full URL pattern
2. **IP:Port** — `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b`
3. **File Path** — `(?:/~?/[\w.-]+)+\.?\w*`
4. **Timestamp** — ISO 8601 + time
5. **Quoted String** — `"[^"]*"|'[^']*'`
6. **ERROR** — `\b(?:ERROR|FAIL|CRITICAL|FATAL)\b`
7. **WARN** — `\b(?:WARN(?:ING)?)\b`
8. **INFO** — `\b(?:INFO|SUCCESS|OK)\b`
9. **Number** — `\b\d+\b`

Patterns that already have ANSI codes (from remote shell coloring) are preserved and not double-wrapped.

## Files

| File | Change |
|------|--------|
| `frontend/src/composables/useHighlight.ts` | **New** — highlight filter logic |
| `frontend/src/components/BaseTerminal.vue` | Inject filter in `session:data` write path |

## Non-Goals

- User-configurable colors (fixed scheme, v1)
- Clickable highlights (color only)
- Highlighting user input/typing (output only)
