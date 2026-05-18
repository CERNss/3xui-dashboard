<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-primary-900 flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <!-- Brand -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-primary-600 text-white font-bold text-xl mb-4">
          3X
        </div>
        <h1 class="text-2xl font-bold text-white">3x-ui Dashboard</h1>
        <p class="text-gray-400 text-sm mt-1">Xray Proxy Management</p>
      </div>
      <slot />
    </div>
  </div>
</template>
