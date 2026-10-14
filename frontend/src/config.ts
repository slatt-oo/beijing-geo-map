import type { BBox } from './lib/bbox'
import { createCoordTransform } from './lib/coord'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './lib/tiles'

export const tileUrls = expandTileUrl(import.meta.env.VITE_TILE_URL || DEFAULT_TILE_URL)

export const { toDisplay, toData } = createCoordTransform(parseTileCrs(import.meta.env.VITE_TILE_CRS))

// 北京市 16 个区边界的外包框（WGS-84），作为初始视野。
export const BEIJING_BBOX: BBox = [115.42, 39.44, 117.51, 41.06]

// 北京市外扩约 1° 的范围（WGS-84）。地图不能移出这个范围；后端导入器用同一个范围过滤 POI。
export const REGION_BBOX: BBox = [114.4, 38.4, 118.5, 42.1]
