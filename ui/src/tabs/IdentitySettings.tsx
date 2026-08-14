import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, ChevronDown } from 'lucide-react'

import { CopyButton, CopyText } from '@/components/CopyText'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { api, AUTH_EVENT } from '@/lib/api'
import { fmtTime } from '@/lib/format'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const TABLE = 'grid w-full gap-x-3 divide-y'
const TABLE_HEAD =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2 text-xs text-muted-foreground'
const TABLE_ROW = 'col-span-full grid grid-cols-subgrid items-center px-6 py-2.5 text-sm'
const KEY_COLS = 'grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_7rem_7rem_4.5rem]'
const AGENT_COLS = 'grid-cols-[minmax(0,1fr)_7rem_7rem_4.5rem]'

type OIDCSettings = {
  name?: string
  issuer: string
  client_id: string
  redirect_uri: string
  allowlist: string[]
  secret_set: boolean
}

type APIKeyRow = {
  id: string
  name: string
  scopes: string[]
  created_at: number
  last_used_at?: number | null
}

type AgentCred = {
  id: string
  created_at: number
  last_used_at?: number | null
}

const SCOPES = ['read', 'write', 'admin'] as const

function parseAllowlist(text: string): string[] {
  return text
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export function IdentitySettings({ onBack }: { onBack: () => void }) {
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [oidcOpen, setOidcOpen] = useState(false)

  const [oidcName, setOidcName] = useState('')
  const [issuer, setIssuer] = useState('')
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [redirectURI, setRedirectURI] = useState(
    () => `${window.location.origin}/v1/auth/oidc/callback`,
  )
  const [allowlistText, setAllowlistText] = useState('')
  const [secretSet, setSecretSet] = useState(false)

  const [keyName, setKeyName] = useState('')
  const [scopes, setScopes] = useState<Record<(typeof SCOPES)[number], boolean>>({
    read: true,
    write: false,
    admin: false,
  })
  const [keys, setKeys] = useState<APIKeyRow[] | null>(null)
  const [newToken, setNewToken] = useState<string | null>(null)

  const [agents, setAgents] = useState<AgentCred[] | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const [oidcR, keysR, agentsR] = await Promise.all([
        api('/v1/auth/oidc'),
        api('/v1/auth/api-keys'),
        api('/v1/auth/agent-credentials'),
      ])
      if (!oidcR.ok) throw new Error((await oidcR.text()) || `oidc ${oidcR.status}`)
      if (!keysR.ok) throw new Error((await keysR.text()) || `api-keys ${keysR.status}`)
      if (!agentsR.ok) throw new Error((await agentsR.text()) || `agents ${agentsR.status}`)

      const oidc = (await oidcR.json()) as OIDCSettings
      setOidcName(oidc.name ?? '')
      setIssuer(oidc.issuer ?? '')
      setClientId(oidc.client_id ?? '')
      setRedirectURI(
        oidc.redirect_uri?.trim() || `${window.location.origin}/v1/auth/oidc/callback`,
      )
      setAllowlistText((oidc.allowlist ?? []).join('\n'))
      setSecretSet(!!oidc.secret_set)
      setClientSecret('')

      const keysBody = (await keysR.json()) as { keys?: APIKeyRow[] }
      setKeys(keysBody.keys ?? [])

      const agentsBody = (await agentsR.json()) as { credentials?: AgentCred[] }
      setAgents(agentsBody.credentials ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed')
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  async function saveOIDC() {
    setBusy(true)
    setError(null)
    try {
      const body: Record<string, unknown> = {
        name: oidcName.trim(),
        issuer: issuer.trim(),
        client_id: clientId.trim(),
        redirect_uri: redirectURI.trim(),
        allowlist: parseAllowlist(allowlistText),
      }
      if (clientSecret.trim()) body.client_secret = clientSecret.trim()
      const r = await api('/v1/auth/oidc', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!r.ok) throw new Error((await r.text()) || `save ${r.status}`)
      sessionStorage.setItem('fp_auth_notice', 'OIDC saved — sign in again')
      window.dispatchEvent(new CustomEvent(AUTH_EVENT))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'save failed')
      setBusy(false)
    }
  }

  async function createKey() {
    setBusy(true)
    setError(null)
    setNewToken(null)
    try {
      const selected = SCOPES.filter((s) => scopes[s])
      const r = await api('/v1/auth/api-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: keyName.trim(), scopes: selected }),
      })
      if (!r.ok) throw new Error((await r.text()) || `create ${r.status}`)
      const body = (await r.json()) as APIKeyRow & { token?: string }
      if (body.token) setNewToken(body.token)
      setKeyName('')
      setScopes({ read: true, write: false, admin: false })
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'create failed')
    } finally {
      setBusy(false)
    }
  }

  async function revokeKey(id: string) {
    setBusy(true)
    setError(null)
    try {
      const r = await api(`/v1/auth/api-keys/${encodeURIComponent(id)}`, { method: 'DELETE' })
      if (!r.ok && r.status !== 204) throw new Error((await r.text()) || `revoke ${r.status}`)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'revoke failed')
    } finally {
      setBusy(false)
    }
  }

  async function revokeAgent(id: string) {
    setBusy(true)
    setError(null)
    try {
      const r = await api(`/v1/auth/agent-credentials/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (!r.ok && r.status !== 204) throw new Error((await r.text()) || `revoke ${r.status}`)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'revoke failed')
    } finally {
      setBusy(false)
    }
  }

  const adminChecked = scopes.admin
  const canCreateKey = keyName.trim().length > 0 && SCOPES.some((s) => scopes[s])
  const oidcConfigured = Boolean(issuer.trim() && clientId.trim())

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
        <h1 className="text-lg font-semibold tracking-tight">Identity & Access</h1>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>API keys</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 px-0 pb-4">
          <div className="space-y-3 px-6">
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
              <label className="text-sm">
                <span className="mb-1 block text-muted-foreground">Name</span>
                <Input value={keyName} onChange={(e) => setKeyName(e.target.value)} />
              </label>
              <div className="flex flex-col justify-end gap-2">
                <div className="flex flex-wrap items-center gap-4 text-sm">
                  {SCOPES.map((s) => (
                    <label key={s} className="inline-flex items-center gap-2">
                      <input
                        type="checkbox"
                        className="size-4 rounded border-input"
                        checked={scopes[s]}
                        onChange={(e) =>
                          setScopes((prev) => ({ ...prev, [s]: e.target.checked }))
                        }
                      />
                      <span>{s}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
            {adminChecked ? (
              <p className="text-sm text-destructive">
                Admin keys can mint agents, push updates, and manage IAM.
              </p>
            ) : null}
            <div className="flex justify-end">
              <Button
                type="button"
                size="xs"
                disabled={busy || !canCreateKey}
                onClick={() => void createKey()}
              >
                Create
              </Button>
            </div>
            {newToken ? (
              <div className="flex flex-wrap items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
                <span className="text-muted-foreground">Secret</span>
                <CopyText value={newToken} mono />
                <CopyButton value={newToken} />
                <Button type="button" size="xs" variant="ghost" onClick={() => setNewToken(null)}>
                  Dismiss
                </Button>
              </div>
            ) : null}
          </div>

          {!keys || keys.length === 0 ? (
            <p className="px-6 py-2 text-sm text-muted-foreground">No keys</p>
          ) : (
            <div className={cn(TABLE, KEY_COLS)}>
              <div className={TABLE_HEAD}>
                <div>Name</div>
                <div>Scopes</div>
                <div>Created</div>
                <div>Last used</div>
                <div />
              </div>
              {keys.map((k) => (
                <div key={k.id} className={TABLE_ROW}>
                  <div className="min-w-0 truncate font-medium">{k.name}</div>
                  <div className="min-w-0 truncate text-muted-foreground">
                    {(k.scopes ?? []).join(', ') || '—'}
                  </div>
                  <div className="text-muted-foreground">{fmtTime(k.created_at)}</div>
                  <div className="text-muted-foreground">
                    {k.last_used_at ? fmtTime(k.last_used_at) : '—'}
                  </div>
                  <div className="flex justify-end">
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void revokeKey(k.id)}
                    >
                      Revoke
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Agents</CardTitle>
        </CardHeader>
        <CardContent className="px-0 pb-4">
          {!agents || agents.length === 0 ? (
            <p className="px-6 py-2 text-sm text-muted-foreground">No credentials</p>
          ) : (
            <div className={cn(TABLE, AGENT_COLS)}>
              <div className={TABLE_HEAD}>
                <div>ID</div>
                <div>Created</div>
                <div>Last used</div>
                <div />
              </div>
              {agents.map((c) => (
                <div key={c.id} className={TABLE_ROW}>
                  <div className="min-w-0 truncate font-mono text-xs">{c.id}</div>
                  <div className="text-muted-foreground">{fmtTime(c.created_at)}</div>
                  <div className="text-muted-foreground">
                    {c.last_used_at ? fmtTime(c.last_used_at) : '—'}
                  </div>
                  <div className="flex justify-end">
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      disabled={busy}
                      onClick={() => void revokeAgent(c.id)}
                    >
                      Revoke
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <button
          type="button"
          className="flex w-full items-center justify-between px-6 py-4 text-left"
          aria-expanded={oidcOpen}
          onClick={() => setOidcOpen((o) => !o)}
        >
          <span className={SECTION_TITLE}>OIDC</span>
          <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
            {oidcConfigured ? (oidcName.trim() || 'Configured') : 'Optional'}
            <ChevronDown
              className={cn('size-4 transition-transform', oidcOpen && 'rotate-180')}
            />
          </span>
        </button>
        {oidcOpen ? (
          <CardContent className="space-y-4 border-t px-6 py-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="text-sm">
                <span className="mb-1 block text-muted-foreground">Name</span>
                <Input
                  value={oidcName}
                  onChange={(e) => setOidcName(e.target.value)}
                />
              </label>
              <label className="text-sm">
                <span className="mb-1 block text-muted-foreground">Issuer URL</span>
                <Input
                  value={issuer}
                  onChange={(e) => setIssuer(e.target.value)}
                  placeholder="https://auth.example.com"
                />
              </label>
              <label className="text-sm">
                <span className="mb-1 block text-muted-foreground">Client ID</span>
                <Input value={clientId} onChange={(e) => setClientId(e.target.value)} />
              </label>
              <label className="text-sm">
                <span className="mb-1 block text-muted-foreground">Client secret</span>
                <Input
                  type="password"
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                  placeholder={secretSet ? 'unchanged' : ''}
                  autoComplete="off"
                />
              </label>
              <label className="text-sm sm:col-span-2">
                <span className="mb-1 block text-muted-foreground">Redirect URI</span>
                <Input value={redirectURI} onChange={(e) => setRedirectURI(e.target.value)} />
              </label>
              <label className="text-sm sm:col-span-2">
                <span className="mb-1 block text-muted-foreground">Allowlist</span>
                <textarea
                  className="min-h-24 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30"
                  value={allowlistText}
                  onChange={(e) => setAllowlistText(e.target.value)}
                  placeholder="you@example.com"
                />
              </label>
            </div>
            <div className="flex justify-end">
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={busy}
                onClick={() => void saveOIDC()}
              >
                Save & sign out
              </Button>
            </div>
          </CardContent>
        ) : null}
      </Card>
    </div>
  )
}
