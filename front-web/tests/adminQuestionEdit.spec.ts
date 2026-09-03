import { describe, expect, beforeEach, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import AdminQuestionEditPage from '@/features/admin/AdminQuestionEditPage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  fieldErrors: () => ({}),
  ApiError: class ApiError extends Error {
    status = 500
    requestId?: string
  },
}))

const question = {
  id: 'question-1',
  status: 'draft' as const,
  hasAnswer: false,
  publishedVersionId: null,
  currentVersion: {
    id: 'version-1',
    questionId: 'question-1',
    versionNo: 1,
    type: 'single_choice' as const,
    stem: 'これは問題です。',
    options: [
      { id: 'a', label: 'A', text: '甲' },
      { id: 'b', label: 'B', text: '乙' },
    ],
    levelId: 'level-1',
    subjectId: 'subject-1',
    sourceSectionId: 'section-3',
    difficulty: 3,
    knowledgePointIds: [],
    createdAt: '2026-09-01T00:00:00Z',
  },
  updatedAt: '2026-09-01T00:00:00Z',
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/questions/:questionId', component: AdminQuestionEditPage }],
  })
}

function mockInitialRequests(): void {
  requestMock.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (options?.method === 'PATCH') throw new Error('save failed')
    if (path === '/catalog') return { exams: [{ id: 'exam-1', code: 'JLPT', name: 'JLPT', levels: [{ id: 'level-1', code: 'N5', name: 'N5' }], subjects: [{ id: 'subject-1', code: 'grammar', name: '语法' }] }] }
    if (path === '/admin/sources') return {
      sources: [
        { id: 'source-1', name: '来源一', kind: 'book', author: '', publisher: '', year: null, licenseNote: 'local', internalNote: '', sections: [{ id: 'section-1', sourceId: 'source-1', name: '第一章' }] },
        { id: 'source-2', name: '来源二', kind: 'book', author: '', publisher: '', year: null, licenseNote: 'local', internalNote: '', sections: [{ id: 'section-2', sourceId: 'source-2', name: '第二章' }] },
        { id: 'source-3', name: '来源三', kind: 'book', author: '', publisher: '', year: null, licenseNote: 'local', internalNote: '', sections: [{ id: 'section-3', sourceId: 'source-3', name: '第三章' }] },
      ],
    }
    if (path === '/admin/questions/question-1') return { question }
    return { knowledgePoints: [] }
  })
}

describe('管理端题目编辑', () => {
  beforeEach(() => {
    requestMock.mockReset()
    mockInitialRequests()
  })

  it('编辑第三个来源的题目时保留原章节', async () => {
    const router = makeRouter()
    await router.push('/admin/questions/question-1')
    await router.isReady()
    const wrapper = mount(AdminQuestionEditPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.find('#q-section').exists()).toBe(true))

    const select = wrapper.find<HTMLSelectElement>('#q-section').element
    expect(select.querySelectorAll('optgroup')).toHaveLength(3)
    expect(Array.from(select.options).find((option) => option.selected)?.value).toBe('section-3')
  })

  it('保存失败时不会提交审核', async () => {
    const router = makeRouter()
    await router.push('/admin/questions/question-1')
    await router.isReady()
    const wrapper = mount(AdminQuestionEditPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.find('#q-section').exists()).toBe(true))
    await flushPromises()

    const reviewButton = wrapper.findAll('button').find((button) => button.text() === '提交审核')
    expect(reviewButton).toBeDefined()
    await reviewButton!.trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/admin/questions/question-1', expect.objectContaining({ method: 'PATCH' }))
    expect(requestMock).not.toHaveBeenCalledWith('/admin/questions/question-1/submit-review', expect.anything())
  })
})
