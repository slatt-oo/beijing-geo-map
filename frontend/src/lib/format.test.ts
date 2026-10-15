import { describe, expect, it } from 'vitest'
import { formatArea, formatDistance } from './format'

describe('formatArea', () => {
  it('keeps at most two decimals and groups thousands', () => {
    expect(formatArea(430.77)).toBe('430.77 平方千米')
    expect(formatArea(2229.4)).toBe('2,229.4 平方千米')
    expect(formatArea(41.8)).toBe('41.8 平方千米')
  })
})

describe('formatDistance', () => {
  it('uses metres below 1 km and one decimal of kilometres above', () => {
    expect(formatDistance(320)).toBe('320 m')
    expect(formatDistance(999)).toBe('999 m')
    expect(formatDistance(1000)).toBe('1 km')
    expect(formatDistance(1234)).toBe('1.2 km')
  })
})
