import { useEffect, useRef, useState, type ReactNode } from 'react'
import { ChevronDown } from 'lucide-react'

import { Card, CardContent } from '@/components/ui/card'
import {
  isOfflineSnoozed,
  snoozeOffline,
  syncOfflineSnoozes,
  unsnoozeOffline,
} from '@/lib/audit-snooze'
import type {
  AuditView,
  NameSyncRow,
  OfflineAuditRow,
  PendingAuditRow,
  STPAuditRow,
  UnknownAuditRow,
  VLANAuditRow,
} from '@/lib/types'
import { cn } from '@/lib/utils'

export type AuditSectionId = 'names' | 'vlan' | 'stp' | 'unknown' | 'offline' | 'pending'

export type AuditFocus = {
  section: AuditSectionId
  mac?: string
  nonce: number
}

const TABLE = 'grid w-full gap-x-3 divide-y'
const TABLE_HEAD =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2 text-xs text-muted-foreground'
const TABLE_ROW =
  'col-span-full grid grid-cols-subgrid items-center px-6 py-2.5 text-sm'
const ROW_BTN =
  'w-full cursor-pointer rounded-none border-0 bg-transparent text-left font-normal hover:bg-accent/40'
const NAME_COLS = 'grid-cols-[minmax(0,1fr)_minmax(0,1fr)_5.5rem]'
const VLAN_COLS = 'grid-cols-[minmax(0,1fr)_5.5rem_5.5rem_5.5rem]'
const STP_COLS = 'grid-cols-[minmax(0,1fr)_7.5rem_minmax(0,1.2fr)]'
const UNKNOWN_COLS = 'grid-cols-[minmax(0,1fr)_7.5rem]'
const OFFLINE_COLS = 'grid-cols-[minmax(0,1fr)_5.5rem_4.5rem]'
const PENDING_COLS = 'grid-cols-[minmax(0,1fr)_minmax(0,1fr)]'

export function AuditTab({
  data,
  focus,
  onSnoozeChange,
  onName,
  onVlan,
  onStp,
  onUnknown,
  onOffline,
  onPending,
}: {
  data: AuditView | null
  focus?: AuditFocus | null
  onSnoozeChange?: () => void
  onName: (row: NameSyncRow) => void
  onVlan: (row: VLANAuditRow) => void
  onStp: (row: STPAuditRow) => void
  onUnknown: (row: UnknownAuditRow) => void
  onOffline: (row: OfflineAuditRow) => void
  onPending: (row: PendingAuditRow) => void
}) {
  const names = data?.names.count ?? 0
  const vlan = data?.vlan.count ?? 0
  const stp = data?.stp.count ?? 0
  const unknown = data?.unknown.count ?? 0
  const offline = data?.offline.count ?? 0
  const pending = data?.pending.count ?? 0
  const empty = names + vlan + stp + unknown + offline + pending === 0
  const showSX = (data?.vlan.rows ?? []).some((r) => r.sx_vid != null)
  const [, setTick] = useState(0)
  const bump = () => {
    setTick((n) => n + 1)
    onSnoozeChange?.()
  }

  useEffect(() => {
    if (!data?.offline.rows) return
    syncOfflineSnoozes(data.offline.rows.map((r) => r.mac))
  }, [data?.offline.rows])

  const focused = (id: AuditSectionId) => focus?.section === id

  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold tracking-tight">Audit</h1>

      <div className="grid grid-cols-3 gap-3">
        <CountCell label="Names" value={names} />
        <CountCell label="VLAN" value={vlan} />
        <CountCell label="STP" value={stp} />
        <CountCell label="Unknown" value={unknown} soft />
        <CountCell label="Offline" value={offline} />
        <CountCell label="Pending" value={pending} />
      </div>

      {data == null ? (
        <p className="text-sm text-muted-foreground">Waiting on catalog</p>
      ) : empty ? (
        <p className="text-sm text-muted-foreground">No findings</p>
      ) : (
        <>
          {names > 0 ? (
            <AuditSection
              title="Names"
              count={names}
              defaultOpen
              forceOpen={focused('names')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, NAME_COLS)}>
                <div className={TABLE_HEAD}>
                  <div>Firewalla</div>
                  <div>UniFi</div>
                  <div>Status</div>
                </div>
                {data.names.rows.map((row) => (
                  <FocusRow key={row.mac} active={focused('names') && focusMac(focus, row.mac)}>
                    <button
                      type="button"
                      className={cn(TABLE_ROW, ROW_BTN)}
                      onClick={() => onName(row)}
                    >
                      <div className="min-w-0 truncate font-medium">{row.firewalla_name}</div>
                      <div className="min-w-0 truncate text-muted-foreground">
                        {row.unifi_name || '—'}
                      </div>
                      <div
                        className={cn(
                          'text-xs',
                          row.status !== 'conflict' && 'text-muted-foreground',
                        )}
                      >
                        {row.status}
                      </div>
                    </button>
                  </FocusRow>
                ))}
              </div>
            </AuditSection>
          ) : null}

          {vlan > 0 ? (
            <AuditSection
              title="VLAN"
              count={vlan}
              defaultOpen
              forceOpen={focused('vlan')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, showSX ? VLAN_COLS : 'grid-cols-[minmax(0,1fr)_5.5rem_5.5rem]')}>
                <div className={TABLE_HEAD}>
                  <div>Name</div>
                  <div>FW vid</div>
                  <div>UniFi vid</div>
                  {showSX ? <div>SX vid</div> : null}
                </div>
                {data.vlan.rows.map((row) => (
                  <FocusRow key={row.mac} active={focused('vlan') && focusMac(focus, row.mac)}>
                    <button
                      type="button"
                      className={cn(TABLE_ROW, ROW_BTN)}
                      onClick={() => onVlan(row)}
                    >
                      <div className="min-w-0 truncate font-medium">{row.name?.trim() || row.mac}</div>
                      <div className="font-mono tabular-nums text-muted-foreground">{row.fw_vid}</div>
                      <div className="font-mono tabular-nums text-muted-foreground">{row.unifi_vid}</div>
                      {showSX ? (
                        <div className="font-mono tabular-nums text-muted-foreground">
                          {row.sx_vid != null ? row.sx_vid : '—'}
                        </div>
                      ) : null}
                    </button>
                  </FocusRow>
                ))}
              </div>
            </AuditSection>
          ) : null}

          {stp > 0 ? (
            <AuditSection
              title="STP"
              count={stp}
              defaultOpen
              forceOpen={focused('stp')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, STP_COLS)}>
                <div className={TABLE_HEAD}>
                  <div>Name</div>
                  <div>Kind</div>
                  <div>Detail</div>
                </div>
                {data.stp.rows.map((row, i) => (
                  <FocusRow
                    key={`${row.mac}-${row.kind}-${i}`}
                    active={focused('stp') && focusMac(focus, row.mac)}
                  >
                    <button
                      type="button"
                      className={cn(TABLE_ROW, ROW_BTN)}
                      onClick={() => onStp(row)}
                    >
                      <div className="min-w-0 truncate font-medium">{row.name?.trim() || row.mac}</div>
                      <div className="truncate text-muted-foreground">{row.kind}</div>
                      <div className="min-w-0 truncate text-muted-foreground">{row.detail || '—'}</div>
                    </button>
                  </FocusRow>
                ))}
              </div>
            </AuditSection>
          ) : null}

          {unknown > 0 ? (
            <AuditSection
              title="Unknown"
              count={unknown}
              defaultOpen={false}
              soft
              forceOpen={focused('unknown')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, UNKNOWN_COLS)}>
                <div className={TABLE_HEAD}>
                  <div>Name</div>
                  <div>Side</div>
                </div>
                {data.unknown.rows.map((row) => (
                  <FocusRow
                    key={`${row.side}-${row.mac}`}
                    active={focused('unknown') && focusMac(focus, row.mac)}
                  >
                    <button
                      type="button"
                      className={cn(TABLE_ROW, ROW_BTN)}
                      onClick={() => onUnknown(row)}
                    >
                      <div className="min-w-0 truncate font-medium">{row.name?.trim() || row.mac}</div>
                      <div className="truncate text-muted-foreground">
                        {row.side === 'fw_only' ? 'FW only' : 'UniFi only'}
                      </div>
                    </button>
                  </FocusRow>
                ))}
              </div>
            </AuditSection>
          ) : null}

          {offline > 0 ? (
            <AuditSection
              title="Offline"
              count={offline}
              defaultOpen
              forceOpen={focused('offline')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, OFFLINE_COLS)}>
                <div className={TABLE_HEAD}>
                  <div>Name</div>
                  <div>Kind</div>
                  <div />
                </div>
                {data.offline.rows.map((row) => {
                  const snoozed = isOfflineSnoozed(row.mac)
                  return (
                    <FocusRow
                      key={row.mac}
                      active={focused('offline') && focusMac(focus, row.mac)}
                      className={snoozed ? 'opacity-50' : undefined}
                    >
                      <div className={cn(TABLE_ROW)}>
                        <button
                          type="button"
                          className={cn(ROW_BTN, 'col-span-2 grid grid-cols-subgrid')}
                          onClick={() => onOffline(row)}
                        >
                          <div className="min-w-0 truncate font-medium">{row.name?.trim() || row.mac}</div>
                          <div className="truncate text-muted-foreground">{row.kind}</div>
                        </button>
                        <div className="flex justify-end">
                          {snoozed ? (
                            <button
                              type="button"
                              className="text-xs text-muted-foreground hover:text-foreground"
                              onClick={(e) => {
                                e.stopPropagation()
                                unsnoozeOffline(row.mac)
                                bump()
                              }}
                            >
                              Unsnooze
                            </button>
                          ) : (
                            <button
                              type="button"
                              className="text-xs text-muted-foreground hover:text-foreground"
                              onClick={(e) => {
                                e.stopPropagation()
                                snoozeOffline(row.mac)
                                bump()
                              }}
                            >
                              Snooze
                            </button>
                          )}
                        </div>
                      </div>
                    </FocusRow>
                  )
                })}
              </div>
            </AuditSection>
          ) : null}

          {pending > 0 ? (
            <AuditSection
              title="Pending"
              count={pending}
              defaultOpen
              forceOpen={focused('pending')}
              focusNonce={focus?.nonce}
            >
              <div className={cn(TABLE, PENDING_COLS)}>
                <div className={TABLE_HEAD}>
                  <div>Name</div>
                  <div>Model</div>
                </div>
                {data.pending.rows.map((row) => (
                  <FocusRow key={row.mac} active={focused('pending') && focusMac(focus, row.mac)}>
                    <button
                      type="button"
                      className={cn(TABLE_ROW, ROW_BTN)}
                      onClick={() => onPending(row)}
                    >
                      <div className="min-w-0 truncate font-medium">{row.name?.trim() || row.mac}</div>
                      <div className="min-w-0 truncate text-muted-foreground">{row.model || '—'}</div>
                    </button>
                  </FocusRow>
                ))}
              </div>
            </AuditSection>
          ) : null}
        </>
      )}
    </div>
  )
}

function focusMac(focus: AuditFocus | null | undefined, mac: string): boolean {
  if (!focus?.mac) return false
  return focus.mac.toUpperCase() === mac.toUpperCase()
}

function FocusRow({
  active,
  className,
  children,
}: {
  active: boolean
  className?: string
  children: ReactNode
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!active || !ref.current) return
    ref.current.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [active])
  return (
    <div
      ref={ref}
      className={cn(
        'col-span-full grid grid-cols-subgrid',
        active && 'bg-accent/50',
        className,
      )}
    >
      {children}
    </div>
  )
}

function AuditSection({
  title,
  count,
  defaultOpen,
  soft,
  forceOpen,
  focusNonce,
  children,
}: {
  title: string
  count: number
  defaultOpen: boolean
  soft?: boolean
  forceOpen?: boolean
  focusNonce?: number
  children: ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!forceOpen) return
    setOpen(true)
    const t = window.setTimeout(() => {
      ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }, 50)
    return () => window.clearTimeout(t)
  }, [forceOpen, focusNonce])

  return (
    <Card ref={ref} className="gap-0 py-0">
      <button
        type="button"
        className="flex w-full items-center gap-2 px-6 py-4 text-left hover:bg-accent/30"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <ChevronDown
          className={cn(
            'size-4 shrink-0 text-muted-foreground transition-transform',
            !open && '-rotate-90',
          )}
        />
        <span
          className={cn(
            'text-xl font-normal leading-8',
            soft ? 'text-muted-foreground/80' : 'text-muted-foreground',
          )}
        >
          {title}
        </span>
        <span className="ml-auto font-mono text-sm tabular-nums text-muted-foreground">{count}</span>
      </button>
      {open ? <CardContent className="px-0 pb-2">{children}</CardContent> : null}
    </Card>
  )
}

function CountCell({ label, value, soft }: { label: string; value: number; soft?: boolean }) {
  return (
    <Card className={cn('gap-1 px-5 py-4', soft && 'opacity-70')}>
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="font-mono text-3xl tabular-nums tracking-tight">{value}</p>
    </Card>
  )
}
