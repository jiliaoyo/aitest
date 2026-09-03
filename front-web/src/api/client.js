import { clearSession } from '@/app/session';
// ApiError 携带后端稳定错误码；页面分支只判断 code，提示优先使用后端 message。
export class ApiError extends Error {
    status;
    code;
    requestId;
    details;
    constructor(body) {
        super(body.message);
        this.name = 'ApiError';
        this.status = body.status;
        this.code = body.code;
        this.requestId = body.requestId;
        this.details = body.details;
    }
}
const BASE = '/api/v1';
// request 是唯一的 HTTP 入口：统一 Cookie、错误结构与 401 处理。
export async function request(path, opts = {}) {
    let res;
    try {
        res = await fetch(`${BASE}${path}`, {
            method: opts.method ?? 'GET',
            credentials: 'include',
            signal: opts.signal,
            headers: {
                ...(opts.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
                ...opts.headers,
            },
            body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
        });
    }
    catch (err) {
        if (opts.signal?.aborted) {
            throw err;
        }
        throw new ApiError({
            status: 0,
            code: 'network_error',
            message: '网络连接失败，请检查网络后重试',
        });
    }
    return readResponse(res);
}
async function readResponse(res) {
    if (res.status === 401) {
        clearSession();
        const redirect = encodeURIComponent(location.pathname + location.search);
        if (!location.pathname.startsWith('/login')) {
            location.assign(`/login?redirect=${redirect}`);
        }
        throw new ApiError({ status: 401, code: 'unauthorized', message: '请先登录' });
    }
    if (!res.ok) {
        let body = {
            status: res.status,
            code: 'internal_error',
            message: '操作失败，请重试',
        };
        try {
            const data = (await res.json());
            if (data.error) {
                body = {
                    status: res.status,
                    code: data.error.code ?? 'internal_error',
                    message: data.error.message ?? body.message,
                    requestId: data.error.requestId,
                    details: data.error.details,
                };
            }
        }
        catch {
            // 非 JSON 错误体保持默认
        }
        throw new ApiError(body);
    }
    if (res.status === 204) {
        return undefined;
    }
    return (await res.json());
}
/** 上传不设置 Content-Type，让浏览器自动补 multipart boundary。 */
export async function upload(path, file, signal) {
    let res;
    const form = new FormData();
    form.append('file', file);
    try {
        res = await fetch(`${BASE}${path}`, { method: 'POST', credentials: 'include', signal, body: form });
    }
    catch (err) {
        if (signal?.aborted)
            throw err;
        throw new ApiError({ status: 0, code: 'network_error', message: '网络连接失败，请检查网络后重试' });
    }
    return readResponse(res);
}
/** 解析后端字段级校验错误 details.fields */
export function fieldErrors(err) {
    if (err instanceof ApiError && err.details && typeof err.details === 'object') {
        const fields = err.details.fields;
        if (fields) {
            return fields;
        }
    }
    return {};
}
