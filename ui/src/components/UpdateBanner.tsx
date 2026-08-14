import { useEffect, useState } from 'react'
import { RefreshCw, X } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  bootUiBuild,
  checkUiUpdate,
  UI_BUILD_POLL_MS,
  UI_UPDATE_FORCE_EVENT,
} from '@/lib/ui-build'
import { cn } from '@/lib/utils'

export function UpdateBanner({ className }: { className?: string }) {
  const [available, setAvailable] = useState(false)
  const [dismissed, setDismissed] = useState(false)

  useEffect(() => {
    const onForce = () => {
      setDismissed(false)
      setAvailable(true)
    }
    window.addEventListener(UI_UPDATE_FORCE_EVENT, onForce)
    return () => window.removeEventListener(UI_UPDATE_FORCE_EVENT, onForce)
  }, [])

  useEffect(() => {
    if (import.meta.env.DEV || !bootUiBuild) return
    let cancelled = false
    const tick = async () => {
      const next = await checkUiUpdate()
      if (!cancelled && next) setAvailable(true)
    }
    void tick()
    const id = window.setInterval(() => void tick(), UI_BUILD_POLL_MS)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [])

  if (dismissed || !available) return null

  return (
    <div
      role="status"
      className={cn(
        'fixed right-4 bottom-4 z-50 w-80 overflow-hidden rounded-lg border border-border bg-popover shadow-lg',
        className,
      )}
    >
      <div className="flex items-start gap-3 p-4">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-emerald-500/15 text-emerald-400">
          <RefreshCw className="size-4" />
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex items-start justify-between gap-2">
            <p className="text-sm font-medium leading-snug">Update available</p>
            <button
              type="button"
              className="-mt-0.5 -mr-1 rounded p-1 text-muted-foreground hover:text-foreground"
              aria-label="Dismiss"
              onClick={() => setDismissed(true)}
            >
              <X className="size-3.5" />
            </button>
          </div>
          <p className="text-xs leading-relaxed text-muted-foreground">
            A newer FireProxy UI is ready. Reload to pick it up.
          </p>
          <div className="flex items-center gap-2 pt-2">
            <Button type="button" size="xs" onClick={() => window.location.reload()}>
              Reload
            </Button>
            <Button type="button" size="xs" variant="ghost" onClick={() => setDismissed(true)}>
              Later
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
