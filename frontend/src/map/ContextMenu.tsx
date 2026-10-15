import { Popup } from 'maplibre-gl'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { toDisplay } from '../config'
import { useAppStore } from '../store/useAppStore'
import { useMap } from './MapView'

// 右键菜单用一个不带关闭按钮的 Popup 显示，点击地图或移动地图时自动关闭。
export function ContextMenu() {
  const map = useMap()
  const position = useAppStore((s) => s.contextMenu)
  const [container] = useState(() => document.createElement('div'))

  useEffect(() => {
    if (!position) return
    const popup = new Popup({ closeButton: false, closeOnMove: true, offset: 4, anchor: 'top-left' })
      .setLngLat(toDisplay(position))
      .setDOMContent(container)
      .addTo(map)
    const onClose = () => useAppStore.getState().closeContextMenu()
    popup.on('close', onClose)
    return () => {
      popup.off('close', onClose)
      popup.remove()
    }
  }, [map, position, container])

  if (!position) return null
  return createPortal(
    <button
      type="button"
      onClick={() => useAppStore.getState().openNearby(position)}
      className="whitespace-nowrap rounded px-2 py-1 text-sm text-slate-800 hover:bg-slate-100"
    >
      以此为中心查附近
    </button>,
    container,
  )
}
