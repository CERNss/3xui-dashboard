<script setup lang="ts">
withDefaults(defineProps<{
  bodyClass?: string
  footerClass?: string
  toolbarClass?: string
}>(), {
  bodyClass: '',
  footerClass: '',
  toolbarClass: '',
})
</script>

<template>
  <section class="flex h-[calc(100dvh-6rem)] min-w-0 flex-col gap-3 overflow-hidden md:h-[calc(100dvh-7.5rem)] lg:h-[calc(100dvh-8.5rem)]">
    <div
      class="shrink-0 rounded-2xl border border-white/80 bg-surface-0/92 p-3 shadow-[0_18px_45px_-34px_rgba(15,23,42,0.75),0_1px_2px_rgba(15,23,42,0.05)] backdrop-blur-xl dark:border-white/10 dark:bg-surface-900/88 dark:shadow-[0_18px_45px_-30px_rgba(0,0,0,0.65)]"
      :class="toolbarClass"
    >
      <slot name="toolbar" />
    </div>

    <div
      class="min-h-0 flex-1 overflow-hidden rounded-2xl border border-white/80 bg-surface-0/98 shadow-[0_18px_45px_-34px_rgba(15,23,42,0.7),0_1px_2px_rgba(15,23,42,0.05)] dark:border-white/10 dark:bg-surface-900/92 dark:shadow-[0_18px_45px_-30px_rgba(0,0,0,0.65)]"
      :class="bodyClass"
    >
      <slot />
    </div>

    <div
      v-if="$slots.footer"
      class="shrink-0 rounded-2xl border border-white/80 bg-surface-0/92 px-4 py-3 shadow-[0_18px_45px_-34px_rgba(15,23,42,0.75),0_1px_2px_rgba(15,23,42,0.05)] backdrop-blur-xl dark:border-white/10 dark:bg-surface-900/88 dark:shadow-[0_18px_45px_-30px_rgba(0,0,0,0.65)]"
      :class="footerClass"
    >
      <slot name="footer" />
    </div>
  </section>
</template>
