<script setup lang="ts">
// Reusable CARLITOS logo. Import the asset through Vite so it is hashed and
// always bundled correctly regardless of the base path. Use anywhere:
//   <BrandLogo :height="32" />                      full logo
//   <BrandLogo :height="28" theme="light" label />  logo + "Seller Desk" caption
import logoUrl from '@/assets/carlitos-logo.png'
import { BRAND } from '@carlitos/shared'

withDefaults(defineProps<{
  height?: number          // rendered logo height in px
  label?: boolean          // show the "Seller Desk" caption next to the mark
  theme?: 'dark' | 'light' // caption color: dark text (default) or light (on dark bg)
}>(), {
  height: 32,
  label: false,
  theme: 'dark',
})
</script>

<template>
  <div class="inline-flex items-center gap-2.5 select-none">
    <img
      :src="logoUrl"
      :alt="BRAND"
      :style="{ height: height + 'px' }"
      class="w-auto object-contain"
      draggable="false"
    />
    <span
      v-if="label"
      :class="['text-sm font-medium', theme === 'light' ? 'text-stone-300' : 'text-stone-500']"
    >
      Seller Desk
    </span>
  </div>
</template>
