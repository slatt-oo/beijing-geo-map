import { describe, expect, it } from 'vitest'
import fixture from '../../../testdata/coord_cases.json'
import { createCoordTransform, gcj02ToWgs84, outOfChina, wgs84ToGcj02, type LngLat } from './coord'

type Case = { name: string; wgs84: LngLat; gcj02: LngLat }

const cases = fixture.cases as Case[]
const TOLERANCE = 1e-6

function expectClose(actual: LngLat, expected: LngLat) {
  expect(Math.abs(actual[0] - expected[0])).toBeLessThan(TOLERANCE)
  expect(Math.abs(actual[1] - expected[1])).toBeLessThan(TOLERANCE)
}

describe('wgs84ToGcj02', () => {
  it.each(cases)('matches the shared fixture: $name', ({ wgs84, gcj02 }) => {
    expectClose(wgs84ToGcj02(wgs84), gcj02)
  })
})

describe('gcj02ToWgs84', () => {
  it.each(cases)('inverts the forward transform: $name', ({ wgs84, gcj02 }) => {
    const inverse = gcj02ToWgs84(gcj02)
    expectClose(wgs84ToGcj02(inverse), gcj02)
    expectClose(inverse, wgs84)
  })

  it.each(cases.filter(({ wgs84 }) => outOfChina(...wgs84)))('returns points outside China unchanged: $name', ({ wgs84 }) => {
    expect(gcj02ToWgs84(wgs84)).toEqual(wgs84)
  })
})

describe('createCoordTransform', () => {
  const tiananmen = cases.find(({ name }) => name === '天安门')!

  it('converts between WGS-84 data and a GCJ-02 basemap', () => {
    const { toDisplay, toData } = createCoordTransform('gcj02')
    expectClose(toDisplay(tiananmen.wgs84), tiananmen.gcj02)
    expectClose(toData(tiananmen.gcj02), tiananmen.wgs84)
  })

  it('passes coordinates through for a WGS-84 basemap', () => {
    const { toDisplay, toData } = createCoordTransform('wgs84')
    expect(toDisplay(tiananmen.wgs84)).toEqual(tiananmen.wgs84)
    expect(toData(tiananmen.gcj02)).toEqual(tiananmen.gcj02)
  })
})
