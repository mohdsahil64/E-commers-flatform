<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Layers, Plus } from 'lucide-vue-next'
import type { Collection } from '@carlitos/types'
import { admin } from '@/api'
import { extractApiError } from '@carlitos/shared'

const collections = ref<Collection[]>([])
const loading  = ref(true)
const showForm = ref(false)
const saving   = ref(false)
const error    = ref('')
const success  = ref('')

const form = ref({ name: '', kind: 'featured', type: 'manual', description: '' })

const KINDS = [
  { value: 'new_arrivals', label: 'New Arrivals' },
  { value: 'best_sellers', label: 'Best Sellers' },
  { value: 'trending',     label: 'Trending' },
  { value: 'sale',         label: 'Sale' },
  { value: 'featured',     label: 'Featured' },
  { value: 'recommended',  label: 'Recommended' },
  { value: 'festival',     label: 'Festival' },
  { value: 'seasonal',     label: 'Seasonal' },
]

async function load() { collections.value = await admin.collections().catch(() => []) }

async function create() {
  saving.value = true; error.value = ''; success.value = ''
  try {
    await admin.createCollection(form.value)
    success.value = 'Collection created.'
    form.value = { name: '', kind: 'featured', type: 'manual', description: '' }
    showForm.value = false
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Collections</h1>
      <button class="btn-primary" @click="showForm = !showForm"><Plus :size="15" /> Add Collection</button>
    </div>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>

    <!-- Create form -->
    <div v-if="showForm" class="card p-5 mb-6 space-y-4">
      <h2 class="text-h4">New Collection</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label">Name</label>
          <input v-model="form.name" class="input" placeholder="e.g. Summer Essentials" required />
        </div>
        <div>
          <label class="label">Kind</label>
          <select v-model="form.kind" class="select">
            <option v-for="k in KINDS" :key="k.value" :value="k.value">{{ k.label }}</option>
          </select>
        </div>
        <div>
          <label class="label">Type</label>
          <select v-model="form.type" class="select">
            <option value="manual">Manual (hand-pick products)</option>
            <option value="automatic">Automatic (rule-based)</option>
            <option value="hybrid">Hybrid</option>
          </select>
        </div>
        <div class="sm:col-span-2">
          <label class="label">Description</label>
          <textarea v-model="form.description" class="textarea" rows="2" placeholder="Short description" />
        </div>
      </div>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="showForm = false">Cancel</button>
        <button class="btn-primary" :disabled="saving || !form.name" @click="create">{{ saving ? 'Creating…' : 'Create Collection' }}</button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!collections.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Layers :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No collections yet</p>
      <p class="mt-1 text-xs text-stone-500">Create collections to curate storefront merchandising.</p>
    </div>

    <!-- Table -->
    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr><th>Name</th><th class="hidden sm:table-cell">Kind</th><th class="hidden md:table-cell">Type</th><th>Status</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in collections" :key="c.id">
            <td>
              <p class="font-medium">{{ c.name }}</p>
              <p class="text-xs text-stone-400">{{ c.slug }}</p>
            </td>
            <td class="hidden sm:table-cell text-stone-500 capitalize">{{ (c.kind || '').replace('_',' ') }}</td>
            <td class="hidden md:table-cell text-stone-500 capitalize">{{ c.type }}</td>
            <td><span :class="c.is_active ? 'badge-green' : 'badge-gray'">{{ c.is_active ? 'Active' : 'Inactive' }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
