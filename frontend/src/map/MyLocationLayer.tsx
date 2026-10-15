import type { Feature, FeatureCollection } from 'geojson'
import type { GeoJSONSource } from 'maplibre-gl'
import { useEffect } from 'react'
import { useLocateDistrict } from '../api/queries'
import { toDisplay } from '../config'
import { circlePolygon } from '../lib/circle'
import { transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, SOURCE } from './layers'
import { useMap } from './MapView'

const LOCATE_ZOOM = 14

export function MyLocationLayer() {
  const map = useMap()
  const myLocation = useAppStore((s) => s.myLocation)
  const { data: district, isSuccess } = useLocateDistrict(myLocation)

  useEffect(() => {
    const source = map.getSource<GeoJSONSource>(SOURCE.myLocation)
    if (!myLocation) {
      source?.setData(EMPTY_COLLECTION)
      return
    }
    const center: [number, number] = [myLocation.lng, myLocation.lat]
    const features: Feature[] = [{ type: 'Feature', properties: {}, geometry: { type: 'Point', coordinates: center } }]
    if (myLocation.accuracy) features.unshift(circlePolygon(center, myLocation.accuracy))
    const collection: FeatureCollection = { type: 'FeatureCollection', features }
    source?.setData(transformCollection(collection, toDisplay))
    map.flyTo({ center: toDisplay(center), zoom: Math.max(map.getZoom(), LOCATE_ZOOM) })
  }, [map, myLocation])

  useEffect(() => {
    if (!myLocation || !isSuccess) return
    useAppStore.getState().showInfo(district ? `你在${district.name}` : '当前位置不在北京市范围内')
  }, [myLocation, district, isSuccess])

  return null
}
