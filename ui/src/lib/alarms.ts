// LAN get-alarms counts describe the bounded reply, not the complete inbox.
export function alarmCountLimited(source: string | undefined, count: number): boolean {
  return source === 'fw-app-get' && count >= 500
}

export function formatAlarmCount(count: number, limited: boolean): string {
  return `${count}${limited ? '+' : ''}`
}
