import { useEffect, useState, type ReactNode } from 'react'
import { ArrowLeftRight, EthernetPort, Shuffle, Wifi } from 'lucide-react'

import { SourceBadge } from '@/components/SourceBadge'
import { Badge } from '@/components/ui/badge'
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api } from '@/lib/api'
import { fmtBytes, netLabel } from '@/lib/format'
import { managerKind, portChips, wanTypeLabel, type PortChip } from '@/lib/ports'
import type {
  NetIface,
  VPNClientFamily,
  VPNInventory,
  VPNPeer,
  VPNVIP,
  VPNVirtWAN,
} from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const LAN_GRID =
  'grid w-full grid-cols-[minmax(8rem,1fr)_12rem_5.5rem_1.5rem_auto] items-center gap-x-3 px-6 py-2.5 text-sm'
const WAN_GRID =
  'grid w-full grid-cols-[minmax(8rem,1fr)_12rem_4.5rem_5.5rem_auto] items-center gap-x-3 px-6 py-2.5 text-sm'
const VPN_GRID =
  'grid w-full grid-cols-[minmax(8rem,1fr)_minmax(6rem,10rem)_auto] items-center gap-x-3 px-6 py-2.5 text-sm'

const FAMILY_LABEL: Record<string, string> = {
  wireguard: 'WireGuard clients',
  amneziawg: 'AmneziaWG clients',
  openvpn: 'OpenVPN clients',
  ssl: 'SSL VPN clients',
  ipsec: 'IPSec clients',
  trojan: 'Trojan clients',
  clash: 'Clash clients',
  nebula: 'Nebula clients',
  tailscale: 'Tailscale clients',
  zerotier: 'ZeroTier clients',
  gost: 'Gost clients',
  hysteria: 'Hysteria clients',
  generic: 'VPN profiles',
}

export function NetworkTab({
  network,
  wanType,
  onSelectLan,
}: {
  network: NetIface[]
  wanType?: string
  onSelectLan: (uuid: string) => void
}) {
  const lans = network.filter((n) => managerKind(n) === 'lan').sort(lanOrder)
  const wans = network.filter((n) => managerKind(n) === 'wan')
  const [vpn, setVpn] = useState<VPNInventory | null>(null)
  const [vpnErr, setVpnErr] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const r = await api('/v1/vpn')
        if (r.status === 404) {
          if (!cancelled) {
            setVpn(null)
            setVpnErr('Not in init')
          }
          return
        }
        if (!r.ok) {
          if (!cancelled) {
            setVpn(null)
            setVpnErr(await r.text())
          }
          return
        }
        const data = (await r.json()) as VPNInventory
        if (!cancelled) {
          setVpn(data)
          setVpnErr(null)
        }
      } catch (e) {
        if (!cancelled) {
          setVpn(null)
          setVpnErr(e instanceof Error ? e.message : 'VPN load failed')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

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
            <MultiWanChip wanType={wanType} wans={wans} />
          </CardAction>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {wans.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No WAN</p>
          ) : (
            wans.map((n) => <WanRow key={n.uuid || n.name} n={n} />)
          )}
        </CardContent>
      </Card>

      <VPNRegion vpn={vpn} err={vpnErr} />
    </div>
  )
}

function VPNRegion({ vpn, err }: { vpn: VPNInventory | null; err: string | null }) {
  const peers = [...(vpn?.wg_peers ?? []), ...(vpn?.awg_peers ?? [])]
  const families = (vpn?.client_profiles ?? []).filter((f) => (f.profiles?.length ?? 0) > 0)
  const vips = vpn?.vips ?? []
  const virt = vpn?.virt_wans ?? []
  const hasData = peers.length > 0 || families.length > 0 || vips.length > 0 || virt.length > 0

  return (
    <Card className="gap-0 py-0">
      <CardHeader className={`${SECTION_HEADER} items-center`}>
        <CardTitle className={SECTION_TITLE}>VPN</CardTitle>
        <CardAction>
          <SourceBadge source={vpn?.source} stale={vpn?.stale} reason={vpn?.reason} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4 px-0 pb-4">
        {err && !hasData ? (
          <p className="px-6 py-2 text-sm text-muted-foreground">{err}</p>
        ) : null}
        {!err && !hasData ? (
          <p className="px-6 py-2 text-sm text-muted-foreground">Not in init</p>
        ) : null}
        {peers.length > 0 ? (
          <VPNSection title="WireGuard peers">
            {peers.map((p, i) => (
              <PeerRow key={`${p.name}-${p.intf}-${i}`} p={p} />
            ))}
          </VPNSection>
        ) : null}
        {families.map((f) => (
          <VPNSection key={f.family} title={FAMILY_LABEL[f.family] || f.family}>
            {f.profiles.map((p) => (
              <ProfileRow key={p.profile_id || p.display_name} p={p} />
            ))}
          </VPNSection>
        ))}
        {vips.length > 0 ? (
          <VPNSection title="VIP">
            {vips.map((v) => (
              <VIPRow key={v.uid || v.ip || v.name} v={v} />
            ))}
          </VPNSection>
        ) : null}
        {virt.length > 0 ? (
          <VPNSection title="Virt WAN">
            {virt.map((v) => (
              <VirtRow key={v.uuid || v.name} v={v} />
            ))}
          </VPNSection>
        ) : null}
      </CardContent>
    </Card>
  )
}

function VPNSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <div className="px-6 pb-2 text-sm font-medium text-muted-foreground">{title}</div>
      <div className="divide-y">{children}</div>
    </div>
  )
}

function PeerRow({ p }: { p: VPNPeer }) {
  const key = p.public_key ? `${p.public_key.slice(0, 8)}…` : '—'
  const meta = [
    p.intf,
    p.rx_bytes || p.tx_bytes ? `${fmtBytes(p.rx_bytes)} ↓ / ${fmtBytes(p.tx_bytes)} ↑` : null,
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <div className={VPN_GRID}>
      <div className="truncate font-medium">{p.name || '—'}</div>
      <div className="truncate font-mono text-muted-foreground">{key}</div>
      <div className="truncate text-muted-foreground">{meta || null}</div>
    </div>
  )
}

function ProfileRow({ p }: { p: VPNClientFamily['profiles'][number] }) {
  const label = p.display_name || p.profile_id || '—'
  const status = p.status || '—'
  return (
    <div className={VPN_GRID}>
      <div className="truncate font-medium">{label}</div>
      <div>
        <Badge variant={status === 'connected' ? 'default' : 'secondary'}>{status}</Badge>
      </div>
      <div className="truncate font-mono text-muted-foreground">{p.remote_ip || p.local_ip || null}</div>
    </div>
  )
}

function VIPRow({ v }: { v: VPNVIP }) {
  return (
    <div className={VPN_GRID}>
      <div className="truncate font-medium">{v.name || v.uid || '—'}</div>
      <div className="truncate font-mono text-muted-foreground">{v.ip || '—'}</div>
      <div />
    </div>
  )
}

function VirtRow({ v }: { v: VPNVirtWAN }) {
  return (
    <div className={VPN_GRID}>
      <div className="truncate font-medium">{v.name || v.uuid || '—'}</div>
      <div className="truncate text-muted-foreground">{v.type || '—'}</div>
      <div className="truncate text-muted-foreground">{v.conn_state || null}</div>
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
