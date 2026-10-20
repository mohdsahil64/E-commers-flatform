<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ChevronLeft, ChevronRight, ArrowRight } from 'lucide-vue-next'
import type { Banner } from '@carlitos/types'
import { MEDIA_BASE } from '@/api'
import { resolveMediaUrl } from '@carlitos/shared'

const props = withDefaults(defineProps<{ banners: Banner[]; interval?: number }>(), {
  interval: 5000,
})

const router = useRouter()
const index = ref(0)
const paused = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

// Reactively track the mobile breakpoint so the slider swaps between the
// admin's mobile and desktop banner images when the viewport crosses 640px
// (e.g. rotating a device or toggling responsive mode), not just on mount.
const mq = typeof window !== 'undefined' ? window.matchMedia('(max-width: 640px)') : null
const isMobile = ref(mq ? mq.matches : false)
function onMqChange(e: MediaQueryListEvent) { isMobile.value = e.matches }

const current = computed(() => props.banners[index.value] || null)
const many = computed(() => props.banners.length > 1)

function go(i: number) {
  if (!props.banners.length) return
  index.value = (i + props.banners.length) % props.banners.length
}
function next() { go(index.value + 1) }
function prev() { go(index.value - 1) }

function start() {
  stop()
  if (props.interval > 0 && many.value) {
    timer = setInterval(() => { if (!paused.value) next() }, props.interval)
  }
}
function stop() { if (timer) { clearInterval(timer); timer = undefined } }

onMounted(() => {
  start()
  mq?.addEventListener('change', onMqChange)
})
onUnmounted(() => {
  stop()
  mq?.removeEventListener('change', onMqChange)
})

// Build the destination route for a banner from its destination_type + targets.
// Destinations are DB-driven (never hardcoded): product / seller / category /
// collection / gender / search / filter.
function destination(b: Banner): string {
  if (!b) return '/'
  const t = b.destination_type
  const tg = b.targets || {}
  switch (t) {
    case 'product':    return tg.slug ? `/product/${tg.slug}` : '/'
    case 'seller':     return tg.slug ? `/seller/${tg.slug}` : '/'
    case 'category':   return tg.slug ? `/category/${tg.slug}` : '/'
    case 'collection': return tg.slug ? `/collections/${tg.slug}` : '/'
    case 'gender':     return tg.code ? `/${tg.code}` : (tg.slug ? `/${tg.slug}` : '/')
    case 'search':
    case 'filter': {
      // Map arbitrary targeting keys to storefront listing query params.
      const params = new URLSearchParams()
      if (tg.q) params.set('q', tg.q)
      if (tg.gender) params.set('gender', tg.gender)
      if (tg.category) params.set('category', tg.category)
      if (tg.brand) params.set('brand', tg.brand)
      if (tg.seller) params.set('seller', tg.seller)
      if (tg.season) params.set('season', tg.season)
      if (tg.min_price) params.set('min_price', tg.min_price)
      if (tg.max_price) params.set('max_price', tg.max_price)
      if (tg.sort) params.set('sort', tg.sort)
      if (tg.availability === 'in_stock' || tg.in_stock === 'true') params.set('in_stock', 'true')
      const qs = params.toString()
      return qs ? `/search?${qs}` : '/'
    }
    default: return '/'
  }
}

function open(b: Banner) {
  const to = destination(b)
  if (to && to !== '/') router.push(to)
}

// Banners whose image already contains its own headline/CTA can opt out of the
// text overlay by setting targets.overlay = 'off'.
function hasOverlay(b: Banner): boolean {
  return (b.targets || {}).overlay !== 'off'
}

// CTA button label is admin-configurable per banner via targets.cta_label.
// Falls back to a sensible default when the admin leaves it blank.
function ctaLabel(b: Banner): string {
  const label = (b.targets || {}).cta_label
  return (typeof label === 'string' && label.trim()) ? label.trim() : 'Shop now'
}

// Prefer the admin's mobile image on small screens, desktop image otherwise.
function imageFor(b: Banner): string {
  const key = isMobile.value && b.mobile_image_key ? b.mobile_image_key : b.image_key
  if (!key) return ''
  // Local/public assets (e.g. /banners/banner1.jpg) and absolute URLs render
  // directly; storage keys resolve against the media base.
  if (key.startsWith('/') || key.startsWith('http')) return key
  return resolveMediaUrl(MEDIA_BASE, key)
}

// Basic touch swipe support.
let touchStartX = 0
function onTouchStart(e: TouchEvent) { touchStartX = e.changedTouches[0].screenX; paused.value = true }
function onTouchEnd(e: TouchEvent) {
  const dx = e.changedTouches[0].screenX - touchStartX
  if (Math.abs(dx) > 40) (dx < 0 ? next() : prev())
  paused.value = false
}
</script>

<template>
  <section
    class="relative overflow-hidden rounded-2xl bg-stone-100"
    aria-label="Featured banners"
    @mouseenter="paused = true"
    @mouseleave="paused = false"
    @touchstart.passive="onTouchStart"
    @touchend.passive="onTouchEnd"
  >
    <div class="relative h-40 xs:h-44 sm:h-64 lg:h-80">
      <!-- Slides (crossfade) -->
      <transition-group name="banner-fade">
        <button
          v-for="(b, i) in banners"
          v-show="i === index"
          :key="b.id"
          type="button"
          class="absolute inset-0 block h-full w-full text-left"
          :aria-label="b.title || 'Banner'"
          @click="open(b)"
        >
          <img
            v-if="imageFor(b)"
            :src="imageFor(b)"
            :alt="b.title"
            class="h-full w-full"
            :class="hasOverlay(b) ? 'object-cover' : 'object-cover sm:object-contain'"
            :loading="i === 0 ? 'eager' : 'lazy'"
          />
          <div v-else class="h-full w-full bg-gradient-to-br from-stone-800 to-stone-900" />

          <!-- Overlay content (skipped for image-only banners that bake in
               their own text, e.g. a designed promo image) -->
          <div
            v-if="hasOverlay(b)"
            class="absolute inset-0 flex flex-col justify-end bg-gradient-to-t from-black/60 via-black/20 to-transparent p-4 sm:p-8 lg:p-10"
          >
            <p v-if="b.subtitle" class="mb-1 text-[10px] font-medium uppercase tracking-widest text-white/70 sm:mb-2 sm:text-xs">
              {{ b.subtitle }}
            </p>
            <h2 v-if="b.title" class="max-w-xl text-lg font-semibold leading-tight text-white sm:text-2xl lg:text-3xl">
              {{ b.title }}
            </h2>
            <span class="mt-3 inline-flex w-fit items-center gap-2 rounded-md bg-accent px-4 py-2 text-xs font-medium text-white sm:mt-4 sm:px-5 sm:py-2.5 sm:text-sm">
              {{ ctaLabel(b) }} <ArrowRight :size="15" />
            </span>
          </div>
        </button>
      </transition-group>

      <!-- Controls -->
      <template v-if="many">
        <button
          type="button"
          class="absolute left-3 top-1/2 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/80 text-ink shadow transition-colors hover:bg-white sm:flex"
          aria-label="Previous banner"
          @click.stop="prev"
        ><ChevronLeft :size="18" /></button>
        <button
          type="button"
          class="absolute right-3 top-1/2 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/80 text-ink shadow transition-colors hover:bg-white sm:flex"
          aria-label="Next banner"
          @click.stop="next"
        ><ChevronRight :size="18" /></button>

        <!-- Dots -->
        <div class="absolute bottom-4 left-1/2 flex -translate-x-1/2 gap-1.5">
          <button
            v-for="(_, i) in banners"
            :key="i"
            type="button"
            :class="['h-1.5 rounded-full transition-all', i === index ? 'w-5 bg-accent' : 'w-1.5 bg-white/60']"
            :aria-label="`Go to banner ${i + 1}`"
            @click.stop="go(i)"
          />
        </div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.banner-fade-enter-active,
.banner-fade-leave-active { transition: opacity 0.5s ease; }
.banner-fade-enter-from,
.banner-fade-leave-to { opacity: 0; }
</style>
