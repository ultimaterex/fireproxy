import type { Switch } from '@/lib/types'

export type PortLoc = {
  switchMac: string
  switchName: string
  portId: string
  uplink: boolean
  clients: string[]
  switchClients: string[]
  label: string
}

export function portLabel(swName: string, portId: string, uplink?: boolean): string {
  return uplink ? `${swName} · Uplink` : `${swName} · ${portId}`
}

type Candidate = PortLoc & { depth: number; sourceRank: number }

function parentDepth(byMac: Map<string, Switch>, mac: string): number {
  let d = 0
  const seen = new Set<string>()
  let cur = mac.toUpperCase()
  while (cur && !seen.has(cur)) {
    seen.add(cur)
    const sw = byMac.get(cur)
    if (!sw?.parent_mac) break
    d++
    cur = sw.parent_mac.toUpperCase()
  }
  return d
}

function sourceRank(sw: Switch): number {
  if (sw.source === 'tplink') return 2
  if (sw.source === 'unifi') return 1
  return 0
}

function better(a: Candidate, b: Candidate): boolean {
  if (a.depth !== b.depth) return a.depth > b.depth
  if (a.sourceRank !== b.sourceRank) return a.sourceRank > b.sourceRank
  // Prefer a concrete jack over switch-only attachment at the same depth.
  if (!!a.portId !== !!b.portId) return !!a.portId
  if (a.uplink !== b.uplink) return !a.uplink && b.uplink
  return false
}

/** Closest switch/port that claims each device MAC (tree depth, then source). */
export function indexDevicePorts(switches: Switch[]): Map<string, PortLoc> {
  const byMac = new Map(switches.map((sw) => [sw.mac.toUpperCase(), sw] as const))
  const best = new Map<string, Candidate>()

  const consider = (mac: string, cand: Candidate) => {
    const key = mac.toUpperCase()
    if (!key) return
    const prev = best.get(key)
    if (!prev || better(cand, prev)) best.set(key, cand)
  }

  for (const sw of switches) {
    const switchName = sw.name?.trim() || sw.mac
    const switchClients = (sw.clients ?? []).map((m) => m.toUpperCase())
    const depth = parentDepth(byMac, sw.mac)
    const rank = sourceRank(sw)

    for (const p of sw.ports ?? []) {
      const uplink = !!(p.uplink || (sw.uplink_port != null && p.id === sw.uplink_port))
      const clients = (p.clients ?? []).map((m) => m.toUpperCase())
      const loc: Candidate = {
        switchMac: sw.mac,
        switchName,
        portId: p.id,
        uplink,
        clients,
        switchClients,
        label: portLabel(switchName, p.id, uplink),
        depth,
        sourceRank: rank,
      }
      for (const mac of clients) consider(mac, loc)
    }

    // Node-level clients (e.g. TP-Link via UniFi parent FDB): attach to switch, no jack.
    if (switchClients.length > 0) {
      const loc: Candidate = {
        switchMac: sw.mac,
        switchName,
        portId: '',
        uplink: false,
        clients: switchClients,
        switchClients,
        label: switchName,
        depth,
        sourceRank: rank,
      }
      for (const mac of switchClients) consider(mac, loc)
    }
  }

  const out = new Map<string, PortLoc>()
  for (const [mac, c] of best) {
    const { depth: _d, sourceRank: _r, ...loc } = c
    out.set(mac, loc)
  }
  return out
}

export function sameMacSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const have = new Set(a.map((m) => m.toUpperCase()))
  if (have.size !== a.length) return false
  return b.every((m) => have.has(m.toUpperCase()))
}

export function describeMacFilter(macs: string[], switches: Switch[]): string {
  if (macs.length === 0) return ''
  for (const sw of switches) {
    const name = sw.name?.trim() || sw.mac
    if (sameMacSet(macs, sw.clients ?? [])) return name
    for (const p of sw.ports ?? []) {
      const uplink = !!(p.uplink || (sw.uplink_port != null && p.id === sw.uplink_port))
      if (sameMacSet(macs, p.clients ?? [])) return portLabel(name, p.id, uplink)
    }
  }
  return `${macs.length} on switch`
}

export function isWholeSwitchFilter(macs: string[], switches: Switch[]): boolean {
  return switches.some((sw) => sameMacSet(macs, sw.clients ?? []))
}
