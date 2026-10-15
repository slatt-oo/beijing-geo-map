import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import { Map as MaplibreMap, setWorkerUrl } from 'maplibre-gl'
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { BEIJING_BBOX, REGION_BBOX, tileUrls, toDisplay } from '../config'
import type { BBox } from '../lib/bbox'
import type { LngLat } from '../lib/coord'
import { addDataLayers, baseStyle } from './layers'

// MapLibre 6 在运行时按 import.meta.url 拼出 worker 地址，打包后那个文件并不存在，所以显式交给 Vite 输出并指定。
setWorkerUrl(workerUrl)

const MapContext = createContext<MaplibreMap | null>(null)

export function useMap(): MaplibreMap {
  const map = useContext(MapContext)
  if (!map) throw new Error('useMap 只能在 MapView 内部使用')
  return map
}

function displayBounds([minLng, minLat, maxLng, maxLat]: BBox): [LngLat, LngLat] {
  return [toDisplay([minLng, minLat]), toDisplay([maxLng, maxLat])]
}

// 样式加载完成、数据图层建好之后才渲染 children，子组件可以直接使用这些图层。
export function MapView({ children }: { children?: ReactNode }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [map, setMap] = useState<MaplibreMap | null>(null)

  useEffect(() => {
    const instance = new MaplibreMap({
      container: containerRef.current!,
      style: baseStyle(tileUrls),
      bounds: displayBounds(BEIJING_BBOX),
      maxBounds: displayBounds(REGION_BBOX),
      maxZoom: 18,
      hash: true,
      locale: { 'Popup.Close': '关闭' },
      attributionControl: { compact: false },
    })
    // 用 style.load 而不是 load：load 要等首屏所有底图瓦片加载完才触发，接口请求会排在瓦片之后。
    instance.once('style.load', () => {
      addDataLayers(instance)
      setMap(instance)
    })
    return () => instance.remove()
  }, [])

  return (
    <>
      {/* maplibre-gl.css 会给容器加上 position: relative，所以用 h-full 撑满，而不是 absolute inset-0。 */}
      <div ref={containerRef} className="h-full w-full" />
      {map && <MapContext value={map}>{children}</MapContext>}
    </>
  )
}
