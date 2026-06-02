const ANSI_RESET = '\x1b[0m'
// Match ANSI escape sequences: CSI (ESC [ ... letter) and OSC (ESC ] ... BEL/ST)
const ANSI_RE = /(\x1b\[[\x20-\x3F]*[\x40-\x7E]|\x1b[\]PX^_][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[\x20-\x2F][\x30-\x7E]|\x1b[\x30-\x7E])/g

// Split text into segments: alternating [plain, CSI, plain, CSI, ...]
function segmentText(text: string): { text: string; isCSI: boolean }[] {
  const segments: { text: string; isCSI: boolean }[] = []
  let lastEnd = 0
  ANSI_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = ANSI_RE.exec(text)) !== null) {
    if (m.index > lastEnd) {
      segments.push({ text: text.slice(lastEnd, m.index), isCSI: false })
    }
    segments.push({ text: m[0], isCSI: true })
    lastEnd = m.index + m[0].length
  }
  if (lastEnd < text.length) {
    segments.push({ text: text.slice(lastEnd), isCSI: false })
  }
  if (segments.length === 0) {
    segments.push({ text, isCSI: false })
  }
  return segments
}

// Patterns ordered longest-first
const PATTERNS: { regex: RegExp; sgr: string }[] = [
  { regex: /https?:\/\/[^\s\x1b]+/gi, sgr: '\x1b[4;38;5;39m' },
  { regex: /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b/g, sgr: '\x1b[38;5;82m' },
  { regex: /(?:\/|~\/)[\w.\/-]+\.\w+\b/g, sgr: '\x1b[38;5;177m' },
  { regex: /\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?\b/g, sgr: '\x1b[38;5;39m' },
  { regex: /\b\d{2}:\d{2}:\d{2}\b/g, sgr: '\x1b[38;5;39m' },
  { regex: /"(?:[^"\\]|\\.){2,}"|'(?:[^'\\]|\\.){2,}'/g, sgr: '\x1b[38;5;215m' },
  { regex: /\b(?:ERROR|FAIL(?:ED|URE)?|CRITICAL|FATAL)\b/g, sgr: '\x1b[38;5;203m' },
  { regex: /\bWARN(?:ING)?\b/g, sgr: '\x1b[38;5;221m' },
  { regex: /\b(?:INFO|SUCCESS|OK)\b/g, sgr: '\x1b[38;5;75m' },
  { regex: /[{}()\[\]|*=<>]/g, sgr: '\x1b[38;5;147m' },
  { regex: /\b\d+\b/g, sgr: '\x1b[38;5;145m' },
]

function highlightPlainText(text: string): string {
  const segments = segmentText(text)
  let result = ''
  for (const seg of segments) {
    if (seg.isCSI) {
      result += seg.text
    } else {
      type MatchEntry = { start: number; end: number; sgr: string }
      const allMatches: MatchEntry[] = []
      for (const { regex, sgr } of PATTERNS) {
        regex.lastIndex = 0
        let m: RegExpExecArray | null
        while ((m = regex.exec(seg.text)) !== null) {
          allMatches.push({ start: m.index, end: m.index + m[0].length, sgr })
          if (allMatches.length > 200) break  // too many matches, skip
        }
        if (allMatches.length > 200) break
      }
      if (allMatches.length > 200) {
        result += seg.text  // pass through unchanged
        continue
      }
      allMatches.sort((a, b) => a.start - b.start || b.end - a.end)
      const filtered: MatchEntry[] = []
      for (const match of allMatches) {
        const last = filtered[filtered.length - 1]
        if (!last || match.start >= last.end) {
          filtered.push(match)
        }
      }
      let highlighted = seg.text
      for (let i = filtered.length - 1; i >= 0; i--) {
        const { start, end, sgr } = filtered[i]
        highlighted = highlighted.slice(0, start) + sgr + highlighted.slice(start, end) + ANSI_RESET + highlighted.slice(end)
      }
      result += highlighted
    }
  }
  return result
}

export function highlight(text: string): string {
  // Process line by line to avoid cross-line regex matches
  const lines = text.split(/(\r?\n)/)
  let result = ''
  for (const line of lines) {
    if (line === '\r\n' || line === '\n' || line === '\r') {
      result += line
    } else if (line) {
      // Skip any line with ANSI escape codes — TUI/colored output must not be modified
      if (line.indexOf('\x1b') !== -1) {
        result += line
      } else {
        result += highlightPlainText(line)
      }
    }
  }
  return result
}
