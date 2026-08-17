import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  ChevronDown,
  MonitorSmartphone,
  Plus,
  RefreshCw,
  Shield,
  Users,
} from 'lucide-react'
import { DropdownMenu } from 'radix-ui'

import { Breadcrumb } from '@/components/Breadcrumb'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { anonymizeFwAppRules, createAnon } from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import { api } from '@/lib/api'
import { preferredName } from '@/lib/format'
import type {
  Device,
  FwAppExceptionRule,
  FwAppRule,
  FwAppRuleSection,
  FwAppRulesView,
  FwAppScopeChip,
  FwAppStatus,
  ViewMode,
} from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTIONS: { id: FwAppRuleSection; label: string }[] = [
  { id: 'allow', label: 'Allow' },
  { id: 'block', label: 'Block' },
  { id: 'disturb', label: 'Disturb' },
  { id: 'timelimit', label: 'Timelimit' },
  { id: 'other', label: 'Other' },
]

const CREATE_ACTIONS = [
  { id: 'allow', label: 'Allow', cap: 'rule.create.allow' },
  { id: 'block', label: 'Block', cap: 'rule.create.block' },
  { id: 'timelimit', label: 'Time Limit', cap: 'rule.create.timelimit' },
  { id: 'disturb', label: 'Disturb', cap: 'rule.create.disturb' },
] as const

const DAP_SCOPE_ID = '__dap__'

type Sheet = 'add' | 'exceptions' | null

function ruleMatchesScope(rule: FwAppRule, chip: FwAppScopeChip | undefined): boolean {
  if (!chip || chip.kind === 'all' || chip.id === 'all') return true
  if (chip.kind === 'device') {
    const mac = chip.id.toUpperCase()
    return (rule.scope ?? []).some((m) => m.toUpperCase() === mac)
  }
  const tagId = chip.id.startsWith('tag:') ? chip.id.slice(4) : chip.id
  return (rule.tags ?? []).some((t) => {
    const id = t.startsWith('tag:') ? t.slice(4) : t
    return id === tagId || t === chip.id
  })
}

function ruleHaystack(r: FwAppRule): string {
  return [r.target, r.name, r.notes, r.scopeLabel, r.type, r.action, r.id]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
}

function fmtHits(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 10_000) return `${Math.round(n / 1000)}k`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

function isEpochish(s: string): boolean {
  return /^\d+(\.\d+)?$/.test(s.trim())
}

function fmtTs(raw?: string | number | null): string | null {
  if (raw == null || raw === '') return null
  const n = typeof raw === 'number' ? raw : Number(raw)
  if (!Number.isFinite(n) || n <= 0) return null
  const d = new Date(n * 1000)
  if (Number.isNaN(d.getTime())) return null
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function fmtHitBadge(r: FwAppRule): string {
  if (!r.hitCount && !r.lastHitTs) return '0'
  const count = fmtHits(r.hitCount ?? 0)
  const when = fmtTs(r.lastHitTs)
  return when ? `${count} · ${when}` : count
}

function directionLabel(r: FwAppRule): string {
  const dir = (r.trafficDirection || r.direction || '').toLowerCase()
  if (dir === 'outbound' || dir === 'out') return 'Outbound only'
  if (dir === 'inbound' || dir === 'in') return 'Inbound only'
  if (dir === 'bidirection' || dir === 'both') return 'Both directions'
  return 'Always'
}

function scheduleLabel(r: FwAppRule): string {
  const raw = r.activatedTime?.trim()
  if (!raw || isEpochish(raw)) return 'Always'
  return raw
}

function metaLine(r: FwAppRule): string {
  const direction = directionLabel(r)
  const schedule = scheduleLabel(r)
  if (direction === 'Always') return schedule
  if (schedule === 'Always') return direction
  return `${direction} · ${schedule}`
}

function looksLikeUUID(s: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s.trim())
}

function scopeTagId(scope: FwAppScopeChip): string {
  return scope.id.startsWith('tag:') ? scope.id.slice(4) : scope.id
}

function matchingLabel(r: FwAppRule): string {
  const kind = (r.type || '').trim()
  const target = (r.name || r.target || '').trim() || '—'
  if (!kind) return target
  return `${kind.toUpperCase()} ${target}`
}

export function RulesTab({
  mode,
  devices = [],
  labelTag,
  onOpenControl,
}: {
  mode: ViewMode
  devices?: Device[]
  labelTag?: (id: string, preferType?: string) => string
  onOpenControl: () => void
}) {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])

  const [status, setStatus] = useState<FwAppStatus | null>(null)
  const [data, setData] = useState<FwAppRulesView | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [scopeId, setScopeId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [sheet, setSheet] = useState<Sheet>(null)
  const [selectedRule, setSelectedRule] = useState<FwAppRule | null>(null)
  const [addAction, setAddAction] = useState<(typeof CREATE_ACTIONS)[number]['id']>('allow')
  const noticeTimer = useRef<number | null>(null)

  const flash = (msg: string) => {
    setNotice(msg)
    if (noticeTimer.current) window.clearTimeout(noticeTimer.current)
    noticeTimer.current = window.setTimeout(() => setNotice(null), 2500)
  }

  const openScope = (id: string) => {
    setQuery('')
    setScopeId(id)
  }
  const backToScopes = () => {
    setQuery('')
    setScopeId(null)
  }

  const canLoad =
    !!status?.paired && status.state !== 'lan-down' && status.state !== 'error' && status.state !== 'unpaired'

  const loadStatus = useCallback(async () => {
    const r = await api('/v1/fw-app/status')
    if (!r.ok) throw new Error(`status ${r.status}`)
    return (await r.json()) as FwAppStatus
  }, [])

  const loadRules = useCallback(async () => {
    const r = await api('/v1/fw-app/rules')
    if (!r.ok) {
      const body = (await r.json().catch(() => ({}))) as {
        error?: string
        status?: FwAppStatus
      }
      if (body.status) setStatus(body.status)
      throw new Error(body.error || `rules ${r.status}`)
    }
    return (await r.json()) as FwAppRulesView
  }, [])

  const load = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      const st = await loadStatus()
      setStatus(st)
      const ok =
        st.paired && st.state !== 'lan-down' && st.state !== 'error' && st.state !== 'unpaired'
      if (!ok) {
        setData(null)
        return
      }
      setData(await loadRules())
      setStatus(await loadStatus())
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
      setData(null)
    } finally {
      setBusy(false)
    }
  }, [loadStatus, loadRules])

  useEffect(() => {
    void load()
  }, [load])

  const sync = async () => {
    if (!canLoad || busy) return
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const r = await api('/v1/fw-app/rules/refresh', { method: 'POST' })
      if (!r.ok) {
        const body = (await r.json().catch(() => ({}))) as {
          error?: string
          status?: FwAppStatus
        }
        if (body.status) setStatus(body.status)
        throw new Error(body.error || `sync ${r.status}`)
      }
      setData((await r.json()) as FwAppRulesView)
      setStatus(await loadStatus())
      flash('Synced')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  const show = useMemo(() => {
    if (!data) return null
    return anon ? anonymizeFwAppRules(anon, data) : data
  }, [anon, data])

  const devicesByMac = useMemo(() => {
    const m = new Map<string, Device>()
    for (const d of devices) m.set(d.mac.toUpperCase(), d)
    return m
  }, [devices])

  const resolveScopeLabel = useCallback(
    (s: FwAppScopeChip): string => {
      if (s.kind === 'device') {
        const d = devicesByMac.get(s.id.toUpperCase())
        if (d) return preferredName(d)
      }
      if (s.kind === 'tag' || s.kind === 'group') {
        const id = scopeTagId(s)
        const fromApp = labelTag?.(id, 'group')
        if (fromApp && fromApp !== id && !looksLikeUUID(fromApp)) return fromApp
        const fromUser = labelTag?.(id, 'user')
        if (fromUser && fromUser !== id && !looksLikeUUID(fromUser)) return fromUser
      }
      if (looksLikeUUID(s.label)) return `Group ${scopeTagId(s)}`
      return s.label
    },
    [devicesByMac, labelTag],
  )

  const displayOn = useCallback(
    (r: FwAppRule): string => {
      const parts: string[] = []
      for (const ref of r.tags ?? []) {
        if (ref.startsWith('tag:')) {
          const id = ref.slice(4)
          const name = labelTag?.(id, 'group')
          if (name && name !== id && !looksLikeUUID(name)) {
            parts.push(name)
            continue
          }
          parts.push(looksLikeUUID(id) ? `Group ${id}` : id)
          continue
        }
        if (ref.startsWith('intf:')) {
          parts.push('Network')
          continue
        }
        if (ref) parts.push(ref)
      }
      for (const mac of r.scope ?? []) {
        const d = devicesByMac.get(mac.toUpperCase())
        parts.push(d ? preferredName(d) : mac)
      }
      if (parts.length) return parts.join(', ')
      if (r.scopeLabel && !looksLikeUUID(r.scopeLabel) && !isEpochish(r.scopeLabel)) {
        return r.scopeLabel
      }
      return 'All Devices'
    },
    [devicesByMac, labelTag],
  )

  const scopes = useMemo(() => {
    const all = show?.scopes ?? []
    return all.filter((s) => s.kind === 'all' || s.count > 0)
  }, [show?.scopes])

  const allScope = scopes.find((s) => s.kind === 'all')
  const groupScopes = useMemo(
    () =>
      scopes
        .filter((s) => s.kind === 'tag' || s.kind === 'group')
        .slice()
        .sort(
          (a, b) =>
            b.count - a.count || resolveScopeLabel(a).localeCompare(resolveScopeLabel(b)),
        ),
    [scopes, resolveScopeLabel],
  )
  const deviceScopes = useMemo(
    () =>
      scopes
        .filter((s) => s.kind === 'device')
        .slice()
        .sort(
          (a, b) =>
            b.count - a.count || resolveScopeLabel(a).localeCompare(resolveScopeLabel(b)),
        ),
    [scopes, resolveScopeLabel],
  )

  const dapRules = show?.dapRules ?? []
  const dapScope: FwAppScopeChip | null =
    dapRules.length > 0
      ? { id: DAP_SCOPE_ID, kind: 'all', label: 'Active Protect', count: dapRules.length }
      : null

  const activeScope: FwAppScopeChip | undefined =
    scopeId === DAP_SCOPE_ID
      ? (dapScope ?? undefined)
      : scopeId
        ? scopes.find((s) => s.id === scopeId)
        : undefined

  useEffect(() => {
    if (scopeId == null) return
    if (scopeId === DAP_SCOPE_ID) {
      if (!dapScope) setScopeId(null)
      return
    }
    if (!scopes.some((s) => s.id === scopeId)) setScopeId(null)
  }, [scopes, scopeId, dapScope])

  const filtered = useMemo(() => {
    if (!show || !activeScope) return []
    const source = scopeId === DAP_SCOPE_ID ? dapRules : show.rules
    const q = query.trim().toLowerCase()
    return source.filter((r) => {
      if (scopeId !== DAP_SCOPE_ID && !ruleMatchesScope(r, activeScope)) return false
      if (!q) return true
      return ruleHaystack(r).includes(q)
    })
  }, [show, activeScope, query, scopeId, dapRules])

  const bySection = useMemo(() => {
    const m = new Map<FwAppRuleSection, FwAppRule[]>()
    for (const s of SECTIONS) m.set(s.id, [])
    for (const r of filtered) {
      const list = m.get(r.section) ?? m.get('other')!
      list.push(r)
    }
    return m
  }, [filtered])

  const scopeQuery = query.trim().toLowerCase()
  const filteredGroups = useMemo(
    () =>
      scopeQuery
        ? groupScopes.filter((s) => resolveScopeLabel(s).toLowerCase().includes(scopeQuery))
        : groupScopes,
    [groupScopes, scopeQuery, resolveScopeLabel],
  )
  const filteredDevices = useMemo(
    () =>
      scopeQuery
        ? deviceScopes.filter((s) => resolveScopeLabel(s).toLowerCase().includes(scopeQuery))
        : deviceScopes,
    [deviceScopes, scopeQuery, resolveScopeLabel],
  )

  const caps = show?.capabilities ?? {}
  const hub = show?.hub
  const exceptions = show?.exceptions ?? []
  const hitDenom = (hub?.allowHits ?? 0) + (hub?.blockHits ?? 0)
  const allowPct = hitDenom > 0 ? Math.round(((hub?.allowHits ?? 0) / hitDenom) * 100) : 0
  const blockPct = hitDenom > 0 ? 100 - allowPct : 0
  const allowBar = hitDenom > 0 ? ((hub?.allowHits ?? 0) / hitDenom) * 100 : 0
  const blockBar = hitDenom > 0 ? ((hub?.blockHits ?? 0) / hitDenom) * 100 : 0

  const actions = (
    <ActionsMenu
      caps={caps}
      exceptionCount={exceptions.length}
      dapCount={dapRules.length}
      onAdd={() => setSheet('add')}
      onExceptions={() => setSheet('exceptions')}
      onActiveProtect={() => openScope(DAP_SCOPE_ID)}
    />
  )

  if (!canLoad && !busy) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Shield className="size-5 text-muted-foreground" />
          <h1 className="text-lg font-semibold tracking-tight">Rules</h1>
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <Card className="gap-0 py-0">
          <CardContent className="flex flex-wrap items-center justify-between gap-3 px-6 py-8">
            <div className="space-y-1">
              <p className="text-sm font-medium">
                {!status
                  ? 'Control unavailable'
                  : !status.paired
                    ? 'Not paired'
                    : status.state === 'lan-down'
                      ? 'LAN down'
                      : status.state === 'error'
                        ? 'Error'
                        : 'Pair required'}
              </p>
              <p className="text-sm text-muted-foreground">Settings → Control</p>
            </div>
            <Button type="button" size="sm" onClick={onOpenControl}>
              Open Control
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (busy && !show) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Shield className="size-5 text-muted-foreground" />
          <h1 className="text-lg font-semibold tracking-tight">Rules</h1>
        </div>
        <Card className="gap-0 py-0">
          <CardContent className="flex items-center gap-2 px-6 py-8 text-sm text-muted-foreground">
            <RefreshCw className="size-3.5 animate-spin" />
            Loading from Firewalla…
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!show) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <Shield className="size-5 text-muted-foreground" />
          <h1 className="text-lg font-semibold tracking-tight">Rules</h1>
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <Card className="gap-0 py-0">
          <CardContent className="flex flex-wrap items-center justify-between gap-3 px-6 py-8">
            <p className="text-sm text-muted-foreground">No rules loaded</p>
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void sync()}>
              <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
              Sync
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {activeScope ? (
        <Breadcrumb
          items={[
            { label: 'Rules', onClick: backToScopes },
            {
              label:
                scopeId === DAP_SCOPE_ID
                  ? 'Active Protect'
                  : resolveScopeLabel(activeScope),
            },
          ]}
          trailing={
            <div className="flex flex-wrap items-center gap-2">
              <Input
                className="h-8 w-36 sm:w-44"
                placeholder="Search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              {actions}
            </div>
          }
        />
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <Shield className="size-5 text-muted-foreground" />
          <h1 className="text-lg font-semibold tracking-tight">Rules</h1>
          <div className="ml-auto">{actions}</div>
        </div>
      )}

      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {notice ? <p className="text-sm text-muted-foreground">{notice}</p> : null}

      <Card className="gap-0 py-0">
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3 border-b py-4">
          <div className="space-y-1">
            <CardTitle className="text-sm">Hits</CardTitle>
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="font-mono text-2xl tabular-nums">
                {fmtHits(hub?.totalHits ?? 0)}
              </span>
              <span className="text-xs text-muted-foreground">
                {hub?.totalRules ?? 0} rules
                {show?.refreshed_at
                  ? ` · ${new Date(show.refreshed_at).toLocaleString(undefined, {
                      month: 'short',
                      day: 'numeric',
                      hour: '2-digit',
                      minute: '2-digit',
                      hour12: false,
                    })}`
                  : ''}
              </span>
            </div>
          </div>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={() => void sync()}
          >
            <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
            Sync
          </Button>
        </CardHeader>
        <CardContent className="space-y-3 px-6 py-4">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Badge variant="secondary">{hub?.allowCount ?? 0} allow</Badge>
            <Badge variant="destructive">{hub?.blockCount ?? 0} block</Badge>
            <span className="font-mono tabular-nums text-muted-foreground">
              {fmtHits(hub?.allowHits ?? 0)} ({allowPct}%) / {fmtHits(hub?.blockHits ?? 0)} ({blockPct}%)
            </span>
          </div>
          <div className="flex h-2 overflow-hidden rounded-full bg-muted">
            <div className="h-full bg-foreground/60" style={{ width: `${allowBar}%` }} />
            <div className="h-full bg-destructive/80" style={{ width: `${blockBar}%` }} />
          </div>
        </CardContent>
      </Card>

      {activeScope ? (
        filtered.length === 0 ? (
          <p className="text-sm text-muted-foreground">{busy ? '…' : 'No rules'}</p>
        ) : mode === 'list' ? (
          <RulesTable
            bySection={bySection}
            displayOn={displayOn}
            onSelect={setSelectedRule}
          />
        ) : (
          <RulesCompact
            bySection={bySection}
            displayOn={displayOn}
            onSelect={setSelectedRule}
          />
        )
      ) : (
        <ScopePicker
          mode={mode}
          query={query}
          onQuery={setQuery}
          allScope={allScope}
          groups={filteredGroups}
          devices={filteredDevices}
          dap={
            dapScope && (!scopeQuery || 'active protect'.includes(scopeQuery))
              ? dapScope
              : null
          }
          scopeLabel={resolveScopeLabel}
          onOpen={openScope}
        />
      )}

      {sheet === 'add' ? (
        <AddRuleSheet
          action={addAction}
          onAction={setAddAction}
          caps={caps}
          onClose={() => setSheet(null)}
        />
      ) : null}
      {sheet === 'exceptions' ? (
        <SheetShell title="Exceptions" onClose={() => setSheet(null)}>
          {exceptions.length === 0 ? (
            <p className="text-sm text-muted-foreground">No exceptions</p>
          ) : (
            <ExceptionsList rows={exceptions} />
          )}
        </SheetShell>
      ) : null}
      {selectedRule ? (
        <RuleDetailSheet
          rule={selectedRule}
          onLabel={displayOn(selectedRule)}
          caps={caps}
          onClose={() => setSelectedRule(null)}
        />
      ) : null}
    </div>
  )
}

function ActionsMenu({
  caps,
  exceptionCount,
  dapCount,
  onAdd,
  onExceptions,
  onActiveProtect,
}: {
  caps: Record<string, boolean>
  exceptionCount: number
  dapCount: number
  onAdd: () => void
  onExceptions: () => void
  onActiveProtect: () => void
}) {
  return (
    <div className="flex items-center">
      <Button type="button" size="sm" className="rounded-r-none" onClick={onAdd}>
        <Plus className="size-3.5" />
        Add Rule
      </Button>
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <Button
            type="button"
            size="sm"
            className="rounded-l-none border-l border-primary-foreground/20 px-2"
            aria-label="More actions"
          >
            <ChevronDown className="size-3.5" />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={6}
            className="z-50 min-w-48 rounded-md border bg-popover p-1 text-sm shadow-md"
          >
            <DropdownMenu.Item
              className="cursor-pointer rounded-sm px-2 py-1.5 outline-none data-[highlighted]:bg-muted"
              onSelect={onExceptions}
            >
              Exceptions{exceptionCount ? ` (${exceptionCount})` : ''}
            </DropdownMenu.Item>
            {dapCount > 0 ? (
              <DropdownMenu.Item
                className="cursor-pointer rounded-sm px-2 py-1.5 outline-none data-[highlighted]:bg-muted"
                onSelect={onActiveProtect}
              >
                Active Protect ({dapCount})
              </DropdownMenu.Item>
            ) : null}
            <DropdownMenu.Separator className="my-1 h-px bg-border" />
            <DropdownMenu.Item
              className="cursor-pointer rounded-sm px-2 py-1.5 text-muted-foreground outline-none data-[highlighted]:bg-muted"
              disabled
            >
              {caps['rule.reset_hits'] ? 'Reset hits' : 'Reset hits · soon'}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              className="cursor-pointer rounded-sm px-2 py-1.5 text-muted-foreground outline-none data-[highlighted]:bg-muted"
              disabled
            >
              {caps['rule.emergency'] ? 'Emergency Access' : 'Emergency · soon'}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              className="cursor-pointer rounded-sm px-2 py-1.5 text-muted-foreground outline-none data-[highlighted]:bg-muted"
              disabled
            >
              {caps['rule.diagnose'] ? 'Diagnostics' : 'Diagnostics · soon'}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>
  )
}

function RulesTable({
  bySection,
  displayOn,
  onSelect,
}: {
  bySection: Map<FwAppRuleSection, FwAppRule[]>
  displayOn: (r: FwAppRule) => string
  onSelect: (r: FwAppRule) => void
}) {
  return (
    <div className="space-y-4">
      {SECTIONS.map(({ id, label }) => {
        const rows = bySection.get(id) ?? []
        if (!rows.length) return null
        return (
          <Card key={id} className="gap-0 py-0">
            <CardHeader className="border-b py-3">
              <CardTitle className="text-sm">
                {label}
                <span className="ml-2 font-mono text-xs font-normal tabular-nums text-muted-foreground">
                  {rows.length}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent className="px-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Matching</TableHead>
                    <TableHead>On</TableHead>
                    <TableHead>When</TableHead>
                    <TableHead className="text-right">Hits</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((r) => (
                    <TableRow
                      key={r.id}
                      className={cn('cursor-pointer', r.disabled && 'opacity-50')}
                      onClick={() => onSelect(r)}
                    >
                      <TableCell className="max-w-[18rem]">
                        <div className="flex min-w-0 items-center gap-2">
                          <Badge variant="outline">{r.action || r.type || '—'}</Badge>
                          <span className="truncate font-mono">{matchingLabel(r)}</span>
                        </div>
                      </TableCell>
                      <TableCell className="max-w-[12rem] truncate text-muted-foreground">
                        {displayOn(r)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{metaLine(r)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtHitBadge(r)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}

function RulesCompact({
  bySection,
  displayOn,
  onSelect,
}: {
  bySection: Map<FwAppRuleSection, FwAppRule[]>
  displayOn: (r: FwAppRule) => string
  onSelect: (r: FwAppRule) => void
}) {
  return (
    <div className="space-y-4">
      {SECTIONS.map(({ id, label }) => {
        const rows = bySection.get(id) ?? []
        if (!rows.length) return null
        return (
          <div key={id} className="space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium">
              {label}
              <span className="font-mono text-xs tabular-nums text-muted-foreground">
                {rows.length}
              </span>
            </div>
            <div className="space-y-1.5">
              {rows.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => onSelect(r)}
                  className={cn(
                    'flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm hover:bg-muted/40',
                    r.disabled && 'opacity-50',
                  )}
                >
                  <Badge variant="outline">{r.action || r.type || '—'}</Badge>
                  <span className="min-w-0 flex-1 truncate font-mono">{matchingLabel(r)}</span>
                  <span className="hidden max-w-[8rem] truncate text-xs text-muted-foreground sm:inline">
                    {displayOn(r)}
                  </span>
                  <span className="hidden text-xs text-muted-foreground md:inline">
                    {metaLine(r)}
                  </span>
                  <span className="font-mono text-xs tabular-nums text-muted-foreground">
                    {fmtHitBadge(r)}
                  </span>
                </button>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function ExceptionsList({ rows }: { rows: FwAppExceptionRule[] }) {
  return (
    <div className="mt-2 space-y-1">
      {rows.map((e) => (
        <div
          key={e.id}
          className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"
        >
          <Badge variant="outline">{e.type || e.alarmType || '—'}</Badge>
          <span className="min-w-0 flex-1 truncate font-mono">
            {e.targetName || e.target || '—'}
          </span>
          <span className="font-mono tabular-nums text-xs text-muted-foreground">
            {fmtHits(e.matchCount)}
          </span>
        </div>
      ))}
    </div>
  )
}

function SheetShell({
  title,
  onClose,
  children,
}: {
  title: string
  onClose: () => void
  children: ReactNode
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-lg border bg-popover shadow-md"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex shrink-0 items-center justify-between gap-3 border-b px-6 py-4">
          <span className="text-base font-medium">{title}</span>
          <Button type="button" size="xs" variant="outline" onClick={onClose}>
            Close
          </Button>
        </div>
        <div className="min-h-0 overflow-y-auto px-6 py-4">{children}</div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm">{children}</div>
    </div>
  )
}

function RuleDetailSheet({
  rule,
  onLabel,
  caps,
  onClose,
}: {
  rule: FwAppRule
  onLabel: string
  caps: Record<string, boolean>
  onClose: () => void
}) {
  const created = fmtTs(rule.timestamp)
  const activated = fmtTs(rule.activatedTime)
  return (
    <SheetShell title="Rule Details" onClose={onClose}>
      <div className="space-y-4">
        {rule.readOnly ? (
          <div className="rounded-md bg-amber-600/90 px-3 py-2 text-sm text-white">
            This rule is read-only.
          </div>
        ) : null}
        <Field label="Action">
          <span className="capitalize">{rule.action || '—'}</span>
        </Field>
        <Field label="Matching">{matchingLabel(rule)}</Field>
        <Field label="On">{onLabel}</Field>
        <Field label="Schedule">{scheduleLabel(rule)}</Field>
        <Field label="Direction">{directionLabel(rule)}</Field>
        <Field label="Hits">{fmtHitBadge(rule)}</Field>
        {rule.notes ? <Field label="Notes">{rule.notes}</Field> : null}
        <div className="space-y-1 text-xs text-muted-foreground">
          {created ? <p>Created: {created}</p> : null}
          {activated ? <p>Activated: {activated}</p> : null}
          <p className="font-mono">Rule ID: {rule.id}</p>
        </div>
        <div className="flex justify-between gap-2 pt-1">
          <Button type="button" size="sm" variant="outline" className="text-destructive" disabled>
            {caps['rule.delete'] && !rule.readOnly ? 'Delete' : 'Delete · soon'}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    </SheetShell>
  )
}

function AddRuleSheet({
  action,
  onAction,
  caps,
  onClose,
}: {
  action: (typeof CREATE_ACTIONS)[number]['id']
  onAction: (id: (typeof CREATE_ACTIONS)[number]['id']) => void
  caps: Record<string, boolean>
  onClose: () => void
}) {
  const selected = CREATE_ACTIONS.find((a) => a.id === action)!
  const ready = !!caps[selected.cap]

  return (
    <SheetShell title="Add Rule" onClose={onClose}>
      <div className="space-y-4">
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">Action</p>
          <div className="flex flex-wrap gap-2">
            {CREATE_ACTIONS.map((a) => (
              <Button
                key={a.id}
                type="button"
                size="sm"
                variant={action === a.id ? 'default' : 'outline'}
                onClick={() => onAction(a.id)}
              >
                {a.label}
              </Button>
            ))}
          </div>
        </div>
        <label className="block space-y-1.5 text-sm">
          <span className="text-muted-foreground">Name</span>
          <Input disabled placeholder="Optional" />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span className="text-muted-foreground">Matching</span>
          <Input disabled placeholder="Target" />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span className="text-muted-foreground">On</span>
          <Input disabled placeholder="Device / group" />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span className="text-muted-foreground">Schedule</span>
          <Input disabled value="Always" readOnly />
        </label>
        <label className="block space-y-1.5 text-sm">
          <span className="text-muted-foreground">Notes</span>
          <Input disabled />
        </label>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" size="sm" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button type="button" size="sm" disabled>
            {ready ? 'Create' : 'Coming soon'}
          </Button>
        </div>
      </div>
    </SheetShell>
  )
}

function ScopePicker({
  mode,
  query,
  onQuery,
  allScope,
  groups,
  devices,
  dap,
  scopeLabel,
  onOpen,
}: {
  mode: ViewMode
  query: string
  onQuery: (q: string) => void
  allScope: FwAppScopeChip | undefined
  groups: FwAppScopeChip[]
  devices: FwAppScopeChip[]
  dap: FwAppScopeChip | null
  scopeLabel: (s: FwAppScopeChip) => string
  onOpen: (id: string) => void
}) {
  const q = query.trim().toLowerCase()
  const showAll = !!allScope && (!q || 'all devices'.includes(q))
  const empty = !showAll && groups.length === 0 && devices.length === 0 && !dap

  return (
    <div className="space-y-4">
      <Input
        className="h-8 max-w-xs"
        placeholder="Search"
        value={query}
        onChange={(e) => onQuery(e.target.value)}
      />

      {empty ? (
        <p className="text-sm text-muted-foreground">No scopes</p>
      ) : mode === 'list' ? (
        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-4">
            <CardTitle className="text-sm">Scopes</CardTitle>
            <CardDescription>
              {(showAll ? 1 : 0) + groups.length + devices.length + (dap ? 1 : 0)}
            </CardDescription>
          </CardHeader>
          <CardContent className="px-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="text-right">Rules</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showAll && allScope ? (
                  <TableRow className="cursor-pointer" onClick={() => onOpen(allScope.id)}>
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5">
                        <Shield className="size-3.5 text-muted-foreground" />
                        {scopeLabel(allScope)}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">All</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">
                      {allScope.count}
                    </TableCell>
                  </TableRow>
                ) : null}
                {groups.map((s) => (
                  <TableRow key={s.id} className="cursor-pointer" onClick={() => onOpen(s.id)}>
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5">
                        <Users className="size-3.5 text-muted-foreground" />
                        {scopeLabel(s)}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">Group</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{s.count}</TableCell>
                  </TableRow>
                ))}
                {devices.map((s) => (
                  <TableRow key={s.id} className="cursor-pointer" onClick={() => onOpen(s.id)}>
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5">
                        <MonitorSmartphone className="size-3.5 text-muted-foreground" />
                        {scopeLabel(s)}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">Device</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{s.count}</TableCell>
                  </TableRow>
                ))}
                {dap ? (
                  <TableRow className="cursor-pointer" onClick={() => onOpen(dap.id)}>
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5">
                        <Shield className="size-3.5 text-muted-foreground" />
                        {dap.label}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">Protect</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{dap.count}</TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {showAll && allScope ? (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <ScopeCard
                icon={<Shield className="size-4 shrink-0 text-muted-foreground" />}
                label={scopeLabel(allScope)}
                count={allScope.count}
                hint="All"
                onClick={() => onOpen(allScope.id)}
              />
            </div>
          ) : null}
          {groups.length > 0 ? (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Groups</p>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {groups.map((s) => (
                  <ScopeCard
                    key={s.id}
                    icon={<Users className="size-4 shrink-0 text-muted-foreground" />}
                    label={scopeLabel(s)}
                    count={s.count}
                    hint={`${s.count} rules`}
                    onClick={() => onOpen(s.id)}
                  />
                ))}
              </div>
            </div>
          ) : null}
          {devices.length > 0 ? (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Devices</p>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {devices.map((s) => (
                  <ScopeCard
                    key={s.id}
                    icon={
                      <MonitorSmartphone className="size-4 shrink-0 text-muted-foreground" />
                    }
                    label={scopeLabel(s)}
                    count={s.count}
                    hint={`${s.count} rules`}
                    onClick={() => onOpen(s.id)}
                  />
                ))}
              </div>
            </div>
          ) : null}
          {dap ? (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Active Protect</p>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                <ScopeCard
                  icon={<Shield className="size-4 shrink-0 text-muted-foreground" />}
                  label={dap.label}
                  count={dap.count}
                  hint="Read-only"
                  onClick={() => onOpen(dap.id)}
                />
              </div>
            </div>
          ) : null}
        </div>
      )}
    </div>
  )
}

function ScopeCard({
  icon,
  label,
  count,
  hint,
  onClick,
}: {
  icon: ReactNode
  label: string
  count: number
  hint: string
  onClick: () => void
}) {
  return (
    <button type="button" className="text-left" onClick={onClick}>
      <Card className="h-full gap-2 py-4 transition-colors hover:bg-accent/40">
        <CardHeader className="px-5">
          <div className="flex items-baseline justify-between gap-2">
            <CardTitle className="inline-flex min-w-0 items-center gap-1.5 text-base">
              {icon}
              <span className="truncate">{label}</span>
            </CardTitle>
            <span className="font-mono text-lg tabular-nums">{count}</span>
          </div>
          <CardDescription>{hint}</CardDescription>
        </CardHeader>
      </Card>
    </button>
  )
}
