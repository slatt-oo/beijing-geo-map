import { keepPreviousData, queryOptions, skipToken, useQuery } from '@tanstack/react-query'
import { bboxParam, type BBox } from '../lib/bbox'
import type { MyLocation, Nearby } from '../store/useAppStore'
import { apiGet, apiGetData } from './client'
import type {
  Category,
  DistrictCollection,
  DistrictDetail,
  DistrictRef,
  NearbyCollection,
  POICollection,
  POIDetail,
  SearchHit,
} from './types'

// 当前快照约 4100 个 POI：按接口上限请求时，整个北京也不会被截断，聚合数字才准确。
export const POI_LIMIT = 5000

export const SEARCH_LIMIT = 10

// 附近接口的上限。结果正好这么多时，列表提示只显示了最近的这些。
export const NEARBY_LIMIT = 100

export function districtsQuery() {
  return queryOptions({
    queryKey: ['districts'],
    queryFn: ({ signal }) => apiGet<DistrictCollection>('/districts', undefined, signal),
    staleTime: Infinity,
  })
}

export function districtQuery(adcode: string | null) {
  return queryOptions({
    queryKey: ['district', adcode],
    queryFn:
      adcode === null
        ? skipToken
        : ({ signal }) => apiGetData<DistrictDetail>(`/districts/${adcode}`, undefined, signal),
    staleTime: Infinity,
  })
}

export function categoriesQuery() {
  return queryOptions({
    queryKey: ['poi-categories'],
    queryFn: ({ signal }) => apiGetData<Category[]>('/poi-categories', undefined, signal),
    staleTime: Infinity,
  })
}

// categories 为 undefined 表示全部分类；为 null 表示分类全部关闭：此时不请求，也不沿用上一次的结果。
export function poisQuery(bbox: BBox | null, categories: string[] | undefined | null) {
  return queryOptions({
    queryKey: ['pois', bbox && bboxParam(bbox), categories === null ? 'none' : (categories ?? 'all')],
    queryFn:
      bbox === null || categories === null
        ? skipToken
        : ({ signal }) => apiGet<POICollection>('/pois', { bbox: bboxParam(bbox), categories, limit: POI_LIMIT }, signal),
    placeholderData: categories === null ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
    // 全市视野下稍一平移 bbox 就会变，每条缓存都有约 4000 个要素，不必按默认在内存里保留 5 分钟。
    gcTime: 60 * 1000,
  })
}

export function poiDetailQuery(id: number | null) {
  return queryOptions({
    queryKey: ['poi', id],
    queryFn: id === null ? skipToken : ({ signal }) => apiGetData<POIDetail>(`/pois/${id}`, undefined, signal),
    staleTime: 5 * 60 * 1000,
  })
}

// q 为空字符串时不请求；输入过程中保留上一次的结果，避免下拉框闪烁。
export function searchQuery(q: string) {
  return queryOptions({
    queryKey: ['search', q],
    queryFn:
      q === '' ? skipToken : ({ signal }) => apiGetData<SearchHit[]>('/pois/search', { q, limit: SEARCH_LIMIT }, signal),
    placeholderData: q === '' ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
  })
}

// categories 的含义与 poisQuery 相同。
export function nearbyQuery(nearby: Nearby | null, categories: string[] | undefined | null) {
  return queryOptions({
    queryKey: [
      'nearby',
      nearby && [...nearby.center, nearby.radius],
      categories === null ? 'none' : (categories ?? 'all'),
    ],
    queryFn:
      nearby === null || categories === null
        ? skipToken
        : ({ signal }) =>
            apiGet<NearbyCollection>(
              '/pois/nearby',
              { lng: nearby.center[0], lat: nearby.center[1], radius: nearby.radius, categories, limit: NEARBY_LIMIT },
              signal,
            ),
    placeholderData: categories === null ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
  })
}

export function locateQuery(location: MyLocation | null) {
  return queryOptions({
    queryKey: ['locate', location && [location.lng, location.lat]],
    queryFn:
      location === null
        ? skipToken
        : ({ signal }) =>
            apiGetData<DistrictRef | null>('/districts/locate', { lng: location.lng, lat: location.lat }, signal),
    staleTime: Infinity,
  })
}

export function useDistricts() {
  return useQuery(districtsQuery())
}

export function useDistrict(adcode: string | null) {
  return useQuery(districtQuery(adcode))
}

export function useCategories() {
  return useQuery(categoriesQuery())
}

export function usePoisInBBox(bbox: BBox | null, categories: string[] | undefined | null) {
  return useQuery(poisQuery(bbox, categories))
}

export function usePoiDetail(id: number | null) {
  return useQuery(poiDetailQuery(id))
}

export function useSearch(q: string) {
  return useQuery(searchQuery(q))
}

export function useNearby(nearby: Nearby | null, categories: string[] | undefined | null) {
  return useQuery(nearbyQuery(nearby, categories))
}

export function useLocateDistrict(location: MyLocation | null) {
  return useQuery(locateQuery(location))
}
