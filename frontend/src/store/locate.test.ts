import { beforeEach, describe, expect, it, vi } from 'vitest'
import { locate } from './locate'
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
    expect(state().locating).toBe(true)

    requests[0].success(position)
    await done

    expect(state().myLocation).toEqual({ lng: 116.31, lat: 39.99, accuracy: 25 })
    expect(state().locating).toBe(false)
  })

  it('switches to picking on the map when locating fails', async () => {
    const requests: Request[] = []
    const done = locate(pendingGeolocation(requests))

    requests[0].failure?.(denied)
    await done

    expect(state().mode).toBe('pickLocation')
    expect(state().locating).toBe(false)
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

  it('switches to picking when the browser has no geolocation', async () => {
    await locate(undefined)

    expect(state().mode).toBe('pickLocation')
  })
})
