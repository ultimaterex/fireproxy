const KEY = 'fireproxy.audit.snooze.offline'
const DEFAULT_MS = 7 * 24 * 60 * 60 * 1000

type Store = Record<string, number> // mac upper -> until epoch ms

function read(): Store {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Store
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed
  } catch {
    return {}
  }
}

function write(store: Store) {
  try {
    localStorage.setItem(KEY, JSON.stringify(store))
  } catch {
    /* ignore */
  }
}

function prune(store: Store, now = Date.now()): Store {
  const out: Store = {}
  for (const [mac, until] of Object.entries(store)) {
    if (typeof until === 'number' && until > now) out[mac] = until
  }
  return out
}

export function isOfflineSnoozed(mac: string, now = Date.now()): boolean {
  const k = mac.trim().toUpperCase()
  if (!k) return false
  const until = prune(read(), now)[k]
  return until != null && until > now
}

export function snoozeOffline(mac: string, ms = DEFAULT_MS, now = Date.now()): void {
  const k = mac.trim().toUpperCase()
  if (!k) return
  const store = prune(read(), now)
  store[k] = now + ms
  write(store)
}

export function unsnoozeOffline(mac: string): void {
  const k = mac.trim().toUpperCase()
  if (!k) return
  const store = prune(read())
  delete store[k]
  write(store)
}

/** Drop snoozes for MACs no longer offline (finding cleared). */
export function syncOfflineSnoozes(liveMacs: string[], now = Date.now()): void {
  const live = new Set(liveMacs.map((m) => m.trim().toUpperCase()).filter(Boolean))
  const store = prune(read(), now)
  let changed = false
  for (const mac of Object.keys(store)) {
    if (!live.has(mac)) {
      delete store[mac]
      changed = true
    }
  }
  if (changed) write(store)
}

export function countUnsnoozedOffline(macs: string[], now = Date.now()): number {
  return macs.reduce((n, mac) => n + (isOfflineSnoozed(mac, now) ? 0 : 1), 0)
}
