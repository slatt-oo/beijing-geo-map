import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import { Map as MaplibreMap, setWorkerUrl } from 'maplibre-gl'
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { BEIJING_BBOX, REGION_BBOX, tileAttribution, tileUrls, toDisplay } from '../config'
import type { BBox } from '../lib/bbox'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { addDataLayers, BASEMAP_SOURCE, baseStyle } from './layers'

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
      style: baseStyle(tileUrls, tileAttribution),
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
    // 瓦片地址被拦截或不可用时地图只剩灰底，没有任何报错；每次打开页面只提示一次。
    let basemapErrorShown = false
    instance.on('error', (e) => {
      // 注册了 error 监听后 MapLibre 不再自己把错误打印到控制台，这里补上，否则数据图层出错时什么也看不到。
      console.error(e.error)
      if (basemapErrorShown || !('sourceId' in e) || e.sourceId !== BASEMAP_SOURCE) return
      basemapErrorShown = true
      useAppStore.getState().showError('底图加载失败，请检查网络；如果瓦片地址不可用，可以用 VITE_TILE_URL 换一个底图')
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
