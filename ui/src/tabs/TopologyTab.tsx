import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { ChevronRight, EthernetPort, RectangleEllipsis, Wifi } from 'lucide-react'

import { Breadcrumb } from '@/components/Breadcrumb'
import { Flag } from '@/components/Flag'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch as Toggle } from '@/components/ui/switch'
import { fmtBytes, fmtCount, netLabel, preferredName } from '@/lib/format'
import { makerLabel, makerLogo, switchMaker, type Maker } from '@/lib/maker'
import { goldNic, goldPort, managerKind, portChips, speedLabel, wanTypeLabel } from '@/lib/ports'
import type { BoxInfo, Device, IfaceStats, NetIface, Switch, SwitchPort, TopoNode, TopoView } from '@/lib/types'
import { cn } from '@/lib/utils'
import { GoldPortChips } from '@/tabs/NetworkTab'
import { VRack } from '@/vrack/VRack'

function DetailCrumb({
  root,
  leaf,
  onRoot,
  trailing,
}: {
  root: string
  leaf: string
  onRoot: () => void
  trailing?: ReactNode
}) {
  return (
    <Breadcrumb
      items={[{ label: root, onClick: onRoot }, { label: leaf }]}
      trailing={trailing}
    />
  )
}

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'

export function TopologyTab({
  view,
  network,
  ifaces,
  devices,
  focusMac,
  onOpenSwitchClients,
  onSelectLan,
  onSelectDevice,
}: {
  view: TopoView | null
  network: NetIface[]
  ifaces?: Record<string, IfaceStats>
  devices: Device[]
  focusMac?: string
  onOpenSwitchClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
  onSelectLan: (uuid: string) => void
  onSelectDevice?: (mac: string, label: string) => void
}) {
  const [selectedMac, setSelectedMac] = useState<string | null>(null)
  const [selectedPort, setSelectedPort] = useState<string | null>(null)
  const [portFromSwitch, setPortFromSwitch] = useState(false)
  const [selectedBox, setSelectedBox] = useState(false)
  const [selectedGoldPort, setSelectedGoldPort] = useState<number | null>(null)
  const [goldFromBox, setGoldFromBox] = useState(false)
  const [mode, setMode] = useState<'tree' | 'vrack'>('tree')
  const [connections, setConnections] = useState(false)

  useEffect(() => {
    if (!focusMac) return
    setSelectedMac(focusMac.toUpperCase())
    setSelectedPort(null)
    setSelectedBox(false)
    setSelectedGoldPort(null)
  }, [focusMac])

  const switchByMac = useMemo(() => {
    const m = new Map<string, Switch>()
    for (const s of view?.switches ?? []) m.set(s.mac.toUpperCase(), s)
    return m
  }, [view?.switches])

  if (!view) {
    return <p className="text-sm text-muted-foreground">Waiting on catalog</p>
  }

  const selected = selectedMac ? switchByMac.get(selectedMac.toUpperCase()) : undefined

  const namedSwitch = selected
    ? {
        ...selected,
        name: switchHostName(selected, devices) || pickSwitchName(selected.name) || selected.mac,
      }
    : undefined

  const boxName = view.box?.name || view.host || 'Box'
  const publicIp = view.box?.public_ip ?? '—'
  const switches = switchesInOrder(view, switchByMac, devices)
  const linkGold = new Set(
    switches.map((sw) => goldPort(sw.parent_nic ?? '').n).filter((n) => n > 0),
  )
  const wanByPort = wanGoldPorts(network)

  if (namedSwitch && selectedPort) {
    const port =
      namedSwitch.ports?.find((p) => p.id === selectedPort) ??
      ({ id: selectedPort, up: false } satisfies SwitchPort)
    return (
      <PortDetail
        sw={namedSwitch}
        port={port}
        maker={switchMaker(namedSwitch, vendorOf(namedSwitch.mac, devices))}
        uplinkTo={uplinkNextHop(namedSwitch, view.switches ?? [], devices, view.box?.name || view.host || 'Box')}
        boxMacs={boxMacSet(view.box)}
        onPickPort={(id) => setSelectedPort(id)}
        onBack={() => {
          setSelectedPort(null)
          if (!portFromSwitch) setSelectedMac(null)
        }}
        hideCrumb={!!focusMac}
        onOpenClients={onOpenSwitchClients}
        onSelectDevice={onSelectDevice}
        devices={devices}
      />
    )
  }

  if (namedSwitch?.kind === 'ap') {
    return (
      <APDetail
        sw={namedSwitch}
        onBack={() => setSelectedMac(null)}
        hideCrumb={!!focusMac}
        onOpenClients={onOpenSwitchClients}
      />
    )
  }

  if (namedSwitch) {
    return (
      <SwitchDetail
        sw={namedSwitch}
        maker={switchMaker(namedSwitch, vendorOf(namedSwitch.mac, devices))}
        onBack={() => setSelectedMac(null)}
        hideCrumb={!!focusMac}
        onOpenClients={onOpenSwitchClients}
        onSelectDevice={onSelectDevice}
        devices={devices}
        onOpenPort={(id) => {
          setPortFromSwitch(true)
          setSelectedPort(id)
        }}
      />
    )
  }

  if (selectedGoldPort != null) {
    return (
      <GoldPortDetail
        n={selectedGoldPort}
        boxName={boxName}
        publicIp={publicIp}
        model={boxModel(view.box)}
        network={network}
        ifaces={ifaces}
        switches={switches}
        linkGold={linkGold}
        wanByPort={wanByPort}
        onPickPort={(n) => setSelectedGoldPort(n)}
        onBack={() => {
          setSelectedGoldPort(null)
          if (!goldFromBox) setSelectedBox(false)
        }}
        onSelectLan={onSelectLan}
      />
    )
  }

  if (selectedBox) {
    return (
      <BoxDetail
        box={view.box}
        boxName={boxName}
        publicIp={publicIp}
        ifaces={ifaces}
        switches={switches}
        linkGold={linkGold}
        wanByPort={wanByPort}
        onBack={() => setSelectedBox(false)}
        onOpenPort={(n) => {
          setGoldFromBox(true)
          setSelectedGoldPort(n)
        }}
      />
    )
  }

  return (
    <div className={cn('space-y-4', mode === 'vrack' ? 'w-full min-w-0' : 'w-max min-w-full')}>
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-semibold tracking-tight">Topology</h1>
        <div className="inline-flex rounded-md border p-0.5">
          <button
            type="button"
            className={cn('rounded-sm px-2 py-1 text-xs', mode === 'tree' && 'bg-accent')}
            onClick={() => setMode('tree')}
          >
            Tree
          </button>
          <button
            type="button"
            className={cn('rounded-sm px-2 py-1 text-xs', mode === 'vrack' && 'bg-accent')}
            onClick={() => setMode('vrack')}
          >
            VRACK
          </button>
        </div>
        {mode === 'vrack' ? (
          <label className="ml-auto inline-flex cursor-pointer items-center gap-2 text-xs text-muted-foreground">
            Connections
            <Toggle checked={connections} onCheckedChange={setConnections} aria-label="Connections" />
          </label>
        ) : null}
      </div>

      <Card className="gap-0 py-0">
        <CardContent className="px-6 py-6">
          {mode === 'vrack' ? (
            <VRack
              box={view.box}
              switches={switches}
              devices={devices}
              ifaces={ifaces}
              linkGold={linkGold}
              wanByPort={wanByPort}
              connections={connections}
              onSelect={(sw, port) => {
                setPortFromSwitch(false)
                setSelectedMac(sw.mac)
                setSelectedPort(port)
              }}
              onSelectBox={() => setSelectedBox(true)}
              onSelectGold={(n) => {
                setGoldFromBox(false)
                setSelectedGoldPort(n)
              }}
            />
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-4">
                <GoldChassis
                  ifaces={ifaces}
                  switches={switches}
                  linkGold={linkGold}
                  wanByPort={wanByPort}
                  onOpenPort={(n) => {
                    setGoldFromBox(false)
                    setSelectedGoldPort(n)
                  }}
                />
                <DeviceHead
                  name={boxName}
                  ip={publicIp}
                  model={boxModel(view.box)}
                  onName={() => setSelectedBox(true)}
                />
              </div>

              {switches.length === 0 ? (
                <p className="mt-6 text-sm text-muted-foreground">No switches</p>
              ) : (
                <TopoTree
                  nodes={view.tree}
                  switches={switches}
                  byMac={switchByMac}
                  devices={devices}
                  onSelect={(sw, port) => {
                    setPortFromSwitch(false)
                    setSelectedMac(sw.mac)
                    setSelectedPort(port)
                  }}
                  onOpenClients={onOpenSwitchClients}
                />
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function switchesInOrder(view: TopoView, byMac: Map<string, Switch>, devices: Device[]): Switch[] {
  const hostName = new Map<string, string>()
  for (const d of devices) {
    const n = preferredName(d)
    if (n && n !== d.mac) hostName.set(d.mac.toUpperCase(), n)
  }
  const out: Switch[] = []
  const seen = new Set<string>()
  const take = (n: TopoNode) => {
    const sw = byMac.get(n.mac.toUpperCase())
    if (!sw || seen.has(sw.mac)) return
    seen.add(sw.mac)
    out.push({
      ...sw,
      name: hostName.get(sw.mac.toUpperCase()) || pickSwitchName(sw.name, n.name) || sw.mac,
      ip: sw.ip || n.ip,
      parent_nic: sw.parent_nic || n.parent_port,
      uplink_port: sw.uplink_port || n.child_port,
    })
  }
  const walk = (nodes?: TopoNode[]) => {
    for (const n of sortedTopo(nodes)) {
      if (n.type === 'switch' || n.type === 'ap' || byMac.has((n.mac ?? '').toUpperCase())) take(n)
      walk(n.children)
    }
  }
  walk(view.tree)
  for (const sw of view.switches ?? []) {
    if (seen.has(sw.mac)) continue
    out.push({
      ...sw,
      name: hostName.get(sw.mac.toUpperCase()) || pickSwitchName(sw.name) || sw.mac,
    })
  }
  return out
}

function vendorOf(mac: string, devices: Device[]): string | undefined {
  return devices.find((d) => d.mac.toUpperCase() === mac.toUpperCase())?.vendor
}

function uplinkNextHop(sw: Switch, switches: Switch[], devices: Device[], boxName: string): string {
  if (sw.parent_mac) {
    const p = switches.find((s) => s.mac.toUpperCase() === sw.parent_mac!.toUpperCase())
    if (p) {
      const name = switchHostName(p, devices) || pickSwitchName(p.name) || p.mac
      return sw.parent_port ? `${name} · ${sw.parent_port}` : name
    }
  }
  if (sw.parent_nic) {
    const g = goldPort(sw.parent_nic)
    return g.ok ? `${boxName} · ${g.n}` : boxName
  }
  return boxName
}

function switchHostName(sw: Switch, devices: Device[]): string {
  const host = devices.find((d) => d.mac.toUpperCase() === sw.mac.toUpperCase())
  const n = host ? preferredName(host) : ''
  return n && n !== host?.mac ? n : ''
}

function boxMacSet(box?: BoxInfo | null): Set<string> {
  const out = new Set<string>()
  for (const m of box?.macs ?? []) {
    out.add((typeof m === 'string' ? m : m.mac).toUpperCase())
  }
  return out
}

function pickSwitchName(...names: (string | undefined)[]): string {
  for (const n of names) {
    const s = n?.trim()
    if (s && !stockSwitchName(s)) return s
  }
  return ''
}

function stockSwitchName(name: string): boolean {
  return /^firewalla[- ]switch/i.test(name.trim())
}

function TopoTree({
  nodes,
  switches,
  byMac,
  devices,
  onSelect,
  onOpenClients,
}: {
  nodes?: TopoNode[]
  switches: Switch[]
  byMac: Map<string, Switch>
  devices: Device[]
  onSelect: (sw: Switch, port: string | null) => void
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
}) {
  const seen = new Set<string>()
  const rows = walkTopo(nodes, 0, seen, byMac)
  nestOrphans(rows, switches, seen)
  const kidsOf = childMap(rows.map((r) => r.sw))
  const inTree = new Set(rows.map((r) => r.sw.mac.toUpperCase()))
  const roots = rows
    .filter((r) => {
      const p = r.sw.parent_mac?.toUpperCase()
      return !p || !inTree.has(p)
    })
    .sort((a, b) => cmpPort(a.sw.parent_port || a.sw.parent_nic, b.sw.parent_port || b.sw.parent_nic))
  return (
    <>
      {roots.map(({ sw, gold }) => (
        <TopoBranch
          key={sw.mac}
          sw={sw}
          gold={gold}
          kidsOf={kidsOf}
          devices={devices}
          onSelect={onSelect}
          onOpenClients={onOpenClients}
        />
      ))}
    </>
  )
}

const RAIL_W = 12
const JACK_W = 36
const JACK_GAP = 4
const CHASSIS_PAD = 12
const CHASSIS_BORDER = 1
const SLOT_GAP = 32

function TopoBranch({
  sw,
  gold,
  kidsOf,
  devices,
  onSelect,
  onOpenClients,
}: {
  sw: Switch
  gold?: ReturnType<typeof goldPort>
  kidsOf: Map<string, Switch[]>
  devices: Device[]
  onSelect: (sw: Switch, port: string | null) => void
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
}) {
  const kids = kidsOf.get(sw.mac.toUpperCase()) ?? []
  return (
    <div>
      {gold?.ok ? <LinkStem goldPortN={gold.n} /> : gold ? <div className="h-8" /> : null}
      <TopoNodeRow
        sw={sw}
        downlinks={new Set(kids.flatMap((k) => inferRails(sw, k, kids)))}
        devices={devices}
        onSelect={onSelect}
        onOpenClients={onOpenClients}
      />
      <KidsBlock
        parent={sw}
        kids={kids}
        kidsOf={kidsOf}
        devices={devices}
        onSelect={onSelect}
        onOpenClients={onOpenClients}
      />
    </div>
  )
}

function KidsBlock({
  parent,
  kids,
  kidsOf,
  devices,
  onSelect,
  onOpenClients,
}: {
  parent: Switch
  kids: Switch[]
  kidsOf: Map<string, Switch[]>
  devices: Device[]
  onSelect: (sw: Switch, port: string | null) => void
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
}) {
  if (kids.length === 0) return null
  const maker = !!switchMaker(parent, vendorOf(parent.mac, devices))
  const ids = portIds(parent)
  const assignments = kids.map((k) => inferRails(parent, k, kids))
  const first = kids[0]
  return (
    <div>
      <PortSweep
        portIds={ids}
        maker={maker}
        assignments={assignments}
        direct={{
          maker: !!switchMaker(first, vendorOf(first.mac, devices)),
          portIds: devicePorts(first).map((p) => p.id),
          uplinkIds: uplinkJacks(first),
        }}
      />
      {kids.map((kid, i) => {
        const grand = kidsOf.get(kid.mac.toUpperCase()) ?? []
        return (
          <KidSlot
            key={kid.mac}
            index={i}
            count={kids.length}
            kid={kid}
            devices={devices}
            row={
              <TopoNodeRow
                sw={kid}
                downlinks={new Set(grand.flatMap((g) => inferRails(kid, g, grand)))}
                devices={devices}
                onSelect={onSelect}
                onOpenClients={onOpenClients}
              />
            }
            subtree={
              <KidsBlock
                parent={kid}
                kids={grand}
                kidsOf={kidsOf}
                devices={devices}
                onSelect={onSelect}
                onOpenClients={onOpenClients}
              />
            }
          />
        )
      })}
    </div>
  )
}

function KidSlot({
  index,
  count,
  kid,
  devices,
  row,
  subtree,
}: {
  index: number
  count: number
  kid: Switch
  devices: Device[]
  row: ReactNode
  subtree: ReactNode
}) {
  const next = index === 0
  const later = count - 1 - index
  const remaining = later + (next ? 0 : 1)
  const maker = !!switchMaker(kid, vendorOf(kid.mac, devices))
  const ids = devicePorts(kid).map((p) => p.id)
  return (
    <div style={{ paddingLeft: next ? 0 : (index - 1) * RAIL_W }}>
      {next ? null : (
        <ChildInlet remaining={remaining} maker={maker} portIds={ids} uplinkIds={uplinkJacks(kid)} />
      )}
      <div className="flex items-stretch">
        {remaining > 0 ? <LaterRails remaining={remaining} skipFirst={!next} /> : null}
        <div className="min-w-0">{row}</div>
      </div>
      {subtree ? (
        <div className="flex items-stretch">
          {remaining > 0 ? <LaterRails remaining={remaining} skipFirst={!next} /> : null}
          <div className="min-w-0">{subtree}</div>
        </div>
      ) : null}
    </div>
  )
}

function LaterRails({ remaining, skipFirst }: { remaining: number; skipFirst?: boolean }) {
  return (
    <div className="relative shrink-0 self-stretch" style={{ width: remaining * RAIL_W }} aria-hidden>
      {Array.from({ length: remaining }, (_, k) =>
        skipFirst && k === 0 ? null : (
          <div
            key={k}
            className="absolute inset-y-0 w-0.5 bg-sky-500"
            style={{ left: k * RAIL_W + RAIL_W / 2 - 1 }}
          />
        ),
      )}
    </div>
  )
}

function ChildInlet({
  remaining,
  maker,
  portIds,
  uplinkIds,
}: {
  remaining: number
  maker: boolean
  portIds: string[]
  uplinkIds: string[]
}) {
  const gutter = remaining * RAIL_W
  const w = gutter + chassisWidth(portIds.length, maker)
  const h = SLOT_GAP
  const railX = RAIL_W / 2
  const y = h - 6
  const dest = destJackXs(gutter, maker, portIds, uplinkIds)
  const xs = dest.length ? dest : [gutter + jackCenter(0, maker)]
  const paths = [
    ...Array.from({ length: remaining - 1 }, (_, k) => {
      const x = (k + 1) * RAIL_W + RAIL_W / 2
      return `M ${x} 0 V ${h}`
    }),
    ...xs.map((x, i) => {
      const yi = xs.length <= 1 ? y : sweepBendY(i, xs.length, h)
      return `M ${railX} 0 V ${yi} H ${x} V ${h}`
    }),
  ]
  return (
    <svg width={w} height={h} className="block text-sky-500" aria-hidden>
      {paths.map((d, i) => (
        <path key={i} d={d} fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="miter" strokeLinecap="square" />
      ))}
    </svg>
  )
}

function chassisWidth(portCount: number, maker: boolean): number {
  return (
    CHASSIS_BORDER * 2 +
    CHASSIS_PAD * 2 +
    (maker ? JACK_W + JACK_GAP : 0) +
    portCount * JACK_W +
    Math.max(0, portCount - 1) * JACK_GAP
  )
}

function jackCenter(index: number, maker: boolean): number {
  const left = CHASSIS_BORDER + CHASSIS_PAD + (maker ? JACK_W + JACK_GAP : 0)
  return left + index * (JACK_W + JACK_GAP) + JACK_W / 2
}

function PortSweep({
  portIds,
  maker,
  assignments,
  direct,
}: {
  portIds: string[]
  maker: boolean
  assignments: string[][]
  direct: { maker: boolean; portIds: string[]; uplinkIds: string[] }
}) {
  const n = assignments.length
  const gutter = Math.max(0, n - 1) * RAIL_W
  const w = Math.max(
    chassisWidth(portIds.length, maker),
    gutter + chassisWidth(direct.portIds.length, direct.maker),
    gutter + 8,
  )
  const h = SLOT_GAP
  const directXs = destJackXs(gutter, direct.maker, direct.portIds, direct.uplinkIds)
  const segs: { px: number; dx: number }[] = []
  assignments.forEach((ports, i) => {
    const src = ports
      .map((p) => portIds.indexOf(p))
      .filter((idx) => idx >= 0)
    const dest =
      i === 0
        ? directXs.length
          ? directXs
          : [gutter + jackCenter(0, direct.maker)]
        : [(i - 1) * RAIL_W + RAIL_W / 2]
    if (src.length === 0) {
      for (const dx of dest) segs.push({ px: dx, dx })
      return
    }
    segs.push(...pairSegs(src, dest, maker))
  })
  const paths = segs.map((s, i) => {
    const y = sweepBendY(i, segs.length, h)
    if (s.px === s.dx) return `M ${s.dx} 0 V ${h}`
    return `M ${s.px} 0 V ${y} H ${s.dx} V ${h}`
  })
  return (
    <svg width={w} height={h} className="block text-sky-500" aria-hidden>
      {paths.map((d, i) => (
        <path key={i} d={d} fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="miter" strokeLinecap="square" />
      ))}
    </svg>
  )
}

function pairSegs(srcIdx: number[], destXs: number[], maker: boolean): { px: number; dx: number }[] {
  const srcXs = srcIdx.map((idx) => jackCenter(idx, maker)).sort((a, b) => a - b)
  const dest = [...destXs].sort((a, b) => a - b)
  if (srcXs.length === dest.length) {
    return srcXs.map((px, i) => ({ px, dx: dest[i] }))
  }
  const out: { px: number; dx: number }[] = []
  for (const px of srcXs) {
    for (const dx of dest) out.push({ px, dx })
  }
  return out
}

function sweepBendY(i: number, n: number, h: number): number {
  const stub = 6
  if (n <= 1) return h - stub
  return h - stub - (i / (n - 1)) * (h - stub * 2)
}

function destJackXs(gutter: number, maker: boolean, portIds: string[], uplinkIds: string[]): number[] {
  return uplinkIds
    .map((id) => portIds.indexOf(id))
    .filter((idx) => idx >= 0)
    .map((idx) => gutter + jackCenter(idx, maker))
}

function childMap(list: Switch[]): Map<string, Switch[]> {
  const m = new Map<string, Switch[]>()
  for (const sw of list) {
    const p = sw.parent_mac?.toUpperCase()
    if (!p) continue
    const arr = m.get(p) ?? []
    arr.push(sw)
    m.set(p, arr)
  }
  for (const arr of m.values()) {
    arr.sort((a, b) => cmpPort(a.parent_port, b.parent_port) || (a.name ?? a.mac).localeCompare(b.name ?? b.mac))
  }
  return m
}

function portIds(sw: Switch): string[] {
  return [...(sw.ports ?? [])]
    .map((p) => p.id)
    .sort((a, b) => Number(a) - Number(b) || a.localeCompare(b))
}

function inferRails(parent: Switch, child: Switch, _siblings: Switch[]): string[] {
  if (child.parent_port) return expandLag(parent, child.parent_port)
  return []
}

function expandLag(sw: Switch, port: string): string[] {
  for (const g of sw.lags ?? []) {
    if (g.includes(port)) return [...g]
  }
  return [port]
}

function lagPeers(sw: Switch, id: string): string[] | undefined {
  const g = (sw.lags ?? []).find((group) => group.includes(id) && group.length > 1)
  return g ? [...g].sort((a, b) => Number(a) - Number(b) || a.localeCompare(b)) : undefined
}

function portRuns(ports: SwitchPort[], lags?: string[][]): SwitchPort[][] {
  const keyOf = new Map<string, string>()
  for (const g of lags ?? []) {
    if (g.length < 2) continue
    const key = [...g].sort().join(',')
    for (const id of g) keyOf.set(id, key)
  }
  const runs: SwitchPort[][] = []
  for (const p of ports) {
    const key = keyOf.get(p.id)
    const last = runs[runs.length - 1]
    if (key && last && keyOf.get(last[0].id) === key) last.push(p)
    else runs.push([p])
  }
  return runs
}

function uplinkJacks(sw: Switch): string[] {
  const ports = devicePorts(sw)
  const marked = ports.filter((p) => p.uplink || p.id === sw.uplink_port).map((p) => p.id)
  if (marked.length) return [...new Set(marked)]
  if (sw.uplink_port) return expandLag(sw, sw.uplink_port)
  return ports[0] ? [ports[0].id] : []
}

function walkTopo(
  nodes: TopoNode[] | undefined,
  depth: number,
  seen: Set<string>,
  byMac: Map<string, Switch>,
): { sw: Switch; depth: number; gold: ReturnType<typeof goldPort> }[] {
  const out: { sw: Switch; depth: number; gold: ReturnType<typeof goldPort> }[] = []
  for (const n of sortedTopo(nodes)) {
    if (n.type === 'box') {
      out.push(...walkTopo(n.children, 0, seen, byMac))
      continue
    }
    const sw = byMac.get((n.mac ?? '').toUpperCase())
    if (sw && !seen.has(sw.mac.toUpperCase())) {
      seen.add(sw.mac.toUpperCase())
      out.push({ sw, depth, gold: goldPort(sw.parent_nic ?? '') })
      out.push(...walkTopo(n.children, depth + 1, seen, byMac))
    } else {
      out.push(...walkTopo(n.children, depth, seen, byMac))
    }
  }
  return out
}

function sortedTopo(nodes?: TopoNode[]): TopoNode[] {
  return [...(nodes ?? [])].sort((a, b) => cmpPort(a.parent_port, b.parent_port) || (a.name || a.mac).localeCompare(b.name || b.mac))
}

function cmpPort(a?: string, b?: string): number {
  const ra = portRank(a)
  const rb = portRank(b)
  if (ra.rank !== rb.rank) return ra.rank - rb.rank
  if (ra.num !== rb.num) return ra.num - rb.num
  return ra.rest.localeCompare(rb.rest)
}

function portRank(s?: string): { rank: number; num: number; rest: string } {
  const v = String(s ?? '').trim()
  if (!v) return { rank: 2, num: 0, rest: '' }
  const n = Number(v)
  if (Number.isFinite(n)) return { rank: 0, num: n, rest: '' }
  return { rank: 1, num: 0, rest: v.toLowerCase() }
}

function nestOrphans(
  rows: { sw: Switch; depth: number; gold: ReturnType<typeof goldPort> }[],
  switches: Switch[],
  seen: Set<string>,
) {
  let grew = true
  while (grew) {
    grew = false
    for (const sw of switches) {
      const mac = sw.mac.toUpperCase()
      if (seen.has(mac) || !sw.parent_mac) continue
      const parent = sw.parent_mac.toUpperCase()
      const idx = rows.findIndex((r) => r.sw.mac.toUpperCase() === parent)
      if (idx < 0) continue
      const depth = rows[idx].depth + 1
      let at = idx + 1
      while (at < rows.length && rows[at].depth >= depth) at++
      rows.splice(at, 0, { sw, depth, gold: goldPort(sw.parent_nic ?? '') })
      seen.add(mac)
      grew = true
    }
  }
  for (const sw of switches) {
    if (seen.has(sw.mac.toUpperCase())) continue
    rows.push({ sw, depth: 0, gold: goldPort(sw.parent_nic ?? '') })
    seen.add(sw.mac.toUpperCase())
  }
}

function openSwitchClients(
  onOpen: (info: { macs: string[]; switchMac: string; switchName: string }) => void,
  sw: Switch,
  macs: string[],
) {
  if (!clientsSupported(sw)) return
  onOpen({
    macs,
    switchMac: sw.mac,
    switchName: sw.name?.trim() || sw.mac,
  })
}

function clientsSupported(sw: Switch): boolean {
  return sw.capabilities?.clients !== false
}

function portClientsSupported(sw: Switch): boolean {
  // Explicit false = unsupported. Omitted capabilities → legacy sources (UniFi/FW) support port clients.
  if (sw.capabilities == null) return true
  return sw.capabilities.port_clients !== false
}

function clientsLabel(macs: string[], devices: Device[]): string {
  if (macs.length === 0) return '0'
  if (macs.length === 1) {
    const mac = macs[0]
    const d = devices.find((x) => x.mac.toUpperCase() === mac.toUpperCase())
    return d ? preferredName(d) : mac
  }
  return String(macs.length)
}

function ClientsMetaRow({
  macs,
  devices,
  supported,
  sw,
  onOpenClients,
  onSelectDevice,
}: {
  macs: string[]
  devices: Device[]
  supported: boolean
  sw: Switch
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
  onSelectDevice?: (mac: string, label: string) => void
}) {
  if (!supported) return <MetaRow label="Clients" value="Unsupported" />
  if (macs.length === 1) {
    const mac = macs[0]
    const label = clientsLabel(macs, devices)
    return (
      <MetaRow
        label="Clients"
        value={label}
        onClick={onSelectDevice ? () => onSelectDevice(mac, label) : undefined}
      />
    )
  }
  if (macs.length > 1) {
    return (
      <MetaRow
        label="Clients"
        value={String(macs.length)}
        onClick={() => openSwitchClients(onOpenClients, sw, macs)}
      />
    )
  }
  return <MetaRow label="Clients" value="0" />
}

function TopoNodeRow({
  sw,
  downlinks,
  devices,
  onSelect,
  onOpenClients,
}: {
  sw: Switch
  downlinks?: Set<string>
  devices: Device[]
  onSelect: (sw: Switch, port: string | null) => void
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
}) {
  const clients = sw.clients ?? []
  const head = (
    <DeviceHead
      name={sw.name ?? sw.mac}
      ip={sw.ip ?? '—'}
      model={switchModelSKU(sw.model)}
      devices={clientsSupported(sw) ? clients.length : undefined}
      clientsUnsupported={!clientsSupported(sw)}
      onName={() => onSelect(sw, null)}
      onDevices={clientsSupported(sw) ? () => openSwitchClients(onOpenClients, sw, clients) : undefined}
    />
  )
  return (
    <div className="flex items-start gap-3">
      <Chassis maker={switchMaker(sw, vendorOf(sw.mac, devices))}>
        <ChassisPorts sw={sw} ports={devicePorts(sw)} downlinks={downlinks} onSelect={(id) => onSelect(sw, id)} />
        {sw.kind === 'ap' ? <RadioPips radios={sw.radios} /> : null}
        {sw.kind === 'ap' ? <WifiMark online={sw.healthy} /> : null}
      </Chassis>
      {head}
    </div>
  )
}

function devicePorts(sw: Switch): SwitchPort[] {
  const ports = [...(sw.ports ?? [])]
  if (ports.length === 0 && sw.kind === 'ap') {
    ports.push({ id: sw.uplink_port || '1', up: sw.healthy, uplink: true })
  }
  return ports.sort((a, b) => Number(a.id) - Number(b.id) || a.id.localeCompare(b.id))
}

function RadioPips({ radios }: { radios?: { band: string; channel?: number; width_mhz?: number }[] }) {
  const byBand = new Map((radios ?? []).map((r) => [r.band, r]))
  return (
    <span className="inline-flex flex-col items-center gap-0.5">
      <span className="text-[10px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
      <span className="inline-flex h-8 items-center gap-1">
        {(['2.4', '5', '6'] as const).map((band) => {
          const r = byBand.get(band)
          return (
            <span
              key={band}
              title={r ? `${band} · ch ${r.channel ?? '—'} · ${r.width_mhz ?? '—'} MHz` : `${band} off`}
              className={cn(
                'rounded px-1.5 py-0.5 text-[10px] tabular-nums',
                r ? 'bg-foreground/80 text-background' : 'border border-dashed border-muted-foreground/30 text-muted-foreground/40',
              )}
            >
              {band}
            </span>
          )
        })}
      </span>
      <span className="text-[9px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
    </span>
  )
}

function WifiMark({ online }: { online: boolean }) {
  return (
    <span title="Wi-Fi" className="inline-flex w-9 flex-col items-center gap-0.5">
      <span className="text-[10px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
      <Wifi
        className={cn('size-8', online ? 'text-sky-500' : 'text-muted-foreground/40')}
        aria-label="Wi-Fi"
      />
      <span className="text-[9px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
    </span>
  )
}

function APDetail({
  sw,
  onBack,
  hideCrumb,
  onOpenClients,
}: {
  sw: Switch
  onBack: () => void
  hideCrumb?: boolean
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
}) {
  const clients = sw.clients ?? []
  const meta: { label: string; value: string }[] = [
    { label: 'Name', value: sw.name ?? '—' },
    { label: 'IP', value: sw.ip ?? '—' },
    { label: 'MAC', value: sw.mac },
    { label: 'Model', value: sw.model?.trim() || '—' },
    { label: 'Firmware', value: sw.firmware || '—' },
    { label: 'Status', value: sw.healthy ? 'Online' : 'Offline' },
  ]
  return (
    <div className="space-y-4">
      {hideCrumb ? null : (
      <DetailCrumb
        root="Topology"
        leaf={sw.name ?? sw.mac}
        onRoot={onBack}
        trailing={<DeviceCount n={clients.length} onClick={() => openSwitchClients(onOpenClients, sw, clients)} />}
      />
      )}
      <Card className="gap-0 py-0">
        <CardContent className="px-6 py-6">
          <div className="flex flex-wrap items-center gap-4">
            <Chassis maker={switchMaker(sw)}>
              <ChassisPorts sw={sw} ports={devicePorts(sw)} />
              <RadioPips radios={sw.radios} />
              <WifiMark online={sw.healthy} />
            </Chassis>
            <DeviceHead
              name={sw.name ?? sw.mac}
              ip={sw.ip ?? '—'}
              model={sw.model}
              devices={clients.length}
              onDevices={() => openSwitchClients(onOpenClients, sw, clients)}
            />
          </div>
        </CardContent>
      </Card>
      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Radios</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {(sw.radios ?? []).length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No radios</p>
          ) : (
            (sw.radios ?? []).map((r) => (
              <div key={r.band} className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
                <div className="text-muted-foreground">{r.band} GHz</div>
                <div className="font-medium tabular-nums">
                  {r.channel != null ? `ch ${r.channel}` : '—'}
                  {r.width_mhz != null ? ` · ${r.width_mhz} MHz` : ''}
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Details</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {meta.map((r) => (
            <div key={r.label} className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">{r.label}</div>
              <div className="min-w-0 break-all font-medium">{r.value}</div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

function DeviceHead({
  name,
  ip,
  model,
  devices,
  clientsUnsupported,
  onName,
  onDevices,
}: {
  name: string
  ip: string
  model?: string
  devices?: number
  clientsUnsupported?: boolean
  onName?: () => void
  onDevices?: () => void
}) {
  return (
    <div className="flex flex-col gap-0.5 whitespace-nowrap text-sm">
      <div className="flex flex-wrap items-center gap-2">
        {onName ? (
          <button
            type="button"
            className="inline-flex items-center gap-0.5 font-medium hover:underline"
            onClick={onName}
          >
            {name}
            <ChevronRight className="size-4 text-muted-foreground" aria-hidden />
          </button>
        ) : (
          <span className="font-medium">{name}</span>
        )}
        {clientsUnsupported ? (
          <span className="text-xs text-muted-foreground/60">Clients unsupported</span>
        ) : devices != null && onDevices ? (
          <DeviceCount n={devices} onClick={onDevices} />
        ) : null}
      </div>
      <span className="font-mono text-muted-foreground">{ip}</span>
      {model ? <span className="text-xs text-muted-foreground/60">{model}</span> : null}
    </div>
  )
}

function boxModel(box?: BoxInfo | null): string {
  return box?.license?.trim() || box?.model?.trim() || ''
}

function switchModelSKU(model?: string): string {
  switch (model) {
    case 'fwsw-A':
      return 'FWSW12X'
    default:
      return model?.trim() || ''
  }
}

function DeviceCount({ n, onClick }: { n: number; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={n === 0}
      onClick={onClick}
      className="inline-flex items-center gap-0.5 text-muted-foreground tabular-nums hover:text-foreground disabled:opacity-40"
    >
      {n} devices
      <ChevronRight className="size-4" aria-hidden />
    </button>
  )
}

function ChassisPorts({
  sw,
  ports,
  downlinks,
  selected,
  onSelect,
}: {
  sw: Switch
  ports: SwitchPort[]
  downlinks?: Set<string>
  selected?: string
  onSelect?: (id: string) => void
}) {
  return (
    <>
      {portRuns(ports, sw.lags).map((run) => {
        const jacks = run.map((p) => {
          const uplink = p.uplink || (sw.uplink_port != null && p.id === sw.uplink_port)
          const down = downlinks?.has(p.id)
          return (
            <PortJack
              key={p.id}
              id={p.id}
              kind={uplink || down ? 'link' : p.up ? 'up' : 'down'}
              speed={p.up ? speedLabel(p.speed_mbps) : ''}
              sfp={p.sfp}
              title={portTitle(
                p,
                uplink ? 'uplink' : down ? 'downlink' : false,
                lagPeers(sw, p.id),
                uplink && !!sw.uplink_static,
              )}
              selected={p.id === selected}
              onClick={onSelect ? () => onSelect(p.id) : undefined}
            />
          )
        })
        if (run.length < 2) return jacks[0]
        return (
          <span key={run.map((p) => p.id).join('+')} className="relative inline-flex gap-1">
            {jacks}
            <span
              aria-hidden
              className="pointer-events-none absolute inset-x-0 top-3 h-8 rounded-sm border border-sky-300"
            />
          </span>
        )
      })}
    </>
  )
}

function Chassis({ maker, children }: { maker?: Maker; children: ReactNode }) {
  return (
    <div className="relative z-[1] inline-flex shrink-0 flex-nowrap gap-1 rounded-lg border border-border/80 bg-muted/25 p-3">
      {maker ? <MakerMark id={maker} /> : null}
      {children}
    </div>
  )
}

function MakerMark({ id }: { id: Maker }) {
  return (
    <span title={makerLabel(id)} className="inline-flex w-9 flex-col items-center gap-0.5">
      <span className="text-[10px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
      <img src={makerLogo(id)} alt="" className="size-8 rounded-sm object-contain" />
      <span className="text-[9px] leading-none opacity-0" aria-hidden>
        {'\u00a0'}
      </span>
    </span>
  )
}

function PortJack({
  id,
  kind,
  title,
  speed,
  sfp,
  selected,
  onClick,
}: {
  id: string
  kind: 'link' | 'wan' | 'up' | 'down' | 'idle'
  title: string
  speed?: string
  sfp?: boolean
  selected?: boolean
  onClick?: () => void
}) {
  const Icon = sfp ? RectangleEllipsis : EthernetPort
  const jack = (
    <>
      <span className="text-[10px] tabular-nums leading-none text-muted-foreground">{id}</span>
      <span
        className={cn(
          'inline-flex size-8 items-center justify-center rounded-sm border',
          kind === 'link' && 'border-transparent bg-sky-500 text-white',
          kind === 'wan' && 'border-transparent bg-yellow-400 text-yellow-950',
          kind === 'up' && 'border-transparent bg-foreground/80 text-background',
          (kind === 'down' || kind === 'idle') &&
            'border-dashed border-muted-foreground/30 text-muted-foreground/40',
          selected && 'outline outline-2 outline-offset-2 outline-foreground',
        )}
      >
        <Icon className="size-3.5" aria-hidden />
      </span>
      <span className="text-[9px] tabular-nums leading-none text-muted-foreground">
        {speed || '\u00a0'}
      </span>
    </>
  )
  if (!onClick) {
    return (
      <span title={title} className="inline-flex w-9 flex-col items-center gap-0.5">
        {jack}
      </span>
    )
  }
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className="inline-flex w-9 flex-col items-center gap-0.5 rounded-sm hover:opacity-90"
    >
      {jack}
    </button>
  )
}

function LinkStem({ goldPortN }: { goldPortN: number }) {
  return (
    <div className="inline-flex gap-1.5 border border-transparent px-3" aria-hidden>
      <div className="h-8 w-9" />
      {[1, 2, 3, 4].map((n) => (
        <div key={n} className="flex h-8 w-9 justify-center">
          {n === goldPortN ? <div className="w-0.5 bg-sky-500" /> : null}
        </div>
      ))}
    </div>
  )
}

function wanGoldPorts(network: NetIface[]): Map<number, NetIface> {
  const byPort = new Map<number, NetIface>()
  for (const n of network) {
    if (managerKind(n) !== 'wan') continue
    for (const c of portChips(n.intfs?.length ? n.intfs : [n.name])) {
      if (!c.unused) byPort.set(c.n, n)
    }
  }
  return byPort
}

function goldPortTitle(n: number, linked: boolean, wan?: NetIface, speed?: string): string {
  const bits = [`Port ${n}`]
  if (linked) bits.push('to switch')
  if (wan) bits.push(wan.desc || wan.name || 'WAN')
  if (speed) bits.push(speed)
  return bits.join(' · ')
}

function goldJackSpeed(
  goldN: number,
  opts: {
    linked: boolean
    wan?: NetIface
    switches: Switch[]
    ifaces?: Record<string, IfaceStats>
  },
): string {
  if (opts.linked) {
    const fromSw = goldLinkSpeed(goldN, opts.switches)
    if (fromSw) return fromSw
  }
  for (const name of [opts.wan?.name, goldNic(goldN)]) {
    if (!name) continue
    const label = speedLabel(opts.ifaces?.[name]?.speed_mbps ?? undefined)
    if (label) return label
  }
  return ''
}

function goldLinkSpeed(goldN: number, switches: Switch[]): string {
  for (const sw of switches) {
    if (goldPort(sw.parent_nic ?? '').n !== goldN) continue
    const up = (sw.ports ?? []).find((p) => p.uplink || p.id === sw.uplink_port)
    return speedLabel(up?.speed_mbps)
  }
  return ''
}

function portTitle(port: SwitchPort, link: boolean | 'uplink' | 'downlink', lag?: string[], staticBind?: boolean): string {
  const speed = port.up ? speedLabel(port.speed_mbps) : ''
  const role =
    link === 'downlink'
      ? ' · downlink'
      : link === 'uplink'
        ? staticBind
          ? ' · uplink · static bind'
          : ' · uplink'
        : ''
  const bond = lag && lag.length > 1 ? ` · LAG ${lag.join('+')}` : ''
  return `Port ${port.id}${port.sfp ? ' · SFP' : ''}${speed ? ` · ${speed}` : ''}${role}${bond}`
}

function GoldChassis({
  ifaces,
  switches,
  linkGold,
  wanByPort,
  selected,
  onOpenPort,
}: {
  ifaces?: Record<string, IfaceStats>
  switches: Switch[]
  linkGold: Set<number>
  wanByPort: Map<number, NetIface>
  selected?: number
  onOpenPort?: (n: number) => void
}) {
  return (
    <Chassis maker="firewalla">
      {[1, 2, 3, 4].map((n) => {
        const linked = linkGold.has(n)
        const wan = wanByPort.get(n)
        const speed = goldJackSpeed(n, { linked, wan, switches, ifaces })
        const kind = linked ? 'link' : wan ? 'wan' : 'idle'
        return (
          <PortJack
            key={n}
            id={String(n)}
            kind={kind}
            speed={speed}
            title={goldPortTitle(n, linked, wan, speed)}
            selected={selected === n}
            onClick={onOpenPort ? () => onOpenPort(n) : undefined}
          />
        )
      })}
    </Chassis>
  )
}

function BoxDetail({
  box,
  boxName,
  publicIp,
  ifaces,
  switches,
  linkGold,
  wanByPort,
  onBack,
  onOpenPort,
}: {
  box?: BoxInfo | null
  boxName: string
  publicIp: string
  ifaces?: Record<string, IfaceStats>
  switches: Switch[]
  linkGold: Set<number>
  wanByPort: Map<number, NetIface>
  onBack: () => void
  onOpenPort: (n: number) => void
}) {
  const macs = labeledMacs(box?.macs)
  const wan = wanTypeLabel(box?.wan_type)
  const about = [
    { label: 'Model', value: box?.license || box?.model || '—' },
    { label: 'Version', value: box?.version || '—' },
    { label: 'Mode', value: modeLabel(box?.mode) },
    { label: 'Timezone', value: box?.timezone || '' },
    { label: 'EID', value: box?.eid || '' },
  ].filter((r) => r.value)

  return (
    <div className="space-y-4">
      <DetailCrumb root="Topology" leaf={boxName} onRoot={onBack} />

      <Card className="gap-0 py-0">
        <CardContent className="px-6 py-6">
          <div className="flex flex-wrap items-center gap-4">
            <GoldChassis
              ifaces={ifaces}
              switches={switches}
              linkGold={linkGold}
              wanByPort={wanByPort}
              onOpenPort={onOpenPort}
            />
            <DeviceHead name={boxName} ip={publicIp} model={boxModel(box)} />
          </div>
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardContent className="divide-y px-0">
          <MetaRow label="Name" value={boxName} />
          {publicIp && publicIp !== '—' ? <MetaRow label="Public IP" value={publicIp} /> : null}
          {box?.ddns ? <MetaRow label="DDNS" value={box.ddns} /> : null}
          {wan ? <MetaRow label="WAN" value={wan} /> : null}
          {about.map((r) => (
            <MetaRow key={r.label} label={r.label} value={r.value} />
          ))}
          {box?.region ? (
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Location</div>
              <div className="flex min-w-0 items-center justify-end gap-2 break-all font-medium">
                <Flag cc={box.country} />
                {box.region}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {macs.length > 0 ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Addresses</CardTitle>
          </CardHeader>
          <CardContent className="divide-y px-0">
            {macs.map((a) => (
              <MetaRow key={a.mac} label={a.name} value={a.mac} />
            ))}
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

function GoldPortDetail({
  n,
  boxName,
  publicIp,
  model,
  network,
  ifaces,
  switches,
  linkGold,
  wanByPort,
  onPickPort,
  onBack,
  onSelectLan,
}: {
  n: number
  boxName: string
  publicIp: string
  model?: string
  network: NetIface[]
  ifaces?: Record<string, IfaceStats>
  switches: Switch[]
  linkGold: Set<number>
  wanByPort: Map<number, NetIface>
  onPickPort: (n: number) => void
  onBack: () => void
  onSelectLan: (uuid: string) => void
}) {
  const nic = goldNic(n)
  const iface = nic ? ifaces?.[nic] : undefined
  const wan = wanByPort.get(n)
  const linked = switches.find((sw) => goldPort(sw.parent_nic ?? '').n === n)
  const speed = goldJackSpeed(n, { linked: !!linked, wan, switches, ifaces })
  const nets = netsOnGoldPort(network, n)
  const role = linked ? linked.name || 'Switch' : wan ? netLabel(wan) : iface?.carrier ? 'Up' : 'Idle'

  return (
    <div className="space-y-4">
      <DetailCrumb
        root="Topology"
        leaf={`Port ${n}${linked ? ' · Uplink' : wan ? ' · WAN' : ''}`}
        onRoot={onBack}
      />

      <Card className="gap-0 py-0">
        <CardContent className="px-6 py-6">
          <div className="flex flex-wrap items-center gap-4">
            <GoldChassis
              ifaces={ifaces}
              switches={switches}
              linkGold={linkGold}
              wanByPort={wanByPort}
              selected={n}
              onOpenPort={onPickPort}
            />
            <DeviceHead name={boxName} ip={publicIp} model={model} />
          </div>
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardContent className="divide-y px-0">
          <MetaRow label="Role" value={role} />
          {nic ? <MetaRow label="Interface" value={nic} /> : null}
          {speed ? <MetaRow label="Port speed" value={gbpsLabel(iface?.speed_mbps ?? undefined) || speed} /> : null}
          {wan?.ip || wan?.subnet ? <MetaRow label="Address" value={wan.ip || wan.subnet || ''} /> : null}
          {iface ? (
            <>
              <MetaRow label="Received" value={fmtBytes(iface.rx_bytes)} />
              <MetaRow label="Sent" value={fmtBytes(iface.tx_bytes)} />
            </>
          ) : null}
        </CardContent>
      </Card>

      {nets.length > 0 ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Networks</CardTitle>
          </CardHeader>
          <CardContent className="divide-y px-0">
            {nets.map((net) => {
              const label = netLabel(net)
              const addr = net.ip || net.subnet || ''
              return net.uuid ? (
                <button
                  key={net.uuid}
                  type="button"
                  onClick={() => onSelectLan(net.uuid!)}
                  className="grid w-full grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-left text-sm hover:bg-accent/40"
                >
                  <div className="text-muted-foreground">{net.vid ? `VLAN ${net.vid}` : managerKind(net) === 'wan' ? 'WAN' : 'LAN'}</div>
                  <div className="min-w-0 text-right font-medium">
                    {label}
                    {addr ? <span className="ml-2 font-mono text-xs text-muted-foreground">{addr}</span> : null}
                  </div>
                </button>
              ) : (
                <MetaRow key={net.name} label={net.vid ? `VLAN ${net.vid}` : 'LAN'} value={label} />
              )
            })}
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

function netsOnGoldPort(network: NetIface[], n: number): NetIface[] {
  return network.filter((iface) => {
    if (managerKind(iface) === 'hide') return false
    return portChips(iface.intfs?.length ? iface.intfs : [iface.name]).some((c) => c.n === n && !c.unused)
  })
}

function labeledMacs(raw?: BoxInfo['macs']): { name: string; mac: string }[] {
  return (raw ?? []).map((m, i) =>
    typeof m === 'string' ? { name: `Address ${i + 1}`, mac: m } : { name: m.name, mac: m.mac },
  )
}

function modeLabel(mode?: string): string {
  switch (mode) {
    case 'router':
      return 'Router Mode'
    case 'dhcp':
      return 'DHCP Mode'
    case 'spoof':
      return 'Simple Mode'
    case 'dhcpSpoof':
      return 'DHCP Spoof'
    case 'manualSpoof':
      return 'Manual Spoof'
    case 'none':
      return 'Bridge Mode'
    default:
      return mode || '—'
  }
}

function SwitchDetail({
  sw,
  maker,
  onBack,
  hideCrumb,
  onOpenClients,
  onSelectDevice,
  onOpenPort,
  devices,
}: {
  sw: Switch
  maker?: Maker
  onBack: () => void
  hideCrumb?: boolean
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
  onSelectDevice?: (mac: string, label: string) => void
  onOpenPort: (id: string) => void
  devices: Device[]
}) {
  const clients = sw.clients ?? []
  const canClients = clientsSupported(sw)
  const poePct =
    sw.poe && sw.poe.budget_w > 0 ? Math.min(100, (sw.poe.draw_w / sw.poe.budget_w) * 100) : 0

  const meta: { label: string; value: string }[] = [
    { label: 'Name', value: sw.name ?? '—' },
    { label: 'IP', value: sw.ip ?? '—' },
    { label: 'MAC', value: sw.mac },
    { label: 'Model', value: switchModelSKU(sw.model) || '—' },
    {
      label: 'Firmware',
      value: [sw.firmware, sw.version].filter(Boolean).join(' · ') || '—',
    },
    { label: 'Status', value: sw.healthy ? 'Online' : 'Offline' },
  ]

  return (
    <div className="space-y-4">
      {hideCrumb ? null : (
      <DetailCrumb
        root="Topology"
        leaf={sw.name ?? sw.mac}
        onRoot={onBack}
        trailing={
          canClients ? (
            <DeviceCount n={clients.length} onClick={() => openSwitchClients(onOpenClients, sw, clients)} />
          ) : (
            <span className="text-xs text-muted-foreground/60">Clients unsupported</span>
          )
        }
      />
      )}

      <Card className="gap-0 py-0">
        <CardContent className="overflow-x-auto px-6 py-6">
          <div className="flex items-center gap-4">
            <Chassis maker={maker}>
              <ChassisPorts
                sw={sw}
                ports={[...(sw.ports ?? [])].sort((a, b) => Number(a.id) - Number(b.id) || a.id.localeCompare(b.id))}
                onSelect={onOpenPort}
              />
            </Chassis>
            <DeviceHead
              name={sw.name ?? sw.mac}
              ip={sw.ip ?? '—'}
              model={switchModelSKU(sw.model)}
              devices={canClients ? clients.length : undefined}
              clientsUnsupported={!canClients}
              onDevices={canClients ? () => openSwitchClients(onOpenClients, sw, clients) : undefined}
            />
          </div>
        </CardContent>
      </Card>

      {sw.poe ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>PoE</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 px-6 pb-6 text-sm">
            <div className="flex justify-between">
              <span>
                {sw.poe.draw_w.toFixed(1)} / {sw.poe.budget_w.toFixed(0)} W
              </span>
              <span className="text-muted-foreground">{sw.poe.active_ports} active</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
              <div className="h-full bg-[#027BFF]" style={{ width: `${poePct}%` }} />
            </div>
            {sw.poe.peak_port ? (
              <p className="text-muted-foreground">Peak port {sw.poe.peak_port}</p>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {sw.acl ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>ACL</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 px-6 pb-6 text-sm sm:grid-cols-4">
            <Kv label="Used" value={`${sw.acl.used} / ${sw.acl.max}`} />
            <Kv label="Control" value={String(sw.acl.control)} />
            <Kv label="Tracking" value={String(sw.acl.tracking)} />
          </CardContent>
        </Card>
      ) : null}

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Details</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {meta.map((r) => (
            <MetaRow key={r.label} label={r.label} value={r.value} />
          ))}
          <ClientsMetaRow
            macs={clients}
            devices={devices}
            supported={canClients}
            sw={sw}
            onOpenClients={onOpenClients}
            onSelectDevice={onSelectDevice}
          />
          {sw.parent_nic ? (
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Box port</div>
              <GoldPortChips chips={portChips([sw.parent_nic])} />
            </div>
          ) : null}
        </CardContent>
      </Card>

    </div>
  )
}

function PortDetail({
  sw,
  port,
  maker,
  uplinkTo,
  boxMacs,
  onPickPort,
  onBack,
  hideCrumb,
  onOpenClients,
  onSelectDevice,
  devices,
}: {
  sw: Switch
  port: SwitchPort
  maker?: Maker
  uplinkTo: string
  boxMacs: Set<string>
  onPickPort: (id: string | null) => void
  onBack: () => void
  hideCrumb?: boolean
  onOpenClients: (info: { macs: string[]; switchMac: string; switchName: string }) => void
  onSelectDevice?: (mac: string, label: string) => void
  devices: Device[]
}) {
  const [dir, setDir] = useState<'in' | 'out'>('in')
  const uplink = !!(port.uplink || (sw.uplink_port != null && port.id === sw.uplink_port))
  const staticBind = uplink && !!sw.uplink_static
  const canPortClients = portClientsSupported(sw)
  const macs = (port.clients ?? []).filter(
    (m) => !boxMacs.has(m.toUpperCase()) && m.toUpperCase() !== sw.mac.toUpperCase(),
  )
  const speed = port.up ? speedLabel(port.speed_mbps) : ''
  const poeOn = port.poe_status === 'delivering' || (port.poe_w != null && port.poe_w > 0)
  const stats = portStats(port, dir)
  const lag = lagPeers(sw, port.id)
  const leafRole = uplink ? (staticBind ? ' · Uplink · Static bind' : ' · Uplink') : ''

  return (
    <div className="space-y-4">
      {hideCrumb ? null : (
      <DetailCrumb
        root="Topology"
        leaf={`Port ${port.id}${leafRole}`}
        onRoot={onBack}
      />
      )}

      <Card className="gap-0 py-0">
        <CardContent className="overflow-x-auto px-6 py-6">
          <div className="flex items-center gap-4">
            <Chassis maker={maker}>
              <ChassisPorts
                sw={sw}
                ports={[...(sw.ports ?? [])].sort((a, b) => Number(a.id) - Number(b.id) || a.id.localeCompare(b.id))}
                selected={port.id}
                onSelect={(id) => onPickPort(id === port.id ? null : id)}
              />
            </Chassis>
            <DeviceHead
              name={sw.name ?? sw.mac}
              ip={sw.ip ?? '—'}
              model={switchModelSKU(sw.model)}
              devices={uplink || !canPortClients ? undefined : macs.length}
              clientsUnsupported={!uplink && !canPortClients}
              onDevices={
                uplink || !canPortClients ? undefined : () => openSwitchClients(onOpenClients, sw, macs)
              }
            />
          </div>
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardContent className="divide-y px-0">
          <MetaRow label="Status" value={port.up ? 'Up' : 'Down'} />
          {port.sfp ? <MetaRow label="Media" value="SFP" /> : null}
          {speed ? <MetaRow label="Port speed" value={gbpsLabel(port.speed_mbps)} /> : null}
          {lag ? <MetaRow label="LAG" value={lag.join(' + ')} /> : null}
          {uplink ? <MetaRow label="Uplink" value={uplinkTo} /> : null}
          {staticBind ? <MetaRow label="Bind" value="Static" /> : null}
          {!uplink ? (
            <ClientsMetaRow
              macs={macs}
              devices={devices}
              supported={canPortClients}
              sw={sw}
              onOpenClients={onOpenClients}
              onSelectDevice={onSelectDevice}
            />
          ) : null}
          {poeOn ? (
            <MetaRow
              label="PoE"
              value={[
                port.poe_w != null ? `${port.poe_w.toFixed(1)} W` : '',
                poeTypeLabel(port.poe_mode),
              ]
                .filter(Boolean)
                .join(' · ')}
            />
          ) : null}
        </CardContent>
      </Card>

      {stats.length > 0 ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <div className="flex items-center justify-between gap-4">
              <CardTitle className={SECTION_TITLE}>Stats</CardTitle>
              <div className="inline-flex rounded-md bg-muted p-0.5 text-xs">
                <button
                  type="button"
                  onClick={() => setDir('in')}
                  className={cn(
                    'rounded px-2.5 py-1',
                    dir === 'in' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground',
                  )}
                >
                  Input
                </button>
                <button
                  type="button"
                  onClick={() => setDir('out')}
                  className={cn(
                    'rounded px-2.5 py-1',
                    dir === 'out' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground',
                  )}
                >
                  Output
                </button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="divide-y px-0">
            {stats.map((r) => (
              <MetaRow key={r.label} label={r.label} value={r.value} />
            ))}
          </CardContent>
        </Card>
      ) : null}
    </div>
  )
}

function MetaRow({
  label,
  value,
  onClick,
}: {
  label: string
  value: string
  onClick?: () => void
}) {
  return (
    <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
      <div className="text-muted-foreground">{label}</div>
      {onClick ? (
        <button
          type="button"
          onClick={onClick}
          className="inline-flex min-w-0 items-center justify-end gap-0.5 break-all text-right font-medium tabular-nums text-muted-foreground hover:text-foreground"
        >
          <span className="min-w-0 truncate">{value}</span>
          <ChevronRight className="size-4 shrink-0" aria-hidden />
        </button>
      ) : (
        <div className="min-w-0 break-all text-right font-medium tabular-nums">{value}</div>
      )}
    </div>
  )
}

function poeTypeLabel(mode?: string): string {
  switch ((mode ?? '').toLowerCase()) {
    case '802.3bt':
      return 'PoE++ (802.3bt)'
    case '802.3at':
      return 'PoE+ (802.3at)'
    case '802.3af':
      return 'PoE (802.3af)'
    default:
      return mode?.trim() || ''
  }
}

function gbpsLabel(mbps?: number): string {
  if (mbps == null || mbps <= 0) return ''
  if (mbps >= 1000) return `${mbps / 1000} Gbps`
  return `${mbps} Mbps`
}

function portStats(port: SwitchPort, dir: 'in' | 'out'): { label: string; value: string }[] {
  const bytes = dir === 'in' ? port.rx_bytes : port.tx_bytes
  const uni = dir === 'in' ? port.rx_unicast : port.tx_unicast
  const bcast = dir === 'in' ? port.rx_broadcast : port.tx_broadcast
  const mcast = dir === 'in' ? port.rx_multicast : port.tx_multicast
  const drop = dir === 'in' ? port.rx_discard : port.tx_discard
  const err = dir === 'in' ? port.rx_error : port.tx_error
  const pkts = (uni ?? 0) + (bcast ?? 0) + (mcast ?? 0)
  if (!bytes && !pkts && !drop && !err) return []
  return [
    { label: 'Data', value: fmtBytes(bytes ?? 0) },
    { label: 'Total packets', value: fmtCount(pkts) },
    { label: 'Unicast', value: fmtCount(uni ?? 0) },
    { label: 'Broadcast', value: fmtCount(bcast ?? 0) },
    { label: 'Multicast', value: fmtCount(mcast ?? 0) },
    { label: 'Dropped', value: fmtCount(drop ?? 0) },
    { label: 'Error', value: fmtCount(err ?? 0) },
  ]
}

function Kv({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-muted-foreground">{label}</div>
      <div className="font-medium tabular-nums">{value}</div>
    </div>
  )
}
