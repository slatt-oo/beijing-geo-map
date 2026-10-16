import { Component, type ReactNode } from 'react'
import { describeFatalError } from '../lib/fatalError'

type Props = { children: ReactNode }
type State = { error: unknown }

// 渲染和 effect 里抛出的错误（例如创建地图时没有 WebGL）会卸载整棵组件树，没有这一层就只剩白屏。
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: unknown): State {
    return { error }
  }

  render() {
    const { error } = this.state
    if (error === null) return this.props.children
    return (
      <div role="alert" className="flex h-full items-center justify-center bg-slate-50 p-6">
        <div className="max-w-md rounded-lg bg-white p-6 text-sm text-slate-700 shadow-lg ring-1 ring-slate-200">
          <h1 className="text-lg font-semibold text-slate-900">地图无法显示</h1>
          <p className="mt-2">{describeFatalError(error)}</p>
          <p className="mt-2 break-all text-xs text-slate-400">{error instanceof Error ? error.message : String(error)}</p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="mt-4 rounded-lg bg-sky-600 px-3 py-1.5 font-medium text-white hover:bg-sky-700"
          >
            刷新页面
          </button>
        </div>
      </div>
    )
  }
}
