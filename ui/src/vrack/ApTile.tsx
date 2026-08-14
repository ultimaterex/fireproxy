import type { Switch } from '@/lib/types'
import { cn } from '@/lib/utils'

export const AP_DISC = 72
export const AP_GAP = 16

export function ApTile({
  sw,
  lit,
  onHover,
  onSelect,
}: {
  sw: Switch
  lit?: boolean
  onHover?: (on: boolean) => void
  onSelect: () => void
}) {
  const online = sw.healthy
  const label = apModel(sw)

  return (
    <button
      type="button"
      title={sw.name ?? sw.mac}
      onClick={onSelect}
      onMouseEnter={() => onHover?.(true)}
      onMouseLeave={() => onHover?.(false)}
      className={cn(
        'relative z-[1] flex items-center justify-center rounded-full border-2 px-2 text-center transition-[border-color,box-shadow,background-color]',
        lit
          ? 'border-sky-400 bg-sky-900/70 ring-2 ring-sky-400/50'
          : online
            ? 'border-sky-500 bg-sky-950/50'
            : 'border-muted-foreground/30 bg-muted/20',
      )}
      style={{ width: AP_DISC, height: AP_DISC }}
    >
      <span
        className={cn(
          'line-clamp-2 text-[10px] leading-tight',
          lit || online ? 'text-foreground/90' : 'text-muted-foreground/50',
        )}
      >
        {label}
      </span>
    </button>
  )
}

function apModel(sw: Switch): string {
  const m = (sw.model || '').trim()
  if (m) return m
  return (sw.name || 'AP').trim()
}
