import type { Point } from 'geojson'
import type { GeoJSONSource, MapGeoJSONFeature } from 'maplibre-gl'
import { useEffect } from 'react'
import { toData } from '../config'
import { routeClick, type PoiHit } from '../lib/clickRoute'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const POI_LAYERS = [LAYER.poiCluster, LAYER.poiPoint]
const CLICKABLE_LAYERS = [...POI_LAYERS, LAYER.districtFill]

function toPoiHit(feature: MapGeoJSONFeature | undefined): PoiHit | undefined {
  if (!feature) return undefined
  if (feature.layer.id === LAYER.poiCluster) {
    return {
      kind: 'cluster',
      clusterId: Number(feature.properties.cluster_id),
      center: (feature.geometry as Point).coordinates as LngLat,
    }
  }
  return { kind: 'point', id: Number(feature.properties.id) }
}

// 地图上的点击和右键都在这里分发，判断规则见 lib/clickRoute.ts。
export function MapInteractions() {
  const map = useMap()

  useEffect(() => {
    const click = map.on('click', (e) => {
      const { mode, openPoi, selectDistrict, setMyLocation } = useAppStore.getState()
      const [poi] = map.queryRenderedFeatures(e.point, { layers: POI_LAYERS })
      const [district] = map.queryRenderedFeatures(e.point, { layers: [LAYER.districtFill] })
      const action = routeClick(mode, toPoiHit(poi), district ? String(district.properties.adcode) : undefined)
      switch (action.type) {
        case 'pickLocation': {
          const [lng, lat] = toData([e.lngLat.lng, e.lngLat.lat])
          setMyLocation({ lng, lat, accuracy: null })
          break
        }
        case 'expandCluster':
          map
            .getSource<GeoJSONSource>(SOURCE.pois)
            ?.getClusterExpansionZoom(action.clusterId)
            .then((zoom) => map.easeTo({ center: action.center, zoom }))
            // 数据在点击后刷新时聚合 ID 会失效，这次点击直接放弃即可。
            .catch(() => {})
          break
        case 'openPoi':
          openPoi(action.id)
          break
        case 'selectDistrict':
          openPoi(null)
          selectDistrict(action.adcode)
          break
      }
    })
    const contextMenu = map.on('contextmenu', (e) => {
      if (useAppStore.getState().mode === 'pickLocation') return
      useAppStore.getState().openContextMenu(toData([e.lngLat.lng, e.lngLat.lat]))
    })
    const move = map.on('mousemove', (e) => {
      if (useAppStore.getState().mode === 'pickLocation') {
        map.getCanvas().style.cursor = 'crosshair'
        return
      }
      const hit = map.queryRenderedFeatures(e.point, { layers: CLICKABLE_LAYERS }).length > 0
      map.getCanvas().style.cursor = hit ? 'pointer' : ''
    })
    return () => {
      click.unsubscribe()
      contextMenu.unsubscribe()
      move.unsubscribe()
    }
  }, [map])

  return null
}
