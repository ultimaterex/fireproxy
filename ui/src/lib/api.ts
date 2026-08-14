export const AUTH_EVENT = 'fireproxy-auth'

/** Credentialed fetch; 401 (except login) dispatches `fireproxy-auth`. */
export async function api(path: string, init?: RequestInit): Promise<Response> {
  const r = await fetch(path, { ...init, credentials: 'include' })
  if (r.status === 401 && !path.startsWith('/v1/auth/login')) {
    window.dispatchEvent(new CustomEvent(AUTH_EVENT))
  }
  return r
}
