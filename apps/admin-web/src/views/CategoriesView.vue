<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { FolderTree, ChevronRight, Plus, Layers } from 'lucide-vue-next'
import type { Category, Gender, Brand } from '@carlitos/types'
import { api, admin } from '@/api'
import { publicApi } from '@carlitos/api-client'
import { extractApiError } from '@carlitos/shared'

const pub      = publicApi(api)
const tree     = ref<Category[]>([])
const genders  = ref<Gender[]>([])
const brands   = ref<Brand[]>([])
const loading  = ref(true)
const tab      = ref<'categories'|'genders'|'brands'>('categories')

// New category form
const showForm = ref(false)
const saving   = ref(false)
const error    = ref('')
const form     = ref({ name: '', parent_id: '', gender_id: '' })

async function load() {
  loading.value = true
  try {
    [tree.value, genders.value, brands.value] = await Promise.all([
      pub.categories().then(c => c as Category[]).catch(() => []),
      pub.genders().catch(() => []),
      pub.brands().catch(() => []),
    ])
  } finally { loading.value = false }
}

async function createCategory() {
  saving.value = true; error.value = ''
  try {
    await admin.createCategory({
      name: form.value.name,
      parent_id: form.value.parent_id || null,
      gender_id: form.value.gender_id || null,
    })
    form.value = { name: '', parent_id: '', gender_id: '' }
    showForm.value = false
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Catalog Taxonomy</h1>
      <button v-if="tab === 'categories'" class="btn-primary" @click="showForm = !showForm">
        <Plus :size="15" /> Add Category
      </button>
    </div>

    <!-- Tabs -->
    <div class="mb-5 flex gap-2">
      <button v-for="t in (['categories','genders','brands'] as const)" :key="t"
        :class="['rounded-full border px-3.5 py-1.5 text-sm capitalize transition-colors', tab === t ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="tab = t"
      >{{ t }}</button>
    </div>

    <!-- New category form -->
    <div v-if="showForm && tab === 'categories'" class="card p-5 mb-5 space-y-4">
      <h2 class="text-h4">New Category</h2>
      <div class="grid gap-4 sm:grid-cols-3">
        <div>
          <label class="label">Name</label>
          <input v-model="form.name" class="input" placeholder="e.g. Dresses" required />
        </div>
        <div>
          <label class="label">Gender</label>
          <select v-model="form.gender_id" class="select">
            <option value="">— None —</option>
            <option v-for="g in genders" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <div>
          <label class="label">Parent Category</label>
          <select v-model="form.parent_id" class="select">
            <option value="">— Top level —</option>
            <option v-for="c in tree" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
      </div>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="showForm = false">Cancel</button>
        <button class="btn-primary" :disabled="saving || !form.name" @click="createCategory">
          {{ saving ? 'Creating…' : 'Create Category' }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Categories tree -->
    <div v-else-if="tab === 'categories'">
      <div v-if="!tree.length" class="flex flex-col items-center rounded-xl border border-stone-200 bg-stone-50 py-12">
        <FolderTree :size="40" class="text-stone-300 mb-3" />
        <p class="text-sm font-medium text-stone-700">No categories yet</p>
      </div>
      <div v-else class="space-y-2">
        <div v-for="c in tree" :key="c.id" class="card p-4">
          <div class="flex items-center gap-2">
            <FolderTree :size="16" class="text-stone-400" />
            <p class="font-medium">{{ c.name }}</p>
            <span v-if="!c.is_active" class="badge-gray">Inactive</span>
          </div>
          <div v-if="c.children && c.children.length" class="mt-3 flex flex-wrap gap-2 pl-6">
            <span v-for="sub in c.children" :key="sub.id" class="inline-flex items-center gap-1 rounded-md border border-stone-200 bg-stone-50 px-2.5 py-1 text-xs text-stone-600">
              <ChevronRight :size="11" /> {{ sub.name }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Genders -->
    <div v-else-if="tab === 'genders'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div v-for="g in genders" :key="g.id" class="card p-4">
        <p class="font-medium">{{ g.name }}</p>
        <p class="text-xs text-stone-400 mt-0.5">{{ g.code }}</p>
        <span :class="['mt-2 inline-flex', g.is_active ? 'badge-green' : 'badge-gray']">{{ g.is_active ? 'Active' : 'Inactive' }}</span>
      </div>
    </div>

    <!-- Brands -->
    <div v-else-if="tab === 'brands'" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="b in brands" :key="b.id" class="card p-4 flex items-center justify-between">
        <div>
          <p class="font-medium">{{ b.name }}</p>
          <p class="text-xs text-stone-400 mt-0.5">{{ b.slug }}</p>
        </div>
        <span :class="[b.is_active ? 'badge-green' : 'badge-gray']">{{ b.is_active ? 'Active' : 'Inactive' }}</span>
      </div>
    </div>
  </div>
</template>
