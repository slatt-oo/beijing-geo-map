import type { Map as MaplibreMap } from 'maplibre-gl'
import { describe, expect, it } from 'vitest'
import { addDataLayers, BASEMAP_SOURCE, baseStyle, LAYER, SOURCE } from './layers'

type Call = { kind: 'source'; id: string } | { kind: 'layer'; id: string; source: string }

// 只记录调用顺序的假地图：addDataLayers 只用到 addSource 和 addLayer，不需要 WebGL。
function recordingMap(calls: Call[]): MaplibreMap {
  return {
    addSource: (id: string) => calls.push({ kind: 'source', id }),
    addLayer: (layer: { id: string; source: string }) => calls.push({ kind: 'layer', id: layer.id, source: layer.source }),
  } as unknown as MaplibreMap
}

describe('addDataLayers', () => {
  it('adds the layers bottom to top in the documented order', () => {
    const calls: Call[] = []

    addDataLayers(recordingMap(calls))

    expect(calls.filter((c) => c.kind === 'layer').map((c) => c.id)).toEqual([
      LAYER.districtFill,
      LAYER.districtLine,
      LAYER.districtLabel,
      LAYER.nearbyFill,
      LAYER.nearbyLine,
      LAYER.poiCluster,
      LAYER.poiClusterCount,
      LAYER.poiPoint,
      LAYER.myLocationAccuracy,
      LAYER.myLocationDot,
    ])
  })

  it('adds every source before the first layer that uses it', () => {
    const calls: Call[] = []

    addDataLayers(recordingMap(calls))

    const added = new Set<string>()
    for (const call of calls) {
      if (call.kind === 'source') added.add(call.id)
      else expect(added.has(call.source), `${call.id} uses ${call.source}`).toBe(true)
    }
    expect([...added].sort()).toEqual(Object.values(SOURCE).sort())
  })
})

describe('baseStyle', () => {
  it('puts the tiles and the configured attribution on the basemap source', () => {
    const style = baseStyle(['https://tiles.example.com/{z}/{x}/{y}.png'], '底图 © 示例')

    expect(style.sources[BASEMAP_SOURCE]).toMatchObject({
      type: 'raster',
      tiles: ['https://tiles.example.com/{z}/{x}/{y}.png'],
      attribution: '底图 © 示例',
    })
    expect(style.layers).toEqual([{ id: 'basemap', type: 'raster', source: BASEMAP_SOURCE }])
  })
})
