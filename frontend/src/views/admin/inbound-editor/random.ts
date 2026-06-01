// Local randomizers used when the operator clicks "Add account" or
// similar. Browser-side only; backend doesn't depend on these.

const ALNUM_LOWER = 'abcdefghijklmnopqrstuvwxyz0123456789'

const REALITY_TARGETS = [
  'www.cloudflare.com:443',
  'www.amazon.com:443',
  'www.apple.com:443',
  'www.bing.com:443',
  'www.microsoft.com:443',
  'www.yahoo.com:443',
  'www.wikipedia.org:443',
  'www.speedtest.net:443',
]

function randomIndex(max: number): number {
  const buf = new Uint32Array(1)
  crypto.getRandomValues(buf)
  return buf[0] % max
}

export function randomLowerAlnum(length: number): string {
  const buf = new Uint8Array(length)
  crypto.getRandomValues(buf)
  let out = ''
  for (let i = 0; i < length; i += 1) {
    out += ALNUM_LOWER[buf[i] % ALNUM_LOWER.length]
  }
  return out
}

export function randomRealityTarget() {
  const target = REALITY_TARGETS[randomIndex(REALITY_TARGETS.length)]
  return {
    target,
    sni: target.split(':')[0],
  }
}

export function randomRealityShortIds(count = 8): string[] {
  const ids: string[] = []
  for (let i = 0; i < count; i += 1) {
    const length = Math.min(i + 1, 8)
    const buf = new Uint8Array(length)
    crypto.getRandomValues(buf)
    ids.push(Array.from(buf, (b) => b.toString(16).padStart(2, '0')).join(''))
  }
  return ids
}
