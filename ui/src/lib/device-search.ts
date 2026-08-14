import { dapLabel, deviceOnline, preferredName } from '@/lib/format'
import type { PortLoc } from '@/lib/switch-port'
import type { Device, NetIface } from '@/lib/types'

export const SEARCH_FIELDS = [
  { key: 'device', label: 'Device', hint: 'Any device connected to your box' },
  { key: 'user', label: 'User', hint: 'Any user created by you' },
  { key: 'group', label: 'Group', hint: 'Any group created by you' },
  { key: 'status', label: 'Status', hint: 'Online or Offline' },
  { key: 'dapstatus', label: 'DAPStatus', hint: 'Learning, Optimizing, Active, or N/A' },
  { key: 'network', label: 'Network', hint: 'Any network created by you' },
  { key: 'switch', label: 'Switch', hint: 'Managed switch name' },
  { key: 'port', label: 'Port', hint: 'Switch port number' },
  { key: 'ip', label: 'IP', hint: 'Device IP address' },
  { key: 'mac', label: 'MAC', hint: 'Device MAC address' },
  { key: 'manufacturer', label: 'Manufacturer', hint: 'Device manufacturer' },
] as const

export type SearchKey = (typeof SEARCH_FIELDS)[number]['key']

export type ParsedQuery = {
  fields: { key: SearchKey; value: string }[]
  text: string
}

const KEY_RE = /(^|\s)(device|user|group|dapstatus|status|network|switch|port|ip|mac|manufacturer):/gi

const LABEL_BY_KEY: Record<SearchKey, string> = Object.fromEntries(
  SEARCH_FIELDS.map((f) => [f.key, f.label]),
) as Record<SearchKey, string>

export function parseDeviceQuery(raw: string): ParsedQuery {
  const parts: { start: number; keyStart: number; key: SearchKey }[] = []
  const re = new RegExp(KEY_RE.source, KEY_RE.flags)
  let m: RegExpExecArray | null
  while ((m = re.exec(raw))) {
    parts.push({
      start: m.index + m[1].length,
      keyStart: m.index + m[1].length,
      key: m[2].toLowerCase() as SearchKey,
    })
  }
  const fields: ParsedQuery['fields'] = []
  let text = ''
  let cursor = 0
  for (let i = 0; i < parts.length; i++) {
    const p = parts[i]
    text += raw.slice(cursor, p.start)
    const valueStart = p.keyStart + p.key.length + 1
    const valueEnd = i + 1 < parts.length ? parts[i + 1].start : raw.length
    fields.push({ key: p.key, value: raw.slice(valueStart, valueEnd).trim() })
    cursor = valueEnd
  }
  text += raw.slice(cursor)
  return { fields, text: text.replace(/\s+/g, ' ').trim() }
}

export function lastToken(raw: string): { kind: 'field'; key: SearchKey; value: string } | { kind: 'prefix'; text: string } {
  const parsed = parseDeviceQuery(raw)
  const trimmed = raw.trimEnd()
  const last = parsed.fields[parsed.fields.length - 1]
  if (last) {
    const label = LABEL_BY_KEY[last.key]
    const at = trimmed.toLowerCase().lastIndexOf(label.toLowerCase() + ':')
    if (at >= 0) {
      const after = trimmed.slice(at + label.length + 1)
      if (!/\s(device|user|group|dapstatus|status|network|switch|port|ip|mac|manufacturer):/i.test(' ' + after)) {
        if (/\s$/.test(raw) && after.trim()) {
          return { kind: 'prefix', text: '' }
        }
        return { kind: 'field', key: last.key, value: after.trim() }
      }
    }
  }
  const m = /(?:^|\s)([a-z]*)$/i.exec(raw)
  return { kind: 'prefix', text: (m?.[1] ?? '').toLowerCase() }
}

export function applyField(raw: string, label: string): string {
  const tok = lastToken(raw)
  if (tok.kind === 'prefix' && tok.text) {
    return raw.replace(/[a-z]*$/i, `${label}:`)
  }
  const base = raw.trimEnd()
  return base ? `${base} ${label}:` : `${label}:`
}

export function applyValue(raw: string, value: string): string {
  const tok = lastToken(raw)
  if (tok.kind !== 'field') return raw
  const label = LABEL_BY_KEY[tok.key]
  const at = raw.toLowerCase().lastIndexOf(label.toLowerCase() + ':')
  if (at < 0) return raw
  return `${raw.slice(0, at)}${label}:${value} `
}

export function setSearchField(raw: string, key: SearchKey, value: string): string {
  const parsed = parseDeviceQuery(raw)
  const fields = parsed.fields.filter((f) => f.key !== key)
  fields.push({ key, value })
  return serializeQuery(parsed.text, fields)
}

export function clearSearchFields(raw: string, keys: SearchKey[]): string {
  const parsed = parseDeviceQuery(raw)
  const drop = new Set(keys)
  return serializeQuery(
    parsed.text,
    parsed.fields.filter((f) => !drop.has(f.key)),
  )
}

function serializeQuery(text: string, fields: ParsedQuery['fields']): string {
  const tokens = fields.filter((f) => f.value).map((f) => `${LABEL_BY_KEY[f.key]}:${f.value}`)
  return [...(text ? [text] : []), ...tokens].join(' ') + (tokens.length ? ' ' : '')
}

export function includesFold(hay: string | undefined, needle: string): boolean {
  if (!needle) return true
  return (hay ?? '').toLowerCase().includes(needle.toLowerCase())
}

export function deviceMatchesSearch(
  d: Device,
  raw: string,
  ctx: {
    nowMs: number
    uuidToNet: Map<string, NetIface>
    labelTag: (id: string, preferType?: string) => string
    portOf?: Map<string, PortLoc>
  },
): boolean {
  const q = parseDeviceQuery(raw)
  const online = deviceOnline(d, ctx.nowMs)
  const lan = d.intf_uuid ? ctx.uuidToNet.get(d.intf_uuid) : undefined
  const loc = ctx.portOf?.get(d.mac.toUpperCase())
  const groups = (d.tag_ids ?? []).map((id) => ctx.labelTag(id, 'group'))
  const users = (d.user_tag_ids ?? []).map((id) => ctx.labelTag(id, 'user'))

  if (q.text) {
    const hay = [
      preferredName(d),
      d.ip,
      d.mac,
      d.vendor,
      ...groups,
      ...users,
      lan?.desc,
      lan?.name,
      online ? 'online' : 'offline',
      d.dap?.status,
      dapLabel(d.dap?.status),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    if (!hay.includes(q.text.toLowerCase())) return false
  }

  for (const f of q.fields) {
    if (!f.value) continue
    const ok = (() => {
      switch (f.key) {
        case 'device':
          return includesFold(preferredName(d), f.value)
        case 'user':
          return users.some((n) => includesFold(n, f.value))
        case 'group':
          return groups.some((n) => includesFold(n, f.value))
        case 'status':
          return includesFold(online ? 'online' : 'offline', f.value)
        case 'dapstatus':
          return includesFold(d.dap?.status, f.value) || includesFold(dapLabel(d.dap?.status), f.value)
        case 'network':
          return includesFold(lan?.desc, f.value) || includesFold(lan?.name, f.value)
        case 'switch':
          return includesFold(loc?.switchName, f.value)
        case 'port':
          return (
            includesFold(loc?.portId, f.value) ||
            includesFold(loc?.label, f.value) ||
            (loc?.uplink ? includesFold('uplink', f.value) : false)
          )
        case 'ip':
          return includesFold(d.ip, f.value) || (d.ipv6 ?? []).some((v) => includesFold(v, f.value))
        case 'mac':
          return includesFold(d.mac, f.value)
        case 'manufacturer':
          return includesFold(d.vendor || 'Unknown', f.value)
      }
    })()
    if (!ok) return false
  }
  return true
}
