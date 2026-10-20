<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft, Check, X, Package } from 'lucide-vue-next'
import type { Product } from '@carlitos/types'
import { admin, MEDIA_BASE } from '@/api'
import { formatINR, formatDate, resolveMediaUrl, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route   = useRoute()
const router  = useRouter()
const product = ref<Product | null>(null)
const loading = ref(true)
const busy    = ref(false)
const message = ref('')
const error   = ref('')
const rejectOpen = ref(false)

async function load() {
  product.value = await admin.product(route.params.id as string).catch(() => null)
}

async function approve() {
  busy.value = true; error.value = ''; message.value = ''
  try { await admin.approveProduct(product.value!.id); message.value = 'Product approved.'; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = false }
}

async function confirmReject(reason: string) {
  busy.value = true; error.value = ''; message.value = ''
  try { await admin.rejectProduct(product.value!.id, reason); message.value = 'Product rejected.'; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = false; rejectOpen.value = false }
}

const p = computed(() => product.value)
const images = computed(() => p.value?.images?.sort((a,b)=>a.sort_order-b.sort_order) || [])
const activeImg = ref(0)

function statusClass(s?: string) {
  if (s === 'approved') return 'badge-green'
  if (['rejected','suspended'].includes(s || '')) return 'badge-red'
  return 'badge-amber'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="skeleton h-8 w-1/3 rounded" />
    <div class="grid gap-6 md:grid-cols-2">
      <div class="skeleton aspect-square rounded-xl" />
      <div class="space-y-3"><div class="skeleton h-6 w-2/3 rounded" /><div class="skeleton h-4 w-full rounded" /></div>
    </div>
  </div>

  <div v-else-if="!p" class="py-12 text-center">
    <p class="text-stone-600">Product not found.</p>
    <button class="btn-ghost btn-sm mt-4" @click="router.push('/products')">← All products</button>
  </div>

  <div v-else class="max-w-4xl">
    <button class="mb-5 flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink" @click="router.push('/products')">
      <ChevronLeft :size="16" /> All products
    </button>

    <!-- Header -->
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <div class="flex items-center gap-2">
          <h1 class="page-title">{{ p.title }}</h1>
          <span :class="['badge', statusClass(p.status)]">{{ p.status.replace('_',' ') }}</span>
        </div>
        <p class="mt-0.5 text-sm text-stone-500">{{ p.brand_name || 'No brand' }} · Created {{ formatDate(p.created_at) }}</p>
      </div>
      <div v-if="p.status === 'submitted'" class="flex gap-2">
        <button class="btn-success btn-sm" :disabled="busy" @click="approve"><Check :size="14" /> Approve</button>
        <button class="btn-danger btn-sm" :disabled="busy" @click="rejectOpen = true"><X :size="14" /> Reject</button>
      </div>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <div class="grid gap-6 md:grid-cols-2">
      <!-- Images -->
      <div>
        <div class="aspect-square overflow-hidden rounded-xl bg-stone-100">
          <img v-if="images[activeImg]" :src="resolveMediaUrl(MEDIA_BASE, images[activeImg].file_key)" :alt="p.title" class="h-full w-full object-cover" />
          <div v-else class="flex h-full w-full items-center justify-center text-stone-400"><Package :size="40" /></div>
        </div>
        <div v-if="images.length > 1" class="mt-2 flex gap-2 overflow-x-auto">
          <button v-for="(img,i) in images" :key="img.id" :class="['h-14 w-12 shrink-0 overflow-hidden rounded border-2', i===activeImg ? 'border-ink' : 'border-transparent']" @click="activeImg = i">
            <img :src="resolveMediaUrl(MEDIA_BASE, img.file_key)" class="h-full w-full object-cover" />
          </button>
        </div>
      </div>

      <!-- Details -->
      <div class="space-y-5">
        <div class="card p-5">
          <h2 class="text-h4 mb-3">Pricing & Variants</h2>
          <div class="space-y-2 text-sm">
            <div v-for="v in p.variants" :key="v.id" class="flex justify-between border-b border-stone-100 pb-2 last:border-0">
              <span class="text-stone-500">{{ v.variant_code }}</span>
              <span class="font-medium">{{ formatINR(v.selling_price) }} <span class="text-stone-400 line-through text-xs">{{ formatINR(v.mrp) }}</span></span>
            </div>
            <p v-if="!p.variants?.length" class="text-stone-400">No variants.</p>
          </div>
        </div>

        <div class="card p-5">
          <h2 class="text-h4 mb-3">Details</h2>
          <dl class="grid grid-cols-2 gap-3 text-sm">
            <div><dt class="text-xs uppercase text-stone-400">HSN</dt><dd>{{ p.hsn_code || '—' }}</dd></div>
            <div><dt class="text-xs uppercase text-stone-400">Tax</dt><dd>{{ p.tax_percent }}%</dd></div>
            <div><dt class="text-xs uppercase text-stone-400">Rating</dt><dd>{{ p.rating_avg.toFixed(1) }} ★ ({{ p.rating_count }})</dd></div>
          </dl>
        </div>
      </div>
    </div>

    <div class="card p-5 mt-5">
      <h2 class="text-h4 mb-2">Description</h2>
      <p class="text-sm text-stone-600 whitespace-pre-line">{{ p.description || 'No description.' }}</p>
    </div>

    <div v-if="p.rejection_reason" class="card p-5 mt-5 border-red-200">
      <h2 class="text-h4 mb-1 text-red-700">Rejection Reason</h2>
      <p class="text-sm text-stone-600">{{ p.rejection_reason }}</p>
    </div>

    <ConfirmDialog
      :open="rejectOpen"
      title="Reject product?"
      variant="danger"
      :require-reason="true"
      confirm-label="Reject"
      message="The seller will be notified with your reason."
      @confirm="confirmReject"
      @cancel="rejectOpen = false"
    />
  </div>
</template>
