import { requestPosition } from '../lib/geolocation'
import { useAppStore } from './useAppStore'

// 同一时间只有一次定位：定位进行中再点「定位」或「附近」不会重复请求，
// 否则先失败的请求会在用户点选位置之后又一次进入点选模式。
export async function locate(geolocation: Geolocation | undefined): Promise<void> {
  const { locating, startLocating } = useAppStore.getState()
  if (locating) return
  startLocating()
  try {
    useAppStore.getState().setMyLocation(await requestPosition(geolocation))
  } catch {
    useAppStore.getState().startPickLocation()
  }
}
