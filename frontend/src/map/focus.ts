import type { Map as MaplibreMap } from 'maplibre-gl'
import { toDisplay } from '../config'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { FOCUS_ZOOM } from './padding'

// 搜索结果和附近列表共用：地图飞到 POI（WGS-84 坐标）并打开它的详情弹窗。
export function focusPoi(map: MaplibreMap, id: number, location: LngLat): void {
  map.flyTo({ center: toDisplay(location), zoom: FOCUS_ZOOM })
  useAppStore.getState().openPoi(id)
}
