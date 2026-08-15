import { useEffect } from 'react'
import { X } from 'lucide-react'

import { cn } from '@/lib/utils'

export function Toast({
  message,
  onDismiss,
  className,
}: {
  message: string
  onDismiss: () => void
  className?: string
}) {
  useEffect(() => {
    const id = window.setTimeout(onDismiss, 4000)
    return () => window.clearTimeout(id)
  }, [message, onDismiss])

  return (
    <div
      role="status"
      className={cn(
        'fixed right-4 bottom-4 z-[60] flex w-80 items-start gap-2 overflow-hidden rounded-lg border border-border bg-popover px-4 py-3 text-sm shadow-lg',
        className,
      )}
    >
      <p className="min-w-0 flex-1">{message}</p>
      <button
        type="button"
        className="shrink-0 text-muted-foreground hover:text-foreground"
        onClick={onDismiss}
        aria-label="Dismiss"
      >
        <X className="size-4" />
      </button>
    </div>
  )
}
