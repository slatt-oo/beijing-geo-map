# 阶段 4：定位与查询 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在阶段 3 的地图上加入定位（失败时在地图上点选）、POI 搜索和附近查询：右下角的「定位」「附近」按钮，左上角的搜索框，左侧共用的面板（区详情或附近结果），地图上的蓝点、精度圈、范围圆和右键菜单。

**Architecture:** 沿用阶段 3 的结构：交互状态放在 Zustand（新增位置、模式、附近查询、右键菜单和统一的提示），地图事件回调只注册一次，通过 `getState()` 读取最新状态。新图层（`nearby-circle`、`my-location`）同样在 `style.load` 里一次建好，由新的图层组件调用 `setData`。可以单测的逻辑都放在纯函数和 store 里：点击分发规则（`lib/clickRoute.ts`）、浏览器定位封装（`lib/geolocation.ts`）、圆形多边形（`lib/circle.ts`）、距离格式化、新的查询选项工厂、图层顺序（用只记录调用的假地图）。依赖 WebGL 和 DOM 的组件靠 lint、类型检查、构建和阶段结束时的浏览器验收来保证。后端不改。

**Tech Stack:** React 19.3、TypeScript ~6.0.3、Vite 8.3、Vitest 5、Tailwind CSS 4.3、maplibre-gl 6.14、@tanstack/react-query 5.104、zustand 5.0；新增 `@turf/circle` 7.4。

**Spec:** `docs/01-implementation-plan.md`（第 5 节「阶段 4」）、`docs/02-technical-design.md`（第 6.1 到 6.4 节、第 7.2 节、第 10 节第 6 到 12 项）

## Global Constraints

- 所有工作在 `phase-4-locate-search` 分支上进行，每个任务结束时提交一次。git 提交身份已经配置好，不要修改；环境会在提交信息末尾自动加一行 `Co-authored-by: Cursor`。
- 前端只新增一个依赖：`@turf/circle@^7.4.0`（只用命名导入 `import { circle } from '@turf/circle'`）。不引入其他 turf 包、jsdom、Testing Library 或 React 地图封装库。TypeScript 保持 `~6.0.3`。
- MapLibre 6 只能用命名导入，类型用 `import type`。地图事件回调用 `map.on(...)` 返回的订阅对象的 `unsubscribe()` 注销。
- 坐标系：接口数据、store、查询参数一律是 WGS-84。只能通过 `src/config.ts` 导出的 `toDisplay()` / `toData()` 转换：交给地图数据源的 GeoJSON 一律经过 `transformCollection(…, toDisplay)`，`flyTo` 的中心经过 `toDisplay()`；从地图上取到的坐标（点击、右键位置）一律经过 `toData()`。圆形多边形在 WGS-84 下生成（按球面距离），再整体经过 `toDisplay()`。
- 前端检查命令：`cd frontend && npm run lint && npm run typecheck && npm test && npm run build`，输出中不能有错误或警告。Vitest 运行在 node 环境，只测纯函数、store 和查询选项。项目不使用 Prettier，不要运行它。
- 计划中的代码已经在一份原型里通过上面全部检查，并在浏览器中验证过；照抄即可，不要自行“改进”。如果检查失败，先确认代码与计划一致，再报告问题。
- 界面文案和提示使用中文。代码注释只用来说明代码本身体现不出来的约束。
- 本地环境：compose 的 `db` 服务在运行（`127.0.0.1:5432`，已导入完整数据）。8080 端口被占用，启动整套服务时使用 `FRONTEND_PORT=18080`。验证完执行 `docker compose stop frontend backend`，保留 db。绝不执行 `docker compose down -v`。
- **沙箱说明：** docker、curl 以及 git commit 命令都需要 Shell `required_permissions: ["all"]`；`npm install` 需要访问 npm 源，被拦截时用 `required_permissions: ["full_network"]`。临时文件放在仓库内被 git 忽略的 `.superpowers/` 目录下，不要放 `/tmp`。

### 本计划的决定

以下内容在规格中没有写明，或与规格的写法不同，由本计划决定：

1. **附近查询以接口上限 `limit=100` 请求，后端不改。** 2 千米内通常有 80 到 100 个 POI，5 千米内约 340 个。结果正好 100 个时，面板提示「仅显示最近的 100 个」。
2. **精度圈用 `@turf/circle` 按 `accuracy` 米数生成多边形**，而不是规格写的“换算成像素半径的圆点”：在任何缩放级别都与底图比例一致，也和范围圆共用同一个函数。在地图上点选的位置没有精度（`accuracy: null`），只画蓝点。
3. **弹窗与分类开关互不影响。** 搜索结果和附近列表可以打开任何分类的 POI 详情，包括当前关闭的分类；关闭某个分类时，已经打开的该分类弹窗不会关闭。附近列表和地图图层一样遵循分类开关（分类全部关闭时不请求，提示「分类已全部关闭」），搜索不受分类开关影响。
4. **提示统一为 `notice`。** store 的 `errorMessage` / `dismissError` 改为 `notice: { tone: 'info' | 'error'; message }`、`showInfo` / `showError` / `dismissNotice`；`ErrorBanner` 改为 `NoticeBanner`：普通提示 4 秒后自动消失，错误提示一直显示到用户关闭或被新的提示替换。`api/client.ts` 的 `errorMessage()` 函数不再和 store 字段重名，保持不变。
5. **「附近」在还没有位置时先定位。** store 记录 `nearbyAfterLocate`；定位成功或点选完成后自动以该位置打开附近查询（默认 1000 米）；在点选模式中点「取消」会一并放弃。重新定位不会移动已经打开的附近查询。
6. **左侧共用一列。** 搜索框和面板（区详情或附近结果）放在左上角同一列，宽 320 像素。选中区会关闭附近查询，打开附近查询会取消选中的区。缩放到某个区或查询范围时的边距 `FIT_PADDING`（左侧 360 像素）统一放在 `map/padding.ts`。
7. **右键菜单是一个不带关闭按钮的 MapLibre `Popup`**，点击或移动地图时自动关闭；点选模式下右键不弹出菜单。
8. **缩放级别。** 定位成功或点选后地图飞到该位置，缩放级别至少 14；搜索结果和附近列表项飞到 16 级；附近查询用 `fitBounds` 缩放到范围圆。
9. **点击分发规则抽成纯函数 `routeClick`**（`lib/clickRoute.ts`）：点选模式下任何点击都用来设置位置；否则聚合圆优先，其次单个 POI，最后是区（点到北京以外取消选中）。
10. **浏览器定位封装在 `lib/geolocation.ts`**（可以用假的 `Geolocation` 对象单测），规格里的 `useGeolocation` 实际叫 `useLocate`：它返回一个 `locate()` 函数，成功时写入位置，失败（拒绝授权、超时、浏览器不支持）时转入点选模式。
11. **地图 `locale` 设置 `'Popup.Close': '关闭'`**，弹窗关闭按钮的无障碍标签显示中文。
12. **查询选项统一用 `apiGetData<T>()` 解包 `{"data": ...}`**，解包逻辑只在 `api/client.test.ts` 测一次。

---

### Task 1: 全局状态、提示条与纯函数

**Files:**
- Modify: `frontend/package.json`、`frontend/package-lock.json`（新增 `@turf/circle`）
- Modify: `frontend/src/store/useAppStore.ts`、`frontend/src/store/useAppStore.test.ts`
- Create: `frontend/src/components/NoticeBanner.tsx`
- Delete: `frontend/src/components/ErrorBanner.tsx`
- Modify: `frontend/src/App.tsx`（只替换提示条组件）
- Create: `frontend/src/lib/clickRoute.ts`、`frontend/src/lib/clickRoute.test.ts`
- Create: `frontend/src/lib/geolocation.ts`、`frontend/src/lib/geolocation.test.ts`
- Create: `frontend/src/lib/circle.ts`、`frontend/src/lib/circle.test.ts`
- Modify: `frontend/src/lib/format.ts`、`frontend/src/lib/format.test.ts`

- [ ] **Step 1: 安装依赖**

```bash
cd frontend && npm install --no-audit --no-fund @turf/circle@^7.4.0
```

确认 `package.json` 的 `dependencies` 中出现 `"@turf/circle": "^7.4.0"`。

- [ ] **Step 2: 编写测试**

`frontend/src/store/useAppStore.test.ts`（整个文件替换）：

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_NEARBY_RADIUS, useAppStore } from './useAppStore'

const initial = useAppStore.getState()
const state = () => useAppStore.getState()

describe('useAppStore', () => {
  beforeEach(() => {
    useAppStore.setState(initial, true)
  })

  it('toggles a category off and back on', () => {
    state().toggleCategory('hospital')
    state().toggleCategory('park')
    expect(state().hiddenCategories).toEqual(['hospital', 'park'])

    state().toggleCategory('hospital')
    expect(state().hiddenCategories).toEqual(['park'])
  })

  it('shows info and error notices and dismisses them', () => {
    state().showInfo('你在海淀区')
    expect(state().notice).toEqual({ tone: 'info', message: '你在海淀区' })

    state().showError('网络错误')
    expect(state().notice).toEqual({ tone: 'error', message: '网络错误' })

    state().dismissNotice()
    expect(state().notice).toBeNull()
  })

  it('leaves pick mode once a location is set', () => {
    state().startPickLocation()
    expect(state().mode).toBe('pickLocation')

    state().setMyLocation({ lng: 116.39, lat: 39.9, accuracy: null })

    expect(state().mode).toBe('normal')
    expect(state().myLocation).toEqual({ lng: 116.39, lat: 39.9, accuracy: null })
    expect(state().nearby).toBeNull()
  })

  it('opens the nearby search after locating when it was requested first', () => {
    state().selectDistrict('110108')
    state().requestNearbyAfterLocate()

    state().setMyLocation({ lng: 116.31, lat: 39.99, accuracy: 30 })

    expect(state().nearby).toEqual({ center: [116.31, 39.99], radius: DEFAULT_NEARBY_RADIUS })
    expect(state().selectedDistrict).toBeNull()
    expect(state().nearbyAfterLocate).toBe(false)
  })

  it('forgets a pending nearby search when picking is cancelled', () => {
    state().requestNearbyAfterLocate()
    state().startPickLocation()

    state().cancelPickLocation()
    state().setMyLocation({ lng: 116.39, lat: 39.9, accuracy: null })

    expect(state().mode).toBe('normal')
    expect(state().nearby).toBeNull()
  })

  it('shows district detail and nearby results one at a time', () => {
    state().selectDistrict('110108')
    state().openNearby([116.4, 39.9])
    expect(state().selectedDistrict).toBeNull()
    expect(state().nearby).toEqual({ center: [116.4, 39.9], radius: DEFAULT_NEARBY_RADIUS })

    state().selectDistrict('110105')
    expect(state().nearby).toBeNull()
  })

  it('keeps the chosen radius when the nearby center moves', () => {
    state().openNearby([116.4, 39.9])
    state().setNearbyRadius(2000)

    state().openNearby([116.3, 39.98])

    expect(state().nearby).toEqual({ center: [116.3, 39.98], radius: 2000 })
  })

  it('closes the context menu when a nearby search starts from it', () => {
    state().openContextMenu([116.5, 39.95])

    state().openNearby([116.5, 39.95])

    expect(state().contextMenu).toBeNull()
  })
})
```

`frontend/src/lib/clickRoute.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { routeClick } from './clickRoute'

describe('routeClick', () => {
  it('uses every click to set the location in pick mode', () => {
    expect(routeClick('pickLocation', { kind: 'point', id: 7 }, '110108')).toEqual({ type: 'pickLocation' })
  })

  it('expands a cluster before anything underneath it', () => {
    expect(routeClick('normal', { kind: 'cluster', clusterId: 3, center: [116.3, 39.9] }, '110108')).toEqual({
      type: 'expandCluster',
      clusterId: 3,
      center: [116.3, 39.9],
    })
  })

  it('opens a POI instead of selecting the district under it', () => {
    expect(routeClick('normal', { kind: 'point', id: 7 }, '110108')).toEqual({ type: 'openPoi', id: 7 })
  })

  it('selects the district when no POI was hit', () => {
    expect(routeClick('normal', undefined, '110108')).toEqual({ type: 'selectDistrict', adcode: '110108' })
  })

  it('clears the selection outside every district', () => {
    expect(routeClick('normal', undefined, undefined)).toEqual({ type: 'selectDistrict', adcode: null })
  })
})
```

`frontend/src/lib/geolocation.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'
import { GEOLOCATION_OPTIONS, requestPosition } from './geolocation'

function fakeGeolocation(impl: Geolocation['getCurrentPosition']): Geolocation {
  return { getCurrentPosition: impl, watchPosition: vi.fn(), clearWatch: vi.fn() }
}

describe('requestPosition', () => {
  it('resolves with the WGS-84 position and its accuracy', async () => {
    const getCurrentPosition = vi.fn<Geolocation['getCurrentPosition']>((success) =>
      success({ coords: { longitude: 116.31, latitude: 39.99, accuracy: 25 } } as GeolocationPosition),
    )

    await expect(requestPosition(fakeGeolocation(getCurrentPosition))).resolves.toEqual({
      lng: 116.31,
      lat: 39.99,
      accuracy: 25,
    })
    expect(getCurrentPosition).toHaveBeenCalledWith(expect.any(Function), expect.any(Function), GEOLOCATION_OPTIONS)
  })

  it('asks for high accuracy with a 10 second timeout', () => {
    expect(GEOLOCATION_OPTIONS).toEqual({ enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 })
  })

  it('rejects when the browser refuses or fails', async () => {
    const geolocation = fakeGeolocation((_success, failure) =>
      failure?.({ code: 1, message: 'User denied Geolocation' } as GeolocationPositionError),
    )

    await expect(requestPosition(geolocation)).rejects.toThrow('User denied Geolocation')
  })

  it('rejects when the browser has no geolocation', async () => {
    await expect(requestPosition(undefined)).rejects.toThrow('浏览器不支持定位')
  })
})
```

`frontend/src/lib/circle.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { circlePolygon } from './circle'
import type { LngLat } from './coord'

// 球面距离（米），只用于检查生成的圆。
function haversine([lng1, lat1]: LngLat, [lng2, lat2]: LngLat): number {
  const rad = Math.PI / 180
  const a =
    Math.sin(((lat2 - lat1) * rad) / 2) ** 2 +
    Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(((lng2 - lng1) * rad) / 2) ** 2
  return 2 * 6371008.8 * Math.asin(Math.sqrt(a))
}

describe('circlePolygon', () => {
  it('builds a closed 64-gon whose vertices are radius metres from the center', () => {
    const center: LngLat = [116.397, 39.908]

    const ring = circlePolygon(center, 1000).geometry.coordinates[0]

    expect(ring).toHaveLength(65)
    expect(ring[64]).toEqual(ring[0])
    for (const vertex of ring) {
      expect(Math.abs(haversine(center, vertex as LngLat) - 1000)).toBeLessThan(1)
    }
  })
})
```

`frontend/src/lib/format.test.ts`（整个文件替换）：

```ts
import { describe, expect, it } from 'vitest'
import { formatArea, formatDistance } from './format'

describe('formatArea', () => {
  it('keeps at most two decimals and groups thousands', () => {
    expect(formatArea(430.77)).toBe('430.77 平方千米')
    expect(formatArea(2229.4)).toBe('2,229.4 平方千米')
    expect(formatArea(41.8)).toBe('41.8 平方千米')
  })
})

describe('formatDistance', () => {
  it('uses metres below 1 km and one decimal of kilometres above', () => {
    expect(formatDistance(320)).toBe('320 m')
    expect(formatDistance(999)).toBe('999 m')
    expect(formatDistance(1000)).toBe('1 km')
    expect(formatDistance(1234)).toBe('1.2 km')
  })
})
```

- [ ] **Step 3: 运行测试，确认失败**

```bash
cd frontend && npm test
```

预期：新测试因为模块或导出不存在而失败。

- [ ] **Step 4: 实现**

`frontend/src/store/useAppStore.ts`（整个文件替换）：

```ts
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
  startPickLocation: () => set({ mode: 'pickLocation' }),
  cancelPickLocation: () => set({ mode: 'normal', nearbyAfterLocate: false }),
  // 点击「附近」时如果还没有位置，会先定位；定位（或点选）完成后接着打开附近查询。
  setMyLocation: (location) =>
    set((state) => ({
      myLocation: location,
      mode: 'normal',
      nearbyAfterLocate: false,
      ...(state.nearbyAfterLocate
        ? {
            nearby: { center: [location.lng, location.lat] as LngLat, radius: DEFAULT_NEARBY_RADIUS },
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
    })),
  setNearbyRadius: (radius) => set((state) => ({ nearby: state.nearby && { ...state.nearby, radius } })),
  closeNearby: () => set({ nearby: null }),
  openContextMenu: (position) => set({ contextMenu: position }),
  closeContextMenu: () => set({ contextMenu: null }),
}))
```

`frontend/src/lib/clickRoute.ts`：

```ts
import type { AppMode } from '../store/useAppStore'
import type { LngLat } from './coord'

export type PoiHit = { kind: 'cluster'; clusterId: number; center: LngLat } | { kind: 'point'; id: number }

export type ClickAction =
  | { type: 'pickLocation' }
  | { type: 'expandCluster'; clusterId: number; center: LngLat }
  | { type: 'openPoi'; id: number }
  | { type: 'selectDistrict'; adcode: string | null }

// 点击的优先级：点选位置模式下任何点击都用来设置位置；否则 POI（聚合或单点）优先于它下面的区。
export function routeClick(mode: AppMode, poi: PoiHit | undefined, adcode: string | undefined): ClickAction {
  if (mode === 'pickLocation') return { type: 'pickLocation' }
  if (poi?.kind === 'cluster') return { type: 'expandCluster', clusterId: poi.clusterId, center: poi.center }
  if (poi) return { type: 'openPoi', id: poi.id }
  return { type: 'selectDistrict', adcode: adcode ?? null }
}
```

`frontend/src/lib/geolocation.ts`：

```ts
import type { MyLocation } from '../store/useAppStore'

export const GEOLOCATION_OPTIONS: PositionOptions = { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 }

// 浏览器定位得到的是 WGS-84 坐标，可以直接用于查询。不支持定位或定位失败时 reject，由调用方转入点选模式。
export function requestPosition(geolocation: Geolocation | undefined): Promise<MyLocation> {
  return new Promise((resolve, reject) => {
    if (!geolocation) {
      reject(new Error('浏览器不支持定位'))
      return
    }
    geolocation.getCurrentPosition(
      ({ coords }) => resolve({ lng: coords.longitude, lat: coords.latitude, accuracy: coords.accuracy }),
      (error) => reject(new Error(error.message)),
      GEOLOCATION_OPTIONS,
    )
  })
}
```

`frontend/src/lib/circle.ts`：

```ts
import { circle } from '@turf/circle'
import type { Feature, Polygon } from 'geojson'
import type { LngLat } from './coord'

// 64 边形近似圆，按球面距离计算，所以在 WGS-84 下生成，交给地图前再统一经过 toDisplay。
export function circlePolygon(center: LngLat, radiusM: number): Feature<Polygon> {
  return circle(center, radiusM, { steps: 64, units: 'meters' })
}
```

`frontend/src/lib/format.ts`（整个文件替换）：

```ts
export function formatArea(km2: number): string {
  return `${km2.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} 平方千米`
}

export function formatDistance(meters: number): string {
  if (meters < 1000) return `${Math.round(meters)} m`
  return `${(meters / 1000).toLocaleString('zh-CN', { maximumFractionDigits: 1 })} km`
}
```

- [ ] **Step 5: 用 `NoticeBanner` 替换 `ErrorBanner`**

删除 `frontend/src/components/ErrorBanner.tsx`，新建 `frontend/src/components/NoticeBanner.tsx`：

```tsx
import { useEffect } from 'react'
import { useAppStore } from '../store/useAppStore'

const INFO_NOTICE_MS = 4000

const TONE_CLASS = {
  info: 'bg-sky-50 text-sky-800 ring-sky-200',
  error: 'bg-red-50 text-red-700 ring-red-200',
}

// 普通提示几秒后自动消失；错误提示一直显示，直到用户关闭或被新的提示替换。
export function NoticeBanner() {
  const notice = useAppStore((s) => s.notice)
  const dismissNotice = useAppStore((s) => s.dismissNotice)

  useEffect(() => {
    if (notice?.tone !== 'info') return
    const timer = setTimeout(() => useAppStore.getState().dismissNotice(), INFO_NOTICE_MS)
    return () => clearTimeout(timer)
  }, [notice])

  if (!notice) return null
  return (
    <div
      role={notice.tone === 'error' ? 'alert' : 'status'}
      className={`pointer-events-auto flex max-w-xs items-start gap-2 rounded-lg px-3 py-2 text-sm shadow-lg ring-1 ${TONE_CLASS[notice.tone]}`}
    >
      <span className="flex-1">{notice.message}</span>
      <button type="button" aria-label="关闭提示" onClick={dismissNotice} className="leading-none opacity-70 hover:opacity-100">
        ×
      </button>
    </div>
  )
}
```

在 `frontend/src/App.tsx` 中只做两处替换：`import { ErrorBanner } from './components/ErrorBanner'` 改为 `import { NoticeBanner } from './components/NoticeBanner'`（按字母顺序放在 `DistrictPanel` 的导入之后），`<ErrorBanner />` 改为 `<NoticeBanner />`。`main.tsx` 调用的 `showError` 名字没变，不用改。

- [ ] **Step 6: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build
```

预期：全部通过，没有警告。然后：

```bash
git add frontend && git commit -m "feat(frontend): add location, nearby and notice state with pure helpers"
```

---

### Task 2: 接口类型与查询选项

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`、`frontend/src/api/client.test.ts`
- Modify: `frontend/src/api/queries.ts`、`frontend/src/api/queries.test.ts`

- [ ] **Step 1: 编写测试**

`frontend/src/api/client.test.ts`（整个文件替换，新增 `apiGetData` 的测试）：

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiGet, apiGetData, buildUrl, errorMessage, shouldRetry } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('buildUrl', () => {
  it('returns the bare path under /api/v1 when there are no params', () => {
    expect(buildUrl('/health')).toBe('/api/v1/health')
  })

  it('encodes params and skips null, undefined and empty arrays', () => {
    const url = buildUrl('/pois/search', { q: '北京', limit: 10, categories: [], radius: undefined, lng: null })
    expect(url).toBe('/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC&limit=10')
  })

  it('joins array params with commas', () => {
    expect(buildUrl('/pois', { categories: ['park', 'university'] })).toBe(
      '/api/v1/pois?categories=park%2Cuniversity',
    )
  })
})

describe('apiGet', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('requests the built URL and returns the parsed body', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse(200, { data: { status: 'ok', db: 'ok' } }))
    vi.stubGlobal('fetch', fetchMock)
    const controller = new AbortController()

    await expect(apiGet('/health', { verbose: 1 }, controller.signal)).resolves.toEqual({
      data: { status: 'ok', db: 'ok' },
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/health?verbose=1',
      expect.objectContaining({ signal: controller.signal }),
    )
  })

  it('throws ApiError carrying the server error code and message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(400, { error: { code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' } })),
    )

    const err = await apiGet('/districts/locate').catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 400, code: 'INVALID_PARAM', message: 'lat 必须在 -90 到 90 之间' })
  })

  it('falls back to HTTP_ERROR when the error body is not the standard format', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>Bad Gateway</html>', { status: 502 })))

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 502, code: 'HTTP_ERROR' })
  })

  it('maps network failures to a NETWORK ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('fetch failed')
      }),
    )

    await expect(apiGet('/health')).rejects.toMatchObject({ status: 0, code: 'NETWORK' })
  })

  function signalAwareFetch() {
    return vi.fn<typeof fetch>(async (_input, init) => {
      init?.signal?.throwIfAborted()
      return jsonResponse(200, { data: null })
    })
  }

  it('rethrows a plain abort unchanged', async () => {
    vi.stubGlobal('fetch', signalAwareFetch())
    const controller = new AbortController()
    controller.abort()

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('rethrows an abort with a custom reason unchanged', async () => {
    vi.stubGlobal('fetch', signalAwareFetch())
    const controller = new AbortController()
    const reason = new Error('navigated away')
    controller.abort(reason)

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toBe(reason)
  })

  it('rethrows an abort that happens while reading an error body', async () => {
    const controller = new AbortController()
    const reason = new Error('navigated away')
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(async () => {
        controller.abort(reason)
        return jsonResponse(500, { error: { code: 'INTERNAL', message: '服务器内部错误' } })
      }),
    )

    await expect(apiGet('/health', undefined, controller.signal)).rejects.toBe(reason)
  })
})

describe('errorMessage', () => {
  it('uses the server message of an ApiError', () => {
    expect(errorMessage(new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe('POI 不存在')
  })

  it('falls back to a generic message for anything else', () => {
    expect(errorMessage(new TypeError('boom'))).toBe('请求失败，请稍后重试')
  })
})

describe('shouldRetry', () => {
  it('retries a network or server error once', () => {
    expect(shouldRetry(0, new ApiError(0, 'NETWORK', '网络错误'))).toBe(true)
    expect(shouldRetry(0, new ApiError(500, 'INTERNAL', '服务器内部错误'))).toBe(true)
    expect(shouldRetry(1, new ApiError(500, 'INTERNAL', '服务器内部错误'))).toBe(false)
  })

  it('does not retry client errors', () => {
    expect(shouldRetry(0, new ApiError(400, 'INVALID_PARAM', 'bbox 格式错误'))).toBe(false)
    expect(shouldRetry(0, new ApiError(404, 'NOT_FOUND', 'POI 不存在'))).toBe(false)
  })
})

describe('apiGetData', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('unwraps the data envelope', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { data: { adcode: '110108', name: '海淀区' } })))

    await expect(apiGetData('/districts/locate', { lng: 116.3, lat: 39.99 })).resolves.toEqual({
      adcode: '110108',
      name: '海淀区',
    })
  })

  it('keeps a null result, which some endpoints use for "not found here"', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { data: null })))

    await expect(apiGetData('/districts/locate', { lng: 116.7, lat: 39.5 })).resolves.toBeNull()
  })
})
```

`frontend/src/api/queries.test.ts`（整个文件替换，新增搜索、附近和反查区的测试）：

```ts
import { QueryClient, QueryObserver, skipToken } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BBox } from '../lib/bbox'
import { districtQuery, locateQuery, NEARBY_LIMIT, nearbyQuery, POI_LIMIT, poiDetailQuery, poisQuery, SEARCH_LIMIT, searchQuery } from './queries'

const box: BBox = [116.3, 39.9, 116.4, 40]
const emptyCollection = { type: 'FeatureCollection', features: [], truncated: false }

function stubFetch(body: unknown) {
  const fetchMock = vi.fn<typeof fetch>(
    async () => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('poisQuery', () => {
  it('keeps all, none and a category list in separate cache entries', () => {
    const keys = [poisQuery(box, undefined), poisQuery(box, null), poisQuery(box, ['park'])].map((q) =>
      JSON.stringify(q.queryKey),
    )
    expect(new Set(keys).size).toBe(3)
  })

  it('skips the request without a bbox or when every category is hidden', () => {
    expect(poisQuery(null, undefined).queryFn).toBe(skipToken)
    expect(poisQuery(box, null).queryFn).toBe(skipToken)
  })

  it('requests the bbox, the categories and the fixed limit', async () => {
    const fetchMock = stubFetch(emptyCollection)

    await new QueryClient().fetchQuery(poisQuery(box, ['park', 'university']))

    expect(fetchMock.mock.calls[0][0]).toBe(
      `/api/v1/pois?bbox=116.3%2C39.9%2C116.4%2C40&categories=park%2Cuniversity&limit=${POI_LIMIT}`,
    )
  })

  it('does not fall back to the previous result when every category is hidden', async () => {
    stubFetch(emptyCollection)
    const observer = new QueryObserver(new QueryClient(), poisQuery(box, undefined))
    const unsubscribe = observer.subscribe(() => {})
    await vi.waitFor(() => expect(observer.getCurrentResult().isSuccess).toBe(true))

    observer.setOptions(poisQuery(box, null))

    expect(observer.getCurrentResult().data).toBeUndefined()
    unsubscribe()
  })
})

describe('detail queries', () => {
  it('unwrap the data envelope', async () => {
    stubFetch({ data: { id: 42, name: '北京大学' } })

    await expect(new QueryClient().fetchQuery(poiDetailQuery(42))).resolves.toEqual({ id: 42, name: '北京大学' })
  })

  it('skip the request without an id', () => {
    expect(poiDetailQuery(null).queryFn).toBe(skipToken)
    expect(districtQuery(null).queryFn).toBe(skipToken)
  })
})

describe('searchQuery', () => {
  it('skips an empty query', () => {
    expect(searchQuery('').queryFn).toBe(skipToken)
  })

  it('requests the query with the dropdown limit and unwraps the hits', async () => {
    const fetchMock = stubFetch({ data: [{ id: 3, name: '北京大学', category: 'university', district_name: '海淀区', location: [116.3, 39.99] }] })

    const hits = await new QueryClient().fetchQuery(searchQuery('北京大学'))

    expect(fetchMock.mock.calls[0][0]).toBe(`/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC%E5%A4%A7%E5%AD%A6&limit=${SEARCH_LIMIT}`)
    expect(hits.map((h) => h.name)).toEqual(['北京大学'])
  })
})

describe('nearbyQuery', () => {
  const nearby = { center: [116.397, 39.908] as [number, number], radius: 1000 }

  it('keys on center, radius and categories', () => {
    const keys = [
      nearbyQuery(nearby, undefined),
      nearbyQuery({ ...nearby, radius: 2000 }, undefined),
      nearbyQuery(nearby, ['park']),
      nearbyQuery(nearby, null),
    ].map((q) => JSON.stringify(q.queryKey))
    expect(new Set(keys).size).toBe(4)
  })

  it('skips the request without a center or when every category is hidden', () => {
    expect(nearbyQuery(null, undefined).queryFn).toBe(skipToken)
    expect(nearbyQuery(nearby, null).queryFn).toBe(skipToken)
  })

  it('requests the center, radius, categories and the endpoint limit', async () => {
    const fetchMock = stubFetch({ type: 'FeatureCollection', features: [], center: nearby.center, radius: 1000 })

    await new QueryClient().fetchQuery(nearbyQuery(nearby, ['park']))

    expect(fetchMock.mock.calls[0][0]).toBe(
      `/api/v1/pois/nearby?lng=116.397&lat=39.908&radius=1000&categories=park&limit=${NEARBY_LIMIT}`,
    )
  })
})

describe('locateQuery', () => {
  it('skips the request without a location', () => {
    expect(locateQuery(null).queryFn).toBe(skipToken)
  })

  it('returns null outside Beijing', async () => {
    stubFetch({ data: null })

    await expect(new QueryClient().fetchQuery(locateQuery({ lng: 116.7, lat: 39.5, accuracy: null }))).resolves.toBeNull()
  })
})
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd frontend && npm test
```

预期：新测试因为导出不存在而失败。

- [ ] **Step 3: 实现**

`frontend/src/api/types.ts`（整个文件替换）：

```ts
import type { FeatureCollection, MultiPolygon, Point } from 'geojson'
import type { LngLat } from '../lib/coord'

// 与后端 internal/service/types.go 的 JSON 结构一一对应，坐标都是 WGS-84。

export type DataResponse<T> = { data: T }

export type Category = { code: string; name: string; color: string }

export type DistrictProperties = {
  adcode: string
  name: string
  area_km2: number
  poi_count: number
  center: LngLat
}

export type DistrictCollection = FeatureCollection<MultiPolygon, DistrictProperties>

export type CategoryCount = { code: string; name: string; count: number }

export type DistrictDetail = {
  adcode: string
  name: string
  area_km2: number
  center: LngLat
  poi_counts: CategoryCount[]
}

export type POIProperties = { id: number; name: string; category: string }

export type POICollection = FeatureCollection<Point, POIProperties> & { truncated: boolean }

export type POIDetail = {
  id: number
  name: string
  name_en: string | null
  category: Category
  district: DistrictRef | null
  location: LngLat
  address: string | null
  tags: Record<string, string>
  osm_url: string
}

export type DistrictRef = { adcode: string; name: string }

export type SearchHit = {
  id: number
  name: string
  category: string
  district_name: string | null
  location: LngLat
}

export type NearbyProperties = POIProperties & { distance_m: number }

export type NearbyCollection = FeatureCollection<Point, NearbyProperties> & { center: LngLat; radius: number }
```

`frontend/src/api/client.ts`（整个文件替换，新增 `apiGetData`）：

```ts
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export type QueryValue = string | number | readonly (string | number)[] | null | undefined

const API_BASE = '/api/v1'

export function buildUrl(path: string, params: Record<string, QueryValue> = {}): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === null || value === undefined) continue
    if (Array.isArray(value)) {
      if (value.length > 0) search.set(key, value.join(','))
      continue
    }
    search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `${API_BASE}${path}?${qs}` : `${API_BASE}${path}`
}

export async function apiGet<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T> {
  let res: Response
  try {
    res = await fetch(buildUrl(path, params), { signal, headers: { Accept: 'application/json' } })
  } catch (err) {
    if (signal?.aborted) throw err
    throw new ApiError(0, 'NETWORK', '网络错误，请确认后端服务已启动')
  }
  if (!res.ok) {
    const apiError = await toApiError(res)
    signal?.throwIfAborted()
    throw apiError
  }
  return (await res.json()) as T
}

// 除 GeoJSON 接口外，其他接口都把结果包在 {"data": ...} 里。
export async function apiGetData<T>(path: string, params?: Record<string, QueryValue>, signal?: AbortSignal): Promise<T> {
  return (await apiGet<{ data: T }>(path, params, signal)).data
}

export function errorMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : '请求失败，请稍后重试'
}

// 4xx 是请求本身的问题，重试也不会成功，只会让错误提示晚出现约 1 秒；网络错误和 5xx 重试一次。
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false
  return failureCount < 1
}

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: { code?: unknown; message?: unknown } }
    const { code, message } = body.error ?? {}
    if (typeof code === 'string' && typeof message === 'string') {
      return new ApiError(res.status, code, message)
    }
  } catch {
    // 非 JSON 响应体（例如 nginx 的 502 页面）走下面的通用错误
  }
  return new ApiError(res.status, 'HTTP_ERROR', `请求失败（HTTP ${res.status}）`)
}
```

`frontend/src/api/queries.ts`（整个文件替换：原有查询改用 `apiGetData`，新增 `searchQuery`、`nearbyQuery`、`locateQuery` 和对应的 hook）：

```ts
import { keepPreviousData, queryOptions, skipToken, useQuery } from '@tanstack/react-query'
import { bboxParam, type BBox } from '../lib/bbox'
import type { MyLocation, Nearby } from '../store/useAppStore'
import { apiGet, apiGetData } from './client'
import type {
  Category,
  DistrictCollection,
  DistrictDetail,
  DistrictRef,
  NearbyCollection,
  POICollection,
  POIDetail,
  SearchHit,
} from './types'

// 当前快照约 4100 个 POI：按接口上限请求时，整个北京也不会被截断，聚合数字才准确。
export const POI_LIMIT = 5000

export const SEARCH_LIMIT = 10

// 附近接口的上限。结果正好这么多时，列表提示只显示了最近的这些。
export const NEARBY_LIMIT = 100

export function districtsQuery() {
  return queryOptions({
    queryKey: ['districts'],
    queryFn: ({ signal }) => apiGet<DistrictCollection>('/districts', undefined, signal),
    staleTime: Infinity,
  })
}

export function districtQuery(adcode: string | null) {
  return queryOptions({
    queryKey: ['district', adcode],
    queryFn:
      adcode === null
        ? skipToken
        : ({ signal }) => apiGetData<DistrictDetail>(`/districts/${adcode}`, undefined, signal),
    staleTime: Infinity,
  })
}

export function categoriesQuery() {
  return queryOptions({
    queryKey: ['poi-categories'],
    queryFn: ({ signal }) => apiGetData<Category[]>('/poi-categories', undefined, signal),
    staleTime: Infinity,
  })
}

// categories 为 undefined 表示全部分类；为 null 表示分类全部关闭：此时不请求，也不沿用上一次的结果。
export function poisQuery(bbox: BBox | null, categories: string[] | undefined | null) {
  return queryOptions({
    queryKey: ['pois', bbox && bboxParam(bbox), categories === null ? 'none' : (categories ?? 'all')],
    queryFn:
      bbox === null || categories === null
        ? skipToken
        : ({ signal }) => apiGet<POICollection>('/pois', { bbox: bboxParam(bbox), categories, limit: POI_LIMIT }, signal),
    placeholderData: categories === null ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
    // 全市视野下稍一平移 bbox 就会变，每条缓存都有约 4000 个要素，不必按默认在内存里保留 5 分钟。
    gcTime: 60 * 1000,
  })
}

export function poiDetailQuery(id: number | null) {
  return queryOptions({
    queryKey: ['poi', id],
    queryFn: id === null ? skipToken : ({ signal }) => apiGetData<POIDetail>(`/pois/${id}`, undefined, signal),
    staleTime: 5 * 60 * 1000,
  })
}

// q 为空字符串时不请求；输入过程中保留上一次的结果，避免下拉框闪烁。
export function searchQuery(q: string) {
  return queryOptions({
    queryKey: ['search', q],
    queryFn:
      q === '' ? skipToken : ({ signal }) => apiGetData<SearchHit[]>('/pois/search', { q, limit: SEARCH_LIMIT }, signal),
    placeholderData: q === '' ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
  })
}

// categories 的含义与 poisQuery 相同。
export function nearbyQuery(nearby: Nearby | null, categories: string[] | undefined | null) {
  return queryOptions({
    queryKey: [
      'nearby',
      nearby && [...nearby.center, nearby.radius],
      categories === null ? 'none' : (categories ?? 'all'),
    ],
    queryFn:
      nearby === null || categories === null
        ? skipToken
        : ({ signal }) =>
            apiGet<NearbyCollection>(
              '/pois/nearby',
              { lng: nearby.center[0], lat: nearby.center[1], radius: nearby.radius, categories, limit: NEARBY_LIMIT },
              signal,
            ),
    placeholderData: categories === null ? undefined : keepPreviousData,
    staleTime: 5 * 60 * 1000,
  })
}

export function locateQuery(location: MyLocation | null) {
  return queryOptions({
    queryKey: ['locate', location && [location.lng, location.lat]],
    queryFn:
      location === null
        ? skipToken
        : ({ signal }) =>
            apiGetData<DistrictRef | null>('/districts/locate', { lng: location.lng, lat: location.lat }, signal),
    staleTime: Infinity,
  })
}

export function useDistricts() {
  return useQuery(districtsQuery())
}

export function useDistrict(adcode: string | null) {
  return useQuery(districtQuery(adcode))
}

export function useCategories() {
  return useQuery(categoriesQuery())
}

export function usePoisInBBox(bbox: BBox | null, categories: string[] | undefined | null) {
  return useQuery(poisQuery(bbox, categories))
}

export function usePoiDetail(id: number | null) {
  return useQuery(poiDetailQuery(id))
}

export function useSearch(q: string) {
  return useQuery(searchQuery(q))
}

export function useNearby(nearby: Nearby | null, categories: string[] | undefined | null) {
  return useQuery(nearbyQuery(nearby, categories))
}

export function useLocateDistrict(location: MyLocation | null) {
  return useQuery(locateQuery(location))
}
```

- [ ] **Step 4: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build
```

预期：全部通过，没有警告。然后：

```bash
git add frontend && git commit -m "feat(frontend): add search, nearby and locate queries"
```

---

### Task 3: 地图图层、定位与右键菜单

**Files:**
- Modify: `frontend/src/map/layers.ts`
- Create: `frontend/src/map/layers.test.ts`
- Create: `frontend/src/map/padding.ts`、`frontend/src/map/focus.ts`
- Modify: `frontend/src/map/DistrictLayer.tsx`（改用共享的 `FIT_PADDING`）
- Modify: `frontend/src/map/MapView.tsx`（设置 `locale`）
- Create: `frontend/src/map/MyLocationLayer.tsx`、`frontend/src/map/NearbyLayer.tsx`、`frontend/src/map/ContextMenu.tsx`
- Modify: `frontend/src/map/MapInteractions.tsx`
- Create: `frontend/src/hooks/useDebounce.ts`、`frontend/src/hooks/useLocate.ts`

本任务新建的图层组件和 hook 在 Task 4 才挂到页面上。

- [ ] **Step 1: 编写图层顺序的测试**

`frontend/src/map/layers.test.ts`：

```ts
import type { Map as MaplibreMap } from 'maplibre-gl'
import { describe, expect, it } from 'vitest'
import { addDataLayers, LAYER, SOURCE } from './layers'

type Call = { kind: 'source'; id: string } | { kind: 'layer'; id: string; source: string }

// 只记录调用顺序的假地图：addDataLayers 只用到 addSource 和 addLayer，不需要 WebGL。
function recordingMap(calls: Call[]): MaplibreMap {
  return {
    addSource: (id: string) => calls.push({ kind: 'source', id }),
    addLayer: (layer: { id: string; source: string }) => calls.push({ kind: 'layer', id: layer.id, source: layer.source }),
  } as unknown as MaplibreMap
}

describe('addDataLayers', () => {
  it('adds the layers bottom to top in the documented order', () => {
    const calls: Call[] = []

    addDataLayers(recordingMap(calls))

    expect(calls.filter((c) => c.kind === 'layer').map((c) => c.id)).toEqual([
      LAYER.districtFill,
      LAYER.districtLine,
      LAYER.districtLabel,
      LAYER.nearbyFill,
      LAYER.nearbyLine,
      LAYER.poiCluster,
      LAYER.poiClusterCount,
      LAYER.poiPoint,
      LAYER.myLocationAccuracy,
      LAYER.myLocationDot,
    ])
  })

  it('adds every source before the first layer that uses it', () => {
    const calls: Call[] = []

    addDataLayers(recordingMap(calls))

    const added = new Set<string>()
    for (const call of calls) {
      if (call.kind === 'source') added.add(call.id)
      else expect(added.has(call.source), `${call.id} uses ${call.source}`).toBe(true)
    }
    expect([...added].sort()).toEqual(Object.values(SOURCE).sort())
  })
})
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd frontend && npm test
```

预期：`LAYER.nearbyFill` 等常量不存在，测试失败。

- [ ] **Step 3: 在 `layers.ts` 中加入范围圆和我的位置**

`frontend/src/map/layers.ts`（整个文件替换）：

```ts
import type { FeatureCollection } from 'geojson'
import type { Map as MaplibreMap, StyleSpecification } from 'maplibre-gl'
import { FALLBACK_POI_COLOR } from '../lib/categories'

export const SOURCE = {
  districts: 'districts',
  districtLabels: 'district-labels',
  nearbyCircle: 'nearby-circle',
  pois: 'pois',
  myLocation: 'my-location',
} as const

export const LAYER = {
  districtFill: 'district-fill',
  districtLine: 'district-line',
  districtLabel: 'district-label',
  nearbyFill: 'nearby-fill',
  nearbyLine: 'nearby-line',
  poiCluster: 'poi-cluster',
  poiClusterCount: 'poi-cluster-count',
  poiPoint: 'poi-point',
  myLocationAccuracy: 'my-location-accuracy',
  myLocationDot: 'my-location-dot',
} as const

export const EMPTY_COLLECTION: FeatureCollection = { type: 'FeatureCollection', features: [] }

export function baseStyle(tiles: string[]): StyleSpecification {
  return {
    version: 8,
    sources: {
      basemap: { type: 'raster', tiles, tileSize: 256, maxzoom: 18, attribution: '底图 © 高德地图' },
    },
    layers: [{ id: 'basemap', type: 'raster', source: 'basemap' }],
  }
}

// 所有数据图层在样式加载完成时一次建好（数据先为空），保证从下到上的顺序固定；之后只更新数据和样式。
export function addDataLayers(map: MaplibreMap): void {
  map.addSource(SOURCE.districts, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    promoteId: 'adcode',
    attribution: '行政区划 © 阿里云 DataV.GeoAtlas',
  })
  map.addSource(SOURCE.districtLabels, { type: 'geojson', data: EMPTY_COLLECTION })
  map.addSource(SOURCE.nearbyCircle, { type: 'geojson', data: EMPTY_COLLECTION })
  map.addSource(SOURCE.pois, {
    type: 'geojson',
    data: EMPTY_COLLECTION,
    cluster: true,
    clusterRadius: 50,
    clusterMaxZoom: 14,
    attribution: '© OpenStreetMap contributors（ODbL）',
  })
  map.addSource(SOURCE.myLocation, { type: 'geojson', data: EMPTY_COLLECTION })

  map.addLayer({
    id: LAYER.districtFill,
    type: 'fill',
    source: SOURCE.districts,
    paint: {
      'fill-color': '#2563eb',
      'fill-opacity': [
        'case',
        ['boolean', ['feature-state', 'selected'], false],
        0.22,
        ['boolean', ['feature-state', 'hover'], false],
        0.14,
        0.04,
      ],
    },
  })
  map.addLayer({
    id: LAYER.districtLine,
    type: 'line',
    source: SOURCE.districts,
    paint: { 'line-color': '#1d4ed8', 'line-width': 1.2, 'line-opacity': 0.8 },
  })
  map.addLayer({
    id: LAYER.districtLabel,
    type: 'symbol',
    source: SOURCE.districtLabels,
    layout: { 'text-field': ['get', 'name'], 'text-size': 13 },
    paint: { 'text-color': '#1e3a8a', 'text-halo-color': '#ffffff', 'text-halo-width': 1.5 },
  })
  map.addLayer({
    id: LAYER.nearbyFill,
    type: 'fill',
    source: SOURCE.nearbyCircle,
    paint: { 'fill-color': '#0ea5e9', 'fill-opacity': 0.1 },
  })
  map.addLayer({
    id: LAYER.nearbyLine,
    type: 'line',
    source: SOURCE.nearbyCircle,
    paint: { 'line-color': '#0284c7', 'line-width': 2, 'line-dasharray': [2, 1] },
  })
  map.addLayer({
    id: LAYER.poiCluster,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    paint: {
      'circle-color': '#f97316',
      'circle-opacity': 0.85,
      'circle-radius': ['step', ['get', 'point_count'], 14, 50, 18, 200, 24],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1.5,
    },
  })
  map.addLayer({
    id: LAYER.poiClusterCount,
    type: 'symbol',
    source: SOURCE.pois,
    filter: ['has', 'point_count'],
    layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-size': 12, 'text-allow-overlap': true },
    paint: { 'text-color': '#ffffff' },
  })
  map.addLayer({
    id: LAYER.poiPoint,
    type: 'circle',
    source: SOURCE.pois,
    filter: ['!', ['has', 'point_count']],
    paint: {
      'circle-color': FALLBACK_POI_COLOR,
      'circle-radius': 5,
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 1,
    },
  })
  map.addLayer({
    id: LAYER.myLocationAccuracy,
    type: 'fill',
    source: SOURCE.myLocation,
    filter: ['==', ['geometry-type'], 'Polygon'],
    paint: { 'fill-color': '#2563eb', 'fill-opacity': 0.15, 'fill-outline-color': '#2563eb' },
  })
  map.addLayer({
    id: LAYER.myLocationDot,
    type: 'circle',
    source: SOURCE.myLocation,
    filter: ['==', ['geometry-type'], 'Point'],
    paint: { 'circle-color': '#2563eb', 'circle-radius': 7, 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 2 },
  })
}
```

- [ ] **Step 4: 共享的缩放边距与聚焦函数**

`frontend/src/map/padding.ts`：

```ts
// 左上角的搜索框和面板宽 320 像素（left-4 + w-80），缩放到某个范围时给它们留出位置。
export const FIT_PADDING = { top: 60, bottom: 60, left: 360, right: 60 }

export const FOCUS_ZOOM = 16
```

`frontend/src/map/focus.ts`：

```ts
import type { Map as MaplibreMap } from 'maplibre-gl'
import { toDisplay } from '../config'
import type { LngLat } from '../lib/coord'
import { useAppStore } from '../store/useAppStore'
import { FOCUS_ZOOM } from './padding'

// 搜索结果和附近列表共用：地图飞到 POI（WGS-84 坐标）并打开它的详情弹窗。
export function focusPoi(map: MaplibreMap, id: number, location: LngLat): void {
  map.flyTo({ center: toDisplay(location), zoom: FOCUS_ZOOM })
  useAppStore.getState().openPoi(id)
}
```

`frontend/src/map/DistrictLayer.tsx`：删除文件里的 `FIT_PADDING` 常量及其上方的注释（「左侧面板约 300 像素宽……」），在 `import { useMap } from './MapView'` 之后加一行 `import { FIT_PADDING } from './padding'`。其余不变。

`frontend/src/map/MapView.tsx`：在创建地图的选项中 `hash: true,` 的下一行加入 `locale: { 'Popup.Close': '关闭' },`。

- [ ] **Step 5: 图层组件与右键菜单**

`frontend/src/map/MyLocationLayer.tsx`：

```tsx
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
```

`frontend/src/map/NearbyLayer.tsx`：

```tsx
import type { FeatureCollection } from 'geojson'
import type { GeoJSONSource } from 'maplibre-gl'
import { useEffect } from 'react'
import { toDisplay } from '../config'
import { circlePolygon } from '../lib/circle'
import { bboxOfGeometry, transformCollection } from '../lib/geojson'
import { useAppStore } from '../store/useAppStore'
import { EMPTY_COLLECTION, SOURCE } from './layers'
import { useMap } from './MapView'
import { FIT_PADDING } from './padding'

export function NearbyLayer() {
  const map = useMap()
  const nearby = useAppStore((s) => s.nearby)

  useEffect(() => {
    const source = map.getSource<GeoJSONSource>(SOURCE.nearbyCircle)
    if (!nearby) {
      source?.setData(EMPTY_COLLECTION)
      return
    }
    const collection: FeatureCollection = {
      type: 'FeatureCollection',
      features: [circlePolygon(nearby.center, nearby.radius)],
    }
    const display = transformCollection(collection, toDisplay)
    source?.setData(display)
    const [minLng, minLat, maxLng, maxLat] = bboxOfGeometry(display.features[0].geometry)
    map.fitBounds(
      [
        [minLng, minLat],
        [maxLng, maxLat],
      ],
      { padding: FIT_PADDING },
    )
  }, [map, nearby])

  return null
}
```

`frontend/src/map/ContextMenu.tsx`：

```tsx
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
```

- [ ] **Step 6: 点击分发改用 `routeClick`，加入点选模式和右键**

`frontend/src/map/MapInteractions.tsx`（整个文件替换）：

```tsx
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
```

- [ ] **Step 7: hooks**

`frontend/src/hooks/useDebounce.ts`：

```ts
import { useEffect, useState } from 'react'

export function useDebounce<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])

  return debounced
}
```

`frontend/src/hooks/useLocate.ts`：

```ts
import { useCallback } from 'react'
import { requestPosition } from '../lib/geolocation'
import { useAppStore } from '../store/useAppStore'

// 浏览器定位成功就写入位置；失败（包括拒绝授权、超时、不支持）就进入在地图上点选的模式。
export function useLocate(): () => void {
  return useCallback(() => {
    const { showInfo, setMyLocation, startPickLocation } = useAppStore.getState()
    showInfo('正在获取你的位置…')
    requestPosition(navigator.geolocation).then(setMyLocation, () => {
      useAppStore.getState().dismissNotice()
      startPickLocation()
    })
  }, [])
}
```

- [ ] **Step 8: 运行全部前端检查并提交**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build
```

预期：全部通过，没有警告。然后：

```bash
git add frontend && git commit -m "feat(frontend): add my-location and nearby layers, pick mode and context menu"
```

---

### Task 4: 搜索框、附近面板、按钮与页面布局

**Files:**
- Create: `frontend/src/components/SearchBox.tsx`、`frontend/src/components/NearbyPanel.tsx`、`frontend/src/components/MapControls.tsx`、`frontend/src/components/PickLocationHint.tsx`
- Modify: `frontend/src/components/DistrictPanel.tsx`（定位交给外层的列）
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: 搜索框**

`frontend/src/components/SearchBox.tsx`：

```tsx
import { useState, type KeyboardEvent } from 'react'
import { useCategories, useSearch } from '../api/queries'
import type { SearchHit } from '../api/types'
import { useDebounce } from '../hooks/useDebounce'
import { FALLBACK_POI_COLOR } from '../lib/categories'
import { focusPoi } from '../map/focus'
import { useMap } from '../map/MapView'

const SEARCH_DEBOUNCE_MS = 300
const MAX_QUERY_LENGTH = 50

export function SearchBox() {
  const map = useMap()
  const [text, setText] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const query = useDebounce(text.trim(), SEARCH_DEBOUNCE_MS)
  const { data: hits, isFetching } = useSearch(query)
  const { data: categories } = useCategories()

  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  const results = query === '' ? [] : (hits ?? [])
  const showDropdown = open && query !== '' && hits !== undefined

  const choose = (hit: SearchHit) => {
    setText(hit.name)
    setOpen(false)
    setActive(-1)
    focusPoi(map, hit.id, hit.location)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' && results.length > 0) {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (i + 1) % results.length)
    } else if (e.key === 'ArrowUp' && results.length > 0) {
      e.preventDefault()
      setActive((i) => (i <= 0 ? results.length - 1 : i - 1))
    } else if (e.key === 'Enter' && results.length > 0) {
      e.preventDefault()
      choose(results[Math.max(active, 0)])
    } else if (e.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <div className="pointer-events-auto relative">
      <input
        type="search"
        value={text}
        maxLength={MAX_QUERY_LENGTH}
        placeholder="搜索地铁站、高校、景点…"
        aria-label="搜索地点"
        role="combobox"
        aria-expanded={showDropdown}
        aria-controls="search-results"
        onChange={(e) => {
          setText(e.target.value)
          setOpen(true)
          setActive(-1)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={onKeyDown}
        className="w-full rounded-lg bg-white/95 px-3 py-2 text-sm text-slate-900 shadow-lg ring-1 ring-slate-200 outline-none placeholder:text-slate-400 focus:ring-2 focus:ring-blue-400"
      />
      {showDropdown && (
        <ul
          id="search-results"
          role="listbox"
          className="absolute left-0 right-0 top-full z-20 mt-1 max-h-96 overflow-y-auto rounded-lg bg-white py-1 shadow-lg ring-1 ring-slate-200"
        >
          {results.length === 0 && (
            <li className="px-3 py-2 text-sm text-slate-500">{isFetching ? '搜索中…' : '没有找到相关地点'}</li>
          )}
          {results.map((hit, i) => (
            <li
              key={hit.id}
              role="option"
              aria-selected={i === active}
              // 用 mousedown 而不是 click：click 之前输入框会先失焦，下拉框已经关掉了。
              onMouseDown={(e) => {
                e.preventDefault()
                choose(hit)
              }}
              onMouseEnter={() => setActive(i)}
              className={`flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm ${i === active ? 'bg-slate-100' : ''}`}
            >
              <span
                className="h-2.5 w-2.5 shrink-0 rounded-full"
                style={{ backgroundColor: colors.get(hit.category) ?? FALLBACK_POI_COLOR }}
              />
              <span className="flex-1 truncate text-slate-900">{hit.name}</span>
              {hit.district_name && <span className="shrink-0 text-xs text-slate-500">{hit.district_name}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
```

- [ ] **Step 2: 附近面板**

`frontend/src/components/NearbyPanel.tsx`：

```tsx
import { useMemo } from 'react'
import { NEARBY_LIMIT, useCategories, useNearby } from '../api/queries'
import { categoriesParam, FALLBACK_POI_COLOR } from '../lib/categories'
import type { LngLat } from '../lib/coord'
import { formatDistance } from '../lib/format'
import { focusPoi } from '../map/focus'
import { useMap } from '../map/MapView'
import { NEARBY_RADII, useAppStore } from '../store/useAppStore'

export function NearbyPanel() {
  const map = useMap()
  const nearby = useAppStore((s) => s.nearby)
  const hidden = useAppStore((s) => s.hiddenCategories)
  const setNearbyRadius = useAppStore((s) => s.setNearbyRadius)
  const closeNearby = useAppStore((s) => s.closeNearby)
  const { data: categories } = useCategories()
  const codes = useMemo(() => categories?.map((c) => c.code) ?? [], [categories])
  const requested = categoriesParam(codes, hidden)
  const { data, isPending } = useNearby(nearby, requested)

  if (!nearby) return null
  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  const features = data?.features ?? []
  return (
    <aside
      aria-label="附近查询"
      className="pointer-events-auto flex max-h-[calc(100vh-7rem)] flex-col rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200"
    >
      <div className="flex items-start justify-between gap-2">
        <h2 className="text-lg font-semibold text-slate-900">附近</h2>
        <button
          type="button"
          aria-label="关闭附近查询"
          onClick={closeNearby}
          className="rounded px-1.5 text-lg leading-none text-slate-500 hover:bg-slate-100 hover:text-slate-800"
        >
          ×
        </button>
      </div>
      <div role="group" aria-label="查询半径" className="mt-2 flex gap-1">
        {NEARBY_RADII.map((r) => (
          <button
            key={r}
            type="button"
            aria-pressed={nearby.radius === r}
            onClick={() => setNearbyRadius(r)}
            className={`flex-1 rounded px-2 py-1 text-sm ${
              nearby.radius === r ? 'bg-sky-600 text-white' : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
            }`}
          >
            {formatDistance(r)}
          </button>
        ))}
      </div>
      <p className="mt-2 text-xs text-slate-500">
        {requested === null
          ? '分类已全部关闭'
          : isPending
            ? '查询中…'
            : features.length === 0
              ? '范围内没有地点'
              : features.length === NEARBY_LIMIT
                ? `仅显示最近的 ${NEARBY_LIMIT} 个`
                : `共 ${features.length} 个`}
      </p>
      <ul className="mt-2 -mx-1 flex-1 overflow-y-auto">
        {features.map((f) => (
          <li key={f.properties.id}>
            <button
              type="button"
              onClick={() => focusPoi(map, f.properties.id, f.geometry.coordinates as LngLat)}
              className="flex w-full items-center gap-2 rounded px-1 py-1.5 text-left text-sm hover:bg-slate-100"
            >
              <span
                className="h-2.5 w-2.5 shrink-0 rounded-full"
                style={{ backgroundColor: colors.get(f.properties.category) ?? FALLBACK_POI_COLOR }}
              />
              <span className="flex-1 truncate text-slate-800">{f.properties.name}</span>
              <span className="shrink-0 tabular-nums text-slate-500">{formatDistance(f.properties.distance_m)}</span>
            </button>
          </li>
        ))}
      </ul>
    </aside>
  )
}
```

- [ ] **Step 3: 定位和附近按钮、点选提示**

`frontend/src/components/MapControls.tsx`：

```tsx
import { useLocate } from '../hooks/useLocate'
import { useAppStore } from '../store/useAppStore'

const BUTTON_CLASS =
  'pointer-events-auto rounded-lg bg-white/95 px-3 py-2 text-sm font-medium text-slate-800 shadow-lg ring-1 ring-slate-200 hover:bg-slate-50'

export function MapControls() {
  const locate = useLocate()

  const searchNearby = () => {
    const { myLocation, openNearby, requestNearbyAfterLocate } = useAppStore.getState()
    if (myLocation) {
      openNearby([myLocation.lng, myLocation.lat])
      return
    }
    requestNearbyAfterLocate()
    locate()
  }

  return (
    <div className="pointer-events-none absolute bottom-10 right-3 z-10 flex flex-col gap-2">
      <button type="button" onClick={locate} className={BUTTON_CLASS}>
        定位
      </button>
      <button type="button" onClick={searchNearby} className={BUTTON_CLASS}>
        附近
      </button>
    </div>
  )
}
```

`frontend/src/components/PickLocationHint.tsx`：

```tsx
import { useAppStore } from '../store/useAppStore'

export function PickLocationHint() {
  const mode = useAppStore((s) => s.mode)
  const cancelPickLocation = useAppStore((s) => s.cancelPickLocation)

  if (mode !== 'pickLocation') return null
  return (
    <div
      role="status"
      className="absolute bottom-10 left-1/2 z-10 flex max-w-lg -translate-x-1/2 items-start gap-3 rounded-lg bg-white/95 px-4 py-3 text-sm text-slate-700 shadow-lg ring-1 ring-slate-200"
    >
      <p className="flex-1">
        <span className="font-medium text-slate-900">浏览器定位失败，请在地图上点击你的位置。</span>
        桌面版 Chrome 在国内依赖的 Google 定位服务通常不可用，可以改用 Safari。
      </p>
      <button type="button" onClick={cancelPickLocation} className="shrink-0 rounded px-2 py-0.5 text-slate-500 hover:bg-slate-100">
        取消
      </button>
    </div>
  )
}
```

- [ ] **Step 4: 区详情面板交给左侧的列定位**

`frontend/src/components/DistrictPanel.tsx`：把 `<aside>` 的 `className` 从 `"absolute left-4 top-4 z-10 w-72 rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200"` 改为 `"pointer-events-auto rounded-lg bg-white/95 p-4 shadow-lg ring-1 ring-slate-200"`。其余不变。

- [ ] **Step 5: 页面布局**

`frontend/src/App.tsx`（整个文件替换）：

```tsx
import { CategoryFilter } from './components/CategoryFilter'
import { DistrictPanel } from './components/DistrictPanel'
import { MapControls } from './components/MapControls'
import { NearbyPanel } from './components/NearbyPanel'
import { NoticeBanner } from './components/NoticeBanner'
import { PickLocationHint } from './components/PickLocationHint'
import { SearchBox } from './components/SearchBox'
import { ContextMenu } from './map/ContextMenu'
import { DistrictLayer } from './map/DistrictLayer'
import { MapInteractions } from './map/MapInteractions'
import { MapView } from './map/MapView'
import { MyLocationLayer } from './map/MyLocationLayer'
import { NearbyLayer } from './map/NearbyLayer'
import { PoiLayer } from './map/PoiLayer'
import { PoiPopup } from './map/PoiPopup'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView>
        <DistrictLayer />
        <NearbyLayer />
        <PoiLayer />
        <MyLocationLayer />
        <MapInteractions />
        <PoiPopup />
        <ContextMenu />
        <div className="pointer-events-none absolute left-4 top-4 z-10 flex w-80 flex-col gap-2">
          <SearchBox />
          <DistrictPanel />
          <NearbyPanel />
        </div>
        <MapControls />
      </MapView>
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <NoticeBanner />
        <CategoryFilter />
      </div>
      <PickLocationHint />
    </main>
  )
}
```

- [ ] **Step 6: 运行全部前端检查**

```bash
cd frontend && npm run lint && npm run typecheck && npm test && npm run build
```

预期：全部通过，没有警告。

- [ ] **Step 7: 冒烟验证**

```bash
FRONTEND_PORT=18080 docker compose up -d --build frontend backend
```

等 backend 健康后，用 curl 确认页面和三个接口都正常（中文参数要 URL 编码）：

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:18080/
curl -s 'http://localhost:18080/api/v1/pois/search?q=%E5%8C%97%E4%BA%AC%E5%A4%A7%E5%AD%A6&limit=10' | head -c 200; echo
curl -s 'http://localhost:18080/api/v1/pois/nearby?lng=116.397&lat=39.908&radius=1000&limit=100' | head -c 200; echo
curl -s 'http://localhost:18080/api/v1/districts/locate?lng=116.70&lat=39.53'; echo
```

预期：页面 200；搜索返回以北京大学开头的结果；附近返回 `FeatureCollection`；廊坊的坐标返回 `{"data":null}`。浏览器验收在整个阶段结束时统一进行。然后执行 `docker compose stop frontend backend`。

- [ ] **Step 8: 提交**

```bash
git add frontend && git commit -m "feat(frontend): add search box, nearby panel and locate controls"
```

---

### Task 5: 文档

**Files:**
- Modify: `docs/02-technical-design.md`
- Modify: `README.md`

- [ ] **Step 1: 更新技术方案**

`docs/02-technical-design.md`：

1. 第 2 节目录结构中，把 `hooks/`、`components/`、`map/` 三行的注释分别改为：
   - `map/`：`# 地图实例、图层定义、各图层组件、点击分发、POI 弹窗、右键菜单`
   - `hooks/`：`# useLocate（浏览器定位，失败时转入点选）、useDebounce`
   - `components/`：`# 搜索框、区详情面板、附近面板、POI 详情卡片、分类开关、定位和附近按钮、点选提示、提示条`
2. 第 6.1 节 store 表格：删除 `errorMessage`、`myLocation`、`nearby`、`mode` 四行，在 `activePoiId` 之后依次加入：

   | 字段 | 含义 |
   |---|---|
   | `notice` | 右上角提示条的内容，格式为 `{tone, message}`：`info` 为普通提示，4 秒后自动消失；`error` 为错误提示，一直显示到用户关闭或被新的提示替换 |
   | `mode` | 当前模式：`normal`（普通）或 `pickLocation`（在地图上点选位置） |
   | `myLocation` | 我的位置，格式为 `{lng, lat, accuracy}`，WGS-84；在地图上点选的位置没有精度，`accuracy` 为 `null` |
   | `nearby` | 附近查询的中心和半径，格式为 `{center, radius}` 或 `null` |
   | `nearbyAfterLocate` | 点击「附近」时还没有位置，会先定位；为 `true` 表示定位或点选完成后接着打开附近查询 |
   | `contextMenu` | 右键菜单的位置（WGS-84），为 `null` 表示菜单关闭 |

3. 第 6.2 节改为：

   ```markdown
   - 地图占满全屏。
   - 左上角：搜索框，下面是浮动面板，显示区详情或附近查询结果，两者互斥：选中区会关闭附近查询，打开附近查询会取消选中的区。搜索框和面板宽 320 像素，地图缩放到某个区或查询范围时，左侧留出 360 像素的边距。
   - 右上角：提示条，下面是分类开关，用分类颜色标识。
   - 右下角：「定位」和「附近」两个按钮。
   - 底部中间：点选位置模式的提示和「取消」按钮。
   - 右下角底部：MapLibre 的署名控件显示各数据源的署名：「底图 © 高德地图」「行政区划 © 阿里云 DataV.GeoAtlas」「© OpenStreetMap contributors（ODbL）」。
   ```

4. 第 6.3 节图层表中，`nearby-circle` 和 `my-location` 两行的说明改为：
   - `nearby-circle`：`用 @turf/circle 生成 64 边形，显示查询范围：浅色填充加虚线边框`（`@turf/circle` 保留反引号）
   - `my-location`：`蓝色圆点；浏览器定位得到的位置还显示精度圈。精度圈同样用 @turf/circle 按 accuracy 的米数生成多边形，在任何缩放级别都与底图比例一致`（`@turf/circle`、`accuracy` 保留反引号）
5. 第 6.4 节：
   - F3 第 4 条开头改为：「点击统一由一个处理函数分发，判断规则在 `lib/clickRoute.ts`：点选位置模式下任何点击都用来设置位置；否则先查 POI 图层，再查区图层，……」（后半句保持原文）。
   - F2、F4、F5 三段整体替换为下面的内容（「第一版以桌面端为主……」一句保留在 F5 之后）：

   ```markdown
   **F2 我的位置**

   1. 点击定位按钮，先提示「正在获取你的位置…」，再调用 `navigator.geolocation.getCurrentPosition`（启用高精度，10 秒超时，不使用缓存的位置）。`localhost` 属于安全上下文，可以直接使用定位接口。
   2. 定位成功：写入 `myLocation`，地图飞到该位置（缩放级别至少 14），显示蓝点和精度圈，请求 `/districts/locate`，提示「你在海淀区」，或者「当前位置不在北京市范围内」。
   3. 定位失败、用户拒绝授权或浏览器不支持定位：进入 `pickLocation` 模式，底部显示「浏览器定位失败，请在地图上点击你的位置」和「取消」按钮。点选模式下鼠标显示为十字，任何点击都用来设置位置，不会打开 POI 或选中区，右键也不弹出菜单。点击后把坐标经 `toData()` 转换并写入 `myLocation`（没有精度，只画蓝点），后续流程同上。
   4. 定位失败的提示文案会说明：桌面版 Chrome 在国内依赖的 Google 定位服务通常不可用，建议使用 Safari。

   **F4 搜索**

   1. 输入防抖 300ms，然后请求 `/pois/search`（`limit=10`）。输入为空时不请求；输入框最多 50 个字符，与接口的限制一致。
   2. 下拉框最多显示 10 条结果，包括名称、分类色块和所在区；支持上下键选择、回车确认（没有选中时取第一条）和 Esc 关闭。没有结果时显示「没有找到相关地点」。
   3. 选中后地图飞到该点（缩放级别 16），并打开详情弹窗。

   **F5 附近**

   1. 点击「附近」按钮时，以 `myLocation` 为中心；如果还没有定位，先走一遍定位流程（包括失败后的点选），完成后自动打开附近查询，在点选模式中点「取消」会一并放弃。也可以在地图上右键，选择「以此为中心查附近」；右键菜单在点击或移动地图时关闭。
   2. 半径有 500、1000、2000、5000 米四档，默认 1000 米；换中心时保留当前半径。
   3. 请求 `/pois/nearby`（`limit=100`，即接口上限；分类开关的规则与 `/pois` 相同），地图画出范围圆并缩放到它，面板列出结果和距离，例如「320 m」或「1.2 km」。结果正好 100 个时提示「仅显示最近的 100 个」：2 千米内通常有 80 到 100 个 POI，5 千米内约 340 个。
   4. 点击某条结果，地图飞过去（缩放级别 16）并打开弹窗。
   5. 关闭面板时清除范围圆。重新定位不会移动已经打开的附近查询，需要再点一次「附近」。

   **弹窗与分类开关**互不影响：搜索结果和附近列表可以打开任何分类的 POI 详情，包括当前关闭的分类；关闭某个分类时，已经打开的该分类弹窗不会关闭。附近列表和地图图层一样遵循分类开关，搜索不受分类开关影响。
   ```

6. 第 7.2 节 Vitest 一行的范围改为：「`lib/coord.ts`（读取同一份共享样例）、`lib/geojson.ts` 的递归坐标转换、`lib/bbox.ts`、`lib/tiles.ts`、`lib/categories.ts`、`lib/format.ts`、`lib/circle.ts`、`lib/clickRoute.ts` 的点击分发规则、`lib/geolocation.ts`（用假的 `Geolocation` 对象）、`store/useAppStore.ts`、`map/layers.ts` 的图层顺序（用只记录调用的假地图）、`api/client.ts` 的参数拼接、错误解析和 `data` 解包、`api/queries.ts` 的查询键和跳过条件（用 TanStack Query 的 `QueryObserver` 在 Node 中运行）。其他地图图层组件依赖 WebGL，不写单元测试，逻辑尽量放进上面这些纯函数」。

全文搜索「阶段 4」，第 2 节和第 6 节中标注「（阶段 4）」的地方都应该已经随上面的修改去掉；第 7.2 节「阶段 3 和阶段 4 完成后」一句保留。

- [ ] **Step 2: 更新 README**

`README.md`：

1. 第 7 行「当前状态」改为：「**当前状态：开发中。** 后端 API 和地图前端的主要功能已经完成：16 个区的边界、按视野加载并聚合显示的 POI、分类筛选和详情、浏览器定位（失败时可以在地图上点选）、POI 搜索和附近查询。剩余的打磨和交付工作见[实现文档](docs/01-implementation-plan.md#5-阶段计划)。」
2. 「功能（第一版）」中「我的位置」一条末尾补一句：「桌面版 Chrome 在国内通常无法定位，建议使用 Safari。」

- [ ] **Step 3: 提交**

```bash
git add docs README.md && git commit -m "docs: describe locate, search and nearby behaviour"
```
