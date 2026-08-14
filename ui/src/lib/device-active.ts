import type { Device } from '@/lib/types'

const KEY = 'fireproxy.devices.showInactive'

/** True when catalog has at least one stamped active flag (post-agent deploy). */
export function catalogHasActiveStamp(devices: Device[]): boolean {
  return devices.some((d) => d.active !== undefined)
}

export function filterDevicesByActive(devices: Device[], showInactive: boolean): Device[] {
  if (showInactive || !catalogHasActiveStamp(devices)) return devices
  return devices.filter((d) => d.active === true)
}

export function inactiveDeviceCount(devices: Device[]): number {
  if (!catalogHasActiveStamp(devices)) return 0
  return devices.reduce((n, d) => n + (d.active === true ? 0 : 1), 0)
}

export function loadShowInactive(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

export function saveShowInactive(on: boolean) {
  try {
    localStorage.setItem(KEY, on ? '1' : '0')
  } catch {
    /* ignore */
  }
}
