<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Status from './Status.vue'
import Stats from './Stats.vue'

const { t } = useI18n()

type Tab = 'status' | 'stats'
const tabs: Tab[] = ['status', 'stats']

const active = ref<Tab>('status')
const direction = ref<'forward' | 'back'>('forward')
const activePanelRef = ref<{ reload: () => void } | null>(null)
const activeIndex = computed(() => tabs.indexOf(active.value))
const activeComponent = computed(() => {
  if (active.value === 'status') return Status
  return Stats
})
const transitionName = computed(() =>
  direction.value === 'forward' ? 'overview-slide-forward' : 'overview-slide-back',
)
const tabThumbStyle = computed(() => ({
  width: `calc((100% - 0.5rem) / ${tabs.length})`,
  transform: `translateX(${activeIndex.value * 100}%)`,
}))

function selectTab(target: Tab) {
  if (target === active.value) return
  direction.value = tabs.indexOf(target) > activeIndex.value ? 'forward' : 'back'
  active.value = target
}

function tabLabel(target: Tab): string {
  if (target === 'status') return t('nav.status')
  return t('nav.stats')
}

function reloadActive() {
  activePanelRef.value?.reload()
}
</script>

<template>
  <div class="min-h-full">
    <section
      class="sticky top-0 z-20 mb-6 min-h-10 backdrop-blur"
      data-overview-toolbar
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <nav
          class="relative grid h-10 w-full max-w-full grid-cols-2 overflow-hidden rounded-full bg-surface-100 p-1 ring-1 ring-inset ring-surface-200 sm:w-[18rem] dark:bg-surface-900/80 dark:ring-surface-600"
          role="tablist"
          :aria-label="$t('section.overview')"
        >
          <span
            aria-hidden="true"
            class="absolute inset-y-1 left-1 rounded-full bg-ink-900 shadow-card transition-transform duration-300 ease-brand dark:bg-surface-50"
            :style="tabThumbStyle"
          />
          <button
            v-for="tab in tabs"
            :key="tab"
            type="button"
            role="tab"
            :aria-selected="active === tab"
            :class="[
              'relative z-10 flex h-8 min-w-0 items-center justify-center rounded-full px-3 text-sm font-semibold leading-none transition-colors duration-200 ease-brand',
              active === tab
                ? 'text-white dark:text-surface-950'
                : 'text-surface-600 hover:text-ink-900 dark:text-surface-200 dark:hover:text-surface-50',
            ]"
            @click="selectTab(tab)"
          >
            {{ tabLabel(tab) }}
          </button>
        </nav>
        <button
          type="button"
          class="inline-flex h-10 items-center gap-1.5 rounded-full border border-surface-200 bg-surface-0 px-3 text-sm text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100 dark:hover:bg-surface-700 dark:hover:text-white"
          @click="reloadActive"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
          {{ $t('admin.status.reload') }}
        </button>
      </div>
    </section>

    <section data-overview-content>
      <Transition :name="transitionName" mode="out-in">
        <KeepAlive>
          <component
            :is="activeComponent"
            :key="active"
            ref="activePanelRef"
          />
        </KeepAlive>
      </Transition>
    </section>
  </div>
</template>
