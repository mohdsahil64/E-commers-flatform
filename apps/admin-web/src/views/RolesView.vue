<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import {
  UserPlus, Search, Mail, Phone, RefreshCw, Ban, CheckCircle2, Trash2, X, Shield,
  ShieldCheck, Lock, Users, Pencil, Plus, Copy, Clock,
} from 'lucide-vue-next'
import type { StaffMember, AdminRole, PermissionItem, StaffStatus } from '@carlitos/types'
import { admin } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatDate, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const store = useAuthStore()
const canStaff = computed(() => store.hasPermission('user.manage'))
const canRoles = computed(() => store.hasPermission('role.manage'))

type Tab = 'employees' | 'roles'
const tab = ref<Tab>(canStaff.value ? 'employees' : 'roles')

const staff   = ref<StaffMember[]>([])
const roles   = ref<AdminRole[]>([])
const perms   = ref<PermissionItem[]>([])
const loading = ref(true)
const loadErr = ref('')

// One-line status banner shown above the content.
const flash = ref<{ kind: 'success' | 'warn' | 'error'; text: string } | null>(null)
let flashTimer: ReturnType<typeof setTimeout> | undefined
function say(kind: 'success' | 'warn' | 'error', text: string) {
  flash.value = { kind, text }
  clearTimeout(flashTimer)
  flashTimer = setTimeout(() => { flash.value = null }, kind === 'success' ? 5000 : 9000)
}
onBeforeUnmount(() => clearTimeout(flashTimer))

async function load() {
  loading.value = true; loadErr.value = ''
  try {
    const [s, r, p] = await Promise.all([
      canStaff.value ? admin.staff() : Promise.resolve([] as StaffMember[]),
      admin.roles(),
      admin.permissions(),
    ])
    staff.value = s; roles.value = r; perms.value = p
  } catch (e) {
    loadErr.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)

/* ── helpers ─────────────────────────────────────────────────── */
function apiFields(e: unknown): Record<string, string> {
  const f = (e as { response?: { data?: { fields?: Record<string, unknown> } } })?.response?.data?.fields || {}
  return Object.fromEntries(Object.entries(f).map(([k, v]) => [k, String(v)]))
}
const permLabel = computed(() => Object.fromEntries(perms.value.map((p) => [p.code, p.label])))
const permGroups = computed(() => {
  const order: string[] = []
  const map: Record<string, PermissionItem[]> = {}
  for (const p of perms.value) {
    if (!map[p.group]) { map[p.group] = []; order.push(p.group) }
    map[p.group].push(p)
  }
  return order.map((g) => ({ name: g, items: map[g] }))
})

const STATUS_META: Record<StaffStatus, { label: string; cls: string }> = {
  active:   { label: 'Active',             cls: 'badge-green' },
  pending:  { label: 'Invitation pending', cls: 'badge-amber' },
  expired:  { label: 'Invite expired',     cls: 'badge-red' },
  disabled: { label: 'Disabled',           cls: 'badge-gray' },
}

/* ── employees: filters ─────────────────────────────────────── */
const query  = ref('')
const status = ref<'all' | StaffStatus>('all')
const counts = computed(() => ({
  active:   staff.value.filter((s) => s.status === 'active').length,
  pending:  staff.value.filter((s) => s.status === 'pending' || s.status === 'expired').length,
  disabled: staff.value.filter((s) => s.status === 'disabled').length,
}))
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return staff.value.filter((s) => {
    if (status.value === 'pending') {
      if (s.status !== 'pending' && s.status !== 'expired') return false
    } else if (status.value !== 'all' && s.status !== status.value) return false
    if (!q) return true
    return [s.full_name, s.email, s.phone, s.role_name].some((v) => (v || '').toLowerCase().includes(q))
  })
})

function canActOn(s: StaffMember) {
  return canStaff.value && !s.is_you && !s.is_super_admin
}
const assignableRoles = computed(() => roles.value.filter((r) => r.assignable))

/* ── invite ─────────────────────────────────────────────────── */
const inviteOpen = ref(false)
const inviting   = ref(false)
const inviteErr  = ref('')
const inviteFields = ref<Record<string, string>>({})
const invite = ref({ full_name: '', email: '', phone: '', role_id: '' })
const inviteRole = computed(() => roles.value.find((r) => r.id === invite.value.role_id))

function openInvite() {
  invite.value = { full_name: '', email: '', phone: '', role_id: '' }
  inviteErr.value = ''; inviteFields.value = {}
  inviteOpen.value = true
}
const inviteReady = computed(() =>
  invite.value.full_name.trim().length > 2 && /^\S+@\S+\.\S+$/.test(invite.value.email.trim()) &&
  invite.value.phone.replace(/\D/g, '').length >= 10 && !!invite.value.role_id,
)
async function sendInvite() {
  if (!inviteReady.value || inviting.value) return
  inviting.value = true; inviteErr.value = ''; inviteFields.value = {}
  try {
    const res = await admin.inviteStaff({
      full_name: invite.value.full_name.trim(), email: invite.value.email.trim(),
      phone: invite.value.phone.trim(), role_id: invite.value.role_id,
    })
    inviteOpen.value = false
    await load()
    if (res.email_sent) say('success', `Invitation sent to ${res.employee.email}. They have 72 hours to sign in.`)
    else say('warn', `${res.employee.full_name} was added, but the invitation email could not be sent. Use "Resend" to try again.`)
  } catch (e) {
    inviteErr.value = extractApiError(e)
    inviteFields.value = apiFields(e)
  } finally {
    inviting.value = false
  }
}

/* ── row actions ────────────────────────────────────────────── */
const busyId = ref('')
async function run(id: string, fn: () => Promise<void>) {
  busyId.value = id
  try { await fn() } catch (e) { say('error', extractApiError(e)) } finally { busyId.value = '' }
}
function resend(s: StaffMember) {
  return run(s.id, async () => {
    const res = await admin.resendStaffInvite(s.id)
    await load()
    if (res.email_sent) say('success', `A new invitation was sent to ${s.email}. The previous temporary password no longer works.`)
    else say('warn', `A new temporary password was created for ${s.full_name}, but the email could not be sent. Try Resend again.`)
  })
}
function enable(s: StaffMember) {
  return run(s.id, async () => { await admin.enableStaff(s.id); await load(); say('success', `${s.full_name} can sign in again.`) })
}

// Confirmations (disable / remove invitation).
const confirm = ref<{ kind: 'disable' | 'remove'; s: StaffMember } | null>(null)
async function doConfirm() {
  const c = confirm.value
  confirm.value = null
  if (!c) return
  if (c.kind === 'disable') {
    await run(c.s.id, async () => { await admin.disableStaff(c.s.id); await load(); say('success', `${c.s.full_name} was disabled and signed out everywhere.`) })
  } else {
    await run(c.s.id, async () => { await admin.cancelStaffInvite(c.s.id); await load(); say('success', 'Invitation removed.') })
  }
}

// Change role.
const roleTarget = ref<StaffMember | null>(null)
const roleChoice = ref('')
const roleErr    = ref('')
const roleSaving = ref(false)
function openChangeRole(s: StaffMember) { roleTarget.value = s; roleChoice.value = s.role_id; roleErr.value = '' }
async function saveChangeRole() {
  const s = roleTarget.value
  if (!s || !roleChoice.value || roleChoice.value === s.role_id) { roleTarget.value = null; return }
  roleSaving.value = true; roleErr.value = ''
  try {
    await admin.changeStaffRole(s.id, roleChoice.value)
    roleTarget.value = null
    await load()
    say('success', `${s.full_name}'s role was updated. It applies immediately.`)
  } catch (e) { roleErr.value = extractApiError(e) } finally { roleSaving.value = false }
}

/* ── roles ──────────────────────────────────────────────────── */
type Editor = { id: string; name: string; description: string; selected: Set<string>; readOnly: boolean; system: boolean; members: number; editable: boolean }
const editor = ref<Editor | null>(null)
const editorErr = ref('')
const editorFields = ref<Record<string, string>>({})
const editorSaving = ref(false)
const delOpen = ref(false)

function openRole(r: AdminRole) {
  editor.value = {
    id: r.id, name: r.name, description: r.description, selected: new Set(r.permissions),
    readOnly: !(canRoles.value && r.editable), system: r.is_system, members: r.member_count, editable: r.editable,
  }
  editorErr.value = ''; editorFields.value = {}
}
function openNewRole(from?: AdminRole) {
  const held = new Set(perms.value.filter((p) => p.held).map((p) => p.code))
  editor.value = {
    id: '', name: from ? `${from.name} (copy)` : '', description: from?.description || '',
    selected: new Set((from?.permissions || []).filter((p) => held.has(p))),
    readOnly: false, system: false, members: 0, editable: true,
  }
  editorErr.value = ''; editorFields.value = {}
}
function togglePerm(code: string) {
  const e = editor.value
  if (!e || e.readOnly) return
  const next = new Set(e.selected)
  next.has(code) ? next.delete(code) : next.add(code)
  e.selected = next
}
function groupState(items: PermissionItem[]) {
  const usable = items.filter((p) => p.held)
  const on = usable.filter((p) => editor.value?.selected.has(p.code)).length
  return { usable: usable.length, on }
}
function toggleGroup(items: PermissionItem[]) {
  const e = editor.value
  if (!e || e.readOnly) return
  const usable = items.filter((p) => p.held)
  const allOn = usable.length > 0 && usable.every((p) => e.selected.has(p.code))
  const next = new Set(e.selected)
  usable.forEach((p) => (allOn ? next.delete(p.code) : next.add(p.code)))
  e.selected = next
}
async function saveRole() {
  const e = editor.value
  if (!e || e.readOnly || editorSaving.value) return
  editorSaving.value = true; editorErr.value = ''; editorFields.value = {}
  const body = { name: e.name.trim(), description: e.description.trim(), permissions: [...e.selected] }
  try {
    if (e.id) await admin.updateRole(e.id, body)
    else await admin.createRole(body)
    editor.value = null
    await load()
    say('success', e.id ? 'Role updated. Changes apply to everyone with this role immediately.' : 'Role created. You can now assign it to employees.')
  } catch (err) {
    editorErr.value = extractApiError(err)
    editorFields.value = apiFields(err)
  } finally { editorSaving.value = false }
}
async function removeRole() {
  const e = editor.value
  delOpen.value = false
  if (!e || !e.id) return
  editorSaving.value = true; editorErr.value = ''
  try {
    await admin.deleteRole(e.id)
    editor.value = null
    await load()
    say('success', 'Role deleted.')
  } catch (err) { editorErr.value = extractApiError(err) } finally { editorSaving.value = false }
}
const sourceRole = computed(() => roles.value.find((r) => r.id === editor.value?.id))

// Esc closes whichever dialog is open.
function onKey(ev: KeyboardEvent) {
  if (ev.key !== 'Escape') return
  if (confirm.value || delOpen.value) return
  if (inviteOpen.value) inviteOpen.value = false
  else if (roleTarget.value) roleTarget.value = null
  else if (editor.value) editor.value = null
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
watch(tab, () => { flash.value = null })
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Employees &amp; Roles</h1>
        <p class="mt-0.5 text-sm text-stone-500">Invite your team and decide exactly which pages each person can open.</p>
      </div>
      <button v-if="tab === 'employees' && canStaff" class="btn-primary" @click="openInvite">
        <UserPlus :size="15" /> Invite employee
      </button>
      <button v-else-if="tab === 'roles' && canRoles" class="btn-primary" @click="openNewRole()">
        <Plus :size="15" /> Create role
      </button>
    </div>

    <!-- Tabs -->
    <div class="mb-5 flex gap-1 border-b border-stone-200" role="tablist">
      <button v-if="canStaff" role="tab" :aria-selected="tab === 'employees'"
        :class="['-mb-px flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm transition-colors', tab === 'employees' ? 'border-ink font-medium text-ink' : 'border-transparent text-stone-500 hover:text-ink']"
        @click="tab = 'employees'">
        <Users :size="15" /> Employees
        <span class="rounded bg-stone-100 px-1.5 text-xs text-stone-600">{{ staff.length }}</span>
      </button>
      <button role="tab" :aria-selected="tab === 'roles'"
        :class="['-mb-px flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm transition-colors', tab === 'roles' ? 'border-ink font-medium text-ink' : 'border-transparent text-stone-500 hover:text-ink']"
        @click="tab = 'roles'">
        <Shield :size="15" /> Roles &amp; permissions
        <span class="rounded bg-stone-100 px-1.5 text-xs text-stone-600">{{ roles.length }}</span>
      </button>
    </div>

    <div v-if="flash" role="status"
      :class="['mb-4 rounded-lg border p-3 text-sm', flash.kind === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : flash.kind === 'warn' ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-red-200 bg-red-50 text-red-700']">
      {{ flash.text }}
    </div>

    <div v-if="loading" class="space-y-2"><div v-for="i in 5" :key="i" class="skeleton h-14 w-full rounded-lg" /></div>
    <div v-else-if="loadErr" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
      {{ loadErr }} <button class="ml-2 underline" @click="load">Try again</button>
    </div>

    <!-- ═══ EMPLOYEES ═══ -->
    <template v-else-if="tab === 'employees' && canStaff">
      <div class="mb-4 grid grid-cols-3 gap-3">
        <button class="stat text-left" @click="status = 'active'"><p class="text-label-sm">Active</p><p class="mt-1 text-xl font-semibold text-emerald-700">{{ counts.active }}</p></button>
        <button class="stat text-left" @click="status = 'pending'"><p class="text-label-sm">Pending invitation</p><p class="mt-1 text-xl font-semibold text-amber-600">{{ counts.pending }}</p></button>
        <button class="stat text-left" @click="status = 'disabled'"><p class="text-label-sm">Disabled</p><p class="mt-1 text-xl font-semibold text-stone-500">{{ counts.disabled }}</p></button>
      </div>

      <div class="mb-4 flex flex-wrap items-center gap-2">
        <div class="relative min-w-0 flex-1 basis-60">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
          <input v-model="query" class="input pl-9" placeholder="Search name, email, phone or role…" aria-label="Search employees" />
        </div>
        <select v-model="status" class="select w-full sm:w-52" aria-label="Filter by status">
          <option value="all">All statuses</option>
          <option value="active">Active</option>
          <option value="pending">Invitation pending</option>
          <option value="expired">Invite expired</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>

      <div v-if="!filtered.length" class="rounded-xl border border-stone-200 bg-stone-50 p-10 text-center text-sm text-stone-500">
        {{ staff.length ? 'No employees match your filters.' : 'No employees yet. Invite your first team member.' }}
      </div>

      <div v-else class="table-wrap card">
        <table class="table min-w-[760px]">
          <thead>
            <tr><th>Employee</th><th>Role</th><th>Status</th><th class="hidden lg:table-cell">Activity</th><th class="text-right">Actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="s in filtered" :key="s.id" class="!cursor-default">
              <td>
                <div class="flex items-center gap-3">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-stone-100 text-sm font-semibold text-stone-600">
                    {{ (s.full_name || s.email).charAt(0).toUpperCase() }}
                  </div>
                  <div class="min-w-0">
                    <p class="truncate font-medium text-ink">{{ s.full_name || '—' }}
                      <span v-if="s.is_you" class="ml-1 rounded bg-stone-100 px-1.5 py-0.5 text-[10px] font-medium text-stone-500">You</span>
                    </p>
                    <p class="flex items-center gap-1 truncate text-xs text-stone-500"><Mail :size="11" /> {{ s.email }}</p>
                    <p v-if="s.phone" class="flex items-center gap-1 truncate text-xs text-stone-400"><Phone :size="11" /> {{ s.phone }}</p>
                  </div>
                </div>
              </td>
              <td>
                <span class="inline-flex items-center gap-1 text-sm">
                  <ShieldCheck v-if="s.is_super_admin" :size="13" class="text-amber-600" />{{ s.role_name || '—' }}
                </span>
              </td>
              <td>
                <span :class="['badge', STATUS_META[s.status].cls]">{{ STATUS_META[s.status].label }}</span>
                <p v-if="s.status === 'pending' && s.invite_expires_at" class="mt-1 flex items-center gap-1 text-[11px] text-stone-400"><Clock :size="10" /> expires {{ formatDate(s.invite_expires_at) }}</p>
              </td>
              <td class="hidden text-xs text-stone-500 lg:table-cell">
                <template v-if="s.status === 'active'">Last sign-in {{ s.last_login_at ? formatDate(s.last_login_at) : 'never' }}</template>
                <template v-else-if="s.status === 'disabled'">Joined {{ formatDate(s.accepted_at || s.created_at) }}</template>
                <template v-else>Invited {{ formatDate(s.invited_at) }}<span v-if="s.invited_by"> by {{ s.invited_by }}</span><span v-if="s.invite_count > 1"> · sent {{ s.invite_count }}×</span></template>
              </td>
              <td>
                <div v-if="canActOn(s)" class="flex flex-wrap justify-end gap-1.5">
                  <template v-if="s.status === 'pending' || s.status === 'expired'">
                    <button class="btn-ghost btn-xs" :disabled="busyId === s.id" @click="resend(s)"><RefreshCw :size="12" /> Resend</button>
                    <button class="btn-ghost btn-xs text-red-600" :disabled="busyId === s.id" @click="confirm = { kind: 'remove', s }"><Trash2 :size="12" /> Remove</button>
                  </template>
                  <template v-else-if="s.status === 'active'">
                    <button class="btn-ghost btn-xs" :disabled="busyId === s.id" @click="openChangeRole(s)"><Pencil :size="12" /> Role</button>
                    <button class="btn-ghost btn-xs text-red-600" :disabled="busyId === s.id" @click="confirm = { kind: 'disable', s }"><Ban :size="12" /> Disable</button>
                  </template>
                  <template v-else>
                    <button class="btn-ghost btn-xs" :disabled="busyId === s.id" @click="enable(s)"><CheckCircle2 :size="12" /> Enable</button>
                    <button v-if="!s.accepted_at" class="btn-ghost btn-xs text-red-600" :disabled="busyId === s.id" @click="confirm = { kind: 'remove', s }"><Trash2 :size="12" /> Remove</button>
                  </template>
                </div>
                <p v-else class="text-right text-xs text-stone-300">{{ s.is_you ? '—' : 'Protected' }}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- ═══ ROLES ═══ -->
    <template v-else>
      <p class="mb-4 text-sm text-stone-500">
        A role is a bundle of permissions. Built-in roles are read-only; create a custom role to give someone exactly the access they need.
        <span v-if="!canRoles">You can view roles but not change them.</span>
      </p>
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <button v-for="r in roles" :key="r.id" class="card p-4 text-left transition-colors hover:border-stone-400" @click="openRole(r)">
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="flex items-center gap-1.5 truncate font-medium text-ink">
                <ShieldCheck v-if="r.code === 'SUPER_ADMIN'" :size="14" class="text-amber-600" />{{ r.name }}
              </p>
              <p v-if="r.description" class="mt-0.5 line-clamp-2 text-xs text-stone-500">{{ r.description }}</p>
            </div>
            <span :class="['badge shrink-0', r.is_system ? 'badge-gray' : 'badge-blue']">
              <Lock v-if="r.is_system" :size="10" class="mr-1" />{{ r.is_system ? 'Built-in' : 'Custom' }}
            </span>
          </div>
          <div class="mt-3 flex flex-wrap gap-1">
            <span v-for="p in r.permissions.slice(0, 4)" :key="p" class="rounded bg-stone-100 px-1.5 py-0.5 text-[11px] text-stone-600">{{ permLabel[p] || p }}</span>
            <span v-if="r.permissions.length > 4" class="rounded bg-stone-100 px-1.5 py-0.5 text-[11px] text-stone-500">+{{ r.permissions.length - 4 }} more</span>
          </div>
          <p class="mt-3 text-xs text-stone-400">{{ r.permissions.length }} permission{{ r.permissions.length === 1 ? '' : 's' }} · {{ r.member_count }} employee{{ r.member_count === 1 ? '' : 's' }}</p>
        </button>
      </div>
    </template>

    <!-- ═══ Invite dialog ═══ -->
    <div v-if="inviteOpen" class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4 sm:items-center" role="dialog" aria-modal="true" aria-labelledby="invite-title">
      <div class="absolute inset-0 bg-black/40" @click="inviteOpen = false" />
      <form class="relative my-8 w-full max-w-lg rounded-xl bg-white p-6 shadow-lg" @submit.prevent="sendInvite" novalidate>
        <div class="flex items-start justify-between">
          <div>
            <h2 id="invite-title" class="text-h4">Invite an employee</h2>
            <p class="mt-1 text-sm text-stone-500">They'll get an email with a one-time temporary password and a link to sign in.</p>
          </div>
          <button type="button" class="text-stone-400 hover:text-stone-600" aria-label="Close" @click="inviteOpen = false"><X :size="18" /></button>
        </div>

        <div class="mt-5 space-y-4">
          <div>
            <label class="label" for="inv-name">Full name</label>
            <input id="inv-name" v-model="invite.full_name" class="input" :class="{ 'input-error': inviteFields.full_name }" autocomplete="off" autofocus />
            <p v-if="inviteFields.full_name" class="field-error">{{ inviteFields.full_name }}</p>
          </div>
          <div>
            <label class="label" for="inv-email">Work email</label>
            <input id="inv-email" v-model="invite.email" type="email" class="input" :class="{ 'input-error': inviteFields.email }" autocomplete="off" />
            <p v-if="inviteFields.email" class="field-error">{{ inviteFields.email }}</p>
          </div>
          <div>
            <label class="label" for="inv-phone">Mobile number</label>
            <input id="inv-phone" v-model="invite.phone" type="tel" inputmode="tel" class="input" :class="{ 'input-error': inviteFields.phone }" placeholder="10-digit mobile" autocomplete="off" />
            <p v-if="inviteFields.phone" class="field-error">{{ inviteFields.phone }}</p>
          </div>
          <div>
            <label class="label" for="inv-role">Role</label>
            <select id="inv-role" v-model="invite.role_id" class="select" :class="{ 'input-error': inviteFields.role_id }">
              <option value="" disabled>Choose a role…</option>
              <option v-for="r in assignableRoles" :key="r.id" :value="r.id">{{ r.name }}{{ r.is_system ? '' : ' (custom)' }}</option>
            </select>
            <p v-if="inviteFields.role_id" class="field-error">{{ inviteFields.role_id }}</p>
            <p v-else-if="!assignableRoles.length" class="field-hint">No role available. Create a custom role first (Roles &amp; permissions tab).</p>
            <div v-if="inviteRole" class="mt-2 rounded-lg border border-stone-200 bg-stone-50 p-3">
              <p class="text-xs font-medium text-stone-600">This role can open:</p>
              <div class="mt-1.5 flex flex-wrap gap-1">
                <span v-for="p in inviteRole.permissions" :key="p" class="rounded bg-white px-1.5 py-0.5 text-[11px] text-stone-600 ring-1 ring-stone-200">{{ permLabel[p] || p }}</span>
              </div>
            </div>
          </div>
        </div>

        <p v-if="inviteErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ inviteErr }}</p>

        <div class="mt-6 flex justify-end gap-3">
          <button type="button" class="btn-ghost" @click="inviteOpen = false">Cancel</button>
          <button type="submit" class="btn-primary" :disabled="!inviteReady || inviting">
            <Mail :size="15" /> {{ inviting ? 'Sending…' : 'Send invitation' }}
          </button>
        </div>
      </form>
    </div>

    <!-- ═══ Change role dialog ═══ -->
    <div v-if="roleTarget" class="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-labelledby="role-title">
      <div class="absolute inset-0 bg-black/40" @click="roleTarget = null" />
      <div class="relative w-full max-w-md rounded-xl bg-white p-6 shadow-lg">
        <h2 id="role-title" class="text-h4">Change role</h2>
        <p class="mt-1 text-sm text-stone-500">{{ roleTarget.full_name }} · {{ roleTarget.email }}. The new role applies on their next click — no sign-out needed.</p>
        <label class="label mt-4" for="chg-role">Role</label>
        <select id="chg-role" v-model="roleChoice" class="select">
          <option v-for="r in assignableRoles" :key="r.id" :value="r.id">{{ r.name }}{{ r.is_system ? '' : ' (custom)' }}</option>
        </select>
        <p v-if="roleErr" class="mt-3 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ roleErr }}</p>
        <div class="mt-6 flex justify-end gap-3">
          <button class="btn-ghost" @click="roleTarget = null">Cancel</button>
          <button class="btn-primary" :disabled="roleSaving || roleChoice === roleTarget.role_id" @click="saveChangeRole">{{ roleSaving ? 'Saving…' : 'Save role' }}</button>
        </div>
      </div>
    </div>

    <!-- ═══ Role editor ═══ -->
    <div v-if="editor" class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4 sm:items-center" role="dialog" aria-modal="true" aria-labelledby="editor-title">
      <div class="absolute inset-0 bg-black/40" @click="editor = null" />
      <form class="relative my-8 w-full max-w-2xl rounded-xl bg-white p-6 shadow-lg" @submit.prevent="saveRole" novalidate>
        <div class="flex items-start justify-between">
          <div>
            <h2 id="editor-title" class="text-h4">{{ !editor.id ? 'Create role' : editor.readOnly ? editor.name : 'Edit role' }}</h2>
            <p v-if="editor.system" class="mt-1 flex items-center gap-1 text-sm text-stone-500"><Lock :size="12" /> Built-in role — read-only. Duplicate it to make a custom version.</p>
            <p v-else-if="editor.id && editor.readOnly" class="mt-1 text-sm text-stone-500">This role grants access you don't have yourself, so you can't change it.</p>
            <p v-else-if="editor.id && editor.members" class="mt-1 text-sm text-amber-700">{{ editor.members }} employee{{ editor.members === 1 ? '' : 's' }} use this role. Changes apply to them immediately.</p>
          </div>
          <button type="button" class="text-stone-400 hover:text-stone-600" aria-label="Close" @click="editor = null"><X :size="18" /></button>
        </div>

        <div v-if="!editor.system" class="mt-5 grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label" for="role-name">Role name</label>
            <input id="role-name" v-model="editor.name" class="input" :class="{ 'input-error': editorFields.name }" :disabled="editor.readOnly" maxlength="40" placeholder="e.g. Catalog Reviewer" />
            <p v-if="editorFields.name" class="field-error">{{ editorFields.name }}</p>
          </div>
          <div>
            <label class="label" for="role-desc">Description <span class="font-normal text-stone-400">(optional)</span></label>
            <input id="role-desc" v-model="editor.description" class="input" :disabled="editor.readOnly" maxlength="200" placeholder="What is this role for?" />
            <p v-if="editorFields.description" class="field-error">{{ editorFields.description }}</p>
          </div>
        </div>

        <div class="mt-5 max-h-[50vh] space-y-4 overflow-y-auto pr-1">
          <fieldset v-for="g in permGroups" :key="g.name" class="rounded-lg border border-stone-200 p-3">
            <legend class="px-1 text-xs font-semibold uppercase tracking-wide text-stone-500">{{ g.name }}</legend>
            <button v-if="!editor.readOnly && groupState(g.items).usable" type="button" class="mb-2 text-xs text-stone-500 underline hover:text-ink" @click="toggleGroup(g.items)">
              {{ groupState(g.items).on === groupState(g.items).usable ? 'Clear group' : 'Select all in group' }}
            </button>
            <div class="grid gap-x-4 gap-y-2 sm:grid-cols-2">
              <label v-for="p in g.items" :key="p.code"
                :class="['flex items-start gap-2 text-sm', editor.readOnly || !p.held ? 'cursor-not-allowed' : 'cursor-pointer']"
                :title="!p.held && !editor.readOnly ? 'You can only grant permissions you hold yourself' : p.description">
                <input type="checkbox" class="mt-0.5 h-4 w-4 accent-ink" :checked="editor.selected.has(p.code)"
                  :disabled="editor.readOnly || !p.held" @change="togglePerm(p.code)" />
                <span :class="p.held || editor.readOnly ? 'text-stone-700' : 'text-stone-300'">
                  {{ p.label }}
                  <span class="block text-[11px] leading-snug text-stone-400">{{ p.description }}</span>
                </span>
              </label>
            </div>
          </fieldset>
        </div>
        <p v-if="editorFields.permissions" class="field-error mt-2">{{ editorFields.permissions }}</p>
        <p v-if="editorErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ editorErr }}</p>

        <div class="mt-6 flex flex-wrap items-center justify-between gap-3">
          <div class="flex gap-2">
            <button v-if="editor.id && editor.editable && !editor.readOnly" type="button" class="btn-ghost text-red-600" :disabled="editorSaving" @click="delOpen = true">
              <Trash2 :size="14" /> Delete
            </button>
            <button v-if="editor.id && canRoles && sourceRole" type="button" class="btn-ghost" @click="openNewRole(sourceRole)">
              <Copy :size="14" /> Duplicate
            </button>
          </div>
          <div class="flex gap-3">
            <button type="button" class="btn-ghost" @click="editor = null">{{ editor.readOnly ? 'Close' : 'Cancel' }}</button>
            <button v-if="!editor.readOnly" type="submit" class="btn-primary" :disabled="editorSaving || !editor.selected.size || editor.name.trim().length < 2">
              {{ editorSaving ? 'Saving…' : editor.id ? 'Save changes' : 'Create role' }}
            </button>
          </div>
        </div>
      </form>
    </div>

    <ConfirmDialog
      :open="!!confirm"
      :title="confirm?.kind === 'disable' ? `Disable ${confirm?.s.full_name}?` : 'Remove this invitation?'"
      :message="confirm?.kind === 'disable'
        ? 'They are signed out immediately and cannot sign in until you enable the account again.'
        : 'The invitation and temporary password are deleted. You can invite them again later.'"
      :confirm-label="confirm?.kind === 'disable' ? 'Disable' : 'Remove'"
      variant="danger"
      @confirm="doConfirm"
      @cancel="confirm = null"
    />
    <ConfirmDialog
      :open="delOpen"
      :title="`Delete role “${editor?.name}”?`"
      message="This can't be undone. A role that is still assigned to employees can't be deleted."
      confirm-label="Delete role"
      variant="danger"
      @confirm="removeRole"
      @cancel="delOpen = false"
    />
  </div>
</template>
