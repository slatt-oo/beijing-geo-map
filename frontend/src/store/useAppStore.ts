import { create } from 'zustand'
import type { LngLat } from '../lib/coord'

export type AppMode = 'normal' | 'pickLocation'

// 坐标都是 WGS-84；浏览器定位得到的位置带精度（米），在地图上点选的位置没有精度。
export type MyLocation = { lng: number; lat: number; accuracy: number | null }

export type Nearby = { center: LngLat; radius: number }

export type Notice = { tone: 'info' | 'error'; message: string }

export const NEARBY_RADII = [500, 1000, 2000, 5000] as const

export const DEFAULT_NEARBY_RADIUS = 1000

export type AppState = {
  hiddenCategories: string[]
  selectedDistrict: string | null
  activePoiId: number | null
  notice: Notice | null
  mode: AppMode
  // 正在等待的浏览器定位请求。点选位置或取消点选时清空，这次请求的结果随之作废。
  locateRequest: object | null
  myLocation: MyLocation | null
  nearby: Nearby | null
  nearbyAfterLocate: boolean
  contextMenu: LngLat | null
  toggleCategory: (code: string) => void
  selectDistrict: (adcode: string | null) => void
  openPoi: (id: number | null) => void
  showError: (message: string) => void
  showInfo: (message: string) => void
  dismissNotice: () => void
  startLocating: (request: object) => void
  startPickLocation: () => void
  cancelPickLocation: () => void
  setMyLocation: (location: MyLocation) => void
  requestNearbyAfterLocate: () => void
  openNearby: (center: LngLat) => void
  setNearbyRadius: (radius: number) => void
  closeNearby: () => void
  openContextMenu: (position: LngLat) => void
  closeContextMenu: () => void
}

export const useAppStore = create<AppState>()((set) => ({
  hiddenCategories: [],
  selectedDistrict: null,
  activePoiId: null,
  notice: null,
  mode: 'normal',
  locateRequest: null,
  myLocation: null,
  nearby: null,
  nearbyAfterLocate: false,
  contextMenu: null,
  toggleCategory: (code) =>
    set((state) => ({
      hiddenCategories: state.hiddenCategories.includes(code)
        ? state.hiddenCategories.filter((c) => c !== code)
        : [...state.hiddenCategories, code],
    })),
  // 左侧面板只显示区详情或附近结果中的一个，选中区时关闭附近查询。
  selectDistrict: (adcode) =>
    set((state) => ({ selectedDistrict: adcode, nearby: adcode === null ? state.nearby : null })),
  openPoi: (id) => set({ activePoiId: id }),
  showError: (message) => set({ notice: { tone: 'error', message } }),
  showInfo: (message) => set({ notice: { tone: 'info', message } }),
  dismissNotice: () => set({ notice: null }),
  startLocating: (request) => set({ locateRequest: request }),
  startPickLocation: () => set({ mode: 'pickLocation', locateRequest: null }),
  cancelPickLocation: () => set({ mode: 'normal', nearbyAfterLocate: false, locateRequest: null }),
  // 点击「附近」时如果还没有位置，会先定位；定位（或点选）完成后接着打开附近查询。
  setMyLocation: (location) =>
    set((state) => ({
      myLocation: location,
      mode: 'normal',
      locateRequest: null,
      nearbyAfterLocate: false,
      ...(state.nearbyAfterLocate
        ? {
            nearby: {
              center: [location.lng, location.lat] as LngLat,
              radius: state.nearby?.radius ?? DEFAULT_NEARBY_RADIUS,
            },
            selectedDistrict: null,
          }
        : {}),
    })),
  requestNearbyAfterLocate: () => set({ nearbyAfterLocate: true }),
  openNearby: (center) =>
    set((state) => ({
      nearby: { center, radius: state.nearby?.radius ?? DEFAULT_NEARBY_RADIUS },
      selectedDistrict: null,
      contextMenu: null,
      nearbyAfterLocate: false,
    })),
  setNearbyRadius: (radius) => set((state) => ({ nearby: state.nearby && { ...state.nearby, radius } })),
  closeNearby: () => set({ nearby: null }),
  openContextMenu: (position) => set({ contextMenu: position }),
  closeContextMenu: () => set({ contextMenu: null }),
}))
