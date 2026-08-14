import { parse } from 'yaml'

import { parseKit, parseProduct } from '@/vrack/parse'
import type { Kit, ProductDef } from '@/vrack/types'

const files = import.meta.glob('./catalog/**/*.yaml', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

export function loadCatalog(): { kit: Kit; products: ProductDef[] } {
  let kit: Kit | undefined
  const products: ProductDef[] = []
  for (const [path, raw] of Object.entries(files)) {
    const data = parse(raw)
    if (
      path.endsWith('kit.yaml') &&
      !path.includes('/unifi/') &&
      !path.includes('/firewalla/') &&
      !path.includes('/tplink/')
    ) {
      kit = parseKit(data)
      continue
    }
    products.push(parseProduct(data, path))
  }
  if (!kit) throw new Error('vrack: catalog/kit.yaml missing')
  return { kit, products }
}
