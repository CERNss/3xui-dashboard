import '@testing-library/jest-dom/vitest'

if (!window.localStorage) {
  const storage = new Map<string, string>()
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      clear: () => storage.clear(),
      getItem: (key: string) => storage.get(key) ?? null,
      key: (index: number) => [...storage.keys()][index] ?? null,
      removeItem: (key: string) => storage.delete(key),
      setItem: (key: string, value: string) => storage.set(key, value),
      get length() {
        return storage.size
      },
    },
  })
}

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
