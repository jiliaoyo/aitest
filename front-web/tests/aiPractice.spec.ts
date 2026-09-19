import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import CreatePracticePage from '@/features/practice/CreatePracticePage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
  },
}))

describe('AI 个性化练习', () => {
  it('把当前级别和选定题型交给 AI 生成队列并进入生成批次', async () => {
    requestMock.mockImplementation(async (path: string, options?: { method?: string }) => {
      if (path === '/catalog') {
        return { exams: [{ id: 'jlpt', code: 'JLPT', name: 'JLPT', levels: [{ id: 'n5', code: 'N5', name: 'N5' }, { id: 'n2', code: 'N2', name: 'N2' }, { id: 'n1', code: 'N1', name: 'N1' }], subjects: [{ id: 'grammar', code: 'grammar', name: '语法' }, { id: 'vocabulary', code: 'vocabulary', name: '文字词汇' }] }] }
      }
      if (path.startsWith('/practice/sources')) return { sources: [] }
      if (path.startsWith('/practice/availability')) return { available: 20 }
      if (path === '/ai-practice-sessions' && options?.method === 'POST') return { id: 'ai-session', status: 'generating' }
      return {}
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/practice/new', component: CreatePracticePage }, { path: '/practice/:sessionId', component: { template: '<div />' } }],
    })
    await router.push('/practice/new')
    await router.isReady()
    const wrapper = mount(CreatePracticePage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('根据我的记忆生成题目'))
    await wrapper.get('input[type="checkbox"][value="grammar"]').setValue(true)
    expect(wrapper.text()).toContain('愿望、计划与推量')
    expect(wrapper.text()).not.toContain('条件、假定与逆接')
    expect(wrapper.text()).not.toContain('可能、被动、使役')
    await wrapper.get('input[type="checkbox"][value="vocabulary"]').setValue(true)
    expect(wrapper.text()).toContain('汉字读音与表记')

    await wrapper.get('input[name="ai-generation-mode"][value="level"]').setValue(true)
    expect(wrapper.get('button.primary[type="button"]').text()).toContain('根据当前级别生成题目')
    await wrapper.get('#ai-level').setValue('n2')
    expect(wrapper.text()).not.toContain('终助词')
    await wrapper.get('#ai-level').setValue('n1')
    expect(wrapper.text()).toContain('终助词')
    await wrapper.get('input[type="checkbox"][value="grammar_case_particle"]').setValue(true)
    await wrapper.get('input[name="ai-question-type"][value="fill_blank"]').setValue(true)
    await wrapper.get('#ai-furigana').setValue(true)
    await wrapper.get('#ai-difficulty').setValue('hard')
    await wrapper.get('button.primary[type="button"]').trigger('click')
    await vi.waitFor(() => expect(requestMock).toHaveBeenCalledWith('/ai-practice-sessions', expect.anything()))
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/ai-practice-sessions', expect.objectContaining({
      method: 'POST',
      body: expect.objectContaining({
        levelId: 'n1', subjectIds: ['grammar', 'vocabulary'], categories: ['grammar_case_particle'], count: 20, difficulty: 'hard', generationMode: 'level', questionType: 'fill_blank', showFurigana: true, knowledgePointIds: [],
      }),
    }))
    expect(router.currentRoute.value.fullPath).toBe('/practice/ai-session')
  })
})
