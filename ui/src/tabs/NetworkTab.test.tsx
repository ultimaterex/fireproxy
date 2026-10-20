import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { NetworkTab } from './NetworkTab'
import type { WanTest } from '@/lib/types'

describe('WAN last-test age', () => {
  const start = 1_700_000_000_000
  const render = (nowMs: number, wanTest: WanTest) => renderToStaticMarkup(
    <NetworkTab network={[]} nowMs={nowMs} wanTest={wanTest} onSelectLan={() => {}} />,
  )

  it('advances existing test ages with the bootstrap clock for seconds and milliseconds', () => {
    const wanTest = { connected: true, wans: {
      eth1: { ts: start / 1000 }, eth2: { ts: start },
    } }
    expect(render(start + 60_000, wanTest)).toContain('eth1 · 1m')
    const later = render(start + 3_600_000, wanTest)
    expect(later).toContain('eth1 · 1h')
    expect(later).toContain('eth2 · 1h')
  })

  it('ages a new test received after the original mount time', () => {
    const wanTest = { connected: true, wans: { eth1: { ts: (start + 120_000) / 1000 } } }
    expect(render(start + 125_000, wanTest)).toContain('eth1 · 5s')
    expect(render(start + 240_000, wanTest)).toContain('eth1 · 2m')
  })

  it('marks stale init extras separately from the WAN interface data', () => {
    const markup = renderToStaticMarkup(
      <NetworkTab network={[]} nowMs={start} onSelectLan={() => {}}
        extrasProvenance={{ source: 'fw-app-init', stale: true, reason: 'fallback' }} />,
    )
    expect(markup).toContain('Stale extras')
    expect(markup).toContain('Fallback · control')
  })
})
