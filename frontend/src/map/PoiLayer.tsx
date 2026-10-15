import type { GeoJSONSource, Map as MaplibreMap } from 'maplibre-gl'
import { useEffect, useMemo, useState } from 'react'
import { useCategories, usePoisInBBox } from '../api/queries'
import { toData, toDisplay } from '../config'
import { useDebounce } from '../hooks/useDebounce'
import { viewToDataBBox, type BBox } from '../lib/bbox'
import { categoriesParam, categoryColorExpression } from '../lib/categories'
import { transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const MOVE_DEBOUNCE_MS = 300

// 本地请求通常几十毫秒就返回，只有慢的时候才提示，避免每次移动地图都闪一下。
const LOADING_HINT_DELAY_MS = 400

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
  const { data, isFetching } = usePoisInBBox(bbox, requested)
  const slow = useDebounce(isFetching, LOADING_HINT_DELAY_MS) && isFetching

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

  const hint = slow
    ? { text: '正在加载 POI…', tone: 'bg-white/95 text-slate-700 ring-slate-200' }
    : data?.truncated
      ? { text: '当前范围数据过多，请放大地图', tone: 'bg-amber-50 text-amber-800 ring-amber-200' }
      : null
  // 读屏软件只朗读已经存在的 live region 里的变化，所以这个元素一直挂着，只切换文字。
  return (
    <div
      role="status"
      className={
        hint
          ? `pointer-events-none absolute left-1/2 top-4 z-10 -translate-x-1/2 rounded-full px-4 py-1.5 text-sm shadow ring-1 ${hint.tone}`
          : 'sr-only'
      }
    >
      {hint?.text}
    </div>
  )
}
