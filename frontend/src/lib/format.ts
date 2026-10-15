export function formatArea(km2: number): string {
  return `${km2.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} 平方千米`
}
