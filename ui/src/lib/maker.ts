export type Maker = 'firewalla' | 'unifi' | 'tplink'

const LOGOS: Record<Maker, string> = {
  firewalla: '/logos/firewalla.png',
  unifi: '/logos/unifi.png',
  tplink: '/logos/tplink.png',
}

const LABELS: Record<Maker, string> = {
  firewalla: 'Firewalla',
  unifi: 'UniFi',
  tplink: 'TP-Link',
}

export function makerLogo(id: Maker): string {
  return LOGOS[id]
}

export function makerLabel(id: Maker): string {
  return LABELS[id]
}

export function switchMaker(
  sw: { source?: string; model?: string; name?: string },
  vendor?: string,
): Maker | undefined {
  if (sw.source === 'unifi') return 'unifi'
  if (sw.source === 'tplink') return 'tplink'
  if (/^fwsw/i.test(sw.model ?? '') || /firewalla/i.test(sw.model ?? '') || /firewalla/i.test(sw.name ?? '')) {
    return 'firewalla'
  }
  const v = (vendor ?? '').toLowerCase()
  if (/ubiquiti|unifi|\bui\b/.test(v)) return 'unifi'
  if (/firewalla/.test(v)) return 'firewalla'
  if (/tp-?link/.test(v)) return 'tplink'
  return undefined
}
