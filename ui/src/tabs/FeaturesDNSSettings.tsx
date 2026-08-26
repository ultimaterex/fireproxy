import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'

type Feature = {
  id: string
  label: string
  enabled: boolean
  writable: boolean
  confirm: boolean
}

type FeaturesView = {
  status: {
    state: string
  }
  features: Feature[]
  dns: {
    unbound_summary: string
    doh_enabled: boolean
    doh_selected: string[]
    config_writable: boolean
  }
}

function Meta({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 break-all text-right font-medium">{value}</div>
    </div>
  )
}

function statusLabel(state: string): string {
  switch (state) {
    case 'lan-down':
      return 'LAN down'
    case 'error':
      return 'Control error'
    case 'unpaired':
      return 'Not paired'
    default:
      return state || 'Control unavailable'
  }
}

export function FeaturesDNSSettingsPage({ onBack }: { onBack: () => void }) {
  const [view, setView] = useState<FeaturesView | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const r = await api('/v1/fw-app/features')
      const body = (await r.json().catch(() => ({}))) as Partial<FeaturesView> & {
        error?: string
      }
      if (body.status) {
        setView((current) => ({
          status: body.status!,
          features: body.features ?? current?.features ?? [],
          dns:
            body.dns ??
            current?.dns ?? {
              unbound_summary: 'off',
              doh_enabled: false,
              doh_selected: [],
              config_writable: false,
            },
        }))
      }
      if (!r.ok) throw new Error(body.error || `features ${r.status}`)
      setView(body as FeaturesView)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  async function toggle(feature: Feature, enabled: boolean) {
    if (busy || view?.status.state !== 'lan-ok' || !feature.writable) return
    if (feature.confirm && !window.confirm(`${enabled ? 'Enable' : 'Disable'} ${feature.label}?`)) {
      return
    }

    setBusy(true)
    setError(null)
    try {
      const r = await api(`/v1/fw-app/features/${encodeURIComponent(feature.id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      })
      const body = (await r.json().catch(() => ({}))) as Partial<FeaturesView> & {
        error?: string
      }
      if (!r.ok) {
        if (body.status) {
          setView((current) => (current ? { ...current, status: body.status! } : current))
        }
        throw new Error(body.error || `save ${r.status}`)
      }
      setView(body as FeaturesView)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
    } finally {
      setBusy(false)
    }
  }

  const lanOK = view?.status.state === 'lan-ok'
  const dohServers = view?.dns.doh_selected?.length ? view.dns.doh_selected.join(', ') : '—'

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
        <h1 className="text-lg font-semibold tracking-tight">Features &amp; DNS</h1>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {view && !lanOK ? (
        <p className="rounded-md border px-3 py-2 text-sm text-muted-foreground">
          {statusLabel(view.status.state)}
        </p>
      ) : null}

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Features</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {loading && !view ? (
            <p className="px-6 py-3 text-sm text-muted-foreground">Loading…</p>
          ) : (
            (view?.features ?? []).map((feature) => (
              <div
                key={feature.id}
                className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-6 py-3 text-sm"
              >
                <div>{feature.label}</div>
                <Switch
                  checked={feature.enabled}
                  disabled={busy || !lanOK || !feature.writable}
                  aria-label={feature.label}
                  onCheckedChange={(enabled) => void toggle(feature, enabled)}
                />
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>DNS</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          <Meta label="Unbound" value={view?.dns.unbound_summary ?? '—'} />
          <Meta label="DoH" value={view ? (view.dns.doh_enabled ? 'On' : 'Off') : '—'} />
          <Meta label="Servers" value={dohServers} />
          <Meta label="Config" value="Coming soon" />
        </CardContent>
      </Card>
    </div>
  )
}
