import type { Tab } from '@/lib/types'

export type WirelessMode = 'radios' | 'networks' | 'clients'

export type NavFrame =
  | { kind: 'tab'; tab: Tab }
  | { kind: 'lan'; uuid: string; label: string }
  | { kind: 'group'; id: string; label: string; tagType?: 'group' | 'user' | 'device' }
  | { kind: 'ports'; macs: string[]; label: string }
  | { kind: 'ap'; mac: string; label: string }
  | { kind: 'ssid'; key: string; label: string }
  | { kind: 'device'; mac: string; label: string }
  | { kind: 'region'; cc: string; label: string }
  | { kind: 'topo-switch'; mac: string; label: string }
  | { kind: 'topo-port'; mac: string; port: string; label: string }
  | { kind: 'topo-box'; label: string }
  | { kind: 'topo-gold'; n: number; label: string }

export function tabOf(stack: NavFrame[]): Tab {
  const root = stack[0]
  return root?.kind === 'tab' ? root.tab : 'metrics'
}

export function pushFrame(stack: NavFrame[], frame: NavFrame): NavFrame[] {
  return [...stack, frame]
}

export function popTo(stack: NavFrame[], index: number): NavFrame[] {
  if (index < 0) return stack.slice(0, 1)
  return stack.slice(0, index + 1)
}

export function resetTab(tab: Tab): NavFrame[] {
  return [{ kind: 'tab', tab }]
}

export function findLast<T extends NavFrame['kind']>(
  stack: NavFrame[],
  kind: T,
): Extract<NavFrame, { kind: T }> | undefined {
  for (let i = stack.length - 1; i >= 0; i--) {
    const f = stack[i]
    if (f.kind === kind) return f as Extract<NavFrame, { kind: T }>
  }
  return undefined
}

export function crumbLabel(f: NavFrame, tabLabel: (t: Tab) => string): string {
  switch (f.kind) {
    case 'tab':
      return tabLabel(f.tab)
    case 'lan':
    case 'group':
    case 'ports':
    case 'ap':
    case 'ssid':
    case 'device':
    case 'region':
    case 'topo-switch':
    case 'topo-port':
    case 'topo-box':
    case 'topo-gold':
      return f.label
  }
}
