import { QueryClient, QueryObserver, skipToken } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BBox } from '../lib/bbox'
import { districtQuery, locateQuery, NEARBY_LIMIT, nearbyQuery, POI_LIMIT, poiDetailQuery, poisQuery, SEARCH_LIMIT, searchQuery } from './queries'

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

describe('searchQuery', () => {
  it('skips an empty query', () => {
    expect(searchQuery('').queryFn).toBe(skipToken)
  })

  it('drops the previous hits when the query is cleared', async () => {
    stubFetch({ data: [] })
    const observer = new QueryObserver(new QueryClient(), searchQuery('北京'))
    const unsubscribe = observer.subscribe(() => {})
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true))

    observer.setOptions(searchQuery(''))

    expect(observer.getCurrentResult().data).toBeUndefined()
    unsubscribe()
  })

  it('requests the query with the dropdown limit and unwraps the hits', async () => {
    const fetchMock = stubFetch({ data: [{ id: 3, name: '北京大学', category: 'university', district_name: '海淀区', location: [116.3, 39.99] }] })

    const hits = await new QueryClient().fetchQuery(searchQuery('北京大学'))

    expect(fetchMock.mock.calls[0][0]).toBe(`/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC%E5%A4%A7%E5%AD%A6&limit=${SEARCH_LIMIT}`)
    expect(hits.map((h) => h.name)).toEqual(['北京大学'])
  })
})

describe('nearbyQuery', () => {
  const nearby = { center: [116.397, 39.908] as [number, number], radius: 1000 }

  it('keys on center, radius and categories', () => {
    const keys = [
      nearbyQuery(nearby, undefined),
      nearbyQuery({ ...nearby, radius: 2000 }, undefined),
      nearbyQuery(nearby, ['park']),
      nearbyQuery(nearby, null),
    ].map((q) => JSON.stringify(q.queryKey))
    expect(new Set(keys).size).toBe(4)
  })

  it('skips the request without a center or when every category is hidden', () => {
    expect(nearbyQuery(null, undefined).queryFn).toBe(skipToken)
    expect(nearbyQuery(nearby, null).queryFn).toBe(skipToken)
  })

  it('keeps the previous list while the radius changes, but not when the center moves', async () => {
    stubFetch({ type: 'FeatureCollection', features: [], center: nearby.center, radius: 1000 })
    const observer = new QueryObserver(new QueryClient(), nearbyQuery(nearby, undefined))
    const unsubscribe = observer.subscribe(() => {})
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true))

    observer.setOptions(nearbyQuery({ ...nearby, radius: 2000 }, undefined))
    expect(observer.getCurrentResult().isPlaceholderData).toBe(true)

    observer.setOptions(nearbyQuery({ center: [116.3, 39.99], radius: 2000 }, undefined))
    expect(observer.getCurrentResult().data).toBeUndefined()
    unsubscribe()
  })

  it('requests the center, radius, categories and the endpoint limit', async () => {
    const fetchMock = stubFetch({ type: 'FeatureCollection', features: [], center: nearby.center, radius: 1000 })

    await new QueryClient().fetchQuery(nearbyQuery(nearby, ['park']))

    expect(fetchMock.mock.calls[0][0]).toBe(
      `/api/v1/pois/nearby?lng=116.397&lat=39.908&radius=1000&categories=park&limit=${NEARBY_LIMIT}`,
    )
  })
})

describe('locateQuery', () => {
  it('skips the request without a location', () => {
    expect(locateQuery(null).queryFn).toBe(skipToken)
  })

  it('requests the location and ignores accuracy in the cache key', async () => {
    const fetchMock = stubFetch({ data: { adcode: '110108', name: '海淀区' } })

    await new QueryClient().fetchQuery(locateQuery({ lng: 116.31, lat: 39.99, accuracy: 30 }))

    expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/districts/locate?lng=116.31&lat=39.99')
    expect(locateQuery({ lng: 116.31, lat: 39.99, accuracy: 30 }).queryKey).toEqual(
      locateQuery({ lng: 116.31, lat: 39.99, accuracy: null }).queryKey,
    )
  })

  it('returns null outside Beijing', async () => {
    stubFetch({ data: null })

    await expect(new QueryClient().fetchQuery(locateQuery({ lng: 116.7, lat: 39.5, accuracy: null }))).resolves.toBeNull()
  })
})
