import { useAppStore } from '../store/useAppStore'

export function PickLocationHint() {
  const mode = useAppStore((s) => s.mode)
  const cancelPickLocation = useAppStore((s) => s.cancelPickLocation)

  if (mode !== 'pickLocation') return null
  return (
    <div
      role="status"
      className="absolute bottom-10 left-1/2 z-10 flex max-w-lg -translate-x-1/2 items-start gap-3 rounded-lg bg-white/95 px-4 py-3 text-sm text-slate-700 shadow-lg ring-1 ring-slate-200"
    >
      <p className="flex-1">
        <span className="font-medium text-slate-900">浏览器定位失败，请在地图上点击你的位置。</span>
        桌面版 Chrome 在国内依赖的 Google 定位服务通常不可用，可以改用 Safari。
      </p>
      <button type="button" onClick={cancelPickLocation} className="shrink-0 rounded px-2 py-0.5 text-slate-500 hover:bg-slate-100">
        取消
      </button>
    </div>
  )
}
