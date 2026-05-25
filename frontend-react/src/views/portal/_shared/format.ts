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

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '∞'
  return new Date(value).toLocaleString()
}

export function trafficPercent(used: number, limit: number): number {
  if (limit <= 0) return 0
  return Math.min(100, Math.round((used / limit) * 1000) / 10)
}
