import type { TileCrs } from './coord'

export const DEFAULT_TILE_URL =
  'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}'

// MapLibre 不认识 {s} 占位符，要展开成 1 到 4 号子域名的多个地址，由 MapLibre 轮流请求。
export function expandTileUrl(template: string): string[] {
  if (!template.includes('{s}')) return [template]
  return ['1', '2', '3', '4'].map((s) => template.replaceAll('{s}', s))
}

export function parseTileCrs(value: string | undefined): TileCrs {
  if (value === undefined || value === '' || value === 'gcj02') return 'gcj02'
  if (value === 'wgs84') return 'wgs84'
  throw new Error(`VITE_TILE_CRS 只能是 gcj02 或 wgs84，当前为「${value}」`)
}
