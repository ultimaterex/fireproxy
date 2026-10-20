import { Activity, Box, Cable, ClipboardList, History, MonitorSmartphone, Network, ScrollText, Shield, ShieldAlert, Users, Wifi } from 'lucide-react'
import type { Tab } from '@/lib/types'

export const NAV: { id: Tab; label: string; icon: typeof Activity }[] = [
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'inventory', label: 'Inventory', icon: Box },
  { id: 'network', label: 'Network', icon: Cable },
  { id: 'topology', label: 'Topology', icon: Network },
  { id: 'wireless', label: 'Wireless', icon: Wifi },
  { id: 'audit', label: 'Audit', icon: ClipboardList },
  { id: 'history', label: 'History', icon: History },
  { id: 'devices', label: 'Devices', icon: MonitorSmartphone },
  { id: 'alarms', label: 'Alarms', icon: ShieldAlert },
  { id: 'rules', label: 'Rules', icon: Shield },
  { id: 'groups', label: 'Groups', icon: Users },
  { id: 'logs', label: 'Logs', icon: ScrollText },
]

