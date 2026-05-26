import { defineStore } from 'pinia'
import { reactive } from 'vue'
import { EventsOn } from '../../wailsjs/runtime'
import type { SessionStatus } from '../types/session'

interface SessionData {
  id: string
  status: SessionStatus
  data: string[]
}

// Module-level reactive state (shared across all store instances)
const sessionState = reactive<{
  sessions: Map<string, SessionData>
}>({
  sessions: new Map()
})

// Register event listeners once at module level
EventsOn('session:status', (payload: { id: string; status: SessionStatus }) => {
  let s = sessionState.sessions.get(payload.id)
  if (!s) {
    s = { id: payload.id, status: 'connecting', data: [] }
    sessionState.sessions.set(payload.id, s)
  }
  s.status = payload.status
})

EventsOn('session:data', (payload: { id: string; data: string }) => {
  let s = sessionState.sessions.get(payload.id)
  if (!s) {
    s = { id: payload.id, status: 'connecting', data: [] }
    sessionState.sessions.set(payload.id, s)
  }
  s.data.push(payload.data)
  if (s.data.length > 2000) {
    s.data.splice(0, s.data.length - 1000)
  }
})

export const useSessionStore = defineStore('session', () => {
  function initSession(id: string) {
    const existing = sessionState.sessions.get(id)
    if (existing) {
      existing.status = 'connecting'
    } else {
      sessionState.sessions.set(id, { id, status: 'connecting', data: [] })
    }
  }

  function updateStatus(id: string, status: SessionStatus) {
    const s = sessionState.sessions.get(id)
    if (s) {
      s.status = status
    }
  }

  function appendData(id: string, chunk: string) {
    const s = sessionState.sessions.get(id)
    if (s) {
      s.data.push(chunk)
      if (s.data.length > 2000) {
        s.data.splice(0, s.data.length - 1000)
      }
    }
  }

  function getData(id: string): string {
    const s = sessionState.sessions.get(id)
    if (!s) return ''
    const raw = s.data.join('')
    // When the buffer is trimmed (2000→1000 chunks), the joined string may
    // start mid-escape-sequence (e.g. DA2, OSC color queries). Those broken
    // fragments lack the \x1b prefix and xterm.js renders them as garbled text.
    // Find the first \n or \x1b to locate a safe restart boundary.
    const nl = raw.indexOf('\n')
    const esc = raw.indexOf('\x1b')
    if (esc >= 0 && esc < 4096 && (nl < 0 || esc < nl)) {
      return raw.slice(esc)
    }
    if (nl > 0 && nl < 4096) {
      return raw.slice(nl + 1)
    }
    return raw
  }

  function removeSession(id: string) {
    sessionState.sessions.delete(id)
  }

  return {
    sessions: sessionState.sessions,
    initSession,
    updateStatus,
    appendData,
    getData,
    removeSession
  }
})
