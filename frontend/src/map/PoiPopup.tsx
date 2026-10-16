import { Popup } from 'maplibre-gl'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { usePoiDetail } from '../api/queries'
import { PoiDetailCard } from '../components/PoiDetailCard'
import { toDisplay } from '../config'
import { useAppStore } from '../store/useAppStore'
import { useMap } from './MapView'

export function PoiPopup() {
  const map = useMap()
  const id = useAppStore((s) => s.activePoiId)
  const { data: poi, isError } = usePoiDetail(id)
  const [container] = useState(() => document.createElement('div'))

  // 详情加载失败时清掉选中的 POI，否则再次点击同一个点不会重新请求。已经显示的弹窗在后台刷新失败时保留。
  const failed = isError && !poi
  useEffect(() => {
    if (failed) useAppStore.getState().openPoi(null)
  }, [failed])

  useEffect(() => {
    if (!poi) return
    // closeOnClick 必须关闭，清理时必须先 off 再 remove：MapLibre 关闭弹窗时会触发 close，
    // 这两处一旦触发，都会把点击分发刚选中的下一个 POI 清掉。
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
