<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import {
  User, Mail, Phone, ShieldCheck, KeyRound, Eye, EyeOff, Check, Circle, Save, Users, Clock,
  LayoutDashboard, Pencil,
} from 'lucide-vue-next'
import type { AdminAccount, PermissionItem } from '@carlitos/types'
import { admin } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatDate, extractApiError } from '@carlitos/shared'

const store  = useAuthStore()
const router = useRouter()

const acc     = ref<AdminAccount | null>(null)
const perms   = ref<PermissionItem[]>([])
const loading = ref(true)
const loadErr = ref('')

async function load() {
  loading.value = true; loadErr.value = ''
  try {
    const [a, p] = await Promise.all([admin.account(), admin.permissions().catch(() => [] as PermissionItem[])])
    acc.value = a; perms.value = p
    form.value = { full_name: a.full_name, phone: a.phone }
  } catch (e) {
    loadErr.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)

/* ── profile ─────────────────────────────────────────────────── */
const editing = ref(false)
const form    = ref({ full_name: '', phone: '' })
const saving  = ref(false)
const msg     = ref('')
const err     = ref('')
const fieldErr = ref<Record<string, string>>({})

async function saveProfile() {
  saving.value = true; err.value = ''; msg.value = ''; fieldErr.value = {}
  try {
    acc.value = await admin.updateAccount({ full_name: form.value.full_name.trim(), phone: form.value.phone.trim() })
    if (store.user) store.user.full_name = acc.value.full_name
    editing.value = false
    msg.value = 'Profile updated.'
    setTimeout(() => { msg.value = '' }, 3000)
  } catch (e) {
    err.value = extractApiError(e)
    const f = (e as { response?: { data?: { fields?: Record<string, unknown> } } })?.response?.data?.fields || {}
    fieldErr.value = Object.fromEntries(Object.entries(f).map(([k, v]) => [k, String(v)]))
  } finally {
    saving.value = false
  }
}
function cancelEdit() {
  editing.value = false
  err.value = ''; fieldErr.value = {}
  if (acc.value) form.value = { full_name: acc.value.full_name, phone: acc.value.phone }
}

/* ── access (what this person can open) ─────────────────────── */
const PAGES = [
  { label: 'Dashboard', name: 'dashboard' }, { label: 'Sellers', name: 'sellers' },
  { label: 'Customers', name: 'customers' }, { label: 'Products', name: 'products' },
  { label: 'Categories', name: 'categories' }, { label: 'Orders', name: 'orders' },
  { label: 'Shipping', name: 'shipping' }, { label: 'Returns & Refunds', name: 'returns' },
  { label: 'Customer Support', name: 'support' }, { label: 'Seller Support', name: 'seller-support' },
  { label: 'Packaging', name: 'packaging' }, { label: 'Payments', name: 'payments' },
  { label: 'Settlements', name: 'settlements' }, { label: 'Reports', name: 'reports' },
  { label: 'Collections', name: 'collections' }, { label: 'Banners', name: 'banners' },
  { label: 'Team page', name: 'team' }, { label: 'Coupons', name: 'coupons' },
  { label: 'Promotions', name: 'promotions' }, { label: 'Employees & Roles', name: 'roles' },
  { label: 'Audit logs', name: 'audit-logs' },
]
const myPages = computed(() => PAGES.filter((p) => store.canAccess(p.name)))
const myPerms = computed(() => {
  const mine = new Set(acc.value?.permissions || [])
  const everything = acc.value?.is_super_admin
  const groups: { name: string; items: PermissionItem[] }[] = []
  for (const p of perms.value) {
    if (!everything && !mine.has(p.code)) continue
    let g = groups.find((x) => x.name === p.group)
    if (!g) { g = { name: p.group, items: [] }; groups.push(g) }
    g.items.push(p)
  }
  return groups
})
const roleNames = computed(() => (acc.value?.roles || []).map((r) => r.name).join(', ') || '—')

/* ── password ────────────────────────────────────────────────── */
const pw = ref({ current: '', next: '', confirm: '' })
const showPw = ref(false)
const pwSaving = ref(false)
const pwErr = ref('')
const rules = computed(() => [
  { ok: pw.value.next.length >= 10,          label: 'At least 10 characters' },
  { ok: /[a-z]/.test(pw.value.next),         label: 'A lowercase letter' },
  { ok: /[A-Z]/.test(pw.value.next),         label: 'An uppercase letter' },
  { ok: /[0-9]/.test(pw.value.next),         label: 'A number' },
  { ok: /[^A-Za-z0-9]/.test(pw.value.next),  label: 'A symbol (e.g. @ # $ !)' },
])
const pwReady = computed(() =>
  !!pw.value.current && rules.value.every((r) => r.ok) && pw.value.next === pw.value.confirm && !pwSaving.value,
)
async function changePassword() {
  if (!pwReady.value) return
  pwSaving.value = true; pwErr.value = ''
  try {
    await admin.changeAdminPassword(pw.value.current, pw.value.next)
    // The server signed every session out; go back to sign-in.
    await store.logout()
    router.replace({ name: 'login' })
  } catch (e) {
    pwErr.value = extractApiError(e)
  } finally {
    pwSaving.value = false
  }
}

const initial = computed(() => (acc.value?.full_name || acc.value?.email || '?').charAt(0).toUpperCase())
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="page-header">
      <div>
        <h1 class="page-title">Settings</h1>
        <p class="mt-0.5 text-sm text-stone-500">Your account, what you can access, and your security.</p>
      </div>
    </div>

    <div v-if="loading" class="space-y-4"><div class="skeleton h-40 w-full rounded-xl" /><div class="skeleton h-48 w-full rounded-xl" /></div>
    <div v-else-if="loadErr" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
      {{ loadErr }} <button class="ml-2 underline" @click="load">Try again</button>
    </div>

    <template v-else-if="acc">
      <!-- Profile -->
      <section class="card mb-5 p-6" aria-labelledby="profile-h">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="flex items-center gap-4">
            <div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-ink text-xl font-semibold text-white">{{ initial }}</div>
            <div class="min-w-0">
              <h2 id="profile-h" class="truncate text-lg font-semibold text-ink">{{ acc.full_name || acc.email }}</h2>
              <div class="mt-1 flex flex-wrap items-center gap-1.5">
                <span class="badge badge-blue"><ShieldCheck :size="11" class="mr-1" />{{ roleNames }}</span>
                <span class="badge badge-gray">{{ acc.kind === 'owner' ? 'Platform owner' : 'Employee' }}</span>
                <span :class="['badge', acc.status === 'active' ? 'badge-green' : 'badge-red']">{{ acc.status === 'active' ? 'Active' : 'Disabled' }}</span>
              </div>
            </div>
          </div>
          <button v-if="!editing" class="btn-ghost btn-sm" @click="editing = true"><Pencil :size="13" /> Edit</button>
        </div>

        <dl v-if="!editing" class="mt-5 grid gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
          <div><dt class="text-label-sm flex items-center gap-1"><Mail :size="11" /> Email</dt><dd class="mt-0.5 break-all text-ink">{{ acc.email }}</dd></div>
          <div><dt class="text-label-sm flex items-center gap-1"><Phone :size="11" /> Mobile</dt><dd class="mt-0.5 text-ink">{{ acc.phone || '—' }}</dd></div>
          <div><dt class="text-label-sm flex items-center gap-1"><Clock :size="11" /> Last sign-in</dt><dd class="mt-0.5 text-ink">{{ formatDate(acc.last_login_at) }}</dd></div>
          <div v-if="acc.kind === 'employee'">
            <dt class="text-label-sm flex items-center gap-1"><User :size="11" /> Joined</dt>
            <dd class="mt-0.5 text-ink">{{ formatDate(acc.accepted_at || acc.created_at) }}<span v-if="acc.invited_by" class="text-stone-500"> · invited by {{ acc.invited_by }}</span></dd>
          </div>
          <div v-else><dt class="text-label-sm flex items-center gap-1"><User :size="11" /> Account created</dt><dd class="mt-0.5 text-ink">{{ formatDate(acc.created_at) }}</dd></div>
        </dl>

        <form v-else class="mt-5 grid gap-4 sm:grid-cols-2" @submit.prevent="saveProfile" novalidate>
          <div>
            <label class="label" for="st-name">Full name</label>
            <input id="st-name" v-model="form.full_name" class="input" :class="{ 'input-error': fieldErr.full_name }" autocomplete="name" />
            <p v-if="fieldErr.full_name" class="field-error">{{ fieldErr.full_name }}</p>
          </div>
          <div>
            <label class="label" for="st-phone">Mobile</label>
            <input id="st-phone" v-model="form.phone" type="tel" class="input" :class="{ 'input-error': fieldErr.phone }" autocomplete="tel" />
            <p v-if="fieldErr.phone" class="field-error">{{ fieldErr.phone }}</p>
          </div>
          <div class="sm:col-span-2">
            <label class="label" for="st-email">Email</label>
            <input id="st-email" :value="acc.email" class="input" disabled />
            <p class="field-hint">Your sign-in email can only be changed by a Super Admin.</p>
          </div>
          <p v-if="err" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600 sm:col-span-2" role="alert">{{ err }}</p>
          <div class="flex gap-2 sm:col-span-2">
            <button type="submit" class="btn-primary" :disabled="saving || form.full_name.trim().length < 3"><Save :size="14" /> {{ saving ? 'Saving…' : 'Save' }}</button>
            <button type="button" class="btn-ghost" :disabled="saving" @click="cancelEdit">Cancel</button>
          </div>
        </form>
        <p v-if="msg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ msg }}</p>
      </section>

      <!-- Team overview: only for people who manage staff -->
      <section v-if="acc.team" class="card mb-5 p-6" aria-labelledby="team-h">
        <div class="mb-3 flex items-center justify-between gap-2">
          <h2 id="team-h" class="flex items-center gap-2 text-base font-semibold"><Users :size="16" class="text-stone-500" /> Your team</h2>
          <RouterLink to="/roles" class="text-sm text-stone-500 underline hover:text-ink">Manage</RouterLink>
        </div>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="rounded-lg bg-stone-50 p-3"><p class="text-label-sm">Active</p><p class="mt-1 text-lg font-semibold text-emerald-700">{{ acc.team.active }}</p></div>
          <div class="rounded-lg bg-stone-50 p-3"><p class="text-label-sm">Pending</p><p class="mt-1 text-lg font-semibold text-amber-600">{{ acc.team.pending }}</p></div>
          <div class="rounded-lg bg-stone-50 p-3"><p class="text-label-sm">Disabled</p><p class="mt-1 text-lg font-semibold text-stone-500">{{ acc.team.disabled }}</p></div>
          <div class="rounded-lg bg-stone-50 p-3"><p class="text-label-sm">Roles</p><p class="mt-1 text-lg font-semibold">{{ acc.team.roles }}</p></div>
        </div>
      </section>

      <!-- Access -->
      <section class="card mb-5 p-6" aria-labelledby="access-h">
        <h2 id="access-h" class="mb-1 flex items-center gap-2 text-base font-semibold"><LayoutDashboard :size="16" class="text-stone-500" /> What you can access</h2>
        <p class="mb-4 text-sm text-stone-500">
          <template v-if="acc.is_super_admin">As a Super Admin you can open every section and perform every action.</template>
          <template v-else>These come from your role ({{ roleNames }}). Ask an administrator if you need more access.</template>
        </p>
        <p class="text-label-sm mb-2">Pages</p>
        <div class="mb-5 flex flex-wrap gap-1.5">
          <span v-for="p in myPages" :key="p.name" class="rounded-md bg-stone-100 px-2 py-1 text-xs text-stone-700">{{ p.label }}</span>
          <span v-if="!myPages.length" class="text-sm text-stone-400">Dashboard only</span>
        </div>
        <details v-if="myPerms.length" class="group">
          <summary class="cursor-pointer text-sm text-stone-600 hover:text-ink">View detailed permissions ({{ acc.is_super_admin ? 'all' : acc.permissions.length }})</summary>
          <div class="mt-3 grid gap-4 sm:grid-cols-2">
            <div v-for="g in myPerms" :key="g.name">
              <p class="text-label-sm mb-1">{{ g.name }}</p>
              <ul class="space-y-1">
                <li v-for="p in g.items" :key="p.code" class="flex items-start gap-1.5 text-sm text-stone-700">
                  <Check :size="13" class="mt-0.5 shrink-0 text-emerald-600" />{{ p.label }}
                </li>
              </ul>
            </div>
          </div>
        </details>
      </section>

      <!-- Security -->
      <section class="card p-6" aria-labelledby="sec-h">
        <h2 id="sec-h" class="mb-1 flex items-center gap-2 text-base font-semibold"><KeyRound :size="16" class="text-stone-500" /> Change password</h2>
        <p class="mb-4 text-sm text-stone-500">You'll be signed out on every device and asked to sign in with the new password.</p>
        <form class="space-y-4" @submit.prevent="changePassword" novalidate>
          <div>
            <label class="label" for="pw-cur">Current password</label>
            <input id="pw-cur" v-model="pw.current" :type="showPw ? 'text' : 'password'" class="input" autocomplete="current-password" />
          </div>
          <div>
            <label class="label" for="pw-new">New password</label>
            <div class="relative">
              <input id="pw-new" v-model="pw.next" :type="showPw ? 'text' : 'password'" class="input pr-10" autocomplete="new-password" />
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" :aria-label="showPw ? 'Hide passwords' : 'Show passwords'" @click="showPw = !showPw">
                <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
              </button>
            </div>
            <ul class="mt-2 grid grid-cols-1 gap-x-3 gap-y-1 text-xs sm:grid-cols-2" aria-label="Password requirements">
              <li v-for="r in rules" :key="r.label" :class="['flex items-center gap-1.5', r.ok ? 'text-emerald-600' : 'text-stone-400']">
                <Check v-if="r.ok" :size="12" /><Circle v-else :size="12" /> {{ r.label }}
              </li>
            </ul>
          </div>
          <div>
            <label class="label" for="pw-conf">Confirm new password</label>
            <input id="pw-conf" v-model="pw.confirm" :type="showPw ? 'text' : 'password'" class="input" autocomplete="new-password" />
            <p v-if="pw.confirm && pw.confirm !== pw.next" class="field-error">Passwords don't match.</p>
          </div>
          <p v-if="pwErr" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ pwErr }}</p>
          <button type="submit" class="btn-primary" :disabled="!pwReady"><KeyRound :size="14" /> {{ pwSaving ? 'Updating…' : 'Update password' }}</button>
        </form>
      </section>
    </template>
  </div>
</template>
