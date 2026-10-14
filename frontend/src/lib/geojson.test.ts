import type { FeatureCollection, MultiPolygon, Point } from 'geojson'
import { describe, expect, it } from 'vitest'
import type { LngLat } from './coord'
import { bboxOfGeometry, transformCollection, transformGeometry } from './geojson'

const shift = ([lng, lat]: LngLat): LngLat => [lng + 1, lat + 2]

const square: MultiPolygon = {
  type: 'MultiPolygon',
  coordinates: [
    [
      [
        [116, 39],
        [117, 39],
        [117, 40],
        [116, 40],
        [116, 39],
      ],
    ],
    [
      [
        [118, 41],
        [118.5, 41],
        [118.5, 41.5],
        [118, 41],
      ],
    ],
  ],
}

describe('transformGeometry', () => {
  it('transforms every position of a MultiPolygon without touching the input', () => {
    const before = structuredClone(square)

    const out = transformGeometry(square, shift) as MultiPolygon

    expect(out.coordinates[0][0][2]).toEqual([118, 42])
    expect(out.coordinates[1][0][1]).toEqual([119.5, 43])
    expect(square).toEqual(before)
  })

  it('keeps a third coordinate such as altitude', () => {
    expect(transformGeometry({ type: 'Point', coordinates: [116, 39, 50] }, shift)).toEqual({
      type: 'Point',
      coordinates: [117, 41, 50],
    })
  })

  it('recurses into geometry collections', () => {
    const out = transformGeometry(
      {
        type: 'GeometryCollection',
        geometries: [
          { type: 'Point', coordinates: [116, 39] },
          { type: 'LineString', coordinates: [[116, 39], [117, 40]] },
        ],
      },
      shift,
    )
    expect(out).toEqual({
      type: 'GeometryCollection',
      geometries: [
        { type: 'Point', coordinates: [117, 41] },
        { type: 'LineString', coordinates: [[117, 41], [118, 42]] },
      ],
    })
  })
})

describe('transformCollection', () => {
  it('converts geometries and keeps ids, properties and foreign members', () => {
    const collection: FeatureCollection<Point, { name: string }> & { truncated: boolean } = {
      type: 'FeatureCollection',
      truncated: true,
      features: [{ type: 'Feature', id: 7, properties: { name: '北京大学' }, geometry: { type: 'Point', coordinates: [116.3, 39.98] } }],
    }

    const out = transformCollection(collection, shift)

    expect(out).toEqual({
      type: 'FeatureCollection',
      truncated: true,
      features: [{ type: 'Feature', id: 7, properties: { name: '北京大学' }, geometry: { type: 'Point', coordinates: [117.3, 41.98] } }],
    })
  })
})

describe('bboxOfGeometry', () => {
  it('returns the extent of all polygons', () => {
    expect(bboxOfGeometry(square)).toEqual([116, 39, 118.5, 41.5])
  })
})
