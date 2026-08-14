export const DEVICE_COLS = [
  { id: 'name', label: 'Device', locked: true },
  { id: 'group', label: 'User/Group', locked: false },
  { id: 'status', label: 'Status', locked: false },
  { id: 'dap', label: 'DAP Status', locked: false },
  { id: 'learned', label: 'Learned Flows', locked: false },
  { id: 'trusted', label: 'Trusted Flows', locked: false },
  { id: 'blocked', label: 'Blocked Flows', locked: false },
  { id: 'ip', label: 'IP Address', locked: false },
  { id: 'port', label: 'Port', locked: false },
  { id: 'network', label: 'Network', locked: false },
  { id: 'ipv6', label: 'IPv6 Addresses', locked: false },
  { id: 'mac', label: 'MAC Address', locked: false },
  { id: 'vendor', label: 'Manufacturer', locked: false },
  { id: 'download', label: 'Download', locked: false },
  { id: 'upload', label: 'Upload', locked: false },
] as const

export type DeviceColId = (typeof DEVICE_COLS)[number]['id']

const DEFAULT: DeviceColId[] = ['name', 'group', 'status', 'ip', 'vendor', 'download', 'upload']
const KEY = 'fireproxy.devices.cols'
const PORT_OFF = 'fireproxy.devices.cols.portOff'

export function defaultDeviceCols(): DeviceColId[] {
  return [...DEFAULT]
}

export function loadDeviceCols(): DeviceColId[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return defaultDeviceCols()
    const ids = JSON.parse(raw) as string[]
    const allowed = new Set<string>(DEVICE_COLS.map((c) => c.id))
    let out = ids.filter((id): id is DeviceColId => allowed.has(id))
    if (!out.includes('name')) out.unshift('name')
    if (!localStorage.getItem(PORT_OFF)) {
      out = out.filter((id) => id !== 'port')
      localStorage.setItem(PORT_OFF, '1')
    }
    return out.length ? out : defaultDeviceCols()
  } catch {
    return defaultDeviceCols()
  }
}

export function saveDeviceCols(ids: DeviceColId[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(ids))
  } catch {
    /* ignore */
  }
}
