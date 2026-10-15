import { describe, expect, it } from 'vitest'
import { categoriesParam, categoryColorExpression, FALLBACK_POI_COLOR } from './categories'

const codes = ['subway_station', 'university', 'attraction', 'hospital', 'park']

describe('categoriesParam', () => {
  it('omits the parameter when nothing is hidden', () => {
    expect(categoriesParam(codes, [])).toBeUndefined()
  })

  it('lists the enabled categories in catalog order', () => {
    expect(categoriesParam(codes, ['hospital', 'subway_station'])).toEqual(['university', 'attraction', 'park'])
  })

  it('returns null when every category is hidden', () => {
    expect(categoriesParam(codes, [...codes].reverse())).toBeNull()
  })
})

describe('categoryColorExpression', () => {
  it('maps each category code to its color', () => {
    const expr = categoryColorExpression([
      { code: 'park', name: '公园', color: '#43A047' },
      { code: 'hospital', name: '医院', color: '#8E24AA' },
    ])
    expect(expr).toEqual(['match', ['get', 'category'], 'park', '#43A047', 'hospital', '#8E24AA', FALLBACK_POI_COLOR])
  })

  it('falls back to a plain color before categories load', () => {
    expect(categoryColorExpression([])).toBe(FALLBACK_POI_COLOR)
  })
})
