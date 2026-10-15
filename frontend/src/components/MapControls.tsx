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
