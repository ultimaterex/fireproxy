import { describe, expect, it } from 'vitest'
import {
  anonymizeAudit,
  anonymizeBox,
  anonymizeControlEvents,
  anonymizeDevices,
  anonymizeLogLines,
  anonymizeNameSync,
  anonymizeNetwork,
  anonymizeTags,
  anonymizeTopo,
  anonymizeWireless,
  switchNamePool,
  AP_NAMES,
  createAnon,
  DEST_POOL,
  DEVICE_NAMES,
  GEO_ROWS,
  GROUP_NAMES,
  ISP_POOL,
  LAN_NAMES,
  PHONE_NAMES,
  SWITCH_NAMES,
} from './anonymity'

const A = () => createAnon('test-salt-aaaaaaaa')
const B = () => createAnon('other-salt-bbbbbbbb')

describe('stable hash', () => {
  it('same salt+kind+original → same fake', () => {
    expect(A().fakeName('device', 'Kitchen TV')).toBe(A().fakeName('device', 'Kitchen TV'))
  })
  it('different salt → different pick (usually)', () => {
    expect(A().fakeName('device', 'Kitchen TV')).not.toBe(B().fakeName('device', 'Kitchen TV'))
  })
  it('dest/ISP from pools; geo row paired', () => {
    const d = A().fakeDest('secret.home.lan')
    expect(DEST_POOL.some((x) => x.name === d.name && x.domain === d.domain)).toBe(true)
    expect(ISP_POOL).toContain(A().fakeISP('Comcast'))
    const g = A().fakeGeo('SR')
    expect(GEO_ROWS).toContainEqual(g)
  })
  it('time offset preserves deltas and is days-scale', () => {
    const a = A()
    const d = a.shiftTS(1_700_000_100) - a.shiftTS(1_700_000_000)
    expect(d).toBe(100)
    expect(Math.abs(a.offsetSec)).toBeGreaterThanOrEqual(3 * 86400)
    expect(Math.abs(a.offsetSec)).toBeLessThanOrEqual(17 * 86400)
  })
})

describe('ip/cidr/mac', () => {
  it('private v4 stays private; public stays public-looking', () => {
    const a = A()
    const p = a.fakeIP('192.168.1.50')
    expect(p.startsWith('10.') || p.startsWith('192.168.') || p.startsWith('172.')).toBe(true)
    const pub = a.fakeIP('8.8.8.8')
    expect(pub.startsWith('10.') || pub.startsWith('192.168.') || /^172\.(1[6-9]|2\d|3[0-1])\./.test(pub)).toBe(false)
  })
  it('device IP stays inside faked LAN CIDR; host bits from full prefix', () => {
    const a = A()
    const cidr = '10.20.30.0/16'
    const fakeCidr = a.fakeCIDR(cidr)
    const fakeIp = a.fakeIP('10.20.30.77', [{ orig: cidr, fake: fakeCidr }])
    expect(a.ipInCIDR(fakeIp, fakeCidr)).toBe(true)
    expect(a.fakeIP('10.20.30.77', [{ orig: cidr, fake: fakeCidr }])).toBe(fakeIp)
  })
  it('same original IP → one fake (public_ip = WAN)', () => {
    expect(A().fakeIP('203.0.113.9')).toBe(A().fakeIP('203.0.113.9'))
  })
  it('MAC locally administered and stable', () => {
    const m = A().fakeMAC('AA:BB:CC:DD:EE:FF')
    expect(m).toMatch(/^([0-9A-F]{2}:){5}[0-9A-F]{2}$/)
    const second = parseInt(m.split(':')[0]!, 16)
    expect(second & 0x02).toBeTruthy()
    expect(A().fakeMAC('aa:bb:cc:dd:ee:ff')).toBe(m)
  })
})

describe('rewrite + walkers', () => {
  it('rewrites IP MAC and domain in log line; shifts ts', () => {
    const a = A()
    const line = a.rewriteText('query A kitchen.lan from 192.168.1.20 mac aa:bb:cc:dd:ee:ff to netflix.com')
    expect(line).not.toMatch(/192\.168\.1\.20/i)
    expect(line).not.toMatch(/aa:bb:cc:dd:ee:ff/i)
    const domains = DEST_POOL.map((x) => x.domain)
    expect(domains.some((d) => line.toLowerCase().includes(d))).toBe(true)
    expect(anonymizeLogLines(a, [{ ts: 1_700_000_000, source: 'unbound', line: 'hi 10.0.0.1' }])[0]!.ts).toBe(
      a.shiftTS(1_700_000_000),
    )
  })
  it('box geo+tz paired; public_ip stable with WAN ip', () => {
    const a = A()
    const box = anonymizeBox(a, {
      name: 'Gold',
      public_ip: '8.8.4.4',
      ddns: 'foo.firewalla.com',
      timezone: 'America/Paramaribo',
      country: 'SR',
      region: 'Paramaribo',
      eid: 'real-eid',
    })
    const g = a.fakeGeo('SR')
    expect(box.country).toBe(g.cc)
    expect(box.region).toBe(g.city)
    expect(box.timezone).toBe(g.tz)
    expect(box.public_ip).toBe(a.fakeIP('8.8.4.4'))
    expect(box.eid).toBe(a.fakeEID('real-eid'))
  })
})

describe('name pools + walkers', () => {
  it('LAN desc uses network-style names, not device names', () => {
    const a = createAnon('lan-salt-111111111111')
    const out = anonymizeNetwork(a, [
      { name: 'br0', desc: 'Home LAN', type: 'lan', ip: '192.168.1.1', subnet: '192.168.1.0/24' },
      { name: 'eth1', desc: 'Comcast', type: 'wan', wan_ready: true, ip: '8.8.8.8' },
    ])
    expect(LAN_NAMES).toContain(out[0]!.desc)
    expect(DEVICE_NAMES).not.toContain(out[0]!.desc)
    expect(ISP_POOL).toContain(out[1]!.desc)
  })

  it('box MAC labels stay real; MAC values are faked', () => {
    const a = createAnon('mac-salt-222222222222')
    const box = anonymizeBox(a, {
      name: 'Gold',
      macs: [
        { name: 'eth0', mac: 'AA:BB:CC:DD:EE:01' },
        { name: 'wlan0', mac: 'AA:BB:CC:DD:EE:02' },
        { name: 'bluetooth', mac: 'AA:BB:CC:DD:EE:03' },
      ],
    })
    expect(box.macs).toEqual([
      { name: 'eth0', mac: a.fakeMAC('AA:BB:CC:DD:EE:01') },
      { name: 'wlan0', mac: a.fakeMAC('AA:BB:CC:DD:EE:02') },
      { name: 'bluetooth', mac: a.fakeMAC('AA:BB:CC:DD:EE:03') },
    ])
  })

  it('offline switch/ap names match kind', () => {
    const a = createAnon('off-salt-333333333333')
    const audit = anonymizeAudit(a, {
      names: { count: 0, rows: [] },
      vlan: { count: 0, rows: [] },
      stp: { count: 0, rows: [] },
      unknown: { count: 0, rows: [] },
      offline: {
        count: 2,
        rows: [
          { mac: 'AA:BB:CC:00:00:01', name: 'USW-Pro-24', kind: 'switch' },
          { mac: 'AA:BB:CC:00:00:02', name: 'U6-LR', kind: 'ap' },
        ],
      },
      pending: { count: 0, rows: [] },
    })
    expect(SWITCH_NAMES).toContain(audit.offline.rows[0]!.name)
    expect(AP_NAMES).toContain(audit.offline.rows[1]!.name)
  })

  it('name-sync empty keeps UniFi blank; conflict uses distinct names', () => {
    const a = createAnon('ns-salt-444444444444')
    const rows = anonymizeNameSync(a, [
      { mac: 'AA:BB:CC:00:00:10', firewalla_name: 'Phone', unifi_name: 'Phone', status: 'empty' },
      { mac: 'AA:BB:CC:00:00:11', firewalla_name: 'Laptop', unifi_name: 'Laptop-Office', status: 'conflict' },
    ])
    expect(rows[0]!.unifi_name).toBeFalsy()
    expect(rows[0]!.firewalla_name).toBeTruthy()
    expect(rows[1]!.firewalla_name).not.toBe(rows[1]!.unifi_name)
    expect(rows[1]!.unifi_name).toBeTruthy()
  })

  it('phone type does not get a switch or console name', () => {
    const a = createAnon('dev-salt-555555555555')
    const [d] = anonymizeDevices(a, [{ mac: 'AA:BB:CC:00:00:20', name: 'Rex iPhone', type: 'phone' }])
    expect(PHONE_NAMES).toContain(d!.name)
    expect(SWITCH_NAMES).not.toContain(d!.name)
  })

  it('AP names are unique within a wireless list', () => {
    const a = createAnon('ap-salt-666666666666')
    const wifi = anonymizeWireless(a, {
      meta: { networks_supported: true },
      aps: Array.from({ length: 12 }, (_, i) => ({
        mac: `AA:00:00:00:00:${(i + 1).toString(16).padStart(2, '0')}`,
        name: `AP-${i}`,
        healthy: true,
      })),
      networks: [],
      clients: [],
    })
    const names = wifi.aps.map((ap) => ap.name)
    expect(new Set(names).size).toBe(names.length)
    expect(names.every((n) => n && AP_NAMES.includes(n as (typeof AP_NAMES)[number]))).toBe(true)
  })

  it('switch name pool omits PoE/port-count unless the hardware matches', () => {
    const small = switchNamePool({
      ports: Array.from({ length: 8 }, (_, i) => ({ id: String(i + 1), up: true })),
    })
    expect(small.some((n) => /poe|24-port|48-port|fiber|fanless/i.test(n))).toBe(false)
    expect(small).toContain('Edge Switch')

    const poe24 = switchNamePool({
      model: 'USW-24-PoE',
      poe: { draw_w: 40, budget_w: 250, active_ports: 8 },
      ports: Array.from({ length: 24 }, (_, i) => ({
        id: String(i + 1),
        up: true,
        poe_status: i < 8 ? 'on' : undefined,
      })),
    })
    expect(poe24).toContain('PoE Switch')
    expect(poe24).toContain('24-Port Switch')
  })

  it('topo fake name does not call a small non-PoE switch a PoE or 24-port', () => {
    const a = createAnon('sw-salt-888888888888')
    const topo = anonymizeTopo(a, {
      switches: Array.from({ length: 12 }, (_, i) => ({
        mac: `AA:BB:CC:00:01:${(i + 1).toString(16).padStart(2, '0')}`,
        name: `SW-${i}`,
        healthy: true,
        ports: Array.from({ length: 8 }, (_, j) => ({ id: String(j + 1), up: true })),
      })),
    })
    for (const s of topo.switches ?? []) {
      expect(s.name).not.toMatch(/poe|24-port|48-port|fiber/i)
    }
  })

  it('group names come from the group pool and stay unique', () => {
    const a = createAnon('grp-salt-777777777777')
    const tags = anonymizeTags(a, [
      { id: '1', name: 'alpha' },
      { id: '2', name: 'beta' },
      { id: '3', name: 'gamma' },
      { id: '4', name: 'delta' },
      { id: '5', name: 'epsilon' },
    ])
    const names = tags.map((t) => t.name)
    expect(names.every((n) => (GROUP_NAMES as readonly string[]).includes(n))).toBe(true)
    expect(new Set(names).size).toBe(names.length)
  })
})

describe('anonymizeControlEvents', () => {
  it('fakes MAC target, summary, before/after; shifts ms ts', () => {
    const a = A()
    const ts = 1_700_000_000_000
    const [out] = anonymizeControlEvents(a, [
      {
        id: 1,
        ts,
        scheme: 'firewalla',
        action: 'host.rename',
        actor_kind: 'user',
        actor: 'admin',
        target: 'aa:bb:cc:dd:ee:ff',
        summary: 'Kitchen TV',
        result: 'ok',
        before: { name: 'Kitchen TV' },
        after: { name: 'Living Room TV' },
      },
    ])
    expect(out!.ts).toBe(ts + a.offsetSec * 1000)
    expect(out!.target).toBe(a.fakeMAC('aa:bb:cc:dd:ee:ff'))
    expect(out!.target).not.toMatch(/aa:bb:cc:dd:ee:ff/i)
    expect(out!.summary).not.toBe('Kitchen TV')
    expect((out!.before as { name: string }).name).not.toBe('Kitchen TV')
    expect((out!.after as { name: string }).name).not.toBe('Living Room TV')
    expect(out!.actor).toBe('admin')
    expect(out!.scheme).toBe('firewalla')
  })
})


