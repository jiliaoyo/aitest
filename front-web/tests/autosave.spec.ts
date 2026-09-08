import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useAnswerAutosave } from '@/features/practice/useAnswerAutosave'
import type { PreSubmitItem } from '@/api/types'

function makeItem(id: string): PreSubmitItem {
  return {
    id,
    position: 1,
    type: 'single_choice',
    material: null,
    stem: 'test',
    options: [
      { id: 'a', label: 'A', text: 'one' },
      { id: 'b', label: 'B', text: 'two' },
    ],
    savedAnswer: null,
    markedForReview: false,
    savedAt: null,
  }
}

const items = [makeItem('item-1')]

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('自动保存', () => {
  it('修改时立即暂存，离开页面后可以恢复尚未发送的答案', () => {
    const first = useAnswerAutosave(ref('session-leave'))
    first.init(items)
    first.setAnswer('item-1', { optionIds: ['b'] })

    expect(JSON.parse(localStorage.getItem('practice-draft:session-leave') ?? '{}')['item-1']?.value)
      .toEqual({ optionIds: ['b'] })
    first.dispose()

    const second = useAnswerAutosave(ref('session-leave'))
    expect(second.init(items)).toBe(true)
    expect(second.entries.get('item-1')?.value).toEqual({ optionIds: ['b'] })
    second.dispose()
  })

  it('同一题连续快速修改时，最终值不会被旧请求覆盖', async () => {
    const bodies: Array<{ value: unknown }> = []
    let releaseFirstSave: () => void = () => {}
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      bodies.push(JSON.parse(String(init?.body)) as { value: unknown })
      if (bodies.length === 1) {
        // 第一次保存挂起，模拟慢请求
        await new Promise<void>((resolve) => {
          releaseFirstSave = resolve
        })
      }
      return new Response(JSON.stringify({ savedAt: '2026-09-01T00:00:00Z' }), { status: 200 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const { init, setAnswer, entries } = useAnswerAutosave(ref('session-1'))
    init(items)

    setAnswer('item-1', { optionIds: ['a'] })
    await vi.waitFor(() => expect(bodies.length).toBe(1))
    // 请求进行中再次修改：应标记 dirty，完成后立即补存最新值
    setAnswer('item-1', { optionIds: ['b'] })
    releaseFirstSave()
    await vi.waitFor(() => expect(bodies.length).toBe(2))

    expect(bodies[0]?.value).toEqual({ optionIds: ['a'] })
    expect(bodies[bodies.length - 1]?.value).toEqual({ optionIds: ['b'] })
    expect(entries.get('item-1')?.state).toBe('saved')
  })

  it('保存失败时写入本地草稿，成功后清理', async () => {
    let fail = true
    const fetchMock = vi.fn(async (_url: string) => {
      if (fail) {
        return new Response(JSON.stringify({ error: { code: 'internal_error', message: 'fail' } }), { status: 500 })
      }
      return new Response(JSON.stringify({ savedAt: '2026-09-01T00:00:00Z' }), { status: 200 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const { init, setAnswer, retryFailed, entries } = useAnswerAutosave(ref('session-draft'))
    init(items)
    setAnswer('item-1', { optionIds: ['b'] })
    await vi.waitFor(() => expect(entries.get('item-1')?.state).toBe('error'))

    const draft = localStorage.getItem('practice-draft:session-draft')
    expect(draft).toBeTruthy()
    expect(JSON.parse(draft ?? '{}')['item-1']?.value).toEqual({ optionIds: ['b'] })

    fail = false
    await retryFailed()
    expect(entries.get('item-1')?.state).toBe('saved')
    expect(localStorage.getItem('practice-draft:session-draft')).toBeNull()
  })

  it('旧请求成功时不会清除请求期间产生的新答案', async () => {
    let releaseFirst!: (response: Response) => void
    let releaseSecond!: (response: Response) => void
    const bodies: Array<{ value: unknown }> = []
    const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
      bodies.push(JSON.parse(String(init?.body)) as { value: unknown })
      return new Promise<Response>((resolve) => {
        if (bodies.length === 1) releaseFirst = resolve
        else releaseSecond = resolve
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    const autosave = useAnswerAutosave(ref('session-newer'))
    autosave.init(items)
    autosave.setAnswer('item-1', { optionIds: ['a'] })
    autosave.flushPending(items)
    autosave.setAnswer('item-1', { optionIds: ['b'] })

    releaseFirst(new Response(JSON.stringify({ savedAt: '2026-09-01T00:00:00Z' }), { status: 200 }))
    await vi.waitFor(() => expect(bodies.length).toBe(2))
    expect(JSON.parse(localStorage.getItem('practice-draft:session-newer') ?? '{}')['item-1']?.value)
      .toEqual({ optionIds: ['b'] })

    releaseSecond(new Response(JSON.stringify({ savedAt: '2026-09-01T00:00:01Z' }), { status: 200 }))
    await vi.waitFor(() => expect(localStorage.getItem('practice-draft:session-newer')).toBeNull())
  })

  it('提交清理后晚到的保存失败不会恢复旧草稿', async () => {
    let release!: (response: Response) => void
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((resolve) => { release = resolve })))

    const autosave = useAnswerAutosave(ref('session-submitted'))
    autosave.init(items)
    autosave.setAnswer('item-1', { optionIds: ['b'] })
    autosave.flushPending(items)
    autosave.cleanup()
    release(new Response(JSON.stringify({ error: { code: 'failed', message: 'failed' } }), { status: 500 }))

    await vi.waitFor(() => expect(localStorage.getItem('practice-draft:session-submitted')).toBeNull())
  })

  it('切换批次时不复用上一批的内存答案', () => {
    const sessionID = ref('session-a')
    const autosave = useAnswerAutosave(sessionID)
    autosave.init(items)
    autosave.setAnswer('item-1', { optionIds: ['b'] })

    sessionID.value = 'session-b'
    autosave.init(items)

    expect(autosave.entries.get('item-1')?.value).toBeNull()
    expect(localStorage.getItem('practice-draft:session-a')).toBeTruthy()
    expect(localStorage.getItem('practice-draft:session-b')).toBeNull()
    autosave.dispose()
  })
})
