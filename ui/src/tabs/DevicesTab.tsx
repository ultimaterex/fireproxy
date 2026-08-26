import { useEffect, useMemo, useRef, useState, Fragment } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown, SlidersHorizontal, Tag as TagIcon, User, X } from 'lucide-react'

import { ContextMenu } from 'radix-ui'

import { DeviceIcon } from '@/components/DeviceIcon'
import { DeviceSearch } from '@/components/DeviceSearch'
import { SourceBadge } from '@/components/SourceBadge'
import { Toast } from '@/components/Toast'
import { Card, CardContent } from '@/components/ui/card'
import { Switch as Toggle } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import {
  catalogHasActiveStamp,
  filterDevicesByActive,
  inactiveDeviceCount,
  loadShowInactive,
  saveShowInactive,
} from '@/lib/device-active'
import { colFilterKeys, colFilterOptions } from '@/lib/device-col-filter'
import {
  DEVICE_COLS,
  defaultDeviceCols,
  loadDeviceCols,
  saveDeviceCols,
  type DeviceColId,
} from '@/lib/device-cols'
import {
  DEVICE_GROUP_BY_OPTIONS,
  groupDeviceRows,
  type DeviceGroupBy,
  type DeviceRowGroup,
} from '@/lib/device-group'
import {
  clearSearchFields,
  deviceMatchesSearch,
  parseDeviceQuery,
  setSearchField,
  type SearchKey,
} from '@/lib/device-search'
import { dapLabel, deviceOnline, fmtBytes, fmtCount, netLabel, preferredName } from '@/lib/format'
import { describeMacFilter, indexDevicePorts, type PortLoc } from '@/lib/switch-port'
import type { Device, FwAppHostPolicy, NetIface, Switch, Tag } from '@/lib/types'
import { cn } from '@/lib/utils'

type Dir = 'asc' | 'desc'

const menuItemCls =
  'cursor-pointer px-3 py-1.5 text-sm outline-none data-[disabled]:opacity-50 data-[highlighted]:bg-accent'

export function DevicesTab({
  devices,
  source,
  stale,
  reason,
  groupFilter,
  lanFilter,
  switchMacs,
  switches,
  query,
  nowMs,
  uuidToNet,
  tags,
  groupByHint,
  onGroup,
  onLan,
  onSwitchMacs,
  onQuery,
  labelTag,
  onSelectDevice,
  onRenamed,
  onGroupUpdated,
}: {
  devices: Device[]
  source?: string
  stale?: boolean
  reason?: string
  groupFilter: string
  lanFilter: string
  switchMacs: string[]
  switches: Switch[]
  query: string
  nowMs: number
  uuidToNet: Map<string, NetIface>
  tags: Tag[]
  groupByHint?: DeviceGroupBy
  onGroup: (id: string) => void
  onLan: (uuid: string) => void
  onSwitchMacs: (macs: string[]) => void
  onQuery: (q: string) => void
  labelTag: (id: string, preferType?: string) => string
  onSelectDevice?: (d: Device) => void
  onRenamed?: (mac: string, name: string) => void
  onGroupUpdated?: (mac: string, tagIds: string[]) => void
}) {
  const [sort, setSort] = useState<{ key: DeviceColId; dir: Dir }>({ key: 'name', dir: 'asc' })
  const [cols, setCols] = useState<DeviceColId[]>(() => loadDeviceCols())
  const [showInactive, setShowInactive] = useState(() => loadShowInactive())
  const [groupBy, setGroupBy] = useState<DeviceGroupBy>(() => groupByHint ?? 'none')
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const [picker, setPicker] = useState(false)
  const [groupPicker, setGroupPicker] = useState(false)
  const pickerRef = useRef<HTMLDivElement>(null)
  const groupPickerRef = useRef<HTMLDivElement>(null)
  const [wakeReady, setWakeReady] = useState(false)
  const [actionBusy, setActionBusy] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)

  const groupTags = useMemo(
    () =>
      tags
        .filter((t) => !t.type || t.type === 'group')
        .slice()
        .sort((a, b) => a.name.localeCompare(b.name)),
    [tags],
  )

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const r = await api('/v1/fw-app/status')
        if (!r.ok || cancelled) return
        const st = (await r.json()) as { paired?: boolean; state?: string }
        if (!cancelled) setWakeReady(!!st.paired && st.state === 'lan-ok')
      } catch {
        if (!cancelled) setWakeReady(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  async function fetchHostPolicy(mac: string): Promise<FwAppHostPolicy | null> {
    const r = await api(`/v1/fw-app/hosts/policy?mac=${encodeURIComponent(mac)}`, {
      cache: 'no-store',
    })
    if (!r.ok) return null
    const body = (await r.json().catch(() => ({}))) as { policy?: FwAppHostPolicy }
    return body.policy ?? null
  }

  async function postHostPolicy(
    mac: string,
    patch: {
      isolation?: boolean
      emergency?: boolean
      adblock?: boolean
      family?: boolean
      tags?: string[]
    },
  ): Promise<{ ok: boolean; error?: string; policy?: FwAppHostPolicy }> {
    const r = await api('/v1/fw-app/hosts/policy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mac, ...patch }),
      cache: 'no-store',
    })
    const body = (await r.json().catch(() => ({}))) as {
      error?: string
      policy?: FwAppHostPolicy
    }
    if (!r.ok) return { ok: false, error: body.error || 'Policy failed' }
    return { ok: true, policy: body.policy }
  }

  async function wakeDevice(d: Device) {
    if (!wakeReady || actionBusy) return
    setActionBusy(d.mac)
    try {
      const r = await api('/v1/fw-app/wol', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac: d.mac }),
      })
      const body = (await r.json().catch(() => ({}))) as { error?: string }
      if (!r.ok) {
        setToast(body.error || 'Wake failed')
        return
      }
      setToast(`Wake sent · ${preferredName(d)}`)
    } catch {
      setToast('Wake failed')
    } finally {
      setActionBusy(null)
    }
  }

  async function renameDevice(d: Device) {
    if (!wakeReady || actionBusy) return
    const next = window.prompt('Name', preferredName(d))
    if (next == null) return
    const name = next.trim()
    if (!name) {
      setToast('Name required')
      return
    }
    setActionBusy(d.mac)
    try {
      const r = await api('/v1/fw-app/hosts/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac: d.mac, name }),
      })
      const body = (await r.json().catch(() => ({}))) as {
        error?: string
        name?: string
        unifi_warning?: string
      }
      if (!r.ok) {
        setToast(body.error || 'Rename failed')
        return
      }
      const saved = body.name || name
      onRenamed?.(d.mac, saved)
      setToast(body.unifi_warning ? `Renamed · ${body.unifi_warning}` : 'Renamed')
    } catch {
      setToast('Rename failed')
    } finally {
      setActionBusy(null)
    }
  }

  async function setDeviceGroup(d: Device, tagId: string | null) {
    if (!wakeReady || actionBusy) return
    const tagsPatch = tagId ? [tagId] : []
    setActionBusy(d.mac)
    try {
      const res = await postHostPolicy(d.mac, { tags: tagsPatch })
      if (!res.ok) {
        setToast(res.error || 'Policy failed')
        return
      }
      onGroupUpdated?.(d.mac, res.policy?.tags ?? tagsPatch)
      setToast(tagId ? `Group · ${labelTag(tagId, 'group')}` : 'Group cleared')
    } catch {
      setToast('Policy failed')
    } finally {
      setActionBusy(null)
    }
  }

  async function togglePolicyFlag(d: Device, key: 'adblock' | 'family') {
    if (!wakeReady || actionBusy) return
    setActionBusy(d.mac)
    try {
      const pol = await fetchHostPolicy(d.mac)
      if (!pol) {
        setToast('Policy failed')
        return
      }
      const next = !pol[key]
      const res = await postHostPolicy(d.mac, { [key]: next })
      if (!res.ok) {
        setToast(res.error || 'Policy failed')
        return
      }
      setToast(next ? `${key === 'adblock' ? 'Adblock' : 'Family'} on` : `${key === 'adblock' ? 'Adblock' : 'Family'} off`)
    } catch {
      setToast('Policy failed')
    } finally {
      setActionBusy(null)
    }
  }

  async function toggleIsolation(d: Device) {
    if (!wakeReady || actionBusy) return
    setActionBusy(d.mac)
    try {
      const pol = await fetchHostPolicy(d.mac)
      if (!pol) {
        setToast('Policy failed')
        return
      }
      const next = !pol.isolated
      const ok = window.confirm(
        next ? 'Isolate this device from the internet?' : 'Remove isolation for this device?',
      )
      if (!ok) return
      const res = await postHostPolicy(d.mac, { isolation: next })
      if (!res.ok) {
        setToast(res.error || 'Policy failed')
        return
      }
      setToast(next ? 'Isolated' : 'Isolation removed')
    } catch {
      setToast('Policy failed')
    } finally {
      setActionBusy(null)
    }
  }

  async function toggleEmergency(d: Device) {
    if (!wakeReady || actionBusy) return
    setActionBusy(d.mac)
    try {
      const pol = await fetchHostPolicy(d.mac)
      if (!pol) {
        setToast('Policy failed')
        return
      }
      const next = !pol.emergency
      const ok = window.confirm(
        next ? 'Enable emergency access (bypass host ACLs)?' : 'Disable emergency access?',
      )
      if (!ok) return
      const res = await postHostPolicy(d.mac, { emergency: next })
      if (!res.ok) {
        setToast(res.error || 'Policy failed')
        return
      }
      setToast(next ? 'Emergency on' : 'Emergency off')
    } catch {
      setToast('Policy failed')
    } finally {
      setActionBusy(null)
    }
  }

  useEffect(() => {
    if (groupByHint) setGroupBy(groupByHint)
  }, [groupByHint])

  useEffect(() => {
    setCollapsed(new Set())
  }, [groupBy])

  useEffect(() => {
    saveDeviceCols(cols)
  }, [cols])

  useEffect(() => {
    saveShowInactive(showInactive)
  }, [showInactive])

  useEffect(() => {
    if (!picker && !groupPicker) return
    const onDoc = (e: MouseEvent) => {
      if (pickerRef.current?.contains(e.target as Node)) return
      if (groupPickerRef.current?.contains(e.target as Node)) return
      setPicker(false)
      setGroupPicker(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [picker, groupPicker])

  const portOf = useMemo(() => indexDevicePorts(switches), [switches])

  const visibleDevices = useMemo(
    () => filterDevicesByActive(devices, showInactive),
    [devices, showInactive],
  )
  const hiddenInactive = inactiveDeviceCount(devices)
  const hasStamp = catalogHasActiveStamp(devices)

  const scoped = useMemo(
    () =>
      visibleDevices.filter((d) => {
        if (groupFilter && !(d.tag_ids ?? []).includes(groupFilter) && !(d.user_tag_ids ?? []).includes(groupFilter)) {
          return false
        }
        if (lanFilter && d.intf_uuid !== lanFilter) return false
        if (switchMacs.length > 0 && !switchMacs.includes(d.mac.toUpperCase())) return false
        return true
      }),
    [visibleDevices, groupFilter, lanFilter, switchMacs],
  )

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    const out = scoped
      .map((d) => {
        const membership = primaryMembership(d, labelTag)
        const online = deviceOnline(d, nowMs)
        const lan = d.intf_uuid ? uuidToNet.get(d.intf_uuid) : undefined
        const loc = portOf.get(d.mac.toUpperCase())
        return { d, membership, online, lan, loc }
      })
      .filter(({ d }) => {
        if (!q) return true
        return deviceMatchesSearch(d, query, { nowMs, uuidToNet, labelTag, portOf })
      })

    const dir = sort.dir === 'asc' ? 1 : -1
    out.sort((a, b) => {
      const cmp = compareRow(a, b, sort.key)
      return cmp === 0 ? preferredName(a.d).localeCompare(preferredName(b.d)) : cmp * dir
    })
    return out
  }, [scoped, query, nowMs, sort, labelTag, uuidToNet, portOf])

  const lanName = lanFilter ? uuidToNet.get(lanFilter) : undefined
  const groupName = groupFilter ? labelTag(groupFilter) : ''
  const switchChip = switchMacs.length > 0 ? describeMacFilter(switchMacs, switches) : ''
  const grouped = useMemo(
    () =>
      groupDeviceRows(rows, groupBy, (r) => ({
        online: r.online,
        membership: r.membership,
        loc: r.loc,
        lan: r.lan ? { uuid: r.lan.uuid, label: netLabel(r.lan) } : undefined,
      })),
    [rows, groupBy],
  )
  const groupByLabel = DEVICE_GROUP_BY_OPTIONS.find((o) => o.id === groupBy)?.label ?? 'None'

  const toggleSort = (key: DeviceColId) => {
    setSort((s) =>
      s.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: key === 'name' ? 'asc' : 'desc' },
    )
  }

  const toggleCol = (id: DeviceColId, locked?: boolean) => {
    if (locked) return
    setCols((cur) => (cur.includes(id) ? cur.filter((c) => c !== id) : [...cur, id]))
  }

  const toggleCollapsed = (key: string) => {
    setCollapsed((cur) => {
      const next = new Set(cur)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  const visible = DEVICE_COLS.filter((c) => cols.includes(c.id))

  return (
    <Card className="gap-4 py-6">
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-baseline gap-2">
            <h1 className="text-lg font-semibold tracking-tight">Devices</h1>
            <span className="text-sm tabular-nums text-muted-foreground">
              {rows.length} / {visibleDevices.length}
            </span>
            <SourceBadge source={source} stale={stale} reason={reason} />
            {hasStamp && !showInactive && hiddenInactive > 0 ? (
              <span className="text-sm tabular-nums text-muted-foreground">+{hiddenInactive} inactive</span>
            ) : null}
          </div>
          {hasStamp ? (
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <Toggle checked={showInactive} onCheckedChange={setShowInactive} />
              Show inactive
            </label>
          ) : null}
        </div>
        <DeviceSearch
          value={query}
          onChange={onQuery}
          devices={visibleDevices}
          tags={tags}
          uuidToNet={uuidToNet}
          switches={switches}
        />
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative" ref={groupPickerRef}>
            <button
              type="button"
              className="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs text-muted-foreground hover:text-foreground"
              onClick={() => setGroupPicker((v) => !v)}
            >
              Group by: {groupByLabel}
              <ChevronDown className="size-3" />
            </button>
            {groupPicker ? (
              <div className="absolute top-8 left-0 z-30 min-w-36 rounded-lg border bg-popover py-1 shadow-md">
                {DEVICE_GROUP_BY_OPTIONS.map((o) => (
                  <button
                    key={o.id}
                    type="button"
                    className={cn(
                      'flex w-full px-3 py-1.5 text-left text-sm hover:bg-accent',
                      o.id === groupBy && 'text-[#027BFF]',
                    )}
                    onClick={() => {
                      setGroupBy(o.id)
                      setGroupPicker(false)
                    }}
                  >
                    {o.label}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          {groupFilter ? (
            <FilterChip label={groupName || groupFilter} onClear={() => onGroup('')} />
          ) : null}
          {lanFilter ? (
            <FilterChip label={lanName?.desc || lanName?.name || lanFilter} onClear={() => onLan('')} />
          ) : null}
          {switchMacs.length > 0 ? (
            <FilterChip label={switchChip} onClear={() => onSwitchMacs([])} />
          ) : null}
        </div>
        {devices.length === 0 ? (
          <p className="py-8 text-sm text-muted-foreground">Waiting on catalog</p>
        ) : rows.length === 0 ? (
          <p className="py-8 text-sm text-muted-foreground">No devices</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                {visible.map((c) => (
                  <SortHead
                    key={c.id}
                    id={c.id}
                    label={c.label}
                    active={sort.key === c.id}
                    dir={sort.dir}
                    query={query}
                    options={colFilterOptions(c.id, scoped, { uuidToNet, labelTag, portOf })}
                    onClick={() => toggleSort(c.id)}
                    onQuery={onQuery}
                  />
                ))}
                <TableHead className="w-10 text-right">
                  <div className="relative inline-block" ref={pickerRef}>
                    <button
                      type="button"
                      className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                      aria-label="Columns"
                      onClick={() => setPicker((v) => !v)}
                    >
                      <SlidersHorizontal className="size-4" />
                    </button>
                    {picker ? (
                      <div className="absolute top-9 right-0 z-30 w-56 rounded-lg border bg-popover py-1 shadow-md">
                        <div className="flex items-center justify-between px-3 py-1.5 text-xs text-muted-foreground">
                          <span>{cols.length} selected</span>
                          <button type="button" className="hover:text-foreground" onClick={() => setCols(defaultDeviceCols())}>
                            Reset
                          </button>
                        </div>
                        {DEVICE_COLS.map((c) => {
                          const on = cols.includes(c.id)
                          return (
                            <label
                              key={c.id}
                              className={cn(
                                'flex items-center gap-2 px-3 py-1.5 text-sm',
                                c.locked ? 'cursor-default opacity-60' : 'cursor-pointer hover:bg-accent',
                              )}
                            >
                              <input
                                type="checkbox"
                                className="size-3.5 accent-[#027BFF]"
                                checked={on}
                                disabled={c.locked}
                                onChange={() => toggleCol(c.id, c.locked)}
                              />
                              {c.label}
                            </label>
                          )
                        })}
                      </div>
                    ) : null}
                  </div>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {grouped.map((g) => (
                <DeviceGroup
                  key={g.key}
                  group={g}
                  colSpan={visible.length + 1}
                  visible={visible}
                  open={!collapsed.has(g.key)}
                  groupFilter={groupFilter}
                  groupTags={groupTags}
                  labelTag={labelTag}
                  onToggle={() => toggleCollapsed(g.key)}
                  onGroup={onGroup}
                  onLan={onLan}
                  onPort={onSwitchMacs}
                  onSelectDevice={onSelectDevice}
                  wakeReady={wakeReady}
                  actionBusy={actionBusy}
                  onWake={wakeDevice}
                  onRename={renameDevice}
                  onSetGroup={setDeviceGroup}
                  onToggleAdblock={(d) => void togglePolicyFlag(d, 'adblock')}
                  onToggleFamily={(d) => void togglePolicyFlag(d, 'family')}
                  onToggleIsolation={toggleIsolation}
                  onToggleEmergency={toggleEmergency}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
      {toast
        ? createPortal(<Toast message={toast} onDismiss={() => setToast(null)} />, document.body)
        : null}
    </Card>
  )
}

type Row = {
  d: Device
  membership: { id: string; name: string; kind: 'user' | 'group' } | null
  online: boolean
  lan?: NetIface
  loc?: PortLoc
}

function DeviceGroup({
  group,
  colSpan,
  visible,
  open,
  groupFilter,
  groupTags,
  labelTag,
  onToggle,
  onGroup,
  onLan,
  onPort,
  onSelectDevice,
  wakeReady,
  actionBusy,
  onWake,
  onRename,
  onSetGroup,
  onToggleAdblock,
  onToggleFamily,
  onToggleIsolation,
  onToggleEmergency,
}: {
  group: DeviceRowGroup<Row>
  colSpan: number
  visible: { id: DeviceColId }[]
  open: boolean
  groupFilter: string
  groupTags: Tag[]
  labelTag: (id: string, preferType?: string) => string
  onToggle: () => void
  onGroup: (id: string) => void
  onLan: (uuid: string) => void
  onPort: (macs: string[]) => void
  onSelectDevice?: (d: Device) => void
  wakeReady: boolean
  actionBusy: string | null
  onWake: (d: Device) => void
  onRename: (d: Device) => void
  onSetGroup: (d: Device, tagId: string | null) => void
  onToggleAdblock: (d: Device) => void
  onToggleFamily: (d: Device) => void
  onToggleIsolation: (d: Device) => void
  onToggleEmergency: (d: Device) => void
}) {
  const onFilter = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (group.clients.length > 0) {
      onPort(group.clients)
      return
    }
    if (group.lanUuid) {
      onLan(group.lanUuid)
      return
    }
    if (group.tagId) {
      onGroup(group.tagId === groupFilter ? '' : group.tagId)
    }
  }
  const canFilter = group.clients.length > 0 || !!group.lanUuid || !!group.tagId

  return (
    <>
      {group.label ? (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={colSpan} className="bg-muted/40 py-2">
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
                aria-expanded={open}
                onClick={onToggle}
              >
                <ChevronDown
                  className={cn(
                    'size-3.5 shrink-0 transition-transform',
                    !open && '-rotate-90',
                  )}
                />
                <span>{group.label}</span>
                <span className="font-mono tabular-nums text-muted-foreground/70">{group.rows.length}</span>
              </button>
              {canFilter ? (
                <button
                  type="button"
                  className="text-xs text-[#027BFF] hover:underline"
                  onClick={onFilter}
                >
                  Filter
                </button>
              ) : null}
            </div>
          </TableCell>
        </TableRow>
      ) : null}
      {open
        ? group.rows.map((row) => {
            const cells = (
              <>
                {visible.map((c) => (
                  <TableCell key={c.id}>
                    {renderCell(c.id, row, { groupFilter, onGroup, onLan, onPort })}
                  </TableCell>
                ))}
                <TableCell />
              </>
            )
            const rowEl = (
              <TableRow
                className={cn('h-14', onSelectDevice && 'cursor-pointer')}
                onClick={() => onSelectDevice?.(row.d)}
              >
                {cells}
              </TableRow>
            )
            if (!wakeReady) return <Fragment key={row.d.mac}>{rowEl}</Fragment>
            const busy = actionBusy != null
            return (
              <ContextMenu.Root key={row.d.mac}>
                <ContextMenu.Trigger asChild>{rowEl}</ContextMenu.Trigger>
                <ContextMenu.Portal>
                  <ContextMenu.Content className="z-50 min-w-44 rounded-lg border bg-popover py-1 shadow-md">
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => onWake(row.d)}
                    >
                      Wake
                    </ContextMenu.Item>
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => onRename(row.d)}
                    >
                      Rename…
                    </ContextMenu.Item>
                    <ContextMenu.Sub>
                      <ContextMenu.SubTrigger className={menuItemCls} disabled={busy}>
                        Set group…
                      </ContextMenu.SubTrigger>
                      <ContextMenu.Portal>
                        <ContextMenu.SubContent className="z-50 min-w-36 rounded-lg border bg-popover py-1 shadow-md">
                          <ContextMenu.Item
                            className={menuItemCls}
                            disabled={busy}
                            onSelect={() => onSetGroup(row.d, null)}
                          >
                            None
                          </ContextMenu.Item>
                          {groupTags.map((t) => (
                            <ContextMenu.Item
                              key={t.id}
                              className={menuItemCls}
                              disabled={busy}
                              onSelect={() => onSetGroup(row.d, t.id)}
                            >
                              {labelTag(t.id, 'group') !== t.id
                                ? labelTag(t.id, 'group')
                                : t.name || t.id}
                            </ContextMenu.Item>
                          ))}
                        </ContextMenu.SubContent>
                      </ContextMenu.Portal>
                    </ContextMenu.Sub>
                    <ContextMenu.Separator className="my-1 h-px bg-border" />
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => onToggleAdblock(row.d)}
                    >
                      Adblock on/off
                    </ContextMenu.Item>
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => onToggleFamily(row.d)}
                    >
                      Family on/off
                    </ContextMenu.Item>
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => void onToggleIsolation(row.d)}
                    >
                      Isolate…
                    </ContextMenu.Item>
                    <ContextMenu.Item
                      className={menuItemCls}
                      disabled={busy}
                      onSelect={() => void onToggleEmergency(row.d)}
                    >
                      Emergency…
                    </ContextMenu.Item>
                    {onSelectDevice ? (
                      <>
                        <ContextMenu.Separator className="my-1 h-px bg-border" />
                        <ContextMenu.Item
                          className={menuItemCls}
                          disabled={busy}
                          onSelect={() => onSelectDevice(row.d)}
                        >
                          Open device
                        </ContextMenu.Item>
                      </>
                    ) : null}
                  </ContextMenu.Content>
                </ContextMenu.Portal>
              </ContextMenu.Root>
            )
          })
        : null}
    </>
  )
}

function renderCell(
  id: DeviceColId,
  row: Row,
  ctx: {
    groupFilter: string
    onGroup: (id: string) => void
    onLan: (uuid: string) => void
    onPort: (macs: string[]) => void
  },
) {
  const { d, membership, online, lan, loc } = row
  switch (id) {
    case 'name':
      return (
        <span className="inline-flex max-w-64 items-center gap-2 font-medium">
          <DeviceIcon type={d.type} className="size-4 text-[#027BFF]" />
          <span className="truncate">{preferredName(d)}</span>
        </span>
      )
    case 'group':
      return membership ? (
        <button
          type="button"
          className="inline-flex items-center gap-1.5 text-[#027BFF] hover:underline"
          onClick={(e) => {
            e.stopPropagation()
            ctx.onGroup(membership.id === ctx.groupFilter ? '' : membership.id)
          }}
        >
          {membership.kind === 'user' ? <User className="size-3.5" /> : <TagIcon className="size-3.5" />}
          {membership.name}
        </button>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    case 'status':
      return (
        <span className="inline-flex items-center gap-2">
          <span className={cn('size-2 rounded-full', online ? 'bg-emerald-500' : 'bg-zinc-500')} />
          {online ? 'Online' : 'Offline'}
        </span>
      )
    case 'dap':
      return d.dap ? (
        <span title={d.dap.reason || undefined}>{dapLabel(d.dap.status)}</span>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    case 'learned':
      return d.dap ? <span className="font-mono tabular-nums">{fmtCount(d.dap.learned)}</span> : <span className="text-muted-foreground">—</span>
    case 'trusted':
      return d.dap ? <span className="font-mono tabular-nums">{fmtCount(d.dap.trusted)}</span> : <span className="text-muted-foreground">—</span>
    case 'blocked':
      return d.dap ? <span className="font-mono tabular-nums">{fmtCount(d.dap.blocked)}</span> : <span className="text-muted-foreground">—</span>
    case 'ip':
      return <span className="font-mono">{d.ip || '—'}</span>
    case 'port':
      return loc ? (
        <span className="inline-flex items-center gap-1">
          <button
            type="button"
            className="text-[#027BFF] hover:underline"
            onClick={(e) => {
              e.stopPropagation()
              ctx.onPort(loc.clients)
            }}
          >
            {loc.uplink ? 'Uplink' : loc.portId}
          </button>
          <span className="text-muted-foreground">·</span>
          <button
            type="button"
            className="text-muted-foreground hover:text-foreground hover:underline"
            onClick={(e) => {
              e.stopPropagation()
              ctx.onPort(loc.switchClients.length ? loc.switchClients : loc.clients)
            }}
          >
            {loc.switchName}
          </button>
        </span>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    case 'network':
      return lan?.uuid ? (
        <button
          type="button"
          className="text-[#027BFF] hover:underline"
          onClick={(e) => {
            e.stopPropagation()
            ctx.onLan(lan.uuid!)
          }}
        >
          {netLabel(lan)}
        </button>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    case 'ipv6':
      return d.ipv6?.length ? (
        <span className="block max-w-52 truncate font-mono text-xs" title={d.ipv6.join('\n')}>
          {d.ipv6.join(', ')}
        </span>
      ) : (
        <span className="text-muted-foreground">—</span>
      )
    case 'mac':
      return <span className="font-mono text-muted-foreground">{d.mac}</span>
    case 'vendor':
      return <span className="max-w-56 truncate text-muted-foreground">{d.vendor || 'Unknown'}</span>
    case 'download':
      return <span className="font-mono tabular-nums">{fmtBytes(d.download ?? 0)}</span>
    case 'upload':
      return <span className="font-mono tabular-nums">{fmtBytes(d.upload ?? 0)}</span>
  }
}

function primaryMembership(
  d: Device,
  labelTag: (id: string, preferType?: string) => string,
): { id: string; name: string; kind: 'user' | 'group' } | null {
  const user = d.user_tag_ids?.[0]
  if (user) return { id: user, name: labelTag(user, 'user'), kind: 'user' }
  const group = d.tag_ids?.[0]
  if (group) return { id: group, name: labelTag(group, 'group'), kind: 'group' }
  return null
}

function compareRow(a: Row, b: Row, key: DeviceColId): number {
  switch (key) {
    case 'name':
      return preferredName(a.d).localeCompare(preferredName(b.d))
    case 'group':
      return (a.membership?.name ?? '').localeCompare(b.membership?.name ?? '')
    case 'status':
      return Number(b.online) - Number(a.online)
    case 'dap':
      return dapLabel(a.d.dap?.status).localeCompare(dapLabel(b.d.dap?.status))
    case 'learned':
      return (a.d.dap?.learned ?? -1) - (b.d.dap?.learned ?? -1)
    case 'trusted':
      return (a.d.dap?.trusted ?? -1) - (b.d.dap?.trusted ?? -1)
    case 'blocked':
      return (a.d.dap?.blocked ?? -1) - (b.d.dap?.blocked ?? -1)
    case 'ip':
      return (a.d.ip ?? '').localeCompare(b.d.ip ?? '', undefined, { numeric: true })
    case 'port':
      return (a.loc?.label ?? '').localeCompare(b.loc?.label ?? '', undefined, { numeric: true })
    case 'network':
      return (a.lan ? netLabel(a.lan) : '').localeCompare(b.lan ? netLabel(b.lan) : '')
    case 'ipv6':
      return (a.d.ipv6?.[0] ?? '').localeCompare(b.d.ipv6?.[0] ?? '')
    case 'mac':
      return a.d.mac.localeCompare(b.d.mac)
    case 'vendor':
      return (a.d.vendor || 'Unknown').localeCompare(b.d.vendor || 'Unknown')
    case 'download':
      return (a.d.download ?? 0) - (b.d.download ?? 0)
    case 'upload':
      return (a.d.upload ?? 0) - (b.d.upload ?? 0)
  }
}

function SortHead({
  id,
  label,
  active,
  dir,
  query,
  options,
  onClick,
  onQuery,
}: {
  id: DeviceColId
  label: string
  active: boolean
  dir: Dir
  query: string
  options: { key: SearchKey; value: string }[]
  onClick: () => void
  onQuery: (q: string) => void
}) {
  const keys = colFilterKeys(id)
  const activeKeys = new Set(parseDeviceQuery(query).fields.map((f) => f.key))
  const filtered = keys.some((k) => activeKeys.has(k))
  const head = (
    <button type="button" className="-mx-2 inline-flex h-10 items-center gap-1 px-2 hover:text-foreground" onClick={onClick}>
      {label}
      {active ? <span className="text-[10px] text-muted-foreground">{dir === 'asc' ? '↑' : '↓'}</span> : null}
    </button>
  )
  if (options.length === 0) {
    return <TableHead>{head}</TableHead>
  }
  return (
    <ContextMenu.Root>
      <TableHead>
        <ContextMenu.Trigger asChild>{head}</ContextMenu.Trigger>
        <ContextMenu.Portal>
          <ContextMenu.Content className="z-50 max-h-80 min-w-44 overflow-auto rounded-lg border bg-popover py-1 shadow-md">
            {filtered ? (
              <ContextMenu.Item
                className="cursor-pointer px-3 py-1.5 text-sm text-muted-foreground outline-none data-[highlighted]:bg-accent data-[highlighted]:text-foreground"
                onSelect={() => onQuery(clearSearchFields(query, keys))}
              >
                Clear
              </ContextMenu.Item>
            ) : null}
            {options.map((o) => (
              <ContextMenu.Item
                key={`${o.key}:${o.value}`}
                className="cursor-pointer px-3 py-1.5 text-sm outline-none data-[highlighted]:bg-accent"
                onSelect={() => onQuery(setSearchField(query, o.key, o.value))}
              >
                {o.value}
              </ContextMenu.Item>
            ))}
          </ContextMenu.Content>
        </ContextMenu.Portal>
      </TableHead>
    </ContextMenu.Root>
  )
}

function FilterChip({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <button
      type="button"
      className="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs"
      onClick={onClear}
    >
      {label}
      <X className="size-3" />
    </button>
  )
}
