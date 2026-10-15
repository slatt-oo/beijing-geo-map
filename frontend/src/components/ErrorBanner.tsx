import { useAppStore } from '../store/useAppStore'

export function ErrorBanner() {
  const message = useAppStore((s) => s.errorMessage)
  const dismissError = useAppStore((s) => s.dismissError)

  if (!message) return null
  return (
    <div
      role="alert"
      className="pointer-events-auto flex max-w-xs items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 shadow-lg ring-1 ring-red-200"
    >
      <span className="flex-1">{message}</span>
      <button type="button" aria-label="关闭提示" onClick={dismissError} className="leading-none hover:text-red-900">
        ×
      </button>
    </div>
  )
}
