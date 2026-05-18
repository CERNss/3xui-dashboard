<script setup lang="ts">
import AuthLayout from '@/components/layout/AuthLayout.vue'
</script>

<template>
  <AuthLayout>
    <h1 class="mb-6 text-xl font-semibold">{{ $t('nav.admin') }} · {{ $t('auth.login') }}</h1>
    <p class="text-sm text-surface-500">
      Admin login form lands in task 14.1.
    </p>
  </AuthLayout>
</template>
