import type { POIDetail } from '../api/types'

export function PoiDetailCard({ poi }: { poi: POIDetail }) {
  return (
    <div className="space-y-1.5 pr-4 text-sm text-slate-700">
      <div>
        <h3 className="text-base font-semibold text-slate-900">{poi.name}</h3>
        {poi.name_en && <p className="text-xs text-slate-500">{poi.name_en}</p>}
      </div>
      <p className="flex items-center gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: poi.category.color }} />
        {poi.category.name}
      </p>
      <p>所在区：{poi.district ? poi.district.name : '不在区界数据范围内'}</p>
      <a href={poi.osm_url} target="_blank" rel="noreferrer" className="inline-block text-blue-600 hover:underline">
        在 OpenStreetMap 中查看
      </a>
    </div>
  )
}
