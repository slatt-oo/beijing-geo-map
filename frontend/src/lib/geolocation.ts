import type { MyLocation } from '../store/useAppStore'

export const GEOLOCATION_OPTIONS: PositionOptions = { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 }

// 浏览器定位得到的是 WGS-84 坐标，可以直接用于查询。不支持定位或定位失败时 reject，由调用方转入点选模式。
export function requestPosition(geolocation: Geolocation | undefined): Promise<MyLocation> {
  return new Promise((resolve, reject) => {
    if (!geolocation) {
      reject(new Error('浏览器不支持定位'))
      return
    }
    geolocation.getCurrentPosition(
      ({ coords }) => resolve({ lng: coords.longitude, lat: coords.latitude, accuracy: coords.accuracy }),
      (error) => reject(new Error(error.message)),
      GEOLOCATION_OPTIONS,
    )
  })
}
