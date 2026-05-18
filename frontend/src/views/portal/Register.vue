<script setup lang="ts">
import AuthLayout from '@/components/layout/AuthLayout.vue'
</script>

<template>
  <AuthLayout>
    <h1 class="mb-6 text-xl font-semibold">{{ $t('nav.portal') }} · {{ $t('auth.register') }}</h1>
    <p class="text-sm text-surface-500">
      User registration form lands in task 15.1.
    </p>
  </AuthLayout>
</template>
