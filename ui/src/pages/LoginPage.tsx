import { useEffect, useState, type FormEvent } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

export function LoginPage({
  oidcEnabled: oidcProp,
  oidcName: oidcNameProp,
  onAuthed,
}: {
  oidcEnabled?: boolean
  oidcName?: string
  onAuthed: () => void
}) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [notice] = useState<string | null>(() => {
    const n = sessionStorage.getItem('fp_auth_notice')
    if (n) sessionStorage.removeItem('fp_auth_notice')
    return n
  })
  const [busy, setBusy] = useState(false)
  const [oidcEnabled, setOidcEnabled] = useState(!!oidcProp)
  const [oidcName, setOidcName] = useState(oidcNameProp?.trim() || '')

  useEffect(() => {
    if (oidcProp != null) {
      setOidcEnabled(oidcProp)
      setOidcName(oidcNameProp?.trim() || '')
      return
    }
    let cancelled = false
    void api('/v1/auth/login-options').then(async (r) => {
      if (!r.ok || cancelled) return
      const body = (await r.json()) as { oidc_enabled?: boolean; oidc_name?: string }
      if (!cancelled) {
        setOidcEnabled(!!body.oidc_enabled)
        setOidcName(body.oidc_name?.trim() || '')
      }
    })
    return () => {
      cancelled = true
    }
  }, [oidcProp, oidcNameProp])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const r = await api('/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: 'admin', password }),
      })
      if (!r.ok) {
        setError(r.status === 401 ? 'Invalid password' : `${r.status}`)
        return
      }
      onAuthed()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background px-4">
      <form onSubmit={onSubmit} className="w-full max-w-xs space-y-4">
        <div className="text-lg font-semibold tracking-tight">FireProxy</div>
        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground" htmlFor="fp-user">
            User
          </label>
          <Input id="fp-user" value="admin" readOnly disabled />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground" htmlFor="fp-pass">
            Password
          </label>
          <Input
            id="fp-pass"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
            required
          />
        </div>
        {notice ? <p className="text-sm text-muted-foreground">{notice}</p> : null}
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={busy}>
          Sign in
        </Button>
        {oidcEnabled ? (
          <Button
            type="button"
            variant="outline"
            className="w-full"
            onClick={() => {
              window.location.assign('/v1/auth/oidc/start')
            }}
          >
            {oidcName || 'OIDC'}
          </Button>
        ) : null}
      </form>
    </div>
  )
}
