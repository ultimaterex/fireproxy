import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Plus, RefreshCw, Settings2, Shield } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
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
import type {
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

type Sheet = 'add' | 'options' | null

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

function fmtHitBadge(r: FwAppRule): string | null {
  if (!r.hitCount && !r.lastHitTs) return null
  const count = fmtHits(r.hitCount)
  if (!r.lastHitTs) return count
  const d = new Date(r.lastHitTs * 1000)
  if (Number.isNaN(d.getTime())) return count
  const date = d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  return `${count} · ${date}`
}

function metaLine(r: FwAppRule): string {
  const dir = (r.trafficDirection || r.direction || '').toLowerCase()
  let direction = 'Always'
  if (dir === 'outbound' || dir === 'out') direction = 'Outbound only'
  else if (dir === 'inbound' || dir === 'in') direction = 'Inbound only'
  else if (dir === 'bidirection' || dir === 'both') direction = 'Both'
  const schedule = r.activatedTime?.trim() || 'Always'
  if (direction === 'Always') return schedule
  if (schedule === 'Always') return direction
  return `${direction}, ${schedule}`
}

export function RulesTab({
  mode,
  onOpenControl,
}: {
  mode: ViewMode
  onOpenControl: () => void
}) {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])

  const [status, setStatus] = useState<FwAppStatus | null>(null)
  const [data, setData] = useState<FwAppRulesView | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [scopeId, setScopeId] = useState('all')
  const [query, setQuery] = useState('')
  const [sheet, setSheet] = useState<Sheet>(null)
  const [exceptionsOpen, setExceptionsOpen] = useState(false)
  const [addAction, setAddAction] = useState<(typeof CREATE_ACTIONS)[number]['id']>('allow')

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
      const rules = await loadRules()
      setData(rules)
      // RefreshRules marks lan-ok; pick up status after first fetch.
      const st2 = await loadStatus()
      setStatus(st2)
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

  const refresh = async () => {
    if (!canLoad || busy) return
    setBusy(true)
    setError(null)
    try {
      const r = await api('/v1/fw-app/rules/refresh', { method: 'POST' })
      if (!r.ok) {
        const body = (await r.json().catch(() => ({}))) as {
          error?: string
          status?: FwAppStatus
        }
        if (body.status) setStatus(body.status)
        throw new Error(body.error || `refresh ${r.status}`)
      }
      setData((await r.json()) as FwAppRulesView)
      const st = await loadStatus()
      setStatus(st)
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

  const scopes = useMemo(() => show?.scopes ?? [], [show?.scopes])
  const activeChip = scopes.find((s) => s.id === scopeId) ?? scopes[0]

  useEffect(() => {
    if (!scopes.length) return
    if (!scopes.some((s) => s.id === scopeId)) setScopeId('all')
  }, [scopes, scopeId])

  const filtered = useMemo(() => {
    if (!show) return []
    const q = query.trim().toLowerCase()
    return show.rules.filter((r) => {
      if (!ruleMatchesScope(r, activeChip)) return false
      if (!q) return true
      return ruleHaystack(r).includes(q)
    })
  }, [show, activeChip, query])

  const bySection = useMemo(() => {
    const m = new Map<FwAppRuleSection, FwAppRule[]>()
    for (const s of SECTIONS) m.set(s.id, [])
    for (const r of filtered) {
      const list = m.get(r.section) ?? m.get('other')!
      list.push(r)
    }
    return m
  }, [filtered])

  const caps = show?.capabilities ?? {}
  const hub = show?.hub
  const exceptions = show?.exceptions ?? []
  const hitTotal = Math.max(1, (hub?.allowHits ?? 0) + (hub?.blockHits ?? 0))
  const allowPct = hub ? ((hub.allowHits / hitTotal) * 100) : 0
  const blockPct = hub ? ((hub.blockHits / hitTotal) * 100) : 0

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
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void refresh()}>
              <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
              Retry
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Shield className="size-5 text-muted-foreground" />
        <h1 className="text-lg font-semibold tracking-tight">Rules</h1>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          <Button type="button" size="sm" variant="outline" onClick={() => setSheet('options')}>
            <Settings2 className="size-3.5" />
            Options
          </Button>
          <Button type="button" size="sm" onClick={() => setSheet('add')}>
            <Plus className="size-3.5" />
            Add Rule
          </Button>
        </div>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

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
            onClick={() => void refresh()}
          >
            <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
            Refresh
          </Button>
        </CardHeader>
        <CardContent className="space-y-3 px-6 py-4">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Badge variant="secondary">{hub?.allowCount ?? 0} allow</Badge>
            <Badge variant="destructive">{hub?.blockCount ?? 0} block</Badge>
            <span className="font-mono tabular-nums text-muted-foreground">
              {fmtHits(hub?.allowHits ?? 0)} / {fmtHits(hub?.blockHits ?? 0)}
            </span>
          </div>
          <div className="flex h-2 overflow-hidden rounded-full bg-muted">
            <div className="h-full bg-foreground/60" style={{ width: `${allowPct}%` }} />
            <div className="h-full bg-destructive/80" style={{ width: `${blockPct}%` }} />
          </div>
          {exceptions.length > 0 ? (
            <div className="border-t pt-3">
              <button
                type="button"
                className="flex w-full items-center justify-between text-sm"
                onClick={() => setExceptionsOpen((o) => !o)}
              >
                <span className="text-muted-foreground">Exceptions</span>
                <span className="font-mono tabular-nums text-muted-foreground">
                  {exceptions.length}
                </span>
              </button>
              {exceptionsOpen ? <ExceptionsList rows={exceptions} /> : null}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center gap-2">
        {scopes.map((s) => (
          <Chip key={s.id} active={scopeId === s.id} onClick={() => setScopeId(s.id)}>
            {s.label}
            <span className="ml-1 font-mono tabular-nums opacity-70">{s.count}</span>
          </Chip>
        ))}
        <Input
          className="ml-auto h-8 min-w-[10rem] max-w-xs"
          placeholder="Search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">{busy ? '…' : 'No rules'}</p>
      ) : mode === 'list' ? (
        <RulesTable bySection={bySection} />
      ) : (
        <RulesCompact bySection={bySection} />
      )}

      {sheet === 'add' ? (
        <AddRuleSheet
          action={addAction}
          onAction={setAddAction}
          caps={caps}
          onClose={() => setSheet(null)}
        />
      ) : null}
      {sheet === 'options' ? (
        <OptionsSheet
          caps={caps}
          exceptions={exceptions}
          onClose={() => setSheet(null)}
        />
      ) : null}
    </div>
  )
}

function RulesTable({ bySection }: { bySection: Map<FwAppRuleSection, FwAppRule[]> }) {
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
                    <TableHead>Type</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Scope</TableHead>
                    <TableHead>When</TableHead>
                    <TableHead className="text-right">Hits</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((r) => (
                    <TableRow key={r.id} className={cn(r.disabled && 'opacity-50')}>
                      <TableCell>
                        <Badge variant="outline">{r.type || '—'}</Badge>
                      </TableCell>
                      <TableCell className="max-w-[18rem] truncate font-mono">
                        {r.name || r.target || '—'}
                      </TableCell>
                      <TableCell className="max-w-[10rem] truncate text-muted-foreground">
                        {r.scopeLabel || '—'}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{metaLine(r)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-muted-foreground">
                        {fmtHitBadge(r) ?? '—'}
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

function RulesCompact({ bySection }: { bySection: Map<FwAppRuleSection, FwAppRule[]> }) {
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
                <div
                  key={r.id}
                  className={cn(
                    'flex items-center gap-2 rounded-md border px-3 py-2 text-sm',
                    r.disabled && 'opacity-50',
                  )}
                >
                  <Badge variant="outline">{r.type || '—'}</Badge>
                  <span className="min-w-0 flex-1 truncate font-mono">
                    {r.name || r.target || '—'}
                  </span>
                  <span className="hidden max-w-[8rem] truncate text-xs text-muted-foreground sm:inline">
                    {r.scopeLabel || '—'}
                  </span>
                  <span className="hidden text-xs text-muted-foreground md:inline">
                    {metaLine(r)}
                  </span>
                  <span className="font-mono tabular-nums text-xs text-muted-foreground">
                    {fmtHitBadge(r) ?? '—'}
                  </span>
                </div>
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

function OptionsSheet({
  caps,
  exceptions,
  onClose,
}: {
  caps: Record<string, boolean>
  exceptions: FwAppExceptionRule[]
  onClose: () => void
}) {
  return (
    <SheetShell title="Options" onClose={onClose}>
      <div className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm">Reset hits</span>
          <Button type="button" size="xs" variant="outline" disabled>
            {caps['rule.reset_hits'] ? 'Reset' : 'Coming soon'}
          </Button>
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm">Emergency Access</span>
          <Button type="button" size="xs" variant="outline" disabled>
            {caps['rule.emergency'] ? 'Enable' : 'Coming soon'}
          </Button>
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm">Diagnostics</span>
          <Button type="button" size="xs" variant="outline" disabled>
            {caps['rule.diagnose'] ? 'Run' : 'Coming soon'}
          </Button>
        </div>
        {exceptions.length > 0 ? (
          <div className="space-y-2 border-t pt-4">
            <p className="text-sm text-muted-foreground">Exceptions</p>
            <ExceptionsList rows={exceptions} />
          </div>
        ) : null}
      </div>
    </SheetShell>
  )
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'rounded-md px-2 py-0.5 text-xs',
        active ? 'bg-[#3f3f44] text-white' : 'border border-border text-muted-foreground',
      )}
    >
      {children}
    </button>
  )
}
