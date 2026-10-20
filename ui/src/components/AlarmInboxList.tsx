import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { fmtRelative } from '@/lib/format'
import type { AlarmSample } from '@/lib/types'
import { cn } from '@/lib/utils'

function alarmTypeLabel(type?: string): string {
  if (!type) return '—'
  const raw = type.replace(/^ALARM_/i, '').replace(/_/g, ' ')
  return raw || type
}

function deviceLabel(a: AlarmSample): string {
  return a.device_name?.trim() || a.device_mac?.trim() || a.device_ip?.trim() || '—'
}

export function AlarmInboxList({
  alarms,
  activeCount = 0,
  nowMs,
  controlLanOk,
  busyAid,
  compact,
  onIgnore,
}: {
  alarms: AlarmSample[]
  /** Active inbox count; used when samples are missing. */
  activeCount?: number
  nowMs: number
  controlLanOk: boolean
  busyAid?: number | null
  compact?: boolean
  onIgnore?: (aid: number) => void
}) {
  if (alarms.length === 0) {
    return (
      <p className="px-1 py-6 text-sm text-muted-foreground">
        {activeCount > 0 ? 'No alarm details' : 'No alarms'}
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className={cn(compact && 'h-8 text-xs')}>Type</TableHead>
          <TableHead className={cn(compact && 'h-8 text-xs')}>Message</TableHead>
          <TableHead className={cn(compact && 'h-8 text-xs')}>Device</TableHead>
          <TableHead className={cn('w-20', compact && 'h-8 text-xs')}>When</TableHead>
          {onIgnore ? (
            <TableHead className={cn('w-24 text-right', compact && 'h-8 text-xs')} />
          ) : null}
        </TableRow>
      </TableHeader>
      <TableBody>
        {alarms.map((a) => (
          <TableRow key={a.aid}>
            <TableCell className={cn(compact ? 'py-1.5 text-xs' : 'text-sm')}>
              {alarmTypeLabel(a.type)}
            </TableCell>
            <TableCell
              className={cn(
                'max-w-[28rem] truncate',
                compact ? 'py-1.5 text-xs' : 'text-sm',
              )}
              title={a.message}
            >
              {a.message?.trim() || '—'}
            </TableCell>
            <TableCell
              className={cn(
                'max-w-[12rem] truncate font-mono text-xs',
                compact ? 'py-1.5' : '',
              )}
              title={deviceLabel(a)}
            >
              {deviceLabel(a)}
            </TableCell>
            <TableCell className={cn('tabular-nums text-muted-foreground', compact ? 'py-1.5 text-xs' : 'text-sm')}>
              {a.timestamp != null ? fmtRelative(a.timestamp, nowMs) : '—'}
            </TableCell>
            {onIgnore ? (
              <TableCell className={cn('text-right', compact && 'py-1.5')}>
                <Button
                  type="button"
                  size={compact ? 'xs' : 'sm'}
                  variant="outline"
                  disabled={!controlLanOk || busyAid != null}
                  onClick={() => onIgnore(a.aid)}
                >
                  Ignore
                </Button>
              </TableCell>
            ) : null}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
