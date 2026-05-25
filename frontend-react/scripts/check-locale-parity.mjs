import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { tmpdir } from 'node:os'

const root = resolve(dirname(new URL(import.meta.url).pathname), '../..')
const tmpRoot = resolve(tmpdir(), `3xui-locale-parity-${Date.now()}-${process.pid}`)

async function loadLocale(file, exportName) {
  const source = await readFile(resolve(root, file), 'utf8')
  const moduleSource = source
    .replace(/export\s+default/, `export const ${exportName} =`)
    .replace(/\s+as\s+const\s*$/, '')
  const tmpFile = resolve(tmpRoot, file.replaceAll('/', '__').replace(/\.ts$/, '.mjs'))

  await mkdir(dirname(tmpFile), { recursive: true })
  await writeFile(tmpFile, moduleSource, 'utf8')

  const mod = await import(pathToFileURL(tmpFile).href)
  return mod[exportName]
}

function flatten(value, prefix = '', out = new Map()) {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    for (const [key, child] of Object.entries(value)) {
      flatten(child, prefix ? `${prefix}.${key}` : key, out)
    }
    return out
  }

  out.set(prefix, value)
  return out
}

function compareLocale(name, vueLocale, reactLocale) {
  const vue = flatten(vueLocale)
  const react = flatten(reactLocale)
  const keys = [...new Set([...vue.keys(), ...react.keys()])].sort()
  const diffs = []

  for (const key of keys) {
    if (!vue.has(key)) {
      diffs.push(`${name}: only in React: ${key}`)
      continue
    }
    if (!react.has(key)) {
      diffs.push(`${name}: missing in React: ${key}`)
      continue
    }

    const vueValue = vue.get(key)
    const reactValue = react.get(key)
    if (typeof vueValue !== 'string' || typeof reactValue !== 'string') {
      if (vueValue !== reactValue) {
        diffs.push(`${name}: value differs: ${key}`)
      }
      continue
    }
    if (vueValue !== reactValue) {
      diffs.push(`${name}: value differs: ${key}`)
    }
  }

  return diffs
}

async function main() {
  try {
    const [vueZh, vueEn, reactZh, reactEn] = await Promise.all([
      loadLocale('frontend/src/i18n/locales/zh.ts', 'zh'),
      loadLocale('frontend/src/i18n/locales/en.ts', 'en'),
      loadLocale('frontend-react/src/i18n/locales/zh.ts', 'zh'),
      loadLocale('frontend-react/src/i18n/locales/en.ts', 'en'),
    ])

    const diffs = [
      ...compareLocale('zh', vueZh, reactZh),
      ...compareLocale('en', vueEn, reactEn),
    ]

    if (diffs.length > 0) {
      console.error(`Locale parity failed (${diffs.length} diff${diffs.length === 1 ? '' : 's'}):`)
      for (const diff of diffs) console.error(`- ${diff}`)
      process.exitCode = 1
      return
    }

    console.log('OK locale parity matches Vue locales')
  } finally {
    await rm(tmpRoot, { recursive: true, force: true })
  }
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
