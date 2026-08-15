import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { ArrowLeft, ChevronDown, RefreshCw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { CopyButton } from '@/components/CopyText'
import {
  anonymizeEnrollCmd,
  anonymizeModules,
  anonymizeNameSync,
  createAnon,
} from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, setAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import { api } from '@/lib/api'
import { fmtTime } from '@/lib/format'
import type { ModuleInfo, NameSyncRow, NameSyncView } from '@/lib/types'
import { cn } from '@/lib/utils'
import { IdentitySettings } from '@/tabs/IdentitySettings'
import { FWAppSettingsPage } from '@/tabs/FWAppSettings'
import { TPLinkSettingsPage } from '@/tabs/TPLinkSettings'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const TABLE = 'grid w-full gap-x-3 divide-y'
const TABLE_HEAD =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2 text-xs text-muted-foreground'
const TABLE_ROW = 'col-span-full grid grid-cols-subgrid items-center px-6 py-2.5 text-sm'
const ROW_COLS = 'grid-cols-[minmax(0,1fr)_minmax(0,1fr)_7rem_9rem_4.5rem_8.5rem]'

export function SettingsTab({
  openModule = null,
  onOpenModule,
  onModulesChange,
}: {
  openModule?: string | null
  onOpenModule?: (id: string | null) => void
  onModulesChange?: (modules: ModuleInfo[]) => void
} = {}) {
  const [mods, setMods] = useState<ModuleInfo[] | null>(null)
  const [agentOnline, setAgentOnline] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [openLocal, setOpenLocal] = useState<string | null>(null)
  const controlled = onOpenModule != null
  const open = controlled ? (openModule ?? null) : openLocal
  const setOpen = (id: string | null) => {
    if (controlled) onOpenModule(id)
    else setOpenLocal(id)
  }
  const [rows, setRows] = useState<NameSyncRow[] | null>(null)
  const [excluded, setExcluded] = useState<NameSyncRow[]>([])
  const [rowsHint, setRowsHint] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [anonOn, setAnonOn] = useState(isAnonymityOn)

  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])

  function replaceMods(next: ModuleInfo[]) {
    setMods(next)
    onModulesChange?.(next)
  }

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [modR, agentR] = await Promise.all([api('/v1/modules'), api('/v1/settings/agent')])
        if (!modR.ok) throw new Error(`modules ${modR.status}`)
        const body = (await modR.json()) as { modules?: ModuleInfo[] }
        if (!cancelled) replaceMods(body.modules ?? [])
        if (agentR.ok) {
          const agentBody = (await agentR.json()) as { agent?: { online?: boolean } }
          if (!cancelled) setAgentOnline(!!agentBody.agent?.online)
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'load failed')
      }
    }
    void load()
    const id = window.setInterval(load, 2000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- poll; publish via replaceMods
  }, [])

  const loadRows = useCallback(async (enabled: boolean, moduleOn: boolean) => {
    if (!enabled) {
      setRows(null)
      setExcluded([])
      setRowsHint(null)
      return
    }
    if (!moduleOn) {
      setRows([])
      setExcluded([])
      setRowsHint('Enable UniFi')
      return
    }
    try {
      const r = await api('/v1/unifi/name-sync')
      if (r.status === 404) {
        const text = (await r.text()).trim()
        setRows([])
        setExcluded([])
        setRowsHint(text.includes('devices') ? 'No devices yet' : 'Enable UniFi')
        return
      }
      if (r.status === 503) {
        setRows([])
        setExcluded([])
        setRowsHint((await r.text()).trim() || 'UniFi down')
        return
      }
      if (!r.ok) throw new Error(`name-sync ${r.status}`)
      const body = (await r.json()) as NameSyncView
      setRows(body.rows ?? [])
      setExcluded(body.excluded ?? [])
      setRowsHint(null)
    } catch (e) {
      setRows([])
      setExcluded([])
      setRowsHint(e instanceof Error ? e.message : 'load failed')
    }
  }, [])

  const unifi = mods?.find((m) => m.id === 'unifi-sync')
  const tplink = mods?.find((m) => m.id === 'tplink-sync')
  const fwApp = mods?.find((m) => m.id === 'fw-app')

  useEffect(() => {
    if (open !== 'unifi-sync' || !unifi) return
    void loadRows(!!unifi.name_sync_enabled, unifi.enabled)
  }, [open, unifi?.enabled, unifi?.name_sync_enabled, loadRows])

  if (open === 'tplink-sync' && tplink) {
    return (
      <TPLinkSettingsPage
        mod={tplink}
        onBack={() => setOpen(null)}
        onToggle={(on) => void toggle(tplink.id, on)}
      />
    )
  }

  if (open === 'fw-app' && fwApp) {
    return (
      <FWAppSettingsPage
        mod={fwApp}
        onBack={() => setOpen(null)}
        onToggle={(on) => void toggle(fwApp.id, on)}
      />
    )
  }

  async function toggle(id: string, enabled: boolean) {
    setError(null)
    const optimistic = mods?.map((m) => (m.id === id ? { ...m, enabled } : m)) ?? null
    if (optimistic) replaceMods(optimistic)
    try {
      const r = await api('/v1/modules', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id, enabled }),
      })
      if (!r.ok) throw new Error(`save ${r.status}`)
      const body = (await r.json()) as { modules?: ModuleInfo[] }
      replaceMods(body.modules ?? [])
    } catch (e) {
      const rolled = mods?.map((m) => (m.id === id ? { ...m, enabled: !enabled } : m)) ?? null
      if (rolled) replaceMods(rolled)
      else setMods((prev) => prev?.map((m) => (m.id === id ? { ...m, enabled: !enabled } : m)) ?? prev)
      setError(e instanceof Error ? e.message : 'save failed')
    }
  }

  async function setNameSync(patch: Record<string, unknown>) {
    setError(null)
    try {
      const r = await api('/v1/unifi/name-sync', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      })
      if (!r.ok) throw new Error(`name-sync ${r.status}`)
      const body = (await r.json()) as { name_sync_enabled?: boolean; name_sync_auto?: boolean }
      setMods(
        (prev) =>
          prev?.map((m) =>
            m.id === 'unifi-sync'
              ? {
                  ...m,
                  name_sync_enabled: !!body.name_sync_enabled,
                  name_sync_auto: !!body.name_sync_auto,
                }
              : m,
          ) ?? prev,
      )
      const mod = (await (await api('/v1/modules')).json()) as { modules?: ModuleInfo[] }
      replaceMods(mod.modules ?? [])
      const u = mod.modules?.find((m) => m.id === 'unifi-sync')
      if (u?.enabled && u.name_sync_enabled) {
        await loadRows(true, true)
      } else if (patch.name_sync_enabled === false) {
        setRows(null)
        setExcluded([])
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
      const r = await api('/v1/modules')
      if (r.ok) {
        const body = (await r.json()) as { modules?: ModuleInfo[] }
        replaceMods(body.modules ?? [])
      }
    }
  }

  async function apply(macs?: string[]) {
    if (!unifi?.enabled || !unifi.name_sync_enabled) return
    setBusy(true)
    setError(null)
    try {
      const r = await api('/v1/unifi/name-sync/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(macs ? { macs } : {}),
      })
      if (!r.ok) throw new Error((await r.text()).trim() || `apply ${r.status}`)
      await loadRows(true, true)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'apply failed')
    } finally {
      setBusy(false)
    }
  }

  async function excludeMac(mac: string) {
    setBusy(true)
    setError(null)
    try {
      await setNameSync({ exclude_add: [mac] })
    } finally {
      setBusy(false)
    }
  }

  async function includeMac(mac: string) {
    setBusy(true)
    setError(null)
    try {
      await setNameSync({ exclude_remove: [mac] })
    } finally {
      setBusy(false)
    }
  }

  if (open === 'firewalla') {
    return <FirewallaSettings onBack={() => setOpen(null)} />
  }

  if (open === 'identity') {
    return <IdentitySettings onBack={() => setOpen(null)} />
  }

  if (open === 'unifi-sync' && unifi) {
    const syncOn = !!unifi.name_sync_enabled
    const canApply = unifi.enabled && syncOn && unifi.status === 'ok' && !busy
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <button
            type="button"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            onClick={() => setOpen(null)}
          >
            <ArrowLeft className="size-4" />
            Back
          </button>
          <h1 className="text-lg font-semibold tracking-tight">{unifi.label}</h1>
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Card className="gap-0 py-0">
          <CardContent className="divide-y px-0">
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Enabled</div>
              <div className="flex items-center justify-end">
                <Switch checked={unifi.enabled} onCheckedChange={(on) => void toggle(unifi.id, on)} />
              </div>
            </div>
            <Meta label="Status" value={unifi.enabled ? (unifi.status === 'ok' ? 'Connected' : 'Down') : 'Off'} />
            {unifi.enabled && unifi.status === 'ok' ? <Meta label="Version" value={unifi.detail || '—'} /> : null}
            {unifi.enabled && unifi.status === 'ok' ? (
              <Meta
                label="Site"
                value={(anon ? anonymizeModules(anon, [unifi])[0]?.site : unifi.site) || '—'}
              />
            ) : null}
            {unifi.enabled && unifi.status === 'down' ? <Meta label="Error" value={unifi.detail || '—'} /> : null}
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Client name sync</div>
              <div className="flex items-center justify-end">
                <Switch
                  checked={syncOn}
                  onCheckedChange={(on) => void setNameSync({ name_sync_enabled: on })}
                />
              </div>
            </div>
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Auto-fill blanks</div>
              <div className="flex items-center justify-end">
                <Switch
                  checked={!!unifi.name_sync_auto}
                  disabled={!syncOn}
                  onCheckedChange={(on) => void setNameSync({ name_sync_auto: on })}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {syncOn ? (
          <>
            <Card className="gap-0 py-0">
              <CardHeader className={cn(SECTION_HEADER, 'flex flex-row items-center justify-between gap-3')}>
                <CardTitle className={SECTION_TITLE}>Review names</CardTitle>
                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={busy}
                    onClick={() => void loadRows(syncOn, unifi.enabled)}
                  >
                    Refresh
                  </Button>
                  <Button
                    type="button"
                    size="xs"
                    disabled={!canApply || !(rows && rows.length > 0)}
                    onClick={() => void apply()}
                  >
                    Overwrite all
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="px-0 pb-2">
                {rowsHint ? (
                  <p className="px-6 py-4 text-sm text-muted-foreground">{rowsHint}</p>
                ) : !rows || rows.length === 0 ? (
                  <p className="px-6 py-4 text-sm text-muted-foreground">No name changes</p>
                ) : (
                  <div className={cn(TABLE, ROW_COLS)}>
                    <div className={TABLE_HEAD}>
                      <div>Firewalla</div>
                      <div>UniFi</div>
                      <div>IP</div>
                      <div>MAC</div>
                      <div>Status</div>
                      <div />
                    </div>
                    {rows.map((row) => {
                      const d = anon ? (anonymizeNameSync(anon, [row])[0] ?? row) : row
                      return (
                      <div key={row.mac} className={TABLE_ROW}>
                        <div className="min-w-0 truncate font-medium">{d.firewalla_name}</div>
                        <div className="min-w-0 truncate text-muted-foreground">{d.unifi_name || '—'}</div>
                        <div className="min-w-0 truncate text-muted-foreground">{d.ip || '—'}</div>
                        <div className="min-w-0 truncate font-mono text-xs text-muted-foreground">{d.mac}</div>
                        <div className={cn('text-xs', row.status === 'conflict' ? '' : 'text-muted-foreground')}>
                          {row.status}
                        </div>
                        <div className="flex justify-end gap-1">
                          <Button
                            type="button"
                            size="xs"
                            variant="ghost"
                            disabled={busy || !unifi.enabled}
                            onClick={() => void excludeMac(row.mac)}
                          >
                            Exclude
                          </Button>
                          <Button
                            type="button"
                            size="xs"
                            variant="outline"
                            disabled={!canApply}
                            onClick={() => void apply([row.mac])}
                          >
                            Apply
                          </Button>
                        </div>
                      </div>
                      )
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            {excluded.length > 0 ? (
              <Card className="gap-0 py-0 opacity-60">
                <CardHeader className={SECTION_HEADER}>
                  <CardTitle className={SECTION_TITLE}>Excluded</CardTitle>
                </CardHeader>
                <CardContent className="px-0 pb-2">
                  <div className={cn(TABLE, ROW_COLS)}>
                    <div className={TABLE_HEAD}>
                      <div>Firewalla</div>
                      <div>UniFi</div>
                      <div>IP</div>
                      <div>MAC</div>
                      <div>Status</div>
                      <div />
                    </div>
                    {excluded.map((row) => {
                      const d = anon ? (anonymizeNameSync(anon, [row])[0] ?? row) : row
                      return (
                      <div key={row.mac} className={TABLE_ROW}>
                        <div className="min-w-0 truncate">{d.firewalla_name}</div>
                        <div className="min-w-0 truncate">{d.unifi_name || '—'}</div>
                        <div className="min-w-0 truncate">{d.ip || '—'}</div>
                        <div className="min-w-0 truncate font-mono text-xs">{d.mac}</div>
                        <div className="text-xs">{row.status}</div>
                        <div className="flex justify-end">
                          <Button
                            type="button"
                            size="xs"
                            variant="outline"
                            disabled={busy || !unifi.enabled}
                            onClick={() => void includeMac(row.mac)}
                          >
                            Include
                          </Button>
                        </div>
                      </div>
                      )
                    })}
                  </div>
                </CardContent>
              </Card>
            ) : null}
          </>
        ) : null}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-lg font-semibold tracking-tight">Settings</h1>
      {error && <p className="text-sm text-destructive">{error}</p>}

      <SettingsSection title="Firewalla">
        <Card
          className="cursor-pointer gap-0 py-0 hover:bg-accent/30"
          onClick={() => setOpen('firewalla')}
        >
          <div className="flex items-center gap-2 px-6 py-5 text-base font-medium">
            <span className="truncate">Agent</span>
            <span
              className={cn(
                'size-2 shrink-0 rounded-full',
                agentOnline ? 'bg-emerald-500' : 'bg-red-500',
              )}
            />
          </div>
        </Card>
        {fwApp ? (
          <ModuleSettingsCard
            mod={fwApp}
            label="Control"
            onOpen={() => setOpen(fwApp.id)}
            onToggle={(on) => void toggle(fwApp.id, on)}
          />
        ) : null}
      </SettingsSection>

      <SettingsSection title="Integrations">
        {(mods ?? [])
          .filter((m) => m.id !== 'fw-app')
          .map((m) => (
            <ModuleSettingsCard
              key={m.id}
              mod={m}
              onOpen={
                m.id === 'unifi-sync' || m.id === 'tplink-sync'
                  ? () => setOpen(m.id)
                  : undefined
              }
              onToggle={(on) => void toggle(m.id, on)}
            />
          ))}
        {(mods ?? []).filter((m) => m.id !== 'fw-app').length === 0 ? (
          <p className="text-sm text-muted-foreground sm:col-span-2">No modules</p>
        ) : null}
      </SettingsSection>

      <SettingsSection title="Access">
        <Card
          className="cursor-pointer gap-0 py-0 hover:bg-accent/30"
          onClick={() => setOpen('identity')}
        >
          <div className="flex items-center gap-2 px-6 py-5 text-base font-medium">
            <span className="truncate">Identity & Access</span>
          </div>
        </Card>
      </SettingsSection>

      <SettingsSection title="General">
        <div className="sm:col-span-2">
          <GeneralSettingsCard />
        </div>
      </SettingsSection>
    </div>
  )
}

function SettingsSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{title}</h2>
      <div className="grid gap-3 sm:grid-cols-2">{children}</div>
    </section>
  )
}

function ModuleSettingsCard({
  mod,
  label,
  onOpen,
  onToggle,
}: {
  mod: ModuleInfo
  label?: string
  onOpen?: () => void
  onToggle: (on: boolean) => void
}) {
  const locked = mod.id === 'ha-mqtt'
  const clickable = !!onOpen && !locked
  return (
    <Card
      className={cn(
        'gap-0 py-0',
        clickable && 'cursor-pointer hover:bg-accent/30',
        locked && 'opacity-60',
      )}
      onClick={clickable ? onOpen : undefined}
    >
      <div className="flex items-center justify-between gap-3 px-6 py-5">
        <div className="inline-flex min-w-0 items-center gap-2 text-base font-medium">
          <span className="truncate">{label ?? mod.label}</span>
          {locked ? (
            <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-normal uppercase tracking-wide text-muted-foreground">
              Coming soon
            </span>
          ) : null}
          {!locked && mod.enabled ? (
            <span
              className={cn(
                'size-2 shrink-0 rounded-full',
                mod.status === 'ok'
                  ? 'bg-emerald-500'
                  : mod.status === 'ready'
                    ? 'bg-amber-400'
                    : 'bg-red-500',
              )}
            />
          ) : null}
        </div>
        <Switch
          checked={locked ? false : mod.enabled}
          disabled={locked}
          onClick={(e) => e.stopPropagation()}
          onCheckedChange={(on) => {
            if (locked) return
            onToggle(on)
          }}
        />
      </div>
    </Card>
  )
}

function FirewallaSettings({ onBack }: { onBack: () => void }) {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = anonOn ? createAnon(anonymitySalt()) : null
  const [publicURL, setPublicURL] = useState(() => window.location.origin)
  const [cmd, setCmd] = useState('')
  const [status, setStatus] = useState<string | null>(null)
  const [agent, setAgent] = useState<{
    online?: boolean
    host?: string
    version?: string
    last_seen?: number
  }>({})
  const [pkgMissing, setPkgMissing] = useState(false)
  const [pkgVer, setPkgVer] = useState('')
  const [boxVer, setBoxVer] = useState('')
  const [updateAvailable, setUpdateAvailable] = useState(false)
  const [busy, setBusy] = useState(false)
  const [catalogBusy, setCatalogBusy] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [events, setEvents] = useState<AgentEventRow[]>([])
  const [pageCursor, setPageCursor] = useState(0)
  const [beforeStack, setBeforeStack] = useState<number[]>([])
  const [logBusy, setLogBusy] = useState(false)

  const refresh = useCallback(async () => {
    const [agentR, boxR] = await Promise.all([api('/v1/settings/agent'), api('/v1/box')])
    if (agentR.ok) {
      const body = (await agentR.json()) as {
        public_base_url?: string
        package_missing?: boolean
        package_version?: string
        update_available?: boolean
        agent?: { online?: boolean; host?: string; version?: string; last_seen?: number }
      }
      if (body.public_base_url) setPublicURL(body.public_base_url)
      setPkgMissing(!!body.package_missing)
      setPkgVer(body.package_version ?? '')
      setUpdateAvailable(!!body.update_available)
      setAgent(body.agent ?? {})
    }
    if (boxR.ok) {
      const body = (await boxR.json()) as { box?: { version?: string } }
      setBoxVer(body.box?.version?.trim() || '')
    } else {
      setBoxVer('')
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = window.setInterval(() => void refresh(), 3000)
    return () => window.clearInterval(id)
  }, [refresh])

  async function loadEvents(beforeId = 0) {
    setLogBusy(true)
    try {
      const q = new URLSearchParams({ limit: '25' })
      if (beforeId > 0) q.set('before_id', String(beforeId))
      const r = await api(`/v1/agent/events?${q}`)
      if (!r.ok) return
      const body = (await r.json()) as { events?: AgentEventRow[] }
      setEvents(body.events ?? [])
      setPageCursor(beforeId)
    } finally {
      setLogBusy(false)
    }
  }

  useEffect(() => {
    if (!logOpen) return
    setBeforeStack([])
    void loadEvents(0)
  }, [logOpen])

  async function generate() {
    setBusy(true)
    setStatus(null)
    setCmd('')
    try {
      const r = await api('/v1/agent/enroll/code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ public_base_url: publicURL }),
      })
      if (!r.ok) {
        setStatus((await r.text()) || `enroll ${r.status}`)
        return
      }
      const body = (await r.json()) as { command?: string; public_base_url?: string }
      setCmd(body.command ?? '')
      if (body.public_base_url) setPublicURL(body.public_base_url)
    } finally {
      setBusy(false)
    }
  }

  async function pushUpdate() {
    setStatus(null)
    const r = await api('/v1/agent/update', { method: 'POST' })
    if (!r.ok) {
      setStatus((await r.text()) || `update ${r.status}`)
      return
    }
    const body = (await r.json()) as { status?: string }
    setStatus(body.status ?? 'ok')
    await refresh()
  }

  async function pushCatalog() {
    setStatus(null)
    setCatalogBusy(true)
    try {
      const r = await api('/v1/agent/catalog', { method: 'POST' })
      if (!r.ok) {
        setStatus((await r.text()) || `catalog ${r.status}`)
        return
      }
      const body = (await r.json()) as { status?: string }
      const st = body.status ?? 'ok'
      setStatus(st === 'ok' ? 'catalog fetched' : st)
    } finally {
      setCatalogBusy(false)
    }
  }

  const lastSeen = agent.last_seen && anon ? anon.shiftTS(agent.last_seen) : agent.last_seen
  const seen =
    lastSeen && lastSeen > 0
      ? new Date(lastSeen * 1000).toLocaleTimeString()
      : '—'
  const showHost = agent.host && anon ? anon.fakeName('host', agent.host) : agent.host
  const showCmd = anon && cmd ? anonymizeEnrollCmd(anon, cmd) : cmd
  const showEvents = anon ? events.map((e) => ({ ...e, ts: anon.shiftTS(e.ts) })) : events

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          onClick={onBack}
        >
          <ArrowLeft className="size-4" />
          Back
        </button>
        <h1 className="text-lg font-semibold tracking-tight">Firewalla</h1>
      </div>

      <Card className="gap-0 py-0">
        <CardContent className="divide-y px-0">
          <Meta label="Status" value={agent.online ? 'Online' : 'Offline'} />
          {boxVer ? <Meta label="Version" value={boxVer} /> : null}
          {agent.online ? <Meta label="Agent" value={agent.version || '—'} /> : null}
          {agent.online && showHost ? <Meta label="Host" value={showHost} /> : null}
          {agent.online ? <Meta label="Seen" value={seen} /> : null}
          <Meta label="Package" value={pkgMissing ? 'missing' : pkgVer || '—'} />
          <div className="flex flex-wrap items-center justify-end gap-2 px-6 py-3">
            {pkgMissing ? <span className="mr-auto text-sm text-destructive">package missing</span> : null}
            {updateAvailable ? <span className="mr-auto text-sm text-muted-foreground">Update available</span> : null}
            <Button
              type="button"
              size="xs"
              variant="outline"
              disabled={!agent.online || catalogBusy}
              onClick={() => void pushCatalog()}
            >
              <RefreshCw className={cn('size-3.5', catalogBusy && 'animate-spin')} />
              Push catalog
            </Button>
            <Button
              type="button"
              size="xs"
              variant="outline"
              disabled={!agent.online || !updateAvailable}
              onClick={() => void pushUpdate()}
            >
              Push update
            </Button>
            <Button type="button" size="xs" disabled={busy || pkgMissing} onClick={() => void generate()}>
              Generate command
            </Button>
          </div>
          {status ? (
            <p
              className={cn(
                'px-6 py-3 text-sm',
                status === 'ok' || status === 'up_to_date' || status === 'catalog fetched'
                  ? 'text-muted-foreground'
                  : 'text-destructive',
              )}
            >
              {status}
            </p>
          ) : null}
        </CardContent>
      </Card>

      {showCmd ? (
        <Card className="gap-0 py-0">
          <CardContent className="space-y-3 px-6 py-4">
            <pre className="overflow-auto rounded-md border border-border bg-[#121214] p-3 text-xs text-zinc-200 whitespace-pre-wrap break-all">
              {showCmd}
            </pre>
            <div className="flex justify-end">
              <CopyButton value={showCmd} />
            </div>
          </CardContent>
        </Card>
      ) : null}

      <Card className="gap-0 py-0">
        <button
          type="button"
          className="flex w-full items-center justify-between px-6 py-3 text-left text-sm"
          onClick={() => setLogOpen((o) => !o)}
        >
          <span>Event log</span>
          <span className="inline-flex items-center gap-2 text-muted-foreground">
            {events.length ? `${events.length}` : null}
            <ChevronDown className={cn('size-4 transition-transform', logOpen && 'rotate-180')} />
          </span>
        </button>
        {logOpen ? (
          <CardContent className="space-y-3 border-t px-6 py-3">
            {showEvents.length === 0 ? (
              <p className="text-sm text-muted-foreground">{logBusy ? '…' : 'No events'}</p>
            ) : (
              <ul className="space-y-1.5 text-sm">
                {showEvents.map((e) => (
                  <li key={e.id} className="grid grid-cols-[6.5rem_minmax(0,1fr)] gap-x-3">
                    <span className="text-muted-foreground tabular-nums">{fmtTime(e.ts)}</span>
                    <span>{agentEventLabel(e)}</span>
                  </li>
                ))}
              </ul>
            )}
            <div className="flex justify-between gap-2">
              <Button
                type="button"
                size="xs"
                variant="ghost"
                disabled={beforeStack.length === 0 || logBusy}
                onClick={() => {
                  const prev = beforeStack[beforeStack.length - 1] ?? 0
                  setBeforeStack((s) => s.slice(0, -1))
                  void loadEvents(prev)
                }}
              >
                Newer
              </Button>
              <Button
                type="button"
                size="xs"
                variant="ghost"
                disabled={events.length < 25 || logBusy}
                onClick={() => {
                  const last = events[events.length - 1]
                  if (!last) return
                  setBeforeStack((s) => [...s, pageCursor])
                  void loadEvents(last.id)
                }}
              >
                Older
              </Button>
            </div>
          </CardContent>
        ) : null}
      </Card>
    </div>
  )
}

type AgentEventRow = {
  id: number
  ts: number
  kind: string
  detail?: string
  from_ver?: string
  to_ver?: string
}

function agentEventLabel(e: AgentEventRow): string {
  switch (e.kind) {
    case 'updated':
      return `updated ${e.from_ver || '?'} → ${e.to_ver || '?'}`
    case 'offline':
      return `offline ${e.detail || ''}`.trim()
    case 'restarted':
      return 'restarted'
    case 'enrolled':
      return 'enrolled'
    case 'update_failed':
      return `update failed: ${e.detail || 'error'}`
    default:
      return e.kind
  }
}

function GeneralSettingsCard() {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const [publicURL, setPublicURL] = useState(() => window.location.origin)
  const [urlDirty, setUrlDirty] = useState(false)
  const [days, setDays] = useState(7)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const [agentR, logsR] = await Promise.all([
          api('/v1/settings/agent'),
          api('/v1/settings/logs'),
        ])
        if (agentR.ok) {
          const body = (await agentR.json()) as { public_base_url?: string }
          if (!cancelled && body.public_base_url && !urlDirty) {
            setPublicURL(body.public_base_url)
          }
        }
        if (logsR.ok) {
          const body = (await logsR.json()) as { retention_days?: number }
          if (!cancelled && body.retention_days) setDays(body.retention_days)
        }
      } catch {
        /* ignore */
      }
    })()
    return () => {
      cancelled = true
    }
  }, [urlDirty])

  async function saveURL() {
    setErr(null)
    const next = publicURL.trim().replace(/\/$/, '')
    try {
      const r = await api('/v1/settings/agent', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ public_base_url: next }),
      })
      if (!r.ok) throw new Error(`save ${r.status}`)
      setPublicURL(next)
      setUrlDirty(false)
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'save failed')
    }
  }

  async function saveDays(n: number) {
    setErr(null)
    setDays(n)
    try {
      const r = await api('/v1/settings/logs', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ retention_days: n }),
      })
      if (!r.ok) throw new Error(`save ${r.status}`)
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'save failed')
    }
  }

  return (
    <Card className="gap-0 py-0">
      <CardContent className="divide-y px-0">
        <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
          <div className="text-muted-foreground">Anonymity mode</div>
          <div className="flex justify-end">
            <Switch checked={anonOn} onCheckedChange={setAnonymityOn} />
          </div>
        </div>
        <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
          <div className="text-muted-foreground">Public URL</div>
          <div className="flex min-w-0 items-center justify-end gap-2">
            <input
              className="min-w-0 flex-1 rounded-md border border-border bg-background px-2 py-1 text-sm"
              value={publicURL}
              onChange={(e) => {
                setUrlDirty(true)
                setPublicURL(e.target.value)
              }}
              onBlur={() => void saveURL()}
            />
            <Button type="button" size="xs" variant="outline" onClick={() => void saveURL()}>
              Save
            </Button>
          </div>
        </div>
        <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
          <div className="text-muted-foreground">Log retention</div>
          <div className="flex items-center justify-end gap-2">
            <input
              type="number"
              min={1}
              max={90}
              className="w-20 rounded-md border border-border bg-background px-2 py-1 text-sm"
              value={days}
              onChange={(e) => setDays(Number(e.target.value) || 7)}
              onBlur={() => void saveDays(days)}
            />
            <span className="text-muted-foreground">days</span>
          </div>
        </div>
        {err ? <p className="px-6 py-3 text-sm text-destructive">{err}</p> : null}
      </CardContent>
    </Card>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 break-all text-right font-medium">{value}</div>
    </div>
  )
}
