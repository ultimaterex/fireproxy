import { EthernetPort, RectangleEllipsis } from 'lucide-react'

import { makerLogo } from '@/lib/maker'
import type { Switch } from '@/lib/types'
import { cn } from '@/lib/utils'
import type { PlacedJack, ResolvedFaceplate } from '@/vrack/types'

export function Faceplate({
  layout,
  sw,
  litPorts,
  onPortHover,
  onSelect,
}: {
  layout: ResolvedFaceplate
  sw: Switch
  litPorts?: Set<string>
  onPortHover?: (port: string, on: boolean) => void
  onSelect: (sw: Switch, port: string | null) => void
}) {
  const byId = new Map((sw.ports ?? []).map((p) => [p.id, p]))
  const flush = layout.chassis.x > 0 && !layout.extension
  const model = faceModel(sw, layout)
  return (
    <div
      className={cn(
        'relative shrink-0 border border-border/80 bg-muted/25',
        layout.def.form === 'shelf' ? 'rounded-lg' : 'rounded-md',
      )}
      style={{ width: layout.width, height: layout.height }}
    >
      {flush ? (
        <>
          <span className="absolute top-1.5 bottom-1.5 w-px bg-border" style={{ left: layout.chassis.x }} />
          <span
            className="absolute top-1.5 bottom-1.5 w-px bg-border"
            style={{ left: layout.chassis.x + layout.chassis.width }}
          />
        </>
      ) : null}
      {layout.extension ? (
        <span
          className="absolute inset-y-0 border-l border-border/80 bg-muted/40"
          style={{ left: layout.extension.x, width: layout.extension.width }}
        />
      ) : null}
      {layout.chassis.x > 0 && layout.extension ? (
        <span className="absolute top-1.5 bottom-1.5 w-px bg-border/70" style={{ left: layout.chassis.x }} />
      ) : null}
      {layout.window ? (
        <button
          type="button"
          title={sw.name ?? sw.mac}
          onClick={() => onSelect(sw, null)}
          className="absolute rounded-sm border border-border/70 bg-background/35"
          style={{ left: layout.window.x, width: layout.window.width, height: layout.height }}
        />
      ) : null}
      {layout.screen ? (
        <button
          type="button"
          title={sw.name ?? sw.mac}
          onClick={() => onSelect(sw, null)}
          className="absolute overflow-hidden rounded-[28%] bg-sky-500 text-xs font-bold text-white"
          style={{
            left: layout.screen.x,
            top: layout.screen.y,
            width: layout.screen.size,
            height: layout.screen.size,
          }}
        >
          <img src={makerLogo(layout.def.manufacturer)} alt="" className="size-full object-contain p-1" />
        </button>
      ) : null}
      {layout.badge ? (
        <button
          type="button"
          title={sw.name ?? sw.mac}
          onClick={() => onSelect(sw, null)}
          className="absolute"
          style={{
            left: layout.badge.x,
            top: layout.badge.y,
            width: layout.badge.w,
            height: layout.badge.h,
          }}
        >
          <img src={makerLogo(layout.def.manufacturer)} alt="" className="size-full object-contain" />
        </button>
      ) : null}
      {model ? (
        <span
          className="pointer-events-none absolute truncate text-center text-[8px] leading-tight text-muted-foreground/70"
          style={{
            left: layout.chassis.x + 6,
            width: Math.max(0, layout.chassis.width - 12),
            top: 3,
            lineHeight: '10px',
            height: 10,
          }}
          title={model}
        >
          {model}
        </span>
      ) : null}
      {layout.keystones.map((k, i) => (
        <span
          key={`ks-${i}`}
          className="absolute rounded-[2px] border border-border/70 bg-background/50"
          style={{ left: k.x, top: k.y, width: k.w, height: k.h }}
        />
      ))}
      {layout.jacks.map((j) => (
        <Jack
          key={j.id}
          jack={j}
          up={!!byId.get(j.id)?.up}
          lit={!!litPorts?.has(j.id)}
          onHover={onPortHover ? (on) => onPortHover(j.id, on) : undefined}
          onClick={() => onSelect(sw, j.id)}
        />
      ))}
    </div>
  )
}

function faceModel(sw: Switch, layout: ResolvedFaceplate): string {
  if (sw.model === 'fwsw-A') return 'FWSW12X'
  return (sw.model || layout.def.match.model[0] || layout.def.product).trim()
}

function Jack({
  jack,
  up,
  lit,
  onHover,
  onClick,
}: {
  jack: PlacedJack
  up: boolean
  lit?: boolean
  onHover?: (on: boolean) => void
  onClick: () => void
}) {
  const Icon = jack.kind === 'sfp' ? RectangleEllipsis : EthernetPort
  const icon = jack.w < 18 ? 'size-2.5' : 'size-3'
  const num = (
    <span
      className={cn(
        'text-[8px] leading-none tabular-nums',
        lit ? 'text-sky-400' : 'text-muted-foreground',
      )}
    >
      {jack.id}
    </span>
  )
  return (
    <button
      type="button"
      title={`Port ${jack.id}`}
      onClick={onClick}
      onMouseEnter={() => onHover?.(true)}
      onMouseLeave={() => onHover?.(false)}
      className="absolute z-[1] flex flex-col items-center gap-0.5"
      style={{
        left: jack.x,
        top: jack.numbers === 'above' ? jack.y - 10 : jack.y,
        width: jack.w,
      }}
    >
      {jack.numbers === 'above' ? num : null}
      <span
        className={cn(
          'inline-flex items-center justify-center rounded-sm ring-offset-1 transition-[box-shadow,background-color,color]',
          lit
            ? 'bg-sky-400 text-sky-950 ring-2 ring-sky-400/80'
            : up
              ? 'bg-foreground/80 text-background'
              : 'border border-dashed border-muted-foreground/30 text-muted-foreground/40',
        )}
        style={{ width: jack.w, height: jack.h }}
      >
        <Icon className={icon} aria-hidden />
      </span>
      {jack.numbers === 'below' ? num : null}
    </button>
  )
}
