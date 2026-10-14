import { describe, expect, it } from 'vitest'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './tiles'

describe('expandTileUrl', () => {
  it('expands {s} into the four Gaode subdomains', () => {
    expect(expandTileUrl(DEFAULT_TILE_URL)).toEqual([
      'https://webrd01.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd02.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd03.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      'https://webrd04.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
    ])
  })

  it('keeps a template without {s} as the only URL', () => {
    expect(expandTileUrl('https://tile.openstreetmap.org/{z}/{x}/{y}.png')).toEqual([
      'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
    ])
  })
})

describe('parseTileCrs', () => {
  it('defaults to gcj02 when unset or empty', () => {
    expect(parseTileCrs(undefined)).toBe('gcj02')
    expect(parseTileCrs('')).toBe('gcj02')
  })

  it('accepts the two supported values', () => {
    expect(parseTileCrs('gcj02')).toBe('gcj02')
    expect(parseTileCrs('wgs84')).toBe('wgs84')
  })

  it('rejects anything else', () => {
    expect(() => parseTileCrs('WGS84')).toThrow('VITE_TILE_CRS 只能是 gcj02 或 wgs84，当前为「WGS84」')
  })
})
