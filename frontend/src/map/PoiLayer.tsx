import type { GeoJSONSource, Map as MaplibreMap } from 'maplibre-gl'
import { useEffect, useMemo, useState } from 'react'
import { useCategories, usePoisInBBox } from '../api/queries'
import { toData, toDisplay } from '../config'
import { viewToDataBBox, type BBox } from '../lib/bbox'
import { categoriesParam, categoryColorExpression } from '../lib/categories'
import { transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const MOVE_DEBOUNCE_MS = 300

function currentBBox(map: MaplibreMap): BBox {
  const b = map.getBounds()
  return viewToDataBBox({ west: b.getWest(), south: b.getSouth(), east: b.getEast(), north: b.getNorth() }, toData)
}

export function PoiLayer() {
  const map = useMap()
  const [bbox, setBbox] = useState<BBox>(() => currentBBox(map))
  const { data: categories } = useCategories()
  const hidden = useAppStore((s) => s.hiddenCategories)

  const codes = useMemo(() => categories?.map((c) => c.code) ?? [], [categories])
  const requested = categoriesParam(codes, hidden)
  const { data } = usePoisInBBox(bbox, requested)

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const moveEnd = map.on('moveend', () => {
      clearTimeout(timer)
      timer = setTimeout(() => setBbox(currentBBox(map)), MOVE_DEBOUNCE_MS)
    })
    return () => {
      clearTimeout(timer)
      moveEnd.unsubscribe()
    }
  }, [map])

  useEffect(() => {
    if (categories) map.setPaintProperty(LAYER.poiPoint, 'circle-color', categoryColorExpression(categories))
  }, [map, categories])

  const display = useMemo(() => (data ? transformCollection(data, toDisplay) : EMPTY_COLLECTION), [data])

  useEffect(() => {
    map.getSource<GeoJSONSource>(SOURCE.pois)?.setData(display)
  }, [map, display])

  if (!data?.truncated) return null
  return (
    <div className="pointer-events-none absolute left-1/2 top-4 z-10 -translate-x-1/2 rounded-full bg-amber-50 px-4 py-1.5 text-sm text-amber-800 shadow ring-1 ring-amber-200">
      当前范围数据过多，请放大地图
    </div>
  )
}
