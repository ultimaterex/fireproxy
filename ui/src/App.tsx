import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  Archive,
  Bell,
  Box,
  Bug,
  Cable,
  ClipboardList,
  History,
  MonitorSmartphone,
  Network,
  PanelLeftClose,
  PanelLeftOpen,
  ScrollText,
  Settings,
  Shield,
  Users,
  Wifi,
} from 'lucide-react'

import { ViewToggle } from '@/components/ViewToggle'
import { Breadcrumb } from '@/components/Breadcrumb'
import { UpdateBanner } from '@/components/UpdateBanner'
import {
  DeviceDetail,
  deviceFromCatalog,
  groupNameFor,
} from '@/components/DeviceDetail'
import { RegionDetail } from '@/components/RegionDetail'
import {
  anonymizeAudit,
  anonymizeBox,
  anonymizeDashboard,
  anonymizeDevices,
  anonymizeHistory,
  anonymizeLatest,
  anonymizeNetwork,
  anonymizePolicies,
  anonymizeTags,
  anonymizeTopo,
  anonymizeUnifi,
  anonymizeWireless,
  createAnon,
  lanCidrsFromNetwork,
} from '@/lib/anonymity'
import { anonymitySalt, isAnonymityOn, subscribeAnonymity } from '@/lib/anonymity-on'
import { api, AUTH_EVENT } from '@/lib/api'
import { isDebugEnabled } from '@/lib/debug'
import { LoginPage } from '@/pages/LoginPage'
import { netLabel, preferredName, affiliatedUserByGroup, resolveTagName } from '@/lib/format'
import {
  crumbLabel,
  findLast,
  popTo,
  pushFrame,
  resetTab,
  tabOf,
  type NavFrame,
  type WirelessMode,
} from '@/lib/nav'
import { indexDevicePorts, isWholeSwitchFilter } from '@/lib/switch-port'
import {
  DAY,
  SEEN_OPTIONS,
  type ChartRange,
  type BoxInfo,
  type Dashboard,
  type Device,
  type HistoryPoint,
  type LatestView,
  type ModuleInfo,
  type NetIface,
  type PersistInfo,
  type AgentHealth,
  type Policy,
  type SeenId,
  type Tab,
  type Tag,
  type TopoView,
  type ViewMode,
  type UnifiConsole,
  type WirelessView,
  type AuditView,
} from '@/lib/types'
import { cn } from '@/lib/utils'
import { loadVRackSims } from '@/lib/vrack-sim'
import {
  isTopoDemoOn,
  loadTopoDemo,
  mergeExtraSims,
} from '@/lib/topo-demo'
import { loadAllViewModes, saveViewMode } from '@/lib/view-mode'
import { DebugTab } from '@/tabs/DebugTab'
import { DevicesTab } from '@/tabs/DevicesTab'
import { InventoryTab } from '@/tabs/InventoryTab'
import { GroupsTab } from '@/tabs/GroupsTab'
import { LegacyTab, type LegacyPage } from '@/tabs/LegacyTab'
import { MetricsTab } from '@/tabs/MetricsTab'
import { NetworkTab } from '@/tabs/NetworkTab'
import { RulesTab } from '@/tabs/RulesTab'
import { SettingsTab } from '@/tabs/SettingsTab'
import { TopologyTab } from '@/tabs/TopologyTab'
import { WirelessTab, WirelessAPDetail, WirelessNetworkDetail, apLabel, netKey, clientLabel } from '@/tabs/WirelessTab'
import { AuditTab, type AuditFocus, type AuditSectionId } from '@/tabs/AuditTab'
import { HistoryTab } from '@/tabs/HistoryTab'
import { LogsTab } from '@/tabs/LogsTab'
import {
  countUnsnoozedOffline,
  isOfflineSnoozed,
} from '@/lib/audit-snooze'

const NAV: { id: Tab; label: string; icon: typeof Activity }[] = [
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'inventory', label: 'Inventory', icon: Box },
  { id: 'network', label: 'Network', icon: Cable },
  { id: 'topology', label: 'Topology', icon: Network },
  { id: 'wireless', label: 'Wireless', icon: Wifi },
  { id: 'audit', label: 'Audit', icon: ClipboardList },
  { id: 'history', label: 'History', icon: History },
  { id: 'devices', label: 'Devices', icon: MonitorSmartphone },
  { id: 'rules', label: 'Rules', icon: Shield },
  { id: 'groups', label: 'Groups', icon: Users },
  { id: 'logs', label: 'Logs', icon: ScrollText },
]

const NAV_COLLAPSED_KEY = 'fp-nav-collapsed'

function loadNavCollapsed(): boolean {
  try {
    return localStorage.getItem(NAV_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

const MODE_TABS: Tab[] = ['rules', 'groups']
const TABS: Tab[] = [...NAV.map((n) => n.id), 'settings', 'debug', 'legacy']

function App() {
  const debugOn = isDebugEnabled()
  const [stack, setStack] = useState<NavFrame[]>(() => resetTab('metrics'))
  const tab = tabOf(stack)
  const [modes, setModes] = useState<Record<Tab, ViewMode>>(() => loadAllViewModes(TABS))
  const [wirelessMode, setWirelessMode] = useState<WirelessMode>('radios')
  const [view, setView] = useState<LatestView | null>(null)
  const [history, setHistory] = useState<HistoryPoint[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [network, setNetwork] = useState<NetIface[]>([])
  const [topoView, setTopoView] = useState<TopoView | null>(null)
  const [wireless, setWireless] = useState<WirelessView | null>(null)
  const [audit, setAudit] = useState<AuditView | null>(null)
  const [simTick, setSimTick] = useState(0)
  const [policies, setPolicies] = useState<Policy[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [invMeta, setInvMeta] = useState<{ ts?: number; host?: string }>({})
  const [error, setError] = useState<string | null>(null)
  const [seenId, setSeenId] = useState<SeenId>('1w')
  const [chartRange, setChartRange] = useState<ChartRange>('6h')
  const [persist, setPersist] = useState<PersistInfo>({})
  const [agentOnline, setAgentOnline] = useState(false)
  const [query, setQuery] = useState('')
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [box, setBox] = useState<BoxInfo | null>(null)
  const [nowMs, setNowMs] = useState(() => Date.now())
  const [legacyPage, setLegacyPage] = useState<LegacyPage>('metrics')
  const [unifiMod, setUnifiMod] = useState<ModuleInfo | null>(null)
  const [tplinkMod, setTPLinkMod] = useState<ModuleInfo | null>(null)
  const [unifiConsole, setUnifiConsole] = useState<UnifiConsole | null>(null)
  const [settingsOpen, setSettingsOpen] = useState<string | null>(null)
  const [notifOpen, setNotifOpen] = useState(false)
  const [auditFocus, setAuditFocus] = useState<AuditFocus | null>(null)
  const [navCollapsed, setNavCollapsed] = useState(loadNavCollapsed)
  const [anonOn, setAnonOn] = useState(isAnonymityOn)
  const anonBoot = useRef(true)
  const [auth, setAuth] = useState<'checking' | 'needed' | 'ok'>('checking')
  const [oidcEnabled, setOidcEnabled] = useState(false)
  const [oidcName, setOidcName] = useState('')

  useEffect(() => subscribeAnonymity(() => setAnonOn(isAnonymityOn())), [])

  useEffect(() => {
    const onNeed = () => setAuth('needed')
    window.addEventListener(AUTH_EVENT, onNeed)
    return () => window.removeEventListener(AUTH_EVENT, onNeed)
  }, [])

  useEffect(() => {
    let cancelled = false
    async function checkAuth() {
      try {
        const [optRes, meRes] = await Promise.all([
          api('/v1/auth/login-options'),
          api('/v1/auth/me'),
        ])
        if (cancelled) return
        if (optRes.ok) {
          const opts = (await optRes.json()) as { oidc_enabled?: boolean; oidc_name?: string }
          setOidcEnabled(!!opts.oidc_enabled)
          setOidcName(opts.oidc_name?.trim() || '')
        }
        if (meRes.ok) {
          const me = (await meRes.json()) as { authenticated?: boolean }
          if (me.authenticated) {
            setAuth('ok')
            return
          }
        }
        setAuth(meRes.status === 401 || !meRes.ok ? 'needed' : 'ok')
      } catch {
        if (!cancelled) setAuth('needed')
      }
    }
    void checkAuth()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (anonBoot.current) {
      anonBoot.current = false
      return
    }
    setStack((s) => {
      const tabFrame = s.find((f) => f.kind === 'tab')
      return tabFrame ? [tabFrame] : resetTab('metrics')
    })
    setAuditFocus(null)
  }, [anonOn])

  const anon = useMemo(() => (anonOn ? createAnon(anonymitySalt()) : null), [anonOn])

  useEffect(() => {
    if (auth !== 'ok') return
    let cancelled = false
    async function load() {
      setNowMs(Date.now())
      try {
        const [mRes, hRes, dRes, nRes, tRes, wRes, pRes, gRes, healthRes, dashRes, boxRes, modRes, uRes, aRes] =
          await Promise.all([
          api('/v1/metrics/latest'),
          api(`/v1/metrics/history?range=${chartRange}`),
          api('/v1/devices'),
          api('/v1/network'),
          api('/v1/topology'),
          api('/v1/wireless'),
          api('/v1/policies'),
          api('/v1/tags'),
          api('/v1/health'),
          api('/v1/dashboard'),
          api('/v1/box'),
          api('/v1/modules'),
          api('/v1/unifi'),
          api('/v1/audit'),
        ])
        if (!cancelled) setError(null)

        if (mRes.ok) {
          const data = (await mRes.json()) as LatestView
          if (!cancelled) setView(data)
        } else if (mRes.status === 404 && !cancelled) {
          setView(null)
        }

        if (hRes.ok) {
          const data = (await hRes.json()) as { points?: HistoryPoint[] }
          if (!cancelled) setHistory(data.points ?? [])
        }

        if (dRes.ok) {
          const data = (await dRes.json()) as {
            ts: number
            host: string
            devices: Device[]
          }
          if (!cancelled) {
            setDevices(data.devices ?? [])
            setInvMeta({ ts: data.ts, host: data.host })
          }
        } else if (dRes.status === 404 && !cancelled) {
          setDevices([])
        }

        if (nRes.ok) {
          const data = (await nRes.json()) as { network: NetIface[] }
          if (!cancelled) setNetwork(data.network ?? [])
        }

        if (tRes.ok) {
          const data = (await tRes.json()) as TopoView
          if (!cancelled) {
            setTopoView({
              ts: data.ts,
              host: data.host,
              box: data.box,
              switches: data.switches ?? [],
              tree: data.tree ?? [],
              wan_type: data.wan_type,
            })
          }
        } else if (tRes.status === 404 && !cancelled) {
          setTopoView(null)
        }

        if (wRes.ok) {
          const data = (await wRes.json()) as WirelessView
          if (!cancelled) setWireless(data)
        } else if (wRes.status === 404 && !cancelled) {
          setWireless(null)
        }

        if (aRes.ok) {
          const data = (await aRes.json()) as AuditView
          if (!cancelled) setAudit(data)
        }

        if (uRes.ok) {
          const data = (await uRes.json()) as UnifiConsole
          if (!cancelled) setUnifiConsole(data)
        } else if (uRes.status === 404 && !cancelled) {
          setUnifiConsole(null)
        }

        if (pRes.ok) {
          const data = (await pRes.json()) as { policies: Policy[] }
          if (!cancelled) setPolicies(data.policies ?? [])
        }

        if (gRes.ok) {
          const data = (await gRes.json()) as { tags: Tag[] }
          if (!cancelled) setTags(data.tags ?? [])
        }

        if (healthRes.ok) {
          const data = (await healthRes.json()) as { persist?: PersistInfo; agent?: AgentHealth }
          if (!cancelled) {
            setPersist(data.persist ?? {})
            setAgentOnline(!!data.agent?.online)
          }
        }

        if (dashRes.ok) {
          const data = (await dashRes.json()) as Dashboard
          if (!cancelled) setDashboard(data)
        } else if (dashRes.status === 404 && !cancelled) {
          setDashboard(null)
        }

        if (boxRes.ok) {
          const data = (await boxRes.json()) as { box?: BoxInfo }
          if (!cancelled) setBox(data.box ?? null)
        } else if (boxRes.status === 404 && !cancelled) {
          setBox(null)
        }

        if (modRes.ok) {
          const data = (await modRes.json()) as { modules?: ModuleInfo[] }
          if (!cancelled) {
            setUnifiMod(data.modules?.find((m) => m.id === 'unifi-sync') ?? null)
            setTPLinkMod(data.modules?.find((m) => m.id === 'tplink-sync') ?? null)
          }
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      }
    }
    void load()
    const id = window.setInterval(load, 5000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [chartRange, auth])

  const cidrs = useMemo(() => (anon ? lanCidrsFromNetwork(anon, network) : []), [anon, network])
  const showNetwork = useMemo(
    () => (anon ? anonymizeNetwork(anon, network) : network),
    [anon, network],
  )
  const showDevices = useMemo(
    () => (anon ? anonymizeDevices(anon, devices, cidrs) : devices),
    [anon, devices, cidrs],
  )
  const showTags = useMemo(() => (anon ? anonymizeTags(anon, tags) : tags), [anon, tags])
  const showView = useMemo(() => (anon && view ? anonymizeLatest(anon, view) : view), [anon, view])
  const showHistory = useMemo(
    () => (anon ? anonymizeHistory(anon, history) : history),
    [anon, history],
  )
  const showDash = useMemo(
    () => (anon && dashboard ? anonymizeDashboard(anon, dashboard, cidrs) : dashboard),
    [anon, dashboard, cidrs],
  )
  const showBox = useMemo(() => (anon && box ? anonymizeBox(anon, box) : box), [anon, box])
  const showWireless = useMemo(
    () => (anon && wireless ? anonymizeWireless(anon, wireless, cidrs) : wireless),
    [anon, wireless, cidrs],
  )
  const showAudit = useMemo(
    () => (anon && audit ? anonymizeAudit(anon, audit, cidrs) : audit),
    [anon, audit, cidrs],
  )
  const showPolicies = useMemo(
    () => (anon ? anonymizePolicies(anon, policies, cidrs) : policies),
    [anon, policies, cidrs],
  )
  const showUnifi = useMemo(
    () => (anon && unifiConsole ? anonymizeUnifi(anon, unifiConsole, cidrs) : unifiConsole),
    [anon, unifiConsole, cidrs],
  )
  const showNowMs = anon ? nowMs + anon.offsetSec * 1000 : nowMs
  const showInvMeta =
    anon && invMeta.host
      ? { ...invMeta, ts: invMeta.ts ? anon.shiftTS(invMeta.ts) : invMeta.ts, host: anon.fakeName('host', invMeta.host) }
      : invMeta

  const uuidToNet = useMemo(() => {
    const m = new Map<string, NetIface>()
    for (const n of showNetwork) {
      if (n.uuid) m.set(n.uuid, n)
    }
    return m
  }, [showNetwork])

  const tagByKey = useMemo(() => {
    const m = new Map<string, Tag>()
    for (const t of showTags) {
      m.set(`${t.type ?? 'group'}:${t.id}`, t)
      if (!m.has(t.id) || t.type === 'group') m.set(t.id, t)
    }
    return m
  }, [showTags])

  const groupTags = useMemo(
    () => showTags.filter((t) => !t.type || t.type === 'group'),
    [showTags],
  )

  const afUsers = useMemo(() => affiliatedUserByGroup(showTags), [showTags])

  const labelTag = (id: string, preferType?: string) => {
    if (preferType) {
      const t = tagByKey.get(`${preferType}:${id}`)
      if (t) return resolveTagName(t, afUsers)
    }
    const t = tagByKey.get(id)
    return t ? resolveTagName(t, afUsers) : id
  }

  const lanFilter = findLast(stack, 'lan')?.uuid ?? ''
  const groupFilter = findLast(stack, 'group')?.id ?? ''
  const switchMacs = findLast(stack, 'ports')?.macs ?? []
  const selectedApMac = findLast(stack, 'ap')?.mac
  const selectedNetKey = findLast(stack, 'ssid')?.key
  const selectedDeviceMac = findLast(stack, 'device')?.mac
  const selectedRegionCc = findLast(stack, 'region')?.cc

  const seenMs = SEEN_OPTIONS.find((o) => o.id === seenId)?.ms ?? 7 * DAY

  const filteredDevices = useMemo(() => {
    const q = query.trim().toLowerCase()
    const cutoff = (showNowMs - seenMs) / 1000
    return showDevices
      .filter((d) => {
        if (!d.last_active_ts || d.last_active_ts < cutoff) return false
        if (groupFilter && !(d.tag_ids ?? []).includes(groupFilter)) return false
        if (lanFilter && d.intf_uuid !== lanFilter) return false
        if (!q) return true
        const lan = d.intf_uuid ? uuidToNet.get(d.intf_uuid) : undefined
        const hay = [
          preferredName(d),
          d.ip,
          d.mac,
          d.vendor,
          d.local_domain,
          lan?.desc,
          lan?.name,
          ...(d.tag_ids ?? []).map((id) => labelTag(id, 'group')),
          ...(d.device_tag_ids ?? []).map((id) => labelTag(id, 'device')),
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()
        return hay.includes(q)
      })
      .sort((a, b) => (b.last_active_ts ?? 0) - (a.last_active_ts ?? 0))
  }, [showDevices, seenMs, query, showNowMs, groupFilter, lanFilter, uuidToNet, tagByKey, afUsers])

  const groups = useMemo(() => {
    return groupTags
      .map((t) => {
        const user = afUsers.get(t.id)
        return {
          ...t,
          name: user ? user.name : t.name,
          // surface as user when this group is a user's affiliated device group
          kind: user ? ('user' as const) : ('group' as const),
          count: showDevices.filter((d) => (d.tag_ids ?? []).includes(t.id)).length,
        }
      })
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [groupTags, showDevices, afUsers])

  const setMode = (mode: ViewMode) => {
    setModes((prev) => ({ ...prev, [tab]: mode }))
    saveViewMode(tab, mode)
  }

  const selectTab = (next: Tab) => {
    setQuery('')
    setWirelessMode('radios')
    if (next !== 'settings') setSettingsOpen(null)
    setStack(resetTab(next))
  }

  useEffect(() => {
    if (debugOn) return
    if (tab === 'debug' || tab === 'legacy') selectTab('metrics')
  }, [debugOn, tab])

  const names = unifiMod?.audit_names ?? unifiMod?.name_sync_pending ?? audit?.names.count ?? 0
  const vlan = unifiMod?.audit_vlan ?? audit?.vlan.count ?? 0
  const stp = unifiMod?.audit_stp ?? audit?.stp.count ?? 0
  const pending = unifiMod?.audit_pending ?? audit?.pending.count ?? 0
  const settingsHealthy =
    agentOnline &&
    (!unifiMod?.enabled || unifiMod.status === 'ok') &&
    (!tplinkMod?.enabled || tplinkMod.status === 'ok' || tplinkMod.status === 'ready')
  const [snoozeRev, setSnoozeRev] = useState(0)
  const offlineMacs = audit?.offline.rows.map((r) => r.mac) ?? null
  const offline = useMemo(() => {
    if (offlineMacs == null) return unifiMod?.audit_offline ?? 0
    return countUnsnoozedOffline(offlineMacs)
  }, [offlineMacs, unifiMod?.audit_offline, snoozeRev])
  // Unknown is soft: Audit tab only, never the bell.
  const notifCount = names + vlan + stp + offline + pending
  const notifRef = useRef<HTMLDivElement>(null)

  const openAudit = (section: AuditSectionId, mac?: string) => {
    setNotifOpen(false)
    const focusMac = mac && anon ? anon.fakeMAC(mac) : mac
    setAuditFocus((prev) => ({
      section,
      mac: focusMac,
      nonce: (prev?.nonce ?? 0) + 1,
    }))
    selectTab('audit')
  }

  const firstUnsnoozedOfflineMac = () => {
    const rows = audit?.offline.rows ?? []
    const hit = rows.find((r) => !isOfflineSnoozed(r.mac))
    return hit?.mac
  }

  useEffect(() => {
    if (!notifOpen) return
    const onDoc = (e: MouseEvent) => {
      if (notifRef.current && !notifRef.current.contains(e.target as Node)) {
        setNotifOpen(false)
      }
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [notifOpen])

  const goDevicesLan = (uuid: string) => {
    const lan = uuidToNet.get(uuid)
    setStack([
      { kind: 'tab', tab: 'network' },
      { kind: 'lan', uuid, label: lan ? netLabel(lan) : uuid },
    ])
  }

  const goDevicesGroup = (id: string) => {
    setStack([
      { kind: 'tab', tab: 'groups' },
      { kind: 'group', id, label: labelTag(id, 'group') },
    ])
  }

  const goDevicesSwitch = (info: { macs: string[]; switchMac: string; switchName: string }) => {
    const macs = info.macs.map((m) => m.toUpperCase())
    const label = info.switchName.trim() || info.switchMac
    setStack([
      { kind: 'tab', tab: 'topology' },
      { kind: 'topo-switch', mac: info.switchMac.toUpperCase(), label },
      { kind: 'ports', macs, label: 'Clients' },
    ])
  }

  const openDevice = (mac: string, label: string) => {
    setStack((s) => {
      const next = s.filter((f) => f.kind !== 'device')
      return pushFrame(next, { kind: 'device', mac: mac.toUpperCase(), label })
    })
  }

  const openRegion = (cc: string, label: string) => {
    setStack((s) => {
      const base = s.filter((f) => f.kind !== 'region' && f.kind !== 'device')
      const root = base[0]?.kind === 'tab' ? base : resetTab('metrics')
      return pushFrame(root, { kind: 'region', cc: cc.toUpperCase(), label })
    })
  }

  const tabLabel = (t: Tab) => NAV.find((n) => n.id === t)?.label ?? t

  const crumbs = stack.map((f, i) => ({
    label: crumbLabel(f, tabLabel),
    onClick: i < stack.length - 1 ? () => setStack(popTo(stack, i)) : undefined,
  }))

  const showDevicesTable =
    (tab === 'devices' && !selectedDeviceMac) ||
    !!findLast(stack, 'lan') ||
    !!findLast(stack, 'group') ||
    !!findLast(stack, 'ports')

  const deviceByMac = useMemo(() => {
    const m = new Map<string, Device>()
    for (const d of showDevices) m.set(d.mac.toUpperCase(), d)
    return m
  }, [showDevices])

  const wifiByMac = useMemo(() => {
    const m = new Map<string, (typeof showWireless extends null ? never : NonNullable<typeof showWireless>['clients'][number])>()
    for (const c of showWireless?.clients ?? []) m.set(c.mac.toUpperCase(), c)
    return m
  }, [showWireless?.clients])

  const selectedAp = selectedApMac
    ? showWireless?.aps.find((a) => a.mac.toUpperCase() === selectedApMac.toUpperCase())
    : undefined
  const selectedNet = selectedNetKey
    ? showWireless?.networks.find((n) => netKey(n) === selectedNetKey)
    : undefined
  const selectedDevice = selectedDeviceMac
    ? deviceFromCatalog(deviceByMac.get(selectedDeviceMac.toUpperCase()), wifiByMac.get(selectedDeviceMac.toUpperCase()))
    : null

  const selectedRegion = useMemo(() => {
    if (!selectedRegionCc || !showDash?.top_regions) return null
    const cc = selectedRegionCc.toUpperCase()
    return (
      showDash.top_regions.find(
        (r) => (r.country || r.id || '').toUpperCase() === cc,
      ) ?? null
    )
  }, [selectedRegionCc, showDash?.top_regions])

  const topoViewWithSims = useMemo(() => {
    if (!debugOn) return topoView
    void simTick
    const demo = isTopoDemoOn() ? loadTopoDemo() ?? null : null
    const base = demo ?? topoView
    if (!base) {
      const sims = loadVRackSims().map((s) => s.sw)
      if (!sims.length) return null
      return mergeExtraSims({ host: 'debug', switches: sims, tree: [] })
    }
    return mergeExtraSims(base)
  }, [topoView, debugOn, simTick])

  const showTopo = useMemo(
    () => (anon && topoViewWithSims ? anonymizeTopo(anon, topoViewWithSims, cidrs) : topoViewWithSims),
    [anon, topoViewWithSims, cidrs],
  )

  const wanType = showBox?.wan_type ?? showTopo?.wan_type

  const portIndex = useMemo(
    () => indexDevicePorts(showTopo?.switches ?? []),
    [showTopo],
  )

  if (auth === 'checking') {
    return <div className="min-h-svh bg-background" />
  }
  if (auth === 'needed') {
    return (
      <LoginPage
        oidcEnabled={oidcEnabled}
        oidcName={oidcName}
        onAuthed={() => setAuth('ok')}
      />
    )
  }

  return (
    <div className="flex min-h-svh">
      <UpdateBanner />
      <aside
        className={cn(
          'sticky top-0 flex h-svh shrink-0 flex-col bg-[#17171a] text-white transition-[width]',
          navCollapsed ? 'w-14' : 'w-50',
        )}
      >
        <div
          className={cn(
            'flex items-center py-3',
            navCollapsed ? 'justify-center px-1' : 'justify-between gap-2 px-3',
          )}
        >
          {!navCollapsed ? (
            <div className="truncate px-2 text-sm font-semibold tracking-tight">FireProxy</div>
          ) : null}
          <button
            type="button"
            className="rounded-md p-2 text-white/70 hover:bg-white/5 hover:text-white"
            aria-label={navCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            title={navCollapsed ? 'Expand' : 'Collapse'}
            onClick={() => {
              setNavCollapsed((c) => {
                const next = !c
                try {
                  localStorage.setItem(NAV_COLLAPSED_KEY, next ? '1' : '0')
                } catch {
                  /* ignore */
                }
                return next
              })
            }}
          >
            {navCollapsed ? <PanelLeftOpen className="size-5" /> : <PanelLeftClose className="size-5" />}
          </button>
        </div>
        <nav className="flex flex-1 flex-col">
          {NAV.map((item) => {
            const Icon = item.icon
            const on = tab === item.id
            return (
              <button
                key={item.id}
                type="button"
                title={navCollapsed ? item.label : undefined}
                aria-label={item.label}
                className={cn(
                  'flex h-10 w-full items-center text-sm text-white/90 hover:bg-white/5',
                  navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
                  on && 'bg-[#3f3f44] text-white',
                )}
                onClick={() => selectTab(item.id)}
              >
                <Icon className="size-6 shrink-0" />
                {!navCollapsed ? item.label : null}
              </button>
            )
          })}
          <div className="flex-1" />
          {anonOn ? (
            <div
              className={cn(
                'mx-3 mb-1 rounded-md bg-white/10 px-2 py-1 text-[11px] text-white/80',
                navCollapsed && 'mx-1 px-0 text-center',
              )}
              title="Anonymity"
            >
              {navCollapsed ? 'A' : 'Anonymity'}
            </div>
          ) : null}
          {debugOn ? (
            <button
              type="button"
              title={navCollapsed ? 'Debug' : undefined}
              aria-label="Debug"
              className={cn(
                'flex h-10 w-full items-center text-sm text-amber-200/90 hover:bg-white/5',
                navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
                tab === 'debug' && 'bg-[#3f3f44] text-amber-100',
              )}
              onClick={() => selectTab('debug')}
            >
              <Bug className="size-6 shrink-0" />
              {!navCollapsed ? 'Debug' : null}
            </button>
          ) : null}
          <button
            type="button"
            title={navCollapsed ? 'Settings' : undefined}
            aria-label="Settings"
            className={cn(
              'relative flex h-10 w-full items-center text-sm text-white/90 hover:bg-white/5',
              navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
              tab === 'settings' && 'bg-[#3f3f44] text-white',
            )}
            onClick={() => {
              setSettingsOpen(null)
              selectTab('settings')
            }}
          >
            <span className="relative shrink-0">
              <Settings className="size-6" />
              {navCollapsed ? (
                <span
                  className={cn(
                    'absolute -right-0.5 -top-0.5 size-2 rounded-full',
                    settingsHealthy ? 'bg-emerald-500' : 'bg-red-500',
                  )}
                />
              ) : null}
            </span>
            {!navCollapsed ? (
              <>
                Settings
                <span
                  className={cn(
                    'ml-auto size-2 rounded-full',
                    settingsHealthy ? 'bg-emerald-500' : 'bg-red-500',
                  )}
                />
              </>
            ) : null}
          </button>
          {debugOn ? (
            <button
              type="button"
              title={navCollapsed ? 'Legacy' : undefined}
              aria-label="Legacy"
              className={cn(
                'flex h-10 w-full items-center text-sm text-white/45 hover:bg-white/5 hover:text-white/70',
                navCollapsed ? 'justify-center px-0' : 'gap-3 px-6 text-left',
                tab === 'legacy' && 'bg-[#3f3f44] text-white/80',
              )}
              onClick={() => selectTab('legacy')}
            >
              <Archive className="size-6 shrink-0" />
              {!navCollapsed ? 'Legacy' : null}
            </button>
          ) : null}
          <a
            href="https://github.com/ultimaterex/fireproxy"
            target="_blank"
            rel="noopener noreferrer"
            title="Source"
            aria-label="Source code"
            className={cn(
              'mb-2 flex h-8 items-center text-[11px] text-white/40 hover:text-white/70',
              navCollapsed ? 'justify-center px-0' : 'px-6',
            )}
          >
            {navCollapsed ? '<>' : 'Source'}
          </a>
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex h-14 shrink-0 items-center justify-end gap-3 border-b bg-background/80 px-5 backdrop-blur-md">
          {tab === 'legacy' && legacyPage === 'devices' ? (
            <ViewToggle
              value={modes.devices}
              onChange={(m) => {
                setModes((prev) => ({ ...prev, devices: m }))
                saveViewMode('devices', m)
              }}
            />
          ) : MODE_TABS.includes(tab) ? (
            <ViewToggle value={modes[tab]} onChange={setMode} />
          ) : null}
          <div className="relative" ref={notifRef}>
            <button
              type="button"
              className="relative rounded-md p-2 text-muted-foreground hover:bg-muted hover:text-foreground"
              aria-label="Notifications"
              onClick={() => setNotifOpen((o) => !o)}
            >
              <Bell className="size-5" />
              {notifCount > 0 ? (
                <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-medium text-white">
                  {notifCount > 99 ? '99+' : notifCount}
                </span>
              ) : null}
            </button>
            {notifOpen ? (
              <div className="absolute right-0 top-full z-30 mt-1 w-64 rounded-md border bg-background shadow-md">
                <div className="flex items-center justify-between border-b px-3 py-2">
                  <span className="text-sm font-medium">Notifications</span>
                  {notifCount > 0 ? (
                    <span className="font-mono text-xs tabular-nums text-muted-foreground">{notifCount}</span>
                  ) : null}
                </div>
                {notifCount > 0 ? (
                  <div className="py-1">
                    {names > 0 ? (
                      <button
                        type="button"
                        className="flex w-full px-3 py-2.5 text-left text-sm hover:bg-muted"
                        onClick={() => openAudit('names')}
                      >
                        {names} {names === 1 ? 'name' : 'names'} to review
                      </button>
                    ) : null}
                    {vlan > 0 ? (
                      <button
                        type="button"
                        className="flex w-full px-3 py-2.5 text-left text-sm hover:bg-muted"
                        onClick={() => openAudit('vlan')}
                      >
                        {vlan} VLAN {vlan === 1 ? 'mismatch' : 'mismatches'}
                      </button>
                    ) : null}
                    {stp > 0 ? (
                      <button
                        type="button"
                        className="flex w-full px-3 py-2.5 text-left text-sm hover:bg-muted"
                        onClick={() => openAudit('stp')}
                      >
                        {stp} STP
                      </button>
                    ) : null}
                    {offline > 0 ? (
                      <button
                        type="button"
                        className="flex w-full px-3 py-2.5 text-left text-sm hover:bg-muted"
                        onClick={() => openAudit('offline', firstUnsnoozedOfflineMac())}
                      >
                        {offline} offline
                      </button>
                    ) : null}
                    {pending > 0 ? (
                      <button
                        type="button"
                        className="flex w-full px-3 py-2.5 text-left text-sm hover:bg-muted"
                        onClick={() => openAudit('pending')}
                      >
                        {pending} pending
                      </button>
                    ) : null}
                  </div>
                ) : (
                  <div className="px-3 py-2.5 text-sm text-muted-foreground">No findings</div>
                )}
                <div className="border-t">
                  <button
                    type="button"
                    className="flex w-full px-3 py-2.5 text-left text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
                    onClick={() => {
                      setNotifOpen(false)
                      selectTab('audit')
                    }}
                  >
                    Open Audit
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        </header>

        <div className="min-h-0 min-w-0 flex-1 overflow-x-auto">
        <main
          className={cn(
            'mx-auto w-full px-5 py-5',
            tab === 'topology' ? 'w-max min-w-full' : 'max-w-[1480px]',
          )}
        >
          {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

          {stack.length > 1 ? (
            <div className="mb-4">
              <Breadcrumb items={crumbs} />
            </div>
          ) : null}

          {selectedDevice ? (
            <DeviceDetail
              device={selectedDevice}
              nowMs={showNowMs}
              lan={
                selectedDevice.intf_uuid
                  ? uuidToNet.get(selectedDevice.intf_uuid)
                  : undefined
              }
              port={portIndex.get(selectedDevice.mac.toUpperCase())}
              groupLabel={groupNameFor(selectedDevice, labelTag)}
              tags={showTags}
              labelTag={labelTag}
              unifi={unifiMod}
              domainSuffix={showBox?.local_domain_suffix}
              onRenamed={(mac, name) => {
                const key = mac.toUpperCase()
                setDevices((prev) =>
                  prev.map((d) =>
                    d.mac.toUpperCase() === key ? { ...d, name } : d,
                  ),
                )
                setStack((prev) => {
                  const idx = prev.findIndex(
                    (f) => f.kind === 'device' && f.mac.toUpperCase() === key,
                  )
                  if (idx < 0) return prev
                  const frame = prev[idx]
                  if (frame.kind !== 'device') return prev
                  const next = [...prev]
                  next[idx] = { ...frame, label: name }
                  return next
                })
              }}
              onDNSUpdated={(mac, hostname) => {
                const key = mac.toUpperCase()
                setDevices((prev) =>
                  prev.map((d) =>
                    d.mac.toUpperCase() === key ? { ...d, local_domain: hostname || undefined } : d,
                  ),
                )
              }}
              onGroupUpdated={(mac, tagIds) => {
                const key = mac.toUpperCase()
                setDevices((prev) =>
                  prev.map((d) =>
                    d.mac.toUpperCase() === key ? { ...d, tag_ids: tagIds } : d,
                  ),
                )
              }}
            />
          ) : selectedRegion && tab === 'metrics' ? (
            <RegionDetail
              region={selectedRegion}
              onSelectDevice={(mac, label) => openDevice(mac, label)}
            />
          ) : tab === 'wireless' && selectedAp ? (
            <WirelessAPDetail ap={selectedAp} />
          ) : tab === 'wireless' && selectedNet ? (
            <WirelessNetworkDetail
              network={selectedNet}
              clients={showWireless?.clients ?? []}
              onSelectClient={(c) => openDevice(c.mac, clientLabel(c))}
            />
          ) : showDevicesTable ? (
            <DevicesTab
              devices={showDevices}
              groupFilter={groupFilter}
              lanFilter={lanFilter}
              switchMacs={switchMacs}
              switches={showTopo?.switches ?? []}
              query={query}
              nowMs={showNowMs}
              uuidToNet={uuidToNet}
              tags={showTags}
              groupByHint={
                switchMacs.length > 0 &&
                isWholeSwitchFilter(switchMacs, showTopo?.switches ?? [])
                  ? 'port'
                  : undefined
              }
              onGroup={(id) => {
                if (!id) {
                  setStack(resetTab(tab === 'groups' ? 'groups' : 'devices'))
                  return
                }
                setStack([
                  { kind: 'tab', tab: tab === 'groups' ? 'groups' : 'devices' },
                  { kind: 'group', id, label: labelTag(id, 'group') },
                ])
              }}
              onLan={(uuid) => {
                if (!uuid) {
                  setStack(resetTab(tab === 'network' ? 'network' : 'devices'))
                  return
                }
                goDevicesLan(uuid)
              }}
              onSwitchMacs={(macs) => {
                if (!macs.length) {
                  setStack(resetTab(tab === 'topology' ? 'topology' : 'devices'))
                  return
                }
                const sw = findLast(stack, 'topo-switch')
                if (sw) {
                  setStack([
                    { kind: 'tab', tab: 'topology' },
                    sw,
                    { kind: 'ports', macs: macs.map((m) => m.toUpperCase()), label: 'Clients' },
                  ])
                  return
                }
                setStack([
                  { kind: 'tab', tab: tab === 'topology' ? 'topology' : 'devices' },
                  { kind: 'ports', macs: macs.map((m) => m.toUpperCase()), label: 'Clients' },
                ])
              }}
              onQuery={setQuery}
              labelTag={labelTag}
              onSelectDevice={(d) => openDevice(d.mac, preferredName(d))}
            />
          ) : (
            <>
          {tab === 'metrics' && (
            <MetricsTab
              latest={showView}
              persist={persist}
              dashboard={showDash}
              agentOnline={agentOnline}
              deviceCount={showDevices.length}
              ruleCount={policies.length}
              onOpenDevices={() => selectTab('devices')}
              onOpenRules={() => selectTab('rules')}
              onSelectDevice={(mac, label) => openDevice(mac, label)}
              onSelectRegion={(cc, label) => openRegion(cc, label)}
              onDashboard={setDashboard}
            />
          )}

          {tab === 'inventory' && (
            <InventoryTab
              box={showBox}
              unifi={unifiMod}
              console={showUnifi}
              devices={showDevices}
              network={showNetwork}
              switches={showTopo?.switches ?? []}
              policies={showPolicies}
            />
          )}

          {tab === 'network' && (
            <NetworkTab network={showNetwork} wanType={wanType} onSelectLan={goDevicesLan} />
          )}

          {tab === 'legacy' && debugOn && (
            <LegacyTab
              latest={showView}
              history={showHistory}
              network={showNetwork}
              persist={persist}
              chartRange={chartRange}
              onChartRange={setChartRange}
              page={legacyPage}
              onPage={setLegacyPage}
              deviceMode={modes.devices}
              devices={showDevices}
              filteredDevices={filteredDevices}
              groupTags={groupTags}
              uuidToNet={uuidToNet}
              seenId={seenId}
              groupFilter={groupFilter}
              lanFilter={lanFilter}
              query={query}
              nowMs={showNowMs}
              onSeen={setSeenId}
              onGroup={(id) =>
                setStack(
                  id
                    ? [
                        { kind: 'tab', tab: 'legacy' },
                        { kind: 'group', id, label: labelTag(id, 'group') },
                      ]
                    : resetTab('legacy'),
                )
              }
              onLan={(uuid) =>
                setStack(
                  uuid
                    ? [
                        { kind: 'tab', tab: 'legacy' },
                        {
                          kind: 'lan',
                          uuid,
                          label: uuidToNet.get(uuid) ? netLabel(uuidToNet.get(uuid)!) : uuid,
                        },
                      ]
                    : resetTab('legacy'),
                )
              }
              onQuery={setQuery}
              labelTag={labelTag}
              onSelectLan={goDevicesLan}
              host={showInvMeta.host}
              ts={showInvMeta.ts}
            />
          )}

          {tab === 'topology' && (
            <TopologyTab
              view={showTopo}
              network={showNetwork}
              ifaces={showView?.snapshot.ifaces}
              devices={showDevices}
              focusMac={findLast(stack, 'topo-switch')?.mac}
              onOpenSwitchClients={goDevicesSwitch}
              onSelectLan={goDevicesLan}
              onSelectDevice={(mac, label) => openDevice(mac, label)}
            />
          )}

          {tab === 'wireless' && (
            <WirelessTab
              wireless={showWireless}
              unifi={unifiMod}
              mode={wirelessMode}
              onMode={(m) => {
                setWirelessMode(m)
                setStack(resetTab('wireless'))
              }}
              onSelectAp={(ap) =>
                setStack([
                  { kind: 'tab', tab: 'wireless' },
                  { kind: 'ap', mac: ap.mac, label: apLabel(ap) },
                ])
              }
              onSelectNet={(n) =>
                setStack([
                  { kind: 'tab', tab: 'wireless' },
                  { kind: 'ssid', key: netKey(n), label: n.name },
                ])
              }
              onSelectClient={(c) => {
                setWirelessMode('clients')
                setStack([
                  { kind: 'tab', tab: 'wireless' },
                  { kind: 'device', mac: c.mac.toUpperCase(), label: clientLabel(c) },
                ])
              }}
            />
          )}

          {tab === 'audit' && (
            <AuditTab
              data={showAudit}
              focus={auditFocus}
              onSnoozeChange={() => setSnoozeRev((n) => n + 1)}
              onName={() => { setSettingsOpen('unifi-sync'); selectTab('settings') }}
              onVlan={(row) => openDevice(row.mac, row.name || row.mac)}
              onStp={(row) => setStack([
                { kind: 'tab', tab: 'topology' },
                { kind: 'topo-switch', mac: row.mac.toUpperCase(), label: row.name || row.mac },
              ])}
              onUnknown={(row) => openDevice(row.mac, row.name || row.mac)}
              onOffline={(row) => setStack([
                { kind: 'tab', tab: 'topology' },
                { kind: 'topo-switch', mac: row.mac.toUpperCase(), label: row.name || row.mac },
              ])}
              onPending={() => { setSettingsOpen('unifi-sync'); selectTab('settings') }}
            />
          )}

          {tab === 'history' && <HistoryTab />}

          {tab === 'logs' && <LogsTab />}

          {tab === 'devices' && (
            <DevicesTab
              devices={showDevices}
              groupFilter={groupFilter}
              lanFilter={lanFilter}
              switchMacs={switchMacs}
              switches={showTopo?.switches ?? []}
              query={query}
              nowMs={showNowMs}
              uuidToNet={uuidToNet}
              tags={showTags}
              groupByHint={
                switchMacs.length > 0 &&
                isWholeSwitchFilter(switchMacs, showTopo?.switches ?? [])
                  ? 'port'
                  : undefined
              }
              onGroup={(id) =>
                setStack(
                  id
                    ? [
                        { kind: 'tab', tab: 'devices' },
                        { kind: 'group', id, label: labelTag(id, 'group') },
                      ]
                    : resetTab('devices'),
                )
              }
              onLan={(uuid) =>
                setStack(
                  uuid
                    ? [
                        { kind: 'tab', tab: 'devices' },
                        {
                          kind: 'lan',
                          uuid,
                          label: uuidToNet.get(uuid) ? netLabel(uuidToNet.get(uuid)!) : uuid,
                        },
                      ]
                    : resetTab('devices'),
                )
              }
              onSwitchMacs={(macs) =>
                setStack(
                  macs.length
                    ? [
                        { kind: 'tab', tab: 'devices' },
                        { kind: 'ports', macs, label: 'Clients' },
                      ]
                    : resetTab('devices'),
                )
              }
              onQuery={setQuery}
              labelTag={labelTag}
              onSelectDevice={(d) => openDevice(d.mac, preferredName(d))}
            />
          )}

          {tab === 'rules' && (
            <RulesTab
              mode={modes.rules}
              devices={showDevices}
              tags={showTags}
              labelTag={labelTag}
              onOpenControl={() => {
                setSettingsOpen('fw-app')
                selectTab('settings')
              }}
            />
          )}

          {tab === 'settings' && (
            <SettingsTab
              openModule={settingsOpen}
              onOpenModule={setSettingsOpen}
              onModulesChange={(modules) => {
                setUnifiMod(modules.find((m) => m.id === 'unifi-sync') ?? null)
                setTPLinkMod(modules.find((m) => m.id === 'tplink-sync') ?? null)
              }}
            />
          )}

          {tab === 'debug' && debugOn ? (
            <DebugTab
              switches={showTopo?.switches ?? topoViewWithSims?.switches ?? []}
              onSimsChange={() => setSimTick((n) => n + 1)}
              onOpenTopology={() => selectTab('topology')}
            />
          ) : null}

          {tab === 'groups' && (
            <GroupsTab
              mode={modes.groups}
              groups={groups}
              onSelectGroup={goDevicesGroup}
            />
          )}
            </>
          )}
        </main>
        </div>
      </div>
    </div>
  )
}

export default App
