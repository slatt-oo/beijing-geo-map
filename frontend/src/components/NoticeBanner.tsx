import { useEffect } from 'react'
import { useAppStore } from '../store/useAppStore'

const INFO_NOTICE_MS = 4000

const TONE_CLASS = {
  info: 'bg-sky-50 text-sky-800 ring-sky-200',
  error: 'bg-red-50 text-red-700 ring-red-200',
}

// 普通提示几秒后自动消失；错误提示一直显示，直到用户关闭或被新的提示替换。
export function NoticeBanner() {
  const notice = useAppStore((s) => s.notice)
  const dismissNotice = useAppStore((s) => s.dismissNotice)

  useEffect(() => {
    if (notice?.tone !== 'info') return
    const timer = setTimeout(() => useAppStore.getState().dismissNotice(), INFO_NOTICE_MS)
    return () => clearTimeout(timer)
  }, [notice])

  if (!notice) return null
  return (
    <div
      role={notice.tone === 'error' ? 'alert' : 'status'}
      className={`pointer-events-auto flex max-w-xs items-start gap-2 rounded-lg px-3 py-2 text-sm shadow-lg ring-1 ${TONE_CLASS[notice.tone]}`}
    >
      <span className="flex-1">{notice.message}</span>
      <button type="button" aria-label="关闭提示" onClick={dismissNotice} className="leading-none opacity-70 hover:opacity-100">
        ×
      </button>
    </div>
  )
}
