// Local randomizers used when the operator clicks "Add account" or
// similar. Browser-side only; backend doesn't depend on these.

const ALNUM_LOWER = 'abcdefghijklmnopqrstuvwxyz0123456789'

export function randomLowerAlnum(length: number): string {
  const buf = new Uint8Array(length)
  crypto.getRandomValues(buf)
  let out = ''
  for (let i = 0; i < length; i += 1) {
    out += ALNUM_LOWER[buf[i] % ALNUM_LOWER.length]
  }
  return out
}
