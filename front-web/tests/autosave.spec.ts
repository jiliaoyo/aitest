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
})
