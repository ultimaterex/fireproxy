import { useEffect, useRef, useState } from 'react'

import { DOWN, UP } from '@/lib/brand'
import { fmtMbps } from '@/lib/format'
import { cn } from '@/lib/utils'

export type SparkPoint = { ts: number; down: number; up: number }

export function SpeedSpark({
  points,
  planDown = 0,
  planUp = 0,
  className,
}: {
  points: SparkPoint[]
  planDown?: number
  planUp?: number
  className?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState({ w: 320, h: 120 })
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

  const ready = points.length >= 2
  const padL = 18
  const padR = 8
  const padT = 10
  const padB = 16
  const innerW = Math.max(1, box.w - padL - padR)
  const innerH = Math.max(1, box.h - padT - padB)
  const downMax = Math.max(planDown, ...points.map((p) => p.down), 1)
  const upMax = Math.max(planUp, ...points.map((p) => p.up), 1)
  const barVals = points.map((p) => p.down + p.up)
  const barLo = barVals.length ? tightFloor(Math.min(...barVals)) : 0
  const barHi = barVals.length ? Math.max(...barVals, barLo + 1) : 1
  const step = innerW / Math.max(points.length, 1)
  const x = (i: number) => padL + (i + 0.5) * step
  const yDown = (v: number) => padT + innerH - (Math.min(Math.max(v, 0), downMax) / downMax) * innerH
  const yUp = (v: number) => padT + innerH - (Math.min(Math.max(v, 0), upMax) / upMax) * innerH
  const yBar = (v: number) => padT + innerH - ((Math.max(v, barLo) - barLo) / (barHi - barLo)) * innerH
  const barW = Math.min(18, Math.max(3, step * 0.72))
  const downPts = points.map((p, i) => ({ x: x(i), y: yDown(p.down) }))
  const upPts = points.map((p, i) => ({ x: x(i), y: yUp(p.up) }))
  const downStat = seriesStats(points.map((p) => p.down))
  const upStat = seriesStats(points.map((p) => p.up))
  const hi = hover != null && ready ? Math.min(points.length - 1, Math.max(0, hover)) : null
  const hp = hi != null ? points[hi] : null
  const dateTicks = sparseIndices(points.length, 4)

  return (
    <div ref={ref} className={cn('relative', className)}>
      {ready ? (
        <svg
          className="block size-full"
          viewBox={`0 0 ${box.w} ${box.h}`}
          role="img"
          aria-label="Speedtest down and up"
          onMouseMove={(e) => {
            const r = e.currentTarget.getBoundingClientRect()
            const px = ((e.clientX - r.left) / r.width) * box.w
            setHover(Math.floor((px - padL) / step))
          }}
          onMouseLeave={() => setHover(null)}
        >
          <line
            x1={padL}
            x2={box.w - padR}
            y1={padT + innerH}
            y2={padT + innerH}
            className="stroke-border"
            strokeWidth="1"
          />
          <line
            x1={padL}
            x2={box.w - padR}
            y1={padT}
            y2={padT}
            className="stroke-border"
            strokeWidth="1"
          />
          <text x={padL - 4} y={padT + innerH + 3} textAnchor="end" className="fill-muted-foreground" fontSize="10">
            0
          </text>
          {points.map((p, i) => {
            const top = yBar(p.down + p.up)
            return (
              <rect
                key={p.ts}
                x={x(i) - barW / 2}
                y={top}
                width={barW}
                height={Math.max(0, padT + innerH - top)}
                className="fill-muted-foreground/10"
                rx="1"
              />
            )
          })}
          <line
            x1={padL}
            x2={box.w - padR}
            y1={yDown(downStat.avg)}
            y2={yDown(downStat.avg)}
            stroke={DOWN}
            strokeOpacity="0.45"
            strokeWidth="1"
            strokeDasharray="4 3"
          />
          <line
            x1={padL}
            x2={box.w - padR}
            y1={yUp(upStat.avg)}
            y2={yUp(upStat.avg)}
            stroke={UP}
            strokeOpacity="0.45"
            strokeWidth="1"
            strokeDasharray="4 3"
          />
          <path d={areaPath(downPts, padT + innerH)} fill={DOWN} fillOpacity="0.1" />
          <path d={areaPath(upPts, padT + innerH)} fill={UP} fillOpacity="0.1" />
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
          {dateTicks.map((i) => (
            <text
              key={`d-${points[i].ts}`}
              x={x(i)}
              y={box.h - 3}
              textAnchor="middle"
              className="fill-muted-foreground"
              fontSize="10"
            >
              {fmtSparkDate(points[i].ts)}
            </text>
          ))}
        </svg>
      ) : null}
      {hp ? (
        <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-[11px] shadow-sm">
          <p className="text-muted-foreground">{fmtSparkDate(hp.ts)}</p>
          <p style={{ color: DOWN }}>↓ {fmtMbps(hp.down)}</p>
          <p style={{ color: UP }}>↑ {fmtMbps(hp.up)}</p>
        </div>
      ) : null}
    </div>
  )
}

export function speedTrend(points: SparkPoint[]): -1 | 0 | 1 {
  return seriesStats(points.map((p) => p.down)).trend
}

function seriesStats(values: number[]): { avg: number; y0: number; y1: number; trend: -1 | 0 | 1 } {
  const n = values.length
  if (n === 0) return { avg: 0, y0: 0, y1: 0, trend: 0 }
  const avg = values.reduce((a, b) => a + b, 0) / n
  if (n === 1) return { avg, y0: values[0], y1: values[0], trend: 0 }
  let sumX = 0
  let sumY = 0
  let sumXY = 0
  let sumXX = 0
  for (let i = 0; i < n; i++) {
    sumX += i
    sumY += values[i]
    sumXY += i * values[i]
    sumXX += i * i
  }
  const denom = n * sumXX - sumX * sumX
  const slope = denom === 0 ? 0 : (n * sumXY - sumX * sumY) / denom
  const intercept = (sumY - slope * sumX) / n
  const delta = slope * (n - 1)
  const trend: -1 | 0 | 1 = Math.abs(delta) < Math.max(avg * 0.05, 0.5) ? 0 : delta > 0 ? 1 : -1
  return { avg, y0: intercept, y1: intercept + slope * (n - 1), trend }
}

export function completeHourPoints<T extends { ts: number }>(points: T[], asOf: number): T[] {
  return points.filter((p) => p.ts + 3600 <= asOf)
}

export function DnsChart({
  points,
  color,
  className,
}: {
  points: { ts: number; count: number }[]
  color: string
  className?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState({ w: 320, h: 140 })
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

  const ready = points.length >= 2
  const padL = 36
  const padR = 8
  const padT = 8
  const padB = 18
  const innerW = Math.max(1, box.w - padL - padR)
  const innerH = Math.max(1, box.h - padT - padB)
  const counts = points.map((p) => p.count)
  const { min: yMin, max: yMax } = typicalRange(counts)
  const step = innerW / Math.max(points.length, 1)
  const x = (i: number) => padL + (i + 0.5) * step
  const y = (v: number) => padT + innerH - ((Math.max(v, yMin) - yMin) / (yMax - yMin)) * innerH
  const barW = Math.min(14, Math.max(2, step * 0.72))
  const linePts = points.map((p, i) => ({ x: x(i), y: y(p.count) }))
  const hi = hover != null && ready ? Math.min(points.length - 1, Math.max(0, hover)) : null
  const hp = hi != null ? points[hi] : null
  const labelEvery = points.length > 12 ? 6 : 3

  return (
    <div ref={ref} className={cn('relative', className)}>
      {ready ? (
        <svg
          className="block size-full"
          viewBox={`0 0 ${box.w} ${box.h}`}
          role="img"
          aria-label="DNS lookups"
          onMouseMove={(e) => {
            const r = e.currentTarget.getBoundingClientRect()
            const px = ((e.clientX - r.left) / r.width) * box.w
            setHover(Math.floor((px - padL) / step))
          }}
          onMouseLeave={() => setHover(null)}
        >
          {[yMin, yMax].map((v) => {
            const yy = y(v)
            return (
              <g key={v}>
                <line
                  x1={padL}
                  x2={box.w - padR}
                  y1={yy}
                  y2={yy}
                  className="stroke-border"
                  strokeWidth="1"
                />
                <text
                  x={padL - 6}
                  y={yy + 3}
                  textAnchor="end"
                  className="fill-muted-foreground"
                  fontSize="10"
                >
                  {fmtCountAxis(v)}
                </text>
              </g>
            )
          })}
          {points.map((p, i) => {
            const top = y(p.count)
            return (
              <rect
                key={p.ts}
                x={x(i) - barW / 2}
                y={top}
                width={barW}
                height={Math.max(0, padT + innerH - top)}
                className="fill-muted-foreground/10"
                rx="1"
              />
            )
          })}
          <path d={smoothPath(linePts)} fill="none" stroke={color} strokeWidth="1.75" />
          {hp && hi != null ? (
            <rect
              x={x(hi) - Math.max(barW, 8) / 2}
              y={padT}
              width={Math.max(barW, 8)}
              height={innerH}
              className="fill-foreground/8"
            />
          ) : null}
          {points.map((p, i) =>
            i % labelEvery === 0 ? (
              <text
                key={`t-${p.ts}`}
                x={x(i)}
                y={box.h - 4}
                textAnchor="middle"
                className="fill-muted-foreground"
                fontSize="10"
              >
                {new Date(p.ts * 1000).toLocaleTimeString([], { hour: 'numeric' })}
              </text>
            ) : null,
          )}
        </svg>
      ) : (
        <p className="text-sm text-muted-foreground">Collecting…</p>
      )}
      {hp ? (
        <div className="pointer-events-none absolute top-1 right-1 rounded-md border bg-popover px-2 py-1 text-[11px] shadow-sm">
          <p className="text-muted-foreground">
            {new Date(hp.ts * 1000).toLocaleString([], { hour: 'numeric', minute: '2-digit' })}
          </p>
          <p className="font-mono tabular-nums">{hp.count.toLocaleString()}</p>
        </div>
      ) : null}
    </div>
  )
}

function typicalRange(values: number[]): { min: number; max: number } {
  if (values.length === 0) return { min: 0, max: 1 }
  const max = Math.max(...values)
  const sorted = [...values].sort((a, b) => a - b)
  const mid = sorted[Math.floor(sorted.length / 2)]
  const typical = values.filter((v) => v >= mid * 0.6)
  const min = typical.length ? Math.min(...typical) : Math.min(...values)
  const lo = tightFloor(min)
  return { min: lo, max: Math.max(max, lo + 1) }
}

function tightFloor(min: number): number {
  if (!Number.isFinite(min) || min <= 0) return 0
  const exp = 10 ** Math.floor(Math.log10(min))
  const step = exp >= 100 ? exp / 2 : exp
  return Math.floor(min / step) * step
}

/** Up to `want` evenly spaced indices, always including first and last when n ≥ 2. */
function sparseIndices(n: number, want: number): number[] {
  if (n <= 0) return []
  if (n === 1) return [0]
  const count = Math.min(want, n)
  if (count === 2) return [0, n - 1]
  const out: number[] = []
  for (let k = 0; k < count; k++) {
    out.push(Math.round((k * (n - 1)) / (count - 1)))
  }
  return [...new Set(out)]
}

function fmtSparkDate(ts: number): string {
  return new Date(ts * 1000).toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function fmtCountAxis(v: number): string {
  if (v >= 1000) {
    const k = v / 1000
    const s = k >= 10 ? k.toFixed(0) : k.toFixed(k >= 1 && k % 1 < 0.05 ? 0 : 1)
    return `${s.replace(/\.0$/, '')}k`
  }
  return String(Math.round(v))
}

function areaPath(pts: { x: number; y: number }[], baseY: number): string {
  const line = smoothPath(pts)
  if (!line) return ''
  const last = pts[pts.length - 1]
  const first = pts[0]
  return `${line} L ${last.x.toFixed(1)} ${baseY.toFixed(1)} L ${first.x.toFixed(1)} ${baseY.toFixed(1)} Z`
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
