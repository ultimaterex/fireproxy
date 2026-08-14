import { LayoutGrid, List } from 'lucide-react'

import { Button } from '@/components/ui/button'
import type { ViewMode } from '@/lib/types'
import { cn } from '@/lib/utils'

export function ViewToggle({
  value,
  onChange,
}: {
  value: ViewMode
  onChange: (mode: ViewMode) => void
}) {
  return (
    <div className="inline-flex rounded-md border p-0.5">
      <Button
        type="button"
        size="xs"
        variant="ghost"
        className={cn(value === 'visual' && 'bg-accent')}
        onClick={() => onChange('visual')}
      >
        <LayoutGrid />
        Visual
      </Button>
      <Button
        type="button"
        size="xs"
        variant="ghost"
        className={cn(value === 'list' && 'bg-accent')}
        onClick={() => onChange('list')}
      >
        <List />
        List
      </Button>
    </div>
  )
}
