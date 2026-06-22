export function formatBytes(value: number | null | undefined): string {
  const n = value ?? 0
  if (n <= 0) return '0 B'
  if (n < 1024) return `${n} B`

  const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let scaled = n / 1024
  let unitIndex = 0

  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024
    unitIndex += 1
  }

  return `${scaled.toFixed(2)} ${units[unitIndex]}`
}

export function formatYuan(cents: number | null | undefined): string {
  return `¥${((cents ?? 0) / 100).toFixed(2)}`
}

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n)
}

/** Date only, "YYYY-MM-DD" — matches the design mockups. */
export function formatDate(value: string | null | undefined): string {
  if (!value) return '∞'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '∞'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Date + time, "YYYY-MM-DD HH:mm:ss" — matches the design mockups
 * (deterministic, not locale-dependent like toLocaleString). */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '∞'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '∞'
  return `${formatDate(value)} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function trafficPercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.round((used / limit) * 1000) / 10)
}
