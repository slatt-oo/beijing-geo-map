import { describe, expect, it } from 'vitest'
import { formatArea } from './format'

describe('formatArea', () => {
  it('keeps at most two decimals and groups thousands', () => {
    expect(formatArea(430.77)).toBe('430.77 平方千米')
    expect(formatArea(2229.4)).toBe('2,229.4 平方千米')
    expect(formatArea(41.8)).toBe('41.8 平方千米')
  })
})
