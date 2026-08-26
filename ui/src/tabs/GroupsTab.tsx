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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { preferredName } from '@/lib/format'
import { hostTagsAdd, hostTagsRemove } from '@/lib/host-tags'
import type { Device, Tag } from '@/lib/types'
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
  groups,
  devices,
  canEditGroupMembers = false,
  onSetHostTags,
  onViewInDevices,
}: {
  groups: GroupRow[]
  devices: Device[]
  canEditGroupMembers?: boolean
  onSetHostTags?: (mac: string, tags: string[]) => Promise<void>
  onViewInDevices: (id: string, tagType: TagKind) => void
}) {
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [busyMac, setBusyMac] = useState<string | null>(null)
  const [addKey, setAddKey] = useState(0)

  const filtered = useMemo(() => {
    if (typeFilter === 'all') return groups
    return groups.filter((g) => tagKind(g) === typeFilter)
  }, [groups, typeFilter])

  const selected = useMemo(
    () =>
      selectedKey
        ? filtered.find((g) => `${tagTypeOf(g)}:${g.id}` === selectedKey) ??
          groups.find((g) => `${tagTypeOf(g)}:${g.id}` === selectedKey)
        : undefined,
    [selectedKey, filtered, groups],
  )

  const selectedType = selected ? tagTypeOf(selected) : null
  const writable =
    !!selected && selectedType === 'group' && canEditGroupMembers && !!onSetHostTags

  const members = useMemo(() => {
    if (!selected || !selectedType) return []
    return membersOf(devices, selected.id, selectedType)
  }, [devices, selected, selectedType])

  const candidates = useMemo(() => {
    if (!writable || !selected) return []
    const inGroup = new Set(members.map((d) => d.mac.toUpperCase()))
    return devices
      .filter((d) => !inGroup.has(d.mac.toUpperCase()))
      .slice()
      .sort((a, b) => preferredName(a).localeCompare(preferredName(b)))
  }, [writable, selected, members, devices])

  async function setMembership(device: Device, next: string[]) {
    if (!onSetHostTags) return
    setBusyMac(device.mac)
    try {
      await onSetHostTags(device.mac, next)
    } finally {
      setBusyMac(null)
    }
  }

  async function assign(mac: string) {
    if (!selected || !writable) return
    const device = devices.find((d) => d.mac.toUpperCase() === mac.toUpperCase())
    if (!device) return
    await setMembership(device, hostTagsAdd(device.tag_ids ?? [], selected.id))
    setAddKey((k) => k + 1)
  }

  async function unassign(device: Device) {
    if (!selected || !writable) return
    await setMembership(device, hostTagsRemove(device.tag_ids ?? [], selected.id))
  }

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
                const key = `${tagTypeOf(g)}:${g.id}`
                const active = selectedKey === key
                return (
                  <TableRow
                    key={key}
                    className={cn('cursor-pointer', active && 'bg-accent/50')}
                    onClick={() => setSelectedKey(key)}
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
            onClick={() => setSelectedKey(null)}
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
        {writable ? (
          <div className="flex items-center gap-2 border-b px-6 py-3">
            <Select
              key={addKey}
              disabled={!!busyMac || candidates.length === 0}
              onValueChange={(mac) => void assign(mac)}
            >
              <SelectTrigger size="sm" className="w-full max-w-xs">
                <SelectValue placeholder="Add device" />
              </SelectTrigger>
              <SelectContent position="popper">
                {candidates.map((d) => (
                  <SelectItem key={d.mac} value={d.mac}>
                    {preferredName(d)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : null}
        {members.length === 0 ? (
          <p className="px-6 py-8 text-sm text-muted-foreground">No devices</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>MAC</TableHead>
                {writable ? <TableHead className="w-[1%] text-right" /> : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((d) => (
                <TableRow key={d.mac}>
                  <TableCell>{preferredName(d)}</TableCell>
                  <TableCell className="font-mono text-muted-foreground">{d.mac}</TableCell>
                  {writable ? (
                    <TableCell className="text-right">
                      <Button
                        type="button"
                        size="xs"
                        variant="outline"
                        disabled={!!busyMac}
                        onClick={() => void unassign(d)}
                      >
                        Remove
                      </Button>
                    </TableCell>
                  ) : null}
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
