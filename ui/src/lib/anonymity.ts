import type {
  AuditView,
  BoxInfo,
  ControlEvent,
  Dashboard,
  Device,
  FwAppRulesView,
  HistoryPoint,
  LatestView,
  ModuleInfo,
  NameSyncRow,
  NetIface,
  Policy,
  RankedFlow,
  Snapshot,
  Switch,
  Tag,
  TopoNode,
  TopoView,
  UnifiConsole,
  WirelessView,
} from './types'

export const DEST_POOL = [
  { name: 'Cloudflare', domain: 'cloudflare.com' },
  { name: 'Netflix', domain: 'netflix.com' },
  { name: 'Amazon', domain: 'amazon.com' },
  { name: 'Steam', domain: 'steampowered.com' },
  { name: 'Akamai', domain: 'akamai.com' },
  { name: 'Hugging Face', domain: 'huggingface.co' },
  { name: 'Google', domain: 'google.com' },
  { name: 'Apple', domain: 'apple.com' },
  { name: 'Microsoft', domain: 'microsoft.com' },
  { name: 'Discord', domain: 'discord.com' },
  { name: 'YouTube', domain: 'youtube.com' },
  { name: 'GitHub', domain: 'github.com' },
] as const

export const ISP_POOL = [
  'Google Fiber',
  'AT&T Fiber',
  'Verizon Fios',
  'Comcast Xfinity',
  'Spectrum',
] as const

export const GEO_ROWS = [
  { cc: 'US', city: 'New York', tz: 'America/New_York' },
  { cc: 'GB', city: 'London', tz: 'Europe/London' },
  { cc: 'DE', city: 'Berlin', tz: 'Europe/Berlin' },
  { cc: 'JP', city: 'Tokyo', tz: 'Asia/Tokyo' },
  { cc: 'FR', city: 'Paris', tz: 'Europe/Paris' },
  { cc: 'CA', city: 'Toronto', tz: 'America/Toronto' },
  { cc: 'AU', city: 'Sydney', tz: 'Australia/Sydney' },
  { cc: 'KR', city: 'Seoul', tz: 'Asia/Seoul' },
] as const

const USER_NAMES = [
  'Alex',
  'Sam',
  'Jordan',
  'Riley',
  'Casey',
  'Morgan',
  'Quinn',
  'Avery',
  'Taylor',
  'Jamie',
  'Cameron',
  'Drew',
  'Skyler',
  'Reese',
  'Parker',
  'Rowan',
]

export const PHONE_NAMES = [
  'Guest iPhone',
  'Alex iPhone',
  'Pixel 8',
  'Galaxy S24',
  'Work Phone',
  'Riley iPhone',
  'Casey Pixel',
  'Jordan Galaxy',
  'Kids iPhone',
  'Office Pixel',
  'Backup Phone',
  'Sam Android',
  'Travel iPhone',
  'Morgan Pixel',
] as const

const TABLET_NAMES = [
  'Kitchen iPad',
  'Office iPad',
  'Galaxy Tab',
  'Kids Tablet',
  'Bedroom iPad',
  'Guest iPad',
  'Studio Tablet',
  'Den iPad',
]

const COMPUTER_NAMES = [
  'Office MacBook',
  'Desk PC',
  'ThinkPad',
  'iMac',
  'Mac mini',
  'Mini PC',
  'NUC',
  'Gaming PC',
  'Work Laptop',
  'Basement PC',
  'Studio Mac',
  'Living Room Mini',
  'Framework Laptop',
  'Home Server PC',
  'Media PC',
]

const CONSOLE_NAMES = [
  'Living Room PS5',
  'Xbox Series X',
  'Switch',
  'Steam Deck',
  'Xbox',
  'PS5 Slim',
  'Retro Pi',
  'Switch Lite',
  'Xbox S',
  'Steam PC',
]

const TV_NAMES = [
  'Living Room TV',
  'Bedroom TV',
  'Office TV',
  'Patio TV',
  'Basement TV',
  'Apple TV',
  'Roku',
  'Fire TV',
  'Shield TV',
  'Kitchen TV',
  'Guest TV',
]

const SMARTHOME_NAMES = [
  'Hue Bridge',
  'Nest Hub',
  'Echo Dot',
  'Thermostat',
  'Vacuum',
  'Smart Plug',
  'Light Strip',
  'Lock',
  'Garage Opener',
  'Sprinkler',
  'Doorbell Chime',
  'Leak Sensor',
  'Smoke Hub',
  'Blinds',
  'Irrigation',
]

const CAMERA_NAMES = [
  'Hallway Cam',
  'Doorbell',
  'Driveway Cam',
  'Patio Cam',
  'Nursery Cam',
  'Garage Cam',
  'Porch Cam',
  'Backyard Cam',
  'Entry Cam',
  'Side Cam',
]

const NAS_NAMES = ['Nas Box', 'Synology', 'TrueNAS', 'Backup NAS', 'Studio NAS', 'Media NAS', 'Lab NAS']
const PRINTER_NAMES = ['Office Printer', 'Label Printer', 'Photo Printer', 'Basement Printer']
const SPEAKER_NAMES = ['Patio Speaker', 'Homepod', 'Sonos', 'Echo Studio', 'Kitchen Speaker', 'Bedroom Speaker']
const WEARABLE_NAMES = ['Apple Watch', 'Galaxy Watch', 'Fitness Band', 'Kids Watch']
const ROUTER_NAMES = ['Edge Router', 'Core Router', 'VPN Router', 'Travel Router', 'Lab Router']

export const DEVICE_NAMES = [
  ...PHONE_NAMES,
  ...TABLET_NAMES,
  ...COMPUTER_NAMES,
  ...CONSOLE_NAMES,
  ...TV_NAMES,
  ...SMARTHOME_NAMES,
  ...CAMERA_NAMES,
  ...NAS_NAMES,
  ...PRINTER_NAMES,
  ...SPEAKER_NAMES,
  ...WEARABLE_NAMES,
  'Basement Pi',
  'Kitchen Tablet',
]

export const AP_NAMES = [
  'Upstairs AP',
  'Downstairs AP',
  'Garage AP',
  'Office AP',
  'Living Room AP',
  'Kitchen AP',
  'Patio AP',
  'Bedroom AP',
  'Hallway AP',
  'Basement AP',
  'Attic AP',
  'Guest Room AP',
  'Studio AP',
  'Dining Room AP',
  'Family Room AP',
  'Media Room AP',
  'Master Bedroom AP',
  'Kids Room AP',
  'Nursery AP',
  'Laundry AP',
  'Foyer AP',
  'Landing AP',
  'Workshop AP',
  'Porch AP',
  'Backyard AP',
  'Driveway AP',
  'Front Door AP',
  'Upstairs Hall AP',
  'Downstairs Hall AP',
  'Office 2 AP',
  'Bedroom 2 AP',
  'Lobby AP',
  'Conference AP',
] as const

export const SWITCH_NAMES = [
  'Core Switch',
  'Edge Switch',
  'Access Switch',
  'Distribution Switch',
  'Leaf Switch',
  'Spine Switch',
  'ToR Switch',
  'Closet Switch',
  'Office Switch',
  'Basement Switch',
  'Garage Switch',
  'Rack Switch',
  'Lab Switch',
  'Camera Switch',
  'AP Switch',
  'Utility Switch',
  'Workshop Switch',
  'Backbone Switch',
] as const

const SWITCH_POE = ['PoE Switch', 'Core PoE', 'Access PoE'] as const
const SWITCH_SFP = ['Fiber Switch', 'Aggregation Switch'] as const

export function switchNamePool(s: Pick<Switch, 'poe' | 'ports' | 'model' | 'name'>): readonly string[] {
  const extra: string[] = []
  const blob = `${s.model || ''} ${s.name || ''}`
  if (s.poe && (s.poe.budget_w > 0 || s.poe.active_ports > 0 || s.poe.draw_w > 0)) extra.push(...SWITCH_POE)
  else if (s.ports?.some((p) => !!(p.poe_status || p.poe_w || p.poe_mode))) extra.push(...SWITCH_POE)
  else if (/poe/i.test(blob)) extra.push(...SWITCH_POE)
  if (s.ports?.some((p) => p.sfp) || /sfp|aggregat|fiber/i.test(blob)) extra.push(...SWITCH_SFP)
  const n = s.ports?.length || Number(/\b(5|8|16|24|48)\b/.exec(blob)?.[1] || 0)
  if (n > 0 && n <= 10) extra.push('Mini Switch', 'Desktop Switch', '8-Port Switch')
  else if (n > 10 && n <= 18) extra.push('16-Port Switch')
  else if (n > 18 && n <= 28) extra.push('24-Port Switch')
  else if (n > 28) extra.push('48-Port Switch')
  return extra.length ? [...SWITCH_NAMES, ...extra] : SWITCH_NAMES
}

export const LAN_NAMES = [
  'LAN',
  'IoT',
  'Guest',
  'Trusted',
  'Cameras',
  'Servers',
  'Kids',
  'Lab',
  'Mgmt',
  'VoIP',
  'VPN',
  'Media',
  'Office',
  'Printers',
  'Gaming',
  'DMZ',
  'Storage',
  'Workshop',
  'Wireless',
  'Wired',
  'Trunk',
  'Peer',
] as const

const SSID_NAMES = [
  'Home',
  'Home 5G',
  'IoT',
  'Guest',
  'Office',
  'Cameras',
  'Lab',
  'Outdoor',
  'Kids',
  'Media',
  'Mesh',
  'Corp',
  'IoT 2G',
  'Guest 5G',
]

export const GROUP_NAMES = [
  'Family',
  'Kids',
  'Guests',
  'IoT',
  'Work',
  'Staff',
  'Media',
  'Cameras',
  'Servers',
  'Lab',
  'VPN',
  'Phones',
  'Tablets',
  'Streaming',
  'Office',
  'Bedroom',
  'Basement',
  'Garage',
  'Trusted',
  'Visitors',
  'Contractors',
  'Smart Home',
  'Lighting',
  'HVAC',
  'Printers',
  'Gaming',
  'Speakers',
  'Watches',
  'Laptops',
  'Desktops',
  'Storage',
  'Workshop',
  'Patio',
  'Nursery',
  'Guest Room',
  'Travel',
  'Admin',
  'Personal',
] as const

const BOX_NAMES = ['Firewalla', 'Gateway', 'Router', 'Edge Box', 'Security Gateway', 'Home Gateway']
const VENDORS = [
  'Apple',
  'Samsung',
  'Google',
  'Amazon',
  'Intel',
  'Ubiquiti',
  'TP-Link',
  'Sony',
  'Microsoft',
  'Dell',
  'HP',
  'Lenovo',
  'Nintendo',
  'Logitech',
  'Philips',
  'Ecobee',
  'Roku',
  'NVIDIA',
  'ASUS',
  'Synology',
]

export type NameKind =
  | 'device'
  | 'ap'
  | 'switch'
  | 'ssid'
  | 'group'
  | 'user'
  | 'box'
  | 'vendor'
  | 'host'
  | 'lan'
  | 'phone'
  | 'tablet'
  | 'computer'
  | 'console'
  | 'tv'
  | 'smarthome'
  | 'nas'
  | 'printer'
  | 'speaker'
  | 'camera'
  | 'wearable'
  | 'router'

type NameMemo = { assigned: Map<string, string>; taken: Map<string, Set<string>> }
const nameMemos = new Map<string, NameMemo>()

function memoFor(salt: string): NameMemo {
  let m = nameMemos.get(salt)
  if (!m) {
    m = { assigned: new Map(), taken: new Map() }
    nameMemos.set(salt, m)
  }
  return m
}

const NAME_POOLS: Record<NameKind, readonly string[]> = {
  device: DEVICE_NAMES,
  ap: AP_NAMES,
  switch: SWITCH_NAMES,
  ssid: SSID_NAMES,
  group: GROUP_NAMES,
  user: USER_NAMES,
  box: BOX_NAMES,
  host: BOX_NAMES,
  vendor: VENDORS,
  lan: LAN_NAMES,
  phone: PHONE_NAMES,
  tablet: TABLET_NAMES,
  computer: COMPUTER_NAMES,
  console: CONSOLE_NAMES,
  tv: TV_NAMES,
  smarthome: SMARTHOME_NAMES,
  nas: NAS_NAMES,
  printer: PRINTER_NAMES,
  speaker: SPEAKER_NAMES,
  camera: CAMERA_NAMES,
  wearable: WEARABLE_NAMES,
  router: ROUTER_NAMES,
}

export function clientNameKind(type?: string, os?: string): NameKind {
  const t = `${type || ''} ${os || ''}`.toLowerCase().replace(/[_-]+/g, ' ')
  if (/\b(iphone|android|phone|smartphone|mobile)\b/.test(t)) return 'phone'
  if (/\b(ipad|tablet)\b/.test(t)) return 'tablet'
  if (/\b(macbook|laptop|desktop|windows|macos|linux|computer|imac|thinkpad|pc|chrome os)\b/.test(t))
    return 'computer'
  if (/\b(playstation|xbox|nintendo|steam|console|gaming|game console)\b/.test(t)) return 'console'
  if (/\b(tv|television|roku|firetv|apple tv|chromecast|media streaming|tvos)\b/.test(t)) return 'tv'
  if (/\b(nas|synology|qnap|server|unraid|nas and server)\b/.test(t)) return 'nas'
  if (/\bprinter\b/.test(t)) return 'printer'
  if (/\b(speaker|echo|homepod|sonos|smart speaker|audio)\b/.test(t)) return 'speaker'
  if (/\b(camera|doorbell)\b/.test(t)) return 'camera'
  if (/\b(watch|wearable)\b/.test(t)) return 'wearable'
  if (/\b(access point|ap|wifi|uap|wap)\b/.test(t)) return 'ap'
  if (/\b(switch|usw)\b/.test(t)) return 'switch'
  if (/\b(router|gateway|udm|usg)\b/.test(t)) return 'router'
  if (/\b(iot|plug|bulb|thermostat|vacuum|lock|hue|nest|smart|appliance|car)\b/.test(t)) return 'smarthome'
  return 'device'
}

export type CidrMap = { orig: string; fake: string }

function fnv(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

function parseV4(s: string): number | null {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(s.trim())
  if (!m) return null
  const oct = [Number(m[1]), Number(m[2]), Number(m[3]), Number(m[4])]
  if (oct.some((n) => n > 255)) return null
  return ((oct[0]! << 24) | (oct[1]! << 16) | (oct[2]! << 8) | oct[3]!) >>> 0
}

function fmtV4(n: number): string {
  return `${(n >>> 24) & 255}.${(n >>> 16) & 255}.${(n >>> 8) & 255}.${n & 255}`
}

function v4Mask(plen: number): number {
  const hostBits = 32 - plen
  if (hostBits >= 32) return 0
  if (hostBits <= 0) return 0xffffffff
  return (~0 << hostBits) >>> 0
}

function parseCIDR(s: string): { net: number; plen: number } | null {
  const parts = s.trim().split('/')
  const addr = parseV4(parts[0] || '')
  if (addr == null) return null
  const plen = parts[1] == null ? 32 : Number(parts[1])
  if (!Number.isInteger(plen) || plen < 0 || plen > 32) return null
  return { net: (addr & v4Mask(plen)) >>> 0, plen }
}

function isPrivateV4(n: number): boolean {
  const a = (n >>> 24) & 255
  const b = (n >>> 16) & 255
  if (a === 10) return true
  if (a === 192 && b === 168) return true
  if (a === 172 && b >= 16 && b <= 31) return true
  return false
}

export function ipInCIDR(ip: string, cidr: string): boolean {
  const addr = parseV4(ip)
  const c = parseCIDR(cidr)
  if (addr == null || !c) return false
  return ((addr & v4Mask(c.plen)) >>> 0) === c.net
}

function fakePrivateV4Net(h: number, plen: number): number {
  const p = Math.max(plen, 8)
  const hostBits = 32 - p
  const inner = Math.min(24, Math.max(0, p - 8))
  const net = (0x0a000000 | ((h & ((1 << inner) - 1 || 0)) << hostBits)) >>> 0
  return (net & v4Mask(p)) >>> 0
}

const TEST_NET24 = [0xcb007100, 0xc6336400, 0xc0000200] // 203.0.113 / 198.51.100 / 192.0.2

function fakePublicV4Net(h: number, plen: number): number {
  if (plen >= 24) {
    const base = TEST_NET24[h % TEST_NET24.length]!
    const hostBits = 32 - plen
    const extraBits = Math.max(0, plen - 24)
    const extra = extraBits === 0 ? 0 : (h >>> 8) & ((1 << extraBits) - 1)
    return ((base | extra << hostBits) & v4Mask(plen)) >>> 0
  }
  const p = Math.max(plen, 10)
  const hostBits = 32 - p
  const inner = Math.min(22, Math.max(0, p - 10))
  const innerMask = inner === 0 ? 0 : (1 << inner) - 1
  const net = (0x64400000 | ((h & innerMask) << hostBits)) >>> 0
  return (net & v4Mask(p)) >>> 0
}

function remapV4(addr: number, orig: { net: number; plen: number }, fakeNet: number): string {
  const host = (addr & ~v4Mask(orig.plen)) >>> 0
  return fmtV4((fakeNet | host) >>> 0)
}

function fakeV6(salt: string, ip: string): string {
  const lower = ip.trim().toLowerCase()
  const ula = lower.startsWith('fc') || lower.startsWith('fd') || lower.startsWith('fe80')
  const h = fnv(`${salt}|v6|${lower}`)
  const parts = [
    (h & 0xffff).toString(16),
    ((h >>> 16) & 0xffff).toString(16),
    fnv(`${salt}|v6b|${lower}`).toString(16).slice(0, 4),
    '0',
    '0',
    '0',
    (h & 0xff).toString(16),
  ]
  if (ula) return `fd00:${parts[0]}:${parts[1]}:${parts[2]}::${parts[6]}`
  return `2001:db8:${parts[0]}:${parts[1]}::${parts[6]}`
}

type RewriteFns = {
  fakeIP: (ip: string, cidrs?: CidrMap[]) => string
  fakeMAC: (mac: string) => string
  fakeDest: (original: string) => { name: string; domain: string }
}

function rewriteTextImpl(fns: RewriteFns, s: string, cidrs: CidrMap[] = []): string {
  type Span = { start: number; end: number; repl: string }
  const spans: Span[] = []
  const overlaps = (start: number, end: number) => spans.some((sp) => start < sp.end && end > sp.start)
  const add = (re: RegExp, fn: (m: string) => string) => {
    re.lastIndex = 0
    let m: RegExpExecArray | null
    while ((m = re.exec(s))) {
      const start = m.index
      const end = start + m[0].length
      if (overlaps(start, end)) continue
      spans.push({ start, end, repl: fn(m[0]) })
    }
  }
  add(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, (ip) => fns.fakeIP(ip, cidrs))
  add(/\b[0-9a-f]{2}(?::[0-9a-f]{2}){5}\b/gi, (mac) => fns.fakeMAC(mac))
  add(/\b(?:[0-9a-f]{1,4}:){2,7}[0-9a-f:]*\b/gi, (ip) => fns.fakeIP(ip, cidrs))
  add(/\b[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+\b/gi, (d) => {
    if (/^[\d.]+$/.test(d)) return d
    return fns.fakeDest(d).domain
  })
  spans.sort((a, b) => a.start - b.start)
  let out = ''
  let i = 0
  for (const sp of spans) {
    out += s.slice(i, sp.start) + sp.repl
    i = sp.end
  }
  return out + s.slice(i)
}

function parseMAC(s: string): number[] | null {
  const hex = s.trim().toLowerCase().replace(/-/g, ':')
  const m = /^([0-9a-f]{2})(:[0-9a-f]{2}){5}$/.exec(hex)
  if (!m) return null
  return hex.split(':').map((b) => parseInt(b, 16))
}

export type Anon = ReturnType<typeof createAnon>

export function createAnon(salt: string) {
  const idx = (kind: string, original: string, n: number) =>
    n <= 0 ? 0 : fnv(`${salt}|${kind}|${original}`) % n

  const offsetSec = -((3 + (fnv(`${salt}|offset`) % 14)) * 86400)

  const pick = <T>(kind: string, original: string, pool: readonly T[]): T =>
    pool[idx(kind, original || '∅', pool.length)]!

  const out = {
    salt,
    offsetSec,
    shiftTS(ts: number) {
      if (!ts) return ts
      return ts + offsetSec
    },
    fakeName(kind: NameKind, original: string, tag = '', pool?: readonly string[]) {
      const names = pool?.length ? pool : NAME_POOLS[kind]
      const key = `${kind}|${tag}|${original || '∅'}`
      const memo = memoFor(salt)
      const hit = memo.assigned.get(key)
      if (hit) return hit
      const used = memo.taken.get(kind) ?? new Set<string>()
      memo.taken.set(kind, used)
      const n = names.length
      const start = n <= 0 ? 0 : fnv(`${salt}|${kind}|${tag}|${original || '∅'}`) % n
      for (let step = 0; step < n; step++) {
        const name = names[(start + step) % n]!
        if (!used.has(name)) {
          used.add(name)
          memo.assigned.set(key, name)
          return name
        }
      }
      const base = names[start] || kind
      let extra = 2
      while (used.has(`${base} ${extra}`)) extra++
      const name = `${base} ${extra}`
      used.add(name)
      memo.assigned.set(key, name)
      return name
    },
    fakeDest(original: string) {
      return pick('dest', original, DEST_POOL)
    },
    fakeISP(original: string) {
      return pick('isp', original, ISP_POOL)
    },
    fakeGeo(cc?: string) {
      return pick('geo', (cc || '').toUpperCase(), GEO_ROWS)
    },
    fakeEID(original: string) {
      const h = fnv(`${salt}|eid|${original}`).toString(16).padStart(8, '0')
      return `eid-${h}${h}`
    },
    ipInCIDR,
    fakeCIDR(cidr: string) {
      const c = parseCIDR(cidr)
      if (!c) return cidr
      const h = fnv(`${salt}|cidr|${c.net}/${c.plen}`)
      const net = isPrivateV4(c.net) ? fakePrivateV4Net(h, c.plen) : fakePublicV4Net(h, c.plen)
      return `${fmtV4(net)}/${c.plen}`
    },
    fakeIP(ip: string, cidrs: CidrMap[] = []) {
      const raw = ip.trim()
      if (raw.includes(':')) return fakeV6(salt, raw)
      const addr = parseV4(raw)
      if (addr == null) return ip
      for (const m of cidrs) {
        if (!ipInCIDR(raw, m.orig)) continue
        const orig = parseCIDR(m.orig)
        const fake = parseCIDR(m.fake)
        if (!orig || !fake || orig.plen !== fake.plen) continue
        return remapV4(addr, orig, fake.net)
      }
      const plen = 32
      const h = fnv(`${salt}|ip|${raw}`)
      const net = isPrivateV4(addr) ? fakePrivateV4Net(h, plen) : fakePublicV4Net(h, plen)
      return fmtV4(net)
    },
    fakeMAC(mac: string) {
      const bytes = parseMAC(mac)
      if (!bytes) return mac
      const h = fnv(`${salt}|mac|${bytes.map((b) => b.toString(16).padStart(2, '0')).join(':')}`)
      const out = [
        ((h >>> 24) & 0xfe) | 0x02,
        (h >>> 16) & 0xff,
        (h >>> 8) & 0xff,
        h & 0xff,
        (fnv(`${salt}|mac2|${mac.toLowerCase()}`) >>> 16) & 0xff,
        fnv(`${salt}|mac2|${mac.toLowerCase()}`) & 0xff,
      ]
      return out.map((b) => b.toString(16).toUpperCase().padStart(2, '0')).join(':')
    },
    rewriteText(s: string, cidrs: CidrMap[] = []) {
      return rewriteTextImpl(
        {
          fakeIP: (ip, c) => out.fakeIP(ip, c),
          fakeMAC: (mac) => out.fakeMAC(mac),
          fakeDest: (d) => out.fakeDest(d),
        },
        s,
        cidrs,
      )
    },
  }
  return out
}

const COUNTRY_LABEL: Record<string, string> = {
  US: 'United States',
  GB: 'United Kingdom',
  DE: 'Germany',
  JP: 'Japan',
  FR: 'France',
  CA: 'Canada',
  AU: 'Australia',
  KR: 'South Korea',
}

function looksLikeMAC(s: string): boolean {
  return /^[0-9a-f]{2}(?::[0-9a-f]{2}){5}$/i.test(s.trim())
}

function wanish(n: NetIface): boolean {
  return /wan/i.test(n.type || '') || n.wan_ready != null
}

export type AnonLogLine = { ts: number; source: string; severity?: string; line: string }
export type AnonAgentEvent = {
  id?: number
  ts: number
  kind: string
  detail?: string
  from_ver?: string
  to_ver?: string
}

export function lanCidrsFromNetwork(anon: Anon, ifaces: NetIface[]): CidrMap[] {
  const out: CidrMap[] = []
  for (const n of ifaces) {
    const orig = n.subnet || (n.ip ? `${n.ip}/32` : '')
    if (!orig || orig.includes(':')) continue
    out.push({ orig, fake: anon.fakeCIDR(orig) })
  }
  return out
}

export function anonymizeNetwork(anon: Anon, ifaces: NetIface[]): NetIface[] {
  const cidrs = lanCidrsFromNetwork(anon, ifaces)
  return ifaces.map((n) => {
    const orig = n.subnet || (n.ip ? `${n.ip}/32` : '')
    const fake = orig ? cidrs.find((c) => c.orig === orig)?.fake : undefined
    return {
      ...n,
      desc: n.desc
        ? wanish(n)
          ? anon.fakeISP(n.desc)
          : anon.fakeName('lan', n.desc)
        : n.desc,
      ip: n.ip ? anon.fakeIP(n.ip, cidrs) : n.ip,
      subnet: fake ?? (n.subnet ? anon.fakeCIDR(n.subnet) : n.subnet),
    }
  })
}

export function anonymizeBox(anon: Anon, box: BoxInfo): BoxInfo {
  const g = anon.fakeGeo(box.country)
  return {
    ...box,
    name: anon.fakeName('box', box.name),
    public_ip: box.public_ip ? anon.fakeIP(box.public_ip) : box.public_ip,
    ddns: box.ddns ? anon.fakeDest(box.ddns).domain : box.ddns,
    eid: box.eid ? anon.fakeEID(box.eid) : box.eid,
    timezone: g.tz,
    country: g.cc,
    region: g.city,
    local_domain_suffix: box.local_domain_suffix
      ? anon.fakeDest(box.local_domain_suffix).domain
      : box.local_domain_suffix,
    macs: box.macs?.map((m) =>
      typeof m === 'string' ? anon.fakeMAC(m) : { name: m.name, mac: anon.fakeMAC(m.mac) },
    ),
  }
}

function fakeFlow(anon: Anon, r: RankedFlow, kind: 'device' | 'dest' | 'region', cidrs: CidrMap[]): RankedFlow {
  const g = anon.fakeGeo(r.country || (kind === 'region' ? r.name : undefined))
  let id = r.id
  if (id) {
    if (looksLikeMAC(id)) id = anon.fakeMAC(id)
    else if (kind === 'region') id = g.cc
    else id = anon.rewriteText(id, cidrs)
  }
  let name = r.name
  if (kind === 'device') name = anon.fakeName(clientNameKind(r.type), r.name)
  else if (kind === 'dest') name = anon.fakeDest(r.name).name
  else name = COUNTRY_LABEL[g.cc] || g.city
  return {
    ...r,
    id,
    name,
    dest_ip: r.dest_ip ? anon.fakeIP(r.dest_ip, cidrs) : r.dest_ip,
    country: r.country ? g.cc : r.country,
    region: r.region ? g.city : r.region,
    devices: r.devices?.map((d) => fakeFlow(anon, d, 'device', cidrs)),
    targets: r.targets?.map((t) => fakeFlow(anon, t, 'dest', cidrs)),
  }
}

export function anonymizeDevices(anon: Anon, devs: Device[], cidrs: CidrMap[] = []): Device[] {
  return devs.map((d) => ({
    ...d,
    mac: anon.fakeMAC(d.mac),
    name: anon.fakeName(clientNameKind(d.type, d.os), d.name),
    hostname: d.hostname ? anon.fakeName(clientNameKind(d.type, d.os), d.hostname) : d.hostname,
    local_domain: d.local_domain ? anon.fakeDest(d.local_domain).domain : d.local_domain,
    ip: d.ip ? anon.fakeIP(d.ip, cidrs) : d.ip,
    ipv6: d.ipv6?.map((ip) => anon.fakeIP(ip, cidrs)),
    vendor: d.vendor ? anon.fakeName('vendor', d.vendor) : d.vendor,
    ssid: d.ssid ? anon.fakeName('ssid', d.ssid) : d.ssid,
    ap_mac: d.ap_mac ? anon.fakeMAC(d.ap_mac) : d.ap_mac,
    ap_name: d.ap_name ? anon.fakeName('ap', d.ap_name) : d.ap_name,
    last_active_ts: d.last_active_ts ? anon.shiftTS(d.last_active_ts) : d.last_active_ts,
    top_dests: d.top_dests?.map((t) => fakeFlow(anon, t, 'dest', cidrs)),
  }))
}

function anonymizeSnapshot(anon: Anon, snap: Snapshot): Snapshot {
  const wan: Snapshot['wan'] = {}
  for (const [k, v] of Object.entries(snap.wan || {})) {
    wan[k] = { ...v, name: anon.fakeISP(v.name) }
  }
  return {
    ...snap,
    ts: anon.shiftTS(snap.ts),
    host: anon.fakeName('host', snap.host),
    wan,
  }
}

export function anonymizeLatest(anon: Anon, view: LatestView): LatestView {
  return { ...view, snapshot: anonymizeSnapshot(anon, view.snapshot) }
}

export function anonymizeHistory(anon: Anon, points: HistoryPoint[]): HistoryPoint[] {
  return points.map((p) => ({ ...p, ts: anon.shiftTS(p.ts) }))
}

export function anonymizeDashboard(anon: Anon, dash: Dashboard, cidrs: CidrMap[] = []): Dashboard {
  return {
    ...dash,
    ts: dash.ts ? anon.shiftTS(dash.ts) : dash.ts,
    host: dash.host ? anon.fakeName('host', dash.host) : dash.host,
    transfer_24h: {
      ...dash.transfer_24h,
      points: dash.transfer_24h.points?.map((p) => ({ ...p, ts: anon.shiftTS(p.ts) })),
    },
    monthly_wans: dash.monthly_wans?.map((w) => ({ ...w, name: anon.fakeISP(w.name) })),
    speedtest: dash.speedtest?.map((s) => ({
      ...s,
      name: anon.fakeISP(s.name),
      server: s.server ? anon.fakeISP(s.server) : s.server,
      location: s.location ? anon.fakeISP(s.location) : s.location,
      points: s.points?.map((p) => ({
        ...p,
        ts: anon.shiftTS(p.ts),
        server: p.server ? anon.fakeISP(p.server) : p.server,
        location: p.location ? anon.fakeISP(p.location) : p.location,
      })),
    })),
    dns: dash.dns
      ? {
          resolvers: dash.dns.resolvers?.map((r) => ({
            ...r,
            server: anon.rewriteText(r.server, cidrs),
            wan: r.wan ? anon.fakeISP(r.wan) : r.wan,
          })),
          queries: dash.dns.queries?.map((q) => ({ ...q, ts: anon.shiftTS(q.ts) })),
        }
      : dash.dns,
    top_upload: dash.top_upload?.map((r) => fakeFlow(anon, r, 'device', cidrs)),
    top_download: dash.top_download?.map((r) => fakeFlow(anon, r, 'device', cidrs)),
    top_dest_upload: dash.top_dest_upload?.map((r) => fakeFlow(anon, r, 'dest', cidrs)),
    top_dest_download: dash.top_dest_download?.map((r) => fakeFlow(anon, r, 'dest', cidrs)),
    top_regions: dash.top_regions?.map((r) => fakeFlow(anon, r, 'region', cidrs)),
  }
}

function fakeSwitch(anon: Anon, s: Switch, cidrs: CidrMap[]): Switch {
  return {
    ...s,
    mac: anon.fakeMAC(s.mac),
    name: s.name
      ? anon.fakeName(
          s.kind === 'ap' ? 'ap' : 'switch',
          s.name,
          '',
          s.kind === 'ap' ? undefined : switchNamePool(s),
        )
      : s.name,
    ip: s.ip ? anon.fakeIP(s.ip, cidrs) : s.ip,
    parent_mac: s.parent_mac ? anon.fakeMAC(s.parent_mac) : s.parent_mac,
    clients: s.clients?.map((m) => anon.fakeMAC(m)),
    ports: s.ports?.map((p) => ({
      ...p,
      clients: p.clients?.map((m) => anon.fakeMAC(m)),
    })),
  }
}

function fakeTree(anon: Anon, n: TopoNode, cidrs: CidrMap[]): TopoNode {
  return {
    ...n,
    mac: anon.fakeMAC(n.mac),
    name: n.name
      ? anon.fakeName(n.type === 'ap' ? 'ap' : n.type === 'box' ? 'box' : 'switch', n.name)
      : n.name,
    ip: n.ip ? anon.fakeIP(n.ip, cidrs) : n.ip,
    clients: n.clients?.map((m) => anon.fakeMAC(m)),
    children: n.children?.map((c) => fakeTree(anon, c, cidrs)),
  }
}

export function anonymizeTopo(anon: Anon, topo: TopoView, cidrs: CidrMap[] = []): TopoView {
  return {
    ...topo,
    ts: topo.ts ? anon.shiftTS(topo.ts) : topo.ts,
    host: topo.host ? anon.fakeName('host', topo.host) : topo.host,
    box: topo.box ? anonymizeBox(anon, topo.box) : topo.box,
    switches: topo.switches?.map((s) => fakeSwitch(anon, s, cidrs)),
    tree: topo.tree?.map((n) => fakeTree(anon, n, cidrs)),
  }
}

export function anonymizeWireless(anon: Anon, wifi: WirelessView, cidrs: CidrMap[] = []): WirelessView {
  return {
    ...wifi,
    meta: {
      ...wifi.meta,
      site: wifi.meta.site ? anon.fakeName('group', wifi.meta.site) : wifi.meta.site,
    },
    aps: wifi.aps.map((ap) => ({
      ...ap,
      mac: anon.fakeMAC(ap.mac),
      name: ap.name ? anon.fakeName('ap', ap.name) : ap.name,
      ip: ap.ip ? anon.fakeIP(ap.ip, cidrs) : ap.ip,
      uplink: ap.uplink ? anon.fakeMAC(ap.uplink) : ap.uplink,
    })),
    networks: wifi.networks.map((n) => ({
      ...n,
      name: anon.fakeName('ssid', n.name),
      ssid: n.ssid ? anon.fakeName('ssid', n.ssid) : n.ssid,
    })),
    clients: wifi.clients.map((c) => ({
      ...c,
      mac: anon.fakeMAC(c.mac),
      name: c.name ? anon.fakeName(clientNameKind(undefined, c.os), c.name) : c.name,
      hostname: c.hostname ? anon.fakeName(clientNameKind(undefined, c.os), c.hostname) : c.hostname,
      ip: c.ip ? anon.fakeIP(c.ip, cidrs) : c.ip,
      ssid: c.ssid ? anon.fakeName('ssid', c.ssid) : c.ssid,
      ap_mac: c.ap_mac ? anon.fakeMAC(c.ap_mac) : c.ap_mac,
      ap_name: c.ap_name ? anon.fakeName('ap', c.ap_name) : c.ap_name,
    })),
  }
}

function fakeNameSyncRow(anon: Anon, r: NameSyncRow, cidrs: CidrMap[]): NameSyncRow {
  return {
    ...r,
    mac: anon.fakeMAC(r.mac),
    ip: r.ip ? anon.fakeIP(r.ip, cidrs) : r.ip,
    firewalla_name: anon.fakeName('device', r.firewalla_name),
    unifi_name:
      r.status === 'empty' ? undefined : anon.fakeName('device', r.unifi_name || r.firewalla_name, 'unifi'),
  }
}

export function anonymizeAudit(anon: Anon, audit: AuditView, cidrs: CidrMap[] = []): AuditView {
  const row = <T extends { mac: string; name?: string; ip?: string }>(r: T, kind: NameKind = 'device'): T => ({
    ...r,
    mac: anon.fakeMAC(r.mac),
    name: r.name ? anon.fakeName(kind, r.name) : r.name,
    ...('ip' in r && r.ip ? { ip: anon.fakeIP(String(r.ip), cidrs) } : {}),
  })
  return {
    names: { count: audit.names.count, rows: audit.names.rows.map((r) => fakeNameSyncRow(anon, r, cidrs)) },
    vlan: { count: audit.vlan.count, rows: audit.vlan.rows.map((r) => row(r)) },
    stp: { count: audit.stp.count, rows: audit.stp.rows.map((r) => row(r, 'switch')) },
    unknown: { count: audit.unknown.count, rows: audit.unknown.rows.map((r) => row(r)) },
    offline: {
      count: audit.offline.count,
      rows: audit.offline.rows.map((r) => row(r, r.kind === 'ap' ? 'ap' : 'switch')),
    },
    pending: {
      count: audit.pending.count,
      rows: audit.pending.rows.map((r) => row(r, /ap|uap|wifi/i.test(r.model || '') ? 'ap' : 'switch')),
    },
  }
}

export function anonymizePolicies(anon: Anon, policies: Policy[], cidrs: CidrMap[] = []): Policy[] {
  return policies.map((p) => ({
    ...p,
    target: p.target ? anon.rewriteText(p.target, cidrs) : p.target,
    notes: p.notes ? anon.rewriteText(p.notes, cidrs) : p.notes,
  }))
}

export function anonymizeFwAppRules(
  anon: Anon,
  view: FwAppRulesView,
  cidrs: CidrMap[] = [],
): FwAppRulesView {
  return {
    ...view,
    rules: view.rules.map((r) => ({
      ...r,
      target: r.target ? anon.rewriteText(r.target, cidrs) : r.target,
      name: r.name ? anon.rewriteText(r.name, cidrs) : r.name,
      notes: r.notes ? anon.rewriteText(r.notes, cidrs) : r.notes,
      scopeLabel: r.scopeLabel ? anon.rewriteText(r.scopeLabel, cidrs) : r.scopeLabel,
      scope: r.scope?.map((m) => anon.fakeMAC(m)),
      tags: r.tags?.map((t) => anon.rewriteText(t, cidrs)),
    })),
    exceptions: view.exceptions.map((e) => ({
      ...e,
      target: e.target ? anon.rewriteText(e.target, cidrs) : e.target,
      targetName: e.targetName ? anon.rewriteText(e.targetName, cidrs) : e.targetName,
      reason: e.reason ? anon.rewriteText(e.reason, cidrs) : e.reason,
      ifTarget: e.ifTarget ? anon.rewriteText(e.ifTarget, cidrs) : e.ifTarget,
    })),
    scopes: view.scopes.map((s) => ({
      ...s,
      id:
        s.kind === 'device'
          ? anon.fakeMAC(s.id)
          : s.id.startsWith('tag:')
            ? `tag:${anon.rewriteText(s.id.slice(4), cidrs)}`
            : s.id,
      label: anon.rewriteText(s.label, cidrs),
    })),
  }
}

export function anonymizeTags(anon: Anon, tags: Tag[]): Tag[] {
  return tags.map((t) => ({
    ...t,
    name: anon.fakeName(t.type === 'user' ? 'user' : 'group', t.name),
  }))
}

export function anonymizeUnifi(anon: Anon, u: UnifiConsole, cidrs: CidrMap[] = []): UnifiConsole {
  return {
    ...u,
    name: u.name ? anon.fakeName('box', u.name) : u.name,
    ip: u.ip ? anon.fakeIP(u.ip, cidrs) : u.ip,
    inform: u.inform ? anon.rewriteText(u.inform, cidrs) : u.inform,
    site: u.site ? anon.fakeName('group', u.site) : u.site,
  }
}

export function anonymizeLogLines(anon: Anon, lines: AnonLogLine[], cidrs: CidrMap[] = []): AnonLogLine[] {
  return lines.map((ln) => ({
    ...ln,
    ts: anon.shiftTS(ln.ts),
    line: anon.rewriteText(ln.line, cidrs),
  }))
}

export function anonymizeNameSync(anon: Anon, rows: NameSyncRow[], cidrs: CidrMap[] = []): NameSyncRow[] {
  return rows.map((r) => fakeNameSyncRow(anon, r, cidrs))
}

export function anonymizeEnrollCmd(anon: Anon, cmd: string): string {
  return anon.rewriteText(cmd)
}

export function anonymizeAgentEvents(anon: Anon, events: AnonAgentEvent[]): AnonAgentEvent[] {
  return events.map((e) => ({ ...e, ts: anon.shiftTS(e.ts) }))
}

function anonymizeControlSnapshot(anon: Anon, v: unknown, cidrs: CidrMap[]): unknown {
  if (v == null) return v
  if (typeof v === 'string') return anon.rewriteText(v, cidrs)
  if (typeof v === 'number' || typeof v === 'boolean') return v
  if (Array.isArray(v)) return v.map((x) => anonymizeControlSnapshot(anon, x, cidrs))
  if (typeof v === 'object') {
    const out: Record<string, unknown> = {}
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
      if (typeof val === 'string' && /^(name|hostname|dns)$/i.test(k)) {
        out[k] = anon.fakeName('device', val)
      } else {
        out[k] = anonymizeControlSnapshot(anon, val, cidrs)
      }
    }
    return out
  }
  return v
}

/** Anonymize control-plane History rows (not metrics HistoryPoint). */
export function anonymizeControlEvents(
  anon: Anon,
  events: ControlEvent[],
  cidrs: CidrMap[] = [],
): ControlEvent[] {
  return events.map((e) => ({
    ...e,
    ts: e.ts ? e.ts + anon.offsetSec * 1000 : e.ts,
    target: e.target
      ? looksLikeMAC(e.target)
        ? anon.fakeMAC(e.target)
        : anon.rewriteText(e.target, cidrs)
      : e.target,
    summary: e.summary
      ? looksLikeMAC(e.summary)
        ? anon.fakeMAC(e.summary)
        : anon.fakeName('device', e.summary)
      : e.summary,
    error: e.error ? anon.rewriteText(e.error, cidrs) : e.error,
    before: anonymizeControlSnapshot(anon, e.before, cidrs),
    after: anonymizeControlSnapshot(anon, e.after, cidrs),
  }))
}

export function anonymizeModules(anon: Anon, mods: ModuleInfo[]): ModuleInfo[] {
  return mods.map((m) => ({
    ...m,
    site: m.site ? anon.fakeName('group', m.site) : m.site,
  }))
}

