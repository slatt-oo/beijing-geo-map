import { DistrictPanel } from './components/DistrictPanel'
import { ErrorBanner } from './components/ErrorBanner'
import { DistrictLayer } from './map/DistrictLayer'
import { MapInteractions } from './map/MapInteractions'
import { MapView } from './map/MapView'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView>
        <DistrictLayer />
        <MapInteractions />
      </MapView>
      <DistrictPanel />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <ErrorBanner />
      </div>
    </main>
  )
}
