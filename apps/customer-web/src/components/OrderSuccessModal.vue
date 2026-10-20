<script setup lang="ts">
// OrderSuccessModal shows a celebratory, animated "Order Confirmed" overlay
// after a successful payment/placement — the kind of confirmation moment
// Myntra/Flipkart show. The green circle pops in, the checkmark draws itself,
// and a few confetti dots burst out. It is purely presentational; the parent
// decides when to show it and where "View order" navigates.
import { onMounted, onBeforeUnmount } from 'vue'

const props = withDefaults(defineProps<{
  show: boolean
  orderNumber?: string
  amount?: string
  /** Auto-dismiss (navigate) after this many ms. 0 disables auto-dismiss. */
  autoDismissMs?: number
}>(), {
  orderNumber: '',
  amount: '',
  autoDismissMs: 3500,
})

const emit = defineEmits<{ (e: 'view'): void }>()

let timer: ReturnType<typeof setTimeout> | null = null

function clearTimer() {
  if (timer) { clearTimeout(timer); timer = null }
}

onMounted(() => {
  if (props.show && props.autoDismissMs > 0) {
    timer = setTimeout(() => emit('view'), props.autoDismissMs)
  }
})

onBeforeUnmount(clearTimer)

// A small, fixed confetti burst (deterministic positions so it reads clean).
const confetti = [
  { x: -70, y: -40, c: '#f59e0b', d: 0 },
  { x: 70, y: -40, c: '#10b981', d: 60 },
  { x: -90, y: 20, c: '#6366f1', d: 120 },
  { x: 90, y: 20, c: '#ec4899', d: 90 },
  { x: -40, y: -80, c: '#10b981', d: 150 },
  { x: 40, y: -80, c: '#f59e0b', d: 40 },
  { x: 0, y: -95, c: '#6366f1', d: 180 },
]
</script>

<template>
  <Transition name="os-fade">
    <div
      v-if="show"
      class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label="Order confirmed"
    >
      <div class="os-card relative w-full max-w-xs overflow-hidden rounded-2xl bg-white px-5 py-6 text-center shadow-2xl">
        <!-- Confetti burst -->
        <div class="pointer-events-none absolute left-1/2 top-[70px] h-0 w-0">
          <span
            v-for="(p, i) in confetti"
            :key="i"
            class="os-confetti absolute block h-2 w-2 rounded-sm"
            :style="{
              backgroundColor: p.c,
              '--tx': p.x + 'px',
              '--ty': p.y + 'px',
              animationDelay: p.d + 'ms',
            }"
          />
        </div>

        <!-- Animated success tick -->
        <div class="relative mx-auto mb-3 flex h-16 w-16 items-center justify-center">
          <span class="os-ring absolute inset-0 rounded-full bg-emerald-100" />
          <svg class="relative h-16 w-16" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle
              class="os-circle"
              cx="50" cy="50" r="44"
              stroke="#10b981" stroke-width="5" stroke-linecap="round"
            />
            <path
              class="os-check"
              d="M30 51 L44 65 L72 36"
              stroke="#10b981" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"
            />
          </svg>
        </div>

        <h2 class="text-base font-bold text-ink">Order Confirmed!</h2>
        <p class="mt-1 text-xs text-stone-500">
          Your payment was successful and your order is placed.
        </p>

        <div v-if="orderNumber || amount" class="mt-3 rounded-xl bg-stone-50 px-3 py-2.5 text-xs">
          <div v-if="orderNumber" class="flex items-center justify-between">
            <span class="text-stone-500">Order</span>
            <span class="font-semibold text-ink">{{ orderNumber }}</span>
          </div>
          <div v-if="amount" class="mt-1 flex items-center justify-between">
            <span class="text-stone-500">Paid</span>
            <span class="font-semibold text-ink">{{ amount }}</span>
          </div>
        </div>

        <button class="btn-primary btn-sm mt-4 w-full" @click="emit('view')">
          View order
        </button>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
/* Overlay fade */
.os-fade-enter-active { transition: opacity .25s ease; }
.os-fade-leave-active { transition: opacity .2s ease; }
.os-fade-enter-from,
.os-fade-leave-to { opacity: 0; }

/* Card pop-in */
.os-card {
  animation: os-pop .35s cubic-bezier(.16, 1, .3, 1);
}
@keyframes os-pop {
  from { transform: translateY(12px) scale(.94); opacity: 0; }
  to   { transform: translateY(0) scale(1); opacity: 1; }
}

/* Soft pulsing ring behind the tick */
.os-ring { animation: os-ring 1.6s ease-out .2s both; transform-origin: center; }
@keyframes os-ring {
  0%   { transform: scale(.4); opacity: 0; }
  40%  { opacity: .9; }
  100% { transform: scale(1); opacity: .5; }
}

/* Draw the circle, then the checkmark */
.os-circle {
  stroke-dasharray: 283;
  stroke-dashoffset: 283;
  animation: os-draw .5s ease-out .15s forwards;
}
.os-check {
  stroke-dasharray: 80;
  stroke-dashoffset: 80;
  animation: os-draw .35s ease-out .6s forwards;
}
@keyframes os-draw { to { stroke-dashoffset: 0; } }

/* Confetti burst outward then fade */
.os-confetti {
  opacity: 0;
  animation: os-burst .9s ease-out .55s forwards;
}
@keyframes os-burst {
  0%   { opacity: 1; transform: translate(0, 0) scale(.6) rotate(0deg); }
  100% { opacity: 0; transform: translate(var(--tx), var(--ty)) scale(1) rotate(160deg); }
}

@media (prefers-reduced-motion: reduce) {
  .os-card, .os-ring, .os-circle, .os-check, .os-confetti { animation: none; }
  .os-circle, .os-check { stroke-dashoffset: 0; }
  .os-confetti { opacity: 0; }
}
</style>
