import { shallowRef } from 'vue';
import { request } from '@/api/client';
// session 只保存当前账号信息；密码与 Cookie 永远不进入应用状态或 localStorage。
const user = shallowRef(null);
const ready = shallowRef(false);
export function sessionUser() {
    return user.value;
}
export function isAdmin() {
    return user.value?.role === 'admin';
}
export async function refreshSession() {
    try {
        const res = await request('/me');
        user.value = res.user;
    }
    catch {
        user.value = null;
    }
    finally {
        ready.value = true;
    }
}
export function clearSession() {
    user.value = null;
}
export async function ensureSessionReady() {
    if (!ready.value) {
        await refreshSession();
    }
}
/** 站内 redirect 必须以单个 / 开头，防止开放重定向。 */
export function safeRedirect(target, fallback = '/') {
    if (target && target.startsWith('/') && !target.startsWith('//')) {
        return target;
    }
    return fallback;
}
