import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { ArrowLeft } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'
import type { ModuleInfo } from '@/lib/types'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const STATUS_POLL_MS = 5000

type FWAppStatus = {
  paired: boolean
  state: string
  box_ip?: string
  gid_hint?: string
  email?: string
  device_name?: string
  paired_at?: string
  last_ping_ok?: boolean
  last_ping_at?: string
  last_error?: string
  secrets_ready: boolean
}

type Step = 'overview' | 'risk' | 'form' | 'working'

function Meta({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
      <div className="text-muted-foreground">{label}</div>
      <div className="min-w-0 break-all text-right font-medium">{value}</div>
    </div>
  )
}

export function FWAppSettingsPage({
  mod,
  onBack,
  onToggle,
}: {
  mod: ModuleInfo
  onBack: () => void
  onToggle: (on: boolean) => void
}) {
  const [status, setStatus] = useState<FWAppStatus | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [checking, setChecking] = useState(false)
  const [step, setStep] = useState<Step>('overview')
  const [ack, setAck] = useState(false)
  const [qrJson, setQrJson] = useState('')
  const [boxIP, setBoxIP] = useState('')
  const [email, setEmail] = useState('')
  const [deviceName, setDeviceName] = useState('FireProxy')
  const pingInFlight = useRef(false)

  const load = useCallback(async () => {
    try {
      const r = await api('/v1/fw-app/status')
      if (!r.ok) throw new Error(`status ${r.status}`)
      setStatus((await r.json()) as FWAppStatus)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed')
    }
  }, [])

  const probe = useCallback(async () => {
    if (pingInFlight.current) return
    pingInFlight.current = true
    setChecking(true)
    try {
      const r = await api('/v1/fw-app/ping', { method: 'POST' })
      const body = (await r.json()) as FWAppStatus & { error?: string; status?: FWAppStatus }
      if (body.status) setStatus(body.status)
      else if (r.ok) setStatus(body)
      else await load()
      if (!r.ok) setError(body.error || `ping ${r.status}`)
      else setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'ping failed')
      void load()
    } finally {
      pingInFlight.current = false
      setChecking(false)
    }
  }, [load])

  useEffect(() => {
    if (!mod.enabled || step !== 'overview') return
    let cancelled = false
    void (async () => {
      await load()
      if (cancelled) return
      const r = await api('/v1/fw-app/status')
      if (cancelled || !r.ok) return
      const st = (await r.json()) as FWAppStatus
      setStatus(st)
      if (st.paired && st.secrets_ready) void probe()
    })()
    const id = window.setInterval(() => {
      void load()
    }, STATUS_POLL_MS)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [mod.enabled, step, load, probe])

  async function pair() {
    setBusy(true)
    setError(null)
    setStep('working')
    try {
      const parsed = qrJson.trim()
      try {
        JSON.parse(parsed)
      } catch {
        throw new Error('QR must be valid JSON')
      }
      if (!boxIP.trim()) throw new Error('Box LAN IP required')
      if (!email.trim()) throw new Error('Email required')
      const r = await api('/v1/fw-app/pair', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          qr_json: parsed,
          box_ip: boxIP.trim(),
          email: email.trim(),
          device_name: deviceName.trim() || 'FireProxy',
        }),
      })
      const body = (await r.json()) as FWAppStatus & { error?: string; status?: FWAppStatus }
      if (!r.ok) {
        if (body.status) setStatus(body.status)
        throw new Error(body.error || `pair ${r.status}`)
      }
      setStatus(body)
      setStep('overview')
      setAck(false)
      setQrJson('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'pair failed')
      setStep('form')
    } finally {
      setBusy(false)
    }
  }

  async function unpair() {
    if (!window.confirm('Remove local Firewalla control credentials?')) return
    setBusy(true)
    setError(null)
    try {
      const r = await api('/v1/fw-app/pair', { method: 'DELETE' })
      if (!r.ok) throw new Error(`unpair ${r.status}`)
      setStatus((await r.json()) as FWAppStatus)
      setStep('overview')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'unpair failed')
    } finally {
      setBusy(false)
    }
  }

  const stateLabel = !mod.enabled
    ? 'Off'
    : checking && status?.paired
      ? 'Checking…'
      : status?.state === 'lan-ok'
        ? 'LAN OK'
        : status?.state === 'lan-down'
          ? 'LAN down'
          : status?.state === 'ready'
            ? 'Paired'
            : status?.state === 'error'
              ? 'Error'
              : 'Not paired'

  const statusDot =
    !mod.enabled
      ? 'bg-muted-foreground/50'
      : checking
        ? 'animate-pulse bg-amber-400'
        : status?.state === 'lan-ok'
          ? 'bg-emerald-500'
          : status?.state === 'lan-down' || status?.state === 'error'
            ? 'bg-red-500'
            : 'bg-muted-foreground/50'

  const wizard = step === 'risk' || step === 'form' || step === 'working'

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          onClick={() => {
            if (wizard) {
              setStep('overview')
              setAck(false)
              return
            }
            onBack()
          }}
        >
          <ArrowLeft className="size-4" />
          Back
        </button>
        <h1 className="text-lg font-semibold tracking-tight">{mod.label}</h1>
      </div>
      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {status && !status.secrets_ready ? (
        <p className="text-sm text-destructive">FIREPROXY_SECRETS_KEY required to store credentials</p>
      ) : null}

      {step === 'risk' ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Before you pair</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 px-6 pb-6">
            <ul className="list-disc space-y-1.5 pl-5 text-sm text-muted-foreground">
              <li>Uses Firewalla’s App API (not Redis); can change firewall behavior.</li>
              <li>Unpair here only clears FireProxy secrets.</li>
              <li>After pair, commands go to the box on LAN :8833.</li>
            </ul>
            <label className="flex items-start gap-2 text-sm">
              <input
                type="checkbox"
                className="mt-1"
                checked={ack}
                onChange={(e) => setAck(e.target.checked)}
              />
              <span>I understand and want to continue</span>
            </label>
            <div className="flex justify-end gap-2">
              <Button type="button" size="xs" variant="outline" onClick={() => setStep('overview')}>
                Cancel
              </Button>
              <Button type="button" size="xs" disabled={!ack} onClick={() => setStep('form')}>
                Continue
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : step === 'form' || step === 'working' ? (
        <Card className="gap-0 py-0">
          <CardHeader className={SECTION_HEADER}>
            <CardTitle className={SECTION_TITLE}>Pair</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 px-6 pb-6">
            <label className="block space-y-1.5 text-sm">
              <span className="text-muted-foreground">QR JSON</span>
              <textarea
                className="min-h-28 w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs shadow-xs dark:bg-input/30"
                value={qrJson}
                onChange={(e) => setQrJson(e.target.value)}
                placeholder='{"gid":"...","seed":"...","license":"...","ek":"..."}'
                disabled={busy}
              />
            </label>
            <label className="block space-y-1.5 text-sm">
              <span className="text-muted-foreground">Box LAN IP</span>
              <Input
                value={boxIP}
                onChange={(e) => setBoxIP(e.target.value)}
                placeholder="192.168.1.1"
                disabled={busy}
              />
            </label>
            <label className="block space-y-1.5 text-sm">
              <span className="text-muted-foreground">Email</span>
              <Input
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                type="email"
                disabled={busy}
              />
            </label>
            <label className="block space-y-1.5 text-sm">
              <span className="text-muted-foreground">Device name</span>
              <Input
                value={deviceName}
                onChange={(e) => setDeviceName(e.target.value)}
                disabled={busy}
              />
            </label>
            <div className="flex justify-end gap-2 pt-1">
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={busy}
                onClick={() => {
                  setStep('overview')
                  setAck(false)
                }}
              >
                Cancel
              </Button>
              <Button type="button" size="xs" disabled={busy} onClick={() => void pair()}>
                {busy ? 'Pairing…' : 'Pair'}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card className="gap-0 py-0">
          <CardContent className="divide-y px-0">
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Enabled</div>
              <div className="flex items-center justify-end">
                <Switch checked={mod.enabled} onCheckedChange={onToggle} disabled={busy} />
              </div>
            </div>
            <Meta
              label="Status"
              value={
                <span className="inline-flex items-center justify-end gap-2">
                  <span className={cn('size-2 shrink-0 rounded-full', statusDot)} />
                  {stateLabel}
                </span>
              }
            />
            {mod.enabled && status?.box_ip ? <Meta label="Box IP" value={status.box_ip} /> : null}
            {mod.enabled && status?.gid_hint ? (
              <Meta label="GID" value={<span className="font-mono text-xs">{status.gid_hint}</span>} />
            ) : null}
            {mod.enabled && status?.email ? <Meta label="Email" value={status.email} /> : null}
            {mod.enabled && status?.device_name ? (
              <Meta label="Device" value={status.device_name} />
            ) : null}
            {mod.enabled && status?.last_error && status.state !== 'lan-ok' ? (
              <Meta label="Error" value={<span className="text-destructive">{status.last_error}</span>} />
            ) : null}
            {mod.enabled && status?.secrets_ready ? (
              <div className="flex flex-wrap items-center justify-end gap-2 px-6 py-3">
                {!status.paired ? (
                  <Button type="button" size="xs" disabled={busy} onClick={() => setStep('risk')}>
                    Pair
                  </Button>
                ) : (
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={busy}
                    onClick={() => void unpair()}
                  >
                    Unpair
                  </Button>
                )}
              </div>
            ) : null}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
