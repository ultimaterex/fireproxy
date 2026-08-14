import type { NetIface } from '@/lib/types'

export type GoldPort = { n: number; tagged: boolean; ok: boolean }

export type PortChip = { n: number; tagged: boolean; unused: boolean; vid?: number }

/** Gold silk-screen: Port 1=eth3 … Port 4=eth0 (matches pkg/inventory/ports.go). */
export function goldNic(n: number): string {
  switch (n) {
    case 1:
      return 'eth3'
    case 2:
      return 'eth2'
    case 3:
      return 'eth1'
    case 4:
      return 'eth0'
    default:
      return ''
  }
}

export function goldPort(ifname: string): GoldPort {
  let base = ifname
  let tagged = false
  const dot = ifname.indexOf('.')
  if (dot >= 0) {
    tagged = true
    base = ifname.slice(0, dot)
  }
  switch (base) {
    case 'eth3':
      return { n: 1, tagged, ok: true }
    case 'eth2':
      return { n: 2, tagged, ok: true }
    case 'eth1':
      return { n: 3, tagged, ok: true }
    case 'eth0':
      return { n: 4, tagged, ok: true }
    default:
      return { n: 0, tagged: false, ok: false }
  }
}

export function portChips(intfs?: string[], vid?: number): PortChip[] {
  const list =
    intfs && intfs.length > 0
      ? intfs
      : vid
        ? [`eth0.${vid}`, `eth3.${vid}`]
        : []
  const byPort = new Map<number, { tagged: boolean; vid?: number }>()
  for (const ifname of list) {
    const gp = goldPort(ifname)
    if (!gp.ok) continue
    if (gp.tagged) {
      const dot = ifname.indexOf('.')
      const vid = dot >= 0 ? parseInt(ifname.slice(dot + 1), 10) : undefined
      byPort.set(gp.n, { tagged: true, vid: Number.isFinite(vid) ? vid : undefined })
    } else if (!byPort.has(gp.n)) {
      byPort.set(gp.n, { tagged: false })
    }
  }
  const chips: PortChip[] = []
  for (let n = 1; n <= 4; n++) {
    const p = byPort.get(n)
    if (p) chips.push({ n, tagged: p.tagged, unused: false, vid: p.vid })
    else chips.push({ n, tagged: false, unused: true })
  }
  return chips
}

export function managerKind(n: NetIface): 'hide' | 'wan' | 'lan' {
  const name = n.name
  if (name.startsWith('wg_ap') || name.startsWith('tun_fwvpn')) {
    return 'hide'
  }
  if (n.type === 'wan') return 'wan'
  // Stale Redis NIC/VLAN leftovers (e.g. eth0.250 "IOT") are not FireRouter LANs.
  if (/^(eth|wlan)\d/.test(name)) return 'hide'
  return 'lan'
}

export function speedLabel(mbps?: number | null): string {
  if (mbps == null || mbps <= 0) return ''
  if (mbps >= 10000) return '10G'
  if (mbps >= 2500) return '2.5G'
  if (mbps >= 1000) return '1G'
  return `${mbps}M`
}

export function wanTypeLabel(t?: string): string {
  switch (t) {
    case 'failover':
    case 'primary_standby':
      return 'Failover'
    case 'load_balance':
      return 'Load balance'
    case 'single':
      return 'Single'
    default:
      return ''
  }
}
