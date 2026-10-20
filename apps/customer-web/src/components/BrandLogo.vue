<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { BRAND } from '@carlitos/shared'

// Reusable CARLITOS brand mark: gold crest image + gold "CARLITOS" wordmark.
// `size` controls height; `wordmark` toggles the text; `to` makes it a link.
withDefaults(defineProps<{ size?: 'sm' | 'md' | 'lg'; to?: string | null; wordmark?: boolean }>(), {
  size: 'md',
  to: '/',
  wordmark: true,
})

const crest: Record<string, string> = {
  sm: 'h-8',
  md: 'h-10',
  lg: 'h-16',
}
const word: Record<string, string> = {
  sm: 'text-base',
  md: 'text-xl',
  lg: 'text-3xl',
}
</script>

<template>
  <component
    :is="to ? RouterLink : 'div'"
    :to="to || undefined"
    class="inline-flex items-center gap-2 transition-opacity hover:opacity-85"
    :aria-label="BRAND"
  >
    <img
      src="/carlitos-logo.png"
      :alt="BRAND"
      :class="crest[size]"
      class="w-auto object-contain"
      draggable="false"
    />
    <span
      v-if="wordmark"
      class="brand-wordmark font-semibold tracking-[0.18em]"
      :class="word[size]"
    >{{ BRAND }}</span>
  </component>
</template>

<style scoped>
/* Premium gold gradient wordmark. */
.brand-wordmark {
  background: linear-gradient(180deg, #e6c661 0%, #c9a227 45%, #a67c1a 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: #c9a227; /* fallback */
}
</style>
