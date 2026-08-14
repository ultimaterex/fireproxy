import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { createAnon } from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import type { ModuleInfo, Switch as InventorySwitch } from '@/lib/types'
import { cn } from '@/lib/utils'
import { findProduct, resolveFaceplate, resolveGenericShelf } from '@/vrack/engine'
import { Faceplate } from '@/vrack/Faceplate'

type TPLinkSwitch = {
  id: string
  ip: string
  mac?: string
  username: string
  has_password: boolean
  enabled: boolean
  last_status?: string
  last_error?: string
  last_model?: string
  last_firmware?: string
  last_ok_at?: string
  uplink_port?: string
}

type Candidate = {
  mac: string
  ip?: string
  name?: string
  vendor?: string
}

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const DEFAULT_MODEL = 'TL-SG1016PE'
const DEFAULT_PORTS = 16

function formatPoll(sec: number): string {
  if (sec < 60) return `${sec}s`
  if (sec % 60 === 0) return `${sec / 60}m`
  return `${Math.floor(sec / 60)}m${sec % 60}s`
}

function notchIndex(notches: number[], sec: number): number {
  let best = 0
  let bestDist = Math.abs(sec - (notches[0] ?? 60))
  for (let i = 1; i < notches.length; i++) {
    const d = Math.abs(sec - notches[i])
    if (d < bestDist) {
      best = i
      bestDist = d
    }
  }
  return best
}

function stubSwitch(model: string, portCount: number, uplink?: string): InventorySwitch {
  const ports = Array.from({ length: portCount }, (_, i) => {
    const id = String(i + 1)
    return { id, up: true, uplink: uplink === id }
  })
  return {
    mac: '00:00:00:00:00:00',
    name: model,
    model,
    healthy: true,
    uplink_port: uplink || undefined,
    ports,
  }
}

function UplinkPortPicker({
  model,
  value,
  onChange,
  busy,
}: {
  model?: string
  value: string
  onChange: (port: string) => void
  busy?: boolean
}) {
  const modelName = model?.trim() || DEFAULT_MODEL
  const sw = useMemo(() => stubSwitch(modelName, DEFAULT_PORTS, value || undefined), [modelName, value])
  const layout = useMemo(() => {
    const product = findProduct({ manufacturer: 'tplink', type: 'switch', model: modelName })
    return product ? resolveFaceplate(product) : resolveGenericShelf(sw, 'tplink')
  }, [modelName, sw])
  const lit = useMemo(() => (value ? new Set([value]) : undefined), [value])
  const boxRef = useRef<HTMLDivElement>(null)
  const [boxW, setBoxW] = useState(0)

  useEffect(() => {
    const el = boxRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width ?? 0
      setBoxW(w)
    })
    ro.observe(el)
    setBoxW(el.clientWidth)
    return () => ro.disconnect()
  }, [])

  const scale = boxW > 0 ? Math.min(1, boxW / layout.width) : 1

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-muted-foreground">Uplink</span>
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-muted-foreground">
            {value ? `Port ${value}` : '—'}
          </span>
          <Button
            type="button"
            size="xs"
            variant="ghost"
            disabled={!value || busy}
            onClick={() => onChange('')}
          >
            Clear
          </Button>
        </div>
      </div>
      <div
        ref={boxRef}
        className={cn('w-full overflow-hidden rounded-md border bg-background/40 p-2', busy && 'opacity-60')}
        style={{ height: layout.height * scale + 16 }}
      >
        <div
          style={{
            width: layout.width,
            height: layout.height,
            transform: `scale(${scale})`,
            transformOrigin: 'top left',
          }}
        >
          <Faceplate
            layout={layout}
            sw={sw}
            litPorts={lit}
            onSelect={(_s, port) => {
              if (busy || !port) return
              onChange(port === value ? '' : port)
            }}
          />
        </div>
      </div>
    </div>
  )
}

export function TPLinkSettingsPage({
  mod,
  onBack,
  onToggle,
}: {
  mod: ModuleInfo
  onBack: () => void
  onToggle: (enabled: boolean) => void
}) {
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])
  const anon = anonOn ? createAnon(anonymitySalt()) : null
  const [switches, setSwitches] = useState<TPLinkSwitch[]>([])
  const [candidates, setCandidates] = useState<Candidate[]>([])
  const [secretsReady, setSecretsReady] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [formIP, setFormIP] = useState('')
  const [formUser, setFormUser] = useState('admin')
  const [formPass, setFormPass] = useState('')
  const [editID, setEditID] = useState<string | null>(null)
  const [testMsg, setTestMsg] = useState<string | null>(null)
  const [showAdd, setShowAdd] = useState(false)
  const [pollSec, setPollSec] = useState(60)
  const [pollNotches, setPollNotches] = useState<number[]>([30, 45, 60, 90, 120, 180, 300, 600])

  const load = useCallback(async () => {
    try {
      const [swR, candR, setR] = await Promise.all([
        fetch('/v1/tplink/switches'),
        fetch('/v1/tplink/candidates'),
        fetch('/v1/tplink/settings'),
      ])
      if (swR.ok) {
        const body = (await swR.json()) as { switches?: TPLinkSwitch[]; secrets_ready?: boolean }
        setSwitches(body.switches ?? [])
        setSecretsReady(body.secrets_ready !== false)
      }
      if (candR.ok) {
        const body = (await candR.json()) as { candidates?: Candidate[] }
        setCandidates(body.candidates ?? [])
      }
      if (setR.ok) {
        const body = (await setR.json()) as { poll_sec?: number; poll_notches?: number[] }
        if (typeof body.poll_sec === 'number') setPollSec(body.poll_sec)
        if (body.poll_notches?.length) setPollNotches(body.poll_notches)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed')
    }
  }, [])

  async function savePoll(sec: number) {
    setBusy(true)
    setError(null)
    try {
      const r = await fetch('/v1/tplink/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ poll_sec: sec }),
      })
      if (!r.ok) throw new Error((await r.text()).trim() || `save ${r.status}`)
      const body = (await r.json()) as { poll_sec?: number; poll_notches?: number[] }
      if (typeof body.poll_sec === 'number') setPollSec(body.poll_sec)
      if (body.poll_notches?.length) setPollNotches(body.poll_notches)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    void load()
    const id = window.setInterval(() => void load(), 5000)
    return () => window.clearInterval(id)
  }, [load])

  function resetAddForm() {
    setFormIP('')
    setFormUser('admin')
    setFormPass('')
    setTestMsg(null)
    setError(null)
  }

  function startAdd(c?: Candidate) {
    setEditID(null)
    setShowAdd(true)
    setFormIP(c?.ip ?? '')
    setFormUser('admin')
    setFormPass('')
    setTestMsg(null)
    setError(null)
  }

  function startEdit(sw: TPLinkSwitch) {
    setShowAdd(false)
    setEditID(sw.id)
    setFormIP(sw.ip)
    setFormUser(sw.username || 'admin')
    setFormPass('')
    setTestMsg(null)
    setError(null)
  }

  function cancelEdit() {
    setEditID(null)
    resetAddForm()
  }

  async function patchSwitch(id: string, body: Record<string, unknown>) {
    const r = await fetch(`/v1/tplink/switches/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!r.ok) throw new Error((await r.text()).trim() || `save ${r.status}`)
  }

  async function saveUplink(id: string, port: string) {
    setBusy(true)
    setError(null)
    try {
      await patchSwitch(id, { uplink_port: port })
      setSwitches((prev) => prev.map((s) => (s.id === id ? { ...s, uplink_port: port || undefined } : s)))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
    } finally {
      setBusy(false)
    }
  }

  async function saveCreds() {
    setBusy(true)
    setError(null)
    try {
      const body: Record<string, unknown> = {
        ip: formIP.trim(),
        username: formUser.trim() || 'admin',
        enabled: true,
      }
      if (formPass) body.password = formPass
      if (editID) {
        await patchSwitch(editID, body)
        setEditID(null)
        resetAddForm()
      } else {
        if (!formPass.trim()) throw new Error('password required')
        const r = await fetch('/v1/tplink/switches', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        })
        if (!r.ok) throw new Error((await r.text()).trim() || `save ${r.status}`)
        setShowAdd(false)
        resetAddForm()
      }
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: string) {
    setBusy(true)
    setError(null)
    try {
      const r = await fetch(`/v1/tplink/switches/${id}`, { method: 'DELETE' })
      if (!r.ok && r.status !== 204) throw new Error(`delete ${r.status}`)
      if (editID === id) cancelEdit()
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'delete failed')
    } finally {
      setBusy(false)
    }
  }

  async function test(id?: string) {
    setBusy(true)
    setError(null)
    setTestMsg(null)
    try {
      const body: Record<string, string> = {
        ip: formIP.trim(),
        username: formUser.trim() || 'admin',
      }
      if (formPass) body.password = formPass
      if (id) body.id = id
      const r = await fetch('/v1/tplink/switches/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const j = (await r.json()) as { ok?: boolean; error?: string; switch?: { model?: string } }
      if (!j.ok) {
        setTestMsg(j.error || 'failed')
        return
      }
      setTestMsg(j.switch?.model || 'ok')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'test failed')
    } finally {
      setBusy(false)
    }
  }

  const editing = editID ? switches.find((s) => s.id === editID) : undefined
  const canSaveAdd = !!formIP.trim() && !!formPass.trim()
  const canSaveEdit = !!formIP.trim()

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
        <h1 className="text-lg font-semibold tracking-tight">{mod.label}</h1>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {!secretsReady && (
        <p className="text-sm text-destructive">FIREPROXY_SECRETS_KEY required to store passwords</p>
      )}

      <Card className="gap-0 py-0">
        <CardContent className="divide-y px-0">
          <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
            <div className="text-muted-foreground">Enabled</div>
            <div className="flex items-center justify-end">
              <Switch checked={mod.enabled} onCheckedChange={onToggle} />
            </div>
          </div>
          <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
            <div className="text-muted-foreground">Status</div>
            <div className="text-right font-medium">
              {!mod.enabled
                ? 'Off'
                : mod.status === 'ok'
                  ? 'Connected'
                  : mod.status === 'ready'
                    ? 'Ready'
                    : 'Down'}
            </div>
          </div>
          {mod.enabled && mod.status === 'ok' && mod.detail ? (
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Detail</div>
              <div className="min-w-0 break-all text-right font-medium">{mod.detail}</div>
            </div>
          ) : null}
          {mod.enabled && mod.status === 'down' && mod.detail ? (
            <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">Error</div>
              <div className="min-w-0 break-all text-right font-medium">{mod.detail}</div>
            </div>
          ) : null}
          <div className="grid grid-cols-[8rem_minmax(0,1fr)] items-center gap-4 px-6 py-3 text-sm">
            <div className="text-muted-foreground">Sync</div>
            <div className="flex items-center justify-end gap-2">
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={busy || notchIndex(pollNotches, pollSec) <= 0}
                onClick={() => {
                  const i = notchIndex(pollNotches, pollSec)
                  if (i > 0) void savePoll(pollNotches[i - 1])
                }}
              >
                −
              </Button>
              <span className="min-w-10 text-center font-mono tabular-nums">{formatPoll(pollSec)}</span>
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={busy || notchIndex(pollNotches, pollSec) >= pollNotches.length - 1}
                onClick={() => {
                  const i = notchIndex(pollNotches, pollSec)
                  if (i < pollNotches.length - 1) void savePoll(pollNotches[i + 1])
                }}
              >
                +
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <div className="flex items-center justify-between gap-2">
            <CardTitle className={SECTION_TITLE}>Configured</CardTitle>
            {!showAdd && !editID ? (
              <Button type="button" size="xs" variant="outline" onClick={() => startAdd()}>
                Add
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className="px-0 pb-2">
          {switches.length === 0 && !showAdd ? (
            <p className="px-6 py-4 text-sm text-muted-foreground">None</p>
          ) : (
            <div className="divide-y">
              {switches.map((sw) => {
                const open = editID === sw.id
                const ip = anon && sw.ip ? anon.fakeIP(sw.ip) : sw.ip
                const mac = anon && sw.mac ? anon.fakeMAC(sw.mac) : sw.mac
                return (
                  <div key={sw.id} className="px-6 py-3">
                    <div className="flex items-center gap-3 text-sm">
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">{sw.last_model || ip}</div>
                        <div className="truncate font-mono text-xs text-muted-foreground">
                          {ip}
                          {mac ? ` · ${mac}` : ''}
                          {sw.uplink_port ? ` · uplink ${sw.uplink_port}` : ''}
                          {sw.last_status ? ` · ${sw.last_status}` : ''}
                        </div>
                      </div>
                      <Button
                        type="button"
                        size="xs"
                        variant={open ? 'secondary' : 'outline'}
                        disabled={busy}
                        onClick={() => (open ? cancelEdit() : startEdit(sw))}
                      >
                        {open ? 'Close' : 'Edit'}
                      </Button>
                      <Button type="button" size="xs" variant="ghost" disabled={busy} onClick={() => void remove(sw.id)}>
                        Remove
                      </Button>
                    </div>
                    <div className="mt-3">
                      <UplinkPortPicker
                        model={sw.last_model}
                        value={sw.uplink_port ?? ''}
                        busy={busy}
                        onChange={(port) => void saveUplink(sw.id, port)}
                      />
                    </div>
                    {open && editing ? (
                      <div className="mt-3 space-y-3 border-t pt-3">
                        <div className="grid gap-3 sm:grid-cols-3">
                          <label className="text-sm">
                            <span className="mb-1 block text-muted-foreground">IP</span>
                            <input
                              className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                              value={formIP}
                              onChange={(e) => setFormIP(e.target.value)}
                            />
                          </label>
                          <label className="text-sm">
                            <span className="mb-1 block text-muted-foreground">Username</span>
                            <input
                              className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                              value={formUser}
                              onChange={(e) => setFormUser(e.target.value)}
                            />
                          </label>
                          <label className="text-sm">
                            <span className="mb-1 block text-muted-foreground">Password</span>
                            <input
                              type="password"
                              className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                              value={formPass}
                              onChange={(e) => setFormPass(e.target.value)}
                              placeholder="unchanged"
                            />
                          </label>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <Button
                            type="button"
                            size="xs"
                            disabled={busy || !canSaveEdit}
                            onClick={() => void saveCreds()}
                          >
                            Save
                          </Button>
                          <Button
                            type="button"
                            size="xs"
                            variant="outline"
                            disabled={busy || !formIP.trim()}
                            onClick={() => void test(sw.id)}
                          >
                            Test
                          </Button>
                          {testMsg ? <span className="text-sm text-muted-foreground">{testMsg}</span> : null}
                        </div>
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          )}

          {showAdd ? (
            <div className="space-y-3 border-t px-6 py-4">
              <div className="text-sm font-medium">Add switch</div>
              <div className="grid gap-3 sm:grid-cols-3">
                <label className="text-sm">
                  <span className="mb-1 block text-muted-foreground">IP</span>
                  <input
                    className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                    value={formIP}
                    onChange={(e) => setFormIP(e.target.value)}
                    placeholder="203.0.113.10"
                  />
                </label>
                <label className="text-sm">
                  <span className="mb-1 block text-muted-foreground">Username</span>
                  <input
                    className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                    value={formUser}
                    onChange={(e) => setFormUser(e.target.value)}
                  />
                </label>
                <label className="text-sm">
                  <span className="mb-1 block text-muted-foreground">Password</span>
                  <input
                    type="password"
                    className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                    value={formPass}
                    onChange={(e) => setFormPass(e.target.value)}
                  />
                </label>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button type="button" size="xs" disabled={busy || !canSaveAdd} onClick={() => void saveCreds()}>
                  Save
                </Button>
                <Button
                  type="button"
                  size="xs"
                  variant="outline"
                  disabled={busy || !formIP.trim() || !formPass.trim()}
                  onClick={() => void test()}
                >
                  Test
                </Button>
                <Button
                  type="button"
                  size="xs"
                  variant="ghost"
                  onClick={() => {
                    setShowAdd(false)
                    resetAddForm()
                  }}
                >
                  Cancel
                </Button>
                {testMsg ? <span className="text-sm text-muted-foreground">{testMsg}</span> : null}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Detected</CardTitle>
        </CardHeader>
        <CardContent className="px-0 pb-2">
          {candidates.length === 0 ? (
            <p className="px-6 py-4 text-sm text-muted-foreground">None</p>
          ) : (
            <div className="divide-y">
              {candidates.map((c) => {
                const mac = anon ? anon.fakeMAC(c.mac) : c.mac
                const ip = anon && c.ip ? anon.fakeIP(c.ip) : c.ip
                const name = c.name && anon ? anon.fakeName('device', c.name) : c.name
                const vendor = c.vendor && anon ? anon.fakeName('vendor', c.vendor) : c.vendor
                return (
                <div key={c.mac} className="flex items-center gap-3 px-6 py-2.5 text-sm">
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium">{name || vendor || 'TP-Link'}</div>
                    <div className="truncate font-mono text-xs text-muted-foreground">
                      {mac}
                      {ip ? ` · ${ip}` : ''}
                    </div>
                  </div>
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={!c.ip}
                    onClick={() => startAdd(c)}
                  >
                    Configure
                  </Button>
                </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
