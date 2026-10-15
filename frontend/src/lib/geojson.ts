import type { FeatureCollection, Geometry, Position } from 'geojson'
import type { BBox } from './bbox'
import type { LngLat } from './coord'

type Transform = (p: LngLat) => LngLat

export function transformGeometry(geometry: Geometry, f: Transform): Geometry {
  switch (geometry.type) {
    case 'Point':
      return { ...geometry, coordinates: mapPosition(geometry.coordinates, f) }
    case 'MultiPoint':
    case 'LineString':
      return { ...geometry, coordinates: geometry.coordinates.map((p) => mapPosition(p, f)) }
    case 'MultiLineString':
    case 'Polygon':
      return { ...geometry, coordinates: geometry.coordinates.map((line) => line.map((p) => mapPosition(p, f))) }
    case 'MultiPolygon':
      return {
        ...geometry,
        coordinates: geometry.coordinates.map((polygon) => polygon.map((ring) => ring.map((p) => mapPosition(p, f)))),
      }
    case 'GeometryCollection':
      return { ...geometry, geometries: geometry.geometries.map((g) => transformGeometry(g, f)) }
  }
}

// 返回新的 FeatureCollection，只转换几何，属性和顶层的其他字段（例如 truncated）原样保留。
export function transformCollection<G extends Geometry, P>(
  collection: FeatureCollection<G, P>,
  f: Transform,
): FeatureCollection<G, P> {
  return {
    ...collection,
    features: collection.features.map((feature) => ({ ...feature, geometry: transformGeometry(feature.geometry, f) as G })),
  }
}

export function bboxOfGeometry(geometry: Geometry): BBox {
  const box: BBox = [Infinity, Infinity, -Infinity, -Infinity]
  for (const [lng, lat] of positions(geometry)) {
    box[0] = Math.min(box[0], lng)
    box[1] = Math.min(box[1], lat)
    box[2] = Math.max(box[2], lng)
    box[3] = Math.max(box[3], lat)
  }
  return box
}

function mapPosition(p: Position, f: Transform): Position {
  const [lng, lat] = f([p[0], p[1]])
  return p.length > 2 ? [lng, lat, ...p.slice(2)] : [lng, lat]
}

function positions(geometry: Geometry): Position[] {
  switch (geometry.type) {
    case 'Point':
      return [geometry.coordinates]
    case 'MultiPoint':
    case 'LineString':
      return geometry.coordinates
    case 'MultiLineString':
    case 'Polygon':
      return geometry.coordinates.flat()
    case 'MultiPolygon':
      return geometry.coordinates.flat(2)
    case 'GeometryCollection':
      return geometry.geometries.flatMap(positions)
  }
}
