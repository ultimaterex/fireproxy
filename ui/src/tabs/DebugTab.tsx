import { useMemo, useState } from 'react'
import { ArrowLeft, Bug } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch as Toggle } from '@/components/ui/switch'
import { isDebugEnabled } from '@/lib/debug'
import { forceUiUpdateBanner } from '@/lib/ui-build'
import {
  disableTopoDemo,
  enableTopoDemo,
  isTopoDemoOn,
  loadTopoDemo,
  mergeExtraSims,
} from '@/lib/topo-demo'
import type { Switch } from '@/lib/types'
import { cn } from '@/lib/utils'
import {
  addVRackSim,
  clearVRackSims,
  loadVRackSims,
  removeVRackSim,
  VRACK_SIM_PRESETS,
  type VRackSim,
  type VRackSimPreset,
} from '@/lib/vrack-sim'
import { TopologyTab } from '@/tabs/TopologyTab'

type Page = 'home' | 'vrack-sim'

export function DebugTab({
  switches,
  onSimsChange,
  onOpenTopology,
}: {
  switches: Switch[]
  onSimsChange: () => void
  onOpenTopology: () => void
}) {
  const [page, setPage] = useState<Page>('home')
  const [sims, setSims] = useState<VRackSim[]>(() => loadVRackSims())
  const [demoOn, setDemoOn] = useState(() => isTopoDemoOn())

  const refresh = () => {
    setSims(loadVRackSims())
    setDemoOn(isTopoDemoOn())
    onSimsChange()
  }

  if (!isDebugEnabled()) {
    return <p className="text-sm text-muted-foreground">Debug is off. Set DEBUG=1 in compose.</p>
  }

  if (page === 'vrack-sim') {
    return (
      <VRackSimPage
        sims={sims}
        live={switches}
        demoOn={demoOn}
        onBack={() => setPage('home')}
        onChange={refresh}
        onOpenTopology={onOpenTopology}
      />
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Bug className="size-5 text-amber-500" />
        <h1 className="text-lg font-semibold tracking-tight">Debug</h1>
      </div>
      <p className="text-sm text-muted-foreground">Dev-only tools. Not written to the server catalog.</p>
      <div className="grid gap-3 sm:grid-cols-2">
        <Card
          className="cursor-pointer gap-0 py-0 hover:bg-accent/30"
          onClick={() => setPage('vrack-sim')}
        >
          <CardHeader className="px-6 pt-5 pb-4">
            <CardTitle className="text-base font-medium">Simulate topology / VRACK</CardTitle>
          </CardHeader>
          <CardContent className="px-6 pb-5 text-sm text-muted-foreground">
            Demo rack, extra generic stubs ({sims.length}), preview Tree &amp; VRACK
            {demoOn ? ' · demo on' : ''}
          </CardContent>
        </Card>
        <Card className="gap-0 py-0">
          <CardHeader className="px-6 pt-5 pb-4">
            <CardTitle className="text-base font-medium">UI update toast</CardTitle>
          </CardHeader>
          <CardContent className="px-6 pb-5">
            <Button type="button" size="sm" variant="outline" onClick={() => forceUiUpdateBanner()}>
              Show toast
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function VRackSimPage({
  sims,
  live,
  demoOn,
  onBack,
  onChange,
  onOpenTopology,
}: {
  sims: VRackSim[]
  live: Switch[]
  demoOn: boolean
  onBack: () => void
  onChange: () => void
  onOpenTopology: () => void
}) {
  const parent = useMemo(() => {
    const demo = demoOn ? loadTopoDemo() : null
    return pickParent(demo?.switches ?? live, sims)
  }, [live, sims, demoOn])

  const preview = useMemo(() => {
    if (demoOn) {
      const d = loadTopoDemo()
      return d ? mergeExtraSims(d) : null
    }
    if (sims.length) {
      return mergeExtraSims({
        host: 'debug',
        switches: sims.map((s) => s.sw),
        tree: [],
      })
    }
    return null
  }, [demoOn, sims])

  const add = (preset: VRackSimPreset) => {
    addVRackSim(preset, parent)
    onChange()
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          onClick={onBack}
        >
          <ArrowLeft className="size-4" />
          Back
        </button>
        <h1 className="text-lg font-semibold tracking-tight">Simulate topology / VRACK</h1>
        <button
          type="button"
          className="ml-auto text-xs text-sky-400 hover:underline"
          onClick={onOpenTopology}
        >
          Open Topology tab
        </button>
      </div>

      <div className="flex items-center justify-between gap-3 rounded-lg border border-border/80 px-4 py-3">
        <div>
          <div className="text-sm font-medium">Demo topology</div>
          <div className="text-xs text-muted-foreground">
            Replaces live catalog with Gold SE + HD + Agg + Max 16 + Switch X + Flex + APs + mystery 48
          </div>
        </div>
        <Toggle
          checked={demoOn}
          onCheckedChange={(on) => {
            if (on) enableTopoDemo()
            else disableTopoDemo()
            onChange()
          }}
          aria-label="Demo topology"
        />
      </div>

      <div>
        <div className="mb-2 text-xs uppercase tracking-wide text-muted-foreground">Extra generic stubs</div>
        <p className="mb-2 text-sm text-muted-foreground">
          Parent:{' '}
          {parent ? (
            <span className="text-foreground">
              …{parent.mac.slice(-8)} · {parent.port}
            </span>
          ) : (
            'none'
          )}
        </p>
        <div className="flex flex-wrap gap-2">
          {VRACK_SIM_PRESETS.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => add(p.id)}
              className="rounded-md border border-border/80 bg-muted/25 px-3 py-2 text-left text-sm hover:bg-muted/40"
            >
              <div className="font-medium">{p.label}</div>
              <div className="text-xs text-muted-foreground">{p.detail}</div>
            </button>
          ))}
          {sims.length > 0 ? (
            <button
              type="button"
              onClick={() => {
                clearVRackSims()
                onChange()
              }}
              className="rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive hover:bg-destructive/10"
            >
              Clear extras
            </button>
          ) : null}
        </div>
      </div>

      {sims.length > 0 ? (
        <Card className="gap-0 py-0">
          <CardContent className="divide-y px-0">
            {sims.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between gap-3 px-6 py-3 text-sm"
              >
                <div className="min-w-0">
                  <div className="font-medium">{s.sw.name}</div>
                  <div className="truncate text-muted-foreground">
                    {s.sw.model} · {s.sw.mac}
                    {s.sw.parent_port ? ` · ↑ ${s.sw.parent_port}` : ''}
                  </div>
                </div>
                <button
                  type="button"
                  className={cn('text-xs text-muted-foreground hover:text-destructive')}
                  onClick={() => {
                    removeVRackSim(s.id)
                    onChange()
                  }}
                >
                  Remove
                </button>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

      <div>
        <div className="mb-2 text-xs uppercase tracking-wide text-muted-foreground">Preview</div>
        {preview ? (
          <div className="rounded-lg border border-border/80 p-3">
            <TopologyTab
              view={preview}
              network={[]}
              devices={[]}
              onOpenSwitchClients={() => undefined}
              onSelectLan={() => undefined}
              onSelectDevice={() => undefined}
            />
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">Turn on demo topology or add a stub to preview.</p>
        )}
      </div>
    </div>
  )
}

function pickParent(
  live: Switch[],
  sims: VRackSim[],
): { mac: string; port: string } | null {
  const simMacs = new Set(sims.map((s) => s.sw.mac.toUpperCase()))
  const host = live.find(
    (s) =>
      s.kind !== 'ap' &&
      s.kind !== 'box' &&
      !simMacs.has(s.mac.toUpperCase()) &&
      (s.ports?.length ?? 0) > 0,
  )
  if (!host) return null
  const port =
    host.ports?.find((p) => !p.uplink && p.up)?.id ??
    host.ports?.find((p) => !p.uplink)?.id ??
    host.ports?.[0]?.id
  if (!port) return null
  return { mac: host.mac, port }
}
