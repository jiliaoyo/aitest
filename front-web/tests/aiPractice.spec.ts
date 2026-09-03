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
        return { exams: [{ id: 'jlpt', code: 'JLPT', name: 'JLPT', levels: [{ id: 'n5', code: 'N5', name: 'N5' }, { id: 'n1', code: 'N1', name: 'N1' }], subjects: [{ id: 'grammar', code: 'grammar', name: '语法' }] }] }
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

    await wrapper.get('input[name="ai-generation-mode"][value="level"]').setValue(true)
    await wrapper.get('#ai-level').setValue('n1')
    await wrapper.get('#ai-subject').setValue('grammar')
    await wrapper.get('input[name="ai-question-type"][value="fill_blank"]').setValue(true)
    await wrapper.get('#ai-furigana').setValue(true)
    await wrapper.get('#ai-difficulty').setValue('hard')
    await wrapper.get('button[type="button"]').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/ai-practice-sessions', expect.objectContaining({
      method: 'POST',
      body: expect.objectContaining({
        levelId: 'n1', subjectId: 'grammar', count: 20, difficulty: 'hard', generationMode: 'level', questionType: 'fill_blank', showFurigana: true, knowledgePointIds: [],
      }),
    }))
    expect(router.currentRoute.value.fullPath).toBe('/practice/ai-session')
  })
})
