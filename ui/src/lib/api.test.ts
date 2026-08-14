import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api, AUTH_EVENT } from './api'

describe('api', () => {
  beforeEach(() => {
    vi.stubGlobal('window', new EventTarget())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends credentials: include', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('ok', { status: 200 }))
    await api('/v1/health')
    expect(fetch).toHaveBeenCalledWith(
      '/v1/health',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('dispatches fireproxy-auth on 401', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('no', { status: 401 }))
    const spy = vi.fn()
    window.addEventListener(AUTH_EVENT, spy)
    await api('/v1/devices')
    expect(spy).toHaveBeenCalledTimes(1)
    window.removeEventListener(AUTH_EVENT, spy)
  })

  it('does not dispatch for login 401', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('no', { status: 401 }))
    const spy = vi.fn()
    window.addEventListener(AUTH_EVENT, spy)
    await api('/v1/auth/login', { method: 'POST' })
    expect(spy).not.toHaveBeenCalled()
    window.removeEventListener(AUTH_EVENT, spy)
  })

  it('does not dispatch for login-options 401', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response('no', { status: 401 }))
    const spy = vi.fn()
    window.addEventListener(AUTH_EVENT, spy)
    await api('/v1/auth/login-options')
    expect(spy).not.toHaveBeenCalled()
    window.removeEventListener(AUTH_EVENT, spy)
  })
})
