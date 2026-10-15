// 与接口一致：去掉首尾空格后最多 50 个字符。接口按 Unicode 码点计数，这里也按码点截断。
export const MAX_QUERY_LENGTH = 50

export function toSearchTerm(text: string): string {
  return Array.from(text.trim()).slice(0, MAX_QUERY_LENGTH).join('')
}
