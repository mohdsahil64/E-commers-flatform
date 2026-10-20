<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChevronRight, ChevronLeft, Save, Send, Info, UploadCloud, X, Star, Loader2,
} from 'lucide-vue-next'
import type { Gender, Category } from '@carlitos/types'
import { seller, api, MEDIA_BASE } from '@/api'
import { publicApi } from '@carlitos/api-client'
import { extractApiError, resolveMediaUrl } from '@carlitos/shared'

const route   = useRoute()
const router  = useRouter()
const editing = computed(() => !!route.params.id)
const pub     = publicApi(api)

const step   = ref(1)
const STEPS  = ['Basics', 'Category', 'Pricing & Stock', 'Review']

// Product "type" categories the seller picks from. Slugs match the backend
// gender-neutral categories (resolved/created server-side by slug).
const TYPE_CATEGORIES = [
  { slug: 'upperwear',  label: 'Upperwear' },
  { slug: 'bottomwear', label: 'Bottomwear' },
  { slug: 'innerwear',  label: 'Innerwear' },
  { slug: 'underwear',  label: 'Underwear' },
  { slug: 'shoes',      label: 'Shoes' },
]
const SEASONS = [
  { value: 'all',    label: 'All season' },
  { value: 'summer', label: 'Summer' },
  { value: 'winter', label: 'Winter' },
]

const MAX_IMAGE_MB = 5           // per-image size limit (client-side guard)
const MAX_IMAGES   = 8           // how many images a seller may attach

const genders    = ref<Gender[]>([])
const categories = ref<Category[]>([])
const error      = ref('')
const saving     = ref(false)
const uploadErr  = ref('')
const uploading  = ref(false)

interface UploadedImage { key: string; url: string; alt: string }

const form = ref({
  title: '', description: '', brand: '',
  gender_id: '', season: 'all',
  mrp: 0, selling_price: 0,
})
const images = ref<UploadedImage[]>([])
// The selected product-type slug is the source of truth we send to the API.
const selectedTypeSlug = ref('')

// ── Size-based stock ──────────────────────────────────────────
// The size set is auto-decided from the product type:
//   • shoes           → shoe numbers (UK/IN)
//   • bottomwear      → waist sizes (jeans/trousers)
//   • upperwear/inner → apparel letter sizes (S/M/L/XL…)
//   • underwear       → letter sizes
const APPAREL_SIZES = ['XS', 'S', 'M', 'L', 'XL', 'XXL', '3XL']
const WAIST_SIZES   = ['28', '30', '32', '34', '36', '38', '40', '42']
const SHOE_SIZES    = ['5', '6', '7', '8', '9', '10', '11', '12']
const sizeSetLabel = computed(() => {
  if (selectedTypeSlug.value === 'shoes') return 'Shoe sizes'
  if (selectedTypeSlug.value === 'bottomwear') return 'Waist sizes'
  return 'Sizes'
})
const sizeOptions = computed(() => {
  switch (selectedTypeSlug.value) {
    case 'shoes':      return SHOE_SIZES
    case 'bottomwear': return WAIST_SIZES
    default:           return APPAREL_SIZES  // upperwear, innerwear, underwear
  }
})

// sizeStock: which sizes are selected and how many pieces each has.
const sizeStock = ref<Record<string, number>>({})
function toggleSize(sz: string) {
  if (sz in sizeStock.value) {
    const next = { ...sizeStock.value }
    delete next[sz]
    sizeStock.value = next
  } else {
    sizeStock.value = { ...sizeStock.value, [sz]: 0 }
  }
}
const selectedSizes = computed(() => Object.keys(sizeStock.value))
const totalStock = computed(() => Object.values(sizeStock.value).reduce((a, b) => a + (Number(b) || 0), 0))
const sizesPayload = computed(() =>
  Object.entries(sizeStock.value).map(([size, quantity]) => ({ size, quantity: Number(quantity) || 0 })),
)

function flatten(cats: Category[]): Category[] {
  const out: Category[] = []
  const walk = (list: Category[]) => list.forEach(c => { out.push(c); if (c.children) walk(c.children) })
  walk(cats)
  return out
}

onMounted(async () => {
  const [g, c] = await Promise.all([
    pub.genders().catch(() => []),
    pub.categories().then(cs => flatten(cs as Category[])).catch(() => []),
  ])
  genders.value    = g as Gender[]
  categories.value = c

  if (editing.value) {
    const p = await seller.product(route.params.id as string).catch(() => null)
    if (p) {
      form.value.title       = p.title
      form.value.description = p.description
      form.value.brand       = p.brand_name || ''
      form.value.gender_id   = p.gender_id || ''
      form.value.season      = p.season || 'all'
      if (p.variants?.[0]) {
        form.value.mrp           = p.variants[0].mrp
        form.value.selling_price = p.variants[0].selling_price
      }
      const match = categories.value.find(cat => cat.id === p.category_id)
      if (match && TYPE_CATEGORIES.some(t => t.slug === match.slug)) {
        selectedTypeSlug.value = match.slug
      }
      // Prefill per-size stock from the product's variants.
      const sizes: Record<string, number> = {}
      for (const v of (p.variants || [])) {
        const label = v.size || v.variant_code
        if (label && label.toUpperCase() !== 'DEFAULT') sizes[label] = v.available_quantity ?? 0
      }
      if (Object.keys(sizes).length) sizeStock.value = sizes
      images.value = (p.images || []).map(img => ({
        key: img.file_key,
        url: resolveMediaUrl(MEDIA_BASE, img.file_key),
        alt: img.alt_text || '',
      }))
    }
  }
})

function pickType(slug: string) {
  const prev = selectedTypeSlug.value
  selectedTypeSlug.value = slug
  error.value = ''
  // Changing the type changes the valid size set, so reset any picked sizes
  // (only on create; edit doesn't collect sizes here).
  if (prev !== slug && !editing.value) sizeStock.value = {}
}

// ── Image upload ──────────────────────────────────────────────
async function onFiles(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  uploadErr.value = ''

  for (const file of files) {
    if (images.value.length >= MAX_IMAGES) { uploadErr.value = `You can upload up to ${MAX_IMAGES} images.`; break }
    if (!file.type.startsWith('image/'))   { uploadErr.value = 'Only image files are allowed.'; continue }
    if (file.size > MAX_IMAGE_MB * 1024 * 1024) { uploadErr.value = `Each image must be under ${MAX_IMAGE_MB}MB.`; continue }
    uploading.value = true
    try {
      const res = await seller.uploadImage(file)
      images.value.push({ key: res.key, url: res.url || resolveMediaUrl(MEDIA_BASE, res.key), alt: '' })
    } catch (err) {
      uploadErr.value = extractApiError(err)
    } finally {
      uploading.value = false
    }
  }
}

function removeImage(i: number) { images.value.splice(i, 1) }
function makePrimary(i: number) {
  if (i === 0) return
  const [img] = images.value.splice(i, 1)
  images.value.unshift(img)
}

// ── Derived ───────────────────────────────────────────────────
const off = computed(() => {
  if (!form.value.mrp || !form.value.selling_price) return 0
  return Math.round((1 - form.value.selling_price / form.value.mrp) * 100)
})
const imagePayload = computed(() => images.value.map(i => ({ key: i.key, alt: i.alt })))

// ── Validation per step ───────────────────────────────────────
const step1Valid = computed(() =>
  form.value.title.trim().length > 1 && form.value.brand.trim().length > 0 && images.value.length > 0
)
const step2Valid = computed(() => !!selectedTypeSlug.value)
const priceValid = computed(() => form.value.mrp > 0 && form.value.selling_price > 0 && form.value.selling_price <= form.value.mrp)
// On create at least one size with stock is required; on edit, sizes/stock are
// managed from the Inventory page so only price matters here.
const step3Valid = computed(() => priceValid.value && (editing.value || selectedSizes.value.length > 0))

function step1Error(): string {
  if (form.value.title.trim().length <= 1) return 'Enter a product title.'
  if (form.value.brand.trim().length === 0) return 'Enter a brand name.'
  if (images.value.length === 0) return 'Upload at least one product image.'
  return ''
}
function step3Error(): string {
  if (!priceValid.value) return 'Enter a valid MRP and selling price (selling ≤ MRP).'
  if (!editing.value && selectedSizes.value.length === 0) return 'Add at least one size with stock.'
  return ''
}

function next() {
  error.value = ''
  if (step.value === 1 && !step1Valid.value) { error.value = step1Error(); return }
  if (step.value === 2 && !step2Valid.value) { error.value = 'Pick a product type.'; return }
  if (step.value === 3 && !step3Valid.value) { error.value = step3Error(); return }
  if (step.value < STEPS.length) step.value++
}

async function save(submit = false) {
  error.value = ''
  if (!step1Valid.value) { error.value = step1Error(); step.value = 1; return }
  if (!step2Valid.value) { error.value = 'Pick a product type.'; step.value = 2; return }
  if (!step3Valid.value) { error.value = step3Error(); step.value = 3; return }
  saving.value = true
  try {
    const common = {
      title: form.value.title,
      description: form.value.description,
      brand_name: form.value.brand.trim(),
      gender_id: form.value.gender_id || null,
      category_slug: selectedTypeSlug.value,
      season: form.value.season,
      mrp: form.value.mrp,
      selling_price: form.value.selling_price,
      discount_percent: off.value,
      images: imagePayload.value,
    }
    if (editing.value) {
      // Sizes/stock are edited from Inventory; here we update core fields only.
      await seller.updateProduct(route.params.id as string, common)
      if (submit) await seller.submitProduct(route.params.id as string).catch(() => {})
    } else {
      const res = await seller.createProduct({ ...common, sizes: sizesPayload.value })
      if (submit && res.id) await seller.submitProduct(res.id).catch(() => {})
    }
    router.push('/products')
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-xl">
    <!-- Header -->
    <div class="mb-4">
      <h1 class="text-lg font-semibold text-ink sm:text-xl">{{ editing ? 'Edit Product' : 'Add Product' }}</h1>
      <p class="mt-0.5 text-xs text-stone-500">Products need admin approval before going live.</p>
    </div>

    <!-- Step indicator -->
    <div class="mb-5 flex items-center gap-1 overflow-x-auto pb-1">
      <template v-for="(s, i) in STEPS" :key="i">
        <button
          type="button"
          :class="[
            'flex shrink-0 items-center gap-1 text-[11px] transition-colors',
            step === i + 1 ? 'font-semibold text-ink' : step > i + 1 ? 'text-emerald-600' : 'text-stone-400'
          ]"
          @click="step > i + 1 && (step = i + 1)"
        >
          <span :class="[
            'flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-semibold',
            step === i+1 ? 'bg-ink text-white' : step > i+1 ? 'bg-emerald-100 text-emerald-700' : 'bg-stone-100 text-stone-400'
          ]">{{ i+1 }}</span>
          <span class="hidden xs:block">{{ s }}</span>
        </button>
        <div v-if="i < STEPS.length-1" class="h-px w-3 shrink-0 bg-stone-200 sm:flex-1" />
      </template>
    </div>

    <div class="card space-y-4 p-4">

      <!-- STEP 1: Basics (title, description, brand, images) -->
      <div v-show="step === 1" class="space-y-3.5">
        <div>
          <label class="label" for="title">Product Title <span class="text-red-500">*</span></label>
          <input id="title" v-model="form.title" class="input" placeholder="e.g. Classic Oxford Shirt" required />
        </div>
        <div>
          <label class="label" for="description">Description</label>
          <textarea id="description" v-model="form.description" class="textarea" rows="3" placeholder="Fabric, fit, care…" />
        </div>
        <div>
          <label class="label" for="brand">Brand <span class="text-red-500">*</span></label>
          <input id="brand" v-model="form.brand" class="input" placeholder="Type your brand name, e.g. Nike" required />
          <p class="field-hint">Enter the brand this product is sold under.</p>
        </div>

        <!-- Images live right here in the first step (at least one required) -->
        <div>
          <label class="label">Product Images <span class="text-red-500">*</span></label>
          <p class="field-hint mb-2">Add at least one. Up to {{ MAX_IMAGES }} images, max {{ MAX_IMAGE_MB }}MB each. The first is the main photo.</p>

          <div v-if="images.length" class="mb-2.5 grid grid-cols-3 gap-2 xs:grid-cols-4">
            <div
              v-for="(img, i) in images" :key="img.key"
              class="group relative aspect-square overflow-hidden rounded-md border border-stone-200 bg-stone-50"
            >
              <img :src="img.url" :alt="img.alt || 'Product image'" class="h-full w-full object-cover" />
              <span v-if="i === 0" class="absolute left-1 top-1 rounded bg-ink px-1.5 py-0.5 text-[9px] font-semibold text-white">Main</span>
              <div class="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-black/45 px-1.5 py-1 opacity-0 transition-opacity group-hover:opacity-100">
                <button v-if="i !== 0" type="button" class="text-[10px] text-white hover:underline" @click="makePrimary(i)">
                  <Star :size="10" class="mr-0.5 inline" />Main
                </button>
                <span v-else />
                <button type="button" class="text-white hover:text-red-300" aria-label="Remove image" @click="removeImage(i)">
                  <X :size="13" />
                </button>
              </div>
            </div>
          </div>

          <label
            v-if="images.length < MAX_IMAGES"
            class="flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-md border-2 border-dashed border-stone-300 bg-stone-50 px-4 py-5 text-center transition-colors hover:border-stone-400"
          >
            <Loader2 v-if="uploading" :size="20" class="animate-spin text-stone-400" />
            <UploadCloud v-else :size="20" class="text-stone-400" />
            <span class="text-xs font-medium text-stone-600">{{ uploading ? 'Uploading…' : 'Tap to upload images' }}</span>
            <span class="text-[11px] text-stone-400">PNG, JPG, WEBP · up to {{ MAX_IMAGE_MB }}MB</span>
            <input type="file" accept="image/*" multiple class="hidden" :disabled="uploading" @change="onFiles" />
          </label>

          <p v-if="uploadErr" class="field-hint text-red-500">{{ uploadErr }}</p>
        </div>
      </div>

      <!-- STEP 2: Category -->
      <div v-show="step === 2" class="space-y-4">
        <div>
          <label class="label">Product Type <span class="text-red-500">*</span></label>
          <p class="field-hint mb-1.5">Customers browse and filter by this on the store.</p>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="t in TYPE_CATEGORIES" :key="t.slug"
              type="button"
              :class="['rounded-md border px-3 py-1.5 text-xs font-medium transition-colors', selectedTypeSlug === t.slug ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-stone-500']"
              @click="pickType(t.slug)"
            >{{ t.label }}</button>
          </div>
        </div>

        <div>
          <label class="label">Season</label>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="s in SEASONS" :key="s.value"
              type="button"
              :class="['rounded-md border px-3 py-1.5 text-xs font-medium transition-colors', form.season === s.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-stone-500']"
              @click="form.season = s.value"
            >{{ s.label }}</button>
          </div>
        </div>

        <div>
          <label class="label">Gender <span class="font-normal text-stone-400">(optional)</span></label>
          <div class="flex flex-wrap gap-1.5">
            <button
              type="button"
              :class="['rounded-md border px-3 py-1.5 text-xs font-medium transition-colors', !form.gender_id ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-stone-500']"
              @click="form.gender_id = ''"
            >Unisex</button>
            <button
              v-for="g in genders" :key="g.id"
              type="button"
              :class="['rounded-md border px-3 py-1.5 text-xs font-medium transition-colors', form.gender_id === g.id ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-stone-500']"
              @click="form.gender_id = g.id"
            >{{ g.name }}</button>
          </div>
        </div>
      </div>

      <!-- STEP 3: Pricing & Stock -->
      <div v-show="step === 3" class="space-y-3.5">
        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">MRP (₹) <span class="text-red-500">*</span></label>
            <input v-model.number="form.mrp" type="number" min="0" inputmode="numeric" class="input" required />
          </div>
          <div>
            <label class="label">Selling Price (₹) <span class="text-red-500">*</span></label>
            <input v-model.number="form.selling_price" type="number" min="0" inputmode="numeric" class="input" required />
          </div>
        </div>
        <div v-if="off > 0" class="flex items-center gap-2 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2">
          <Info :size="14" class="shrink-0 text-emerald-600" />
          <p class="text-xs text-emerald-700">Offer: <strong>{{ off }}% off</strong> from MRP</p>
        </div>
        <div v-else-if="form.selling_price > form.mrp && form.mrp > 0" class="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2">
          <Info :size="14" class="shrink-0 text-red-600" />
          <p class="text-xs text-red-700">Selling price can't be higher than MRP.</p>
        </div>

        <!-- Size-wise stock (create only; edit stock from Inventory) -->
        <div v-if="!editing">
          <label class="label">Stock by Size <span class="text-red-500">*</span></label>
          <p class="field-hint mb-2">
            {{ sizeSetLabel }} for <span class="capitalize">{{ selectedTypeSlug || 'this product' }}</span>.
            Pick the sizes you stock, then enter how many pieces of each.
          </p>
          <div class="mb-3 flex flex-wrap gap-1.5">
            <button
              v-for="sz in sizeOptions" :key="sz"
              type="button"
              :class="['rounded-md border px-3 py-1.5 text-xs font-medium transition-colors', sz in sizeStock ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-stone-500']"
              @click="toggleSize(sz)"
            >{{ sz }}</button>
          </div>

          <div v-if="selectedSizes.length" class="space-y-2">
            <div v-for="sz in selectedSizes" :key="sz" class="flex items-center gap-3">
              <span class="w-12 shrink-0 text-sm font-medium text-ink">{{ sz }}</span>
              <input
                v-model.number="sizeStock[sz]"
                type="number" min="0" inputmode="numeric"
                class="input flex-1"
                :placeholder="`Qty for ${sz}`"
              />
              <button type="button" class="shrink-0 rounded p-1 text-stone-400 hover:text-red-500" :aria-label="`Remove ${sz}`" @click="toggleSize(sz)">
                <X :size="15" />
              </button>
            </div>
            <p class="pt-1 text-xs text-stone-500">Total: <strong class="text-ink">{{ totalStock }}</strong> pcs across {{ selectedSizes.length }} size(s)</p>
          </div>
          <p v-else class="text-xs text-stone-400">No sizes selected yet.</p>
        </div>

        <div v-else class="rounded-md border border-stone-200 bg-stone-50 p-3 text-xs text-stone-600">
          Per-size stock is managed on the <strong>Inventory</strong> page. This form updates product details &amp; price only.
        </div>
      </div>

      <!-- STEP 4: Review -->
      <div v-show="step === 4" class="space-y-3">
        <h3 class="text-sm font-semibold text-ink">Review before {{ editing ? 'saving' : 'submitting' }}</h3>

        <div v-if="images.length" class="flex gap-1.5 overflow-x-auto pb-1">
          <img
            v-for="img in images" :key="img.key"
            :src="img.url" alt="" class="h-16 w-12 shrink-0 rounded-md border border-stone-200 object-cover"
          />
        </div>

        <div class="rounded-md border border-stone-200 p-3 text-xs">
          <div class="flex justify-between py-1"><span class="text-stone-500">Title</span><span class="text-right font-medium">{{ form.title || '—' }}</span></div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Brand</span><span>{{ form.brand || '—' }}</span></div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Type</span><span class="capitalize">{{ selectedTypeSlug || '—' }}</span></div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Season</span><span class="capitalize">{{ form.season }}</span></div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Gender</span><span>{{ genders.find(g=>g.id===form.gender_id)?.name || 'Unisex' }}</span></div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Price</span><span>₹{{ form.selling_price }} <span v-if="off>0" class="text-emerald-600">({{ off }}% off)</span></span></div>
          <div v-if="!editing" class="flex justify-between gap-3 py-1">
            <span class="text-stone-500">Stock</span>
            <span class="text-right">
              <template v-if="selectedSizes.length">
                <span v-for="sz in selectedSizes" :key="sz" class="ml-1 inline-block">{{ sz }}: {{ sizeStock[sz] || 0 }}</span>
                <span class="block text-stone-400">Total {{ totalStock }} pcs</span>
              </template>
              <template v-else>—</template>
            </span>
          </div>
          <div class="flex justify-between py-1"><span class="text-stone-500">Images</span><span>{{ images.length }}</span></div>
        </div>
        <div class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[11px] text-amber-800">
          <Info :size="13" class="mt-0.5 shrink-0" />
          Submitting sends the product to admin review before it appears on the marketplace.
        </div>
      </div>

      <p v-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-600">{{ error }}</p>

      <!-- Navigation -->
      <div class="flex items-center gap-2 border-t border-stone-100 pt-4">
        <button v-if="step > 1" type="button" class="btn-ghost btn-sm" @click="step--">
          <ChevronLeft :size="14" /> Back
        </button>
        <div class="flex-1" />
        <button v-if="step < STEPS.length" type="button" class="btn-primary btn-sm" @click="next">
          Continue <ChevronRight :size="14" />
        </button>
        <template v-else>
          <button type="button" class="btn-ghost btn-sm" :disabled="saving" @click="save(false)">
            <Save :size="14" /> Draft
          </button>
          <button type="button" class="btn-primary btn-sm" :disabled="saving" @click="save(true)">
            <Send :size="14" /> {{ saving ? 'Saving…' : (editing ? 'Save & Submit' : 'Submit') }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>
