import type { BoxInfo, Switch, SwitchPort, TopoNode, TopoView } from '@/lib/types'
import { loadVRackSims } from '@/lib/vrack-sim'

const DEMO_KEY = 'fireproxy.debug.topoDemo'
const DEMO_ON_KEY = 'fireproxy.debug.topoDemoOn'

export function isTopoDemoOn(): boolean {
  try {
    return localStorage.getItem(DEMO_ON_KEY) === '1'
  } catch {
    return false
  }
}

export function setTopoDemoOn(on: boolean) {
  localStorage.setItem(DEMO_ON_KEY, on ? '1' : '0')
}

export function loadTopoDemo(): TopoView | null {
  try {
    const raw = localStorage.getItem(DEMO_KEY)
    if (!raw) return null
    return JSON.parse(raw) as TopoView
  } catch {
    return null
  }
}

export function saveTopoDemo(view: TopoView | null) {
  if (!view) localStorage.removeItem(DEMO_KEY)
  else localStorage.setItem(DEMO_KEY, JSON.stringify(view))
}

/** Full catalog-backed demo rack + wireless for Topology / VRACK. */
export function buildTopoDemo(): TopoView {
  const goldMac = 'AA:BB:CC:DD:EE:00'
  const hd = mk(
    'AA:BB:CC:DD:EE:01',
    'Core HD',
    'USW-Pro-HD-24',
    [...rj(1, 24), ...sfp(25, 28)],
    { parent_nic: 'eth3', uplink: '28' },
  )
  const agg = mk(
    'AA:BB:CC:DD:EE:02',
    'Aggregation',
    'USW-Aggregation',
    sfp(1, 8),
    { parent_mac: hd.mac, parent_port: '27', uplink: '1' },
  )
  // LAG-ish: also mark parent lags on HD
  hd.lags = [['27', '28']]
  agg.lags = [['1', '2']]
  agg.parent_port = '27'

  const max16 = mk(
    'AA:BB:CC:DD:EE:03',
    'Pro Max 16',
    'USW-Pro-Max-16',
    [...rj(1, 16), ...sfp(17, 18)],
    { parent_mac: hd.mac, parent_port: '1', uplink: '17' },
  )
  const swx = mk(
    'AA:BB:CC:DD:EE:04',
    'Switch X',
    'fwsw-A',
    [...rj(1, 8), ...sfp(9, 12)],
    { parent_nic: 'eth2', uplink: '9' },
  )
  const flex8 = mk(
    'AA:BB:CC:DD:EE:05',
    'Flex 8',
    'USW Flex 2.5G 8',
    [...rj(1, 9), ...sfp(10, 10)],
    { parent_mac: max16.mac, parent_port: '8', uplink: '9' },
  )
  const flex5 = mk(
    'AA:BB:CC:DD:EE:06',
    'Flex 5',
    'USW Flex 2.5G 5',
    rj(1, 5),
    { parent_mac: max16.mac, parent_port: '7', uplink: '5' },
  )
  const ap1 = mkAp('AA:BB:CC:DD:EE:11', 'Office AP', 'U7 Pro', {
    parent_mac: max16.mac,
    parent_port: '2',
  })
  const ap2 = mkAp('AA:BB:CC:DD:EE:12', 'Hall AP', 'U6 Lite', {
    parent_mac: flex8.mac,
    parent_port: '3',
  })
  // Generic unmatched shelf (exercises resolveGenericShelf)
  const mystery = mk(
    'AA:BB:CC:DD:EE:20',
    'Mystery 48',
    'Some-Unknown-48',
    rj(1, 48),
    { parent_mac: hd.mac, parent_port: '3', uplink: '1' },
  )

  const switches = [hd, agg, max16, swx, flex8, flex5, ap1, ap2, mystery]
  const box: BoxInfo = {
    name: 'Demo Gold',
    model: 'gold',
    license: 'Gold SE',
    public_ip: '203.0.113.10',
    macs: [{ name: 'eth0', mac: goldMac }],
  }

  const tree: TopoNode[] = [
    {
      mac: goldMac,
      name: box.name,
      type: 'box',
      children: [
        {
          mac: hd.mac,
          name: hd.name,
          type: 'switch',
          parent_port: 'eth3',
          child_port: '28',
          children: [
            {
              mac: agg.mac,
              name: agg.name,
              type: 'switch',
              parent_port: '27',
              child_port: '1',
            },
            {
              mac: max16.mac,
              name: max16.name,
              type: 'switch',
              parent_port: '1',
              child_port: '17',
              children: [
                {
                  mac: flex8.mac,
                  name: flex8.name,
                  type: 'switch',
                  parent_port: '8',
                  child_port: '9',
                  children: [
                    {
                      mac: ap2.mac,
                      name: ap2.name,
                      type: 'ap',
                      parent_port: '3',
                      child_port: '1',
                    },
                  ],
                },
                {
                  mac: flex5.mac,
                  name: flex5.name,
                  type: 'switch',
                  parent_port: '7',
                  child_port: '5',
                },
                {
                  mac: ap1.mac,
                  name: ap1.name,
                  type: 'ap',
                  parent_port: '2',
                  child_port: '1',
                },
              ],
            },
            {
              mac: mystery.mac,
              name: mystery.name,
              type: 'switch',
              parent_port: '3',
              child_port: '1',
            },
          ],
        },
        {
          mac: swx.mac,
          name: swx.name,
          type: 'switch',
          parent_port: 'eth2',
          child_port: '9',
        },
      ],
    },
  ]

  return {
    host: 'demo',
    box,
    switches,
    tree,
    wan_type: 'single',
  }
}

export function enableTopoDemo(): TopoView {
  const view = buildTopoDemo()
  saveTopoDemo(view)
  setTopoDemoOn(true)
  return view
}

export function disableTopoDemo() {
  setTopoDemoOn(false)
}

/** Merge extra generic sims into a topo view (demo or live). */
export function mergeExtraSims(view: TopoView): TopoView {
  const sims = loadVRackSims().map((s) => s.sw)
  if (!sims.length) return view
  const seen = new Set(view.switches?.map((s) => s.mac.toUpperCase()) ?? [])
  const extra = sims.filter((s) => !seen.has(s.mac.toUpperCase()))
  if (!extra.length) return view
  return { ...view, switches: [...(view.switches ?? []), ...extra] }
}

function mk(
  mac: string,
  name: string,
  model: string,
  ports: SwitchPort[],
  link: { parent_mac?: string; parent_port?: string; parent_nic?: string; uplink: string },
): Switch {
  const ports2 = ports.map((p) =>
    p.id === link.uplink ? { ...p, uplink: true, up: true } : p,
  )
  return {
    mac,
    name,
    model,
    kind: 'switch',
    source: model.startsWith('fwsw') ? undefined : 'unifi',
    healthy: true,
    uplink_port: link.uplink,
    parent_mac: link.parent_mac,
    parent_port: link.parent_port,
    parent_nic: link.parent_nic,
    ports: ports2,
  }
}

function mkAp(
  mac: string,
  name: string,
  model: string,
  link: { parent_mac: string; parent_port: string },
): Switch {
  return {
    mac,
    name,
    model,
    kind: 'ap',
    source: 'unifi',
    healthy: true,
    uplink_port: '1',
    parent_mac: link.parent_mac,
    parent_port: link.parent_port,
    ports: [{ id: '1', up: true, uplink: true }],
    radios: [
      { band: '2.4', channel: 6, width_mhz: 20 },
      { band: '5', channel: 36, width_mhz: 80 },
    ],
    clients: ['a', 'b', 'c'],
  }
}

function rj(from: number, to: number): SwitchPort[] {
  const out: SwitchPort[] = []
  for (let i = from; i <= to; i++) out.push({ id: String(i), up: i % 3 !== 0 })
  return out
}

function sfp(from: number, to: number): SwitchPort[] {
  const out: SwitchPort[] = []
  for (let i = from; i <= to; i++) out.push({ id: String(i), up: i % 2 === 0, sfp: true })
  return out
}
