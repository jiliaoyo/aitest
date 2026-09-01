import { clearSession } from '@/app/session'

export interface ApiErrorBody {
  status: number
  code: string
  message: string
  requestId?: string
  details?: Record<string, unknown>
}

// ApiError 携带后端稳定错误码；页面分支只判断 code，提示优先使用后端 message。
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string
  readonly details?: Record<string, unknown>

  constructor(body: ApiErrorBody) {
    super(body.message)
    this.name = 'ApiError'
    this.status = body.status
    this.code = body.code
    this.requestId = body.requestId
    this.details = body.details
  }
}

const BASE = '/api/v1'

interface RequestOptions {
  method?: string
  body?: unknown
  signal?: AbortSignal
  headers?: Record<string, string>
}

// request 是唯一的 HTTP 入口：统一 Cookie、错误结构与 401 处理。
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  let res: Response
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
    })
  } catch (err) {
    if (opts.signal?.aborted) {
      throw err
    }
    throw new ApiError({
      status: 0,
      code: 'network_error',
      message: '网络连接失败，请检查网络后重试',
    })
  }

  if (res.status === 401) {
    clearSession()
    const redirect = encodeURIComponent(location.pathname + location.search)
    if (!location.pathname.startsWith('/login')) {
      location.assign(`/login?redirect=${redirect}`)
    }
    throw new ApiError({ status: 401, code: 'unauthorized', message: '请先登录' })
  }

  if (!res.ok) {
    let body: ApiErrorBody = {
      status: res.status,
      code: 'internal_error',
      message: '操作失败，请重试',
    }
    try {
      const data = (await res.json()) as { error?: Partial<ApiErrorBody> }
      if (data.error) {
        body = {
          status: res.status,
          code: data.error.code ?? 'internal_error',
          message: data.error.message ?? body.message,
          requestId: data.error.requestId,
          details: data.error.details,
        }
      }
    } catch {
      // 非 JSON 错误体保持默认
    }
    throw new ApiError(body)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

/** 解析后端字段级校验错误 details.fields */
export function fieldErrors(err: unknown): Record<string, string> {
  if (err instanceof ApiError && err.details && typeof err.details === 'object') {
    const fields = (err.details as { fields?: Record<string, string> }).fields
    if (fields) {
      return fields
    }
  }
  return {}
}
