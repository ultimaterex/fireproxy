export type VRackMaker = 'unifi' | 'firewalla' | 'tplink'
export type VRackType = 'switch' | 'ap' | 'box'
export type Rail = 'high' | 'center' | 'low'
export type NumberSide = 'above' | 'below'

export type Kit = {
  scale_px_per_in: number
  u: { width_in: number; height_in: number }
  screen: { diagonal_in: number }
  rj45: { width_in: number }
  sfp: { width_mm: number; height_mm: number }
  keystone: { width_in: number }
}

export type BlockDef =
  | { kind: 'screen' }
  | { kind: 'badge' }
  | { kind: 'trailer' }
  | { kind: 'keystone'; side: 'left' | 'right'; rail: Rail; slots: number }
  | {
      kind: 'rj45'
      rail?: Rail
      /** columns = dual-rail odd/even pairs; rows (default) = single rail L→R */
      order?: 'columns' | 'rows'
      /** Dual-rail: which rail holds odd ports. Default low (UniFi). TP-Link PE = high. */
      odds?: 'high' | 'low'
      bank?: number
      ports: string
      numbers?: NumberSide
      gap?: number
      /** Extra px before this bank (shelf: gap between 1–8 and PoE IN). */
      lead?: number
    }
  | {
      kind: 'sfp'
      cols: number
      order: 'columns' | 'rows'
      ports: Array<string | number>
      numbers?: { high?: NumberSide; low?: NumberSide }
      rail?: Rail
      gap?: number
    }

export type ProductDef = {
  manufacturer: VRackMaker
  type: VRackType
  product: string
  match: { model: string[] }
  ru: number
  form?: 'rack' | 'shelf'
  /** Chassis width in inches. Default = 19″. Narrower bodies get flush ears. */
  width_in?: number
  /** Front height in inches. Shelf units only; rack uses ru × 1.75″. */
  height_in?: number
  /** Chassis placement when narrower than 19″. Default center. */
  align?: 'left' | 'center' | 'right'
  /** Short ear on the non-extension side (inches). Used with align left/right. */
  ear_in?: number
  /** Center device cutout (manufacturer shelf). */
  window_in?: number
  blocks: BlockDef[]
  source: string
}

export type PlacedJack = {
  id: string
  kind: 'rj45' | 'sfp'
  rail: Rail
  numbers: NumberSide
  x: number
  y: number
  w: number
  h: number
}

export type ResolvedFaceplate = {
  def: ProductDef
  width: number
  height: number
  chassis: { x: number; width: number }
  /** Long blank bay filling the short side (Pro Max 16 right ear). */
  extension?: { x: number; width: number }
  window?: { x: number; width: number }
  screen?: { x: number; y: number; size: number }
  badge?: { x: number; y: number; w: number; h: number }
  jacks: PlacedJack[]
  keystones: { x: number; y: number; w: number; h: number }[]
}
