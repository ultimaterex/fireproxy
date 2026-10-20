import { describe, expect, it } from 'vitest'
import { isNavigationReturn, navigationKey } from './navigation-scroll'
import type { NavFrame } from './nav'

describe('navigation scroll destinations', () => {
  const list: NavFrame[] = [{ kind: 'tab', tab: 'devices' }, { kind: 'group', id: '1', label: 'Lab' }]
  const detail: NavFrame[] = [...list, { kind: 'device', mac: '00:00:00:00:00:01', label: 'Test device' }]

  it('restores only when returning to an ancestor, not when switching screens or opening details', () => {
    expect(isNavigationReturn(detail, list)).toBe(true)
    expect(isNavigationReturn(list, detail)).toBe(false)
    expect(isNavigationReturn(detail, [{ kind: 'tab', tab: 'groups' }])).toBe(false)
    expect(isNavigationReturn(list, list)).toBe(false)
  })

  it('ignores display-label updates while preserving identity and filter scope', () => {
    expect(navigationKey(list)).toBe(navigationKey([{ kind: 'tab', tab: 'devices' }, { kind: 'group', id: '1', label: 'Renamed' }]))
    expect(navigationKey(list)).not.toBe(navigationKey([{ kind: 'tab', tab: 'devices' }, { kind: 'group', id: '2', label: 'Lab' }]))
  })
})
