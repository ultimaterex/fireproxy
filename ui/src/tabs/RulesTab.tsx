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
import type { Policy, ViewMode } from '@/lib/types'

function RulesNotice() {
  return (
    <div
      role="status"
      className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground"
    >
      Unfinished — rework planned
    </div>
  )
}

export function RulesTab({
  mode,
  policies,
}: {
  mode: ViewMode
  policies: Policy[]
}) {
  const ranked = [...policies]
    .filter((p) => p.hit_count > 0)
    .sort((a, b) => b.hit_count - a.hit_count)
    .slice(0, 16)
  const maxHits = Math.max(1, ...ranked.map((p) => p.hit_count))
  const blocks = policies.filter((p) => p.action === 'block').length
  const allows = policies.length - blocks

  if (mode === 'list') {
    return (
      <div className="space-y-4">
        <RulesNotice />
        <Card className="gap-0 py-0">
          <CardHeader className="border-b py-4">
            <CardTitle className="text-sm">Rules</CardTitle>
            <CardDescription>{policies.length} policies</CardDescription>
          </CardHeader>
          <CardContent className="px-0">
            {policies.length === 0 ? (
              <p className="px-6 py-8 text-sm text-muted-foreground">No policies</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead className="text-right">Hits</TableHead>
                    <TableHead>On</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {policies.slice(0, 200).map((p) => (
                    <TableRow key={p.id}>
                      <TableCell className="font-mono">{p.id}</TableCell>
                      <TableCell>
                        <Badge variant={p.action === 'block' ? 'destructive' : 'secondary'}>
                          {p.action || '—'}
                        </Badge>
                      </TableCell>
                      <TableCell>{p.type || '—'}</TableCell>
                      <TableCell className="max-w-[18rem] truncate font-mono">
                        {p.notes || p.target || '—'}
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {p.hit_count}
                      </TableCell>
                      <TableCell>{p.disabled ? 'no' : 'yes'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    )
  }

  if (policies.length === 0) {
    return (
      <div className="space-y-4">
        <RulesNotice />
        <p className="text-sm text-muted-foreground">No policies</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <RulesNotice />
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="destructive">{blocks} block</Badge>
        <Badge variant="secondary">{allows} allow</Badge>
        <span className="text-xs text-muted-foreground">{ranked.length} with hits</span>
      </div>
      <div className="space-y-2">
        {ranked.map((p) => (
          <div key={p.id} className="space-y-1">
            <div className="flex items-center gap-2 text-sm">
              <Badge variant={p.action === 'block' ? 'destructive' : 'secondary'}>
                {p.action || '—'}
              </Badge>
              <span className="min-w-0 flex-1 truncate font-mono">
                {p.notes || p.target || p.id}
              </span>
              <span className="font-mono tabular-nums text-muted-foreground">
                {p.hit_count}
              </span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className={
                  p.action === 'block' ? 'h-full bg-destructive/80' : 'h-full bg-foreground/60'
                }
                style={{ width: `${Math.max((p.hit_count / maxHits) * 100, 2)}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
