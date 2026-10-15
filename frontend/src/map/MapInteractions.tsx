import type { Point } from 'geojson'
import type { GeoJSONSource } from 'maplibre-gl'
import { useEffect } from 'react'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { LAYER, SOURCE } from './layers'
import { useMap } from './MapView'

const POI_LAYERS = [LAYER.poiCluster, LAYER.poiPoint]
const CLICKABLE_LAYERS = [...POI_LAYERS, LAYER.districtFill]

// 点击统一在这里分发：POI 优先于它下面的区；点到 POI 以外的地方会关闭 POI 弹窗。
export function MapInteractions() {
  const map = useMap()

  useEffect(() => {
    const click = map.on('click', (e) => {
      const { openPoi, selectDistrict } = useAppStore.getState()
      const [poi] = map.queryRenderedFeatures(e.point, { layers: POI_LAYERS })
      if (poi?.layer.id === LAYER.poiCluster) {
        const center = (poi.geometry as Point).coordinates as LngLat
        map
          .getSource<GeoJSONSource>(SOURCE.pois)
          ?.getClusterExpansionZoom(Number(poi.properties.cluster_id))
          .then((zoom) => map.easeTo({ center, zoom }))
          // 数据在点击后刷新时聚合 ID 会失效，这次点击直接放弃即可。
          .catch(() => {})
        return
      }
      if (poi) {
        openPoi(Number(poi.properties.id))
        return
      }
      openPoi(null)
      const [district] = map.queryRenderedFeatures(e.point, { layers: [LAYER.districtFill] })
      selectDistrict(district ? String(district.properties.adcode) : null)
    })
    const move = map.on('mousemove', (e) => {
      const hit = map.queryRenderedFeatures(e.point, { layers: CLICKABLE_LAYERS }).length > 0
      map.getCanvas().style.cursor = hit ? 'pointer' : ''
    })
    return () => {
      click.unsubscribe()
      move.unsubscribe()
    }
  }, [map])

  return null
}
