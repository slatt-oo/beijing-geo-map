import { beforeEach, describe, expect, it, vi } from 'vitest'
import { LOCATE_GIVE_UP_MS, locate } from './locate'
import { useAppStore } from './useAppStore'

type Request = { success: PositionCallback; failure?: PositionErrorCallback | null }

// 假的 Geolocation：记下每次请求的回调，由测试决定何时返回、返回什么。
function pendingGeolocation(requests: Request[]): Geolocation {
  return {
    getCurrentPosition: (success, failure) => {
      requests.push({ success, failure })
    },
    watchPosition: vi.fn(),
    clearWatch: vi.fn(),
  }
}

const position = { coords: { longitude: 116.31, latitude: 39.99, accuracy: 25 } } as GeolocationPosition
const denied = { code: 1, message: 'User denied Geolocation' } as GeolocationPositionError
const initial = useAppStore.getState()
const state = () => useAppStore.getState()

describe('locate', () => {
  beforeEach(() => {
    useAppStore.setState(initial, true)
  })

  it('stores the position and stops locating', async () => {
    const requests: Request[] = []
    const done = locate(pendingGeolocation(requests))
    expect(state().locateRequest).not.toBeNull()

    requests[0].success(position)
    await done

    expect(state().myLocation).toEqual({ lng: 116.31, lat: 39.99, accuracy: 25 })
    expect(state().locateRequest).toBeNull()
  })

  it('switches to picking on the map when locating fails', async () => {
    const requests: Request[] = []
    const done = locate(pendingGeolocation(requests))

    requests[0].failure?.(denied)
    await done

    expect(state().mode).toBe('pickLocation')
    expect(state().locateRequest).toBeNull()
  })

  it('ignores another request while one is in flight', async () => {
    const requests: Request[] = []
    const geolocation = pendingGeolocation(requests)
    const first = locate(geolocation)

    await locate(geolocation)

    expect(requests).toHaveLength(1)
    requests[0].failure?.(denied)
    await first
  })

  it('ignores a late failure once the user has picked a location', async () => {
    const requests: Request[] = []
    const done = locate(pendingGeolocation(requests))

    state().setMyLocation({ lng: 116.4, lat: 39.9, accuracy: null })
    requests[0].failure?.(denied)
    await done

    expect(state().mode).toBe('normal')
    expect(state().myLocation).toEqual({ lng: 116.4, lat: 39.9, accuracy: null })
  })

  it('ignores a late position once picking was cancelled', async () => {
    state().startPickLocation()
    const requests: Request[] = []
    const done = locate(pendingGeolocation(requests))

    state().cancelPickLocation()
    requests[0].success(position)
    await done

    expect(state().myLocation).toBeNull()
  })

  it('gives up when the browser never answers', async () => {
    vi.useFakeTimers()
    try {
      const done = locate(pendingGeolocation([]))

      await vi.advanceTimersByTimeAsync(LOCATE_GIVE_UP_MS)
      await done

      expect(state().mode).toBe('pickLocation')
    } finally {
      vi.useRealTimers()
    }
  })

  it('switches to picking when the browser has no geolocation', async () => {
    await locate(undefined)

    expect(state().mode).toBe('pickLocation')
  })
})
