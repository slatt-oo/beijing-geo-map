import { QueryClient, QueryObserver, skipToken } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BBox } from '../lib/bbox'
import { districtQuery, POI_LIMIT, poiDetailQuery, poisQuery } from './queries'

const box: BBox = [116.3, 39.9, 116.4, 40]
const emptyCollection = { type: 'FeatureCollection', features: [], truncated: false }

function stubFetch(body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(
    async () => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('poisQuery', () => {
  it('keeps all, none and a category list in separate cache entries', () => {
    const keys = [poisQuery(box, undefined), poisQuery(box, null), poisQuery(box, ['park'])].map((q) =>
      JSON.stringify(q.queryKey),
    )
    expect(new Set(keys).size).toBe(3)
  })

  it('skips the request without a bbox or when every category is hidden', () => {
    expect(poisQuery(null, undefined).queryFn).toBe(skipToken)
    expect(poisQuery(box, null).queryFn).toBe(skipToken)
  })

  it('requests the bbox, the categories and the fixed limit', async () => {
    const fetchMock = stubFetch(emptyCollection)

    await new QueryClient().fetchQuery(poisQuery(box, ['park', 'university']))

    expect(fetchMock.mock.calls[0][0]).toBe(
      `/api/v1/pois?bbox=116.3%2C39.9%2C116.4%2C40&categories=park%2Cuniversity&limit=${POI_LIMIT}`,
    )
  })

  it('does not fall back to the previous result when every category is hidden', async () => {
    stubFetch(emptyCollection)
    const observer = new QueryObserver(new QueryClient(), poisQuery(box, undefined))
    const unsubscribe = observer.subscribe(() => {})
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true))

    observer.setOptions(poisQuery(box, null))

    expect(observer.getCurrentResult().data).toBeUndefined()
    unsubscribe()
  })
})

describe('detail queries', () => {
  it('unwrap the data envelope', async () => {
    stubFetch({ data: { id: 42, name: '北京大学' } })

    await expect(new QueryClient().fetchQuery(poiDetailQuery(42))).resolves.toEqual({ id: 42, name: '北京大学' })
  })

  it('skip the request without an id', () => {
    expect(poiDetailQuery(null).queryFn).toBe(skipToken)
    expect(districtQuery(null).queryFn).toBe(skipToken)
  })
})
