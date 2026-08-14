import { useEffect, useMemo, useRef, useState } from 'react'

import { switchMaker } from '@/lib/maker'
import { goldNic, goldPort } from '@/lib/ports'
import type { BoxInfo, Device, IfaceStats, NetIface, Switch } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ApTile, AP_DISC, AP_GAP } from '@/vrack/ApTile'
import { findProduct, rackFaceWidth, resolveFaceplate, resolveGenericShelf } from '@/vrack/engine'
import { Faceplate } from '@/vrack/Faceplate'
import type { ResolvedFaceplate } from '@/vrack/types'

const RACK_PAD = 8
const UNIT_GAP = 4
const SHELF_GAP = 16
const WIRELESS_GAP = 20
const WIRELESS_LABEL = 12
const FAN_STEP = 18
const FAN_BASE = 14
const FAN_MAX_RIGHT = 22

type PlacedUnit = {
  sw: Switch
  layout: ResolvedFaceplate
  x: number
  y: number
}

type PlacedAp = {
  sw: Switch
  x: number
  y: number
}

type Link = {
  key: string
  parentMac: string
  parentPort: string
  childMac: string
  /** Faceplate jack id, or null when the child is an AP (top-of-disc anchor). */
  childPort: string | null
  x1: number
  y1: number
  x2: number
  y2: number
  fan: number
  fans: number
}

type Hover =
  | { kind: 'link'; key: string }
  | { kind: 'port'; mac: string; port: string }
  | { kind: 'ap'; mac: string }

export function VRack({
  box,
  switches,
  devices,
  ifaces,
  linkGold,
  wanByPort,
  connections,
  onSelect,
  onSelectBox,
  onSelectGold,
}: {
  box?: BoxInfo | null
  switches: Switch[]
  devices: Device[]
  ifaces?: Record<string, IfaceStats>
  linkGold: Set<number>
  wanByPort: Map<number, NetIface>
  connections: boolean
  onSelect: (sw: Switch, port: string | null) => void
  onSelectBox: () => void
  onSelectGold: (n: number) => void
}) {
  const nativeW = rackFaceWidth()

  const { units, aps, goldMac, nativeH, wirelessY } = useMemo(() => {
    const rackLayouts: { sw: Switch; layout: ResolvedFaceplate }[] = []
    const shelfLayouts: { sw: Switch; layout: ResolvedFaceplate }[] = []
    const apList: Switch[] = []

    const gold = goldUnit(box, { ifaces, linkGold, wanByPort })
    if (gold) {
      const product = findProduct({
        manufacturer: 'firewalla',
        type: 'box',
        model: box?.license || box?.model,
        name: box?.name,
      })
      if (product && product.form !== 'shelf') {
        rackLayouts.push({ sw: gold, layout: resolveFaceplate(product) })
      }
    }

    for (const sw of switches) {
      if (sw.kind === 'ap') {
        apList.push(sw)
        continue
      }
      const maker = switchMaker(sw, vendorOf(sw.mac, devices))
      const product = findProduct({
        manufacturer: maker,
        type: 'switch',
        model: sw.model,
        name: sw.name,
      })
      if (product?.form === 'shelf') {
        shelfLayouts.push({ sw, layout: resolveFaceplate(product) })
      } else if (product) {
        rackLayouts.push({ sw, layout: resolveFaceplate(product) })
      } else {
        shelfLayouts.push({ sw, layout: resolveGenericShelf(sw, maker) })
      }
    }

    const placed: PlacedUnit[] = []
    let y = 0
    for (const u of rackLayouts) {
      placed.push({ ...u, x: 0, y })
      y += u.layout.height + UNIT_GAP
    }
    if (rackLayouts.length && shelfLayouts.length) y += SHELF_GAP - UNIT_GAP
    let sx = 0
    let shelfY = y
    let shelfRowH = 0
    for (const u of shelfLayouts) {
      if (sx > 0 && sx + u.layout.width > nativeW) {
        sx = 0
        shelfY += shelfRowH + UNIT_GAP
        shelfRowH = 0
      }
      placed.push({ ...u, x: sx, y: shelfY })
      sx += u.layout.width + UNIT_GAP
      shelfRowH = Math.max(shelfRowH, u.layout.height)
    }
    if (shelfLayouts.length) {
      y = shelfY + shelfRowH
    } else if (rackLayouts.length) {
      y = Math.max(...placed.map((u) => u.y + u.layout.height), 0)
    }

    let wirelessY = y
    const apPlaced: PlacedAp[] = []
    if (apList.length) {
      if (placed.length) y += WIRELESS_GAP
      wirelessY = y
      y += WIRELESS_LABEL + 6
      let ax = 0
      let rowY = y
      for (const sw of apList) {
        if (ax > 0 && ax + AP_DISC > nativeW) {
          ax = 0
          rowY += AP_DISC + AP_GAP
        }
        apPlaced.push({ sw, x: ax, y: rowY })
        ax += AP_DISC + AP_GAP
      }
      y = rowY + AP_DISC
    }

    const bottoms = [
      ...placed.map((u) => u.y + u.layout.height),
      ...apPlaced.map((a) => a.y + AP_DISC),
    ]
    return {
      units: placed,
      aps: apPlaced,
      goldMac: gold?.mac,
      nativeH: bottoms.length ? Math.max(...bottoms) : 0,
      wirelessY,
    }
  }, [box, switches, devices, ifaces, linkGold, wanByPort, nativeW])

  const links = useMemo(
    () => (connections ? buildLinks(units, aps, goldMac) : []),
    [connections, units, aps, goldMac],
  )

  const [hover, setHover] = useState<Hover | null>(null)
  useEffect(() => {
    if (!connections) setHover(null)
  }, [connections])

  const lit = useMemo(() => {
    const ports = new Map<string, Set<string>>()
    const apMacs = new Set<string>()
    const keys = new Set<string>()
    const addPort = (mac: string, port: string) => {
      const m = mac.toUpperCase()
      const set = ports.get(m) ?? new Set()
      set.add(port)
      ports.set(m, set)
    }
    const match = (l: Link) => {
      if (!hover) return false
      if (hover.kind === 'link') return l.key === hover.key
      if (hover.kind === 'ap') return l.childMac.toUpperCase() === hover.mac.toUpperCase() && l.childPort == null
      const mac = hover.mac.toUpperCase()
      return (
        (l.parentMac.toUpperCase() === mac && l.parentPort === hover.port) ||
        (l.childMac.toUpperCase() === mac && l.childPort === hover.port)
      )
    }
    if (hover) {
      for (const l of links) {
        if (!match(l)) continue
        keys.add(l.key)
        addPort(l.parentMac, l.parentPort)
        if (l.childPort) addPort(l.childMac, l.childPort)
        else apMacs.add(l.childMac.toUpperCase())
      }
      if (hover.kind === 'port') addPort(hover.mac, hover.port)
      if (hover.kind === 'ap') apMacs.add(hover.mac.toUpperCase())
    }
    return { ports, apMacs, keys, active: keys.size > 0 || hover != null }
  }, [hover, links])

  const boxEl = useRef<HTMLDivElement>(null)
  const [vw, setVw] = useState(0)
  useEffect(() => {
    const el = boxEl.current
    if (!el) return
    const ro = new ResizeObserver(() => setVw(el.clientWidth))
    ro.observe(el)
    setVw(el.clientWidth)
    return () => ro.disconnect()
  }, [])

  const inner = Math.max(0, vw - RACK_PAD * 2)
  const scale = inner > 0 && nativeW > 0 ? inner / nativeW : 1
  const empty = units.length === 0 && aps.length === 0

  return (
    <div ref={boxEl} className="w-full min-w-0 space-y-3">
      <div
        className="rounded-lg border border-border/80 bg-background/40"
        style={{
          padding: RACK_PAD,
          width: vw > 0 ? inner + RACK_PAD * 2 : undefined,
          height: vw > 0 && nativeH > 0 ? nativeH * scale + RACK_PAD * 2 : undefined,
        }}
      >
        {empty ? (
          <p className="px-2 py-4 text-sm text-muted-foreground">No rack definitions</p>
        ) : vw === 0 ? null : (
          <div
            className="relative"
            style={{
              width: nativeW,
              height: nativeH,
              transform: `scale(${scale})`,
              transformOrigin: 'top left',
            }}
          >
            {units.map(({ sw, layout, x, y }) => (
              <div key={sw.mac} className="absolute" style={{ left: x, top: y }}>
                <Faceplate
                  layout={layout}
                  sw={sw}
                  litPorts={lit.ports.get(sw.mac.toUpperCase())}
                  onPortHover={
                    connections
                      ? (port, on) =>
                          setHover(on && port ? { kind: 'port', mac: sw.mac, port } : null)
                      : undefined
                  }
                  onSelect={(unit, port) => {
                    if (unit.kind === 'box') {
                      if (port == null) {
                        onSelectBox()
                        return
                      }
                      const n = Number(port)
                      if (n >= 1 && n <= 4) onSelectGold(n)
                      return
                    }
                    onSelect(unit, port)
                  }}
                />
              </div>
            ))}
            {aps.length > 0 ? (
              <>
                <div
                  className="absolute left-0 right-0 border-t border-dashed border-sky-500/40"
                  style={{ top: wirelessY }}
                />
                <div
                  className="absolute left-0 text-[9px] tracking-wide text-muted-foreground uppercase"
                  style={{ top: wirelessY + 2 }}
                >
                  Wireless
                </div>
                {aps.map(({ sw, x, y }) => (
                  <div key={sw.mac} className="absolute" style={{ left: x, top: y }}>
                    <ApTile
                      sw={sw}
                      lit={lit.apMacs.has(sw.mac.toUpperCase())}
                      onHover={
                        connections
                          ? (on) => setHover(on ? { kind: 'ap', mac: sw.mac } : null)
                          : undefined
                      }
                      onSelect={() => onSelect(sw, null)}
                    />
                  </div>
                ))}
              </>
            ) : null}
            {connections && links.length > 0 ? (
              <svg
                className="pointer-events-none absolute inset-0 z-[2]"
                width={nativeW}
                height={nativeH}
                aria-hidden
              >
                {links.map((l) => {
                  const on = lit.keys.has(l.key)
                  const dim = lit.active && !on
                  const d = bendPath(l, nativeW)
                  return (
                    <g key={l.key}>
                      <path
                        d={d}
                        fill="none"
                        stroke="transparent"
                        strokeWidth={12}
                        className="pointer-events-auto cursor-pointer"
                        style={{ pointerEvents: 'stroke' }}
                        onMouseEnter={() => setHover({ kind: 'link', key: l.key })}
                        onMouseLeave={() => setHover(null)}
                      />
                      <path
                        d={d}
                        fill="none"
                        stroke="currentColor"
                        strokeWidth={on ? 2.5 : 1.5}
                        strokeDasharray="5 4"
                        className={cn(
                          'pointer-events-none transition-[stroke-width,color]',
                          on ? 'text-sky-400' : dim ? 'text-sky-500/20' : 'text-sky-500/80',
                        )}
                      />
                    </g>
                  )
                })}
              </svg>
            ) : null}
          </div>
        )}
      </div>
    </div>
  )
}

function buildLinks(units: PlacedUnit[], aps: PlacedAp[], goldMac?: string): Link[] {
  const byMac = new Map(units.map((u) => [u.sw.mac.toUpperCase(), u]))
  const raw: Omit<Link, 'fan' | 'fans'>[] = []

  for (const child of units) {
    if (child.sw.kind === 'box') continue
    const childPorts = uplinkPorts(child.sw)
    if (!childPorts.length) continue

    const parentMac = child.sw.parent_mac?.toUpperCase()
    if (parentMac && byMac.has(parentMac)) {
      const parent = byMac.get(parentMac)!
      const parentPorts = parentPortsFor(parent.sw, child.sw.parent_port, childPorts.length)
      pushPaired(raw, parent, parentPorts, child, childPorts)
      continue
    }

    const goldN = goldPort(child.sw.parent_nic ?? '').n
    if (goldN > 0 && goldMac) {
      const parent = byMac.get(goldMac.toUpperCase())
      if (!parent) continue
      pushPaired(raw, parent, Array(childPorts.length).fill(String(goldN)), child, childPorts)
    }
  }

  for (const ap of aps) {
    const top = { x: ap.x + AP_DISC / 2, y: ap.y }
    const parentMac = ap.sw.parent_mac?.toUpperCase()
    if (parentMac && byMac.has(parentMac)) {
      const parent = byMac.get(parentMac)!
      const parentPorts = ap.sw.parent_port ? expandLag(parent.sw, ap.sw.parent_port) : []
      pushApLinks(raw, parent, parentPorts, ap.sw.mac, top)
      continue
    }

    const goldN = goldPort(ap.sw.parent_nic ?? '').n
    if (goldN > 0 && goldMac) {
      const parent = byMac.get(goldMac.toUpperCase())
      if (!parent) continue
      pushApLinks(raw, parent, [String(goldN)], ap.sw.mac, top)
    }
  }

  return fanLinks(raw)
}

function pushApLinks(
  out: Omit<Link, 'fan' | 'fans'>[],
  parent: PlacedUnit,
  parentPorts: string[],
  apMac: string,
  top: { x: number; y: number },
) {
  for (let i = 0; i < parentPorts.length; i++) {
    const pp = parentPorts[i]
    const parentPt = jackPoint(parent, pp)
    if (!parentPt) continue
    out.push({
      key: `${parent.sw.mac}:${pp}->${apMac}:top:${i}`,
      parentMac: parent.sw.mac,
      parentPort: pp,
      childMac: apMac,
      childPort: null,
      x1: parentPt.x,
      y1: parentPt.y,
      x2: top.x,
      y2: top.y,
    })
  }
}

function pushPaired(
  out: Omit<Link, 'fan' | 'fans'>[],
  parent: PlacedUnit,
  parentPorts: string[],
  child: PlacedUnit,
  childPorts: string[],
) {
  const n = Math.max(parentPorts.length, childPorts.length)
  for (let i = 0; i < n; i++) {
    const pp = parentPorts[Math.min(i, parentPorts.length - 1)]
    const cp = childPorts[Math.min(i, childPorts.length - 1)]
    const parentPt = jackPoint(parent, pp)
    const childPt = jackPoint(child, cp)
    if (!parentPt || !childPt) continue
    out.push({
      key: `${parent.sw.mac}:${pp}->${child.sw.mac}:${cp}:${i}`,
      parentMac: parent.sw.mac,
      parentPort: pp,
      childMac: child.sw.mac,
      childPort: cp,
      x1: parentPt.x,
      y1: parentPt.y,
      x2: childPt.x,
      y2: childPt.y,
    })
  }
}

function fanLinks(raw: Omit<Link, 'fan' | 'fans'>[]): Link[] {
  const groups = new Map<string, typeof raw>()
  for (const l of raw) {
    const m = l.key.match(/^([^:]+):[^->]+->([^:]+):/)
    const key = m ? `${m[1]}->${m[2]}` : l.key
    const g = groups.get(key) ?? []
    g.push(l)
    groups.set(key, g)
  }

  const out: Link[] = []
  for (const g of groups.values()) {
    g.sort((a, b) => a.x1 - b.x1 || a.x2 - b.x2 || a.key.localeCompare(b.key))
    const fans = g.length
    g.forEach((l, fan) => out.push({ ...l, fan, fans }))
  }
  return out
}

function parentPortsFor(parent: Switch, parentPort: string | undefined, need: number): string[] {
  if (!parentPort) return []
  const lag = expandLag(parent, parentPort)
  if (lag.length >= need) return lag.slice(0, need)
  while (lag.length < need) lag.push(lag[lag.length - 1] ?? parentPort)
  return lag
}

function uplinkPorts(sw: Switch): string[] {
  const ports = sw.ports ?? []
  const marked = ports.filter((p) => p.uplink || (sw.uplink_port != null && p.id === sw.uplink_port)).map((p) => p.id)
  if (marked.length) {
    const expanded = new Set<string>()
    for (const id of marked) for (const p of expandLag(sw, id)) expanded.add(p)
    return [...expanded].sort((a, b) => Number(a) - Number(b) || a.localeCompare(b))
  }
  if (sw.uplink_port) return expandLag(sw, sw.uplink_port)
  return []
}

function expandLag(sw: Switch, port: string): string[] {
  for (const g of sw.lags ?? []) {
    if (g.includes(port)) return [...g].sort((a, b) => Number(a) - Number(b) || a.localeCompare(b))
  }
  return [port]
}

function jackPoint(unit: PlacedUnit, portId?: string | null): { x: number; y: number } | null {
  if (!portId) return null
  const jack = unit.layout.jacks.find((j) => j.id === portId)
  if (!jack) return null
  return {
    x: unit.x + jack.x + jack.w / 2,
    y: unit.y + jack.y + jack.h / 2,
  }
}

function bendPath(l: Link, rackW: number): string {
  const { x1, y1, x2, y2, fan, fans } = l
  const want = fans <= 1 ? FAN_BASE * 0.6 : FAN_BASE + fan * FAN_STEP
  const lo = Math.min(x1, x2)
  const hi = Math.max(x1, x2)
  const roomRight = Math.max(0, rackW - hi - FAN_MAX_RIGHT)
  const roomLeft = Math.max(0, lo - FAN_MAX_RIGHT)
  const apexX =
    roomRight >= roomLeft
      ? hi + Math.min(want, Math.max(roomRight, 0))
      : lo - Math.min(want, Math.max(roomLeft, 0))

  if (Math.abs(y1 - y2) < 12) {
    const under = Math.max(y1, y2) + 18 + fan * 12
    return `M ${x1} ${y1} C ${x1} ${under}, ${apexX} ${under}, ${x2} ${y2}`
  }

  return `M ${x1} ${y1} C ${apexX} ${y1}, ${apexX} ${y2}, ${x2} ${y2}`
}

function goldUnit(
  box: BoxInfo | null | undefined,
  opts: {
    ifaces?: Record<string, IfaceStats>
    linkGold: Set<number>
    wanByPort: Map<number, NetIface>
  },
): Switch | undefined {
  if (!box) return undefined
  return {
    mac: boxMac(box),
    name: box.name,
    model: box.license || box.model,
    kind: 'box',
    healthy: true,
    ports: [1, 2, 3, 4].map((n) => ({
      id: String(n),
      up: opts.linkGold.has(n) || opts.wanByPort.has(n) || !!opts.ifaces?.[goldNic(n)]?.carrier,
    })),
  }
}

function boxMac(box: BoxInfo): string {
  const first = box.macs?.[0]
  if (!first) return '__box__'
  return typeof first === 'string' ? first : first.mac || '__box__'
}

function vendorOf(mac: string, devices: Device[]): string | undefined {
  return devices.find((d) => d.mac.toUpperCase() === mac.toUpperCase())?.vendor
}
