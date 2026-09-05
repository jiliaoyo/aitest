import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import PracticePage from '@/features/practice/PracticePage.vue'
import type { PreSubmitSession } from '@/api/types'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
  },
}))

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

const generatingSession: PreSubmitSession = {
  id: 'session-1', status: 'generating', answeredCount: 0, totalCount: 0, items: [],
}

const activeSession: PreSubmitSession = {
  id: 'session-1', status: 'active', answeredCount: 0, totalCount: 1, items: [{
    id: 'item-1', position: 1, type: 'short_answer', material: null, stem: '题干', options: [],
    savedAnswer: null, markedForReview: false, savedAt: null,
  }],
}

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('AI 出题等待页', () => {
  it('忽略晚返回的旧轮询结果，不把已生成状态覆盖回 generating', async () => {
    vi.useFakeTimers()
    const firstPoll = deferred<PreSubmitSession>()
    const secondPoll = deferred<PreSubmitSession>()
    let reads = 0
    requestMock.mockImplementation(async () => {
      reads++
      if (reads === 1) return generatingSession
      if (reads === 2) return firstPoll.promise
      if (reads === 3) return secondPoll.promise
      return activeSession
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/practice/:sessionId', component: PracticePage }],
    })
    await router.push('/practice/session-1')
    await router.isReady()
    const wrapper = mount(PracticePage, { global: { plugins: [router] } })
    await flushPromises()

    await vi.advanceTimersByTimeAsync(4000)
    secondPoll.resolve(activeSession)
    await flushPromises()
    expect(wrapper.text()).toContain('练习进行中')

    firstPoll.resolve(generatingSession)
    await flushPromises()
    expect(wrapper.text()).toContain('练习进行中')
    expect(wrapper.text()).not.toContain('AI 正在生成个性化题目')
    wrapper.unmount()
  })
})
