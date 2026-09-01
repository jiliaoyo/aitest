import { shallowRef } from 'vue'
import { request } from '@/api/client'
import type { Me } from '@/api/types'

// session 只保存当前账号信息；密码与 Cookie 永远不进入应用状态或 localStorage。
const user = shallowRef<Me | null>(null)
const ready = shallowRef(false)

export function sessionUser(): Readonly<typeof user.value> {
  return user.value
}

export function isAdmin(): boolean {
  return user.value?.role === 'admin'
}

export async function refreshSession(): Promise<void> {
  try {
    const res = await request<{ user: Me }>('/me')
    user.value = res.user
  } catch {
    user.value = null
  } finally {
    ready.value = true
  }
}

export function clearSession(): void {
  user.value = null
}

export async function ensureSessionReady(): Promise<void> {
  if (!ready.value) {
    await refreshSession()
  }
}

/** 站内 redirect 必须以单个 / 开头，防止开放重定向。 */
export function safeRedirect(target: string | null | undefined, fallback = '/'): string {
  if (target && target.startsWith('/') && !target.startsWith('//')) {
    return target
  }
  return fallback
}
