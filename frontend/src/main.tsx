import { QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { errorMessage } from './api/client'
import App from './App'
import { useAppStore } from './store/useAppStore'
import './index.css'

const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => useAppStore.getState().showError(errorMessage(error)),
  }),
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
