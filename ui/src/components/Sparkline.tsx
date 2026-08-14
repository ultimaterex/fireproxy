export function Sparkline({
  values,
  width = 160,
  height = 36,
  title,
}: {
  values: number[]
  width?: number
  height?: number
  title?: string
}) {
  if (values.length < 2) {
    return <span className="text-xs text-muted-foreground">—</span>
  }
  const max = Math.max(...values, 0.01)
  const min = Math.min(...values, 0)
  const span = max - min || 1
  const pts = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * (width - 2) + 1
      const y = height - 1 - ((v - min) / span) * (height - 2)
      return `${x},${y}`
    })
    .join(' ')
  return (
    <svg
      className="text-chart-2"
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={title}
    >
      {title ? <title>{title}</title> : null}
      <polyline fill="none" stroke="currentColor" strokeWidth="1.5" points={pts} />
    </svg>
  )
}
