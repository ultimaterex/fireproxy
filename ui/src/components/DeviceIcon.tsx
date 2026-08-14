import type { LucideIcon } from 'lucide-react'
import {
  Camera,
  Car,
  Cpu,
  Gamepad2,
  Globe,
  HardDrive,
  Laptop,
  Monitor,
  Network,
  Printer,
  Router,
  Smartphone,
  Speaker,
  Tablet,
  Tv,
  Watch,
  Wifi,
} from 'lucide-react'

import { cn } from '@/lib/utils'

const ICONS: Record<string, LucideIcon> = {
  phone: Smartphone,
  smartphone: Smartphone,
  tablet: Tablet,
  desktop: Monitor,
  laptop: Laptop,
  computer: Monitor,
  tv: Tv,
  television: Tv,
  'media streaming': Tv,
  nas: HardDrive,
  server: HardDrive,
  'nas and server': HardDrive,
  printer: Printer,
  camera: Camera,
  router: Router,
  ap: Wifi,
  'access point': Wifi,
  wifi: Wifi,
  speaker: Speaker,
  'smart speaker': Speaker,
  wearable: Watch,
  watch: Watch,
  console: Gamepad2,
  gaming: Gamepad2,
  'game console': Gamepad2,
  iot: Cpu,
  'iot default': Cpu,
  appliance: Cpu,
  car: Car,
  'appliance and car': Car,
  switch: Network,
  dest: Globe,
  destination: Globe,
}

export function DeviceIcon({
  type,
  className,
}: {
  type?: string
  className?: string
}) {
  const key = (type ?? '').toLowerCase().replace(/[_-]+/g, ' ').trim()
  const Icon = ICONS[key] ?? Cpu
  return (
    <span title={key || undefined} className="inline-flex">
      <Icon className={cn('size-4 shrink-0 text-muted-foreground', className)} aria-hidden />
    </span>
  )
}
