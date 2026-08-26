export type Load = { m1: number; m5: number; m15: number; cores?: number }
export type IfaceStats = {
  rx_bytes: number
  tx_bytes: number
  speed_mbps: number | null
  carrier: boolean
}
export type WANLink = { name: string; ready: boolean; active: boolean }
export type CPU = { user: number; sys: number; idle: number; softirq: number }
export type DNSSvc = { name: string; ok: boolean; since?: number }
export type Unbound = { queries: number; hits: number; misses: number; hit_pct: number }
export type Rates = { rx_mbps: number | null; tx_mbps: number | null }

export type Snapshot = {
  ts: number
  host: string
  load: Load
  cpu?: CPU
  dns_restarts: number
  dns_svcs?: DNSSvc[]
  dns_blocks_delta?: number | null
  unbound?: Unbound
  ifaces: Record<string, IfaceStats>
  wan: Record<string, WANLink>
  disks?: DiskMount[]
  nic_metrics?: NICMetric[]
}

export type DiskMount = {
  mount: string
  filesystem?: string
  capacity: number
  size?: number
  used?: number
  available?: number
}

export type NICMetric = {
  name: string
  rx_median?: number
  tx_median?: number
  rx_pt90?: number
  tx_pt90?: number
}

export type UnboundHit = {
  pct: number
  /** true = lifetime fallback (no usable 24h baseline yet) */
  life?: boolean
}

/** Observatory data provenance from dual-source facades (optional on old servers). */
export type Provenance = {
  source?: string
  fetched_at?: string
  stale?: boolean
  enriched_from?: string
  /** prefer = forced control refresh; fallback = agent down/stale. */
  reason?: string
}

export type DataSource = 'agent' | 'fw-app-init' | 'empty' | (string & {})

export type LatestView = {
  snapshot: Snapshot
  rates: Record<string, Rates>
  have_prev: boolean
  unbound_hit?: UnboundHit
  source?: DataSource
  fetched_at?: string
  stale?: boolean
  enriched_from?: string
  reason?: string
}

export type HistoryPoint = {
  ts: number
  load: Load
  dns_restarts: number
  rates: Record<string, Rates>
}

export type Device = {
  mac: string
  name: string
  ip?: string
  ipv6?: string[]
  vendor?: string
  type?: string
  local_domain?: string
  last_active_ts?: number
  active?: boolean
  intf_uuid?: string
  tag_ids?: string[]
  device_tag_ids?: string[]
  user_tag_ids?: string[]
  upload?: number
  download?: number
  top_dests?: RankedFlow[]
  dap?: DAP
  hostname?: string
  os?: string
  tx_kbps?: number
  rx_kbps?: number
  ssid?: string
  band?: string
  ap_mac?: string
  ap_name?: string
}

export type DAP = {
  status: string
  reason?: string
  learned: number
  trusted: number
  blocked: number
}

export type NetIface = {
  name: string
  desc?: string
  type?: string
  uuid?: string
  ip?: string
  subnet?: string
  device_count?: number
  vid?: number
  intfs?: string[]
  dhcp?: boolean
  wan_ready?: boolean
  wan_active?: boolean
}

export type SwitchPort = {
  id: string
  up: boolean
  speed_mbps?: number
  poe_status?: string
  poe_w?: number
  poe_mode?: string
  sfp?: boolean
  uplink?: boolean
  clients?: string[]
  rx_bytes?: number
  tx_bytes?: number
  rx_unicast?: number
  tx_unicast?: number
  rx_broadcast?: number
  tx_broadcast?: number
  rx_multicast?: number
  tx_multicast?: number
  rx_discard?: number
  tx_discard?: number
  rx_error?: number
  tx_error?: number
}

export type SwitchPoE = {
  draw_w: number
  budget_w: number
  active_ports: number
  peak_port?: string
}

export type SwitchACL = {
  used: number
  max: number
  control: number
  tracking: number
}

export type Switch = {
  mac: string
  name?: string
  ip?: string
  model?: string
  version?: string
  firmware?: string
  source?: string
  kind?: string
  healthy: boolean
  uplink_port?: string
  uplink_static?: boolean
  parent_nic?: string
  parent_mac?: string
  parent_port?: string
  ports?: SwitchPort[]
  lags?: string[][]
  radios?: { band: string; channel?: number; width_mhz?: number }[]
  poe?: SwitchPoE
  acl?: SwitchACL
  clients?: string[]
  capabilities?: { clients?: boolean; port_clients?: boolean }
}

export type TopoNode = {
  mac: string
  name?: string
  ip?: string
  type?: string
  parent_port?: string
  child_port?: string
  clients?: string[]
  children?: TopoNode[]
}

export type TopoView = {
  ts?: number
  host?: string
  box?: BoxInfo | null
  switches?: Switch[]
  tree?: TopoNode[]
  wan_type?: string
}

export type Policy = {
  id: string
  action?: string
  type?: string
  target?: string
  hit_count: number
  disabled: boolean
  notes?: string
}

export type FwAppRuleSection = 'allow' | 'block' | 'disturb' | 'timelimit' | 'other'

export type FwAppRule = {
  id: string
  section: FwAppRuleSection
  action: string
  type: string
  target: string
  name?: string
  notes?: string
  direction?: string
  trafficDirection?: string
  disabled: boolean
  scope?: string[]
  tags?: string[]
  scopeLabel?: string
  hitCount: number
  lastHitTs?: number
  activatedTime?: string
  timestamp?: string
  purpose?: string
  method?: string
  alarmType?: string
  readOnly?: boolean
}

export type FwAppExceptionRule = {
  id: string
  type?: string
  alarmType?: string
  target?: string
  targetName?: string
  matchCount: number
  timestamp?: number
  reason?: string
  category?: string
  ifType?: string
  ifTarget?: string
}

export type FwAppScopeChipKind = 'all' | 'device' | 'tag' | 'group'

export type FwAppScopeChip = {
  id: string
  kind: FwAppScopeChipKind
  label: string
  count: number
}

export type FwAppRulesHub = {
  totalRules: number
  totalHits: number
  allowHits: number
  blockHits: number
  allowCount: number
  blockCount: number
}

export type FwAppRulesCapabilities = Record<string, boolean>

export type FwAppCatalogItem = {
  id: string
  label?: string
}

export type FwAppRuleCatalog = {
  apps?: FwAppCatalogItem[]
}

export type FwAppCreateRuleRequest = {
  action: 'allow' | 'block'
  type?: string
  target: string
  scope?: string[]
  direction?: string
  notes?: string
  name?: string
}

export type FwAppHostPolicy = {
  mac: string
  label?: string
  monitor: boolean
  isolated: boolean
  emergency?: boolean
  adblock?: boolean
  family?: boolean
  note?: string
  tags?: string[]
}

export type FwAppRulesView = {
  hub: FwAppRulesHub
  rules: FwAppRule[]
  dapRules?: FwAppRule[]
  scopes: FwAppScopeChip[]
  exceptions: FwAppExceptionRule[]
  catalog?: FwAppRuleCatalog
  capabilities: FwAppRulesCapabilities
  refreshed_at?: string
}

export type FwAppStatus = {
  paired: boolean
  state: string
  box_ip?: string
  gid_hint?: string
  email?: string
  device_name?: string
  paired_at?: string
  last_ping_ok?: boolean
  last_ping_at?: string
  last_error?: string
  secrets_ready: boolean
}

export type Tag = {
  id: string
  name: string
  type?: string
  affiliated_tag?: string
}

export type ModuleInfo = {
  id: string
  label: string
  enabled: boolean
  status?: 'ok' | 'down' | 'ready'
  detail?: string
  site?: string
  name_sync_enabled?: boolean
  name_sync_auto?: boolean
  name_sync_pending?: number
  audit_names?: number
  audit_vlan?: number
  audit_stp?: number
  audit_unknown?: number
  audit_offline?: number
  audit_pending?: number
}

export type NameSyncRow = {
  mac: string
  ip?: string
  firewalla_name: string
  unifi_name?: string
  status: 'empty' | 'conflict'
}

export type NameSyncView = {
  name_sync_enabled: boolean
  name_sync_auto: boolean
  name_sync_excluded?: string[]
  rows: NameSyncRow[]
  excluded?: NameSyncRow[]
}

export type VLANAuditRow = {
  mac: string
  name?: string
  fw_vid: number
  unifi_vid: number
  sx_vid?: number
}
export type STPAuditRow = {
  mac: string
  name?: string
  kind: 'off' | 'wrong_root' | 'priority' | 'uplink_blocking'
  detail?: string
}
export type UnknownAuditRow = {
  mac: string
  name?: string
  side: 'unifi_only' | 'fw_only'
}
export type OfflineAuditRow = {
  mac: string
  name?: string
  kind: 'switch' | 'ap'
}
export type PendingAuditRow = {
  mac: string
  name?: string
  model?: string
}
export type AuditView = {
  names: { count: number; rows: NameSyncRow[] }
  vlan: { count: number; rows: VLANAuditRow[] }
  stp: { count: number; rows: STPAuditRow[] }
  unknown: { count: number; rows: UnknownAuditRow[] }
  offline: { count: number; rows: OfflineAuditRow[] }
  pending: { count: number; rows: PendingAuditRow[] }
}

export type UnifiConsole = {
  name?: string
  hardware?: string
  ip?: string
  inform?: string
  uptime_sec?: number
  network_version?: string
  network_update?: boolean
  site?: string
  cloud?: boolean
  aps?: number
  switches?: number
  devices?: number
  clients?: number
}

export type WirelessMeta = {
  source?: string
  site?: string
  networks_supported: boolean
  error?: string
}

export type WirelessRadio = {
  band: string
  channel?: number
  width_mhz?: number
  clients?: number
}

export type WirelessAP = {
  mac: string
  name?: string
  ip?: string
  model?: string
  firmware?: string
  healthy: boolean
  uplink?: string
  source?: string
  radios?: WirelessRadio[]
  clients?: number
}

export type WirelessNetwork = {
  id?: string
  name: string
  ssid?: string
  enabled?: boolean
  bands?: string[]
  clients?: number
}

export type WirelessClient = {
  mac: string
  name?: string
  ip?: string
  ssid?: string
  band?: string
  ap_mac?: string
  ap_name?: string
  hostname?: string
  os?: string
  tx_kbps?: number
  rx_kbps?: number
}

export type WirelessView = {
  meta: WirelessMeta
  aps: WirelessAP[]
  networks: WirelessNetwork[]
  clients: WirelessClient[]
}

export type ControlEvent = {
  id: number
  ts: number
  scheme: string
  action: string
  actor_kind: string
  actor: string
  target: string
  summary?: string
  result: string
  error?: string
  before?: unknown
  after?: unknown
}

export type ControlHistoryView = {
  events: ControlEvent[]
  actions: Record<string, string[]>
}

export type Tab =
  | 'metrics'
  | 'inventory'
  | 'network'
  | 'topology'
  | 'wireless'
  | 'audit'
  | 'history'
  | 'devices'
  | 'rules'
  | 'groups'
  | 'logs'
  | 'settings'
  | 'debug'
  | 'legacy'

export type BoxMAC = { name: string; mac: string }

export type BoxInfo = {
  name: string
  public_ip?: string
  public_ips?: Record<string, string>
  ddns?: string
  macs?: Array<BoxMAC | string>
  version?: string
  model?: string
  license?: string
  eid?: string
  mode?: string
  timezone?: string
  country?: string
  region?: string
  wan_type?: string
  local_domain_suffix?: string
  uptime_sec?: number
  os_uptime_sec?: number
  cloud_connected?: boolean
}
export type ViewMode = 'visual' | 'list'

export type BytePoint = { ts: number; upload: number; download: number; conn?: number }

export type Transfer24h = {
  upload: number
  download: number
  points?: BytePoint[]
}

export type WANUsage = {
  uuid: string
  name: string
  upload: number
  download: number
}

export type BlockedMix = { blocked: number; allowed: number }

export type SpeedtestPoint = {
  ts: number
  down: number
  up: number
  ping?: number
  server_id?: string
  server?: string
  location?: string
}

export type SpeedtestWAN = {
  uuid: string
  name: string
  active?: boolean
  plan_down?: number
  plan_up?: number
  down?: number
  up?: number
  ping?: number
  jitter?: number
  server_id?: string
  server?: string
  location?: string
  points?: SpeedtestPoint[]
}

export type DNSResolver = { server: string; wan?: string; ok: boolean }

export type DNSQuery = { ts: number; count: number }

export type DNSHealth = {
  resolvers?: DNSResolver[]
  queries?: DNSQuery[]
}

export type RankedFlow = {
  id?: string
  name: string
  type?: string
  dest_ip?: string
  country?: string
  region?: string
  upload?: number
  download?: number
  bytes: number
  devices?: RankedFlow[]
  targets?: RankedFlow[]
}

export type Dashboard = {
  ts?: number
  host?: string
  devices: number
  rules: number
  alarm_count: number
  transfer_24h: Transfer24h
  transfer_30d?: Transfer24h
  transfer_60?: Transfer24h
  transfer_12m?: Transfer24h
  monthly_wans?: WANUsage[]
  monthly_begin_ts?: number
  monthly_end_ts?: number
  blocked: BlockedMix
  top_upload?: RankedFlow[]
  top_download?: RankedFlow[]
  top_dest_upload?: RankedFlow[]
  top_dest_download?: RankedFlow[]
  top_regions?: RankedFlow[]
  speedtest?: SpeedtestWAN[]
  dns?: DNSHealth
  source?: DataSource
  fetched_at?: string
  stale?: boolean
  enriched_from?: string
  reason?: string
}

export type PersistInfo = {
  enabled?: boolean
  retention_days?: number
  points?: number
}

export type AgentHealth = {
  online?: boolean
  host?: string
  version?: string
  last_seen?: number
}

export const MIN = 60_000
export const HOUR = 60 * MIN
export const DAY = 24 * HOUR

export const SEEN_OPTIONS = [
  { id: '1h', label: '1h', ms: HOUR },
  { id: '6h', label: '6h', ms: 6 * HOUR },
  { id: '1d', label: '1d', ms: DAY },
  { id: '1w', label: '1w', ms: 7 * DAY },
  { id: '1mo', label: '1mo', ms: 30 * DAY },
  { id: '3mo', label: '3mo', ms: 90 * DAY },
  { id: '6mo', label: '6mo', ms: 180 * DAY },
] as const

export type SeenId = (typeof SEEN_OPTIONS)[number]['id']

export const CHART_RANGES = [
  { id: '6h', label: '6h' },
  { id: '1d', label: '1d' },
  { id: '7d', label: '7d' },
  { id: '30d', label: '30d' },
  { id: '90d', label: '90d' },
] as const

export type ChartRange = (typeof CHART_RANGES)[number]['id']
