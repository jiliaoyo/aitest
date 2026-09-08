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

const session: PreSubmitSession = {
  id: 'session-1', status: 'active', answeredCount: 0, totalCount: 1, items: [{
    id: 'item-1', position: 1, type: 'short_answer', material: null, stem: '题干', options: [],
    savedAnswer: null, markedForReview: false, savedAt: null,
  }],
}

afterEach(() => {
  vi.restoreAllMocks()
  requestMock.mockReset()
})

describe('练习提交', () => {
  it('不等待自动保存，并在存储不可用时复用页面内幂等键', async () => {
    vi.spyOn(localStorage, 'getItem').mockImplementation(() => { throw new Error('storage disabled') })
    vi.spyOn(localStorage, 'setItem').mockImplementation(() => { throw new Error('storage disabled') })
    vi.spyOn(localStorage, 'removeItem').mockImplementation(() => { throw new Error('storage disabled') })
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('00000000-0000-4000-8000-000000000001')

    const never = new Promise(() => {})
    const submitHeaders: string[] = []
    let submits = 0
    requestMock.mockImplementation(async (path: string, options?: { method?: string; headers?: Record<string, string> }) => {
      if (path === '/practice-sessions/session-1' && !options) return session
      if (options?.method === 'PUT') return never
      if (options?.method === 'POST') {
        submitHeaders.push(options.headers?.['Idempotency-Key'] ?? '')
        submits++
        if (submits === 1) throw new Error('network failed')
        return {}
      }
      return undefined
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/practice/:sessionId', component: PracticePage },
        { path: '/practice/:sessionId/result', component: { template: '<div>result</div>' } },
      ],
    })
    await router.push('/practice/session-1')
    await router.isReady()
    const wrapper = mount(PracticePage, { global: { plugins: [router] } })
    await flushPromises()

    await wrapper.get('textarea').setValue('回答')
    await wrapper.findAll('button').find((button) => button.text() === '提交本批练习')!.trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)

    await wrapper.findAll('button').find((button) => button.text() === '确认提交')!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '提交本批练习')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === '确认提交')!.trigger('click')
    await flushPromises()

    expect(submitHeaders).toEqual([
      '00000000-0000-4000-8000-000000000001',
      '00000000-0000-4000-8000-000000000001',
    ])
    expect(router.currentRoute.value.path).toBe('/practice/session-1/result')
    wrapper.unmount()
  })
})
