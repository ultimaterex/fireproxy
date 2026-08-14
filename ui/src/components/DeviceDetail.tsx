import { useMemo } from 'react'

import { DeviceIcon } from '@/components/DeviceIcon'
import { Flag } from '@/components/Flag'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  dapLabel,
  deviceOnline,
  fmtBytes,
  fmtCount,
  netLabel,
  preferredName,
} from '@/lib/format'
import type { PortLoc } from '@/lib/switch-port'
import type { Device, NetIface, RankedFlow, WirelessClient } from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const ROW = 'grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm'

export type DeviceDetailModel = {
  mac: string
  name: string
  ip?: string
  ipv6?: string[]
  vendor?: string
  type?: string
  local_domain?: string
  last_active_ts?: number
  hostname?: string
  os?: string
  upload?: number
  download?: number
  top_dests?: RankedFlow[]
  tx_kbps?: number
  rx_kbps?: number
  ssid?: string
  band?: string
  ap_mac?: string
  ap_name?: string
  intf_uuid?: string
  tag_ids?: string[]
  dap?: Device['dap']
}

export function deviceFromCatalog(
  d: Device | undefined,
  wifi?: WirelessClient | null,
): DeviceDetailModel | null {
  if (!d && !wifi) return null
  if (!d && wifi) {
    return {
      mac: wifi.mac,
      name: wifi.name?.trim() || wifi.hostname?.trim() || wifi.mac,
      ip: wifi.ip,
      hostname: wifi.hostname,
      os: wifi.os,
      tx_kbps: wifi.tx_kbps,
      rx_kbps: wifi.rx_kbps,
      ssid: wifi.ssid,
      band: wifi.band,
      ap_mac: wifi.ap_mac,
      ap_name: wifi.ap_name,
      last_active_ts: Date.now() / 1000,
    }
  }
  const cat = d!
  return {
    mac: cat.mac,
    name: preferredName(cat),
    ip: cat.ip || wifi?.ip,
    ipv6: cat.ipv6,
    vendor: cat.vendor,
    type: cat.type,
    local_domain: cat.local_domain,
    last_active_ts: cat.last_active_ts,
    hostname: cat.hostname || wifi?.hostname,
    os: cat.os || wifi?.os,
    upload: cat.upload,
    download: cat.download,
    top_dests: cat.top_dests,
    tx_kbps: cat.tx_kbps || wifi?.tx_kbps,
    rx_kbps: cat.rx_kbps || wifi?.rx_kbps,
    ssid: cat.ssid || wifi?.ssid,
    band: cat.band || wifi?.band,
    ap_mac: cat.ap_mac || wifi?.ap_mac,
    ap_name: cat.ap_name || wifi?.ap_name,
    intf_uuid: cat.intf_uuid,
    tag_ids: cat.tag_ids,
    dap: cat.dap,
  }
}

function fmtRate(kbps?: number): string {
  if (kbps == null || kbps <= 0) return ''
  if (kbps >= 1_000_000) return `${(kbps / 1_000_000).toFixed(1)} Gbps`
  if (kbps >= 1000) return `${Math.round(kbps / 1000)} Mbps`
  return `${kbps} kbps`
}

function FactRows({ rows }: { rows: { label: string; value: string }[] }) {
  if (rows.length === 0) return <p className="px-6 py-6 text-sm text-muted-foreground">—</p>
  return (
    <>
      {rows.map((r) => (
        <div key={r.label} className={ROW}>
          <div className="text-muted-foreground">{r.label}</div>
          <div className="min-w-0 break-all font-medium">{r.value}</div>
        </div>
      ))}
    </>
  )
}

export function DeviceDetail({
  device,
  nowMs,
  lan,
  port,
  groupLabel,
}: {
  device: DeviceDetailModel
  nowMs: number
  lan?: NetIface
  port?: PortLoc
  groupLabel?: string
}) {
  const online = deviceOnline(
    { mac: device.mac, name: device.name, last_active_ts: device.last_active_ts },
    nowMs,
  )
  const wireless = !!(device.ssid || device.band || device.ap_mac)

  const identity = useMemo(() => {
    const rows: { label: string; value: string }[] = []
    if (device.hostname && device.hostname !== device.name) {
      rows.push({ label: 'Hostname', value: device.hostname })
    }
    if (device.local_domain) rows.push({ label: 'Domain', value: device.local_domain })
    if (device.vendor) rows.push({ label: 'Manufacturer', value: device.vendor })
    if (device.type) rows.push({ label: 'Type', value: device.type })
    if (device.os) rows.push({ label: 'OS', value: device.os })
    if (groupLabel) rows.push({ label: 'Group', value: groupLabel })
    if (device.dap) {
      rows.push({
        label: 'DAP',
        value: `${dapLabel(device.dap.status)}${device.dap.reason ? ` · ${device.dap.reason}` : ''}`,
      })
      rows.push({
        label: 'Learned',
        value: `${fmtCount(device.dap.learned)} / ${fmtCount(device.dap.trusted)} trusted / ${fmtCount(device.dap.blocked)} blocked`,
      })
    }
    return rows
  }, [device, groupLabel])

  const link = useMemo(() => {
    const rows: { label: string; value: string }[] = []
    if (lan) rows.push({ label: 'Network', value: netLabel(lan) })
    if (port) rows.push({ label: 'Port', value: port.label })
    if (device.ssid) rows.push({ label: 'SSID', value: device.ssid })
    if (device.band) rows.push({ label: 'Band', value: `${device.band} GHz` })
    if (device.ap_name || device.ap_mac) {
      rows.push({ label: 'AP', value: device.ap_name || device.ap_mac || '—' })
    }
    const tx = fmtRate(device.tx_kbps)
    const rx = fmtRate(device.rx_kbps)
    if (tx || rx) {
      rows.push({
        label: 'Rate',
        value: [tx && `↑ ${tx}`, rx && `↓ ${rx}`].filter(Boolean).join(' · '),
      })
    }
    if (device.ipv6?.length) rows.push({ label: 'IPv6', value: device.ipv6.join(', ') })
    return rows
  }, [device, lan, port])

  const traffic = useMemo(() => {
    const rows: { label: string; value: string }[] = []
    if (device.download != null || device.upload != null) {
      rows.push({ label: 'Download 24h', value: fmtBytes(device.download ?? 0) })
      rows.push({ label: 'Upload 24h', value: fmtBytes(device.upload ?? 0) })
    }
    return rows
  }, [device])

  return (
    <div className="space-y-4">
      <Card className="gap-0 py-0">
        <CardContent className="flex flex-wrap items-center gap-4 px-6 py-5">
          <div className="flex size-12 items-center justify-center rounded-xl border bg-muted/40">
            <DeviceIcon type={device.type || (wireless ? 'phone' : undefined)} className="size-6 text-[#027BFF]" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate text-lg font-semibold tracking-tight">{device.name}</div>
            <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
              <span className="font-mono">{device.mac}</span>
              {device.ip ? <span className="font-mono">{device.ip}</span> : null}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-2 text-sm">
              <span className={cn('size-2 rounded-full', online ? 'bg-emerald-500' : 'bg-zinc-500')} />
              {online ? 'Online' : 'Offline'}
            </span>
            {device.ssid ? <Badge variant="secondary">{device.ssid}</Badge> : null}
            {device.band ? <Badge variant="outline">{device.band} GHz</Badge> : null}
            {device.ap_name ? <Badge variant="outline">{device.ap_name}</Badge> : null}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Identity</CardTitle>
          </CardHeader>
          <CardContent className="divide-y px-0">
            <FactRows rows={identity} />
          </CardContent>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Link</CardTitle>
          </CardHeader>
          <CardContent className="divide-y px-0">
            <FactRows rows={link} />
          </CardContent>
        </Card>

        {traffic.length > 0 ? (
          <Card className="gap-0 py-0 lg:col-span-2">
            <CardHeader className={SECTION_HEADER}>
              <CardTitle className={SECTION_TITLE}>Traffic</CardTitle>
            </CardHeader>
            <CardContent className="divide-y px-0">
              <FactRows rows={traffic} />
            </CardContent>
          </Card>
        ) : null}

        {device.top_dests?.length ? (
          <Card className="gap-0 py-0 lg:col-span-2">
            <CardHeader className={SECTION_HEADER}>
              <CardTitle className={SECTION_TITLE}>Top destinations</CardTitle>
            </CardHeader>
            <CardContent className="px-0 pb-2">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Destination</TableHead>
                    <TableHead>Region</TableHead>
                    <TableHead className="text-right">Upload</TableHead>
                    <TableHead className="text-right">Download</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {device.top_dests.map((t) => (
                    <TableRow key={t.id || t.name}>
                      <TableCell className="font-mono">{t.name}</TableCell>
                      <TableCell className="text-muted-foreground">
                        <span className="inline-flex items-center gap-2">
                          <Flag cc={t.country} />
                          {t.region || t.country || '—'}
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtBytes(t.upload)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtBytes(t.download)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </div>
  )
}

export function groupNameFor(
  d: DeviceDetailModel,
  labelTag: (id: string, preferType?: string) => string,
): string | undefined {
  const id = d.tag_ids?.[0]
  if (!id) return undefined
  return labelTag(id, 'group')
}
