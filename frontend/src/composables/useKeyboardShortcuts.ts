import type { KeyboardSettings, KeyBinding, ShortcutAction } from '../types/settings'

type ActionHandlers = Record<ShortcutAction, () => void>

function bindingKey(b: KeyBinding): string {
  if (!b.key) return ''
  let k = ''
  if (b.ctrl) k += 'ctrl+'
  if (b.meta) k += 'meta+'
  if (b.shift) k += 'shift+'
  if (b.alt) k += 'alt+'
  k += b.key.toLowerCase()
  return k
}

function normalize(e: KeyboardEvent): string {
  const parts: string[] = []
  if (e.ctrlKey) parts.push('ctrl')
  if (e.metaKey) parts.push('meta')
  if (e.shiftKey) parts.push('shift')
  if (e.altKey) parts.push('alt')
  parts.push(e.key.toLowerCase())
  return parts.join('+')
}

// Module-level state: key combo → action handler
const shortcutMap = new Map<string, () => void>()
// Reverse lookup: action → key combo (for display / dedup)
const actionKeyMap = new Map<ShortcutAction, string>()

export function loadKeybindings(bindings: KeyboardSettings, handlers: ActionHandlers) {
  shortcutMap.clear()
  actionKeyMap.clear()
  for (const [action, b] of Object.entries(bindings) as [ShortcutAction, KeyBinding][]) {
    const key = bindingKey(b)
    if (!key) continue
    const handler = handlers[action]
    if (handler) {
      shortcutMap.set(key, handler)
      if (!b.meta && b.ctrl) {
        shortcutMap.set(key.replace(/^ctrl\+/, 'meta+'), handler)
      }
      actionKeyMap.set(action, key)
    }
  }
}

export function getActionKey(action: ShortcutAction): string {
  return actionKeyMap.get(action) || ''
}

function fire(e: KeyboardEvent, normalized: string): boolean {
  const handler = shortcutMap.get(normalized)
  if (!handler) return false
  e.preventDefault()
  e.stopPropagation()
  handler()
  return true
}

export function onGlobalKeydown(e: KeyboardEvent) {
  fire(e, normalize(e))
}

export function onTerminalKey(e: KeyboardEvent): boolean {
  const normalized = normalize(e)
  if (shortcutMap.has(normalized)) {
    fire(e, normalized)
    return false
  }
  return true
}

let registered = false

export function installGlobalListener() {
  if (registered) return
  registered = true
  window.addEventListener('keydown', onGlobalKeydown, true)
}

export function uninstallGlobalListener() {
  registered = false
  window.removeEventListener('keydown', onGlobalKeydown, true)
}
