import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { errorMessage, isRetryable, shouldRetry } from './api/client'
import { isPoiDetailQuery } from './api/queries'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import { useAppStore } from './store/useAppStore'
import './index.css'

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error, query) =>
      useAppStore
        .getState()
        .showError(errorMessage(error), isRetryable(error) && !isPoiDetailQuery(query.queryKey) ? retryFailed : undefined),
  }),
  defaultOptions: {
    queries: { retry: shouldRetry, refetchOnWindowFocus: false },
  },
})

// 区边界和分类的 staleTime 是 Infinity，第一次请求失败后不会自己再请求，只能手动重试。
function retryFailed() {
  void queryClient.refetchQueries({ type: 'active', predicate: (query) => query.state.status === 'error' })
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
)
