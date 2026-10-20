import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { MetricsTab } from './MetricsTab'
import type { Dashboard, LatestView } from '@/lib/types'

const base: Dashboard = {
  ts: 1_700_000_000, devices: 1, rules: 2, alarm_count: 0,
  transfer_24h: { upload: 0, download: 0 }, blocked: { blocked: 0, allowed: 0 },
}
const render = (dashboard: Dashboard, latest: LatestView | null = null) => renderToStaticMarkup(
  <MetricsTab dashboard={dashboard} latest={latest} persist={{}} agentOnline deviceCount={1} ruleCount={2}
    onOpenDevices={() => {}} onOpenRules={() => {}} />,
)

describe('Metrics missing data and time scope', () => {
  it('identifies the reporting source without inventing a collection failure', () => {
    const agent = render({ ...base, source: 'agent' })
    expect(agent).toContain('No monthly WAN usage reported by the agent.')
    expect(agent).toContain('No speedtest results reported by the agent.')
    expect(render({ ...base, source: 'fw-app-init' })).toContain('No monthly WAN usage reported by the Firewalla control.')
    expect(render(base)).toContain('No monthly WAN usage reported by the current source.')
  })

  it('describes missing stale data as last received rather than waiting for a catalog', () => {
    expect(render({ ...base, source: 'agent', stale: true })).toContain('The last received data from the agent contains no monthly WAN usage.')
  })

  it('keeps reported zero monthly usage distinct from absent usage', () => {
    const markup = render({ ...base, monthly_wans: [{ uuid: 'test', name: 'Example ISP', upload: 0, download: 0 }] })
    expect(markup).toContain('Example ISP')
    expect(markup).not.toContain('No monthly WAN usage')
  })

  it('renders default WAN name in monthly usage', () => {
    const markup = render({ ...base, monthly_wans: [{ uuid: 'test', name: 'WAN', upload: 100, download: 200 }] })
    expect(markup).toContain('WAN')
    expect(markup).not.toContain('No monthly WAN usage')
  })

  it('labels lifetime cache hit rate separately from the 24-hour query totals', () => {
    const latest: LatestView = {
      snapshot: { ts: base.ts!, host: 'test', load: { m1: 0, m5: 0, m15: 0 }, dns_restarts: 0, ifaces: {}, wan: {} },
      rates: {}, have_prev: false, unbound_hit: { pct: 90, life: true },
    }
    const markup = render(base, latest)
    expect(markup).toContain('Lifetime cache hit rate')
    expect(markup).toContain('Queries over the last 24 hours')
    expect(markup).toContain('Lookups · last 24 hours')
    expect(markup).not.toContain('Hit · life')
  })
})
