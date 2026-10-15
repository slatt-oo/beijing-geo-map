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
        <PoiLayer />
        <MyLocationLayer />
        <NearbyLayer />
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
