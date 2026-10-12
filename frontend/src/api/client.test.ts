import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiGet, buildUrl } from './client'

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
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Bad Gateway</html>', { status: 502 })))

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 502, code: 'HTTP_ERROR' })
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
