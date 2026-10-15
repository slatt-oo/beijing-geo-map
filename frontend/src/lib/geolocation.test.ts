import { describe, expect, it, vi } from 'vitest'
import { GEOLOCATION_OPTIONS, requestPosition } from './geolocation'

function fakeGeolocation(impl: Geolocation['getCurrentPosition']): Geolocation {
  return { getCurrentPosition: impl, watchPosition: vi.fn(), clearWatch: vi.fn() }
}

describe('requestPosition', () => {
  it('resolves with the WGS-84 position and its accuracy', async () => {
    const getCurrentPosition = vi.fn<Geolocation['getCurrentPosition']>((success) =>
      success({ coords: { longitude: 116.31, latitude: 39.99, accuracy: 25 } } as GeolocationPosition),
    )

    await expect(requestPosition(fakeGeolocation(getCurrentPosition))).resolves.toEqual({
      lng: 116.31,
      lat: 39.99,
      accuracy: 25,
    })
    expect(getCurrentPosition).toHaveBeenCalledWith(expect.any(Function), expect.any(Function), GEOLOCATION_OPTIONS)
  })

  it('asks for high accuracy with a 10 second timeout', () => {
    expect(GEOLOCATION_OPTIONS).toEqual({ enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 })
  })

  it('rejects when the browser refuses or fails', async () => {
    const geolocation = fakeGeolocation((_success, failure) =>
      failure?.({ code: 1, message: 'User denied Geolocation' } as GeolocationPositionError),
    )

    await expect(requestPosition(geolocation)).rejects.toThrow('User denied Geolocation')
  })

  it('rejects when the browser has no geolocation', async () => {
    await expect(requestPosition(undefined)).rejects.toThrow('浏览器不支持定位')
  })
})
