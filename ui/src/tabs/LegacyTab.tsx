import { Sparkline } from '@/components/Sparkline'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
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
import { fmtMbps, fmtTime, ifaceLabel } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  CHART_RANGES,
  type ChartRange,
  type Device,
  type HistoryPoint,
  type LatestView,
  type NetIface,
  type PersistInfo,
  type SeenId,
  type Tag,
  type ViewMode,
} from '@/lib/types'
import { LegacyDevicesTab } from '@/tabs/LegacyDevicesTab'
import { LegacyTopology } from '@/tabs/LegacyTopology'

export type LegacyPage = 'metrics' | 'devices' | 'topology'

export function LegacyTab({
  latest,
  history,
  network,
  persist,
  chartRange,
  onChartRange,
  page,
  onPage,
  deviceMode,
  devices,
  filteredDevices,
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
  onSelectLan,
  host,
  ts,
}: {
  latest: LatestView | null
  history: HistoryPoint[]
  network: NetIface[]
  persist: PersistInfo
  chartRange: ChartRange
  onChartRange: (r: ChartRange) => void
  page: LegacyPage
  onPage: (p: LegacyPage) => void
  deviceMode: ViewMode
  devices: Device[]
  filteredDevices: Device[]
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
  onSelectLan: (uuid: string) => void
  host?: string
  ts?: number
}) {
  const topology = network.map((n) => ({
    ...n,
    device_count: n.uuid ? devices.filter((d) => d.intf_uuid === n.uuid).length : 0,
  }))

  return (
    <div className="space-y-4">
      <div className="inline-flex rounded-md border p-0.5">
        {(['metrics', 'devices', 'topology'] as const).map((id) => (
          <button
            key={id}
            type="button"
            className={cn(
              'h-7 rounded-sm px-3 text-xs capitalize',
              page === id && 'bg-accent',
            )}
            onClick={() => onPage(id)}
          >
            {id}
          </button>
        ))}
      </div>
      {page === 'devices' ? (
        <LegacyDevicesTab
          mode={deviceMode}
          devices={devices}
          filtered={filteredDevices}
          groupTags={groupTags}
          uuidToNet={uuidToNet}
          seenId={seenId}
          groupFilter={groupFilter}
          lanFilter={lanFilter}
          query={query}
          nowMs={nowMs}
          onSeen={onSeen}
          onGroup={onGroup}
          onLan={onLan}
          onQuery={onQuery}
          labelTag={labelTag}
        />
      ) : page === 'topology' ? (
        <LegacyTopology
          mode="visual"
          topology={topology}
          host={host}
          ts={ts}
          onSelectLan={onSelectLan}
        />
      ) : (
        <LegacyMetrics
          latest={latest}
          history={history}
          network={network}
          persist={persist}
          chartRange={chartRange}
          onChartRange={onChartRange}
        />
      )}
    </div>
  )
}

function LegacyMetrics({
  latest,
  history,
  network,
  persist,
  chartRange,
  onChartRange,
}: {
  latest: LatestView | null
  history: HistoryPoint[]
  network: NetIface[]
  persist: PersistInfo
  chartRange: ChartRange
  onChartRange: (r: ChartRange) => void
}) {
  const snap = latest?.snapshot
  if (!snap) {
    return <p className="text-sm text-muted-foreground">No snapshots yet</p>
  }

  const ifaces = Object.keys(snap.ifaces).sort((a, b) => a.localeCompare(b))
  const wans = Object.entries(snap.wan ?? {}).sort(([a], [b]) => a.localeCompare(b))
  const have = new Set<string>()
  for (const p of history) {
    Object.keys(p.rates ?? {}).forEach((k) => have.add(k))
  }
  const prefer = ['eth1', 'eth2', 'eth3', 'br0', 'br2']
  const chartIfaces = [
    ...prefer.filter((i) => have.has(i)),
    ...[...have].sort().filter((i) => !prefer.includes(i)),
  ].slice(0, 6)

  return (
    <div className="space-y-4">
      <p className="text-xs text-muted-foreground">
        {fmtTime(snap.ts)}
        {latest?.have_prev ? '' : ' · waiting'}
        {persist.enabled
          ? ` · ${persist.retention_days ?? 90}d · ${persist.points ?? 0}`
          : ' · memory'}
      </p>
      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-4">
          <CardTitle className="text-sm">Throughput</CardTitle>
          <CardDescription>Mbps · {chartRange}</CardDescription>
          <CardAction>
            <Select value={chartRange} onValueChange={(v) => onChartRange(v as ChartRange)}>
              <SelectTrigger size="sm" className="w-22">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CHART_RANGES.map((o) => (
                  <SelectItem key={o.id} value={o.id}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          {history.length < 2 ? (
            <p className="px-6 py-8 text-sm text-muted-foreground">Collecting…</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Iface</TableHead>
                  <TableHead className="text-right">RX</TableHead>
                  <TableHead className="text-right">TX</TableHead>
                  <TableHead>RX trend</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {chartIfaces.map((name) => {
                  const rx = history.map((p) => p.rates?.[name]?.rx_mbps ?? 0)
                  const last = history[history.length - 1]?.rates?.[name]
                  return (
                    <TableRow key={name}>
                      <TableCell>{ifaceLabel(name, network, snap.wan)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtMbps(last?.rx_mbps)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtMbps(last?.tx_mbps)}
                      </TableCell>
                      <TableCell>
                        <Sparkline values={rx} title={`${name} RX Mbps`} />
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-4">
            <CardTitle className="text-sm">WAN</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Iface</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Ready</TableHead>
                  <TableHead>Active</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {wans.map(([iface, w]) => (
                  <TableRow key={iface}>
                    <TableCell className="font-mono">{iface}</TableCell>
                    <TableCell>{w.name}</TableCell>
                    <TableCell>
                      <Badge variant={w.ready ? 'secondary' : 'destructive'}>
                        {w.ready ? 'yes' : 'no'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={w.active ? 'secondary' : 'outline'}>
                        {w.active ? 'yes' : 'no'}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-4">
            <CardTitle className="text-sm">Interfaces</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Iface</TableHead>
                  <TableHead className="text-right">RX</TableHead>
                  <TableHead className="text-right">TX</TableHead>
                  <TableHead className="text-right">Link</TableHead>
                  <TableHead>Carrier</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ifaces.map((name) => {
                  const iface = snap.ifaces[name]
                  const rates = latest?.rates?.[name]
                  return (
                    <TableRow key={name}>
                      <TableCell>{ifaceLabel(name, network, snap.wan)}</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtMbps(rates?.rx_mbps)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtMbps(rates?.tx_mbps)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums text-muted-foreground">
                        {iface.speed_mbps == null ? '—' : `${iface.speed_mbps}`}
                      </TableCell>
                      <TableCell>
                        <Badge variant={iface.carrier ? 'secondary' : 'destructive'}>
                          {iface.carrier ? 'up' : 'down'}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
