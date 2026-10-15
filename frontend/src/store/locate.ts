import { requestPosition } from '../lib/geolocation'
import { useAppStore, type MyLocation } from './useAppStore'

// 浏览器的 10 秒超时不包括等待用户授权的时间；授权弹窗被关掉时浏览器可能一直不回应。
export const LOCATE_GIVE_UP_MS = 20_000

// 同一时间只有一次定位。等待期间用户点选了位置或取消点选，这次请求的结果作废，
// 否则迟到的失败会在用户点选之后又一次进入点选模式。
export async function locate(geolocation: Geolocation | undefined): Promise<void> {
  if (useAppStore.getState().locateRequest) return
  const request = {}
  useAppStore.getState().startLocating(request)

  let location: MyLocation | null
  let timer: ReturnType<typeof setTimeout> | undefined
  const giveUp = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error('定位超时')), LOCATE_GIVE_UP_MS)
  })
  try {
    location = await Promise.race([requestPosition(geolocation), giveUp])
  } catch {
    location = null
  } finally {
    clearTimeout(timer)
  }

  const { locateRequest, setMyLocation, startPickLocation } = useAppStore.getState()
  if (locateRequest !== request) return
  if (location) setMyLocation(location)
  else startPickLocation()
}
