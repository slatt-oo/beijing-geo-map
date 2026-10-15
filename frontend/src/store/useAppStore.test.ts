import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_NEARBY_RADIUS, useAppStore } from './useAppStore'

const initial = useAppStore.getState()
const state = () => useAppStore.getState()

describe('useAppStore', () => {
  beforeEach(() => {
    useAppStore.setState(initial, true)
  })

  it('toggles a category off and back on', () => {
    state().toggleCategory('hospital')
    state().toggleCategory('park')
    expect(state().hiddenCategories).toEqual(['hospital', 'park'])

    state().toggleCategory('hospital')
    expect(state().hiddenCategories).toEqual(['park'])
  })

  it('shows info and error notices and dismisses them', () => {
    state().showInfo('你在海淀区')
    expect(state().notice).toEqual({ tone: 'info', message: '你在海淀区' })

    state().showError('网络错误')
    expect(state().notice).toEqual({ tone: 'error', message: '网络错误' })

    state().dismissNotice()
    expect(state().notice).toBeNull()
  })

  it('leaves pick mode once a location is set', () => {
    state().startPickLocation()
    expect(state().mode).toBe('pickLocation')

    state().setMyLocation({ lng: 116.39, lat: 39.9, accuracy: null })

    expect(state().mode).toBe('normal')
    expect(state().myLocation).toEqual({ lng: 116.39, lat: 39.9, accuracy: null })
    expect(state().nearby).toBeNull()
  })

  it('opens the nearby search after locating when it was requested first', () => {
    state().selectDistrict('110108')
    state().requestNearbyAfterLocate()

    state().setMyLocation({ lng: 116.31, lat: 39.99, accuracy: 30 })

    expect(state().nearby).toEqual({ center: [116.31, 39.99], radius: DEFAULT_NEARBY_RADIUS })
    expect(state().selectedDistrict).toBeNull()
    expect(state().nearbyAfterLocate).toBe(false)
  })

  it('forgets a pending nearby search when picking is cancelled', () => {
    state().requestNearbyAfterLocate()
    state().startPickLocation()

    state().cancelPickLocation()
    state().setMyLocation({ lng: 116.39, lat: 39.9, accuracy: null })

    expect(state().mode).toBe('normal')
    expect(state().nearby).toBeNull()
  })

  it('shows district detail and nearby results one at a time', () => {
    state().selectDistrict('110108')
    state().openNearby([116.4, 39.9])
    expect(state().selectedDistrict).toBeNull()
    expect(state().nearby).toEqual({ center: [116.4, 39.9], radius: DEFAULT_NEARBY_RADIUS })

    state().selectDistrict('110105')
    expect(state().nearby).toBeNull()
  })

  it('keeps the chosen radius when the nearby center moves', () => {
    state().openNearby([116.4, 39.9])
    state().setNearbyRadius(2000)

    state().openNearby([116.3, 39.98])

    expect(state().nearby).toEqual({ center: [116.3, 39.98], radius: 2000 })
  })

  it('closes the context menu when a nearby search starts from it', () => {
    state().openContextMenu([116.5, 39.95])

    state().openNearby([116.5, 39.95])

    expect(state().contextMenu).toBeNull()
  })

  it('keeps an open nearby search where it is when locating again', () => {
    state().openNearby([116.4, 39.9])

    state().setMyLocation({ lng: 116.31, lat: 39.99, accuracy: 30 })

    expect(state().nearby).toEqual({ center: [116.4, 39.9], radius: DEFAULT_NEARBY_RADIUS })
  })

  it('keeps nearby results open when the click lands outside every district', () => {
    state().openNearby([116.4, 39.9])

    state().selectDistrict(null)

    expect(state().nearby).toEqual({ center: [116.4, 39.9], radius: DEFAULT_NEARBY_RADIUS })
  })

  it('keeps the chosen radius when a pending nearby search opens after locating', () => {
    state().openNearby([116.4, 39.9])
    state().setNearbyRadius(2000)
    state().requestNearbyAfterLocate()

    state().setMyLocation({ lng: 116.31, lat: 39.99, accuracy: null })

    expect(state().nearby).toEqual({ center: [116.31, 39.99], radius: 2000 })
  })
})
