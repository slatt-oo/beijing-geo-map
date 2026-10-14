import { createCoordTransform } from './lib/coord'
import { DEFAULT_TILE_URL, expandTileUrl, parseTileCrs } from './lib/tiles'

export const tileUrls = expandTileUrl(import.meta.env.VITE_TILE_URL || DEFAULT_TILE_URL)

export const { toDisplay, toData } = createCoordTransform(parseTileCrs(import.meta.env.VITE_TILE_CRS))
