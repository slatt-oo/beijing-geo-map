import type { FeatureCollection } from 'geojson'
import type { Map as MaplibreMap, StyleSpecification } from 'maplibre-gl'
import { FALLBACK_POI_COLOR } from '../lib/categories'

export const BASEMAP_SOURCE = 'basemap'

export const SOURCE = {
  districts: 'districts',
  districtLabels: 'district-labels',
  nearbyCircle: 'nearby-circle',
  pois: 'pois',
  myLocation: 'my-location',
} as const

export const LAYER = {
  districtFill: 'district-fill',
  districtLine: 'district-line',
  districtLabel: 'district-label',
  nearbyFill: 'nearby-fill',
  nearbyLine: 'nearby-line',
  poiCluster: 'poi-cluster',
  poiClusterCount: 'poi-cluster-count',
  poiPoint: 'poi-point',
  myLocationAccuracy: 'my-location-accuracy',
  myLocationDot: 'my-location-dot',
} as const

export const EMPTY_COLLECTION: FeatureCollection = { type: 'FeatureCollection', features: [] }

export function baseStyle(tiles: string[], attribution: string): StyleSpecification {
  return {
    version: 8,
    sources: {
      [BASEMAP_SOURCE]: { type: 'raster', tiles, tileSize: 256, maxzoom: 18, attribution },
    },
    layers: [{ id: 'basemap', type: 'raster', source: BASEMAP_SOURCE }],
  }
}

// 所有数据图层在样式加载完成时一次建好（数据先为空），保证从下到上的顺序固定；之后只更新数据和样式。
export function addDataLayers(map: MaplibreMap): void {
  map.addSource(SOURCE.districts, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    promoteId: 'adcode',
    attribution: '行政区划 © 阿里云 DataV.GeoAtlas',
  })
  map.addSource(SOURCE.districtLabels, { type: 'geojson', data: EMPTY_COLLECTION })
  map.addSource(SOURCE.nearbyCircle, { type: 'geojson', data: EMPTY_COLLECTION })
  map.addSource(SOURCE.pois, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    cluster: true,
    clusterRadius: 50,
    clusterMaxZoom: 14,
    attribution: '© OpenStreetMap contributors（ODbL）',
  })
  map.addSource(SOURCE.myLocation, { type: 'geojson', data: EMPTY_COLLECTION })

  map.addLayer({
    id: LAYER.districtFill,
    type: 'fill',
    source: SOURCE.districts,
    paint: {
      'fill-color': '#2563eb',
      'fill-opacity': [
        'case',
        ['boolean', ['feature-state', 'selected'], false],
        0.22,
        ['boolean', ['feature-state', 'hover'], false],
        0.14,
        0.04,
      ],
    },
  })
  map.addLayer({
    id: LAYER.districtLine,
    type: 'line',
    source: SOURCE.districts,
    paint: { 'line-color': '#1d4ed8', 'line-width': 1.2, 'line-opacity': 0.8 },
  })
  map.addLayer({
    id: LAYER.districtLabel,
    type: 'symbol',
    source: SOURCE.districtLabels,
    layout: { 'text-field': ['get', 'name'], 'text-size': 13 },
    paint: { 'text-color': '#1e3a8a', 'text-halo-color': '#ffffff', 'text-halo-width': 1.5 },
  })
  map.addLayer({
    id: LAYER.nearbyFill,
    type: 'fill',
    source: SOURCE.nearbyCircle,
    paint: { 'fill-color': '#0ea5e9', 'fill-opacity': 0.1 },
  })
  map.addLayer({
    id: LAYER.nearbyLine,
    type: 'line',
    source: SOURCE.nearbyCircle,
    paint: { 'line-color': '#0284c7', 'line-width': 2, 'line-dasharray': [2, 1] },
  })
  map.addLayer({
    id: LAYER.poiCluster,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    paint: {
      'circle-color': '#f97316',
      'circle-opacity': 0.85,
      'circle-radius': ['step', ['get', 'point_count'], 14, 50, 18, 200, 24],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1.5,
    },
  })
  map.addLayer({
    id: LAYER.poiClusterCount,
    type: 'symbol',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-size': 12, 'text-allow-overlap': true },
    paint: { 'text-color': '#ffffff' },
  })
  map.addLayer({
    id: LAYER.poiPoint,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['!', ['has', 'point_count']],
    paint: {
      'circle-color': FALLBACK_POI_COLOR,
      'circle-radius': 5,
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1,
    },
  })
  map.addLayer({
    id: LAYER.myLocationAccuracy,
    type: 'fill',
    source: SOURCE.myLocation,
    filter: ['==', ['geometry-type'], 'Polygon'],
    paint: { 'fill-color': '#2563eb', 'fill-opacity': 0.15, 'fill-outline-color': '#2563eb' },
  })
  map.addLayer({
    id: LAYER.myLocationDot,
    type: 'circle',
    source: SOURCE.myLocation,
    filter: ['==', ['geometry-type'], 'Point'],
    paint: { 'circle-color': '#2563eb', 'circle-radius': 7, 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 2 },
  })
}
