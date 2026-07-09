/**
 * copyText copies to the clipboard and reports success instead of
 * throwing. navigator.clipboard only exists in secure contexts
 * (https / localhost) — self-hosted panels are routinely reached over
 * plain http://IP, where the API is undefined and the old call threw
 * a TypeError that nothing caught. Falls back to the legacy
 * textarea + execCommand path before giving up.
 */
export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Permission denied or transient failure — try the fallback.
    }
  }
  try {
    const area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(area)
    return ok
  } catch {
    return false
  }
}
