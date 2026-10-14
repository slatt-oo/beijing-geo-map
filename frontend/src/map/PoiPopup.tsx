import { Popup } from 'maplibre-gl'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { usePoiDetail } from '../api/queries'
import { PoiDetailCard } from '../components/PoiDetailCard'
import { toDisplay } from '../config'
import { useAppStore } from '../store/useAppStore'
import { useMap } from './MapView'

// 弹窗内容由 React 渲染到一个固定的 div 里，再把这个 div 交给 MapLibre 的 Popup。
export function PoiPopup() {
  const map = useMap()
  const id = useAppStore((s) => s.activePoiId)
  const { data: poi } = usePoiDetail(id)
  const [container] = useState(() => document.createElement('div'))

  useEffect(() => {
    if (!poi) return
    const popup = new Popup({ closeOnClick: false, offset: 10, maxWidth: '300px' })
      .setLngLat(toDisplay(poi.location))
      .setDOMContent(container)
      .addTo(map)
    const onClose = () => useAppStore.getState().openPoi(null)
    popup.on('close', onClose)
    return () => {
      popup.off('close', onClose)
      popup.remove()
    }
  }, [map, poi, container])

  return poi ? createPortal(<PoiDetailCard poi={poi} />, container) : null
}
