import { describe, expect, it } from 'vitest'
import { routeClick, type ClickContext } from './clickRoute'

const normal: ClickContext = { mode: 'normal', nearbyOpen: false }
const withNearby: ClickContext = { mode: 'normal', nearbyOpen: true }

describe('routeClick', () => {
  it('uses every click to set the location in pick mode', () => {
    expect(routeClick({ mode: 'pickLocation', nearbyOpen: true }, { kind: 'point', id: 7 }, '110108')).toEqual({
      type: 'pickLocation',
    })
  })

  it('expands a cluster before anything underneath it', () => {
    expect(routeClick(normal, { kind: 'cluster', clusterId: 3, center: [116.3, 39.9] }, '110108')).toEqual({
      type: 'expandCluster',
      clusterId: 3,
      center: [116.3, 39.9],
    })
  })

  it('opens a POI instead of selecting the district under it', () => {
    expect(routeClick(normal, { kind: 'point', id: 7 }, '110108')).toEqual({ type: 'openPoi', id: 7 })
  })

  it('selects the district when no POI was hit', () => {
    expect(routeClick(normal, undefined, '110108')).toEqual({ type: 'selectDistrict', adcode: '110108' })
  })

  it('clears the selection outside every district', () => {
    expect(routeClick(normal, undefined, undefined)).toEqual({ type: 'selectDistrict', adcode: null })
  })

  it('keeps nearby results open: a click beside the POIs only closes the popup', () => {
    expect(routeClick(withNearby, undefined, '110108')).toEqual({ type: 'closePopup' })
  })

  it('still opens POIs while nearby results are open', () => {
    expect(routeClick(withNearby, { kind: 'point', id: 7 }, '110108')).toEqual({ type: 'openPoi', id: 7 })
  })
})
