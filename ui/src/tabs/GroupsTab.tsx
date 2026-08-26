import { useMemo, useState } from 'react'
import { ArrowLeft, MonitorSmartphone, Tag as TagIcon, User } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { preferredName } from '@/lib/format'
import type { Device, Tag, ViewMode } from '@/lib/types'
import { cn } from '@/lib/utils'

type TagKind = 'group' | 'user' | 'device'

type GroupRow = Tag & {
  count: number
  kind?: TagKind
  type?: string
}

type TypeFilter = 'all' | TagKind

const TYPE_FILTERS: { id: TypeFilter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'group', label: 'Group' },
  { id: 'user', label: 'User' },
  { id: 'device', label: 'Device' },
]

function tagKind(g: GroupRow): TagKind {
  if (g.kind === 'user' || g.kind === 'device' || g.kind === 'group') return g.kind
  const typ = g.type || 'group'
  if (typ === 'user' || typ === 'device') return typ
  return 'group'
}

/** Namespace used for membership / nav — not display kind (affiliated groups stay type group). */
function tagTypeOf(g: GroupRow): TagKind {
  const typ = g.type || 'group'
  if (typ === 'user' || typ === 'device') return typ
  return 'group'
}

function membersOf(devices: Device[], id: string, typ: TagKind): Device[] {
  return devices.filter((d) => {
    if (typ === 'device') return (d.device_tag_ids ?? []).includes(id)
    if (typ === 'user') return (d.user_tag_ids ?? []).includes(id)
    return (d.tag_ids ?? []).includes(id)
  })
}

function KindIcon({ kind, className }: { kind: TagKind; className?: string }) {
  if (kind === 'user') return <User className={className} />
  if (kind === 'device') return <MonitorSmartphone className={className} />
  return <TagIcon className={className} />
}

export function GroupsTab({
  mode: _mode,
  groups,
  devices,
  onViewInDevices,
}: {
  mode?: ViewMode
  groups: GroupRow[]
  devices: Device[]
  onViewInDevices: (id: string, tagType: TagKind) => void
}) {
  void _mode
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const filtered = useMemo(() => {
    if (typeFilter === 'all') return groups
    return groups.filter((g) => tagKind(g) === typeFilter)
  }, [groups, typeFilter])

  const selected = useMemo(
    () => (selectedId ? filtered.find((g) => g.id === selectedId) ?? groups.find((g) => g.id === selectedId) : undefined),
    [selectedId, filtered, groups],
  )

  const members = useMemo(() => {
    if (!selected) return []
    return membersOf(devices, selected.id, tagTypeOf(selected))
  }, [devices, selected])

  const list = (
    <Card className="gap-0 py-0">
      <CardHeader className="border-b py-3">
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-sm">Groups</CardTitle>
          <span className="font-mono text-xs tabular-nums text-muted-foreground">{filtered.length}</span>
          <div className="ml-auto flex flex-wrap gap-1">
            {TYPE_FILTERS.map((f) => (
              <Button
                key={f.id}
                type="button"
                size="xs"
                variant={typeFilter === f.id ? 'default' : 'outline'}
                onClick={() => setTypeFilter(f.id)}
              >
                {f.label}
              </Button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent className="px-0">
        {filtered.length === 0 ? (
          <p className="px-6 py-8 text-sm text-muted-foreground">No groups</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead className="text-right">Devices</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((g) => {
                const kind = tagKind(g)
                const active = selectedId === g.id
                return (
                  <TableRow
                    key={`${kind}:${g.id}`}
                    className={cn('cursor-pointer', active && 'bg-accent/50')}
                    onClick={() => setSelectedId(g.id)}
                  >
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5">
                        <KindIcon kind={kind} className="size-3.5 text-muted-foreground" />
                        {g.name}
                      </span>
                    </TableCell>
                    <TableCell className="capitalize text-muted-foreground">{kind}</TableCell>
                    <TableCell className="text-right font-mono tabular-nums">{g.count}</TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )

  const detail = selected ? (
    <Card className="gap-0 py-0">
      <CardHeader className="border-b py-3">
        <div className="flex items-start gap-2">
          <Button
            type="button"
            size="xs"
            variant="ghost"
            className="sm:hidden"
            onClick={() => setSelectedId(null)}
          >
            <ArrowLeft className="size-4" />
            Back
          </Button>
          <div className="min-w-0 flex-1">
            <CardTitle className="inline-flex items-center gap-1.5 text-base">
              <KindIcon kind={tagKind(selected)} className="size-4 shrink-0 text-muted-foreground" />
              <span className="truncate">{selected.name}</span>
            </CardTitle>
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
              <span className="capitalize">{tagKind(selected)}</span>
              <span className="font-mono">{selected.id}</span>
            </div>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => onViewInDevices(selected.id, tagTypeOf(selected))}
          >
            View in Devices
          </Button>
        </div>
      </CardHeader>
      <CardContent className="px-0">
        {members.length === 0 ? (
          <p className="px-6 py-8 text-sm text-muted-foreground">No devices</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>MAC</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((d) => (
                <TableRow key={d.mac}>
                  <TableCell>{preferredName(d)}</TableCell>
                  <TableCell className="font-mono text-muted-foreground">{d.mac}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  ) : (
    <Card className="hidden gap-0 py-0 sm:block">
      <CardContent className="px-6 py-8">
        <p className="text-sm text-muted-foreground">Select a group</p>
      </CardContent>
    </Card>
  )

  return (
    <>
      {/* Wide: list | detail */}
      <div className="hidden gap-4 sm:grid sm:grid-cols-[1fr_1.2fr]">
        {list}
        {detail}
      </div>
      {/* Narrow: list or detail */}
      <div className="sm:hidden">{selected ? detail : list}</div>
    </>
  )
}
