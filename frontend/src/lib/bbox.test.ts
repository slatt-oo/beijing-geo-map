import { describe, expect, it } from 'vitest'
import { bboxParam, viewToDataBBox } from './bbox'
import type { LngLat } from './coord'

const identity = ([lng, lat]: LngLat): LngLat => [lng, lat]

describe('viewToDataBBox', () => {
  it('rounds outward to 0.001 degrees', () => {
    const box = viewToDataBBox({ west: 116.30049, south: 39.90001, east: 116.40001, north: 39.99999 }, identity)
    expect(box).toEqual([116.3, 39.9, 116.401, 40])
  })

  it('takes the extent of all four converted corners', () => {
    // 纬度越高向西偏得越多：最小经度来自西北角而不是西南角。
    const skew = ([lng, lat]: LngLat): LngLat => [lng - (lat - 39) * 0.5, lat]
    const box = viewToDataBBox({ west: 116, south: 39, east: 117, north: 40 }, skew)
    expect(box).toEqual([115.5, 39, 117, 40])
  })

  it('clamps to valid longitude and latitude', () => {
    const box = viewToDataBBox({ west: -200, south: -95, east: 200, north: 95 }, identity)
    expect(box).toEqual([-180, -90, 180, 90])
  })
})

describe('bboxParam', () => {
  it('formats the box as minLng,minLat,maxLng,maxLat', () => {
    expect(bboxParam([116.3, 39.9, 116.401, 40])).toBe('116.3,39.9,116.401,40')
  })
})
