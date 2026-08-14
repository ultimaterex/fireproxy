import type { BlockDef, Kit, NumberSide, ProductDef, Rail, VRackMaker, VRackType } from '@/vrack/types'

export function parseKit(raw: unknown): Kit {
  const o = obj(raw)
  const u = obj(o.u)
  const screen = obj(o.screen ?? o.lcm)
  const rj45 = obj(o.rj45)
  const sfp = obj(o.sfp)
  const keystone = obj(o.keystone)
  return {
    scale_px_per_in: num(o.scale_px_per_in, 'kit.scale_px_per_in'),
    u: {
      width_in: num(u.width_in, 'kit.u.width_in'),
      height_in: num(u.height_in, 'kit.u.height_in'),
    },
    screen: { diagonal_in: num(screen.diagonal_in, 'kit.screen.diagonal_in') },
    rj45: { width_in: num(rj45.width_in, 'kit.rj45.width_in') },
    sfp: {
      width_mm: num(sfp.width_mm, 'kit.sfp.width_mm'),
      height_mm: typeof sfp.height_mm === 'number' ? sfp.height_mm : 8.5,
    },
    keystone: { width_in: typeof keystone.width_in === 'number' ? keystone.width_in : 0.58 },
  }
}

export function parseProduct(raw: unknown, source: string): ProductDef {
  const o = obj(raw)
  const inferred = fromPath(source)
  const manufacturer = (str(o.manufacturer) ?? inferred.manufacturer) as VRackMaker
  const type = (str(o.type) ?? inferred.type) as VRackType
  const product = str(o.product) ?? inferred.product
  if (!manufacturer || !type || !product) {
    throw new Error(`vrack: ${source} needs manufacturer/type/product`)
  }
  const match = obj(o.match)
  const models = arr(match.model).map((m) => String(m).trim()).filter(Boolean)
  if (models.length === 0) models.push(product)
  return {
    manufacturer,
    type,
    product,
    match: { model: models },
    ru: typeof o.ru === 'number' ? o.ru : 1,
    form: o.form === 'shelf' ? 'shelf' : 'rack',
    width_in: typeof o.width_in === 'number' ? o.width_in : undefined,
    height_in: typeof o.height_in === 'number' ? o.height_in : undefined,
    align: o.align === 'left' || o.align === 'right' || o.align === 'center' ? o.align : undefined,
    ear_in: typeof o.ear_in === 'number' ? o.ear_in : undefined,
    window_in: typeof o.window_in === 'number' ? o.window_in : undefined,
    blocks: arr(o.blocks).map((b, i) => parseBlock(b, `${source} blocks[${i}]`)),
    source,
  }
}

export function expandPorts(spec: string | Array<string | number>): string[] {
  if (Array.isArray(spec)) return spec.map((v) => String(v).trim()).filter(Boolean)
  const out: string[] = []
  for (const part of spec.split(',').map((s) => s.trim()).filter(Boolean)) {
    const m = /^(\d+)\s*-\s*(\d+)$/.exec(part)
    if (!m) {
      out.push(part)
      continue
    }
    const a = Number(m[1])
    const b = Number(m[2])
    const step = a <= b ? 1 : -1
    for (let n = a; step > 0 ? n <= b : n >= b; n += step) out.push(String(n))
  }
  return out
}

function parseBlock(raw: unknown, path: string): BlockDef {
  const o = obj(raw)
  const kind = str(o.kind)
  if (kind === 'screen' || kind === 'badge' || kind === 'trailer') return { kind }
  if (kind === 'keystone') {
    return {
      kind,
      side: o.side === 'right' ? 'right' : 'left',
      rail: rail(o.rail, 'center'),
      slots: typeof o.slots === 'number' && o.slots > 0 ? Math.floor(o.slots) : 8,
    }
  }
  if (kind === 'rj45') {
    const order = o.order === 'columns' || o.order === 'rows' ? o.order : undefined
    return {
      kind,
      rail: o.rail === 'high' || o.rail === 'center' || o.rail === 'low' ? o.rail : order === 'columns' ? undefined : 'low',
      order,
      odds: o.odds === 'high' || o.odds === 'low' ? o.odds : undefined,
      bank: typeof o.bank === 'number' ? o.bank : undefined,
      ports: typeof o.ports === 'string' ? o.ports : expandPorts(arr(o.ports) as Array<string | number>).join(','),
      numbers: o.numbers === 'above' || o.numbers === 'below' ? o.numbers : 'below',
      gap: typeof o.gap === 'number' ? o.gap : undefined,
      lead: typeof o.lead === 'number' ? o.lead : undefined,
    }
  }
  if (kind === 'sfp') {
    const nums = o.numbers && typeof o.numbers === 'object' ? (o.numbers as Record<string, unknown>) : {}
    return {
      kind,
      cols: typeof o.cols === 'number' ? o.cols : 2,
      order: o.order === 'rows' ? 'rows' : 'columns',
      ports: expandPorts(o.ports as string | Array<string | number>),
      numbers: {
        high: side(nums.high, 'above'),
        low: side(nums.low, 'below'),
      },
      rail: o.rail === 'high' || o.rail === 'center' || o.rail === 'low' ? o.rail : undefined,
      gap: typeof o.gap === 'number' ? o.gap : undefined,
    }
  }
  throw new Error(`vrack: unknown block ${kind ?? '?'} at ${path}`)
}

function fromPath(source: string): { manufacturer?: VRackMaker; type?: VRackType; product?: string } {
  const m = /catalog\/([^/]+)\/([^/]+)\/([^/]+)\.ya?ml$/.exec(source.replaceAll('\\', '/'))
  if (!m) return {}
  return {
    manufacturer: m[1] as VRackMaker,
    type: m[2] as VRackType,
    product: m[3],
  }
}

function rail(v: unknown, fallback: Rail): Rail {
  return v === 'high' || v === 'center' || v === 'low' ? v : fallback
}

function side(v: unknown, fallback: NumberSide): NumberSide {
  return v === 'above' || v === 'below' ? v : fallback
}

function obj(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {}
}

function arr(v: unknown): unknown[] {
  return Array.isArray(v) ? v : []
}

function str(v: unknown): string | undefined {
  return typeof v === 'string' && v.trim() ? v.trim() : undefined
}

function num(v: unknown, path: string): number {
  if (typeof v !== 'number' || !Number.isFinite(v)) throw new Error(`vrack: ${path} must be a number`)
  return v
}
