export interface BarsPanelProps {
  values: number[]
  ariaLabel: string
  color?: string
}

function clamp(value: number) {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
}

export function BarsPanel({ values, ariaLabel, color = '#cbd5e1' }: BarsPanelProps) {
  const bars = values.length > 0 ? values : [28, 44, 22, 52, 36]
  const width = 120 / bars.length

  return (
    <svg aria-label={ariaLabel} role="img" viewBox="0 0 120 48" preserveAspectRatio="none" style={{ height: 64, width: '100%' }}>
      {bars.map((value, index) => {
        const height = 6 + clamp(value) * 0.38
        return (
          <rect
            key={index}
            x={index * width + 2}
            y={48 - height}
            width={Math.max(4, width - 4)}
            height={height}
            rx="1.5"
            fill={color}
            opacity="0.72"
          />
        )
      })}
    </svg>
  )
}
