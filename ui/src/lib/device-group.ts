export type DeviceGroupBy = 'none' | 'port' | 'lan' | 'tag' | 'online'

export const DEVICE_GROUP_BY_OPTIONS: { id: DeviceGroupBy; label: string }[] = [
  { id: 'none', label: 'None' },
  { id: 'port', label: 'Port' },
  { id: 'lan', label: 'LAN' },
  { id: 'tag', label: 'Tag' },
  { id: 'online', label: 'Online' },
]

export type DeviceGroupFields = {
  online: boolean
  membership: { id: string; name: string } | null
  lan?: { uuid?: string; label: string }
  loc?: {
    switchMac: string
    switchName: string
    portId: string
    label: string
    clients: string[]
  }
}

export type DeviceRowGroup<T> = {
  key: string
  label: string
  clients: string[]
  lanUuid?: string
  tagId?: string
  rows: T[]
}

type Bucket<T> = DeviceRowGroup<T> & { order: number; sortName: string }

export function groupDeviceRows<T>(
  rows: T[],
  groupBy: DeviceGroupBy,
  fields: (row: T) => DeviceGroupFields,
): DeviceRowGroup<T>[] {
  if (groupBy === 'none') return [{ key: 'all', label: '', clients: [], rows }]

  const buckets = new Map<string, Bucket<T>>()
  for (const row of rows) {
    const parts = bucketFor(fields(row), groupBy)
    let g = buckets.get(parts.key)
    if (!g) {
      g = { ...parts, rows: [] }
      buckets.set(parts.key, g)
    }
    g.rows.push(row)
  }

  return [...buckets.values()].sort((a, b) => {
    if (a.key === '_') return 1
    if (b.key === '_') return -1
    if (a.order !== b.order) return a.order - b.order
    const byName = a.sortName.localeCompare(b.sortName, undefined, { numeric: true })
    if (byName !== 0) return byName
    return a.label.localeCompare(b.label, undefined, { numeric: true })
  })
}

function bucketFor(
  row: DeviceGroupFields,
  groupBy: Exclude<DeviceGroupBy, 'none'>,
): Omit<Bucket<unknown>, 'rows'> {
  switch (groupBy) {
    case 'port': {
      const loc = row.loc
      if (!loc) return { key: '_', label: 'Other', clients: [], order: 9999, sortName: '' }
      const portNum = Number(loc.portId)
      return {
        key: `${loc.switchMac}:${loc.portId}`,
        label: loc.label,
        clients: loc.clients,
        order: Number.isFinite(portNum) ? portNum : 999,
        sortName: loc.switchName,
      }
    }
    case 'lan': {
      const lan = row.lan
      if (!lan?.uuid) return { key: '_', label: 'Other', clients: [], order: 1, sortName: '' }
      return {
        key: lan.uuid,
        label: lan.label || lan.uuid,
        clients: [],
        lanUuid: lan.uuid,
        order: 0,
        sortName: lan.label || lan.uuid,
      }
    }
    case 'tag': {
      const m = row.membership
      if (!m) return { key: '_', label: 'Other', clients: [], order: 1, sortName: '' }
      return {
        key: m.id,
        label: m.name,
        clients: [],
        tagId: m.id,
        order: 0,
        sortName: m.name,
      }
    }
    case 'online':
      return row.online
        ? { key: 'online', label: 'Online', clients: [], order: 0, sortName: 'Online' }
        : { key: 'offline', label: 'Offline', clients: [], order: 1, sortName: 'Offline' }
  }
}
