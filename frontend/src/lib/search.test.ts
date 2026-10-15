import { describe, expect, it } from 'vitest'
import { MAX_QUERY_LENGTH, toSearchTerm } from './search'

describe('toSearchTerm', () => {
  it('trims surrounding spaces', () => {
    expect(toSearchTerm('  北京大学 ')).toBe('北京大学')
  })

  it('keeps at most the endpoint limit, counted in code points', () => {
    const name = 'East Campus Community Health Service Center of China Agricultural University'

    expect(toSearchTerm(name)).toBe(name.slice(0, MAX_QUERY_LENGTH))
    expect(Array.from(toSearchTerm('😀'.repeat(60)))).toHaveLength(MAX_QUERY_LENGTH)
  })
})
