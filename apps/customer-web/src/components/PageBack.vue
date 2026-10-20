<script setup lang="ts">
import { ChevronLeft } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

// Reusable back control for inner pages.
//
// Default behaviour (home): single-step pages (listing, collections, support,
// legal, …) send the user straight to the fallback route — Home by default —
// regardless of browser history. These pages have no meaningful "previous step"
// of their own, so a predictable one-tap exit to Home is the expected UX.
//
// Set `:history="true"` for genuine multi-step flows (e.g. checkout) where the
// back control should walk the browser history one entry at a time instead.
const props = withDefaults(defineProps<{ label?: string; fallback?: string; history?: boolean }>(), {
  label: 'Back',
  fallback: '/',
  history: false,
})

const router = useRouter()

function goBack() {
  if (props.history) {
    // Multi-step flow: step back one history entry when there is one, otherwise
    // fall back to the sensible route.
    const back = window.history.state?.back
    if (typeof back === 'string' && back.length > 0) {
      router.back()
    } else {
      router.push(props.fallback)
    }
    return
  }
  // Single-step page: always go straight to the fallback (Home) — no history.
  router.push(props.fallback)
}
</script>

<template>
  <button
    type="button"
    class="mb-2 inline-flex items-center gap-0.5 text-xs text-stone-500 transition-colors hover:text-ink"
    @click="goBack"
  >
    <ChevronLeft :size="14" />
    <span>{{ label }}</span>
  </button>
</template>
