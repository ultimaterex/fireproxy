import type { Switch, SwitchPort } from '@/lib/types'

const KEY = 'fireproxy.debug.vrackSims'
const MAC_PREFIX = 'DE:BU:60:00:00:'

export type VRackSimPreset = 'rj24-sfp4' | 'rj48' | 'rj8-sfp2'

export type VRackSim = {
  id: string
  preset: VRackSimPreset
  sw: Switch
}

export const VRACK_SIM_PRESETS: {
  id: VRackSimPreset
  label: string
  detail: string
}[] = [
  { id: 'rj24-sfp4', label: '24× RJ45 + 4× SFP', detail: 'Generic shelf stub' },
  { id: 'rj48', label: '48× RJ45', detail: 'Dual-rail generic shelf' },
  { id: 'rj8-sfp2', label: '8× RJ45 + 2× SFP', detail: 'Small unknown switch' },
]

export function loadVRackSims(): VRackSim[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as VRackSim[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function saveVRackSims(sims: VRackSim[]) {
  localStorage.setItem(KEY, JSON.stringify(sims))
}

export function addVRackSim(
  preset: VRackSimPreset,
  parent?: { mac: string; port: string } | null,
): VRackSim {
  const sims = loadVRackSims()
  const n = sims.length + 1
  const mac = `${MAC_PREFIX}${n.toString(16).padStart(2, '0')}`.toUpperCase()
  const sw = buildSimSwitch(preset, mac, n, parent)
  const sim: VRackSim = { id: mac, preset, sw }
  sims.push(sim)
  saveVRackSims(sims)
  return sim
}

export function removeVRackSim(id: string) {
  saveVRackSims(loadVRackSims().filter((s) => s.id.toUpperCase() !== id.toUpperCase()))
}

export function clearVRackSims() {
  saveVRackSims([])
}

function buildSimSwitch(
  preset: VRackSimPreset,
  mac: string,
  n: number,
  parent?: { mac: string; port: string } | null,
): Switch {
  let ports: SwitchPort[] = []
  let model = 'Sim Switch'
  if (preset === 'rj24-sfp4') {
    model = 'Sim-24-SFP'
    ports = [...rjPorts(1, 24), ...sfpPorts(25, 28)]
  } else if (preset === 'rj48') {
    model = 'Sim-48'
    ports = rjPorts(1, 48)
  } else {
    model = 'Sim-8-SFP'
    ports = [...rjPorts(1, 8), ...sfpPorts(9, 10)]
  }
  if (ports[0]) ports[0] = { ...ports[0], uplink: true, up: true }

  return {
    mac,
    name: `Sim ${n}`,
    model,
    kind: 'switch',
    source: 'debug',
    healthy: true,
    uplink_port: ports[0]?.id,
    parent_mac: parent?.mac,
    parent_port: parent?.port,
    ports,
  }
}

function rjPorts(from: number, to: number): SwitchPort[] {
  const out: SwitchPort[] = []
  for (let i = from; i <= to; i++) {
    out.push({ id: String(i), up: i % 3 !== 0, sfp: false })
  }
  return out
}

function sfpPorts(from: number, to: number): SwitchPort[] {
  const out: SwitchPort[] = []
  for (let i = from; i <= to; i++) {
    out.push({ id: String(i), up: i % 2 === 0, sfp: true })
  }
  return out
}
