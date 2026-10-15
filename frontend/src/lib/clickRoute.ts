import type { AppMode } from '../store/useAppStore'
import type { LngLat } from './coord'

export type PoiHit = { kind: 'cluster'; clusterId: number; center: LngLat } | { kind: 'point'; id: number }

export type ClickAction =
  | { type: 'pickLocation' }
  | { type: 'expandCluster'; clusterId: number; center: LngLat }
  | { type: 'openPoi'; id: number }
  | { type: 'selectDistrict'; adcode: string | null }

// 点击的优先级：点选位置模式下任何点击都用来设置位置；否则 POI（聚合或单点）优先于它下面的区。
export function routeClick(mode: AppMode, poi: PoiHit | undefined, adcode: string | undefined): ClickAction {
  if (mode === 'pickLocation') return { type: 'pickLocation' }
  if (poi?.kind === 'cluster') return { type: 'expandCluster', clusterId: poi.clusterId, center: poi.center }
  if (poi) return { type: 'openPoi', id: poi.id }
  return { type: 'selectDistrict', adcode: adcode ?? null }
}
