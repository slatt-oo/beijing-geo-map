import { describe, expect, it } from 'vitest'
import { describeFatalError } from './fatalError'

describe('describeFatalError', () => {
  it('explains a missing WebGL context', () => {
    expect(describeFatalError(
        new Error(
          'WebGL2 is required to display this map. We are sorry, but it seems that your browser does not support WebGL2, a technology for rendering 3D graphics on the web.',
        ),
      )).toBe(
      '浏览器无法创建 WebGL 上下文，地图显示不了。请确认浏览器开启了硬件加速，或者换用最新版的 Chrome、Edge、Safari、Firefox。',
    )
  })

  it('falls back to a generic message for other errors', () => {
    expect(describeFatalError(new TypeError('Cannot read properties of undefined'))).toBe('页面出错了，请刷新后重试。')
    expect(describeFatalError('boom')).toBe('页面出错了，请刷新后重试。')
  })
})
