import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import PracticeHistoryPage from '@/features/practice/PracticeHistoryPage.vue'
import WrongItemsPage from '@/features/practice/WrongItemsPage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
  },
}))

function routerFor(component: object) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/history', component },
      { path: '/wrong-items', component },
      { path: '/practice/:sessionId/result', component: { template: '<div />' } },
    ],
  })
}

describe('历史与错题本软删除', () => {
  it('隐藏一条练习历史并调用软删除接口', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    requestMock.mockImplementation(async (path: string) => {
      if (path.startsWith('/practice-sessions?')) {
        return { sessions: [{ id: 'session-1', status: 'active', totalCount: 20, createdAt: '2026-01-01T00:00:00Z', submittedAt: null }], nextCursor: '' }
      }
      return undefined
    })
    const router = routerFor(PracticeHistoryPage)
    await router.push('/history')
    await router.isReady()
    const wrapper = mount(PracticeHistoryPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('session-'))

    await wrapper.get('button.danger').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/practice-sessions/session-1', { method: 'DELETE' })
    expect(wrapper.text()).not.toContain('session-')
  })

  it('从错题本移除单题并调用软删除接口', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    requestMock.mockImplementation(async (path: string) => {
      if (path.startsWith('/wrong-items?')) {
        return {
          wrongItems: [{
            itemId: 'item-1', sessionId: 'session-1', questionId: 'question-1', position: 1,
            type: 'single_choice', stem: '練習問題です。', options: [], knowledgePoints: [], gradingStatus: 'incorrect',
            userAnswer: { optionIds: ['a'] }, correctAnswer: { optionIds: ['b'] },
          }],
        }
      }
      if (path === '/knowledge-points') return { knowledgePoints: [] }
      return undefined
    })
    const router = routerFor(WrongItemsPage)
    await router.push('/wrong-items')
    await router.isReady()
    const wrapper = mount(WrongItemsPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('練習問題です。'))

    await wrapper.get('button.danger').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/wrong-items/item-1', { method: 'DELETE' })
    expect(wrapper.text()).not.toContain('練習問題です。')
  })
})
