import { locate } from '../store/locate'
import { useAppStore } from '../store/useAppStore'

const BUTTON_CLASS =
  'pointer-events-auto rounded-lg bg-white/95 px-3 py-2 text-sm font-medium text-slate-800 shadow-lg ring-1 ring-slate-200 hover:bg-slate-50 disabled:cursor-wait disabled:text-slate-400 disabled:hover:bg-white/95'

export function MapControls() {
  const locating = useAppStore((s) => s.locateRequest !== null)

  const searchNearby = () => {
    const { myLocation, openNearby, requestNearbyAfterLocate } = useAppStore.getState()
    if (myLocation) {
      openNearby([myLocation.lng, myLocation.lat])
      return
    }
    requestNearbyAfterLocate()
    void locate(navigator.geolocation)
  }

  return (
    <div className="pointer-events-none absolute bottom-10 right-3 z-10 flex flex-col gap-2">
      <button type="button" disabled={locating} onClick={() => void locate(navigator.geolocation)} className={BUTTON_CLASS}>
        {locating ? '定位中…' : '定位'}
      </button>
      <button type="button" onClick={searchNearby} className={BUTTON_CLASS}>
        附近
      </button>
    </div>
  )
}
