import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// EventsOn is called at module load; stub it so importing the store is safe.
import { vi } from 'vitest'
vi.mock('../../wailsjs/runtime', () => ({
  EventsOn: vi.fn(() => () => {}),
}))

import { useSessionStore } from './sessionStore'

const MAX_CHUNKS = 2000
const TRIM_TO = 1000

describe('sessionStore replay tracking', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('getChunkCount is a monotonic sequence, not the buffer length', () => {
    const store = useSessionStore()
    const id = 'seq-monotonic'
    store.initSession(id)

    // Push more than MAX_CHUNKS so the buffer trims from the front.
    const total = MAX_CHUNKS + 500
    for (let i = 0; i < total; i++) store.appendData(id, `c${i}\n`)

    // seq counts everything ever appended, even though data was trimmed.
    expect(store.getChunkCount(id)).toBe(total)
  })

  it('replays the gap correctly after the buffer is trimmed', () => {
    const store = useSessionStore()
    const id = 'seq-gap'
    store.initSession(id)

    // Simulate: component recorded its position, then went to background
    // while a long compile flooded the session past the trim threshold.
    for (let i = 0; i < 500; i++) store.appendData(id, `pre${i}\n`)
    const writtenChunks = store.getChunkCount(id) // = 500

    // Flood well past MAX_CHUNKS so the first 500 chunks are trimmed away.
    const flood = MAX_CHUNKS + 800
    for (let i = 0; i < flood; i++) store.appendData(id, `post${i}\n`)

    // Old behavior (array index): getChunkCount would have been <= writtenChunks
    // after trimming, so `total > writtenChunks` was false and NOTHING replayed.
    const total = store.getChunkCount(id)
    expect(total).toBeGreaterThan(writtenChunks)

    // The gap replay must return recent output, including the very last chunk,
    // so the terminal never freezes mid-stream.
    const tail = store.getDataFromChunk(id, writtenChunks)
    expect(tail.length).toBeGreaterThan(0)
    expect(tail.endsWith(`post${flood - 1}\n`)).toBe(true)
  })

  it('getDataFromChunk returns best-effort tail when position was trimmed away', () => {
    const store = useSessionStore()
    const id = 'seq-trimmed-pos'
    store.initSession(id)

    for (let i = 0; i < MAX_CHUNKS + 1000; i++) store.appendData(id, `x${i}\n`)

    // Ask from sequence 0 (long since trimmed). Should not throw or return '',
    // but the buffered remainder — bounded by TRIM_TO..MAX_CHUNKS chunks.
    const tail = store.getDataFromChunk(id, 0)
    expect(tail.length).toBeGreaterThan(0)
    expect(tail.endsWith(`x${MAX_CHUNKS + 1000 - 1}\n`)).toBe(true)
    const lines = tail.trimEnd().split('\n')
    expect(lines.length).toBeLessThanOrEqual(MAX_CHUNKS)
    expect(lines.length).toBeGreaterThanOrEqual(TRIM_TO - 1)
  })

  it('getDataFromChunk returns empty once fully consumed', () => {
    const store = useSessionStore()
    const id = 'seq-consumed'
    store.initSession(id)
    for (let i = 0; i < 10; i++) store.appendData(id, `y${i}\n`)
    const total = store.getChunkCount(id)
    expect(store.getDataFromChunk(id, total)).toBe('')
  })
})
