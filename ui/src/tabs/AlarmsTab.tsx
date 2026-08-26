import { useCallback, useEffect, useState } from 'react'
import { ShieldAlert } from 'lucide-react'

import { AlarmInboxList } from '@/components/AlarmInboxList'
import { SourceBadge } from '@/components/SourceBadge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { api } from '@/lib/api'
import type { AlarmsView } from '@/lib/types'
import { cn } from '@/lib/utils'

const emptyView = (): AlarmsView => ({
  active_alarm_count: 0,
  new_alarms: [],
})

export function AlarmsTab({ controlLanOk }: { controlLanOk: boolean }) {
  const [view, setView] = useState<AlarmsView | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [busyAid, setBusyAid] = useState<number | null>(null)
  const [confirmAll, setConfirmAll] = useState(false)
  const [nowMs, setNowMs] = useState(() => Date.now())

  const load = useCallback(async () => {
    setBusy(true)
    setError(null)
    setNowMs(Date.now())
    try {
      const r = await api('/v1/alarms')
      if (r.status === 404) {
        setView(emptyView())
        return
      }
      if (!r.ok) throw new Error(`alarms ${r.status}`)
      const data = (await r.json()) as AlarmsView
      setView({
        active_alarm_count: data.active_alarm_count ?? 0,
        new_alarms: data.new_alarms ?? [],
        source: data.source,
        fetched_at: data.fetched_at,
        stale: data.stale,
        reason: data.reason,
        enriched_from: data.enriched_from,
      })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setView(null)
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const ignoreOne = async (aid: number) => {
    if (!controlLanOk || busyAid != null) return
    setBusyAid(aid)
    setError(null)
    try {
      const r = await api('/v1/fw-app/alarms/ignore', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ alarm_id: aid }),
      })
      if (!r.ok) {
        const msg = (await r.text().catch(() => '')) || `ignore ${r.status}`
        throw new Error(msg)
      }
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusyAid(null)
    }
  }

  const ignoreAll = async () => {
    if (!controlLanOk || busyAid != null) return
    setConfirmAll(false)
    setBusyAid(-1)
    setError(null)
    try {
      const r = await api('/v1/fw-app/alarms/ignore-all', { method: 'POST' })
      if (!r.ok) {
        const msg = (await r.text().catch(() => '')) || `ignore-all ${r.status}`
        throw new Error(msg)
      }
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusyAid(null)
    }
  }

  const count = view?.active_alarm_count ?? 0
  const alarms = view?.new_alarms ?? []
  const source = view?.source
  const showSource =
    source === 'agent' || source === 'fw-app-init' || source === 'fw-app-get'

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <ShieldAlert className="size-5 text-muted-foreground" />
        <h1 className="text-lg font-semibold tracking-tight">Alarms</h1>
        <span className="text-sm tabular-nums text-muted-foreground">{count}</span>
        {showSource ? (
          <SourceBadge
            source={source}
            stale={view?.stale}
            enrichedFrom={view?.enriched_from}
            reason={view?.reason}
          />
        ) : null}
        <div className="ml-auto flex items-center gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={!controlLanOk || busy || busyAid != null || count === 0}
            onClick={() => {
              if (busyAid != null) return
              setConfirmAll(true)
            }}
          >
            Ignore all
          </Button>
        </div>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <Card className="gap-0 py-0">
        <CardContent className={cn('px-0', busy && !view && 'opacity-60')}>
          {view ? (
            <AlarmInboxList
              alarms={alarms}
              activeCount={count}
              nowMs={nowMs}
              controlLanOk={controlLanOk}
              busyAid={busyAid}
              onIgnore={(aid) => void ignoreOne(aid)}
            />
          ) : busy ? (
            <p className="px-6 py-8 text-sm text-muted-foreground">Loading…</p>
          ) : (
            <p className="px-6 py-8 text-sm text-muted-foreground">No alarms</p>
          )}
        </CardContent>
      </Card>

      {confirmAll ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setConfirmAll(false)}
        >
          <div
            className="w-full max-w-sm overflow-hidden rounded-lg border bg-popover shadow-md"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-labelledby="ignore-all-title"
          >
            <div className="border-b px-6 py-4">
              <h2 id="ignore-all-title" className="text-base font-medium">
                Ignore all
              </h2>
            </div>
            <div className="px-6 py-5 text-sm text-muted-foreground">
              {count} active {count === 1 ? 'alarm' : 'alarms'}
            </div>
            <div className="flex justify-end gap-2 border-t px-6 py-3">
              <Button type="button" size="sm" variant="outline" onClick={() => setConfirmAll(false)}>
                Cancel
              </Button>
              <Button
                type="button"
                size="sm"
                disabled={!controlLanOk || busy || busyAid != null}
                onClick={() => void ignoreAll()}
              >
                Ignore all
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
