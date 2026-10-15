// 页面级错误的说明文字。MapLibre 创建不了 WebGL 上下文时抛出的错误信息里带有「WebGL」。
export function describeFatalError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error)
  if (/webgl/i.test(message)) {
    return '浏览器无法创建 WebGL 上下文，地图显示不了。请确认浏览器开启了硬件加速，或者换用最新版的 Chrome、Edge、Safari、Firefox。'
  }
  return '页面出错了，请刷新后重试。'
}
