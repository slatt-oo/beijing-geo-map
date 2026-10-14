import { ErrorBanner } from './components/ErrorBanner'
import { MapView } from './map/MapView'

export default function App() {
  return (
    <main className="relative h-full w-full overflow-hidden">
      <MapView />
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex flex-col items-end gap-2">
        <ErrorBanner />
      </div>
    </main>
  )
}
