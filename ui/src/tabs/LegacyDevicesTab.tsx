import { DeviceIcon } from '@/components/DeviceIcon'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
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
import { formatTagName, fmtRelative, fmtTime, preferredName } from '@/lib/format'
import {
  SEEN_OPTIONS,
  type Device,
  type NetIface,
  type SeenId,
  type Tag,
  type ViewMode,
} from '@/lib/types'

export function LegacyDevicesTab({
  mode,
  devices,
  filtered,
  groupTags,
  uuidToNet,
  seenId,
  groupFilter,
  lanFilter,
  query,
  nowMs,
  onSeen,
  onGroup,
  onLan,
  onQuery,
  labelTag,
}: {
  mode: ViewMode
  devices: Device[]
  filtered: Device[]
  groupTags: Tag[]
  uuidToNet: Map<string, NetIface>
  seenId: SeenId
  groupFilter: string
  lanFilter: string
  query: string
  nowMs: number
  onSeen: (id: SeenId) => void
  onGroup: (id: string) => void
  onLan: (uuid: string) => void
  onQuery: (q: string) => void
  labelTag: (id: string, preferType?: string) => string
}) {
  const lanOptions = [...uuidToNet.values()]
    .filter((n) => n.uuid)
    .sort((a, b) => (a.desc || a.name).localeCompare(b.desc || b.name))

  const grouped = (() => {
    const m = new Map<string, { lan?: NetIface; devices: Device[] }>()
    for (const d of filtered) {
      const key = d.intf_uuid || '_'
      let g = m.get(key)
      if (!g) {
        g = { lan: d.intf_uuid ? uuidToNet.get(d.intf_uuid) : undefined, devices: [] }
        m.set(key, g)
      }
      g.devices.push(d)
    }
    return [...m.entries()].sort((a, b) => {
      if (a[0] === '_') return 1
      if (b[0] === '_') return -1
      return b[1].devices.length - a[1].devices.length
    })
  })()

  const toolbar = (
    <div className="flex flex-wrap items-center gap-2">
      <Select value={seenId} onValueChange={(v) => onSeen(v as SeenId)}>
        <SelectTrigger size="sm" className="w-[5.5rem]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SEEN_OPTIONS.map((o) => (
            <SelectItem key={o.id} value={o.id}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={groupFilter || 'all'} onValueChange={(v) => onGroup(v === 'all' ? '' : v)}>
        <SelectTrigger size="sm" className="w-[11rem]">
          <SelectValue placeholder="Group" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All groups</SelectItem>
          {groupTags.map((t) => (
            <SelectItem key={t.id} value={t.id}>
              {formatTagName(t)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={lanFilter || 'all'} onValueChange={(v) => onLan(v === 'all' ? '' : v)}>
        <SelectTrigger size="sm" className="w-[11rem]">
          <SelectValue placeholder="LAN" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All LANs</SelectItem>
          {lanOptions.map((n) => (
            <SelectItem key={n.uuid} value={n.uuid!}>
              {n.desc || n.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        className="h-8 max-w-xs"
        type="search"
        placeholder="Filter"
        value={query}
        onChange={(e) => onQuery(e.target.value)}
      />
      <span className="ml-auto font-mono text-xs text-muted-foreground">
        {filtered.length}/{devices.length}
      </span>
    </div>
  )

  if (mode === 'list') {
    return (
      <div className="space-y-4">
        {toolbar}
        <Card className="gap-0 py-0">
          <CardContent className="px-0">
            {devices.length === 0 ? (
              <p className="px-6 py-8 text-sm text-muted-foreground">No inventory</p>
            ) : filtered.length === 0 ? (
              <p className="px-6 py-8 text-sm text-muted-foreground">No devices</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>IP</TableHead>
                    <TableHead>LAN</TableHead>
                    <TableHead>Groups</TableHead>
                    <TableHead>MAC</TableHead>
                    <TableHead className="text-right">Last</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.map((d) => {
                    const lan = d.intf_uuid ? uuidToNet.get(d.intf_uuid) : undefined
                    const groupLabels = (d.tag_ids ?? [])
                      .map((id) => labelTag(id, 'group'))
                      .filter(Boolean)
                    return (
                      <TableRow key={d.mac}>
                        <TableCell className="font-medium">
                          <span className="inline-flex items-center gap-2">
                            <DeviceIcon type={d.type} />
                            {preferredName(d)}
                          </span>
                        </TableCell>
                        <TableCell className="font-mono">{d.ip || '—'}</TableCell>
                        <TableCell>{lan?.desc || lan?.name || '—'}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {groupLabels.length
                              ? groupLabels.map((g) => (
                                  <Badge key={g} variant="outline">
                                    {g}
                                  </Badge>
                                ))
                              : '—'}
                          </div>
                        </TableCell>
                        <TableCell className="font-mono text-muted-foreground">
                          {d.mac}
                        </TableCell>
                        <TableCell
                          className="text-right font-mono tabular-nums text-muted-foreground"
                          title={
                            d.last_active_ts
                              ? fmtTime(Math.floor(d.last_active_ts))
                              : undefined
                          }
                        >
                          {d.last_active_ts ? fmtRelative(d.last_active_ts, nowMs) : '—'}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {toolbar}
      {devices.length === 0 ? (
        <p className="text-sm text-muted-foreground">No inventory</p>
      ) : filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">No devices</p>
      ) : (
        <div className="space-y-6">
          {grouped.map(([key, g]) => (
            <section key={key} className="space-y-2">
              <div className="flex items-baseline justify-between">
                <h2 className="text-sm font-medium">
                  {g.lan?.desc || g.lan?.name || 'Unassigned'}
                </h2>
                <span className="font-mono text-xs text-muted-foreground">
                  {g.devices.length}
                </span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {g.devices.map((d) => (
                  <div
                    key={d.mac}
                    className="inline-flex items-center gap-1.5 rounded-md border bg-card px-2 py-1 text-sm"
                    title={d.mac}
                  >
                    <DeviceIcon type={d.type} className="size-3.5" />
                    <span className="font-medium">{preferredName(d)}</span>
                    {d.ip ? (
                      <span className="font-mono text-xs text-muted-foreground">{d.ip}</span>
                    ) : null}
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
