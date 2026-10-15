export function formatArea(km2: number): string {
  return `${km2.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} 平方千米`
}

export function formatDistance(meters: number): string {
  if (Math.round(meters) < 1000) return `${Math.round(meters)} m`
  return `${(meters / 1000).toLocaleString('zh-CN', { maximumFractionDigits: 1 })} km`
}
