import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import PracticeResultPage from '@/features/practice/PracticeResultPage.vue'
import type { ResultSession } from '@/api/types'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
    requestId?: string
  },
}))

const failedResult: ResultSession = {
  id: 'session-1',
  status: 'analysis_failed',
  createdAt: '2026-09-01T00:00:00Z',
  submittedAt: '2026-09-01T00:01:00Z',
  summary: {
    confirmed: { correct: 1, total: 1, accuracy: 1 },
    ai: { correct: 0, completed: 0, pending: 1, failed: 1 },
  },
  aiAnalysis: { status: 'failed', text: 'AI 分析失败' },
  items: [],
}

const pendingResult: ResultSession = {
  ...failedResult,
  status: 'grading',
  aiAnalysis: { status: 'pending', text: '' },
}

describe('结果页 AI 重试', () => {
  it('失败时可重新分析，处理中按钮禁用', async () => {
    requestMock.mockImplementation(async (_path: string, options?: { method?: string }) =>
      options?.method === 'POST' ? pendingResult : failedResult)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/practice/:sessionId/result', component: PracticeResultPage }],
    })
    await router.push('/practice/session-1/result')
    await router.isReady()
    const wrapper = mount(PracticeResultPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('重新分析'))

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/practice-sessions/session-1/analysis/retry', { method: 'POST' })
    const retryButton = wrapper.findAll('button').find((button) => button.text() === '重新分析中…')
    expect(retryButton?.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})
