import { useEffect, useState } from 'react'

// active 持续 delayMs 之后才返回 true；active 变为 false 时立即返回 false。
export function useDelayedFlag(active: boolean, delayMs: number): boolean {
  const [elapsed, setElapsed] = useState(false)

  useEffect(() => {
    if (!active) return
    const timer = setTimeout(() => setElapsed(true), delayMs)
    return () => {
      clearTimeout(timer)
      setElapsed(false)
    }
  }, [active, delayMs])

  return active && elapsed
}
