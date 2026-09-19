import { describe, expect, it, vi } from 'vitest'
import { request } from '@/api/client'

describe('access/refresh token', () => {
  it('401 后轮换 refresh token 并重试原请求', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: 'unauthorized', message: '请先登录' } }), { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ user: { id: 'u1' } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request<{ ok: boolean }>('/dashboard')).resolves.toEqual({ ok: true })
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[1]![0]).toBe('/api/v1/auth/refresh')
    expect(fetchMock.mock.calls[2]![0]).toBe('/api/v1/dashboard')
  })
})
