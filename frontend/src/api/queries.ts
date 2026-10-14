import { keepPreviousData, queryOptions, skipToken, useQuery } from '@tanstack/react-query'
import { bboxParam, type BBox } from '../lib/bbox'
import { apiGet } from './client'
import type { Category, DataResponse, DistrictCollection, DistrictDetail, POICollection, POIDetail } from './types'

// 当前快照约 4100 个 POI：按接口上限请求时，整个北京也不会被截断，聚合数字才准确。
export const POI_LIMIT = 5000

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
        : ({ signal }) => apiGet<DataResponse<DistrictDetail>>(`/districts/${adcode}`, undefined, signal).then((r) => r.data),
    staleTime: Infinity,
  })
}

export function categoriesQuery() {
  return queryOptions({
    queryKey: ['poi-categories'],
    queryFn: ({ signal }) => apiGet<DataResponse<Category[]>>('/poi-categories', undefined, signal).then((r) => r.data),
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
  })
}

export function poiDetailQuery(id: number | null) {
  return queryOptions({
    queryKey: ['poi', id],
    queryFn:
      id === null
        ? skipToken
        : ({ signal }) => apiGet<DataResponse<POIDetail>>(`/pois/${id}`, undefined, signal).then((r) => r.data),
    staleTime: 5 * 60 * 1000,
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
