export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export type QueryValue = string | number | readonly (string | number)[] | null | undefined

const API_BASE = '/api/v1'

export function buildUrl(path: string, params: Record<string, QueryValue> = {}): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined) continue
    if (Array.isArray(value)) {
      if (value.length > 0) search.set(key, value.join(','))
      continue
    }
    search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `${API_BASE}${path}?${qs}` : `${API_BASE}${path}`
}

export async function apiGet<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T> {
  let res: Response
  try {
    res = await fetch(buildUrl(path, params), { signal, headers: { Accept: 'application/json' } })
  } catch (err) {
    if (signal?.aborted) throw err
    throw new ApiError(0, 'NETWORK', '网络错误，请确认后端服务已启动')
  }
  if (!res.ok) {
    const apiError = await toApiError(res)
    signal?.throwIfAborted()
    throw apiError
  }
  if (!isJson(res)) throw badResponse(res)
  try {
    return (await res.json()) as T
  } catch (err) {
    if (signal?.aborted) throw err
    throw badResponse(res)
  }
}

// 除 GeoJSON 接口外，其他接口都把结果包在 {"data": ...} 里。
export async function apiGetData<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T> {
  return (await apiGet<{ data: T }>(path, params, signal)).data
}

export function errorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : '请求失败，请稍后重试'
}

// 4xx 是请求本身的问题，重试也不会成功；网络错误、5xx 和无法识别的响应可能是暂时的。
export function isRetryable(error: unknown): boolean {
  return !(error instanceof ApiError && error.status >= 400 && error.status < 500)
}

// 自动重试一次；4xx 不重试，否则错误提示会晚出现约 1 秒。
export function shouldRetry(failureCount: number, error: unknown): boolean {
  return isRetryable(error) && failureCount < 1
}

// 三个 GeoJSON 接口返回 application/geo+json，其余返回 application/json。
function isJson(res: Response): boolean {
  const mediaType = (res.headers.get('Content-Type') ?? '').split(';')[0].trim().toLowerCase()
  return mediaType === 'application/json' || mediaType.endsWith('+json')
}

function badResponse(res: Response): ApiError {
  return new ApiError(res.status, 'BAD_RESPONSE', '服务器返回了无法识别的内容')
}

// nginx 连不上后端时返回自己的 HTML 错误页，状态码是这几个。
const BACKEND_UNAVAILABLE_STATUSES = new Set([502, 503, 504])

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: { code?: unknown; message?: unknown } }
    const { code, message } = body.error ?? {}
    if (typeof code === 'string' && typeof message === 'string') {
      return new ApiError(res.status, code, message)
    }
  } catch {
    // 非 JSON 响应体（例如 nginx 的 502 页面）走下面的通用错误
  }
  if (BACKEND_UNAVAILABLE_STATUSES.has(res.status)) {
    return new ApiError(res.status, 'BACKEND_UNAVAILABLE', '后端服务未启动或暂时不可用')
  }
  return new ApiError(res.status, 'HTTP_ERROR', `请求失败（HTTP ${res.status}）`)
}
