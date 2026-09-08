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
      { path: '/practice/:sessionId', component: { template: '<div />' } },
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
      if (path === '/catalog') return { exams: [{ id: 'exam', code: 'jlpt', name: 'JLPT', levels: [{ id: 'level-1', code: 'n5', name: 'N5' }], subjects: [] }] }
      if (path === '/me') return { user: { defaultLevelId: 'level-1' } }
      if (path.startsWith('/wrong-items?')) {
        return {
          wrongItems: [{
            itemId: 'item-1', sessionId: 'session-1', questionId: 'question-1', position: 1,
            type: 'single_choice', stem: '練習問題です。', options: [{ id: 'a', label: 'A', text: '甲' }, { id: 'b', label: 'B', text: '乙' }], knowledgePoints: [], gradingStatus: 'incorrect',
            gradingSource: 'deterministic',
            userAnswer: { optionIds: ['a'] }, correctAnswer: { optionIds: ['b'] },
          }],
        }
      }
      if (path.startsWith('/knowledge-points?')) return { knowledgePoints: [] }
      return undefined
    })
    const router = routerFor(WrongItemsPage)
    await router.push('/wrong-items')
    await router.isReady()
    const wrapper = mount(WrongItemsPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('練習問題です。'))
    expect(wrapper.text()).toContain('你的答案：A. 甲')
    expect(wrapper.text()).toContain('标准答案：B. 乙')
    expect((wrapper.get('#include-correct').element as HTMLInputElement).checked).toBe(false)

    await wrapper.get('#include-correct').setValue(true)
    expect(requestMock.mock.calls.some(([path]) => path === '/wrong-items?limit=20&levelId=level-1&includeCorrect=true')).toBe(true)

    await wrapper.get('button.danger').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/wrong-items/item-1', { method: 'DELETE' })
    expect(wrapper.text()).not.toContain('練習問題です。')
  })

  it('按日期和关键词筛选错题', async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path === '/catalog') return { exams: [{ id: 'exam', code: 'jlpt', name: 'JLPT', levels: [{ id: 'level-1', code: 'n5', name: 'N5' }], subjects: [] }] }
      if (path === '/me') return { user: { defaultLevelId: 'level-1' } }
      if (path.startsWith('/wrong-items?')) return { wrongItems: [] }
      if (path.startsWith('/knowledge-points?')) return { knowledgePoints: [] }
      return undefined
    })
    const router = routerFor(WrongItemsPage)
    await router.push('/wrong-items')
    await router.isReady()
    const wrapper = mount(WrongItemsPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('没有待复习的错题'))

    await wrapper.get('#wrong-keyword').setValue('语法')
    await wrapper.get('#wrong-from').setValue('2026-01-01')
    await wrapper.get('#wrong-to').setValue('2026-01-31')
    await wrapper.get('#include-correct').setValue(true)
    await wrapper.get('#apply-wrong-filters').trigger('click')
    await flushPromises()

    expect(requestMock.mock.calls.some(([path]) => path === '/wrong-items?limit=20&levelId=level-1&from=2026-01-01&to=2026-01-31&keyword=%E8%AF%AD%E6%B3%95&includeCorrect=true')).toBe(true)
  })

  it('错题重练沿用当前级别和筛选', async () => {
    requestMock.mockImplementation(async (path: string, options?: { method?: string }) => {
      if (path === '/catalog') return { exams: [{ id: 'exam', code: 'jlpt', name: 'JLPT', levels: [{ id: 'level-1', code: 'n5', name: 'N5' }], subjects: [] }] }
      if (path === '/me') return { user: { defaultLevelId: 'level-1' } }
      if (path.startsWith('/knowledge-points?')) return { knowledgePoints: [{
        id: 'kp-1', name: '助词', levelId: 'level-1', levelCode: 'n5', subjectId: 'subject', subjectName: '语法',
        parentId: null, questionCount: 10, stats: { confirmedAnswered: 1 },
      }] }
      if (path.startsWith('/wrong-items?')) return { wrongItems: [{
        itemId: 'item-1', sessionId: 'old-session', questionId: 'question-1', position: 1,
        type: 'single_choice', stem: '助词题', options: [], knowledgePoints: [], gradingStatus: 'incorrect',
        gradingSource: 'deterministic', userAnswer: null, correctAnswer: null,
      }] }
      if (path === '/practice-sessions' && options?.method === 'POST') return { id: 'new-session' }
      return undefined
    })
    const router = routerFor(WrongItemsPage)
    await router.push('/wrong-items')
    await router.isReady()
    const wrapper = mount(WrongItemsPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('助词题'))

    await wrapper.get('#wrong-keyword').setValue('助词')
    await wrapper.get('#wrong-from').setValue('2026-01-01')
    await wrapper.get('#kp-filter').setValue('kp-1')
    await wrapper.findAll('button').find((button) => button.text() === '错题重练 10 题')!.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/practice-sessions', {
      method: 'POST',
      body: {
        levelId: 'level-1', mode: 'wrong_items', knowledgePointIds: ['kp-1'],
        from: '2026-01-01', to: '', keyword: '助词', count: 10,
      },
    })
    expect(router.currentRoute.value.path).toBe('/practice/new-session')
  })
})
