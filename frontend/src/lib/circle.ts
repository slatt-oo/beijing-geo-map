import { circle } from '@turf/circle'
import type { Feature, Polygon } from 'geojson'
import type { LngLat } from './coord'

// 64 边形近似圆，按球面距离计算，所以在 WGS-84 下生成，交给地图前再统一经过 toDisplay。
export function circlePolygon(center: LngLat, radiusM: number): Feature<Polygon> {
  return circle(center, radiusM, { steps: 64, units: 'meters' })
}
