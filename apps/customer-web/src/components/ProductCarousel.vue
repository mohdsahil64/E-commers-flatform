<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import type { ListingCard } from '@carlitos/types'
import ProductCard from '@/components/ProductCard.vue'

const props = withDefaults(defineProps<{ products: ListingCard[]; loading?: boolean; skeletonCount?: number; addToCart?: boolean }>(), {
  loading: false,
  skeletonCount: 6,
  addToCart: false,
})

const track = ref<HTMLElement | null>(null)
const canLeft = ref(false)
const canRight = ref(false)

function updateArrows() {
  const el = track.value
  if (!el) return
  canLeft.value = el.scrollLeft > 4
  canRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 4
}

// Scroll by roughly one "page" (90% of the visible width) for a natural slide.
function scrollByPage(dir: 1 | -1) {
  const el = track.value
  if (!el) return
  el.scrollBy({ left: dir * Math.round(el.clientWidth * 0.9), behavior: 'smooth' })
}

onMounted(async () => {
  await nextTick()
  updateArrows()
  window.addEventListener('resize', updateArrows)
})
onUnmounted(() => window.removeEventListener('resize', updateArrows))
</script>

<template>
  <div class="relative">
    <!-- Left arrow (desktop) -->
    <button
      v-if="canLeft"
      type="button"
      class="absolute -left-3 top-1/2 z-10 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full border border-stone-200 bg-white text-ink shadow-md transition-colors hover:bg-stone-50 md:flex"
      aria-label="Scroll left"
      @click="scrollByPage(-1)"
    >
      <ChevronLeft :size="18" />
    </button>

    <!-- Right arrow (desktop) -->
    <button
      v-if="canRight"
      type="button"
      class="absolute -right-3 top-1/2 z-10 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full border border-stone-200 bg-white text-ink shadow-md transition-colors hover:bg-stone-50 md:flex"
      aria-label="Scroll right"
      @click="scrollByPage(1)"
    >
      <ChevronRight :size="18" />
    </button>

    <!-- Scroll track -->
    <div
      ref="track"
      class="carousel-track flex snap-x snap-mandatory gap-3 overflow-x-auto scroll-smooth pb-1"
      @scroll.passive="updateArrows"
    >
      <!-- Skeletons -->
      <template v-if="loading">
        <div
          v-for="i in skeletonCount"
          :key="'s' + i"
          class="carousel-item shrink-0 snap-start overflow-hidden rounded-lg border border-stone-200"
        >
          <div class="skeleton aspect-[3/4] w-full" />
          <div class="space-y-2 p-2.5">
            <div class="skeleton h-3 w-1/2 rounded" />
            <div class="skeleton h-3 w-3/4 rounded" />
            <div class="skeleton h-3 w-1/3 rounded" />
          </div>
        </div>
      </template>

      <!-- Products -->
      <template v-else>
        <div
          v-for="p in products"
          :key="p.product_id"
          class="carousel-item shrink-0 snap-start"
        >
          <ProductCard :product="p" compact :add-to-cart="props.addToCart" />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
/* Hide the scrollbar but keep scrolling/swipe. */
.carousel-track {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.carousel-track::-webkit-scrollbar {
  display: none;
}

/* Card width per breakpoint — smaller cards on larger screens so more show at
   once (2.2 visible on mobile → up to ~6 on wide desktop). */
.carousel-item {
  width: 44%;
}
@media (min-width: 480px) {
  .carousel-item { width: 30%; }
}
@media (min-width: 768px) {
  .carousel-item { width: 22%; }
}
@media (min-width: 1024px) {
  .carousel-item { width: 18%; }
}
@media (min-width: 1280px) {
  .carousel-item { width: 15.5%; }
}
</style>
