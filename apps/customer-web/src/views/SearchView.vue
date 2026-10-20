<script setup lang="ts">
import { onMounted, ref, watch, nextTick } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { Search, X, TrendingUp } from 'lucide-vue-next'
import type { ListingCard } from '@carlitos/types'
import { pub, MEDIA_BASE } from '@/api'
import { resolveMediaUrl } from '@carlitos/shared'
import { useSeo } from '@/composables/useSeo'
import ProductCard from '@/components/ProductCard.vue'
import PageBack from '@/components/PageBack.vue'

const route = useRoute()
const router = useRouter()

const query = ref((route.query.q as string) || '')
const results = ref<ListingCard[]>([])
const suggestions = ref<ListingCard[]>([])
const popular = ref<ListingCard[]>([])
const loading = ref(false)
const suggestLoading = ref(false)
const showSuggest = ref(false)
const searched = ref(false)
const inputEl = ref<HTMLInputElement | null>(null)

// Quick category chips shown when idle (real gender-neutral categories).
const quickCategories = [
  { label: 'Upperwear', slug: 'upperwear' },
  { label: 'Bottomwear', slug: 'bottomwear' },
  { label: 'Shoes', slug: 'shoes' },
  { label: 'Innerwear', slug: 'innerwear' },
]

let debounce: ReturnType<typeof setTimeout> | undefined

const { set: setSeo } = useSeo()

function updateSeo() {
  setSeo({
    title: query.value ? `Search "${query.value}"` : 'Search',
    description: query.value
      ? `Search results for "${query.value}" on CARLITOS.`
      : 'Search products, brands and sellers on CARLITOS.',
  })
}

// Full-results fetch (on submit / when arriving with ?q=).
async function runSearch(q: string) {
  const term = q.trim()
  showSuggest.value = false
  if (!term) { results.value = []; searched.value = false; updateSeo(); return }
  loading.value = true
  searched.value = true
  try {
    const res = await pub.products({ q: term, page_size: 40 })
    results.value = (res.data as ListingCard[]) || []
  } catch {
    results.value = []
  } finally {
    loading.value = false
    updateSeo()
  }
}

// Debounced live suggestions as the user types.
function onInput() {
  clearTimeout(debounce)
  const q = query.value.trim()
  if (q.length < 2) { suggestions.value = []; showSuggest.value = false; return }
  showSuggest.value = true
  suggestLoading.value = true
  debounce = setTimeout(async () => {
    try {
      const res = await pub.products({ q, page_size: 6 })
      suggestions.value = (res.data as ListingCard[]) || []
    } catch {
      suggestions.value = []
    } finally {
      suggestLoading.value = false
    }
  }, 300)
}

function submit() {
  const term = query.value.trim()
  if (!term) return
  router.replace({ name: 'search', query: { q: term } })
  runSearch(term)
}

function pickSuggestion(s: ListingCard) {
  showSuggest.value = false
  router.push(`/product/${s.slug}`)
}

function clearQuery() {
  query.value = ''
  results.value = []
  suggestions.value = []
  searched.value = false
  showSuggest.value = false
  updateSeo()
  nextTick(() => inputEl.value?.focus())
}

onMounted(async () => {
  await nextTick()
  // Load popular products for the idle state (real data).
  pub.products({ sort: 'popularity', page_size: 8 })
    .then(res => { popular.value = (res.data as ListingCard[]) || [] })
    .catch(() => {})
  if (query.value) runSearch(query.value)
  else { inputEl.value?.focus(); updateSeo() }
})

// Respond to external ?q= changes (e.g. banner links, header removed but links remain).
watch(() => route.query.q, (q) => {
  const term = (q as string) || ''
  if (term !== query.value) { query.value = term; runSearch(term) }
})
</script>

<template>
  <div class="mx-auto max-w-5xl">
    <PageBack />
    <!-- Search box -->
    <div class="relative" data-search-box>
      <form class="relative" @submit.prevent="submit">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
        <input
          ref="inputEl"
          v-model="query"
          type="search"
          enterkeyhint="search"
          placeholder="Search products, brands, sellers…"
          class="w-full rounded-full border border-stone-300 bg-white py-2.5 pl-10 pr-10 text-sm text-ink outline-none transition-colors focus:border-accent focus:ring-1 focus:ring-accent"
          aria-label="Search"
          autocomplete="off"
          @input="onInput"
          @focus="query.trim().length >= 2 && suggestions.length && (showSuggest = true)"
        />
        <button
          v-if="query"
          type="button"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 flex h-6 w-6 items-center justify-center rounded-full text-stone-400 hover:bg-stone-100 hover:text-ink"
          aria-label="Clear search"
          @click="clearQuery"
        >
          <X :size="15" />
        </button>
      </form>

      <!-- Live suggestions -->
      <transition name="fade">
        <div
          v-if="showSuggest && (suggestLoading || suggestions.length)"
          class="absolute left-0 right-0 top-full z-20 mt-2 overflow-hidden rounded-xl border border-stone-200 bg-white shadow-lg"
        >
          <div v-if="suggestLoading" class="space-y-2 p-3">
            <div v-for="i in 3" :key="i" class="flex items-center gap-3">
              <div class="skeleton h-10 w-8 rounded" />
              <div class="flex-1 space-y-1"><div class="skeleton h-3 w-2/3 rounded" /><div class="skeleton h-3 w-1/3 rounded" /></div>
            </div>
          </div>
          <ul v-else>
            <li v-for="s in suggestions" :key="s.product_id">
              <button class="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-stone-50" @click="pickSuggestion(s)">
                <div class="h-10 w-8 shrink-0 overflow-hidden rounded bg-stone-100">
                  <img v-if="s.primary_image" :src="resolveMediaUrl(MEDIA_BASE, s.primary_image)" :alt="s.title" class="h-full w-full object-cover" loading="lazy" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-sm text-ink">{{ s.title }}</p>
                  <p class="truncate text-xs text-stone-400">{{ s.brand_name }}</p>
                </div>
              </button>
            </li>
            <li class="border-t border-stone-100">
              <button class="w-full px-3 py-2.5 text-left text-sm font-medium text-accent hover:bg-accent-soft/40" @click="submit">
                See all results for "{{ query }}"
              </button>
            </li>
          </ul>
        </div>
      </transition>
    </div>

    <!-- Results -->
    <div class="mt-6">
      <!-- Loading -->
      <div v-if="loading" class="grid grid-cols-3 gap-2.5 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6">
        <div v-for="i in 12" :key="i" class="overflow-hidden rounded-lg border border-stone-200">
          <div class="skeleton aspect-[3/4] w-full" />
          <div class="space-y-2 p-2"><div class="skeleton h-3 w-1/2 rounded" /><div class="skeleton h-3 w-3/4 rounded" /></div>
        </div>
      </div>

      <!-- Results grid -->
      <template v-else-if="searched">
        <div v-if="results.length" class="space-y-4">
          <p class="text-sm text-stone-500">{{ results.length }} result{{ results.length > 1 ? 's' : '' }} for "<span class="font-medium text-ink">{{ query }}</span>"</p>
          <div class="grid grid-cols-3 gap-2.5 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6">
            <ProductCard v-for="p in results" :key="p.product_id" :product="p" compact />
          </div>
        </div>
        <!-- No results -->
        <div v-else class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-16 text-center">
          <Search :size="40" class="mb-3 text-stone-300" />
          <p class="text-base font-medium text-stone-700">No results for "{{ query }}"</p>
          <p class="mt-1 text-sm text-stone-500">Try a different keyword or check the spelling.</p>
        </div>
      </template>

      <!-- Idle: quick categories + popular products -->
      <div v-else class="space-y-6">
        <div>
          <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-stone-400">Browse categories</p>
          <div class="flex flex-wrap gap-2">
            <RouterLink
              v-for="c in quickCategories"
              :key="c.slug"
              :to="`/category/${c.slug}`"
              class="rounded-full border border-stone-300 bg-white px-4 py-1.5 text-sm text-stone-700 transition-colors hover:border-accent hover:text-accent"
            >{{ c.label }}</RouterLink>
          </div>
        </div>

        <div v-if="popular.length">
          <p class="mb-3 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-stone-400">
            <TrendingUp :size="14" /> Popular right now
          </p>
          <div class="grid grid-cols-3 gap-2.5 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6">
            <ProductCard v-for="p in popular" :key="p.product_id" :product="p" compact />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active { transition: opacity 150ms ease, transform 150ms ease; }
.fade-enter-from,
.fade-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
