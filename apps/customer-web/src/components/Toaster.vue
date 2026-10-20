<script setup lang="ts">
import { CheckCircle, AlertCircle, Info, X } from 'lucide-vue-next'
import { useToastStore } from '@/stores/toast'

const toast = useToastStore()

const iconFor = (t: string) => (t === 'success' ? CheckCircle : t === 'error' ? AlertCircle : Info)
</script>

<template>
  <!-- Fixed toast stack. Sits above bottom nav on mobile, top-right on desktop. -->
  <div
    class="pointer-events-none fixed inset-x-0 bottom-20 z-[60] flex flex-col items-center gap-2 px-4 sm:inset-x-auto sm:right-6 sm:top-6 sm:bottom-auto sm:items-end"
    role="status"
    aria-live="polite"
  >
    <transition-group name="toast">
      <div
        v-for="t in toast.toasts"
        :key="t.id"
        class="pointer-events-auto flex w-full max-w-sm items-start gap-2.5 rounded-lg border px-3.5 py-2.5 shadow-md sm:w-auto sm:min-w-[240px]"
        :class="{
          'border-emerald-200 bg-emerald-50 text-emerald-800': t.type === 'success',
          'border-red-200 bg-red-50 text-red-700': t.type === 'error',
          'border-stone-200 bg-white text-ink': t.type === 'info',
        }"
      >
        <component :is="iconFor(t.type)" :size="17" class="mt-0.5 shrink-0" />
        <p class="flex-1 text-sm leading-snug">{{ t.message }}</p>
        <button
          class="shrink-0 text-current/60 transition-opacity hover:opacity-70"
          aria-label="Dismiss"
          @click="toast.dismiss(t.id)"
        >
          <X :size="15" />
        </button>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
