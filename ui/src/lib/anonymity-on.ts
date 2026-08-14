const ON_KEY = 'fireproxy.anonymity.on'
const SALT_KEY = 'fireproxy.anonymity.salt'

export function isAnonymityOn(): boolean {
  try {
    return localStorage.getItem(ON_KEY) === '1'
  } catch {
    return false
  }
}

export function setAnonymityOn(on: boolean) {
  try {
    localStorage.setItem(ON_KEY, on ? '1' : '0')
  } catch {
    // Storage blocked (privacy mode / quota). Anonymity is display-only, so
    // still notify subscribers rather than letting the write break the UI.
  }
  window.dispatchEvent(new Event('fireproxy-anonymity'))
}

export function anonymitySalt(): string {
  try {
    let s = localStorage.getItem(SALT_KEY)
    if (s && s.length >= 16) return s
    const buf = new Uint8Array(16)
    crypto.getRandomValues(buf)
    s = [...buf].map((b) => b.toString(16).padStart(2, '0')).join('')
    localStorage.setItem(SALT_KEY, s)
    return s
  } catch {
    return 'dev-salt-not-random'
  }
}

export function subscribeAnonymity(cb: () => void): () => void {
  const fn = () => cb()
  window.addEventListener('fireproxy-anonymity', fn)
  window.addEventListener('storage', fn)
  return () => {
    window.removeEventListener('fireproxy-anonymity', fn)
    window.removeEventListener('storage', fn)
  }
}
