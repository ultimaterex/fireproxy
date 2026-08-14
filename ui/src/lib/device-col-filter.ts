import type { DeviceColId } from '@/lib/device-cols'
import { dapLabel, netLabel, preferredName } from '@/lib/format'
import type { SearchKey } from '@/lib/device-search'
import type { PortLoc } from '@/lib/switch-port'
import type { Device, NetIface } from '@/lib/types'

export type ColFilterOpt = { key: SearchKey; value: string }

const NONE: DeviceColId[] = ['learned', 'trusted', 'blocked', 'download', 'upload']

export function colFilterKeys(id: DeviceColId): SearchKey[] {
  switch (id) {
    case 'name':
      return ['device']
    case 'group':
      return ['user', 'group']
    case 'status':
      return ['status']
    case 'dap':
      return ['dapstatus']
    case 'ip':
    case 'ipv6':
      return ['ip']
    case 'network':
      return ['network']
    case 'port':
      return ['switch', 'port']
    case 'mac':
      return ['mac']
    case 'vendor':
      return ['manufacturer']
    default:
      return []
  }
}

export function colFilterOptions(
  id: DeviceColId,
  devices: Device[],
  ctx: {
    uuidToNet: Map<string, NetIface>
    labelTag: (id: string, preferType?: string) => string
    portOf?: Map<string, PortLoc>
  },
): ColFilterOpt[] {
  if (NONE.includes(id)) return []
  const out: ColFilterOpt[] = []
  const add = (key: SearchKey, value: string) => {
    const v = value.trim()
    if (v) out.push({ key, value: v })
  }

  if (id === 'status') {
    add('status', 'Online')
    add('status', 'Offline')
    return uniq(out)
  }

  for (const d of devices) {
    switch (id) {
      case 'name':
        add('device', preferredName(d))
        break
      case 'group':
        for (const uid of d.user_tag_ids ?? []) add('user', ctx.labelTag(uid, 'user'))
        for (const gid of d.tag_ids ?? []) add('group', ctx.labelTag(gid, 'group'))
        break
      case 'dap':
        if (d.dap) add('dapstatus', dapLabel(d.dap.status))
        break
      case 'ip':
        add('ip', d.ip ?? '')
        break
      case 'ipv6':
        for (const v of d.ipv6 ?? []) add('ip', v)
        break
      case 'network': {
        const lan = d.intf_uuid ? ctx.uuidToNet.get(d.intf_uuid) : undefined
        if (lan) add('network', netLabel(lan))
        break
      }
      case 'port': {
        const loc = ctx.portOf?.get(d.mac.toUpperCase())
        if (loc) {
          add('switch', loc.switchName)
          add('port', loc.uplink ? 'Uplink' : loc.portId)
        }
        break
      }
      case 'mac':
        add('mac', d.mac)
        break
      case 'vendor':
        add('manufacturer', d.vendor || 'Unknown')
        break
    }
  }
  return uniq(out)
}

function uniq(items: ColFilterOpt[]): ColFilterOpt[] {
  const seen = new Set<string>()
  const out: ColFilterOpt[] = []
  for (const it of items) {
    const k = `${it.key}:${it.value.toLowerCase()}`
    if (seen.has(k)) continue
    seen.add(k)
    out.push(it)
  }
  return out.sort((a, b) => a.value.localeCompare(b.value, undefined, { numeric: true }))
}
