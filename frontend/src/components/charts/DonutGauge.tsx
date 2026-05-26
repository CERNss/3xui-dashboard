export interface DonutGaugeProps {
  value: number
  label: string
  detail?: string
  tone?: 'live' | 'warn' | 'muted'
  ariaLabel?: string
}

const toneColors = {
  live: '#10b981',
  warn: '#f59e0b',
  muted: '#94a3b8',
}

function clampScore(value: number) {
  return Math.min(100, Math.max(0, Number.isFinite(value) ? value : 0))
}

export function DonutGauge({ value, label, detail, tone = 'live', ariaLabel }: DonutGaugeProps) {
  const score = clampScore(value)

  return (
    <div style={{ display: 'grid', placeItems: 'center', position: 'relative' }}>
      <svg
        aria-label={ariaLabel ?? label}
        role="img"
        viewBox="0 0 120 120"
        style={{ display: 'block', height: 176, transform: 'rotate(-90deg)', width: 176 }}
      >
        <circle cx="60" cy="60" r="50" fill="none" stroke="currentColor" strokeOpacity={0.14} strokeWidth="10" />
        <circle
          cx="60"
          cy="60"
          r="50"
          fill="none"
          pathLength="100"
          stroke={toneColors[tone]}
          strokeDasharray={`${score} 100`}
          strokeLinecap="round"
          strokeWidth="10"
        />
      </svg>
      <div style={{ inset: 0, display: 'grid', placeItems: 'center', position: 'absolute', textAlign: 'center' }}>
        <div>
          <div style={{ fontSize: 42, fontWeight: 650, lineHeight: 1 }}>{Math.round(score)}</div>
          <div style={{ color: '#64748b', fontSize: 12, fontWeight: 600, marginTop: 4 }}>{label}</div>
          {detail ? <div style={{ color: '#94a3b8', fontSize: 11, marginTop: 4 }}>{detail}</div> : null}
        </div>
      </div>
    </div>
  )
}
