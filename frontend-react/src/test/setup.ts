import '@testing-library/jest-dom/vitest'

if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    }),
  })
}

const getComputedStyle = window.getComputedStyle.bind(window)
window.getComputedStyle = (element: Element, pseudoElt?: string | null): CSSStyleDeclaration => {
  if (pseudoElt) {
    return getComputedStyle(element)
  }
  return getComputedStyle(element, pseudoElt)
}
