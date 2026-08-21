import { type ReactNode, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

import { DeviceIcon } from '@/components/DeviceIcon'
import { Flag } from '@/components/Flag'
import { Donut } from '@/components/Donut'
import { RegionMap } from '@/components/RegionMap'
import { SourceBadge } from '@/components/SourceBadge'
import { completeHourPoints, DnsChart, SpeedSpark, speedTrend } from '@/components/SpeedSpark'
import { Toast } from '@/components/Toast'
import { TransferChart } from '@/components/TransferChart'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import { DOWN, UP } from '@/lib/brand'
import { cpuBusy, cpuTone, fmtBytes, fmtCount, fmtMbps, fmtPlan, fmtRelative, fmtTime } from '@/lib/format'
import {
  type Dashboard,
  type LatestView,
  type PersistInfo,
  type RankedFlow,
  type SpeedtestWAN,
} from '@/lib/types'
import { cn } from '@/lib/utils'

export function MetricsTab({
  latest,
  persist,
  dashboard,
  agentOnline,
  deviceCount,
  ruleCount,
  onOpenDevices,
  onOpenRules,
  onSelectDevice,
  onSelectRegion,
  onDashboard,
}: {
  latest: LatestView | null
  persist: PersistInfo
  dashboard: Dashboard | null
  agentOnline: boolean
  deviceCount: number
  ruleCount: number
  onOpenDevices: () => void
  onOpenRules: () => void
  onSelectDevice?: (mac: string, label: string) => void
  onSelectRegion?: (cc: string, label: string) => void
  onDashboard?: (dash: Dashboard) => void
}) {
  const [detail, setDetail] = useState<'speed' | null>(null)
  const [heroFrom, setHeroFrom] = useState<HeroRect | null>(null)
  const [speedClosing, setSpeedClosing] = useState(false)
  const speedTileRef = useRef<HTMLButtonElement>(null)
  const snap = latest?.snapshot
  if (!snap && !dashboard) {
    return (
      <p className="text-sm text-muted-foreground">
        {agentOnline ? 'No snapshots yet' : 'Firewalla offline'}
      </p>
    )
  }

  const openSpeed = () => {
    const r = speedTileRef.current?.getBoundingClientRect()
    setHeroFrom(r ? { left: r.left, top: r.top, width: r.width, height: r.height } : null)
    setSpeedClosing(false)
    setDetail('speed')
  }

  const beginCloseSpeed = () => setSpeedClosing(true)

  const closeSpeed = () => {
    setDetail(null)
    setHeroFrom(null)
    setSpeedClosing(false)
  }

  const cpuPct = snap?.cpu ? cpuBusy(snap.cpu) : null

  const wans = snap
    ? Object.entries(snap.wan ?? {})
        .filter(([iface]) => !iface.includes('.'))
        .sort(([a], [b]) => a.localeCompare(b))
    : []

  // Prefer dashboard provenance; fall back to metrics/latest (old servers omit both).
  const dataSource = dashboard?.source ?? latest?.source
  const dataStale = dashboard?.stale ?? latest?.stale

  const meta = (
    <p className="text-xs text-muted-foreground">
      {snap ? fmtTime(snap.ts) : dashboard?.ts ? fmtTime(dashboard.ts) : '—'}
      {persist.enabled ? ` · ${persist.retention_days ?? 90}d` : ' · memory'}
    </p>
  )

  const xfer = dashboard?.transfer_24h
  const blocked = dashboard?.blocked
  const blockedAll = (blocked?.blocked ?? 0) + (blocked?.allowed ?? 0)
  const monthly = (dashboard?.monthly_wans ?? []).filter((w) => isISPLabel(w.name))
  const monthMax = Math.max(1, ...monthly.map((w) => w.upload + w.download))
  const speed = dashboard?.speedtest ?? []
  const dns = dashboard?.dns
  const dnsHours = completeHourPoints(dns?.queries ?? [], dashboard?.ts || Math.floor(Date.now() / 1000))
  const dnsLookups = dnsHours.reduce((n, q) => n + q.count, 0)

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {meta}
        <div className="flex flex-wrap items-center gap-2">
          <StatusGroup label="Firewalla">
            <StatusChip tone={agentOnline ? 'ok' : 'bad'}>
              {agentOnline ? 'Online' : 'Offline'}
            </StatusChip>
          </StatusGroup>
          {(dataSource === 'agent' || dataSource === 'fw-app-init') ? (
            <StatusGroup label="Source">
              <SourceBadge source={dataSource} stale={dataStale} />
            </StatusGroup>
          ) : null}
          {cpuPct != null ? (
            <StatusGroup label="CPU">
              <StatusChip tone={cpuTone(cpuPct)}>{cpuPct.toFixed(0)}%</StatusChip>
            </StatusGroup>
          ) : null}
          {(snap?.dns_svcs?.length ?? 0) > 0 ? (
            <StatusGroup label="DNS">
              {(snap?.dns_svcs ?? []).map((s) => (
                <StatusChip key={s.name} tone={s.ok ? 'ok' : 'bad'}>
                  {dnsSvcLabel(s.name)} {s.ok ? (s.since ? fmtRelative(s.since, Date.now()) : 'up') : 'down'}
                </StatusChip>
              ))}
            </StatusGroup>
          ) : null}
          {wans.length > 0 ? (
            <StatusGroup label="WAN">
              {wans.map(([iface, w]) => (
                <StatusChip key={iface} tone={w.ready ? 'ok' : 'bad'}>
                  {w.name || iface}
                </StatusChip>
              ))}
            </StatusGroup>
          ) : null}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard label="Devices" value={dashboard?.devices ?? deviceCount} onClick={onOpenDevices} />
        <StatCard label="Alarms" value={dashboard?.alarm_count ?? 0} />
        <StatCard label="Rules" value={dashboard?.rules ?? ruleCount} onClick={onOpenRules} />
      </div>

      <div className="grid gap-3 lg:grid-cols-[minmax(0,7fr)_minmax(22rem,5fr)] lg:items-stretch">
        <Card className="min-h-0 gap-3 py-4">
          <CardHeader className="shrink-0 px-5">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <CardTitle className="text-sm">Network data transferred</CardTitle>
                <CardDescription>Last 24 hours</CardDescription>
              </div>
              <div className="flex gap-8 text-right">
                <div>
                  <p className="font-mono text-2xl tabular-nums" style={{ color: UP }}>
                    {fmtBytes(xfer?.upload)}
                  </p>
                  <p className="text-[11px] text-muted-foreground">Upload</p>
                </div>
                <div>
                  <p className="font-mono text-2xl tabular-nums" style={{ color: DOWN }}>
                    {fmtBytes(xfer?.download)}
                  </p>
                  <p className="text-[11px] text-muted-foreground">Download</p>
                </div>
                <div>
                  <p className="font-mono text-2xl tabular-nums">
                    {fmtBytes((xfer?.upload ?? 0) + (xfer?.download ?? 0))}
                  </p>
                  <p className="text-[11px] text-muted-foreground">Total</p>
                </div>
              </div>
            </div>
          </CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col px-5">
            <TransferChart points={xfer?.points ?? []} />
          </CardContent>
        </Card>

        <div className="grid gap-3 lg:grid-rows-2">
          <Card className="gap-2 py-4">
            <CardHeader className="px-5">
              <CardTitle className="text-sm">Monthly data usage</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 px-5">
              {monthly.length === 0 ? (
                <p className="text-sm text-muted-foreground">Waiting on catalog</p>
              ) : (
                monthly.map((w) => {
                  const used = w.upload + w.download
                  return (
                    <div key={w.uuid} className="space-y-1">
                      <div className="flex items-baseline justify-between text-sm">
                        <span>{w.name}</span>
                        <span className="font-mono tabular-nums">
                          {fmtBytes(used)} <span className="text-muted-foreground">Used</span>
                        </span>
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                        <div
                          className="h-full rounded-full bg-chart-2"
                          style={{ width: `${Math.max((used / monthMax) * 100, used > 0 ? 2 : 0)}%` }}
                        />
                      </div>
                    </div>
                  )
                })
              )}
            </CardContent>
          </Card>
          <Card className="gap-2 py-4">
            <CardHeader className="px-5">
              <CardTitle className="text-sm">Blocked flows</CardTitle>
              <CardDescription>Last 24 hours</CardDescription>
            </CardHeader>
            <CardContent className="flex items-center gap-4 px-5">
              <Donut value={blocked?.blocked ?? 0} total={blockedAll} size={96} />
              <div className="space-y-1 text-sm">
                <p className="flex items-center gap-2">
                  <span className="size-1.5 rounded-full bg-destructive" />
                  <span className="font-mono tabular-nums">{fmtCount(blocked?.blocked)}</span>
                  <span className="text-muted-foreground">blocked</span>
                </p>
                <p className="flex items-center gap-2">
                  <span className="size-1.5 rounded-full bg-muted-foreground" />
                  <span className="font-mono tabular-nums">{fmtCount(blockedAll)}</span>
                  <span className="text-muted-foreground">all</span>
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <div className="grid gap-3 lg:grid-cols-2 lg:items-stretch">
        <button
          ref={speedTileRef}
          type="button"
          className={cn(
            'text-left',
            detail === 'speed' && !speedClosing && 'invisible pointer-events-none',
          )}
          onClick={openSpeed}
        >
          <Card className="h-full gap-2 py-4 transition-colors hover:bg-accent/40">
            <CardHeader className="px-5">
              <CardTitle className="text-sm">Speedtest</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5 px-5">
              {speed.length === 0 ? (
                <p className="text-sm text-muted-foreground">Waiting on catalog</p>
              ) : (
                speed.map((w, i) => (
                  <div key={w.uuid}>
                    {i > 0 ? <div className="mb-5 border-t border-border/80" /> : null}
                    <IspRow wan={w} />
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </button>

        <Card className="flex h-full min-h-0 flex-col gap-2 py-4">
          <CardHeader className="shrink-0 px-5">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <CardTitle className="text-sm">DNS</CardTitle>
                <CardDescription>Last 24 hours</CardDescription>
              </div>
              <div className="flex gap-8 text-right">
                <div>
                  <p
                    className={cn(
                      'font-mono text-2xl tabular-nums',
                      latest?.unbound_hit?.life && 'text-muted-foreground',
                    )}
                  >
                    {latest?.unbound_hit != null
                      ? `${latest.unbound_hit.pct.toFixed(0)}%`
                      : '—'}
                  </p>
                  <p className="text-[11px] text-muted-foreground">
                    {latest?.unbound_hit?.life ? 'Hit · life' : 'Hit'}
                  </p>
                </div>
                <div>
                  <p className="font-mono text-2xl tabular-nums">{fmtCount(dnsLookups)}</p>
                  <p className="text-[11px] text-muted-foreground">Lookups</p>
                </div>
              </div>
            </div>
          </CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col px-5">
            <DnsChart points={dnsHours} color={DOWN} className="h-full min-h-[140px] w-full" />
          </CardContent>
        </Card>
      </div>

      {detail === 'speed' ? (
        <SpeedtestDetail
          wans={speed}
          dashboard={dashboard}
          from={heroFrom}
          onCloseBegin={beginCloseSpeed}
          onClose={closeSpeed}
          onDashboard={onDashboard}
        />
      ) : null}

      <Card className="gap-3 py-4">
        <CardHeader className="px-4">
          <CardTitle className="text-sm">Top regions by data transfer</CardTitle>
          <CardDescription>Last 24 hours</CardDescription>
        </CardHeader>
        <CardContent className="px-4">
          {!dashboard?.top_regions?.length ? (
            <p className="py-6 text-sm text-muted-foreground">Waiting on catalog</p>
          ) : (
            <div className="grid gap-4 lg:grid-cols-2 lg:items-center">
              <div className="flex justify-center">
                <div className="w-full max-w-md">
                  <RegionMap
                    regions={dashboard.top_regions}
                    onSelectRegion={onSelectRegion}
                  />
                </div>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="h-8">Region</TableHead>
                    <TableHead className="h-8 text-right">Upload</TableHead>
                    <TableHead className="h-8 text-right">Download</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {dashboard.top_regions.map((r) => {
                    const cc = (r.country || r.id || '').toUpperCase()
                    const clickable = !!cc && !!onSelectRegion
                    return (
                      <TableRow
                        key={r.id || r.name}
                        className={clickable ? 'cursor-pointer hover:bg-accent/40' : undefined}
                        onClick={
                          clickable ? () => onSelectRegion?.(cc, r.name) : undefined
                        }
                      >
                        <TableCell className="py-2 font-medium">
                          <span className="inline-flex items-center gap-2">
                            <Flag cc={r.country || r.id} />
                            {r.name}
                          </span>
                        </TableCell>
                        <TableCell className="py-2 text-right font-mono tabular-nums text-chart-5">
                          {fmtBytes(r.upload)}
                        </TableCell>
                        <TableCell className="py-2 text-right font-mono tabular-nums text-chart-2">
                          {fmtBytes(r.download)}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-3 lg:grid-cols-2">
        <RankTable
          title="Top devices by upload"
          rows={dashboard?.top_upload}
          kind="device"
          metric="upload"
          onSelectDevice={onSelectDevice}
        />
        <RankTable
          title="Top devices by download"
          rows={dashboard?.top_download}
          kind="device"
          metric="download"
          onSelectDevice={onSelectDevice}
        />
        <RankTable
          title="Top destinations by upload"
          rows={dashboard?.top_dest_upload}
          kind="dest"
          metric="upload"
        />
        <RankTable
          title="Top destinations by download"
          rows={dashboard?.top_dest_download}
          kind="dest"
          metric="download"
        />
      </div>
    </div>
  )
}

function IspRow({ wan }: { wan: SpeedtestWAN }) {
  const plan = fmtPlan(wan.plan_down, wan.plan_up)
  const trend = speedTrend(wan.points ?? [])
  const server = [wan.server, wan.location].filter(Boolean).join(' · ')
  const hint = [wan.active ? 'active' : 'standby', plan || null, server || null].filter(Boolean).join(' · ')
  return (
    <div className="grid grid-cols-[8.5rem_minmax(0,1fr)] items-center gap-3">
      <div className="min-w-0 space-y-0.5">
        <p className="font-mono text-xl tabular-nums tracking-tight" style={{ color: DOWN }}>
          {fmtMbps(wan.down)}{' '}
          <span className="font-sans text-[11px] text-muted-foreground">down</span>
        </p>
        <p className="font-mono text-xl tabular-nums tracking-tight" style={{ color: UP }}>
          {fmtMbps(wan.up)}{' '}
          <span className="font-sans text-[11px] text-muted-foreground">up</span>
        </p>
        <p className="text-sm">
          {wan.name}{' '}
          <span className="text-muted-foreground">{trend === 1 ? '↗' : trend === -1 ? '↘' : '→'}</span>
        </p>
        <p className="text-[11px] text-muted-foreground">{hint}</p>
      </div>
      <SpeedSpark
        points={wan.points ?? []}
        planDown={wan.plan_down}
        planUp={wan.plan_up}
        className="h-[120px] w-full"
      />
    </div>
  )
}

type HeroRect = { left: number; top: number; width: number; height: number }

const HERO_OPEN_MS = 280
const HERO_CLOSE_MS = 160
/** Snappy decelerate — less floaty than springy ease-out. */
const HERO_EASE = 'cubic-bezier(0.2, 0, 0, 1)'

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function SpeedtestDetail({
  wans,
  dashboard,
  from,
  onCloseBegin,
  onClose,
  onDashboard,
}: {
  wans: SpeedtestWAN[]
  dashboard: Dashboard | null
  from: HeroRect | null
  onCloseBegin: () => void
  onClose: () => void
  onDashboard?: (dash: Dashboard) => void
}) {
  const backdropRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const closingRef = useRef(false)
  const rows = wans
    .flatMap((w) => (w.points ?? []).map((p) => ({ ...p, name: w.name })))
    .sort((a, b) => b.ts - a.ts)

  const [canRun, setCanRun] = useState(false)
  const [picking, setPicking] = useState(false)
  const [phase, setPhase] = useState<'idle' | 'running'>('idle')
  const [runError, setRunError] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [pickWan, setPickWan] = useState<string | null>(null)
  const [serverId, setServerId] = useState('auto')
  const [servers, setServers] = useState<{ id: string; name: string; sponsor: string; distance?: number }[]>([])
  const [serversErr, setServersErr] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const [stR, modR] = await Promise.all([api('/v1/fw-app/status'), api('/v1/modules')])
        if (cancelled) return
        let lanOk = false
        if (stR.ok) {
          const st = (await stR.json()) as { state?: string; paired?: boolean }
          lanOk = st.state === 'lan-ok'
        }
        let enabled = false
        if (modR.ok) {
          const body = (await modR.json()) as { modules?: { id: string; enabled?: boolean }[] }
          enabled = !!body.modules?.find((m) => m.id === 'fw-app')?.enabled
        }
        const ok = enabled && lanOk && wans.length > 0
        setCanRun(ok)
        if (ok) {
          try {
            await api('/v1/fw-app/speedtest/sync', { method: 'POST' })
            const dashR = await api('/v1/dashboard')
            if (!cancelled && dashR.ok) {
              onDashboard?.((await dashR.json()) as Dashboard)
            }
          } catch {
            /* keep current dashboard */
          }
        }
      } catch {
        if (!cancelled) setCanRun(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [wans.length, onDashboard])

  const loadServers = async () => {
    setServersErr(null)
    try {
      const r = await api('/v1/fw-app/speedtest/servers?limit=20')
      const body = (await r.json()) as {
        error?: string
        servers?: { id: string; name: string; sponsor: string; distance?: number }[]
      }
      if (!r.ok) throw new Error(body.error || `servers ${r.status}`)
      setServers(body.servers ?? [])
    } catch (e) {
      setServers([])
      setServersErr(e instanceof Error ? e.message : 'servers failed')
    }
  }

  const dismissToast = useCallback(() => setToast(null), [])

  const applyOptimistic = (
    wanUUID: string,
    result: {
      down?: number
      up?: number
      ping?: number
      ts?: number
      server_id?: string
      server?: string
      location?: string
    },
  ) => {
    if (!onDashboard) return
    const rawTs = result.ts || Math.floor(Date.now() / 1000)
    const ts = rawTs > 1e12 ? Math.floor(rawTs / 1000) : rawTs
    const down = result.down ?? 0
    const up = result.up ?? 0
    const ping = result.ping
    const point = {
      ts,
      down,
      up,
      ...(ping ? { ping } : {}),
      ...(result.server_id ? { server_id: result.server_id } : {}),
      ...(result.server ? { server: result.server } : {}),
      ...(result.location ? { location: result.location } : {}),
    }
    const base = dashboard?.speedtest?.length ? dashboard.speedtest : wans
    let hit = false
    const speedtest = base.map((w) => {
      if (w.uuid !== wanUUID) return w
      hit = true
      const pts = [...(w.points ?? [])]
      if (!pts.some((p) => p.ts === ts)) pts.push(point)
      return {
        ...w,
        down,
        up,
        ping,
        server_id: result.server_id ?? w.server_id,
        server: result.server ?? w.server,
        location: result.location ?? w.location,
        points: pts,
      }
    })
    if (!hit) {
      speedtest.push({
        uuid: wanUUID,
        name: wanUUID,
        down,
        up,
        ping,
        server_id: result.server_id,
        server: result.server,
        location: result.location,
        points: [point],
      })
    }
    onDashboard({ ...(dashboard ?? ({} as Dashboard)), speedtest })
  }

  const startRun = async (wanUUID: string, sid: string) => {
    setPicking(false)
    setRunError(null)
    setPhase('running')
    const picked = sid ? servers.find((s) => s.id === sid) : undefined
    try {
      const r = await api('/v1/fw-app/speedtest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          wan_uuid: wanUUID,
          ...(sid ? { server_id: sid } : {}),
        }),
      })
      const body = (await r.json()) as {
        error?: string
        job?: { id?: string; state?: string }
      }
      if (!r.ok) throw new Error(body.error || `speedtest ${r.status}`)
      const jobId = body.job?.id
      if (!jobId) throw new Error('no job id')

      const deadline = Date.now() + 180_000
      let result:
        | {
            down?: number
            up?: number
            ping?: number
            ts?: number
            server_id?: string
            server?: string
            location?: string
          }
        | undefined
      while (Date.now() < deadline) {
        await new Promise((res) => setTimeout(res, 1500))
        const jr = await api(`/v1/fw-app/speedtest/${jobId}`)
        if (!jr.ok) throw new Error(`job ${jr.status}`)
        const jb = (await jr.json()) as {
          job?: {
            state?: string
            error?: string
            result?: {
              down?: number
              up?: number
              ping?: number
              ts?: number
              server_id?: string
              server?: string
              location?: string
            }
          }
        }
        const st = jb.job?.state
        if (st === 'error') throw new Error(jb.job?.error || 'speedtest failed')
        if (st === 'done') {
          result = jb.job?.result
          break
        }
      }
      if (!result) throw new Error('speedtest timed out')
      if (!result.server_id && sid) result.server_id = sid
      if (!result.server && picked) result.server = picked.sponsor
      if (!result.location && picked) result.location = picked.name
      const down = result.down ?? 0
      const up = result.up ?? 0
      const ping = result.ping
      const bits = [`↓ ${fmtMbps(down)}`, `↑ ${fmtMbps(up)}`]
      if (ping) bits.push(`${Math.round(ping)} ms`)
      const serverLabel = [result.server, result.location].filter(Boolean).join(' · ')
      if (serverLabel) bits.push(serverLabel)
      applyOptimistic(wanUUID, result)
      setToast(bits.join(' · '))
      setPhase('idle')
      void (async () => {
        try {
          const dashR = await api('/v1/dashboard')
          if (!dashR.ok) return
          const dash = (await dashR.json()) as Dashboard
          const rawTs = result.ts || Math.floor(Date.now() / 1000)
          const ts = rawTs > 1e12 ? Math.floor(rawTs / 1000) : rawTs
          const w = dash.speedtest?.find((x) => x.uuid === wanUUID)
          const has = (w?.points ?? []).some((p) => p.ts === ts || p.ts >= ts - 2)
          if (has) onDashboard?.(dash)
        } catch {
          /* keep optimistic */
        }
      })()
    } catch (e) {
      setRunError(e instanceof Error ? e.message : 'speedtest failed')
      setPhase('idle')
    }
  }

  const onRunClick = () => {
    const preferred = wans.find((w) => w.active)?.uuid ?? wans[0]?.uuid ?? null
    setPickWan(preferred)
    setServerId('auto')
    setPicking(true)
    void loadServers()
  }

  const onConfirmRun = () => {
    const wan = pickWan ?? wans.find((w) => w.active)?.uuid ?? wans[0]?.uuid
    if (!wan) {
      setRunError('Pick ISP')
      return
    }
    void startRun(wan, serverId === 'auto' ? '' : serverId)
  }

  useLayoutEffect(() => {
    const backdrop = backdropRef.current
    const panel = panelRef.current
    if (!backdrop || !panel) return

    if (prefersReducedMotion() || !from || from.width < 8 || from.height < 8) {
      backdrop.style.opacity = '1'
      return
    }

    const to = panel.getBoundingClientRect()
    const sx = from.width / Math.max(to.width, 1)
    const sy = from.height / Math.max(to.height, 1)
    const dx = from.left - to.left
    const dy = from.top - to.top
    panel.style.transformOrigin = 'top left'
    backdrop.style.opacity = '0'

    const bg = backdrop.animate([{ opacity: 0 }, { opacity: 1 }], {
      duration: HERO_OPEN_MS,
      easing: HERO_EASE,
      fill: 'forwards',
    })
    panel.animate(
      [
        { transform: `translate(${dx}px, ${dy}px) scale(${sx}, ${sy})` },
        { transform: 'none' },
      ],
      { duration: HERO_OPEN_MS, easing: HERO_EASE, fill: 'forwards' },
    )
    void bg.finished.then(() => {
      backdrop.style.opacity = '1'
      bg.cancel()
    })
  }, [from])

  const requestClose = () => {
    if (closingRef.current || phase === 'running') return
    closingRef.current = true
    onCloseBegin()
    const backdrop = backdropRef.current
    const panel = panelRef.current
    if (!backdrop || !panel || prefersReducedMotion()) {
      onClose()
      return
    }
    const bg = backdrop.animate([{ opacity: Number(backdrop.style.opacity) || 1 }, { opacity: 0 }], {
      duration: HERO_CLOSE_MS,
      easing: HERO_EASE,
      fill: 'forwards',
    })
    const anim = panel.animate(
      [
        { opacity: 1, transform: 'scale(1)' },
        { opacity: 0, transform: 'scale(0.98)' },
      ],
      { duration: HERO_CLOSE_MS, easing: HERO_EASE, fill: 'forwards' },
    )
    void Promise.all([anim.finished, bg.finished]).then(
      () => onClose(),
      () => onClose(),
    )
  }

  const busy = phase === 'running'

  return (
    <div
      ref={backdropRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={requestClose}
    >
      <div
        ref={panelRef}
        className="flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-lg border bg-popover shadow-md will-change-transform"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex shrink-0 items-center justify-between gap-3 border-b px-6 py-4">
          <span className="text-base font-medium">Speedtest</span>
          {canRun ? (
            <Button
              type="button"
              size="sm"
              disabled={busy}
              onClick={onRunClick}
            >
              {phase === 'running' ? 'Running…' : 'Run'}
            </Button>
          ) : null}
        </div>
        {busy ? (
          <div className="h-1 w-full overflow-hidden bg-muted">
            <div className="h-full w-1/3 animate-[speedIndeterminate_1.2s_ease-in-out_infinite] bg-foreground/70" />
          </div>
        ) : null}
        {picking ? (
          <div className="shrink-0 space-y-3 border-b px-6 py-4">
            {wans.length > 1 ? (
              <div className="space-y-2">
                <p className="text-sm text-muted-foreground">ISP</p>
                <div className="flex flex-wrap gap-2">
                  {wans.map((w) => (
                    <Button
                      key={w.uuid}
                      type="button"
                      size="sm"
                      variant={pickWan === w.uuid ? 'default' : 'outline'}
                      disabled={busy}
                      onClick={() => setPickWan(w.uuid)}
                    >
                      {w.name}
                    </Button>
                  ))}
                </div>
              </div>
            ) : null}
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Server</p>
              <Select
                value={serverId || 'auto'}
                onValueChange={setServerId}
                disabled={busy}
              >
                <SelectTrigger className="w-full max-w-md">
                  <SelectValue placeholder="Auto" />
                </SelectTrigger>
                <SelectContent position="popper" className="z-[70]">
                  <SelectItem value="auto">Auto</SelectItem>
                  {servers.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.sponsor}
                      {s.name ? ` · ${s.name}` : ''}
                      {s.distance != null ? ` · ${Math.round(s.distance)} km` : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {serversErr ? <p className="text-sm text-destructive">{serversErr}</p> : null}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button type="button" size="sm" disabled={busy} onClick={onConfirmRun}>
                Start
              </Button>
              <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => setPicking(false)}>
                Cancel
              </Button>
            </div>
          </div>
        ) : null}
        {runError ? <p className="px-6 pt-3 text-sm text-destructive">{runError}</p> : null}
        <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-5">
          {wans.map((w) => {
            const trend = speedTrend(w.points ?? [])
            return (
              <div key={w.uuid} className="space-y-2">
                <div className="flex items-baseline justify-between gap-2 text-sm">
                  <span>
                    {w.name}{' '}
                    <span className="text-muted-foreground">
                      {trend === 1 ? '↗' : trend === -1 ? '↘' : '→'}
                    </span>
                    {fmtPlan(w.plan_down, w.plan_up) ? (
                      <span className="text-muted-foreground"> · {fmtPlan(w.plan_down, w.plan_up)}</span>
                    ) : null}
                    {[w.server, w.location].filter(Boolean).length ? (
                      <span className="text-muted-foreground">
                        {' '}
                        · {[w.server, w.location].filter(Boolean).join(' · ')}
                      </span>
                    ) : null}
                  </span>
                  <span className="font-mono tabular-nums text-muted-foreground">
                    {w.ping ? `${Math.round(w.ping)} ms` : ''}
                  </span>
                </div>
                <SpeedSpark
                  points={w.points ?? []}
                  planDown={w.plan_down}
                  planUp={w.plan_up}
                  className="h-[220px] w-full"
                />
              </div>
            )
          })}
          {rows.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>When</TableHead>
                  <TableHead>WAN</TableHead>
                  <TableHead>Server</TableHead>
                  <TableHead className="text-right">Down</TableHead>
                  <TableHead className="text-right">Up</TableHead>
                  <TableHead className="text-right">Ping</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((r) => (
                  <TableRow key={`${r.name}-${r.ts}`}>
                    <TableCell className="text-muted-foreground">{fmtTime(r.ts)}</TableCell>
                    <TableCell>{r.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {[r.server, r.location].filter(Boolean).join(' · ') || '—'}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums" style={{ color: DOWN }}>
                      {fmtMbps(r.down)}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums" style={{ color: UP }}>
                      {fmtMbps(r.up)}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums text-muted-foreground">
                      {r.ping ? `${Math.round(r.ping)} ms` : '—'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : null}
        </div>
        <div className="flex shrink-0 justify-end border-t px-6 py-3">
          <button
            type="button"
            className="text-sm font-medium disabled:opacity-50"
            disabled={phase === 'running'}
            onClick={requestClose}
          >
            Close
          </button>
        </div>
      </div>
      {toast
        ? createPortal(<Toast message={toast} onDismiss={dismissToast} />, document.body)
        : null}
      <style>{`
@keyframes speedIndeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(400%); }
}
`}</style>
    </div>
  )
}

function StatCard({
  label,
  value,
  onClick,
}: {
  label: string
  value: number
  onClick?: () => void
}) {
  const inner = (
    <Card className={cn('h-full gap-1 px-5 py-4', onClick && 'transition-colors hover:bg-accent/40')}>
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-3xl tabular-nums tracking-tight">{fmtCount(value)}</p>
    </Card>
  )
  if (!onClick) return inner
  return (
    <button type="button" className="text-left" onClick={onClick}>
      {inner}
    </button>
  )
}

function RankTable({
  title,
  rows,
  kind,
  metric,
  onSelectDevice,
}: {
  title: string
  rows?: RankedFlow[]
  kind: 'device' | 'dest'
  metric: 'upload' | 'download'
  onSelectDevice?: (mac: string, label: string) => void
}) {
  const singleMetric = kind === 'dest'
  const metricLabel = metric === 'upload' ? 'Upload' : 'Download'
  return (
    <Card className="gap-0 py-0">
      <CardHeader className="flex-row items-baseline justify-between space-y-0 border-b py-3">
        <CardTitle className="text-sm">{title}</CardTitle>
        <CardDescription>Last 24 hours</CardDescription>
      </CardHeader>
      <CardContent className="px-0">
        {!rows?.length ? (
          <p className="px-6 py-6 text-sm text-muted-foreground">Waiting on catalog</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{kind === 'dest' ? 'Destination' : 'Device'}</TableHead>
                {kind === 'dest' ? <TableHead>Region</TableHead> : null}
                {singleMetric ? (
                  <TableHead className="text-right">{metricLabel}</TableHead>
                ) : (
                  <>
                    <TableHead className="text-right">Upload</TableHead>
                    <TableHead className="text-right">Download</TableHead>
                  </>
                )}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => {
                const clickable = kind === 'device' && !!r.id && !!onSelectDevice
                return (
                  <TableRow
                    key={r.id || r.name}
                    className={clickable ? 'cursor-pointer hover:bg-accent/40' : undefined}
                    onClick={
                      clickable
                        ? () => onSelectDevice?.(r.id!.toUpperCase(), r.name)
                        : undefined
                    }
                  >
                    <TableCell className={kind === 'dest' ? 'font-mono' : 'font-medium'}>
                      <span className="inline-flex items-center gap-2">
                        {kind === 'dest' ? (
                          <Flag cc={r.country} />
                        ) : (
                          <DeviceIcon type={r.type} className="size-4" />
                        )}
                        {r.name}
                      </span>
                    </TableCell>
                    {kind === 'dest' ? (
                      <TableCell className="text-muted-foreground">{r.region || '—'}</TableCell>
                    ) : null}
                    {singleMetric ? (
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtBytes(metric === 'upload' ? r.upload : r.download)}
                      </TableCell>
                    ) : (
                      <>
                        <TableCell className="text-right font-mono tabular-nums">
                          {fmtBytes(r.upload)}
                        </TableCell>
                        <TableCell className="text-right font-mono tabular-nums">
                          {fmtBytes(r.download)}
                        </TableCell>
                      </>
                    )}
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function StatusGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-1.5 rounded-md border border-border/80 bg-card/40 px-2 py-1">
      <span className="text-[10px] font-medium tracking-wide text-muted-foreground uppercase">{label}</span>
      <div className="flex flex-wrap items-center gap-1">{children}</div>
    </div>
  )
}

function StatusChip({ tone, children }: { tone: 'ok' | 'warn' | 'bad'; children: ReactNode }) {
  return (
    <Badge
      variant={tone === 'bad' ? 'destructive' : 'outline'}
      className={cn(
        tone === 'ok' && 'border-emerald-500/40 bg-emerald-500/15 text-emerald-400',
        tone === 'warn' && 'border-amber-500/40 bg-amber-500/15 text-amber-400',
      )}
    >
      {children}
    </Badge>
  )
}

function dnsSvcLabel(name: string): string {
  switch (name) {
    case 'unbound':
      return 'Unbound'
    case 'dnsmasq':
      return 'Dnsmasq'
    case 'firerouter':
      return 'Firerouter'
    default:
      return name
  }
}

function isISPLabel(name: string): boolean {
  const n = name.trim().toLowerCase()
  if (!n || n.includes('vpn')) return false
  if (/^(eth|enp|ens|enx|wlan|wlp|br|bond|vlan|wg|tun)\d*/.test(n)) return false
  if (/^wan\d?$/.test(n)) return false
  return true
}
