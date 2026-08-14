import { useState } from 'react'

import { cn } from '@/lib/utils'

const SRC = 'https://cdn.jsdelivr.net/gh/lipis/flag-icons/flags/4x3'

const ALIAS: Record<string, string> = {
  suriname: 'sr',
  sur: 'sr',
  usa: 'us',
  uk: 'gb',
  'united kingdom': 'gb',
  'united states': 'us',
  netherlands: 'nl',
  holland: 'nl',
}

export function Flag({ cc, className }: { cc?: string; className?: string }) {
  const [dead, setDead] = useState(false)
  const iso = iso2(cc)
  if (!iso || dead) return null
  return (
    <span
      className={cn(
        'inline-flex h-[18px] w-6 shrink-0 overflow-hidden rounded-[2px]',
        className,
      )}
    >
      <img
        src={`${SRC}/${iso}.svg`}
        alt=""
        className="size-full object-cover"
        onError={() => setDead(true)}
      />
    </span>
  )
}

function iso2(raw?: string): string {
  const s = (raw ?? '').trim()
  if (!s || s === '[]' || s === 'null') return ''
  const lower = s.toLowerCase()
  if (/^[a-z]{2}$/.test(lower) && lower !== 'xx' && lower !== 'zz') return lower
  if (ALIAS[lower]) return ALIAS[lower]
  if (/^[a-z]{3}$/.test(lower) && ALIAS[lower]) return ALIAS[lower]
  return ''
}
