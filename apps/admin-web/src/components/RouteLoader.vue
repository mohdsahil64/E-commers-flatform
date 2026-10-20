<script setup lang="ts">
/*
 * App loader — a SMALL centered spinner shown over the content whenever work
 * is in flight. Reflects REAL time (no fake duration):
 *  - Route switches (router.beforeEach → afterEach)
 *  - Button clicks / data fetches (global loader store, driven by every API call)
 * Light, semi-transparent backdrop so the data stays visible beneath.
 */
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useLoaderStore } from '@/stores/loader'

const router = useRouter()
const loader = useLoaderStore()

const navigating = ref(false)
let hideTimer: ReturnType<typeof setTimeout> | null = null
const visible = computed(() => navigating.value || loader.active)

function startNav() {
  if (hideTimer) { clearTimeout(hideTimer); hideTimer = null }
  navigating.value = true
}
function finishNav() {
  if (hideTimer) clearTimeout(hideTimer)
  hideTimer = setTimeout(() => { navigating.value = false }, 120)
}

let removeBefore: (() => void) | null = null
let removeAfter: (() => void) | null = null
let removeError: (() => void) | null = null

onMounted(() => {
  removeBefore = router.beforeEach((to, from) => {
    if (to.path !== from.path) startNav()
    return true
  })
  removeAfter = router.afterEach(() => { if (navigating.value) finishNav() })
  removeError = router.onError(() => finishNav())
})
onBeforeUnmount(() => {
  if (hideTimer) clearTimeout(hideTimer)
  removeBefore?.(); removeAfter?.(); removeError?.()
})
</script>

<template>
  <Transition name="rl-fade">
    <div v-if="visible" class="rl-overlay" role="status" aria-live="polite" aria-label="Loading">
      <span class="rl-spinner" />
    </div>
  </Transition>
</template>

<style scoped>
.rl-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.45);
  backdrop-filter: blur(1px);
}
.rl-spinner {
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  border: 3px solid rgba(255, 63, 108, 0.2);
  border-top-color: #ff3f6c;
  border-right-color: #ff3f6c;
  animation: rl-spin 0.65s linear infinite;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.08);
}
@keyframes rl-spin { to { transform: rotate(360deg); } }
.rl-fade-enter-active { transition: opacity 100ms ease; }
.rl-fade-leave-active { transition: opacity 160ms ease; }
.rl-fade-enter-from, .rl-fade-leave-to { opacity: 0; }
@media (prefers-reduced-motion: reduce) { .rl-spinner { animation-duration: 1.4s; } }
</style>
