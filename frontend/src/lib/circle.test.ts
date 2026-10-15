import { describe, expect, it } from 'vitest'
import { circlePolygon } from './circle'
import type { LngLat } from './coord'

// 球面距离（米），只用于检查生成的圆。
function haversine([lng1, lat1]: LngLat, [lng2, lat2]: LngLat): number {
  const rad = Math.PI / 180
  const a =
    Math.sin(((lat2 - lat1) * rad) / 2) ** 2 +
    Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(((lng2 - lng1) * rad) / 2) ** 2
  return 2 * 6371008.8 * Math.asin(Math.sqrt(a))
}

describe('circlePolygon', () => {
  it('builds a closed 64-gon whose vertices are radius metres from the center', () => {
    const center: LngLat = [116.397, 39.908]

    const ring = circlePolygon(center, 1000).geometry.coordinates[0]

    expect(ring).toHaveLength(65)
    expect(ring[64]).toEqual(ring[0])
    for (const vertex of ring) {
      expect(Math.abs(haversine(center, vertex as LngLat) - 1000)).toBeLessThan(1)
    }
  })
})
