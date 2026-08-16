import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronDown, History, RefreshCw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { anonymizeControlEvents, createAnon } from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import { api } from '@/lib/api'
import type { ControlEvent } from '@/lib/types'
import { cn } from '@/lib/utils'

const PAGE = 50
const TABLE = 'grid w-full gap-x-3 divide-y'
const TABLE_HEAD =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2 text-xs text-muted-foreground'
const TABLE_ROW = 'col-span-full grid grid-cols-subgrid items-center px-6 py-2.5 text-sm'
const COLS =
  'grid-cols-[7.5rem_5.5rem_minmax(0,1fr)_4.5rem_minmax(0,1fr)_3.5rem_minmax(0,1.2fr)_1.25rem]'

const SCHEMES = ['', 'firewalla', 'unifi'] as const
const ACTOR_KINDS = ['', 'user', 'system'] as const
const RESULTS = ['', 'ok', '400', '409', 'busy', '502', 'error'] as const

export function HistoryTab() {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])

  const [scheme, setScheme] = useState('')
  const [action, setAction] = useState('')
  const [actorKind, setActorKind] = useState('')
  const [result, setResult] = useState('')
  const [q, setQ] = useState('')
  const [actionsByScheme, setActionsByScheme] = useState<Record<string, string[]>>({})
  const [events, setEvents] = useState<ControlEvent[]>([])
  const [busy, setBusy] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [expanded, setExpanded] = useState<Record<number, boolean>>({})

  const actionOptions = useMemo(() => {
    if (!scheme) {
      const all = new Set<string>()
      for (const list of Object.values(actionsByScheme)) {
        for (const a of list) all.add(a)
      }
      return [...all].sort()
    }
    return actionsByScheme[scheme] ?? []
  }, [actionsByScheme, scheme])

  useEffect(() => {
    if (action && !actionOptions.includes(action)) setAction('')
  }, [action, actionOptions])

  const serverQ = anonOn ? '' : q.trim()

  const fetchPage = useCallback(
    async (beforeId: number) => {
      const params = new URLSearchParams({ limit: String(PAGE) })
      if (scheme) params.set('scheme', scheme)
      if (action) params.set('action', action)
      if (actorKind) params.set('actor_kind', actorKind)
      if (result) params.set('result', result)
      if (serverQ) params.set('q', serverQ)
      if (beforeId > 0) params.set('before_id', String(beforeId))
      const r = await api(`/v1/history?${params}`)
      if (!r.ok) throw new Error(`history ${r.status}`)
      const body = (await r.json()) as {
        events?: ControlEvent[]
        actions?: Record<string, string[]>
      }
      if (body.actions) setActionsByScheme(body.actions)
      return body.events ?? []
    },
    [scheme, action, actorKind, result, serverQ],
  )

  const load = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      const rows = await fetchPage(0)
      setEvents(rows)
      setHasMore(rows.length >= PAGE)
      setExpanded({})
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
      setEvents([])
      setHasMore(false)
    } finally {
      setBusy(false)
    }
  }, [fetchPage])

  useEffect(() => {
    void load()
  }, [load])

  const loadMore = async () => {
    const last = events[events.length - 1]
    if (!last || loadingMore) return
    setLoadingMore(true)
    setError(null)
    try {
      const rows = await fetchPage(last.id)
      setEvents((prev) => [...prev, ...rows])
      setHasMore(rows.length >= PAGE)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed')
    } finally {
      setLoadingMore(false)
    }
  }

  const showEvents = useMemo(() => {
    const faked = anon ? anonymizeControlEvents(anon, events) : events
    const needle = q.trim().toLowerCase()
    if (!anon || !needle) return faked
    return faked.filter(
      (e) =>
        e.target.toLowerCase().includes(needle) ||
        (e.summary || '').toLowerCase().includes(needle) ||
        e.actor.toLowerCase().includes(needle) ||
        e.action.toLowerCase().includes(needle),
    )
  }, [anon, events, q])

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <History className="size-5 text-muted-foreground" />
        <h1 className="text-lg font-semibold tracking-tight">History</h1>
        <div className="ml-auto">
          <Button size="sm" variant="secondary" disabled={busy} onClick={() => void load()}>
            <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
            Refresh
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {SCHEMES.map((id) => (
          <Chip key={id || 'all'} active={scheme === id} onClick={() => setScheme(id)}>
            {id || 'all'}
          </Chip>
        ))}
        <select
          className="rounded-md border border-border bg-background px-2 py-1 text-xs"
          value={action}
          onChange={(e) => setAction(e.target.value)}
        >
          <option value="">action</option>
          {actionOptions.map((a) => (
            <option key={a} value={a}>
              {a}
            </option>
          ))}
        </select>
        <select
          className="rounded-md border border-border bg-background px-2 py-1 text-xs"
          value={actorKind}
          onChange={(e) => setActorKind(e.target.value)}
        >
          {ACTOR_KINDS.map((id) => (
            <option key={id || 'actor-all'} value={id}>
              {id || 'actor'}
            </option>
          ))}
        </select>
        <select
          className="rounded-md border border-border bg-background px-2 py-1 text-xs"
          value={result}
          onChange={(e) => setResult(e.target.value)}
        >
          {RESULTS.map((id) => (
            <option key={id || 'result-all'} value={id}>
              {id || 'result'}
            </option>
          ))}
        </select>
        <input
          className="min-w-[10rem] flex-1 rounded-md border border-border bg-background px-2 py-1 text-sm"
          placeholder="filter"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <Card className="gap-0 py-0">
        <CardContent className="px-0 py-0">
          {showEvents.length === 0 ? (
            <p className="px-6 py-4 text-sm text-muted-foreground">{busy ? '…' : 'No events'}</p>
          ) : (
            <div className={cn(TABLE, COLS)}>
              <div className={TABLE_HEAD}>
                <div>Time</div>
                <div>Scheme</div>
                <div>Action</div>
                <div>Actor</div>
                <div>Target</div>
                <div>Result</div>
                <div>Summary</div>
                <div />
              </div>
              {showEvents.map((e) => {
                const open = !!expanded[e.id]
                const canExpand = e.before != null || e.after != null
                return (
                  <div key={e.id} className="col-span-full grid grid-cols-subgrid">
                    <button
                      type="button"
                      className={cn(
                        TABLE_ROW,
                        'w-full cursor-pointer rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40',
                        !canExpand && 'cursor-default hover:bg-transparent',
                      )}
                      onClick={() => {
                        if (!canExpand) return
                        setExpanded((m) => ({ ...m, [e.id]: !m[e.id] }))
                      }}
                    >
                      <div className="tabular-nums text-muted-foreground">{fmtMs(e.ts)}</div>
                      <div className="truncate text-muted-foreground">{e.scheme}</div>
                      <div className="min-w-0 truncate font-medium">{e.action}</div>
                      <div className="truncate text-muted-foreground">
                        {e.actor || e.actor_kind}
                      </div>
                      <div className="min-w-0 truncate font-mono text-xs">{e.target || '—'}</div>
                      <div
                        className={cn(
                          'text-xs',
                          e.result === 'ok' ? 'text-muted-foreground' : 'text-destructive',
                        )}
                      >
                        {e.result}
                      </div>
                      <div className="min-w-0 truncate text-muted-foreground">
                        {e.summary || e.error || '—'}
                      </div>
                      <div className="flex justify-end">
                        {canExpand ? (
                          <ChevronDown
                            className={cn(
                              'size-4 text-muted-foreground transition-transform',
                              open && 'rotate-180',
                            )}
                          />
                        ) : null}
                      </div>
                    </button>
                    {open && canExpand ? (
                      <div className="col-span-full border-t border-border/60 bg-muted/20 px-6 py-3 font-mono text-xs leading-5 text-muted-foreground">
                        <Diff before={e.before} after={e.after} />
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {hasMore ? (
        <div className="flex justify-center">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={loadingMore || busy}
            onClick={() => void loadMore()}
          >
            {loadingMore ? '…' : 'Load more'}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function Diff({ before, after }: { before?: unknown; after?: unknown }) {
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      <div>
        <div className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">Before</div>
        <pre className="whitespace-pre-wrap break-all">{fmtSnap(before)}</pre>
      </div>
      <div>
        <div className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">After</div>
        <pre className="whitespace-pre-wrap break-all">{fmtSnap(after)}</pre>
      </div>
    </div>
  )
}

function fmtSnap(v: unknown): string {
  if (v == null) return '—'
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

function fmtMs(ts: number): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
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
