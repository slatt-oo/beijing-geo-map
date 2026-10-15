import type { FeatureCollection, Point } from 'geojson'
import { Popup, type GeoJSONSource } from 'maplibre-gl'
import { useEffect, useMemo } from 'react'
import { useDistricts } from '../api/queries'
import { toDisplay } from '../config'
import { bboxOfGeometry, transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { LAYER, SOURCE } from './layers'
import { useMap } from './MapView'
import { FIT_PADDING } from './padding'

export function DistrictLayer() {
  const map = useMap()
  const { data } = useDistricts()
  const selected = useAppStore((s) => s.selectedDistrict)

  const display = useMemo(() => (data ? transformCollection(data, toDisplay) : null), [data])

  useEffect(() => {
    if (!display) return
    map.getSource<GeoJSONSource>(SOURCE.districts)?.setData(display)
    const labels: FeatureCollection<Point, { name: string }> = {
      type: 'FeatureCollection',
      features: display.features.map((f) => ({
        type: 'Feature',
        properties: { name: f.properties.name },
        geometry: { type: 'Point', coordinates: toDisplay(f.properties.center) },
      })),
    }
    map.getSource<GeoJSONSource>(SOURCE.districtLabels)?.setData(labels)
  }, [map, display])

  useEffect(() => {
    const tooltip = new Popup({ closeButton: false, closeOnClick: false, offset: 14, className: 'district-tooltip' })
    let hovered: string | null = null
    const setHovered = (adcode: string | null) => {
      if (adcode === hovered) return
      if (hovered) map.setFeatureState({ source: SOURCE.districts, id: hovered }, { hover: false })
      hovered = adcode
      if (hovered) map.setFeatureState({ source: SOURCE.districts, id: hovered }, { hover: true })
    }
    const move = map.on('mousemove', LAYER.districtFill, (e) => {
      const feature = e.features?.[0]
      if (!feature) return
      const adcode = String(feature.properties.adcode)
      if (adcode !== hovered) {
        setHovered(adcode)
        tooltip.setText(String(feature.properties.name))
      }
      tooltip.setLngLat(e.lngLat)
      if (!tooltip.isOpen()) tooltip.addTo(map)
    })
    const hide = () => {
      setHovered(null)
      tooltip.remove()
    }
    const leave = map.on('mouseleave', LAYER.districtFill, hide)
    // 地图飞到别处时鼠标没动，提示会停在原来的坐标上；等下一次 mousemove 再显示。
    const moveStart = map.on('movestart', hide)
    return () => {
      move.unsubscribe()
      leave.unsubscribe()
      moveStart.unsubscribe()
      setHovered(null)
      tooltip.remove()
    }
  }, [map])

  useEffect(() => {
    if (!selected) return
    map.setFeatureState({ source: SOURCE.districts, id: selected }, { selected: true })
    return () => {
      map.setFeatureState({ source: SOURCE.districts, id: selected }, { selected: false })
    }
  }, [map, selected])

  useEffect(() => {
    const feature = display?.features.find((f) => f.properties.adcode === selected)
    if (!feature) return
    const [minLng, minLat, maxLng, maxLat] = bboxOfGeometry(feature.geometry)
    map.fitBounds(
      [
        [minLng, minLat],
        [maxLng, maxLat],
      ],
      { padding: FIT_PADDING },
    )
  }, [map, display, selected])

  return null
}
