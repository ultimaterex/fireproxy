import type { ReactNode } from 'react'

import { ChevronRight } from 'lucide-react'

import { cn } from '@/lib/utils'

export type Crumb = {
  label: string
  onClick?: () => void
}

export function Breadcrumb({
  items,
  trailing,
}: {
  items: Crumb[]
  trailing?: ReactNode
}) {
  if (items.length === 0) return null
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <nav className="flex min-w-0 flex-wrap items-center gap-1 text-sm">
        {items.map((c, i) => {
          const last = i === items.length - 1
          return (
            <span key={`${c.label}-${i}`} className="inline-flex min-w-0 items-center gap-1">
              {i > 0 ? <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" /> : null}
              {last || !c.onClick ? (
                <span
                  className={cn(
                    'truncate',
                    last ? 'font-semibold tracking-tight text-foreground' : 'text-muted-foreground',
                  )}
                >
                  {c.label}
                </span>
              ) : (
                <button
                  type="button"
                  className="truncate text-muted-foreground hover:text-foreground"
                  onClick={c.onClick}
                >
                  {c.label}
                </button>
              )}
            </span>
          )
        })}
      </nav>
      {trailing ? <div className="ml-auto flex shrink-0 items-center gap-2">{trailing}</div> : null}
    </div>
  )
}
