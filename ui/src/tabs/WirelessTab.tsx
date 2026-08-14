import { useMemo } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type {
  ModuleInfo,
  WirelessAP,
  WirelessClient,
  WirelessNetwork,
  WirelessRadio,
  WirelessView,
} from '@/lib/types'
import type { WirelessMode } from '@/lib/nav'
import { cn } from '@/lib/utils'

const SECTION_HEADER = 'px-6 pt-6 pb-4'
const SECTION_TITLE = 'text-xl font-normal leading-8 text-muted-foreground'
const TABLE = 'grid w-full gap-x-3 divide-y'
const TABLE_HEAD =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2 text-xs text-muted-foreground'
const TABLE_ROW = 'col-span-full grid grid-cols-subgrid items-center px-6 py-2.5 text-sm'
const AP_COLS = 'grid-cols-[minmax(0,1fr)_7rem_4.5rem_9rem]'
const RADIO_COLS = 'grid-cols-[minmax(0,1fr)_3.5rem_3.5rem_4.5rem_5rem]'
const NET_COLS = 'grid-cols-[minmax(0,1fr)_4.5rem_9rem_4.5rem]'
const CLIENT_COLS =
  'grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)_3.5rem_minmax(0,1fr)]'
const NET_CLIENT_COLS =
  'grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_3.5rem_minmax(0,1fr)]'

const BAND_ORDER = ['2.4', '5', '6'] as const

function bandRank(band: string): number {
  const i = BAND_ORDER.indexOf(band as (typeof BAND_ORDER)[number])
  return i === -1 ? 99 : i
}

export function apLabel(ap: WirelessAP): string {
  return ap.name?.trim() || ap.mac
}

export function netKey(n: WirelessNetwork): string {
  return n.id || n.name
}

export function clientLabel(c: WirelessClient): string {
  return c.name?.trim() || c.hostname?.trim() || c.mac
}

function clientAp(c: WirelessClient): string {
  return c.ap_name?.trim() || c.ap_mac || '—'
}

export function clientOnNetwork(c: WirelessClient, n: WirelessNetwork): boolean {
  const ssid = c.ssid?.trim().toLowerCase()
  if (!ssid) return false
  const keys = [n.name, n.ssid]
    .map((s) => s?.trim().toLowerCase())
    .filter((s): s is string => !!s)
  return keys.includes(ssid)
}

function uniqueBands(radios: WirelessRadio[] | undefined): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const r of radios ?? []) {
    if (!seen.has(r.band)) {
      seen.add(r.band)
      out.push(r.band)
    }
  }
  return out.sort((a, b) => bandRank(a) - bandRank(b))
}

function BandSlots({ bands }: { bands: string[] }) {
  const on = new Set(bands)
  return (
    <div className="grid w-36 grid-cols-3 gap-1">
      {BAND_ORDER.map((b) =>
        on.has(b) ? (
          <span
            key={b}
            className="rounded border px-1.5 py-0.5 text-center text-xs tabular-nums text-muted-foreground"
          >
            {b}
          </span>
        ) : (
          <span key={b} className="px-1.5 py-0.5 text-xs opacity-0" aria-hidden>
            {b}
          </span>
        ),
      )}
    </div>
  )
}

export function WirelessTab({
  wireless,
  unifi,
  mode,
  onMode,
  onSelectAp,
  onSelectNet,
  onSelectClient,
}: {
  wireless: WirelessView | null
  unifi: ModuleInfo | null
  mode: WirelessMode
  onMode: (m: WirelessMode) => void
  onSelectAp: (ap: WirelessAP) => void
  onSelectNet: (n: WirelessNetwork) => void
  onSelectClient: (c: WirelessClient) => void
}) {
  if (!wireless) {
    const hint =
      unifi?.status === 'down'
        ? unifi.detail || 'UniFi is down.'
        : 'No wireless data yet.'
    return (
      <div className="space-y-4">
        <h1 className="text-lg font-semibold tracking-tight">Wireless</h1>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-lg font-semibold tracking-tight">Wireless</h1>
        <div className="inline-flex rounded-md border p-0.5">
          {(['radios', 'networks', 'clients'] as WirelessMode[]).map((m) => (
            <Button
              key={m}
              type="button"
              size="xs"
              variant="ghost"
              className={cn(mode === m && 'bg-accent', 'capitalize')}
              onClick={() => onMode(m)}
            >
              {m}
            </Button>
          ))}
        </div>
      </div>
      {wireless.meta.error ? (
        <p className="text-sm text-muted-foreground">{wireless.meta.error}</p>
      ) : null}
      {mode === 'radios' ? (
        <RadiosView aps={wireless.aps} onSelectAp={onSelectAp} />
      ) : mode === 'networks' ? (
        <NetworksView
          networks={wireless.networks ?? []}
          supported={wireless.meta.networks_supported}
          onSelectNet={onSelectNet}
        />
      ) : (
        <ClientsView clients={wireless.clients ?? []} onSelectClient={onSelectClient} />
      )}
    </div>
  )
}

function RadiosView({
  aps,
  onSelectAp,
}: {
  aps: WirelessAP[]
  onSelectAp: (ap: WirelessAP) => void
}) {
  const sortedAps = useMemo(
    () =>
      [...aps].sort((a, b) =>
        apLabel(a).localeCompare(apLabel(b), undefined, { sensitivity: 'base' }),
      ),
    [aps],
  )

  const radioRows = useMemo(() => {
    type Row = {
      key: string
      apName: string
      name: string
      band: string
      channel?: number
      width_mhz?: number
      clients?: number
    }
    const rows: Row[] = []
    for (const ap of sortedAps) {
      const label = apLabel(ap)
      for (const r of ap.radios ?? []) {
        rows.push({
          key: `${ap.mac}:${r.band}`,
          apName: label,
          name: `${label} · ${r.band} GHz`,
          band: r.band,
          channel: r.channel,
          width_mhz: r.width_mhz,
          clients: r.clients,
        })
      }
    }
    rows.sort((a, b) => {
      const d = a.apName.localeCompare(b.apName, undefined, { sensitivity: 'base' })
      if (d !== 0) return d
      return bandRank(a.band) - bandRank(b.band)
    })
    return rows
  }, [sortedAps])

  return (
    <>
      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Access points</CardTitle>
        </CardHeader>
        <CardContent className="px-0">
          {sortedAps.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No access points</p>
          ) : (
            <div className={cn(TABLE, AP_COLS)}>
              <div className={TABLE_HEAD}>
                <div>Name</div>
                <div>Uplink</div>
                <div>Clients</div>
                <div>Bands</div>
              </div>
              {sortedAps.map((ap) => {
                const bands = uniqueBands(ap.radios)
                return (
                  <button
                    key={ap.mac}
                    type="button"
                    className={cn(
                      TABLE_ROW,
                      'rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40',
                    )}
                    onClick={() => onSelectAp(ap)}
                  >
                    <div className="min-w-0 truncate font-medium">
                      {apLabel(ap)}
                      {ap.source === 'firewalla' ? (
                        <span className="ml-2 text-xs font-normal text-muted-foreground">Firewalla</span>
                      ) : null}
                    </div>
                    <div className="min-w-0 truncate text-muted-foreground">{ap.uplink || '—'}</div>
                    <div className="tabular-nums text-muted-foreground">{ap.clients ?? 0}</div>
                    <BandSlots bands={bands} />
                  </button>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Radios</CardTitle>
        </CardHeader>
        <CardContent className="px-0">
          {radioRows.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No radios</p>
          ) : (
            <div className={cn(TABLE, RADIO_COLS)}>
              <div className={TABLE_HEAD}>
                <div>Name</div>
                <div>Band</div>
                <div>Ch</div>
                <div>Width</div>
                <div>Clients</div>
              </div>
              {radioRows.map((r) => (
                <div key={r.key} className={TABLE_ROW}>
                  <div className="min-w-0 truncate font-medium">{r.name}</div>
                  <div className="tabular-nums text-muted-foreground">{r.band}</div>
                  <div className="tabular-nums text-muted-foreground">
                    {r.channel != null ? r.channel : '—'}
                  </div>
                  <div className="tabular-nums text-muted-foreground">
                    {r.width_mhz != null ? `${r.width_mhz}` : '—'}
                  </div>
                  <div className="tabular-nums text-muted-foreground">{r.clients ?? 0}</div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  )
}

function NetworksView({
  networks,
  supported,
  onSelectNet,
}: {
  networks: WirelessNetwork[]
  supported: boolean
  onSelectNet: (n: WirelessNetwork) => void
}) {
  const sorted = useMemo(
    () =>
      [...networks].sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
      ),
    [networks],
  )

  return (
    <Card className="gap-0 py-0">
      <CardHeader className={SECTION_HEADER}>
        <CardTitle className={SECTION_TITLE}>Networks</CardTitle>
      </CardHeader>
      <CardContent className="px-0">
        {!supported ? (
          <p className="px-6 py-6 text-sm text-muted-foreground">SSID list needs UniFi Network 10+.</p>
        ) : sorted.length === 0 ? (
          <p className="px-6 py-6 text-sm text-muted-foreground">No networks</p>
        ) : (
          <div className={cn(TABLE, NET_COLS)}>
            <div className={TABLE_HEAD}>
              <div>Name</div>
              <div>Status</div>
              <div>Bands</div>
              <div>Clients</div>
            </div>
            {sorted.map((n) => {
              const bands = [...(n.bands ?? [])].sort((a, b) => bandRank(a) - bandRank(b))
              return (
                <button
                  key={netKey(n)}
                  type="button"
                  className={cn(
                    TABLE_ROW,
                    'rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40',
                  )}
                  onClick={() => onSelectNet(n)}
                >
                  <div className="min-w-0 truncate font-medium">{n.name}</div>
                  <div>
                    {n.enabled == null ? (
                      <span className="text-muted-foreground">—</span>
                    ) : (
                      <Badge variant={n.enabled ? 'default' : 'secondary'}>
                        {n.enabled ? 'On' : 'Off'}
                      </Badge>
                    )}
                  </div>
                  <BandSlots bands={bands} />
                  <div className="tabular-nums text-muted-foreground">
                    {n.clients != null && n.clients > 0 ? n.clients : '—'}
                  </div>
                </button>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ClientsView({
  clients,
  onSelectClient,
}: {
  clients: WirelessClient[]
  onSelectClient: (c: WirelessClient) => void
}) {
  const sorted = useMemo(
    () =>
      [...clients].sort((a, b) =>
        clientLabel(a).localeCompare(clientLabel(b), undefined, { sensitivity: 'base' }),
      ),
    [clients],
  )

  return (
    <Card className="gap-0 py-0">
      <CardHeader className={SECTION_HEADER}>
        <CardTitle className={SECTION_TITLE}>Clients</CardTitle>
      </CardHeader>
      <CardContent className="px-0">
        {sorted.length === 0 ? (
          <p className="px-6 py-6 text-sm text-muted-foreground">No wireless clients</p>
        ) : (
          <div className={cn(TABLE, CLIENT_COLS)}>
            <div className={TABLE_HEAD}>
              <div>Name</div>
              <div>IP</div>
              <div>SSID</div>
              <div>Band</div>
              <div>AP</div>
            </div>
            {sorted.map((c) => (
              <button
                key={c.mac}
                type="button"
                className={cn(
                  TABLE_ROW,
                  'rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40',
                )}
                onClick={() => onSelectClient(c)}
              >
                <div className="min-w-0 truncate font-medium">{clientLabel(c)}</div>
                <div className="min-w-0 truncate font-mono text-muted-foreground">{c.ip || '—'}</div>
                <div className="min-w-0 truncate text-muted-foreground">{c.ssid || '—'}</div>
                <div className="tabular-nums text-muted-foreground">{c.band || '—'}</div>
                <div className="min-w-0 truncate text-muted-foreground">{clientAp(c)}</div>
              </button>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function WirelessAPDetail({ ap }: { ap: WirelessAP }) {
  const radios = [...(ap.radios ?? [])].sort((a, b) => bandRank(a.band) - bandRank(b.band))
  const meta: { label: string; value: string }[] = [
    { label: 'Name', value: ap.name?.trim() || '—' },
    { label: 'IP', value: ap.ip ?? '—' },
    { label: 'MAC', value: ap.mac },
    { label: 'Model', value: ap.model?.trim() || '—' },
    { label: 'Firmware', value: ap.firmware || '—' },
    { label: 'Status', value: ap.healthy ? 'Online' : 'Offline' },
  ]

  return (
    <div className="space-y-4">
      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Details</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {meta.map((r) => (
            <div key={r.label} className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">{r.label}</div>
              <div className="min-w-0 break-all font-medium">{r.value}</div>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Radios</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {radios.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No radios</p>
          ) : (
            radios.map((r) => (
              <div key={r.band} className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
                <div className="text-muted-foreground">{r.band} GHz</div>
                <div className="font-medium tabular-nums">
                  {r.channel != null ? `ch ${r.channel}` : '—'}
                  {r.width_mhz != null ? ` · ${r.width_mhz} MHz` : ''}
                  {r.clients != null ? ` · ${r.clients} clients` : ''}
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export function WirelessNetworkDetail({
  network,
  clients,
  onSelectClient,
}: {
  network: WirelessNetwork
  clients: WirelessClient[]
  onSelectClient: (c: WirelessClient) => void
}) {
  const onNet = useMemo(
    () =>
      clients
        .filter((c) => clientOnNetwork(c, network))
        .sort((a, b) =>
          clientLabel(a).localeCompare(clientLabel(b), undefined, { sensitivity: 'base' }),
        ),
    [clients, network],
  )
  const aps = useMemo(() => {
    const by = new Map<string, { name: string; n: number }>()
    for (const c of onNet) {
      const key = (c.ap_mac || c.ap_name || '').toUpperCase()
      if (!key) continue
      const cur = by.get(key)
      if (cur) cur.n++
      else by.set(key, { name: clientAp(c), n: 1 })
    }
    return [...by.values()].sort((a, b) =>
      a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
    )
  }, [onNet])
  const bands = [...(network.bands ?? [])].sort((a, b) => bandRank(a) - bandRank(b))
  const ssid = network.ssid?.trim()
  const meta: { label: string; value: string }[] = [
    { label: 'Name', value: network.name },
    ...(ssid && ssid !== network.name ? [{ label: 'SSID', value: ssid }] : []),
    {
      label: 'Status',
      value: network.enabled == null ? '—' : network.enabled ? 'On' : 'Off',
    },
    { label: 'Bands', value: bands.length ? bands.map((b) => `${b} GHz`).join(' · ') : '—' },
    { label: 'Clients', value: String(onNet.length) },
  ]

  return (
    <div className="space-y-4">
      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Details</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {meta.map((r) => (
            <div key={r.label} className="grid grid-cols-[8rem_minmax(0,1fr)] gap-4 px-6 py-3 text-sm">
              <div className="text-muted-foreground">{r.label}</div>
              <div className="min-w-0 break-all font-medium">{r.value}</div>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Access points</CardTitle>
        </CardHeader>
        <CardContent className="divide-y px-0">
          {aps.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No access points</p>
          ) : (
            aps.map((a) => (
              <div
                key={a.name}
                className="grid grid-cols-[minmax(0,1fr)_4.5rem] gap-4 px-6 py-3 text-sm"
              >
                <div className="min-w-0 truncate font-medium">{a.name}</div>
                <div className="tabular-nums text-muted-foreground">{a.n}</div>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className={SECTION_HEADER}>
          <CardTitle className={SECTION_TITLE}>Clients</CardTitle>
        </CardHeader>
        <CardContent className="px-0">
          {onNet.length === 0 ? (
            <p className="px-6 py-6 text-sm text-muted-foreground">No wireless clients</p>
          ) : (
            <div className={cn(TABLE, NET_CLIENT_COLS)}>
              <div className={TABLE_HEAD}>
                <div>Name</div>
                <div>IP</div>
                <div>Band</div>
                <div>AP</div>
              </div>
              {onNet.map((c) => (
                <button
                  key={c.mac}
                  type="button"
                  className={cn(
                    TABLE_ROW,
                    'rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40',
                  )}
                  onClick={() => onSelectClient(c)}
                >
                  <div className="min-w-0 truncate font-medium">{clientLabel(c)}</div>
                  <div className="min-w-0 truncate font-mono text-muted-foreground">{c.ip || '—'}</div>
                  <div className="tabular-nums text-muted-foreground">{c.band || '—'}</div>
                  <div className="min-w-0 truncate text-muted-foreground">{clientAp(c)}</div>
                </button>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
