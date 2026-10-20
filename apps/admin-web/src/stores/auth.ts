import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { AuthUser } from '@carlitos/types'
import { auth, tokenStore } from '@/api'

// Admin Desk auth store. Admin accounts are provisioned (no public register).
export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const loading = ref(false)
  const initialized = ref(false)
  // Reactive mirror of the access token — reading localStorage inside a computed
  // is not reactive, which made post-login redirects flaky (only worked after a
  // reload). Tracking it in a ref keeps isAuthenticated reactive.
  const accessToken = ref<string | null>(tokenStore.getAccess())
  function setTokens(access: string, refresh: string) {
    tokenStore.set(access, refresh)
    accessToken.value = access
  }
  function clearTokens() {
    tokenStore.clear()
    accessToken.value = null
  }
  const isAuthenticated = computed(() => !!accessToken.value && !!user.value)

  // hasPermission mirrors the backend RBAC (SUPER_ADMIN implicitly has all).
  function hasPermission(perm: string) {
    if (!user.value) return false
    if (user.value.roles.includes('SUPER_ADMIN')) return true
    return user.value.permissions.includes(perm)
  }

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const pair = await auth.loginAdmin(email, password)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
    } finally { loading.value = false }
  }

  async function fetchMe() {
    if (!tokenStore.getAccess()) { initialized.value = true; return }
    try { user.value = (await auth.me()) as AuthUser; accessToken.value = tokenStore.getAccess() } catch { clearTokens(); user.value = null } finally { initialized.value = true }
  }

  async function logout() {
    const r = tokenStore.getRefresh()
    if (r) { try { await auth.logout(r) } catch { /* ignore */ } }
    clearTokens(); user.value = null
  }

  return { user, loading, initialized, isAuthenticated, hasPermission, login, fetchMe, logout }
})
