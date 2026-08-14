import { useEffect, useMemo, useRef, useState } from 'react'

import { fmtBytes, fmtCount } from '@/lib/format'
import { DOWN, UP } from '@/lib/brand'
import type { BytePoint } from '@/lib/types'

export function TransferChart({ points }: { points: BytePoint[] }) {
  const ref = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState({ w: 800, h: 220 })
  const [hover, setHover] = useState<number | null>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => {
      const { width, height } = e.contentRect
      if (width > 1 && height > 1) setBox({ w: width, h: height })
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const width = box.w
  const height = box.h
  const hasConn = points.some((p) => (p.conn ?? 0) > 0)
  const padL = 52
  const padR = hasConn ? 36 : 12
  const padT = 8
  const padB = 22
  const innerW = Math.max(1, width - padL - padR)
  const innerH = Math.max(1, height - padT - padB)

  const byteMax = useMemo(
    () => niceMax(Math.max(...points.flatMap((p) => [p.upload, p.download]), 1)),
    [points],
  )
  const barMax = useMemo(
    () => niceMax(Math.max(...points.map((p) => barValue(p, hasConn)), 1)),
    [points, hasConn],
  )

  if (points.length < 2) {
    return <p className="text-sm text-muted-foreground">Collecting…</p>
  }

  const step = innerW / points.length
  const x = (i: number) => padL + (i + 0.5) * step
  const yB = (v: number) => padT + innerH - (v / byteMax) * innerH
  const yBar = (v: number) => padT + innerH - (v / barMax) * innerH
  const barW = Math.min(18, Math.max(3, step * 0.72))

  const upPts = points.map((p, i) => ({ x: x(i), y: yB(p.upload) }))
  const downPts = points.map((p, i) => ({ x: x(i), y: yB(p.download) }))
  const ticks = [0, 0.2, 0.4, 0.6, 0.8, 1]
  const labelEvery = points.length > 48 ? 8 : 2
  const xLabels = points
    .map((p, i) => ({ i, p }))
    .filter(({ i }) => i % labelEvery === 0)

  const hi = hover != null ? Math.min(points.length - 1, Math.max(0, hover)) : null
  const hp = hi != null ? points[hi] : null

  return (
    <div ref={ref} className="relative size-full min-h-52">
      <svg
        className="block size-full"
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label="Upload, download, and activity last 24 hours"
        onMouseMove={(e) => {
          const r = e.currentTarget.getBoundingClientRect()
          const px = ((e.clientX - r.left) / r.width) * width
          setHover(Math.floor((px - padL) / step))
        }}
        onMouseLeave={() => setHover(null)}
      >
        {ticks.map((f) => {
          const yy = yB(byteMax * f)
          return (
            <g key={f}>
              <line
                x1={padL}
                x2={width - padR}
                y1={yy}
                y2={yy}
                className="stroke-border"
                strokeDasharray="4 4"
                strokeWidth="1"
              />
              <text
                x={padL - 8}
                y={yy + 3}
                textAnchor="end"
                className="fill-muted-foreground"
                fontSize="11"
              >
                {axisBytes(byteMax * f)}
              </text>
              {hasConn ? (
                <text
                  x={width - padR + 8}
                  y={yy + 3}
                  className="fill-muted-foreground"
                  fontSize="11"
                >
                  {axisCount(barMax * f)}
                </text>
              ) : null}
            </g>
          )
        })}
        {points.map((p, i) => {
          const v = barValue(p, hasConn)
          const top = yBar(v)
          return (
            <rect
              key={p.ts}
              x={x(i) - barW / 2}
              y={top}
              width={barW}
              height={Math.max(0, padT + innerH - top)}
              className="fill-foreground/12"
              rx="1"
            />
          )
        })}
        <path d={smoothPath(downPts)} fill="none" stroke={DOWN} strokeWidth="1.75" />
        <path d={smoothPath(upPts)} fill="none" stroke={UP} strokeWidth="1.75" />
        {hp && hi != null ? (
          <rect
            x={x(hi) - Math.max(barW, 8) / 2}
            y={padT}
            width={Math.max(barW, 8)}
            height={innerH}
            className="fill-foreground/8"
          />
        ) : null}
        {xLabels.map(({ i, p }) => (
          <text
            key={p.ts}
            x={x(i)}
            y={height - 4}
            textAnchor="middle"
            className="fill-muted-foreground"
            fontSize="11"
          >
            {new Date(p.ts * 1000).toLocaleTimeString([], { hour: 'numeric' })}
          </text>
        ))}
      </svg>
      {hp ? (
        <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-[11px] shadow-sm">
          <p className="text-muted-foreground">
            {new Date(hp.ts * 1000).toLocaleString([], { hour: 'numeric', minute: '2-digit' })}
          </p>
          <p style={{ color: UP }}>↑ {fmtBytes(hp.upload)}</p>
          <p style={{ color: DOWN }}>↓ {fmtBytes(hp.download)}</p>
          {hp.conn ? <p className="text-muted-foreground">{fmtCount(hp.conn)} flows</p> : null}
        </div>
      ) : null}
    </div>
  )
}

function barValue(p: BytePoint, hasConn: boolean): number {
  if (hasConn) return p.conn ?? 0
  return p.upload + p.download
}

function smoothPath(pts: { x: number; y: number }[], t = 0.1): string {
  if (pts.length < 2) return ''
  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i === 0 ? i : i - 1]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2] ?? p2
    const c1x = p1.x + (p2.x - p0.x) * t
    const c1y = p1.y + (p2.y - p0.y) * t
    const c2x = p2.x - (p3.x - p1.x) * t
    const c2y = p2.y - (p3.y - p1.y) * t
    d += ` C ${c1x.toFixed(1)} ${c1y.toFixed(1)} ${c2x.toFixed(1)} ${c2y.toFixed(1)} ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d
}

function niceMax(v: number): number {
  if (v <= 0) return 1
  const exp = 10 ** Math.floor(Math.log10(v))
  const n = v / exp
  const nice = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10
  return nice * exp
}

function axisBytes(v: number): string {
  if (v >= 1e12) return `${trim(v / 1e12)} TB`
  if (v >= 1e9) return `${trim(v / 1e9)} GB`
  if (v >= 1e6) return `${trim(v / 1e6)} MB`
  if (v >= 1e3) return `${trim(v / 1e3)} KB`
  return v === 0 ? '0 B' : `${Math.round(v)} B`
}

function axisCount(v: number): string {
  if (v >= 1e6) return `${trim(v / 1e6)} M`
  if (v >= 1e3) return `${trim(v / 1e3)} K`
  return v === 0 ? '0' : `${Math.round(v)}`
}

function trim(n: number): string {
  const s = n >= 10 ? n.toFixed(0) : n.toFixed(n >= 1 ? 0 : 1)
  return s.replace(/\.0$/, '')
}
