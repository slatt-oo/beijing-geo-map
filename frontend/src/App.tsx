import { useEffect, useState } from 'react'
import { ApiError, apiGet } from './api/client'

type HealthResponse = { data: { status: string; db: string } }

type HealthState = { kind: 'loading' } | { kind: 'ok' } | { kind: 'error'; message: string }

export default function App() {
  const [health, setHealth] = useState<HealthState>({ kind: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    apiGet<HealthResponse>('/health', undefined, controller.signal)
      .then(() => setHealth({ kind: 'ok' }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setHealth({ kind: 'error', message: err instanceof ApiError ? err.message : String(err) })
      })
    return () => controller.abort()
  }, [])

  return (
    <main className="flex h-full flex-col items-center justify-center gap-3 bg-slate-50 text-slate-800">
      <h1 className="text-2xl font-semibold">北京地图 Demo</h1>
      <p className="text-sm">
        后端状态：
        {health.kind === 'loading' && <span className="text-slate-500">检查中…</span>}
        {health.kind === 'ok' && <span className="text-emerald-600">正常</span>}
        {health.kind === 'error' && <span className="text-red-600">{health.message}</span>}
      </p>
    </main>
  )
}
