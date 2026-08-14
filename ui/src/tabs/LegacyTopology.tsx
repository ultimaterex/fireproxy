import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
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
import { fmtTime, netLabel } from '@/lib/format'
import type { NetIface, ViewMode } from '@/lib/types'
import { cn } from '@/lib/utils'

export function LegacyTopology({
  mode,
  topology,
  host,
  ts,
  onSelectLan,
}: {
  mode: ViewMode
  topology: NetIface[]
  host?: string
  ts?: number
  onSelectLan: (uuid: string) => void
}) {
  const lans = topology.filter((n) => n.type === 'lan')
  const wans = topology.filter((n) => n.type === 'wan')
  const maxCount = Math.max(1, ...lans.map((n) => n.device_count ?? 0))

  if (mode === 'list') {
    return (
      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-4">
          <CardTitle className="text-sm">Topology</CardTitle>
          <CardDescription>
            {host ?? '—'} · {ts ? fmtTime(ts) : '—'}
          </CardDescription>
        </CardHeader>
        <CardContent className="px-0">
          {topology.length === 0 ? (
            <p className="px-6 py-8 text-sm text-muted-foreground">No catalog</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Label</TableHead>
                  <TableHead>Subnet</TableHead>
                  <TableHead className="text-right">Devices</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topology.map((n) => (
                  <TableRow key={n.uuid || n.name}>
                    <TableCell>
                      <Badge variant="outline">{n.type || '—'}</Badge>
                    </TableCell>
                    <TableCell className="font-mono">{n.name}</TableCell>
                    <TableCell>{n.desc || '—'}</TableCell>
                    <TableCell className="font-mono text-muted-foreground">
                      {n.subnet || n.ip || '—'}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums">
                      {n.device_count ?? 0}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    )
  }

  if (topology.length === 0) {
    return <p className="text-sm text-muted-foreground">No catalog</p>
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {lans.map((n) => {
          const count = n.device_count ?? 0
          const weight = count / maxCount
          return (
            <button
              key={n.uuid || n.name}
              type="button"
              disabled={!n.uuid}
              onClick={() => n.uuid && onSelectLan(n.uuid)}
              className="text-left"
            >
              <Card
                className={cn(
                  'h-full gap-2 py-4 transition-colors hover:bg-accent/40',
                  weight >= 0.5 ? 'py-5' : 'py-4',
                )}
              >
                <CardHeader className="px-5">
                  <div className="flex items-center justify-between gap-2">
                    <Badge variant="outline">lan</Badge>
                    <span className="font-mono text-lg tabular-nums">{count}</span>
                  </div>
                  <CardTitle className={cn(weight >= 0.5 ? 'text-xl' : 'text-base')}>
                    {netLabel(n)}
                  </CardTitle>
                  <CardDescription className="font-mono">
                    {n.subnet || n.ip || n.name}
                  </CardDescription>
                </CardHeader>
              </Card>
            </button>
          )
        })}
      </div>
      {wans.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2">
          {wans.map((n) => (
            <Card key={n.uuid || n.name} className="gap-2 py-4">
              <CardHeader className="px-5">
                <Badge variant="secondary">wan</Badge>
                <CardTitle className="text-base">{netLabel(n)}</CardTitle>
                <CardDescription className="font-mono">
                  {n.subnet || n.ip || n.name}
                </CardDescription>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
