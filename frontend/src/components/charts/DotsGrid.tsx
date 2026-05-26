export interface DotsGridProps {
  values: number[]
  ariaLabel: string
  color?: string
}

function radius(value: number) {
  const safe = Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
  return 2.4 + safe * 0.035
}

export function DotsGrid({ values, ariaLabel, color = '#cbd5e1' }: DotsGridProps) {
  const dots = values.length > 0 ? values : [60, 28, 84, 38, 70, 28]

  return (
    <svg aria-label={ariaLabel} role="img" viewBox="0 0 120 48" style={{ height: 64, width: '100%' }}>
      {dots.map((value, index) => {
        const col = index % 6
        const row = Math.floor(index / 6)
        return (
          <circle
            key={index}
            cx={12 + col * 19}
            cy={18 + row * 16}
            r={radius(value)}
            fill={color}
            opacity={0.55 + Math.min(0.35, value / 220)}
          />
        )
      })}
    </svg>
  )
}
