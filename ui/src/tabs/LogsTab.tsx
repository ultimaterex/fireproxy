import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { anonymizeLogLines, createAnon } from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import { cn } from '@/lib/utils'

type Range = '6h' | '24h' | '7d'
type Source = 'all' | 'unbound' | 'dnsmasq' | 'firerouter'
type LiveState = 'off' | 'connecting' | 'on' | 'error'

type LogLine = {
  ts: number
  source: string
  severity?: string
  line: string
}

export function LogsTab() {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])
  const [range, setRange] = useState<Range>('6h')
  const [source, setSource] = useState<Source>('all')
  const [q, setQ] = useState('')
  const [lines, setLines] = useState<LogLine[]>([])
  const [status, setStatus] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [wantLive, setWantLive] = useState(false)
  const [liveState, setLiveState] = useState<LiveState>('off')
  const fetched = useRef(false)
  const listRef = useRef<HTMLDivElement>(null)
  const stickBottom = useRef(true)
  const sourceRef = useRef(source)
  const qRef = useRef(q)
  sourceRef.current = source
  qRef.current = q

  const load = useCallback(async () => {
    const params = new URLSearchParams({ range })
    if (source !== 'all') params.set('source', source)
    if (!anonOn && q.trim()) params.set('q', q.trim())
    const r = await fetch(`/v1/logs?${params}`)
    if (!r.ok) throw new Error(`logs ${r.status}`)
    const body = (await r.json()) as { lines?: LogLine[] }
    setLines(body.lines ?? [])
  }, [range, source, q, anonOn])

  const refresh = useCallback(async () => {
    setBusy(true)
    setStatus(null)
    try {
      const fr = await fetch('/v1/logs/fetch', { method: 'POST' })
      if (fr.ok) {
        const body = (await fr.json()) as { status?: string }
        if (body.status && body.status !== 'ok') setStatus(body.status)
      } else {
        setStatus(`fetch ${fr.status}`)
      }
      await load()
    } catch (e) {
      setStatus(e instanceof Error ? e.message : 'failed')
    } finally {
      setBusy(false)
    }
  }, [load])

  useEffect(() => {
    if (fetched.current) return
    fetched.current = true
    void refresh()
  }, [refresh])

  useEffect(() => {
    void load().catch(() => {})
  }, [load])

  useEffect(() => {
    if (!wantLive) {
      setLiveState('off')
      return
    }
    setLiveState('connecting')
    setStatus(null)
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/v1/logs/ws`)
    ws.onopen = () => {
      // Wait for server logs.status{live:true}; socket open alone isn't enough.
      setLiveState((s) => (s === 'on' ? s : 'connecting'))
    }
    ws.onmessage = (ev) => {
      try {
        const env = JSON.parse(String(ev.data)) as {
          type?: string
          logs_batch?: { lines?: LogLine[] }
          logs_status?: { error?: string; live?: boolean }
        }
        if (env.type === 'logs.status') {
          if (env.logs_status?.error) {
            setStatus(env.logs_status.error)
            setLiveState('error')
            return
          }
          if (env.logs_status?.live) {
            setLiveState('on')
            setStatus(null)
          }
          return
        }
        if (env.type !== 'logs.batch' || !env.logs_batch?.lines?.length) return
        // Receiving batches also proves the pipe is live.
        setLiveState('on')
        const src = sourceRef.current
        const needle = qRef.current.trim().toLowerCase()
        const fake = isAnonymityOn() ? createAnon(anonymitySalt()) : null
        const add = env.logs_batch.lines.filter((ln) => {
          if (src !== 'all' && ln.source !== src) return false
          if (!needle) return true
          const line = fake ? fake.rewriteText(ln.line) : ln.line
          return line.toLowerCase().includes(needle)
        })
        if (!add.length) return
        setLines((prev) => [...prev, ...add].slice(-2000))
      } catch {
        /* ignore */
      }
    }
    ws.onerror = () => {
      setStatus('live ws error')
      setLiveState('error')
    }
    ws.onclose = () => {
      setLiveState((s) => (s === 'off' ? s : 'off'))
    }
    return () => {
      ws.close()
    }
  }, [wantLive])

  useEffect(() => {
    if (liveState !== 'on' || !stickBottom.current) return
    const el = listRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines, liveState])

  const liveLabel = liveState === 'connecting' ? 'Live…' : 'Live'

  const showLines = useMemo(() => {
    const faked = anon ? anonymizeLogLines(anon, lines) : lines
    const needle = q.trim().toLowerCase()
    if (!anon || !needle) return faked
    return faked.filter(
      (ln) => ln.line.toLowerCase().includes(needle) || ln.source.toLowerCase().includes(needle),
    )
  }, [anon, lines, q])

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-lg font-semibold tracking-tight">Logs</h1>
        <div className="flex gap-1">
          {(['6h', '24h', '7d'] as Range[]).map((id) => (
            <Chip key={id} active={range === id} onClick={() => setRange(id)}>
              {id}
            </Chip>
          ))}
        </div>
        <div className="ml-auto flex gap-2">
          <button
            type="button"
            onClick={() => setWantLive((v) => !v)}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs',
              liveState === 'on' && 'border border-emerald-500/50 bg-emerald-500/15 text-emerald-400',
              liveState === 'connecting' && 'border border-amber-500/40 bg-amber-500/10 text-amber-400',
              liveState === 'error' && 'border border-red-500/40 bg-red-500/10 text-red-400',
              liveState === 'off' && 'border border-border text-muted-foreground',
            )}
          >
            <span
              className={cn(
                'size-1.5 rounded-full',
                liveState === 'on' && 'bg-emerald-500',
                liveState === 'connecting' && 'animate-pulse bg-amber-400',
                liveState === 'error' && 'bg-red-500',
                liveState === 'off' && 'bg-zinc-500',
              )}
            />
            {liveLabel}
          </button>
          <Button size="sm" variant="secondary" disabled={busy} onClick={() => void refresh()}>
            <RefreshCw className={cn('size-3.5', busy && 'animate-spin')} />
            Refresh
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {(['all', 'unbound', 'dnsmasq', 'firerouter'] as Source[]).map((id) => (
          <Chip key={id} active={source === id} onClick={() => setSource(id)}>
            {id}
          </Chip>
        ))}
        <input
          className="min-w-[10rem] flex-1 rounded-md border border-border bg-background px-2 py-1 text-sm"
          placeholder="filter"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>

      {status ? <p className="text-sm text-destructive">{status}</p> : null}

      <div
        ref={listRef}
        className="max-h-[70vh] overflow-auto rounded-md border border-border bg-[#121214] p-3 font-mono text-xs leading-5 text-zinc-200"
        onScroll={() => {
          const el = listRef.current
          if (!el) return
          stickBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48
        }}
      >
        {showLines.length === 0 ? (
          <div className="text-muted-foreground">No lines</div>
        ) : (
          showLines.map((ln, i) => (
            <div key={`${ln.ts}-${i}`} className="whitespace-pre-wrap break-all">
              <span className="text-zinc-500">{fmtTime(ln.ts)} </span>
              <span className="text-amber-300">{ln.source} </span>
              <span>{ln.line}</span>
            </div>
          ))
        )}
      </div>
    </div>
  )
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

function fmtTime(ts: number) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  return d.toLocaleTimeString(undefined, { hour12: false })
}
