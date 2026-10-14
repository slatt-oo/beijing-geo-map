import type { LngLat } from './coord'

// 依次为 minLng, minLat, maxLng, maxLat。
export type BBox = [number, number, number, number]

export type ViewBounds = { west: number; south: number; east: number; north: number }

// 向外取整到 0.001°（约 100 米）：视野只移动一点时请求参数不变，可以直接命中缓存。
const STEP = 1000

// 地图视野是底图坐标系，四个角都要换算成数据坐标系再取外包框，因为偏移量随位置变化。
export function viewToDataBBox(view: ViewBounds, toData: (p: LngLat) => LngLat): BBox {
  const corners: LngLat[] = [
    [view.west, view.south],
    [view.west, view.north],
    [view.east, view.south],
    [view.east, view.north],
  ]
  const converted = corners.map(toData)
  const lngs = converted.map(([lng]) => lng)
  const lats = converted.map(([, lat]) => lat)
  return [
    clamp(Math.floor(Math.min(...lngs) * STEP) / STEP, -180, 180),
    clamp(Math.floor(Math.min(...lats) * STEP) / STEP, -90, 90),
    clamp(Math.ceil(Math.max(...lngs) * STEP) / STEP, -180, 180),
    clamp(Math.ceil(Math.max(...lats) * STEP) / STEP, -90, 90),
  ]
}

export function bboxParam(box: BBox): string {
  return box.join(',')
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}
