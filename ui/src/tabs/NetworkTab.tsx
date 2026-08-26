import { ArrowLeftRight, EthernetPort, Shuffle, Wifi } from 'lucide-react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { fmtRelative, netLabel } from '@/lib/format'
import { managerKind, portChips, wanTypeLabel, type PortChip } from '@/lib/ports'
import type { NetIface, VirtWAN, WanFeatures, WanTest } from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const LAN_GRID =
  'grid w-full grid-cols-[minmax(8rem,1fr)_12rem_5.5rem_1.5rem_auto] items-center gap-x-3 px-6 py-2.5 text-sm'
const WAN_GRID =
  'grid w-full grid-cols-[minmax(8rem,1fr)_12rem_4.5rem_5.5rem_5.5rem_auto] items-center gap-x-3 px-6 py-2.5 text-sm'

type NetworkTabProps = {
  network: NetIface[]
  wanType?: string
  features?: WanFeatures
  wanTest?: WanTest
  virtWans?: VirtWAN[]
  onSelectLan: (uuid: string) => void
}

export function NetworkTab({
  network,
  wanType,
  features,
  wanTest,
  virtWans,
  onSelectLan,
}: NetworkTabProps) {
  const lans = network.filter((n) => managerKind(n) === 'lan').sort(lanOrder)
  const wans = network.filter((n) => managerKind(n) === 'wan')

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold tracking-tight">Network</h1>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Local</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {lans.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No networks</p>
          ) : (
            lans.map((n) => (
              <NetworkRow
                key={n.uuid || n.name}
                n={n}
                onClick={() => n.uuid && onSelectLan(n.uuid)}
                clickable={!!n.uuid}
              />
            ))
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={`${SECTION_HEADER} items-center`}>
          <CardTitle className={SECTION_TITLE}>WAN</CardTitle>
          <CardAction>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <FeatureChip value={features?.dual_wan} on="dual_wan" off="dual_wan off" />
              <FeatureChip
                value={features?.single_wan_conn_check}
                on="conn_check"
                off="conn_check off"
              />
              <MultiWanChip wanType={wanType} wans={wans} />
            </div>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          {wans.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No WAN</p>
          ) : (
            <div className="divide-y">
              {wans.map((n) => <WanRow key={n.uuid || n.name} n={n} />)}
            </div>
          )}
          {virtWans?.length ? <VirtWanStrip virtWans={virtWans} /> : null}
          {wanTest ? <WanTestStrip wanTest={wanTest} /> : null}
        </CardContent>
      </Card>
    </div>
  )
}

function FeatureChip({
  value,
  on,
  off,
}: {
  value?: boolean
  on: string
  off: string
}) {
  if (value == null) return null
  return <Badge variant="outline">{value ? on : off}</Badge>
}

function VirtWanStrip({ virtWans }: { virtWans: VirtWAN[] }) {
  return (
    <div className="space-y-1 border-t border-dashed px-6 py-3 text-xs text-muted-foreground">
      {virtWans.map((wan, i) => {
        const parts = [wan.name || wan.uuid || '—', wan.type || '—', wan.conn_state || '—']
        if (wan.failback != null) parts.push(wan.failback ? 'failback' : 'failback off')
        if (wan.strict_vpn != null) parts.push(wan.strict_vpn ? 'strictVPN' : 'strictVPN off')
        return <div key={wan.uuid || wan.name || i}>{parts.join(' · ')}</div>
      })}
    </div>
  )
}

function WanTestStrip({ wanTest }: { wanTest: WanTest }) {
  const [nowMs] = useState(() => Date.now())

  return (
    <div className="space-y-1 border-t border-dashed px-6 py-3 text-xs text-muted-foreground">
      <div>
        <span className="font-medium text-foreground">Last test</span>
        <span className="mx-1.5">·</span>
        connected {wanTest.connected ? 'yes' : 'no'}
      </div>
      {Object.entries(wanTest.wans).map(([iface, wan]) => {
        const parts = [iface]
        if (wan.ready != null) parts.push(wan.ready ? 'ready' : 'not ready')
        if (wan.active != null) parts.push(wan.active ? 'active' : 'standby')
        if (wan.failures?.[0]) parts.push(wan.failures[0])
        if (wan.ts != null) {
          const tsSec = wan.ts > 1e12 ? wan.ts / 1000 : wan.ts
          parts.push(fmtRelative(tsSec, nowMs))
        }
        return <div key={iface}>{parts.join(' · ')}</div>
      })}
    </div>
  )
}

function lanBucket(n: NetIface): number {
  const name = n.name
  if (name.startsWith('wg') || name.startsWith('tun_fwvpn')) return 3
  if ((n.intfs ?? []).some((i) => i.startsWith('wlan'))) return 2
  if (n.vid) return 1
  return 0
}

function lanOrder(a: NetIface, b: NetIface): number {
  const d = lanBucket(a) - lanBucket(b)
  if (d !== 0) return d
  return netLabel(a).localeCompare(netLabel(b), undefined, { sensitivity: 'base' })
}

function MultiWanChip({ wanType, wans }: { wanType?: string; wans: NetIface[] }) {
  const label = wanTypeLabel(wanType)
  if (!label && wans.length < 2) return null
  if (label === 'Single' && wans.length < 2) return null

  const primary = wans.find((w) => w.wan_active)
  const standby = wans.find((w) => w.wan_active === false)
  const failover = label === 'Failover' || wanType === 'primary_standby'
  const Icon = failover ? ArrowLeftRight : Shuffle

  return (
    <div className="inline-flex items-center gap-2 rounded-full border bg-muted/50 px-2.5 py-1 text-xs">
      <Icon className="size-3.5 text-muted-foreground" />
      <span className="font-medium">{label || 'Multi-WAN'}</span>
      {primary && standby ? (
        <span className="text-muted-foreground">
          {netLabel(primary)}
          <span className="mx-1">→</span>
          {netLabel(standby)}
        </span>
      ) : null}
    </div>
  )
}

function NetworkRow({
  n,
  onClick,
  clickable,
}: {
  n: NetIface
  onClick: () => void
  clickable: boolean
}) {
  const chips = portChips(n.intfs, n.vid)
  const hasWifi = (n.intfs ?? []).some((i) => i.startsWith('wlan'))
  const vpn = n.name.startsWith('wg') || n.name.startsWith('tun_fwvpn')
  const cidr = n.subnet || n.ip || '—'

  const inner = (
    <div className={LAN_GRID}>
      <div className="truncate font-medium">{netLabel(n)}</div>
      <div className="truncate font-mono text-muted-foreground">{cidr}</div>
      <div>
        {n.vid ? (
          <Badge variant="outline" className="font-mono">
            VLAN {n.vid}
          </Badge>
        ) : null}
      </div>
      <div className="flex size-6 items-center justify-center">
        {hasWifi ? <WifiChip /> : null}
      </div>
      <div className={cn(vpn && 'invisible')}>
        <GoldPortChips chips={chips} />
      </div>
    </div>
  )

  if (clickable) {
    return (
      <button type="button" className="w-full text-left hover:bg-accent/40" onClick={onClick}>
        {inner}
      </button>
    )
  }
  return inner
}

function WanRow({ n }: { n: NetIface }) {
  const chips = portChips(n.intfs?.length ? n.intfs : [n.name])
  const addr = n.ip || n.subnet || '—'
  const mode = n.dhcp ? 'DHCP' : 'Static'
  const status = n.wan_active == null ? null : n.wan_active ? 'Active' : 'Standby'

  return (
    <div className={WAN_GRID}>
      <div className="truncate font-medium">{netLabel(n)}</div>
      <div className="truncate font-mono text-muted-foreground">{addr}</div>
      <div className="text-muted-foreground">{mode}</div>
      <div className="text-muted-foreground">
        {n.wan_ready == null ? null : n.wan_ready ? 'ready' : 'not ready'}
      </div>
      <div>
        {status ? <Badge variant={n.wan_active ? 'default' : 'secondary'}>{status}</Badge> : null}
      </div>
      <GoldPortChips chips={chips} />
    </div>
  )
}

function WifiChip() {
  return (
    <span
      title="Wi-Fi"
      className="inline-flex size-6 items-center justify-center rounded border border-transparent bg-sky-500 text-white"
    >
      <Wifi className="size-3.5" aria-label="Wi-Fi" />
    </span>
  )
}

function GoldPortChips({ chips, up }: { chips: PortChip[]; up?: boolean[] }) {
  return (
    <div className="inline-flex gap-1">
      {chips.map((c, i) => (
        <span
          key={c.n}
          title={c.unused ? `Port ${c.n}` : c.tagged ? `Port ${c.n} tagged` : `Port ${c.n}`}
          className={cn(
            'inline-flex size-6 items-center justify-center rounded border',
            c.unused && 'border-dashed border-muted-foreground/30 text-muted-foreground/40',
            !c.unused && !c.tagged && 'border-transparent bg-foreground/80 text-background',
            !c.unused &&
              c.tagged &&
              'border-muted-foreground bg-[repeating-linear-gradient(-45deg,transparent,transparent_2px,rgba(128,128,128,0.25)_2px,rgba(128,128,128,0.25)_4px)]',
            up && up[i] === false && 'opacity-40',
          )}
        >
          <EthernetPort className="size-3.5" aria-hidden />
        </span>
      ))}
    </div>
  )
}

export { GoldPortChips }
