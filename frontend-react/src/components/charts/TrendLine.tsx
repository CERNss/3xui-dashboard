export interface TrendLineSeries {
  points: number[]
  color?: string
  strokeWidth?: number
}

export interface TrendLineProps {
  series: TrendLineSeries[]
  ariaLabel: string
  height?: number
  showGrid?: boolean
}

function clamp(value: number) {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
}

function polyline(points: number[]) {
  if (points.length < 2) return ''
  return points
    .map((value, index) => {
      const x = (index / (points.length - 1)) * 120
      const y = 44 - clamp(value) * 0.4
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
}

export function TrendLine({ series, ariaLabel, height = 160, showGrid = true }: TrendLineProps) {
  const drawable = series.filter((item) => item.points.length >= 2)

  return (
    <svg
      aria-label={ariaLabel}
      role="img"
      viewBox="0 0 120 48"
      preserveAspectRatio="none"
      style={{ display: 'block', height, overflow: 'visible', width: '100%' }}
    >
      {showGrid
        ? [10, 20, 30, 40].map((y) => (
            <line key={y} x1="0" x2="120" y1={y} y2={y} stroke="currentColor" strokeOpacity={0.16} strokeWidth={0.4} />
          ))
        : null}
      {drawable.map((item, index) => (
        <polyline
          key={index}
          points={polyline(item.points)}
          fill="none"
          stroke={item.color ?? 'currentColor'}
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={item.strokeWidth ?? 2}
          vectorEffect="non-scaling-stroke"
        />
      ))}
    </svg>
  )
}
