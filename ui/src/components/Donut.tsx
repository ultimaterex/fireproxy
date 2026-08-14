export function Donut({
  value,
  total,
  size = 120,
}: {
  value: number
  total: number
  size?: number
}) {
  const pct = total > 0 ? value / total : 0
  const r = 42
  const c = 2 * Math.PI * r
  const dash = `${pct * c} ${c}`
  return (
    <svg width={size} height={size} viewBox="0 0 120 120" role="img" aria-label={`${Math.round(pct * 100)}%`}>
      <circle cx="60" cy="60" r={r} className="fill-none stroke-muted" strokeWidth="12" />
      <circle
        cx="60"
        cy="60"
        r={r}
        className="fill-none stroke-destructive"
        strokeWidth="12"
        strokeDasharray={dash}
        strokeLinecap="butt"
        transform="rotate(-90 60 60)"
      />
      <text
        x="60"
        y="64"
        textAnchor="middle"
        className="fill-foreground"
        fontSize="18"
        fontFamily="ui-monospace, monospace"
      >
        {`${Math.round(pct * 100)}%`}
      </text>
    </svg>
  )
}
