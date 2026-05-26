#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const repoRoot = path.resolve(new URL('../..', import.meta.url).pathname)
const vueRoot = path.join(repoRoot, 'frontend', 'src')
const reactRoot = path.join(repoRoot, 'frontend-react', 'src')

const exclusions = new Map([
  [
    'components/common/ConfirmModal.spec.ts',
    'React uses AntD Modal.confirm directly; there is no ConfirmModal component after P1.',
  ],
  [
    'components/common/ToastHost.spec.ts',
    'React uses AntD App/message instead of the removed Vue ToastHost component.',
  ],
  [
    'composables/useConfirm.spec.ts',
    'React removed useConfirm in favor of explicit AntD Modal.confirm callsites.',
  ],
])

const explicitMappings = new Map([
  ['components/common/AccountMenu.spec.ts', 'components/common/common.spec.tsx'],
  ['components/common/EmptyState.spec.ts', 'components/common/common.spec.tsx'],
  ['components/common/Skeleton.spec.ts', 'components/common/common.spec.tsx'],
  ['components/layout/AdminLayout.spec.ts', 'components/layout/layout.spec.tsx'],
  ['components/portal/AlipayPayModal.spec.ts', 'components/portal/AlipayPayModal.spec.tsx'],
  ['router/index.spec.ts', ['router.spec.tsx', 'components/ProtectedRoute.spec.tsx']],
  ['views/admin/InboundEditorModal.spec.ts', 'views/admin/InboundEditor.spec.tsx'],
  ['views/admin/Status.spec.ts', 'views/admin/Overview.spec.tsx'],
  ['views/admin/Stats.spec.ts', 'views/admin/Overview.spec.tsx'],
  ['views/admin/settings/DataCollectionSettings.spec.ts', 'views/admin/Settings.spec.tsx'],
])

function walk(dir) {
  if (!fs.existsSync(dir)) return []
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return walk(full)
    return entry.isFile() && /\.spec\.tsx?$/.test(entry.name) ? [full] : []
  })
}

function rel(root, file) {
  return path.relative(root, file).split(path.sep).join('/')
}

function itCount(file) {
  const text = fs.readFileSync(file, 'utf8')
  return (text.match(/\bit\s*(?:\.each\s*\([^)]*\)\s*)?\(/g) ?? []).length
}

function defaultReactPath(vueRel) {
  return vueRel.replace(/\.spec\.ts$/, '.spec.tsx')
}

const vueSpecs = walk(vueRoot).map((file) => rel(vueRoot, file)).sort()
const rows = []
const failures = []

for (const vueRel of vueSpecs) {
  if (exclusions.has(vueRel)) {
    rows.push({ vueRel, status: 'excluded', reason: exclusions.get(vueRel) })
    continue
  }

  const mapped = explicitMappings.get(vueRel) ?? defaultReactPath(vueRel)
  const reactRels = Array.isArray(mapped) ? mapped : [mapped]
  const missing = reactRels.filter((reactRel) => !fs.existsSync(path.join(reactRoot, reactRel)))

  if (missing.length) {
    failures.push(`missing React spec for ${vueRel}: ${missing.join(', ')}`)
    rows.push({ vueRel, reactRels, status: 'missing' })
    continue
  }

  const vueCount = itCount(path.join(vueRoot, vueRel))
  const reactCount = reactRels.reduce((sum, reactRel) => sum + itCount(path.join(reactRoot, reactRel)), 0)
  if (reactCount < vueCount) {
    failures.push(`${vueRel}: Vue has ${vueCount} it(...) blocks, React mapping has ${reactCount}`)
    rows.push({ vueRel, reactRels, status: 'short', vueCount, reactCount })
    continue
  }

  rows.push({ vueRel, reactRels, status: 'ok', vueCount, reactCount })
}

for (const row of rows) {
  if (row.status === 'excluded') {
    console.log(`EXCLUDE ${row.vueRel} - ${row.reason}`)
  } else {
    console.log(
      `${row.status.toUpperCase()} ${row.vueRel} -> ${row.reactRels.join(' + ')} (${row.vueCount ?? 0}/${row.reactCount ?? 0})`,
    )
  }
}

if (failures.length) {
  console.error('\nSpec parity failed:')
  for (const failure of failures) console.error(`- ${failure}`)
  process.exit(1)
}

console.log(`\nSpec parity passed for ${rows.filter((row) => row.status === 'ok').length} mapped specs.`)
