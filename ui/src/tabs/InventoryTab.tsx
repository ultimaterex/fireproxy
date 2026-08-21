import { useState, type ReactNode } from 'react'
import { EthernetPort, MonitorSmartphone, Network, Server, Shield, Wifi } from 'lucide-react'

import { CopyText } from '@/components/CopyText'
import { SourceBadge } from '@/components/SourceBadge'

import { Flag } from '@/components/Flag'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { netLabel } from '@/lib/format'
import { makerLabel, makerLogo } from '@/lib/maker'
import { managerKind, wanTypeLabel } from '@/lib/ports'
import type { BoxInfo, BoxMAC, Device, ModuleInfo, NetIface, Policy, Switch, UnifiConsole } from '@/lib/types'
import { cn } from '@/lib/utils'

type Mode = 'firewalla' | 'unifi'

const MODES: { id: Mode; maker: 'firewalla' | 'unifi'; label: string }[] = [
  { id: 'firewalla', maker: 'firewalla', label: 'Firewalla' },
  { id: 'unifi', maker: 'unifi', label: 'UniFi' },
]

export function InventoryTab({
  box,
  source,
  stale,
  unifi,
  console: ucon,
  devices,
  network,
  switches,
  policies,
}: {
  box: BoxInfo | null
  source?: string
  stale?: boolean
  unifi: ModuleInfo | null
  console: UnifiConsole | null
  devices: Device[]
  network: NetIface[]
  switches: Switch[]
  policies: Policy[]
}) {
  const [mode, setMode] = useState<Mode>('firewalla')
  const unifiOn = !!unifi?.enabled
  const active: Mode = unifiOn && mode === 'unifi' ? 'unifi' : 'firewalla'
  const modes = unifiOn ? MODES : MODES.filter((m) => m.id === 'firewalla')

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-lg font-semibold tracking-tight">Inventory</h1>
        {modes.length > 1 ? (
          <div className="relative inline-grid grid-cols-2 rounded-md border p-0.5">
            <span
              aria-hidden
              className={cn(
                'pointer-events-none absolute inset-y-0.5 left-0.5 w-[calc(50%-2px)] rounded-md bg-accent transition-transform duration-250 ease-out',
                active === 'unifi' && 'translate-x-full',
              )}
            />
            {modes.map((m) => (
              <Button
                key={m.id}
                type="button"
                size="xs"
                variant="ghost"
                className={cn(
                  'relative z-10 hover:bg-transparent dark:hover:bg-transparent',
                  active === m.id ? 'text-foreground' : 'text-muted-foreground',
                )}
                onClick={() => setMode(m.id)}
              >
                <img src={makerLogo(m.maker)} alt="" className="size-4 rounded-sm object-contain" />
                {m.label}
              </Button>
            ))}
          </div>
        ) : null}
      </div>
      {active === 'firewalla' ? (
        <FirewallaPane
          box={box}
          source={source}
          stale={stale}
          devices={devices}
          network={network}
          switches={switches}
          policies={policies}
        />
      ) : (
        <UnifiPane unifi={unifi} console={ucon} />
      )}
    </div>
  )
}

function FirewallaPane({
  box,
  source,
  stale,
  devices,
  network,
  switches,
  policies,
}: {
  box: BoxInfo | null
  source?: string
  stale?: boolean
  devices: Device[]
  network: NetIface[]
  switches: Switch[]
  policies: Policy[]
}) {
  const [macOpen, setMacOpen] = useState(false)
  if (!box) {
    return <p className="text-sm text-muted-foreground">Waiting on catalog</p>
  }

  const macs = labeledMacs(box.macs)
  const fwSwitches = switches.filter((s) => s.source !== 'unifi' && s.kind !== 'ap')
  const lans = network.filter((n) => managerKind(n) === 'lan')
  const wans = network.filter((n) => managerKind(n) === 'wan')
  const wanNames = wans.map(netLabel).filter(Boolean)
  const wan = wanTypeLabel(box.wan_type)
  const wanValue = [wan, wanNames.join(', ')].filter(Boolean).join(' · ')

  const rows: { label: string; value: ReactNode }[] = []
  if (box.public_ip) rows.push({ label: 'Public IP', value: <CopyText value={box.public_ip} /> })
  if (wanValue) rows.push({ label: 'WAN', value: wanValue })
  if (box.version) rows.push({ label: 'Version', value: box.version })
  if (box.mode) rows.push({ label: 'Mode', value: modeLabel(box.mode) })
  if (box.region) {
    rows.push({
      label: 'Location',
      value: (
        <span className="inline-flex items-center gap-2">
          <Flag cc={box.country} />
          {box.region}
        </span>
      ),
    })
  }
  if (box.timezone) rows.push({ label: 'Timezone', value: box.timezone })
  if (box.eid) rows.push({ label: 'EID', value: box.eid })
  if (box.ddns) rows.push({ label: 'DDNS', value: <CopyText value={box.ddns} mono /> })
  if (macs.length) {
    rows.push({
      label: 'MAC Addresses',
      value: (
        <button type="button" className="cursor-pointer font-medium" onClick={() => setMacOpen(true)}>
          {macs.length} address{macs.length === 1 ? '' : 'es'}
        </button>
      ),
    })
  }

  return (
    <div className="space-y-3">
      <Card className="gap-0 py-0">
        <CardHeader className="px-6 pt-6 pb-4">
          <CardTitle className="flex items-center gap-3 text-xl font-normal leading-8">
            <img src={makerLogo('firewalla')} alt="" className="size-8 rounded-sm object-contain" />
            <span>{box.name || makerLabel('firewalla')}</span>
            <SourceBadge source={source} stale={stale} />
            {box.license ? (
              <span className="ml-auto text-sm text-muted-foreground">{box.license}</span>
            ) : null}
          </CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          <div className="flex flex-wrap gap-6 px-6 py-4 text-sm">
            <Count icon={MonitorSmartphone} n={devices.length} label="clients" />
            <Count icon={EthernetPort} n={fwSwitches.length} label="switches" />
            <Count icon={Network} n={lans.length} label="LANs" />
            <Count icon={Shield} n={policies.length} label="rules" />
          </div>
          {rows.map((r) => (
            <div key={r.label} className="grid grid-cols-[12rem_minmax(0,1fr)] gap-4 px-6 py-4 text-sm">
              <div className="text-muted-foreground">{r.label}</div>
              <div className="min-w-0 break-all font-medium">{r.value}</div>
            </div>
          ))}
        </CardContent>
      </Card>
      {macOpen ? <MacModal addrs={macs} onClose={() => setMacOpen(false)} /> : null}
    </div>
  )
}

function UnifiPane({ unifi, console: c }: { unifi: ModuleInfo | null; console: UnifiConsole | null }) {
  if (!unifi?.enabled) {
    return <p className="text-sm text-muted-foreground">Enable UniFi in Settings.</p>
  }
  if (!c) {
    return <p className="text-sm text-muted-foreground">No UniFi data yet.</p>
  }

  const rows: { label: string; value: ReactNode }[] = []
  if (c.ip) rows.push({ label: 'Server IP', value: c.ip })
  if (c.uptime_sec) rows.push({ label: 'Uptime', value: formatUptime(c.uptime_sec) })
  if (c.network_version) {
    rows.push({
      label: `Network ${c.network_version}`,
      value: c.network_update ? 'Update available' : 'Up to date',
    })
  }
  if (c.hardware) rows.push({ label: 'Hardware', value: c.hardware })
  if (c.site) rows.push({ label: 'Site', value: c.site })
  if (c.inform) rows.push({ label: 'Inform URL', value: <CopyText value={c.inform} /> })
  if (c.cloud != null) rows.push({ label: 'Cloud', value: c.cloud ? 'Connected' : 'Offline' })

  const status =
    unifi.status === 'ok' ? 'Connected' : unifi.status === 'down' ? unifi.detail || 'Down' : '—'

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="px-6 pt-6 pb-4">
        <CardTitle className="flex items-center gap-3 text-xl font-normal leading-8">
          <img src={makerLogo('unifi')} alt="" className="size-8 rounded-sm object-contain" />
          <span>{c.name || makerLabel('unifi')}</span>
          <span className="ml-auto text-sm text-muted-foreground">{status}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="divide-y px-0">
        <div className="flex flex-wrap gap-6 px-6 py-4 text-sm">
          <Count icon={Server} n={c.devices} label="devices" />
          <Count icon={Wifi} n={c.aps} label="APs" />
          <Count icon={EthernetPort} n={c.switches} label="switches" />
          <Count icon={MonitorSmartphone} n={c.clients} label="clients" />
        </div>
        {rows.map((r) => (
          <div key={r.label} className="grid grid-cols-[12rem_minmax(0,1fr)] gap-4 px-6 py-4 text-sm">
            <div className="text-muted-foreground">{r.label}</div>
            <div className="min-w-0 break-all font-medium">{r.value}</div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function Count({
  icon: Icon,
  n,
  label,
}: {
  icon: typeof Server
  n?: number
  label: string
}) {
  if (n == null) return null
  return (
    <div className="inline-flex items-center gap-2 text-muted-foreground">
      <Icon className="size-4" />
      <span className="font-medium text-foreground tabular-nums">{n}</span>
      <span>{label}</span>
    </div>
  )
}

function formatUptime(sec: number): string {
  if (sec < 0) return '—'
  const m = Math.floor(sec / 2592000)
  const w = Math.floor((sec % 2592000) / 604800)
  const d = Math.floor((sec % 604800) / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const parts = []
  if (m) parts.push(`${m}m`)
  if (w || m) parts.push(`${w}w`)
  if (d || w || m) parts.push(`${d}d`)
  parts.push(`${h}h`)
  return parts.join(' ')
}

function labeledMacs(raw?: Array<BoxMAC | string>): BoxMAC[] {
  return (raw ?? []).map((m, i) =>
    typeof m === 'string' ? { name: `Address ${i + 1}`, mac: m } : m,
  )
}

function MacModal({ addrs, onClose }: { addrs: BoxMAC[]; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-160 overflow-hidden rounded-lg border bg-popover shadow-md"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="border-b px-6 py-4 text-base font-medium">MAC Addresses</div>
        <div className="space-y-4 px-6 py-5 text-sm">
          {addrs.map((a) => (
            <div key={a.mac} className="grid grid-cols-[10rem_minmax(0,1fr)] gap-4">
              <div className="text-muted-foreground">{a.name}</div>
              <div className="font-mono">{a.mac}</div>
            </div>
          ))}
        </div>
        <div className="flex justify-end border-t px-6 py-3">
          <button type="button" className="text-sm font-medium" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
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
