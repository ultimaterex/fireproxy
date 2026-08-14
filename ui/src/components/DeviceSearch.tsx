import { useMemo, useRef, useState } from 'react'
import { Search } from 'lucide-react'

import { Input } from '@/components/ui/input'
import {
  SEARCH_FIELDS,
  applyField,
  applyValue,
  lastToken,
  type SearchKey,
} from '@/lib/device-search'
import { preferredName } from '@/lib/format'
import type { Switch, Device, NetIface, Tag } from '@/lib/types'
import { cn } from '@/lib/utils'

export function DeviceSearch({
  value,
  onChange,
  devices,
  tags,
  uuidToNet,
  switches,
}: {
  value: string
  onChange: (q: string) => void
  devices: Device[]
  tags: Tag[]
  uuidToNet: Map<string, NetIface>
  switches?: Switch[]
}) {
  const [open, setOpen] = useState(false)
  const [hi, setHi] = useState(0)
  const box = useRef<HTMLDivElement>(null)
  const input = useRef<HTMLInputElement>(null)

  const items = useMemo(
    () => suggestions(value, { devices, tags, uuidToNet, switches: switches ?? [] }),
    [value, devices, tags, uuidToNet, switches],
  )

  const pick = (item: Suggest) => {
    const next = item.kind === 'field' ? applyField(value, item.label) : applyValue(value, item.value)
    onChange(next)
    setHi(0)
    input.current?.focus()
  }

  return (
    <div ref={box} className="relative">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-muted-foreground" />
      <Input
        ref={input}
        className="h-10 rounded-full pl-10"
        type="text"
        autoComplete="off"
        placeholder="Search for devices by device name, status, IP, MAC, etc."
        value={value}
        onChange={(e) => {
          onChange(e.target.value)
          setOpen(true)
          setHi(0)
        }}
        onFocus={() => setOpen(true)}
        onBlur={(e) => {
          if (!box.current?.contains(e.relatedTarget as Node)) setOpen(false)
        }}
        onKeyDown={(e) => {
          if (!open || items.length === 0) return
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            setHi((i) => (i + 1) % items.length)
          } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            setHi((i) => (i - 1 + items.length) % items.length)
          } else if (e.key === 'Enter') {
            e.preventDefault()
            pick(items[hi] ?? items[0])
          } else if (e.key === 'Escape') {
            setOpen(false)
          }
        }}
      />
      {open && items.length > 0 ? (
        <ul className="absolute z-30 mt-1 max-h-80 w-full overflow-auto rounded-lg border bg-popover py-1 shadow-md">
          {items.map((item, i) => (
            <li key={item.id}>
              <button
                type="button"
                className={cn(
                  'flex w-full items-center gap-2 px-3 py-2 text-left text-sm',
                  i === hi && 'bg-accent',
                )}
                onMouseDown={(e) => e.preventDefault()}
                onMouseEnter={() => setHi(i)}
                onClick={() => pick(item)}
              >
                <Search className="size-4 shrink-0 text-[#027BFF]" />
                {item.kind === 'field' ? (
                  <>
                    <span className="font-medium">{item.label}:</span>
                    <span className="text-muted-foreground">{item.hint}</span>
                  </>
                ) : (
                  <>
                    <span className="font-medium">{item.label}:</span>
                    <span>{item.value}</span>
                  </>
                )}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

type Suggest =
  | { kind: 'field'; id: string; label: string; hint: string }
  | { kind: 'value'; id: string; label: string; value: string }

function suggestions(
  raw: string,
  ctx: {
    devices: Device[]
    tags: Tag[]
    uuidToNet: Map<string, NetIface>
    switches: Switch[]
  },
): Suggest[] {
  const tok = lastToken(raw)
  if (tok.kind === 'field') {
    return valueSuggest(tok.key, tok.value, ctx).slice(0, 10)
  }
  const p = tok.text
  return SEARCH_FIELDS.filter((f) => !p || f.label.toLowerCase().startsWith(p) || f.key.startsWith(p)).map(
    (f) => ({ kind: 'field' as const, id: f.key, label: f.label, hint: f.hint }),
  )
}

function valueSuggest(
  key: SearchKey,
  needle: string,
  ctx: {
    devices: Device[]
    tags: Tag[]
    uuidToNet: Map<string, NetIface>
    switches: Switch[]
  },
): Suggest[] {
  const label = SEARCH_FIELDS.find((f) => f.key === key)?.label ?? key
  const uniq = (vals: string[]) => {
    const seen = new Set<string>()
    const out: Suggest[] = []
    for (const v of vals) {
      const t = v.trim()
      if (!t || seen.has(t.toLowerCase())) continue
      if (needle && !t.toLowerCase().includes(needle.toLowerCase())) continue
      seen.add(t.toLowerCase())
      out.push({ kind: 'value', id: `${key}:${t}`, label, value: t })
    }
    return out
  }

  switch (key) {
    case 'status':
      return uniq(['Online', 'Offline'])
    case 'dapstatus':
      return uniq(['Learning', 'Optimizing', 'Active', 'Not applicable', 'Disabled'])
    case 'device':
      return uniq(ctx.devices.map((d) => preferredName(d)))
    case 'user':
      return uniq(ctx.tags.filter((t) => t.type === 'user').map((t) => t.name))
    case 'group':
      return uniq(ctx.tags.filter((t) => !t.type || t.type === 'group').map((t) => t.name))
    case 'network':
      return uniq(
        [...ctx.uuidToNet.values()]
          .filter((n) => n.type !== 'wan')
          .map((n) => n.desc || n.name),
      )
    case 'switch':
      return uniq(ctx.switches.map((s) => s.name?.trim() || s.mac))
    case 'port':
      return uniq(
        ctx.switches.flatMap((s) =>
          (s.ports ?? [])
            .filter((p) => (p.clients?.length ?? 0) > 0)
            .map((p) => (p.uplink || p.id === s.uplink_port ? 'Uplink' : p.id)),
        ),
      )
    case 'ip':
      return uniq(ctx.devices.map((d) => d.ip ?? ''))
    case 'mac':
      return uniq(ctx.devices.map((d) => d.mac))
    case 'manufacturer':
      return uniq(ctx.devices.map((d) => d.vendor || 'Unknown'))
    default:
      return []
  }
}
