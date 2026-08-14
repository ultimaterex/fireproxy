import type { Tab, ViewMode } from '@/lib/types'

const PREFIX = 'fireproxy.view.'

export function loadViewMode(tab: Tab): ViewMode {
  try {
    const v = localStorage.getItem(PREFIX + tab)
    if (v === 'list' || v === 'visual') return v
  } catch {
    /* ignore */
  }
  return 'visual'
}

export function saveViewMode(tab: Tab, mode: ViewMode) {
  try {
    localStorage.setItem(PREFIX + tab, mode)
  } catch {
    /* ignore */
  }
}

export function loadAllViewModes(tabs: Tab[]): Record<Tab, ViewMode> {
  const out = {} as Record<Tab, ViewMode>
  for (const tab of tabs) out[tab] = loadViewMode(tab)
  return out
}
