import type { Switch } from '@/lib/types'
import { loadCatalog } from '@/vrack/catalog'
import { expandPorts } from '@/vrack/parse'
import type { NumberSide, PlacedJack, ProductDef, Rail, ResolvedFaceplate, VRackMaker, VRackType } from '@/vrack/types'

const catalog = loadCatalog()

export function rackFaceWidth(): number {
  return catalog.kit.u.width_in * catalog.kit.scale_px_per_in
}

export function normModel(s: string): string {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, '')
}

export function findProduct(opts: {
  manufacturer?: VRackMaker
  type: VRackType
  model?: string
  name?: string
}): ProductDef | undefined {
  const keys = [opts.model, opts.name].filter(Boolean).map((s) => normModel(s as string))
  return catalog.products.find((p) => {
    if (opts.manufacturer && p.manufacturer !== opts.manufacturer) return false
    if (p.type !== opts.type) return false
    const aliases = [p.product, ...p.match.model].map(normModel)
    return keys.some((k) =>
      aliases.some((a) => k === a || (a.length >= 8 && k.includes(a)) || (k.length >= 8 && a.includes(k))),
    )
  })
}

/** Unmatched inventory switch: shelf stub from live RJ45 / SFP ports. */
export function resolveGenericShelf(sw: Switch, manufacturer?: VRackMaker): ResolvedFaceplate {
  const kit = catalog.kit
  const px = kit.scale_px_per_in
  const ports = [...(sw.ports ?? [])].sort(
    (a, b) => Number(a.id) - Number(b.id) || a.id.localeCompare(b.id),
  )
  const copper = ports.filter((p) => !p.sfp)
  const fibre = ports.filter((p) => p.sfp)
  const dual = copper.length > 24

  const height = dual ? kit.u.height_in * px : Math.max(28, 1.1 * px)
  const pad = 4
  const packGap = 3
  const sfpLead = 8
  const sfpColGap = 3
  const sfpGap = 2
  const screenSize = (kit.screen.diagonal_in / Math.SQRT2) * px
  const guideSize = Math.min(screenSize, height - 10)
  const guide = { y: (height - guideSize) / 2, size: guideSize }
  const rjW0 = kit.rj45.width_in * px
  const rjH0 = Math.min(screenSize / 2, height * 0.55)
  const sfpW0 = (kit.sfp.width_mm / 25.4) * px
  const sfpH0 = Math.min((guideSize - sfpGap) / 2, height * 0.4)
  const badgeSize = manufacturer ? Math.min(14, height - 8) : 0
  const rackW = kit.u.width_in * px

  const jacks: PlacedJack[] = []
  const origin = pad + (badgeSize ? badgeSize + pad : 0)
  let x = origin

  if (dual) {
    const low = copper.slice(0, 24)
    const high = copper.slice(24)
    const cols = Math.max(low.length, high.length, 1)
    const slot = rjW0 + packGap
    for (let i = 0; i < cols; i++) {
      const cx = x + i * slot + packGap / 2
      if (high[i]) {
        jacks.push({
          id: high[i].id,
          kind: 'rj45',
          rail: 'high',
          numbers: 'above',
          x: cx,
          y: railY('high', rjH0, height, guide),
          w: rjW0,
          h: rjH0,
        })
      }
      if (low[i]) {
        jacks.push({
          id: low[i].id,
          kind: 'rj45',
          rail: 'low',
          numbers: 'below',
          x: cx,
          y: railY('low', rjH0, height, guide),
          w: rjW0,
          h: rjH0,
        })
      }
    }
    x += cols * slot
  } else {
    const slot = rjW0 + packGap
    for (const p of copper) {
      jacks.push({
        id: p.id,
        kind: 'rj45',
        rail: 'center',
        numbers: 'below',
        x: x + packGap / 2,
        y: railY('center', rjH0, height, guide),
        w: rjW0,
        h: rjH0,
      })
      x += slot
    }
  }

  if (fibre.length) {
    x += sfpLead
    const stacked = fibre.length > 4
    const cols = stacked ? Math.ceil(fibre.length / 2) : fibre.length
    for (let i = 0; i < fibre.length; i++) {
      const col = stacked ? Math.floor(i / 2) : i
      const row = stacked ? i % 2 : 0
      const rail: Rail = stacked ? (row === 0 ? 'high' : 'low') : 'center'
      jacks.push({
        id: fibre[i].id,
        kind: 'sfp',
        rail,
        numbers: stacked ? (rail === 'high' ? 'above' : 'below') : 'below',
        x: x + col * (sfpW0 + sfpColGap),
        y: stacked ? guide.y + row * (sfpH0 + sfpGap) : railY('center', sfpH0, height, guide),
        w: sfpW0,
        h: sfpH0,
      })
    }
    x += cols * sfpW0 + Math.max(0, cols - 1) * sfpColGap
  }

  x += pad
  let width = Math.max(x, origin + 48 + pad)
  if (width > rackW && jacks.length) {
    const span = x - origin - pad
    const fit = Math.max(0.35, (rackW - origin - pad) / Math.max(span, 1))
    for (const j of jacks) {
      j.x = origin + (j.x - origin) * fit
      j.w *= fit
      j.h *= fit
      if (j.kind === 'sfp' && j.rail !== 'center') {
        const row = j.rail === 'high' ? 0 : 1
        j.y = guide.y + row * (j.h + sfpGap * fit)
      } else {
        j.y = railY(j.rail, j.h, height, guide)
      }
    }
    width = rackW
  } else {
    width = Math.min(width, rackW)
  }

  const badge = badgeSize
    ? {
        x: pad,
        y: (height - badgeSize) / 2,
        w: badgeSize,
        h: badgeSize,
      }
    : undefined

  if (badge && jacks.length > 0) {
    const first = Math.min(...jacks.map((j) => j.x))
    const left = pad
    const right = first - pad
    const room = right - left
    if (room >= badge.w) {
      badge.x = left + (room - badge.w) / 2
    }
  }

  const model = (sw.model || sw.name || 'switch').trim()
  const def: ProductDef = {
    manufacturer: manufacturer ?? 'unifi',
    type: 'switch',
    product: 'generic-shelf',
    match: { model: [model] },
    ru: 1,
    form: 'shelf',
    width_in: width / px,
    height_in: height / px,
    blocks: badge ? [{ kind: 'badge' }] : [],
    source: 'generic',
  }

  return {
    def,
    width,
    height,
    chassis: { x: 0, width },
    badge,
    jacks,
    keystones: [],
  }
}

export function resolveFaceplate(product: ProductDef, kit = catalog.kit): ResolvedFaceplate {
  const px = kit.scale_px_per_in
  const shelf = product.form === 'shelf'
  const height = shelf
    ? Math.max(28, (product.height_in ?? 0.83) * px)
    : kit.u.height_in * px * (product.ru || 1)
  const rackW = kit.u.width_in * px
  const chassisW = Math.min(
    shelf ? Number.POSITIVE_INFINITY : rackW,
    (product.width_in ?? (shelf ? 4.6 : kit.u.width_in)) * px,
  )
  const width = shelf ? chassisW : rackW
  const shortEar = shelf ? 0 : Math.max(0, (product.ear_in ?? 0) * px)
  const align = product.align ?? 'center'
  let chassisX = shelf ? 0 : (width - chassisW) / 2
  let extension: ResolvedFaceplate['extension']
  if (!shelf && chassisW < width - 0.5) {
    if (align === 'left') {
      chassisX = shortEar
      const extW = width - chassisX - chassisW
      if (extW > 1) extension = { x: chassisX + chassisW, width: extW }
    } else if (align === 'right') {
      chassisX = width - chassisW - shortEar
      const extW = chassisX
      if (extW > 1) extension = { x: 0, width: extW }
    }
  }
  const chassis = { x: chassisX, width: chassisW }

  const screenSize = (kit.screen.diagonal_in / Math.SQRT2) * px
  const guideSize = Math.min(screenSize, height - 10)
  const rjW = kit.rj45.width_in * px
  const rjH = Math.min(screenSize / 2, height * 0.55)
  const sfpW = (kit.sfp.width_mm / 25.4) * px
  const ksW = kit.keystone.width_in * px
  const sfpGap = 2
  const sfpH = Math.min((guideSize - sfpGap) / 2, height * 0.4)
  const pad = shelf ? 4 : 6
  const bankGap = 6
  const packGap = shelf ? 3 : 4
  const guide = { y: (height - guideSize) / 2, size: guideSize }
  const badgeSize = shelf ? Math.min(14, height - 8) : 22

  const hasScreen = product.blocks.some((b) => b.kind === 'screen')
  const hasBadge = product.blocks.some((b) => b.kind === 'badge')
  const pack = !hasScreen
  const windowW = product.window_in ? Math.min(chassisW, product.window_in * px) : 0
  const window = windowW ? { x: chassisX + (chassisW - windowW) / 2, width: windowW } : undefined

  let screen: ResolvedFaceplate['screen']
  let badge: ResolvedFaceplate['badge']
  if (hasScreen) {
    screen = { x: (window?.x ?? chassisX) + pad, y: guide.y, size: Math.min(screenSize, guideSize) }
  }
  if (hasBadge) {
    badge = {
      x: (window?.x ?? chassisX) + pad,
      y: (height - badgeSize) / 2,
      w: badgeSize,
      h: badgeSize,
    }
  }

  const jacks: PlacedJack[] = []
  const keystones: ResolvedFaceplate['keystones'] = []

  let cursor = chassisX + pad
  if (window) cursor = window.x + pad
  if (screen) cursor = screen.x + screen.size + pad * 2
  // Shelf badges sit in the pre-port margin; don't reserve width until after jacks land.
  if (badge && !shelf) cursor = badge.x + badge.w + pad

  const trailer = product.blocks.some((b) => b.kind === 'trailer') ? sfpW : 0
  const sfpBlock = product.blocks.find((b) => b.kind === 'sfp')
  const sfpCols = sfpBlock && sfpBlock.kind === 'sfp' ? sfpBlock.cols : 0
  const sfpColGap = 3
  const sfpWidth = sfpCols ? sfpCols * sfpW + (sfpCols - 1) * sfpColGap : 0
  const bayRight = window ? window.x + window.width : chassisX + chassisW
  const sfpLead =
    sfpBlock && sfpBlock.kind === 'sfp' && typeof sfpBlock.gap === 'number' ? sfpBlock.gap : pad
  let sfpX0 = bayRight - pad - trailer - sfpWidth
  const copperEnd = sfpWidth ? sfpX0 - sfpLead : bayRight - pad - trailer
  let packFit = 1

  const rjBlocks = product.blocks.filter((b) => b.kind === 'rj45')

  if (window) {
    const ids = rjBlocks.flatMap((b) => (b.kind === 'rj45' ? expandPorts(b.ports) : []))
    const copperW = ids.length * (rjW + packGap)
    const groupW = (hasBadge ? badgeSize + pad : 0) + copperW
    const groupX = window.x + Math.max(pad, (window.width - groupW) / 2)
    if (badge) badge = { ...badge, x: groupX }
    let x = groupX + (hasBadge ? badgeSize + pad : 0)
    const rail = (rjBlocks[0] && rjBlocks[0].kind === 'rj45' ? rjBlocks[0].rail : undefined) ?? 'center'
    const numbers = rjBlocks[0] && rjBlocks[0].kind === 'rj45' ? (rjBlocks[0].numbers ?? 'above') : 'above'
    for (const id of ids) {
      jacks.push({
        id,
        kind: 'rj45',
        rail,
        numbers,
        x: x + packGap / 2,
        y: railY(rail, rjH, height, guide),
        w: rjW,
        h: rjH,
      })
      x += rjW + packGap
    }
  } else if (pack) {
    type Row = {
      ids: string[]
      bank: number
      rail: Rail
      order?: 'columns' | 'rows'
      odds?: 'high' | 'low'
      numbers: NonNullable<PlacedJack['numbers']>
      gap: number
      lead: number
    }
    const rows: Row[] = []
    for (let i = 0; i < rjBlocks.length; i++) {
      const block = rjBlocks[i]
      if (block.kind !== 'rj45') continue
      const ids = expandPorts(block.ports)
      rows.push({
        ids,
        bank: block.bank && block.bank > 0 ? block.bank : ids.length,
        rail: block.rail ?? 'low',
        order: block.order,
        odds: block.odds,
        numbers: block.numbers ?? (shelf ? 'below' : 'above'),
        gap: block.gap ?? packGap,
        lead: typeof block.lead === 'number' ? block.lead : i === 0 ? 0 : shelf ? 14 : 10,
      })
    }
    const copperNat = rows.reduce((n, r) => {
      const banks = chunk(r.ids, r.bank)
      const slots =
        r.order === 'columns'
          ? banks.reduce((c, b) => c + Math.ceil(b.length / 2), 0)
          : r.ids.length
      return n + r.lead + slots * (rjW + r.gap) + Math.max(0, banks.length - 1) * bankGap
    }, 0)
    const groupNat = copperNat + (sfpWidth ? sfpLead + sfpWidth : 0)
    const usable = Math.max(0, bayRight - pad - trailer - cursor)
    const fit = groupNat > 0 ? Math.min(1, usable / groupNat) : 1
    packFit = fit
    const jW = rjW * fit
    const jH = rjH * fit
    let x = cursor
    if (shelf) x = cursor + Math.max(0, (usable - groupNat * fit) / 2)
    else if (fit >= 1) x = Math.max(cursor, copperEnd - copperNat)
    for (const row of rows) {
      x += row.lead * fit
      const banks = chunk(row.ids, row.bank)
      const slot = jW + row.gap * fit
      for (let bi = 0; bi < banks.length; bi++) {
        if (row.order === 'columns') {
          const bankIds = banks[bi]
          const cols = Math.ceil(bankIds.length / 2)
          for (let ci = 0; ci < cols; ci++) {
            const pair = [bankIds[ci * 2], bankIds[ci * 2 + 1]].filter(Boolean) as string[]
            for (const id of pair) {
              const { rail, numbers } = rj45ColumnRail(id, row.odds)
              jacks.push({
                id,
                kind: 'rj45',
                rail,
                numbers,
                x: x + (slot - jW) / 2,
                y: railY(rail, jH, height, guide),
                w: jW,
                h: jH,
              })
            }
            x += slot
          }
        } else {
          for (const id of banks[bi]) {
            jacks.push({
              id,
              kind: 'rj45',
              rail: row.rail,
              numbers: row.numbers,
              x: x + (slot - jW) / 2,
              y: railY(row.rail, jH, height, guide),
              w: jW,
              h: jH,
            })
            x += slot
          }
        }
        if (bi < banks.length - 1) x += bankGap * fit
      }
    }
    if (shelf && sfpWidth) sfpX0 = x + sfpLead * fit
  } else {
    for (const block of rjBlocks) {
      if (block.kind !== 'rj45') continue
      const ids = expandPorts(block.ports)
      const bank = block.bank && block.bank > 0 ? block.bank : ids.length
      const banks = chunk(ids, bank)
      const portGap = block.gap ?? 0
      const colMajor = block.order === 'columns'
      const slots = colMajor ? banks.reduce((c, b) => c + Math.ceil(b.length / 2), 0) : ids.length
      const withinGaps = colMajor ? 0 : Math.max(0, ids.length - banks.length)
      const usable =
        copperEnd - cursor - Math.max(0, banks.length - 1) * bankGap - withinGaps * portGap
      const slot = slots > 0 ? usable / slots : rjW
      const scale = Math.min(1, slot / rjW)
      const jW = rjW * scale
      const jH = rjH * scale
      let x = cursor
      for (let bi = 0; bi < banks.length; bi++) {
        if (colMajor) {
          const bankIds = banks[bi]
          const cols = Math.ceil(bankIds.length / 2)
          for (let ci = 0; ci < cols; ci++) {
            const pair = [bankIds[ci * 2], bankIds[ci * 2 + 1]].filter(Boolean) as string[]
            for (const id of pair) {
              const { rail, numbers } = rj45ColumnRail(id, block.odds)
              jacks.push({
                id,
                kind: 'rj45',
                rail,
                numbers,
                x: x + (slot - jW) / 2,
                y: railY(rail, jH, height, guide),
                w: jW,
                h: jH,
              })
            }
            x += slot
          }
        } else {
          for (let pi = 0; pi < banks[bi].length; pi++) {
            const id = banks[bi][pi]
            const rail = block.rail ?? 'low'
            jacks.push({
              id,
              kind: 'rj45',
              rail,
              numbers: block.numbers ?? 'below',
              x: x + (slot - jW) / 2,
              y: railY(rail, jH, height, guide),
              w: jW,
              h: jH,
            })
            x += slot
            if (pi < banks[bi].length - 1) x += portGap
          }
        }
        if (bi < banks.length - 1) x += bankGap
      }
      cursor = copperEnd
    }
  }

  if (sfpBlock && sfpBlock.kind === 'sfp') {
    const ids = sfpBlock.ports.map(String)
    const cols = sfpBlock.cols
    const rows = Math.ceil(ids.length / cols)
    const colMajor = sfpBlock.order !== 'rows'
    const single = rows === 1
    const jackW = sfpW * (shelf ? packFit : 1)
    const jackH = sfpH * (shelf ? packFit : 1)
    for (let i = 0; i < ids.length; i++) {
      const col = colMajor ? Math.floor(i / rows) : i % cols
      const row = colMajor ? i % rows : Math.floor(i / cols)
      const rail: Rail = single ? (sfpBlock.rail ?? 'center') : row === 0 ? 'high' : 'low'
      jacks.push({
        id: ids[i],
        kind: 'sfp',
        rail,
        numbers: single
          ? (sfpBlock.numbers?.high ?? sfpBlock.numbers?.low ?? 'above')
          : rail === 'high'
            ? (sfpBlock.numbers?.high ?? 'above')
            : (sfpBlock.numbers?.low ?? 'below'),
        x: sfpX0 + col * (jackW + sfpColGap),
        y: single ? railY(rail, jackH, height, guide) : guide.y + row * (sfpH + sfpGap),
        w: jackW,
        h: jackH,
      })
    }
  }

  for (const block of product.blocks) {
    if (block.kind !== 'keystone' || !window) continue
    const wing =
      block.side === 'left'
        ? { x: chassisX + pad, w: window.x - chassisX - pad * 2 }
        : { x: window.x + window.width + pad, w: chassisX + chassisW - (window.x + window.width) - pad * 2 }
    if (wing.w <= 0 || block.slots <= 0) continue
    const slot = wing.w / block.slots
    const y = railY(block.rail, ksW, height, guide)
    for (let i = 0; i < block.slots; i++) {
      keystones.push({
        x: wing.x + i * slot + (slot - ksW) / 2,
        y,
        w: ksW,
        h: ksW,
      })
    }
  }

  if (shelf && badge && jacks.length > 0) {
    const first = Math.min(...jacks.map((j) => j.x))
    const left = chassisX + pad
    const right = first - pad
    const room = right - left
    if (room >= badge.w) {
      badge = { ...badge, x: left + (room - badge.w) / 2 }
    } else if (room > 4) {
      const size = Math.min(badge.w, room)
      badge = {
        x: left + (room - size) / 2,
        y: (height - size) / 2,
        w: size,
        h: size,
      }
    }
  }

  return { def: product, width, height, chassis, extension, window, screen, badge, jacks, keystones }
}

/** Dual-rail RJ45 columns. Default: odd → low (UniFi). odds:high → odd on top (TP-Link PE). */
function rj45ColumnRail(id: string, odds: 'high' | 'low' = 'low'): { rail: Rail; numbers: NumberSide } {
  const n = Number(id)
  const odd = Number.isFinite(n) ? n % 2 === 1 : true
  if (odds === 'high') {
    return odd ? { rail: 'high', numbers: 'above' } : { rail: 'low', numbers: 'below' }
  }
  return odd ? { rail: 'low', numbers: 'below' } : { rail: 'high', numbers: 'above' }
}

function railY(rail: Rail, jackH: number, height: number, guide: { y: number; size: number }): number {
  if (rail === 'center') return (height - jackH) / 2
  if (rail === 'high') return guide.y
  return guide.y + guide.size - jackH
}

function chunk<T>(xs: T[], n: number): T[][] {
  const out: T[][] = []
  for (let i = 0; i < xs.length; i += n) out.push(xs.slice(i, i + n))
  return out
}
