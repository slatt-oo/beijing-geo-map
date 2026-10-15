import { CategoryFilter } from './components/CategoryFilter'
import { DistrictPanel } from './components/DistrictPanel'
import { NoticeBanner } from './components/NoticeBanner'
import { DistrictLayer } from './map/DistrictLayer'
import { MapInteractions } from './map/MapInteractions'
import { MapView } from './map/MapView'
import { PoiLayer } from './map/PoiLayer'
import { PoiPopup } from './map/PoiPopup'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView>
        <DistrictLayer />
        <PoiLayer />
        <MapInteractions />
        <PoiPopup />
      </MapView>
      <DistrictPanel />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <NoticeBanner />
        <CategoryFilter />
      </div>
    </main>
  )
}
