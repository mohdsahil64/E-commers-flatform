import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { AuthUser, SellerStatus } from '@carlitos/types'
import { auth, seller as sellerApi, tokenStore } from '@/api'

// Seller Desk auth store.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  // Reactive mirror of the access token. Reading localStorage directly inside a
  // computed is NOT reactive, which made post-login redirects flaky (worked only
  // after a reload). Tracking it in a ref keeps isAuthenticated reactive.
  const accessToken = ref<string | null>(tokenStore.getAccess())
  function setTokens(access: string, refresh: string) {
    tokenStore.set(access, refresh)
    accessToken.value = access
  }
  function clearTokens() {
    tokenStore.clear()
    accessToken.value = null
  }
  // Seller store approval status. Product listing and other selling actions are
  // gated on this being 'approved' (the backend enforces it too).
  const sellerStatus = ref<SellerStatus | null>(null)
  const loading = ref(false)
  const initialized = ref(false)
  const isAuthenticated = computed(() => !!accessToken.value && !!user.value)
  const isApproved = computed(() => sellerStatus.value === 'approved')

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const pair = await auth.loginSeller(email, password)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
    } finally { loading.value = false }
  }

  async function register(body: { owner_name: string; email: string; phone?: string; password: string; store_name: string }) {
    loading.value = true
    try {
      const pair = await auth.registerSeller(body)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
    } finally { loading.value = false }
  }

  async function fetchMe() {
    if (!tokenStore.getAccess()) { initialized.value = true; return }
    try {
      user.value = (await auth.me()) as AuthUser
      accessToken.value = tokenStore.getAccess()
      await refreshSellerStatus()
    } catch { clearTokens(); user.value = null; sellerStatus.value = null } finally { initialized.value = true }
  }

  // Loads the seller store's approval status (best-effort; null on failure).
  async function refreshSellerStatus() {
    try {
      const s = await sellerApi.profile()
      sellerStatus.value = (s?.status as SellerStatus) ?? null
    } catch {
      sellerStatus.value = null
    }
  }

  async function logout() {
    const r = tokenStore.getRefresh()
    if (r) { try { await auth.logout(r) } catch { /* ignore */ } }
    clearTokens(); user.value = null; sellerStatus.value = null
  }

  return { user, sellerStatus, loading, initialized, isAuthenticated, isApproved, login, register, fetchMe, refreshSellerStatus, logout }
})
