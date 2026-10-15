import type { FeatureCollection } from 'geojson'
import type { GeoJSONSource } from 'maplibre-gl'
import { useEffect } from 'react'
import { toDisplay } from '../config'
import { circlePolygon } from '../lib/circle'
import { bboxOfGeometry, transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, SOURCE } from './layers'
import { useMap } from './MapView'
import { FIT_PADDING } from './padding'

export function NearbyLayer() {
  const map = useMap()
  const nearby = useAppStore((s) => s.nearby)

  useEffect(() => {
    const source = map.getSource<GeoJSONSource>(SOURCE.nearbyCircle)
    if (!nearby) {
      source?.setData(EMPTY_COLLECTION)
      return
    }
    const collection: FeatureCollection = {
      type: 'FeatureCollection',
      features: [circlePolygon(nearby.center, nearby.radius)],
    }
    const display = transformCollection(collection, toDisplay)
    source?.setData(display)
    const [minLng, minLat, maxLng, maxLat] = bboxOfGeometry(display.features[0].geometry)
    map.fitBounds(
      [
        [minLng, minLat],
        [maxLng, maxLat],
      ],
      { padding: FIT_PADDING },
    )
  }, [map, nearby])

  return null
}
