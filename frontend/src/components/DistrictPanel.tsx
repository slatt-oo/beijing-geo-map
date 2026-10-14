import { useCategories, useDistrict } from '../api/queries'
import { FALLBACK_POI_COLOR } from '../lib/categories'
import { formatArea } from '../lib/format'
import { useAppStore } from '../store/useAppStore'

export function DistrictPanel() {
  const adcode = useAppStore((s) => s.selectedDistrict)
  const selectDistrict = useAppStore((s) => s.selectDistrict)
  const { data: detail, isError } = useDistrict(adcode)
  const { data: categories } = useCategories()

  if (adcode === null) return null
  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  return (
    <aside className="absolute left-4 top-4 z-10 w-72 rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200">
      <div className="flex items-start justify-between gap-2">
        <h2 className="text-lg font-semibold text-slate-900">{detail?.name ?? (isError ? '加载失败' : '加载中…')}</h2>
        <button
          type="button"
          aria-label="关闭区详情"
          onClick={() => selectDistrict(null)}
          className="rounded px-1.5 text-lg leading-none text-slate-500 hover:bg-slate-100 hover:text-slate-800"
        >
          ×
        </button>
      </div>
      {detail && (
        <>
          <p className="mt-1 text-sm text-slate-500">面积 {formatArea(detail.area_km2)}</p>
          <ul className="mt-3 space-y-1.5">
            {detail.poi_counts.map((c) => (
              <li key={c.code} className="flex items-center gap-2 text-sm">
                <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: colors.get(c.code) ?? FALLBACK_POI_COLOR }} />
                <span className="flex-1 text-slate-700">{c.name}</span>
                <span className="tabular-nums text-slate-900">{c.count}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </aside>
  )
}
