import type { TileCrs } from './tiles'

export type LngLat = [number, number]

export type CoordTransform = {
  toDisplay: (p: LngLat) => LngLat
  toData: (p: LngLat) => LngLat
}

// 克拉索夫斯基椭球参数，与后端 internal/geo 相同（后端写作 0.00669342162296594323，取 float64 后就是这个值）。
const KRASOVSKY_A = 6378245.0
const KRASOVSKY_EE = 0.006693421622965943

const INVERSE_TOLERANCE = 1e-7
const MAX_INVERSE_ITERATIONS = 10

export function outOfChina(lng: number, lat: number): boolean {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

export function wgs84ToGcj02([lng, lat]: LngLat): LngLat {
  if (outOfChina(lng, lat)) return [lng, lat]
  const [dLng, dLat] = offset(lng, lat)
  return [lng + dLng, lat + dLat]
}

// 没有解析解，用迭代法反算：每轮按正向转换的残差修正估计值，与后端 GCJ02ToWGS84 的循环逐步一致。
export function gcj02ToWgs84([lng, lat]: LngLat): LngLat {
  if (outOfChina(lng, lat)) return [lng, lat]
  let wLng = lng
  let wLat = lat
  for (let i = 0; i < MAX_INVERSE_ITERATIONS; i++) {
    const [gLng, gLat] = wgs84ToGcj02([wLng, wLat])
    const dLng = gLng - lng
    const dLat = gLat - lat
    wLng -= dLng
    wLat -= dLat
    if (Math.abs(dLng) < INVERSE_TOLERANCE && Math.abs(dLat) < INVERSE_TOLERANCE) break
  }
  return [wLng, wLat]
}

// 底图是 GCJ-02 时，接口数据（WGS-84）显示前要转换，从地图上取到的坐标查询前要反算；底图是 WGS-84 时原样使用。
export function createCoordTransform(crs: TileCrs): CoordTransform {
  if (crs === 'wgs84') {
    const same = ([lng, lat]: LngLat): LngLat => [lng, lat]
    return { toDisplay: same, toData: same }
  }
  return { toDisplay: wgs84ToGcj02, toData: gcj02ToWgs84 }
}

function offset(lng: number, lat: number): LngLat {
  const x = lng - 105.0
  const y = lat - 35.0
  let dLat = transformLat(x, y)
  let dLng = transformLng(x, y)
  const radLat = (lat / 180.0) * Math.PI
  const magic = 1 - KRASOVSKY_EE * Math.sin(radLat) * Math.sin(radLat)
  const sqrtMagic = Math.sqrt(magic)
  dLat = (dLat * 180.0) / (((KRASOVSKY_A * (1 - KRASOVSKY_EE)) / (magic * sqrtMagic)) * Math.PI)
  dLng = (dLng * 180.0) / ((KRASOVSKY_A / sqrtMagic) * Math.cos(radLat) * Math.PI)
  return [dLng, dLat]
}

function transformLat(x: number, y: number): number {
  let r = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x))
  r += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  r += ((20.0 * Math.sin(y * Math.PI) + 40.0 * Math.sin((y / 3.0) * Math.PI)) * 2.0) / 3.0
  r += ((160.0 * Math.sin((y / 12.0) * Math.PI) + 320.0 * Math.sin((y * Math.PI) / 30.0)) * 2.0) / 3.0
  return r
}

function transformLng(x: number, y: number): number {
  let r = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x))
  r += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  r += ((20.0 * Math.sin(x * Math.PI) + 40.0 * Math.sin((x / 3.0) * Math.PI)) * 2.0) / 3.0
  r += ((150.0 * Math.sin((x / 12.0) * Math.PI) + 300.0 * Math.sin((x / 30.0) * Math.PI)) * 2.0) / 3.0
  return r
}
