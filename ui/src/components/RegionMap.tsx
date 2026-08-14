import { geoCentroid, geoEqualEarth, geoPath, type GeoPermissibleObjects } from 'd3-geo'
import { useEffect, useMemo, useState } from 'react'
import { feature } from 'topojson-client'
import type { Feature, FeatureCollection, Geometry } from 'geojson'
import type { Topology } from 'topojson-specification'

import { fmtBytes } from '@/lib/format'
import { iso2FromAtlasId } from '@/lib/iso-numeric'
import type { RankedFlow } from '@/lib/types'

const GEO_URL = 'https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json'
const W = 800
const H = 360
const ZOOM = 1.22
const ACTIVE = '#027BFF'
const IDLE = 'color-mix(in oklch, var(--muted-foreground) 38%, var(--card))'

type CountryFeat = Feature<Geometry, { name?: string }>

let load: Promise<CountryFeat[]> | null = null

function countries(): Promise<CountryFeat[]> {
  load ??= fetch(GEO_URL)
    .then((r) => {
      if (!r.ok) throw new Error('map')
      return r.json()
    })
    .then((topo: Topology) => {
      const obj = topo.objects.countries
      const fc = feature(topo, obj) as FeatureCollection<Geometry, { name?: string }>
      return fc.features.filter((f) => {
        const cc = iso2FromAtlasId(f.id as string | number)
        const name = (f.properties.name ?? '').toLowerCase()
        if (cc === 'AQ' || name.includes('antarct')) return false
        return geoCentroid(f)[1] > -60
      })
    })
  return load
}

export function RegionMap({
  regions,
  onSelectRegion,
}: {
  regions: RankedFlow[]
  onSelectRegion?: (cc: string, label: string) => void
}) {
  const [feats, setFeats] = useState<CountryFeat[] | null>(null)
  const [hover, setHover] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    countries()
      .then((f) => {
        if (live) setFeats(f)
      })
      .catch(() => {
        if (live) setFeats([])
      })
    return () => {
      live = false
    }
  }, [])

  const byCC = useMemo(() => {
    const m = new Map<string, RankedFlow>()
    for (const r of regions) {
      const cc = (r.country || r.id || '').toUpperCase()
      if (cc.length === 2) m.set(cc, r)
    }
    return m
  }, [regions])

  const max = useMemo(
    () => Math.max(1, ...[...byCC.values()].map((r) => bytesOf(r))),
    [byCC],
  )

  const paths = useMemo(() => {
    if (!feats?.length) return []
    const proj = geoEqualEarth().fitExtent(
      [
        [0, 0],
        [W, H],
      ],
      { type: 'FeatureCollection', features: feats },
    )
    const [tx, ty] = proj.translate()
    proj.scale(proj.scale() * ZOOM).translate([tx, ty + H * 0.04])
    const path = geoPath(proj)
    return feats.map((f) => ({
      key: String(f.id ?? f.properties.name),
      cc: iso2FromAtlasId(f.id as string | number),
      name: f.properties.name ?? '',
      d: path(f as GeoPermissibleObjects) ?? '',
    }))
  }, [feats])

  const tip = hover ? byCC.get(hover) : undefined
  const tipName = hover
    ? (tip?.name || paths.find((p) => p.cc === hover)?.name || hover)
    : null

  if (feats && feats.length === 0) return null
  if (!feats) {
    return <div className="h-56" />
  }

  return (
    <div className="relative overflow-hidden">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="block h-56 w-full"
        role="img"
        aria-label="Top regions map"
      >
        {paths.map((p) => {
          const row = p.cc ? byCC.get(p.cc) : undefined
          const bytes = row ? bytesOf(row) : 0
          const on = bytes > 0
          const t = on ? 0.55 + 0.45 * Math.sqrt(bytes / max) : 0
          return (
            <path
              key={p.key}
              d={p.d}
              fill={on ? mix('#3f3f46', ACTIVE, t) : IDLE}
              stroke="var(--card)"
              strokeWidth={0.8}
              className={on && onSelectRegion ? 'cursor-pointer' : undefined}
              onMouseEnter={() => p.cc && setHover(p.cc)}
              onMouseLeave={() => setHover(null)}
              onClick={() => {
                if (!on || !p.cc || !onSelectRegion) return
                onSelectRegion(p.cc, row?.name || p.name || p.cc)
              }}
            />
          )
        })}
      </svg>
      {tipName ? (
        <div className="pointer-events-none absolute bottom-2 left-2 rounded-md border bg-popover/95 px-2 py-1 text-[11px] shadow-sm">
          <p className="font-medium">{tipName}</p>
          {tip ? (
            <p className="font-mono text-muted-foreground">
              {fmtBytes(tip.upload)} ↑ · {fmtBytes(tip.download)} ↓
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function bytesOf(r: RankedFlow): number {
  return r.bytes || (r.upload ?? 0) + (r.download ?? 0)
}

function mix(a: string, b: string, t: number): string {
  const pa = hex(a)
  const pb = hex(b)
  const m = (i: number) => Math.round(pa[i] + (pb[i] - pa[i]) * t)
  return `rgb(${m(0)},${m(1)},${m(2)})`
}

function hex(c: string): [number, number, number] {
  const h = c.replace('#', '')
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
}
