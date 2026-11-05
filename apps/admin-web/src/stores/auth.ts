import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { AuthUser, AdminSetupRequired } from '@carlitos/types'
import { auth, tokenStore } from '@/api'
import { PAGE_ACCESS } from '@/access'

// Admin Desk auth store. Admin accounts are provisioned or invited (no public
// register). The permissions held here are a CONVENIENCE for the UI; the server
// re-checks them against the database on every API call.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const loading = ref(false)
  const initialized = ref(false)
  // Pending first-login setup for an invited employee (temp password accepted,
  // emailed code + new password still required). No tokens exist yet.
  const setup = ref<AdminSetupRequired | null>(null)
  // Reactive mirror of the access token — reading localStorage inside a computed
  // is not reactive, which made post-login redirects flaky (only worked after a
  // reload). Tracking it in a ref keeps isAuthenticated reactive.
  const accessToken = ref<string | null>(tokenStore.getAccess())
  let lastSync = 0

  function setTokens(access: string, refresh: string) {
    tokenStore.set(access, refresh)
    accessToken.value = access
  }
  function clearTokens() {
    tokenStore.clear()
    accessToken.value = null
  }
  const isAuthenticated = computed(() => !!accessToken.value && !!user.value)
  const isSuperAdmin = computed(() => !!user.value?.roles?.includes('SUPER_ADMIN'))

  // hasPermission mirrors the backend RBAC (SUPER_ADMIN implicitly has all).
  function hasPermission(perm: string) {
    if (!user.value) return false
    if (user.value.roles?.includes('SUPER_ADMIN')) return true
    return (user.value.permissions || []).includes(perm)
  }
  /** True when no permission is required, or the user holds at least one. */
  function hasAny(perms: string[]) {
    return perms.length === 0 || perms.some(hasPermission)
  }
  /** Can this person open the page behind a route name? Unknown routes: no. */
  function canAccess(routeName: unknown) {
    if (typeof routeName !== 'string') return false
    const need = PAGE_ACCESS[routeName]
    if (!need) return false
    return hasAny(need)
  }

  function adoptSession(pair: { access_token: string; refresh_token: string; user: AuthUser }) {
    setTokens(pair.access_token, pair.refresh_token)
    user.value = pair.user
    setup.value = null
    lastSync = Date.now()
  }

  /** Sign in. Returns 'setup' when an invited employee must finish setup. */
  async function login(email: string, password: string): Promise<'ok' | 'setup'> {
    loading.value = true
    try {
      const res = await auth.loginAdmin(email, password)
      if ('setup_required' in res && res.setup_required) {
        setup.value = res
        return 'setup'
      }
      adoptSession(res as Parameters<typeof adoptSession>[0])
      return 'ok'
    } finally { loading.value = false }
  }

  /** Finish first sign-in with the emailed code and a new password. */
  async function completeSetup(code: string, newPassword: string) {
    if (!setup.value) throw new Error('No account setup in progress')
    loading.value = true
    try {
      const pair = await auth.adminSetupVerify(setup.value.setup_token, code.trim(), newPassword)
      adoptSession(pair)
    } finally { loading.value = false }
  }

  async function resendSetupCode() {
    if (!setup.value) throw new Error('No account setup in progress')
    await auth.adminSetupResend(setup.value.setup_token)
  }

  function cancelSetup() { setup.value = null }

  async function fetchMe() {
    if (!tokenStore.getAccess()) { initialized.value = true; return }
    try {
      user.value = (await auth.me()) as AuthUser
      accessToken.value = tokenStore.getAccess()
      lastSync = Date.now()
    } catch { clearTokens(); user.value = null } finally { initialized.value = true }
  }

  /**
   * Re-read who I am (and what I may do) from the server when the copy held
   * here is older than `maxAgeMs`. A role change or a disabled account therefore
   * shows up within a minute without signing in again. Network blips keep the
   * current session; only an explicit "not allowed" answer signs the user out.
   */
  async function refreshIfStale(maxAgeMs = 60_000) {
    if (!user.value || Date.now() - lastSync < maxAgeMs) return
    try {
      user.value = (await auth.me()) as AuthUser
      lastSync = Date.now()
    } catch (e) {
      const status = (e as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) { clearTokens(); user.value = null }
    }
  }

  async function logout() {
    const r = tokenStore.getRefresh()
    if (r) { try { await auth.logout(r) } catch { /* ignore */ } }
    clearTokens(); user.value = null; setup.value = null
  }

  return {
    user, loading, initialized, setup, isAuthenticated, isSuperAdmin,
    hasPermission, hasAny, canAccess,
    login, completeSetup, resendSetupCode, cancelSetup, fetchMe, refreshIfStale, logout,
  }
})
