<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Image, Plus, Pencil, X, UploadCloud } from 'lucide-vue-next'
import type { Banner } from '@carlitos/types'
import { admin, MEDIA_BASE } from '@/api'
import { resolveMediaUrl, extractApiError } from '@carlitos/shared'

/* ── Banner image specs ───────────────────────────────────────────
   Two separate images per banner (desktop 16:9, mobile 4:3). Whatever the
   admin uploads, we normalise it client-side onto a fixed-size canvas: the
   image is scaled to fit entirely inside the target frame and the leftover
   space is filled white. So any shape/size is accepted — it's letterboxed /
   pillarboxed with white padding to the exact banner dimensions. */
const MAX_FILE_MB = 10
const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']

interface ImageSpec {
  label: string
  outWidth: number
  outHeight: number
  recommend: string
}
const DESKTOP_SPEC: ImageSpec = {
  label: 'Desktop / Laptop', outWidth: 1600, outHeight: 900, recommend: '1600×900 (16:9)',
}
const MOBILE_SPEC: ImageSpec = {
  label: 'Mobile', outWidth: 1080, outHeight: 810, recommend: '1080×810 (4:3)',
}

const banners  = ref<Banner[]>([])
const loading  = ref(true)
const showForm = ref(false)
const saving   = ref(false)
const error    = ref('')
const success  = ref('')
const desktopInput = ref<HTMLInputElement | null>(null)
const mobileInput  = ref<HTMLInputElement | null>(null)
const uploading = ref({ desktop: false, mobile: false })

// null id = creating a new banner; otherwise editing that banner.
const editingId = ref<string | null>(null)

const form = ref({
  title: '',               // banner text (headline)
  cta_label: '',           // button text
  image_key: '',           // desktop/laptop storage key
  mobile_image_key: '',    // mobile storage key
  desktop_preview: '',
  mobile_preview: '',
})

function resetForm() {
  form.value = { title: '', cta_label: '', image_key: '', mobile_image_key: '', desktop_preview: '', mobile_preview: '' }
  editingId.value = null
  error.value = ''
  if (desktopInput.value) desktopInput.value.value = ''
  if (mobileInput.value) mobileInput.value.value = ''
}

async function load() {
  banners.value = await admin.banners().catch(() => [])
}

function openCreate() {
  resetForm()
  showForm.value = true
  success.value = ''
}

function openEdit(b: Banner) {
  resetForm()
  editingId.value = b.id
  form.value.title = b.title
  form.value.cta_label = (b.targets || {}).cta_label || ''
  form.value.image_key = b.image_key || ''
  form.value.mobile_image_key = b.mobile_image_key || ''
  form.value.desktop_preview = b.image_key ? resolveMediaUrl(MEDIA_BASE, b.image_key) : ''
  form.value.mobile_preview = b.mobile_image_key ? resolveMediaUrl(MEDIA_BASE, b.mobile_image_key) : ''
  showForm.value = true
  success.value = ''
}

function closeForm() {
  showForm.value = false
  resetForm()
}

// Load a File into an HTMLImageElement.
function loadImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const img = new window.Image()
    img.onload = () => { resolve(img); /* keep url until drawn */ ;(img as any)._url = url }
    img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('Could not read that image file.')) }
    img.src = url
  })
}

// Normalise any image to the spec's exact dimensions: scale to fit entirely
// inside the frame (no cropping), centre it, and fill the leftover space white.
// Returns a new JPEG File ready to upload. So a wrong-shaped image just gets
// white padding (letterbox/pillarbox) instead of being rejected.
async function normalizeImage(file: File, spec: ImageSpec): Promise<File> {
  if (!ACCEPTED.includes(file.type)) throw new Error('Please choose a JPG, PNG or WebP image.')
  if (file.size > MAX_FILE_MB * 1024 * 1024) throw new Error(`Image is too large. Keep it under ${MAX_FILE_MB} MB.`)

  const img = await loadImage(file)
  try {
    const canvas = document.createElement('canvas')
    canvas.width = spec.outWidth
    canvas.height = spec.outHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Image processing is not supported in this browser.')

    // White background first — this is the padding fill.
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, spec.outWidth, spec.outHeight)

    // Scale to fit entirely inside the frame (contain), then centre.
    const scale = Math.min(spec.outWidth / img.naturalWidth, spec.outHeight / img.naturalHeight)
    const drawW = Math.round(img.naturalWidth * scale)
    const drawH = Math.round(img.naturalHeight * scale)
    const dx = Math.round((spec.outWidth - drawW) / 2)
    const dy = Math.round((spec.outHeight - drawH) / 2)
    ctx.imageSmoothingQuality = 'high'
    ctx.drawImage(img, dx, dy, drawW, drawH)

    const blob: Blob = await new Promise((resolve, reject) =>
      canvas.toBlob(b => b ? resolve(b) : reject(new Error('Could not process the image.')), 'image/jpeg', 0.9),
    )
    const base = (file.name.replace(/\.[^.]+$/, '') || 'banner')
    return new File([blob], `${base}-${spec.outWidth}x${spec.outHeight}.jpg`, { type: 'image/jpeg' })
  } finally {
    const url = (img as any)._url as string | undefined
    if (url) URL.revokeObjectURL(url)
  }
}

async function onFileChange(e: Event, which: 'desktop' | 'mobile') {
  error.value = ''
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const spec = which === 'desktop' ? DESKTOP_SPEC : MOBILE_SPEC

  uploading.value[which] = true
  try {
    // Auto-fit the image to the exact banner size (white padding where needed),
    // then upload the processed version.
    const normalized = await normalizeImage(file, spec)
    const res = await admin.uploadImage(normalized)
    const resolved = res.url || resolveMediaUrl(MEDIA_BASE, res.key)
    if (which === 'desktop') {
      form.value.image_key = res.key
      form.value.desktop_preview = resolved
    } else {
      form.value.mobile_image_key = res.key
      form.value.mobile_preview = resolved
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : extractApiError(err)
    input.value = ''
  } finally {
    uploading.value[which] = false
  }
}

async function save() {
  error.value = ''
  success.value = ''
  if (!form.value.title.trim()) { error.value = 'Banner text is required.'; return }
  if (!editingId.value && !form.value.image_key) { error.value = 'Please upload a desktop banner image.'; return }
  if (!editingId.value && !form.value.mobile_image_key) { error.value = 'Please upload a mobile banner image.'; return }

  saving.value = true
  try {
    const targets: Record<string, string> = {}
    if (form.value.cta_label.trim()) targets.cta_label = form.value.cta_label.trim()

    const body = {
      title: form.value.title.trim(),
      image_key: form.value.image_key,               // empty on edit = keep existing
      mobile_image_key: form.value.mobile_image_key,  // empty on edit = keep existing
      targets,
    }

    if (editingId.value) {
      await admin.updateBanner(editingId.value, body)
      success.value = 'Banner updated.'
    } else {
      await admin.createBanner(body)
      success.value = 'Banner created.'
    }
    closeForm()
    await load()
  } catch (err) {
    error.value = extractApiError(err)
  } finally {
    saving.value = false
  }
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Banners</h1>
      <button class="btn-primary" @click="openCreate"><Plus :size="15" /> Add Banner</button>
    </div>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>

    <!-- Create / edit form -->
    <div v-if="showForm" class="card p-5 mb-6 space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-h4">{{ editingId ? 'Edit Banner' : 'New Banner' }}</h2>
        <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="closeForm">
          <X :size="18" />
        </button>
      </div>

      <!-- Two separate image uploads: desktop + mobile -->
      <div class="grid gap-4 sm:grid-cols-2">
        <!-- Desktop / laptop image -->
        <div>
          <label class="label">Desktop / Laptop Image</label>
          <input ref="desktopInput" type="file" accept="image/jpeg,image/png,image/webp" class="hidden" @change="onFileChange($event, 'desktop')" />
          <div class="relative overflow-hidden rounded-lg border-2 border-dashed border-stone-300 bg-stone-50 transition-colors hover:border-stone-400">
            <div v-if="form.desktop_preview" class="relative aspect-[16/9] w-full">
              <img :src="form.desktop_preview" alt="Desktop banner preview" class="h-full w-full object-cover" />
              <button type="button" class="absolute right-2 top-2 rounded-md bg-white/90 px-2.5 py-1 text-xs font-medium text-ink shadow hover:bg-white" :disabled="uploading.desktop" @click="desktopInput?.click()">Change</button>
            </div>
            <button v-else type="button" class="flex aspect-[16/9] w-full flex-col items-center justify-center gap-2 px-3 text-stone-400" :disabled="uploading.desktop" @click="desktopInput?.click()">
              <UploadCloud :size="26" />
              <span class="text-sm font-medium text-stone-600">{{ uploading.desktop ? 'Processing…' : 'Upload desktop image' }}</span>
              <span class="text-center text-xs text-stone-400">Any image — auto-fit to {{ DESKTOP_SPEC.recommend }} with white padding</span>
            </button>
          </div>
        </div>

        <!-- Mobile image -->
        <div>
          <label class="label">Mobile Image</label>
          <input ref="mobileInput" type="file" accept="image/jpeg,image/png,image/webp" class="hidden" @change="onFileChange($event, 'mobile')" />
          <div class="relative overflow-hidden rounded-lg border-2 border-dashed border-stone-300 bg-stone-50 transition-colors hover:border-stone-400">
            <div v-if="form.mobile_preview" class="relative aspect-[4/3] w-full">
              <img :src="form.mobile_preview" alt="Mobile banner preview" class="h-full w-full object-cover" />
              <button type="button" class="absolute right-2 top-2 rounded-md bg-white/90 px-2.5 py-1 text-xs font-medium text-ink shadow hover:bg-white" :disabled="uploading.mobile" @click="mobileInput?.click()">Change</button>
            </div>
            <button v-else type="button" class="flex aspect-[4/3] w-full flex-col items-center justify-center gap-2 px-3 text-stone-400" :disabled="uploading.mobile" @click="mobileInput?.click()">
              <UploadCloud :size="26" />
              <span class="text-sm font-medium text-stone-600">{{ uploading.mobile ? 'Processing…' : 'Upload mobile image' }}</span>
              <span class="text-center text-xs text-stone-400">Any image — auto-fit to {{ MOBILE_SPEC.recommend }} with white padding</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Banner text -->
      <div>
        <label class="label">Banner Text</label>
        <input v-model="form.title" class="input" placeholder="e.g. Big Fashion Sale" required />
      </div>

      <!-- Button text -->
      <div>
        <label class="label">Button Text</label>
        <input v-model="form.cta_label" class="input" placeholder="Shop now" />
        <p class="field-hint">Shown on the banner's button (defaults to "Shop now").</p>
      </div>

      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>

      <div class="flex gap-3">
        <button class="btn-ghost" @click="closeForm">Cancel</button>
        <button class="btn-primary" :disabled="saving || uploading.desktop || uploading.mobile || !form.title" @click="save">
          {{ saving ? 'Saving…' : editingId ? 'Save Changes' : 'Create Banner' }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="i in 3" :key="i" class="skeleton h-48 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!banners.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Image :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No banners yet</p>
      <p class="mt-1 text-xs text-stone-500">Create a hero banner to feature on the storefront.</p>
    </div>

    <!-- Grid -->
    <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="b in banners" :key="b.id" class="card overflow-hidden">
        <div class="aspect-[16/9] w-full bg-stone-100">
          <img v-if="b.image_key" :src="resolveMediaUrl(MEDIA_BASE, b.image_key)" :alt="b.title" class="h-full w-full object-cover" loading="lazy" />
          <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Image :size="28" /></div>
        </div>
        <div class="p-4">
          <div class="flex items-center justify-between gap-2">
            <p class="min-w-0 flex-1 truncate font-medium text-sm">{{ b.title }}</p>
            <button
              class="flex shrink-0 items-center gap-1 rounded-md border border-stone-200 px-2 py-1 text-xs text-stone-600 hover:border-stone-400 hover:text-ink"
              @click="openEdit(b)"
            ><Pencil :size="13" /> Edit</button>
          </div>
          <p v-if="(b.targets || {}).cta_label" class="mt-1 text-xs text-stone-500">Button: {{ (b.targets || {}).cta_label }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
