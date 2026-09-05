import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import DashboardPage from '@/features/dashboard/DashboardPage.vue'
import SettingsPage from '@/features/settings/SettingsPage.vue'

const requestMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  request: requestMock,
  ApiError: class ApiError extends Error {
    status = 500
    code = 'internal_error'
  },
  fieldErrors: () => ({}),
}))

vi.mock('@/app/session', () => ({
  sessionUser: () => ({ email: 'learner@example.com', defaultLevelId: null }),
  isAdmin: () => false,
  clearSession: vi.fn(),
}))

afterEach(() => {
  requestMock.mockReset()
  vi.restoreAllMocks()
})

describe('账号学习记忆', () => {
  it('展示基于累计统计生成的 AI 学习建议', async () => {
    requestMock.mockResolvedValue({
      activeSession: null, recentSessions: [], recommendations: [], statsEmpty: true,
      memory: {
        confirmedAnswered: 12, confirmedCorrect: 7, aiAnswered: 1, aiCorrect: 1, estimatedAccuracy: 8 / 13,
        advice: { status: 'completed', text: '继续练习助词的场所用法。' },
      },
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(DashboardPage, { global: { plugins: [router] } })
    await vi.waitFor(() => expect(wrapper.text()).toContain('继续练习助词的场所用法。'))
    expect(wrapper.text()).toContain('已确认作答 12 题，正确 7 题')
    expect(wrapper.text()).toContain('含 AI 判定的估算正确率 61.5%（可能有误）')
  })

  it('确认后调用删除接口并提示从新进度累计', async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path === '/catalog') return { exams: [] }
      return undefined
    })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/settings', component: SettingsPage }],
    })
    await router.push('/settings')
    await router.isReady()
    const wrapper = mount(SettingsPage, { global: { plugins: [router] } })
    await flushPromises()

    await wrapper.get('button.danger').trigger('click')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/learning-memory', { method: 'DELETE' })
    expect(wrapper.text()).toContain('已删除，新的进度会重新累计')
  })

  it('个人中心可修改密码并保留当前会话', async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path === '/catalog') return { exams: [] }
      if (path === '/auth/change-password') return { ok: true }
      return undefined
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/settings', component: SettingsPage }],
    })
    await router.push('/settings')
    await router.isReady()
    const wrapper = mount(SettingsPage, { global: { plugins: [router] } })
    await flushPromises()

    await wrapper.get('#current-password').setValue('old-password')
    await wrapper.get('#new-password').setValue('new-password')
    await wrapper.get('#confirm-password').setValue('new-password')
    await wrapper.get('#change-password-form').trigger('submit')
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/auth/change-password', {
      method: 'POST', body: { currentPassword: 'old-password', newPassword: 'new-password' },
    })
    expect(wrapper.text()).toContain('密码已修改')
  })
})
