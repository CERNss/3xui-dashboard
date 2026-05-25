import { useEffect, useState } from 'react'

export function useMinWidth(width: number) {
  const [matches, setMatches] = useState(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return true
    return window.matchMedia(`(min-width: ${width}px)`).matches
  })

  useEffect(() => {
    const media = window.matchMedia(`(min-width: ${width}px)`)
    const handleChange = () => setMatches(media.matches)

    handleChange()
    media.addEventListener('change', handleChange)
    return () => media.removeEventListener('change', handleChange)
  }, [width])

  return matches
}
