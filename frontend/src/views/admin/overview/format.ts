export function formatBytes(value: number): string {
  if (!value) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = Math.abs(value)
  let unit = 0

  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }

  const prefix = value < 0 ? '-' : ''
  return `${prefix}${unit === 0 ? size.toFixed(0) : size.toFixed(2)} ${units[unit]}`
}

export function formatYuan(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

export function formatDateTime(value?: string | null): string {
  return value ? new Date(value).toLocaleString() : '—'
}
