import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'

import { DeviceIcon } from '@/components/DeviceIcon'
import { Flag } from '@/components/Flag'
import { Toast } from '@/components/Toast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch as Toggle } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import {
  dapLabel,
  deviceOnline,
  fmtBytes,
  fmtCount,
  netLabel,
  preferredName,
} from '@/lib/format'
import type { PortLoc } from '@/lib/switch-port'
import type { Device, ModuleInfo, NetIface, RankedFlow, WirelessClient } from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const ROW = 'grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm'

/** Redis local:domain:suffix when known — empty if unset (no invented default). */
export function normalizeDomainSuffix(suffix?: string | null): string {
  return (suffix ?? '').trim().replace(/^\.+/, '').toLowerCase()
}

/** Hostname label only — strip a trailing known suffix if present. */
export function dnsHostnameLabel(host: string, suffix: string): string {
  const h = host.trim()
  const s = normalizeDomainSuffix(suffix)
  if (!h || !s) return h
  const lower = h.toLowerCase()
  const tail = '.' + s
  if (lower.endsWith(tail) && lower.length > tail.length) {
    return h.slice(0, h.length - tail.length)
  }
  return h
}

export function formatLocalFQDN(host: string, suffix?: string | null): string {
  const s = normalizeDomainSuffix(suffix)
  const label = dnsHostnameLabel(host, s)
  if (!label) return ''
  return s ? `${label}.${s}` : label
}

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

function FactRows({ rows, empty = true }: { rows: { label: string; value: string }[]; empty?: boolean }) {
  if (rows.length === 0) {
    return empty ? <p className="px-6 py-6 text-sm text-muted-foreground">—</p> : null
  }
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
  unifi,
  domainSuffix,
  onRenamed,
  onDNSUpdated,
}: {
  device: DeviceDetailModel
  nowMs: number
  lan?: NetIface
  port?: PortLoc
  groupLabel?: string
  unifi?: ModuleInfo | null
  domainSuffix?: string | null
  onRenamed?: (mac: string, name: string) => void
  onDNSUpdated?: (mac: string, hostname: string) => void
}) {
  const online = deviceOnline(
    { mac: device.mac, name: device.name, last_active_ts: device.last_active_ts },
    nowMs,
  )
  const wireless = !!(device.ssid || device.band || device.ap_mac)
  const suffix = normalizeDomainSuffix(domainSuffix)
  const [controlReady, setControlReady] = useState(false)
  const [wakeBusy, setWakeBusy] = useState(false)
  const [toast, setToast] = useState<string | null>(null)
  const [displayName, setDisplayName] = useState(device.name)
  const [displayDomain, setDisplayDomain] = useState(
    () => dnsHostnameLabel(device.local_domain ?? '', suffix),
  )
  const [renaming, setRenaming] = useState(false)
  const [editingDNS, setEditingDNS] = useState(false)
  const [draftName, setDraftName] = useState(device.name)
  const [draftDNS, setDraftDNS] = useState(() => dnsHostnameLabel(device.local_domain ?? '', suffix))
  const [pushUnifi, setPushUnifi] = useState(false)
  const [renameBusy, setRenameBusy] = useState(false)
  const [dnsBusy, setDnsBusy] = useState(false)

  const unifiPushAvailable =
    !!unifi?.enabled && unifi.status === 'ok' && !!unifi.name_sync_enabled

  useEffect(() => {
    const label = dnsHostnameLabel(device.local_domain ?? '', suffix)
    setDisplayName(device.name)
    setDraftName(device.name)
    setDisplayDomain(label)
    setDraftDNS(label)
    setRenaming(false)
    setEditingDNS(false)
  }, [device.name, device.local_domain, device.mac, suffix])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const r = await api('/v1/fw-app/status')
        if (!r.ok || cancelled) return
        const st = (await r.json()) as { paired?: boolean; state?: string }
        if (!cancelled) setControlReady(!!st.paired && st.state === 'lan-ok')
      } catch {
        if (!cancelled) setControlReady(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  function startRename() {
    setEditingDNS(false)
    setDraftName(displayName)
    setPushUnifi(unifiPushAvailable && !!unifi?.name_sync_auto)
    setRenaming(true)
  }

  function startDNS() {
    setRenaming(false)
    setDraftDNS(displayDomain)
    setEditingDNS(true)
  }

  async function wake() {
    if (!controlReady || wakeBusy) return
    setWakeBusy(true)
    try {
      const r = await api('/v1/fw-app/wol', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac: device.mac }),
      })
      const body = (await r.json().catch(() => ({}))) as { error?: string }
      if (!r.ok) {
        setToast(body.error || 'Wake failed')
        return
      }
      setToast(`Wake sent · ${displayName}`)
    } catch {
      setToast('Wake failed')
    } finally {
      setWakeBusy(false)
    }
  }

  async function saveRename() {
    if (!controlReady || renameBusy) return
    const name = draftName.trim()
    if (!name) {
      setToast('Name required')
      return
    }
    setRenameBusy(true)
    try {
      const r = await api('/v1/fw-app/hosts/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mac: device.mac,
          name,
          push_unifi: unifiPushAvailable ? pushUnifi : false,
        }),
      })
      const body = (await r.json().catch(() => ({}))) as {
        error?: string
        name?: string
        unifi_warning?: string
        unifi_pushed?: boolean
      }
      if (!r.ok) {
        setToast(body.error || 'Rename failed')
        return
      }
      const next = body.name || name
      setDisplayName(next)
      setRenaming(false)
      onRenamed?.(device.mac, next)
      setToast(body.unifi_warning ? `Renamed · ${body.unifi_warning}` : 'Renamed')
    } catch {
      setToast('Rename failed')
    } finally {
      setRenameBusy(false)
    }
  }

  async function saveDNS() {
    if (!controlReady || dnsBusy) return
    const hostname = dnsHostnameLabel(draftDNS, suffix)
    setDnsBusy(true)
    try {
      const r = await api('/v1/fw-app/hosts/dns', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac: device.mac, hostname }),
      })
      const body = (await r.json().catch(() => ({}))) as {
        error?: string
        hostname?: string
      }
      if (!r.ok) {
        setToast(body.error || 'DNS update failed')
        return
      }
      const next = dnsHostnameLabel(body.hostname ?? hostname, suffix)
      setDisplayDomain(next)
      setEditingDNS(false)
      onDNSUpdated?.(device.mac, next)
      setToast('Saved')
    } catch {
      setToast('DNS update failed')
    } finally {
      setDnsBusy(false)
    }
  }

  const identity = useMemo(() => {
    const rows: { label: string; value: string }[] = []
    if (device.hostname && device.hostname !== displayName) {
      rows.push({ label: 'Hostname', value: device.hostname })
    }
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
  }, [device, groupLabel, displayName])

  const showDomainRow = controlReady || !!displayDomain

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
            <div className="truncate text-lg font-semibold tracking-tight">{displayName}</div>
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
            {controlReady ? (
              <>
                <Button type="button" size="sm" variant="outline" disabled={renaming} onClick={startRename}>
                  Rename
                </Button>
                <Button type="button" size="sm" variant="outline" disabled={wakeBusy} onClick={wake}>
                  Wake
                </Button>
              </>
            ) : null}
            {device.ssid ? <Badge variant="secondary">{device.ssid}</Badge> : null}
            {device.band ? <Badge variant="outline">{device.band} GHz</Badge> : null}
            {device.ap_name ? <Badge variant="outline">{device.ap_name}</Badge> : null}
          </div>
        </CardContent>
        {renaming ? (
          <div className="space-y-3 border-t px-6 py-4">
            <Input
              value={draftName}
              onChange={(e) => setDraftName(e.target.value)}
              maxLength={64}
              autoFocus
              disabled={renameBusy}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void saveRename()
                if (e.key === 'Escape') setRenaming(false)
              }}
            />
            {unifiPushAvailable ? (
              <label className="flex items-center gap-2 text-sm">
                <Toggle checked={pushUnifi} onCheckedChange={setPushUnifi} disabled={renameBusy} />
                Also update UniFi
              </label>
            ) : null}
            <div className="flex gap-2">
              <Button type="button" size="sm" disabled={renameBusy} onClick={() => void saveRename()}>
                Save
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={renameBusy}
                onClick={() => setRenaming(false)}
              >
                Cancel
              </Button>
            </div>
          </div>
        ) : null}
      </Card>

      {toast
        ? createPortal(<Toast message={toast} onDismiss={() => setToast(null)} />, document.body)
        : null}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Identity</CardTitle>
          </CardHeader>
          <CardContent className="divide-y px-0">
            {showDomainRow ? (
              <div className={ROW}>
                <div className="text-muted-foreground">Domain</div>
                <div className="min-w-0">
                  {controlReady && editingDNS ? (
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="flex min-w-0 max-w-xs items-center gap-0.5 font-mono text-sm">
                        <Input
                          className="h-8 min-w-0 flex-1"
                          value={draftDNS}
                          onChange={(e) => setDraftDNS(e.target.value)}
                          maxLength={63}
                          autoFocus
                          disabled={dnsBusy}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') void saveDNS()
                            if (e.key === 'Escape') setEditingDNS(false)
                          }}
                        />
                        {suffix ? (
                          <span className="shrink-0 text-muted-foreground">.{suffix}</span>
                        ) : null}
                      </div>
                      <Button type="button" size="xs" disabled={dnsBusy} onClick={() => void saveDNS()}>
                        Save
                      </Button>
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        disabled={dnsBusy}
                        onClick={() => setEditingDNS(false)}
                      >
                        Cancel
                      </Button>
                    </div>
                  ) : controlReady ? (
                    <button
                      type="button"
                      className="break-all text-left font-medium hover:text-[#027BFF]"
                      onClick={startDNS}
                    >
                      {displayDomain ? formatLocalFQDN(displayDomain, suffix) : '—'}
                    </button>
                  ) : (
                    <div className="break-all font-medium">
                      {displayDomain ? formatLocalFQDN(displayDomain, suffix) : displayDomain}
                    </div>
                  )}
                </div>
              </div>
            ) : null}
            <FactRows rows={identity} empty={false} />
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
