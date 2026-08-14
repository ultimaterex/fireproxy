import { DeviceIcon } from '@/components/DeviceIcon'
import { Flag } from '@/components/Flag'
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
import { fmtBytes } from '@/lib/format'
import type { RankedFlow } from '@/lib/types'

export function RegionDetail({
  region,
  onSelectDevice,
}: {
  region: RankedFlow
  onSelectDevice?: (mac: string, label: string) => void
}) {
  const devices = region.devices ?? []
  const targets = region.targets ?? []
  return (
    <div className="space-y-3">
      <Card className="gap-2 py-4">
        <CardHeader className="px-5">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Flag cc={region.country || region.id} />
            {region.name}
          </CardTitle>
          <CardDescription>Last 24 hours</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-8 px-5">
          <div>
            <p className="font-mono text-2xl tabular-nums">{fmtBytes(region.upload)}</p>
            <p className="text-[11px] text-muted-foreground">Upload</p>
          </div>
          <div>
            <p className="font-mono text-2xl tabular-nums">{fmtBytes(region.download)}</p>
            <p className="text-[11px] text-muted-foreground">Download</p>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-3 lg:grid-cols-2">
        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-3">
            <CardTitle className="text-sm">Devices</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            {!devices.length ? (
              <p className="px-6 py-6 text-sm text-muted-foreground">No devices in sample</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Device</TableHead>
                    <TableHead className="text-right">Upload</TableHead>
                    <TableHead className="text-right">Download</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {devices.map((d) => {
                    const clickable = !!d.id && !!onSelectDevice
                    return (
                      <TableRow
                        key={d.id || d.name}
                        className={clickable ? 'cursor-pointer hover:bg-accent/40' : undefined}
                        onClick={
                          clickable
                            ? () => onSelectDevice?.(d.id!.toUpperCase(), d.name)
                            : undefined
                        }
                      >
                        <TableCell className="font-medium">
                          <span className="inline-flex items-center gap-2">
                            <DeviceIcon type={d.type} className="size-4" />
                            {d.name}
                          </span>
                        </TableCell>
                        <TableCell className="text-right font-mono tabular-nums">
                          {fmtBytes(d.upload)}
                        </TableCell>
                        <TableCell className="text-right font-mono tabular-nums">
                          {fmtBytes(d.download)}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-3">
            <CardTitle className="text-sm">Targets</CardTitle>
          </CardHeader>
          <CardContent className="px-0">
            {!targets.length ? (
              <p className="px-6 py-6 text-sm text-muted-foreground">No targets in sample</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Destination</TableHead>
                    <TableHead className="text-right">Upload</TableHead>
                    <TableHead className="text-right">Download</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {targets.map((t) => (
                    <TableRow key={t.id || t.name}>
                      <TableCell className="font-mono">
                        <span className="inline-flex items-center gap-2">
                          <Flag cc={t.country || region.country || region.id} />
                          {t.name}
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtBytes(t.upload)}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {fmtBytes(t.download)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
