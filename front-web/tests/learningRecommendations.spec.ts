import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import DashboardPage from '@/features/dashboard/DashboardPage.vue'
import KnowledgeDetailPage from '@/features/knowledge/KnowledgeDetailPage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
    requestId?: string
  },
}))

function routerFor() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: DashboardPage },
      { path: '/knowledge/:knowledgePointId', component: KnowledgeDetailPage },
      { path: '/practice/new', component: { template: '<div />' } },
      { path: '/practice/:sessionId', component: { template: '<div />' } },
    ],
  })
}

const emptyRecent = {
  activeSession: null,
  recentSessions: [],
  recommendations: [],
  comprehensive: { type: 'comprehensive', name: '综合练习', suggestedCount: 20, reason: '数据不足，建议综合练习。', knowledgePointIds: [] },
  statsEmpty: true,
}

describe('学习推荐闭环', () => {
  it('推荐卡创建对应知识点专项练习', async () => {
    requestMock.mockImplementation(async (path: string, options?: { method?: string; body?: unknown }) => {
      if (path === '/dashboard') {
        return {
          ...emptyRecent,
          recommendations: [{
            type: 'knowledge', knowledgePointId: 'kp-1', knowledgePointIds: ['kp-1'], name: '助词与格关系',
            recentAnswered: 9, recentWrongCount: 5, accuracy: 4 / 9, consecutiveWrong: 3, suggestedCount: 10,
            reason: '最近 30 天错了 5 题。',
          }],
        }
      }
      if (path === '/me') return { user: { defaultLevelId: 'n5' } }
      if (path === '/practice-sessions' && options?.method === 'POST') return { id: 'special-session' }
      return emptyRecent
    })
    const router = routerFor()
    await router.push('/')
    await router.isReady()
    const wrapper = mount(DashboardPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('助词与格关系'))

    const practiceButton = wrapper.findAll('button').find((button) => button.text().includes('练习 10'))
    expect(practiceButton).toBeDefined()
    await practiceButton!.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/practice-sessions', expect.objectContaining({
      method: 'POST',
      body: expect.objectContaining({ mode: 'knowledge', knowledgePointIds: ['kp-1'], count: 10 }),
    }))
    expect(router.currentRoute.value.fullPath).toBe('/practice/special-session')
  })

  it('样本不足时显示综合练习建议', async () => {
    requestMock.mockResolvedValue(emptyRecent)
    const router = routerFor()
    await router.push('/')
    await router.isReady()
    const wrapper = mount(DashboardPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('数据不足'))
    expect(wrapper.text()).toContain('创建综合练习')
  })

  it('知识点详情可直接开始专项练习，题量为零时禁用入口', async () => {
    requestMock.mockResolvedValue({
      id: 'kp-1', name: '助词与格关系', levelId: 'level-1', levelCode: 'N5', subjectId: 'subject-1',
      subjectName: '语法', parentId: null, questionCount: 12, description: '说明', commonMistakes: '误区',
      examples: '例句', status: 'published',
    })
    const router = routerFor()
    await router.push('/knowledge/kp-1')
    await router.isReady()
    const wrapper = mount(KnowledgeDetailPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('专项练习 10 题'))
    requestMock.mockResolvedValueOnce({ id: 'detail-session' })
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(requestMock).toHaveBeenCalledWith('/practice-sessions', expect.objectContaining({
      method: 'POST', body: expect.objectContaining({ knowledgePointIds: ['kp-1'], mode: 'knowledge', count: 10 }),
    }))

    requestMock.mockResolvedValueOnce({
      id: 'kp-empty', name: '无题知识点', levelId: 'level-1', levelCode: 'N5', subjectId: 'subject-1',
      subjectName: '语法', parentId: null, questionCount: 0, description: '', commonMistakes: '', examples: '', status: 'published',
    })
    await router.push('/knowledge/kp-empty')
    const emptyWrapper = mount(KnowledgeDetailPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(emptyWrapper.text()).toContain('还没有已发布题目'))
    expect(emptyWrapper.find('button').attributes('disabled')).toBeDefined()
  })

  it('知识点详情可按选择的难度默认生成 20 题', async () => {
    requestMock.mockImplementation(async (path: string, options?: { method?: string; body?: unknown }) => {
      if (path === '/knowledge-points/kp-1') {
        return {
          id: 'kp-1', name: '助词与格关系', levelId: 'level-1', levelCode: 'N5', subjectId: 'subject-1',
          subjectName: '语法', parentId: null, questionCount: 0, description: '说明', commonMistakes: '', examples: '', status: 'published',
        }
      }
      if (path === '/ai-practice-sessions' && options?.method === 'POST') return { id: 'ai-session', status: 'generating' }
      return {}
    })
    const router = routerFor()
    await router.push('/knowledge/kp-1')
    await router.isReady()
    const wrapper = mount(KnowledgeDetailPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('生成 AI 题目（20 题）'))

    await wrapper.get('#ai-question-type').setValue('short_answer')
    await wrapper.get('#ai-difficulty').setValue('hard')
    await wrapper.get('#ai-furigana').setValue(true)
    await wrapper.findAll('button').find((button) => button.text().includes('生成 AI 题目'))!.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/ai-practice-sessions', expect.objectContaining({
      method: 'POST',
      body: expect.objectContaining({
        levelId: 'level-1', subjectId: 'subject-1', knowledgePointIds: ['kp-1'], count: 20, difficulty: 'hard', questionType: 'short_answer', showFurigana: true,
      }),
    }))
    expect(router.currentRoute.value.fullPath).toBe('/practice/ai-session')
  })
})
