<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Users, Pencil, X, UploadCloud } from 'lucide-vue-next'
import type { TeamMember } from '@carlitos/types'
import { admin, MEDIA_BASE } from '@/api'
import { resolveMediaUrl, extractApiError } from '@carlitos/shared'

/* Team & Management manager.
   Members are seeded with system-default role/bio; admins only edit each
   member's name and photo. No create/delete — the roster is fixed. */
const MAX_FILE_MB = 10
const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']

const members   = ref<TeamMember[]>([])
const loading   = ref(true)
const saving    = ref(false)
const uploading = ref(false)
const error     = ref('')
const success   = ref('')

// null = no member being edited; otherwise the id of the member in the form.
const editingId = ref<string | null>(null)
const photoInput = ref<HTMLInputElement | null>(null)

const form = ref({
  name: '',
  role: '',
  bio: '',
  photo_key: '',
  photo_preview: '',
})

async function load() {
  members.value = await admin.team().catch(() => [])
}

function openEdit(m: TeamMember) {
  editingId.value = m.id
  form.value = {
    name: m.name,
    role: m.role,
    bio: m.bio,
    photo_key: '', // empty = keep existing unless a new photo is uploaded
    photo_preview: m.photo_key ? resolveMediaUrl(MEDIA_BASE, m.photo_key) : '',
  }
  error.value = ''
  success.value = ''
  if (photoInput.value) photoInput.value.value = ''
}

function closeForm() {
  editingId.value = null
  error.value = ''
  if (photoInput.value) photoInput.value.value = ''
}

async function onFileChange(e: Event) {
  error.value = ''
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!ACCEPTED.includes(file.type)) { error.value = 'Please choose a JPG, PNG or WebP image.'; input.value = ''; return }
  if (file.size > MAX_FILE_MB * 1024 * 1024) { error.value = `Image is too large. Keep it under ${MAX_FILE_MB} MB.`; input.value = ''; return }

  uploading.value = true
  try {
    const res = await admin.uploadImage(file)
    form.value.photo_key = res.key
    form.value.photo_preview = res.url || resolveMediaUrl(MEDIA_BASE, res.key)
  } catch (err) {
    error.value = err instanceof Error ? err.message : extractApiError(err)
    input.value = ''
  } finally {
    uploading.value = false
  }
}

async function save() {
  error.value = ''
  success.value = ''
  if (!form.value.name.trim()) { error.value = 'Name is required.'; return }
  if (!editingId.value) return

  saving.value = true
  try {
    await admin.updateTeamMember(editingId.value, {
      name: form.value.name.trim(),
      photo_key: form.value.photo_key, // empty = keep existing
    })
    success.value = 'Team member updated.'
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
      <h1 class="page-title">Team &amp; Management</h1>
    </div>

    <p class="mb-4 text-sm text-stone-500">
      Edit each member's name and photo. Roles and bios are fixed defaults.
    </p>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>

    <!-- Loading -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="i in 3" :key="i" class="skeleton h-48 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!members.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Users :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No team members yet</p>
    </div>

    <!-- Grid -->
    <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="m in members" :key="m.id" class="card overflow-hidden">
        <div class="aspect-square w-full bg-stone-100">
          <img v-if="m.photo_key" :src="resolveMediaUrl(MEDIA_BASE, m.photo_key)" :alt="m.name" class="h-full w-full object-cover" loading="lazy" />
          <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Users :size="28" /></div>
        </div>
        <div class="p-4">
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0 flex-1">
              <p class="truncate font-medium text-sm">{{ m.name }}</p>
              <p class="mt-0.5 text-xs font-medium text-accent">{{ m.role }}</p>
            </div>
            <button
              class="flex shrink-0 items-center gap-1 rounded-md border border-stone-200 px-2 py-1 text-xs text-stone-600 hover:border-stone-400 hover:text-ink"
              @click="openEdit(m)"
            ><Pencil :size="13" /> Edit</button>
          </div>
          <p v-if="m.bio" class="mt-2 text-xs text-stone-500">{{ m.bio }}</p>
        </div>
      </div>
    </div>

    <!-- Edit modal -->
    <div v-if="editingId" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="closeForm">
      <div class="card w-full max-w-md p-5 space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-h4">Edit Team Member</h2>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="closeForm">
            <X :size="18" />
          </button>
        </div>

        <!-- Photo -->
        <div>
          <label class="label">Photo</label>
          <input ref="photoInput" type="file" accept="image/jpeg,image/png,image/webp" class="hidden" @change="onFileChange" />
          <div class="relative overflow-hidden rounded-lg border-2 border-dashed border-stone-300 bg-stone-50 transition-colors hover:border-stone-400">
            <div v-if="form.photo_preview" class="relative aspect-square w-full">
              <img :src="form.photo_preview" alt="Photo preview" class="h-full w-full object-cover" />
              <button type="button" class="absolute right-2 top-2 rounded-md bg-white/90 px-2.5 py-1 text-xs font-medium text-ink shadow hover:bg-white" :disabled="uploading" @click="photoInput?.click()">Change</button>
            </div>
            <button v-else type="button" class="flex aspect-square w-full flex-col items-center justify-center gap-2 px-3 text-stone-400" :disabled="uploading" @click="photoInput?.click()">
              <UploadCloud :size="26" />
              <span class="text-sm font-medium text-stone-600">{{ uploading ? 'Uploading…' : 'Upload photo' }}</span>
              <span class="text-center text-xs text-stone-400">JPG, PNG or WebP</span>
            </button>
          </div>
        </div>

        <!-- Name (editable) -->
        <div>
          <label class="label">Name</label>
          <input v-model="form.name" class="input" placeholder="Full name" required />
        </div>

        <!-- Role (read-only) -->
        <div>
          <label class="label">Role</label>
          <input :value="form.role" class="input bg-stone-50 text-stone-500" disabled />
          <p class="field-hint">Role is a fixed default and cannot be changed.</p>
        </div>

        <!-- Bio (read-only) -->
        <div v-if="form.bio">
          <label class="label">Bio</label>
          <p class="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-sm text-stone-500">{{ form.bio }}</p>
        </div>

        <p v-if="error" class="text-sm text-red-600">{{ error }}</p>

        <div class="flex gap-3">
          <button class="btn-ghost" @click="closeForm">Cancel</button>
          <button class="btn-primary" :disabled="saving || uploading || !form.name" @click="save">
            {{ saving ? 'Saving…' : 'Save Changes' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
