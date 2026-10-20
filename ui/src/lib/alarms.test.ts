import { expect, it } from 'vitest'
import { alarmCountLimited, formatAlarmCount } from './alarms'

it('labels a full bounded LAN reply as a lower bound without changing true totals', () => {
  expect(formatAlarmCount(75, alarmCountLimited('fw-app-get', 75))).toBe('75')
  expect(formatAlarmCount(500, alarmCountLimited('fw-app-get', 500))).toBe('500+')
  expect(formatAlarmCount(620, alarmCountLimited('fw-app-init', 620))).toBe('620')
})
