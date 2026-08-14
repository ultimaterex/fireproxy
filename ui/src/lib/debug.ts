declare global {
  interface Window {
    __FIREPROXY__?: { debug?: boolean }
  }
}

/** True when compose `DEBUG=1` wrote config.js, or Vite `VITE_DEBUG=1`. */
export function isDebugEnabled(): boolean {
  const v = window.__FIREPROXY__?.debug as unknown
  if (v === true || v === 1 || v === '1') return true
  if (import.meta.env.VITE_DEBUG === '1' || import.meta.env.VITE_DEBUG === 'true') return true
  return false
}
