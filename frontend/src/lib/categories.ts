import type { ExpressionSpecification } from 'maplibre-gl'
import type { Category } from '../api/types'

export const FALLBACK_POI_COLOR = '#64748b'

// 请求 /pois 时的 categories 参数：undefined 表示不传（全部分类），null 表示分类全部关闭、不必请求。
export function categoriesParam(allCodes: string[], hidden: string[]): string[] | undefined | null {
  if (hidden.length === 0) return undefined
  const enabled = allCodes.filter((code) => !hidden.includes(code))
  return enabled.length === 0 ? null : enabled
}

export function categoryColorExpression(categories: Category[]): ExpressionSpecification | string {
  if (categories.length === 0) return FALLBACK_POI_COLOR
  const [first, ...rest] = categories
  return [
    'match',
    ['get', 'category'],
    first.code,
    first.color,
    ...rest.flatMap((c) => [c.code, c.color]),
    FALLBACK_POI_COLOR,
  ]
}
