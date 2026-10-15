import type { FeatureCollection, MultiPolygon, Point } from 'geojson'
import type { LngLat } from '../lib/coord'

// 与后端 internal/service/types.go 的 JSON 结构一一对应，坐标都是 WGS-84。

export type DataResponse<T> = { data: T }

export type Category = { code: string; name: string; color: string }

export type DistrictProperties = {
  adcode: string
  name: string
  area_km2: number
  poi_count: number
  center: LngLat
}

export type DistrictCollection = FeatureCollection<MultiPolygon, DistrictProperties>

export type CategoryCount = { code: string; name: string; count: number }

export type DistrictDetail = {
  adcode: string
  name: string
  area_km2: number
  center: LngLat
  poi_counts: CategoryCount[]
}

export type POIProperties = { id: number; name: string; category: string }

export type POICollection = FeatureCollection<Point, POIProperties> & { truncated: boolean }

export type POIDetail = {
  id: number
  name: string
  name_en: string | null
  category: Category
  district: { adcode: string; name: string } | null
  location: LngLat
  address: string | null
  tags: Record<string, string>
  osm_url: string
}
