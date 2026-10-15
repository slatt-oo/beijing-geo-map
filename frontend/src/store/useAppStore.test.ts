import { beforeEach, describe, expect, it } from 'vitest'
import { useAppStore } from './useAppStore'

const initial = useAppStore.getState()

describe('useAppStore', () => {
  beforeEach(() => {
    useAppStore.setState(initial, true)
  })

  it('toggles a category off and back on', () => {
    const { toggleCategory } = useAppStore.getState()

    toggleCategory('hospital')
    toggleCategory('park')
    expect(useAppStore.getState().hiddenCategories).toEqual(['hospital', 'park'])

    toggleCategory('hospital')
    expect(useAppStore.getState().hiddenCategories).toEqual(['park'])
  })

  it('shows and dismisses an error message', () => {
    useAppStore.getState().showError('网络错误')
    expect(useAppStore.getState().errorMessage).toBe('网络错误')

    useAppStore.getState().dismissError()
    expect(useAppStore.getState().errorMessage).toBeNull()
  })
})
