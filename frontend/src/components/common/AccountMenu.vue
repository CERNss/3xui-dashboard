<script lang="ts">
import type { RouteLocationRaw } from 'vue-router'

export interface AccountMenuItem {
  label: string
  icon: string
  to?: RouteLocationRaw
  href?: string
}
</script>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId } from 'vue'

const props = defineProps<{
  name: string
  subtitle?: string
  roleLabel?: string
  avatarText?: string
  openLabel: string
  logoutLabel: string
  items: AccountMenuItem[]
}>()

const emit = defineEmits<{
  logout: []
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const menuId = useId()

const avatar = computed(() => {
  const raw = props.avatarText || props.name || 'A'
  return raw.slice(0, 2).toUpperCase()
})

function menuItems(): HTMLElement[] {
  if (!menu.value) return []
  return Array.from(menu.value.querySelectorAll<HTMLElement>('[role="menuitem"]'))
}

function focusItem(index: number) {
  const items = menuItems()
  if (!items.length) return
  const wrapped = (index + items.length) % items.length
  items[wrapped]?.focus()
}

function close(returnFocus = false) {
  if (!open.value) return
  open.value = false
  if (returnFocus) trigger.value?.focus()
}

async function openMenu(focusFirst = false) {
  open.value = true
  await nextTick()
  if (focusFirst) focusItem(0)
  else menu.value?.focus()
}

async function toggle() {
  if (open.value) close()
  else await openMenu(false)
}

function logout() {
  close(true)
  emit('logout')
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    void openMenu(true)
  }
}

function onMenuKeydown(event: KeyboardEvent) {
  const items = menuItems()
  if (!items.length) return
  const current = items.indexOf(document.activeElement as HTMLElement)
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      focusItem(current < 0 ? 0 : current + 1)
      break
    case 'ArrowUp':
      event.preventDefault()
      focusItem(current < 0 ? items.length - 1 : current - 1)
      break
    case 'Home':
      event.preventDefault()
      focusItem(0)
      break
    case 'End':
      event.preventDefault()
      focusItem(items.length - 1)
      break
    case 'Tab':
      close()
      break
  }
}

function onDocumentPointerDown(event: PointerEvent) {
  if (!open.value || !root.value) return
  if (!root.value.contains(event.target as Node)) close()
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
})
</script>

<template>
  <div ref="root" class="relative" @keydown.escape.stop.prevent="close(true)">
    <button
      ref="trigger"
      type="button"
      class="group inline-flex h-11 max-w-[220px] items-center gap-2 rounded-2xl border border-surface-200 bg-surface-0 px-2.5 text-left shadow-sm transition-all duration-150 ease-brand hover:border-surface-300 hover:bg-surface-50 active:scale-[0.98] dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700 dark:hover:bg-surface-800"
      :aria-label="openLabel"
      aria-haspopup="menu"
      :aria-controls="menuId"
      :aria-expanded="open"
      @click="toggle"
      @keydown="onTriggerKeydown"
    >
      <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-accent-500 text-xs font-semibold text-white shadow-card ring-1 ring-accent-700/20">
        {{ avatar }}
      </span>
      <span class="hidden min-w-0 leading-tight sm:block">
        <span class="block truncate text-sm font-semibold text-ink-900 dark:text-surface-50">{{ name }}</span>
        <span v-if="roleLabel" class="block truncate text-xs text-surface-500 dark:text-surface-400">{{ roleLabel }}</span>
      </span>
      <svg
        class="h-4 w-4 shrink-0 text-surface-400 transition-transform duration-150 ease-brand group-hover:text-surface-600 dark:group-hover:text-surface-200"
        :class="open ? 'rotate-180' : ''"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="m6 9 6 6 6-6" />
      </svg>
    </button>

    <div
      v-if="open"
      :id="menuId"
      ref="menu"
      class="absolute right-0 z-50 mt-2 w-[17rem] overflow-hidden rounded-2xl border border-surface-200 bg-surface-0 p-1.5 shadow-elevated outline-none ring-1 ring-ink-900/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-white/10"
      role="menu"
      tabindex="-1"
      @keydown="onMenuKeydown"
    >
      <div class="px-3 pb-3 pt-2.5">
        <div class="truncate text-sm font-semibold text-ink-900 dark:text-surface-50">{{ name }}</div>
        <div v-if="subtitle" class="mt-0.5 truncate text-xs text-surface-500 dark:text-surface-400">{{ subtitle }}</div>
      </div>

      <div class="h-px bg-surface-100 dark:bg-surface-800"></div>

      <div class="py-1.5">
        <template v-for="item in items" :key="item.label">
          <router-link
            v-if="item.to"
            :to="item.to"
            class="flex h-10 items-center gap-2.5 rounded-xl px-3 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-100 hover:text-ink-900 focus:bg-surface-100 focus:text-ink-900 focus:outline-none dark:text-surface-200 dark:hover:bg-surface-800 dark:hover:text-surface-50 dark:focus:bg-surface-800 dark:focus:text-surface-50"
            role="menuitem"
            @click="close()"
          >
            <svg class="h-4 w-4 shrink-0 text-surface-500 dark:text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <path :d="item.icon" />
            </svg>
            <span>{{ item.label }}</span>
          </router-link>

          <a
            v-else-if="item.href"
            :href="item.href"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 items-center gap-2.5 rounded-xl px-3 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-100 hover:text-ink-900 focus:bg-surface-100 focus:text-ink-900 focus:outline-none dark:text-surface-200 dark:hover:bg-surface-800 dark:hover:text-surface-50 dark:focus:bg-surface-800 dark:focus:text-surface-50"
            role="menuitem"
            @click="close()"
          >
            <svg class="h-4 w-4 shrink-0 text-surface-500 dark:text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <path :d="item.icon" />
            </svg>
            <span>{{ item.label }}</span>
          </a>
        </template>
      </div>

      <div class="h-px bg-surface-100 dark:bg-surface-800"></div>

      <button
        type="button"
        class="mt-1.5 flex h-10 w-full items-center gap-2.5 rounded-xl px-3 text-left text-sm font-semibold text-red-600 transition-colors hover:bg-red-50 focus:bg-red-50 focus:outline-none dark:text-red-300 dark:hover:bg-red-950/40 dark:focus:bg-red-950/40"
        role="menuitem"
        @click="logout"
      >
        <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M15 12H3M9 6l-6 6 6 6M21 4v16" />
        </svg>
        <span>{{ logoutLabel }}</span>
      </button>
    </div>
  </div>
</template>
