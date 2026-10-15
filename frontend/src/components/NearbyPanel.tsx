import { useMemo } from 'react'
import { NEARBY_LIMIT, useCategories, useNearby } from '../api/queries'
import { categoriesParam, FALLBACK_POI_COLOR } from '../lib/categories'
import type { LngLat } from '../lib/coord'
import { formatDistance } from '../lib/format'
import { focusPoi } from '../map/focus'
import { useMap } from '../map/MapView'
import { NEARBY_RADII, useAppStore } from '../store/useAppStore'
import { CategoryDot } from './CategoryDot'

export function NearbyPanel() {
  const map = useMap()
  const nearby = useAppStore((s) => s.nearby)
  const hidden = useAppStore((s) => s.hiddenCategories)
  const setNearbyRadius = useAppStore((s) => s.setNearbyRadius)
  const closeNearby = useAppStore((s) => s.closeNearby)
  const { data: categories } = useCategories()
  const codes = useMemo(() => categories?.map((c) => c.code) ?? [], [categories])
  const requested = categoriesParam(codes, hidden)
  const { data, isPending, isError, isFetching, refetch } = useNearby(nearby, requested)

  if (!nearby) return null
  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  const features = data?.features ?? []
  return (
    <aside
      aria-label="附近查询"
      className="pointer-events-auto flex max-h-[calc(100vh-7rem)] flex-col rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200"
    >
      <div className="flex items-start justify-between gap-2">
        <h2 className="text-lg font-semibold text-slate-900">附近</h2>
        <button
          type="button"
          aria-label="关闭附近查询"
          onClick={closeNearby}
          className="rounded px-1.5 text-lg leading-none text-slate-500 hover:bg-slate-100 hover:text-slate-800"
        >
          ×
        </button>
      </div>
      <div role="group" aria-label="查询半径" className="mt-2 flex gap-1">
        {NEARBY_RADII.map((r) => (
          <button
            key={r}
            type="button"
            aria-pressed={nearby.radius === r}
            onClick={() => setNearbyRadius(r)}
            className={`flex-1 rounded px-2 py-1 text-sm ${
              nearby.radius === r ? 'bg-sky-600 text-white' : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
            }`}
          >
            {formatDistance(r)}
          </button>
        ))}
      </div>
      <p className="mt-2 flex items-center gap-2 text-xs text-slate-500">
        {requested === null
          ? '分类已全部关闭'
          : isPending || (isError && isFetching)
            ? '查询中…'
            : isError
              ? '查询失败'
              : features.length === 0
                ? '范围内没有地点'
                : features.length === NEARBY_LIMIT
                  ? `仅显示最近的 ${NEARBY_LIMIT} 个`
                  : `共 ${features.length} 个`}
        {requested !== null && isError && !isFetching && (
          <button type="button" onClick={() => void refetch()} className="text-sky-700 underline underline-offset-2">
            重试
          </button>
        )}
      </p>
      <ul className="mt-2 -mx-1 flex-1 overflow-y-auto">
        {features.map((f) => (
          <li key={f.properties.id}>
            <button
              type="button"
              onClick={() => focusPoi(map, f.properties.id, f.geometry.coordinates as LngLat)}
              className="flex w-full items-center gap-2 rounded px-1 py-1.5 text-left text-sm hover:bg-slate-100"
            >
              <CategoryDot color={colors.get(f.properties.category) ?? FALLBACK_POI_COLOR} />
              <span className="flex-1 truncate text-slate-800">{f.properties.name}</span>
              <span className="shrink-0 tabular-nums text-slate-500">{formatDistance(f.properties.distance_m)}</span>
            </button>
          </li>
        ))}
      </ul>
    </aside>
  )
}
