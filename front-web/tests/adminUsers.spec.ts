import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import AdminUsersPage from '@/features/admin/AdminUsersPage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    requestId?: string
  },
}))

describe('管理端用户用量', () => {
  it('展示 AI 出题请求、实际调用、token 和用户明细', async () => {
    requestMock.mockResolvedValue({
      summary: {
        totalUsers: 2, learnerUsers: 1, adminUsers: 1, newUsers: 1, activeUsers: 1,
        usage: {
          activeDays: 2, lastActiveAt: '2026-09-05T00:00:00Z', practiceSessions: 3,
          completedSessions: 2, analysisFailedSessions: 0, generationFailedSessions: 0,
          activeSessions: 1, submittedSessions: 2, practiceItems: 30, answeredItems: 28,
          aiGenerationRequests: 1, aiGeneratedQuestions: 10,
          ai: { calls: 2, successfulCalls: 2, failedCalls: 0, generationCalls: 1, promptTokens: 100, completionTokens: 40, totalTokens: 140, durationMs: 800, costedCalls: 2, estimatedCostUsd: 0.12 },
        },
        aiByKind: [], aiByModel: [], aiDaily: [],
      },
      users: [{
        id: 'user-1', email: 'learner@example.com', role: 'learner', defaultLevelId: null,
        defaultLevelCode: '', defaultLevelName: '', createdAt: '2026-09-01T00:00:00Z',
        lastActiveAt: '2026-09-05T00:00:00Z',
        usage: {
          activeDays: 2, lastActiveAt: '2026-09-05T00:00:00Z', practiceSessions: 3,
          completedSessions: 2, analysisFailedSessions: 0, generationFailedSessions: 0,
          activeSessions: 1, submittedSessions: 2, practiceItems: 30, answeredItems: 28,
          aiGenerationRequests: 1, aiGeneratedQuestions: 10,
          ai: { calls: 2, successfulCalls: 2, failedCalls: 0, generationCalls: 1, promptTokens: 100, completionTokens: 40, totalTokens: 140, durationMs: 800, costedCalls: 2, estimatedCostUsd: 0.12 },
        },
      }],
      nextCursor: '',
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/admin/users', component: AdminUsersPage }],
    })
    await router.push('/admin/users')
    await router.isReady()
    const wrapper = mount(AdminUsersPage, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('出题请求批次')
    expect(wrapper.text()).toContain('learner@example.com')
    expect(wrapper.text()).toContain('0.12')
    expect(requestMock).toHaveBeenCalledWith('/admin/users?limit=20')
  })
})
