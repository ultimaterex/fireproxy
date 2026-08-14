import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Check, Copy } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** Copy text; falls back when Clipboard API is blocked (e.g. http:// LAN). */
export async function writeClipboard(text: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return
    } catch {
      /* fall through */
    }
  }
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.style.position = 'fixed'
  ta.style.left = '-9999px'
  document.body.appendChild(ta)
  ta.select()
  ta.setSelectionRange(0, text.length)
  const ok = document.execCommand('copy')
  document.body.removeChild(ta)
  if (!ok) throw new Error('copy failed')
}

export function useCopyFeedback(ms = 1500) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (timer.current != null) window.clearTimeout(timer.current)
    }
  }, [])

  const copy = useCallback(
    async (text: string) => {
      await writeClipboard(text)
      setCopied(true)
      if (timer.current != null) window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), ms)
    },
    [ms],
  )

  return { copied, copy }
}

/** Inline value + icon that flashes a check on copy. */
export function CopyText({ value, mono }: { value: string; mono?: boolean }) {
  const { copied, copy } = useCopyFeedback()
  return (
    <span className="inline-flex items-center gap-2">
      <span className={cn(mono && 'font-mono text-xs')}>{value}</span>
      <button
        type="button"
        className={cn(
          'hover:text-foreground',
          copied ? 'text-emerald-400' : 'text-muted-foreground',
        )}
        aria-label={copied ? 'Copied' : 'Copy'}
        onClick={() => void copy(value)}
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </button>
    </span>
  )
}

/** Labeled button that swaps to “Copied” briefly. */
export function CopyButton({
  value,
  size = 'xs',
  variant = 'outline',
  className,
  label = 'Copy',
}: {
  value: string
  size?: 'xs' | 'sm' | 'default'
  variant?: 'outline' | 'secondary' | 'ghost' | 'default'
  className?: string
  label?: ReactNode
}) {
  const { copied, copy } = useCopyFeedback()
  return (
    <Button
      type="button"
      size={size}
      variant={variant}
      className={className}
      onClick={() => void copy(value)}
    >
      {copied ? (
        <>
          <Check className="size-3.5 text-emerald-400" />
          Copied
        </>
      ) : (
        label
      )}
    </Button>
  )
}
