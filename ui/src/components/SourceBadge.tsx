import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

/** Chip for observatory provenance. Ignores missing/unknown sources (old servers). */
export function SourceBadge({
  source,
  stale,
  enrichedFrom,
  reason,
  className,
}: {
  source?: string | null
  stale?: boolean
  enrichedFrom?: string | null
  /** prefer = forced control refresh; fallback = agent down/stale. */
  reason?: string | null
  className?: string
}) {
  if (!source || source === 'empty') return null

  if (source === 'agent' && stale) {
    return (
      <Badge
        variant="outline"
        className={cn('border-amber-500/40 bg-amber-500/15 text-amber-400', className)}
      >
        Stale
      </Badge>
    )
  }

  if (source === 'agent') {
    return (
      <Badge
        variant="outline"
        className={cn('border-emerald-500/40 bg-emerald-500/15 text-emerald-400', className)}
      >
        Live · agent
      </Badge>
    )
  }

  if (source === 'fw-app-init') {
    const prefer = reason === 'prefer'
    const withAgent = enrichedFrom === 'agent'
    const label = prefer
      ? withAgent
        ? 'Control · agent'
        : 'Control'
      : withAgent
        ? 'Fallback · control + agent'
        : 'Fallback · control'
    return (
      <Badge
        variant="outline"
        className={cn(
          prefer
            ? 'border-emerald-500/40 bg-emerald-500/15 text-emerald-400'
            : 'border-amber-500/40 bg-amber-500/15 text-amber-400',
          className,
        )}
      >
        {label}
      </Badge>
    )
  }

  if (source === 'fw-app-get') {
    return (
      <Badge
        variant="outline"
        className={cn('border-emerald-500/40 bg-emerald-500/15 text-emerald-400', className)}
      >
        Live · control
      </Badge>
    )
  }

  return null
}
