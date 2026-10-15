import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiGet, apiGetData, buildUrl, errorMessage, isRetryable, shouldRetry } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('buildUrl', () => {
  it('returns the bare path under /api/v1 when there are no params', () => {
    expect(buildUrl('/health')).toBe('/api/v1/health')
  })

  it('encodes params and skips null, undefined and empty arrays', () => {
    const url = buildUrl('/pois/search', { q: '北京', limit: 10, categories: [], radius: undefined, lng: null })
    expect(url).toBe('/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC&limit=10')
  })

  it('joins array params with commas', () => {
    expect(buildUrl('/pois', { categories: ['park', 'university'] })).toBe(
      '/api/v1/pois?categories=park%2Cuniversity',
    )
  })
})

describe('apiGet', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('requests the built URL and returns the parsed body', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse(200, { data: { status: 'ok', db: 'ok' } }))
    vi.stubGlobal('fetch', fetchMock)
    const controller = new AbortController()

    await expect(apiGet('/health', { verbose: 1 }, controller.signal)).resolves.toEqual({
      data: { status: 'ok', db: 'ok' },
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/health?verbose=1',
      expect.objectContaining({ signal: controller.signal }),
    )
  })

  it('throws ApiError carrying the server error code and message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(400, { error: { code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' } })),
    )

    const err = await apiGet('/districts/locate').catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 400, code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' })
  })

  it('falls back to HTTP_ERROR when the error body is not the standard format', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Internal Server Error</html>', { status: 500 })))

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 500, code: 'HTTP_ERROR' })
  })

  it.each([502, 503, 504])('reports an unreachable backend for an HTML %i from nginx', async (status) => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Bad Gateway</html>', { status })))

    await expect(apiGet('/health')).rejects.toMatchObject({
      status,
      code: 'BACKEND_UNAVAILABLE',
      message: '后端服务未启动或暂时不可用',
    })
  })

  it('accepts GeoJSON responses', async () => {
    const body = { type: 'FeatureCollection', features: [] }
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/geo+json' } }),
      ),
    )

    await expect(apiGet('/districts')).resolves.toEqual(body)
  })

  it('rejects a successful response that is not JSON', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response('<!doctype html><title>北京地图 Demo</title>', { status: 200 })),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 200, code: 'BAD_RESPONSE' })
  })

  it('rejects a JSON content type whose body does not parse', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response('{"data":', { status: 200, headers: { 'Content-Type': 'application/json' } })),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 200, code: 'BAD_RESPONSE' })
  })

  it('maps network failures to a NETWORK ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('fetch failed')
      }),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 0, code: 'NETWORK' })
  })

  function signalAwareFetch() {
    return vi.fn<typeof fetch>(async (_input, init) => {
      init?.signal?.throwIfAborted()
      return jsonResponse(200, { data: null })
    })
  }

  it('rethrows a plain abort unchanged', async () => {
    vi.stubGlobal('fetch', signalAwareFetch())
    const controller = new AbortController()
    controller.abort()

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('rethrows an abort with a custom reason unchanged', async () => {
    vi.stubGlobal('fetch', signalAwareFetch())
    const controller = new AbortController()
    const reason = new Error('navigated away')
    controller.abort(reason)

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toBe(reason)
  })

  it('rethrows an abort that happens while reading an error body', async () => {
    const controller = new AbortController()
    const reason = new Error('navigated away')
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(async () => {
        controller.abort(reason)
        return jsonResponse(500, { error: { code: 'INTERNAL', message: '服务器内部错误' } })
      }),
    )

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toBe(reason)
  })
})

describe('errorMessage', () => {
  it('uses the server message of an ApiError', () => {
    expect(errorMessage(new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe('POI 不存在')
  })

  it('falls back to a generic message for anything else', () => {
    expect(errorMessage(new TypeError('boom'))).toBe('请求失败，请稍后重试')
  })
})

describe('isRetryable', () => {
  it('treats network, server and unrecognised responses as worth retrying', () => {
    expect(isRetryable(new ApiError(0, 'NETWORK', '网络错误'))).toBe(true)
    expect(isRetryable(new ApiError(502, 'BACKEND_UNAVAILABLE', '后端服务未启动'))).toBe(true)
    expect(isRetryable(new ApiError(200, 'BAD_RESPONSE', '无法识别'))).toBe(true)
    expect(isRetryable(new TypeError('boom'))).toBe(true)
  })

  it('does not retry client errors', () => {
    expect(isRetryable(new ApiError(400, 'INVALID_PARAM', 'bbox 格式错误'))).toBe(false)
    expect(isRetryable(new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe(false)
  })
})

describe('shouldRetry', () => {
  it('retries a network or server error once', () => {
    expect(shouldRetry(0, new ApiError(0, 'NETWORK', '网络错误'))).toBe(true)
    expect(shouldRetry(0, new ApiError(500, 'INTERNAL', '服务器内部错误'))).toBe(true)
    expect(shouldRetry(1, new ApiError(500, 'INTERNAL', '服务器内部错误'))).toBe(false)
  })

  it('does not retry client errors', () => {
    expect(shouldRetry(0, new ApiError(400, 'INVALID_PARAM', 'bbox 格式错误'))).toBe(false)
    expect(shouldRetry(0, new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe(false)
  })
})

describe('apiGetData', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('unwraps the data envelope', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { data: { adcode: '110108', name: '海淀区' } })))

    await expect(apiGetData('/districts/locate', { lng: 116.3, lat: 39.99 })).resolves.toEqual({
      adcode: '110108',
      name: '海淀区',
    })
  })

  it('keeps a null result, which some endpoints use for "not found here"', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { data: null })))

    await expect(apiGetData('/districts/locate', { lng: 116.7, lat: 39.5 })).resolves.toBeNull()
  })
})
