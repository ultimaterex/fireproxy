import {
  DAY,
  HOUR,
  MIN,
  type Device,
  type Load,
  type NetIface,
  type Tag,
  type WANLink,
} from '@/lib/types'

export function fmtBytes(v: number | null | undefined): string {
  if (v == null || Number.isNaN(v)) return '—'
  const abs = Math.abs(v)
  if (abs >= 1e12) return `${(v / 1e12).toFixed(2)} TB`
  if (abs >= 1e9) return `${(v / 1e9).toFixed(2)} GB`
  if (abs >= 1e6) return `${(v / 1e6).toFixed(2)} MB`
  if (abs >= 1e3) return `${(v / 1e3).toFixed(1)} KB`
  return `${Math.round(v)} B`
}

export function fmtCount(v: number | null | undefined): string {
  if (v == null || Number.isNaN(v)) return '—'
  return Math.round(v).toLocaleString()
}

export function fmtMbps(v: number | null | undefined): string {
  if (v == null || Number.isNaN(v)) return '—'
  if (v >= 100) return v.toFixed(0)
  if (v >= 10) return v.toFixed(1)
  return v.toFixed(2)
}

export function fmtPlanMbps(v: number | null | undefined): string {
  if (v == null || Number.isNaN(v) || v <= 0) return '—'
  if (v >= 1000) {
    const g = v / 1000
    const s = Number.isInteger(g) ? String(g) : String(g).replace(/\.0$/, '')
    return `${s} Gbps`
  }
  return `${Math.round(v)} Mbps`
}

export function fmtPlan(down?: number, up?: number): string {
  if (!down && !up) return ''
  return `${fmtPlanMbps(down)} / ${fmtPlanMbps(up)}`
}

export function fmtTime(ts: number): string {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

export function cpuBusy(cpu: { idle: number }): number {
  if (cpu.idle == null || Number.isNaN(cpu.idle)) return 0
  return Math.max(0, Math.min(100, 100 - cpu.idle))
}

export function cpuTone(pct: number): 'ok' | 'warn' | 'bad' {
  if (pct >= 90) return 'bad'
  if (pct >= 70) return 'warn'
  return 'ok'
}

export function fmtRelative(tsSec: number, nowMs: number): string {
  const diff = nowMs - tsSec * 1000
  if (diff < 0) return 'now'
  if (diff < MIN) return `${Math.max(1, Math.floor(diff / 1000))}s`
  if (diff < HOUR) return `${Math.floor(diff / MIN)}m`
  if (diff < DAY) return `${Math.floor(diff / HOUR)}h`
  if (diff < 7 * DAY) return `${Math.floor(diff / DAY)}d`
  if (diff < 30 * DAY) return `${Math.floor(diff / (7 * DAY))}w`
  return `${Math.floor(diff / (30 * DAY))}mo`
}

export function preferredName(d: Device): string {
  return d.name?.trim() || d.local_domain?.trim() || d.mac
}

/** Firewalla treats last-seen within 15 minutes as online. */
export const ONLINE_WINDOW_SEC = 15 * 60

export function deviceOnline(d: Device, nowMs: number): boolean {
  if (!d.last_active_ts) return false
  return nowMs / 1000 - d.last_active_ts < ONLINE_WINDOW_SEC
}

export function ifaceLabel(
  name: string,
  network: NetIface[],
  wan: Record<string, WANLink> | undefined,
): string {
  const w = wan?.[name]
  if (w?.name) return `${w.name} (${name})`
  const n = network.find((x) => x.name === name)
  if (n?.desc) return `${n.desc} (${name})`
  return name
}

export type LoadLevel = 'ok' | 'elevated' | 'high'

export function loadPressure(load: Load): {
  level: LoadLevel
  label: string
  pct: number | null
  cores: number | null
} {
  const cores = load.cores && load.cores > 0 ? load.cores : null
  const pct = cores ? (load.m1 / cores) * 100 : null
  if (pct != null) {
    if (pct >= 100) return { level: 'high', label: 'High', pct, cores }
    if (pct >= 70) return { level: 'elevated', label: 'Elevated', pct, cores }
    return { level: 'ok', label: 'Normal', pct, cores }
  }
  if (load.m1 >= 3) return { level: 'high', label: 'High', pct: null, cores: null }
  if (load.m1 >= 1.5)
    return { level: 'elevated', label: 'Elevated', pct: null, cores: null }
  return { level: 'ok', label: 'Normal', pct: null, cores: null }
}

export function formatTagName(t: Tag): string {
  const name = t.name?.trim() || t.id
  if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(name)) {
    return `group ${t.id}`
  }
  return name
}

/** Map affiliated device-group id → owning user tag (Firewalla user creates a native group). */
export function affiliatedUserByGroup(tags: Tag[]): Map<string, Tag> {
  const out = new Map<string, Tag>()
  for (const t of tags) {
    if (t.type !== 'user') continue
    const af = t.affiliated_tag?.trim()
    if (af) out.set(af, t)
  }
  return out
}

/** Display name for a tag; affiliated groups inherit the parent user name. */
export function resolveTagName(t: Tag, afUsers: Map<string, Tag>): string {
  const user = afUsers.get(t.id)
  if (user) return formatTagName(user)
  return formatTagName(t)
}

export function netLabel(n: NetIface): string {
  return n.desc?.trim() || n.name
}

const DAP_LABEL: Record<string, string> = {
  learning: 'Learning',
  optimizing: 'Optimizing',
  active: 'Active',
  not_applicable: 'Not applicable',
  disabled: 'Disabled',
}

export function dapLabel(status?: string): string {
  if (!status) return ''
  return DAP_LABEL[status] || status
}
