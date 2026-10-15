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
