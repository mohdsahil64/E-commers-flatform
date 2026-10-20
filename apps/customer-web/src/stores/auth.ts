import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { AuthUser } from '@carlitos/types'
import { auth, tokenStore } from '@/api'

// The customer auth store manages the session for the Customer Store portal.
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

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const pair = await auth.loginCustomer(email, password)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
    } finally {
      loading.value = false
    }
  }

  // requestOtp sends a one-time code to the given mobile number.
  async function requestOtp(phone: string) {
    loading.value = true
    try {
      await auth.requestOtp(phone)
    } finally {
      loading.value = false
    }
  }

  // verifyOtp verifies the code and logs in / registers the customer.
  async function verifyOtp(phone: string, code: string) {
    loading.value = true
    try {
      const pair = await auth.verifyOtp(phone, code)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
      // Merge any guest cart + wishlist accumulated before sign-in onto the
      // server. Imported lazily to avoid a circular store dependency at load.
      try {
        const { useCartStore } = await import('@/stores/cart')
        await useCartStore().mergeGuestIntoServer()
      } catch { /* non-fatal */ }
      try {
        const { useWishlistStore } = await import('@/stores/wishlist')
        await useWishlistStore().mergeGuestIntoServer()
      } catch { /* non-fatal */ }
    } finally {
      loading.value = false
    }
  }

  async function register(body: { full_name: string; email: string; phone?: string; password: string }) {
    loading.value = true
    try {
      const pair = await auth.registerCustomer(body)
      setTokens(pair.access_token, pair.refresh_token)
      user.value = pair.user
    } finally {
      loading.value = false
    }
  }

  // fetchMe restores the session from a stored token (called once at startup).
  async function fetchMe() {
    if (!tokenStore.getAccess()) {
      initialized.value = true
      return
    }
    try {
      user.value = (await auth.me()) as AuthUser
      accessToken.value = tokenStore.getAccess()
    } catch {
      clearTokens()
      user.value = null
    } finally {
      initialized.value = true
    }
  }

  async function logout() {
    const refresh = tokenStore.getRefresh()
    if (refresh) {
      try { await auth.logout(refresh) } catch { /* ignore */ }
    }
    clearTokens()
    user.value = null
  }

  return { user, loading, initialized, isAuthenticated, login, register, requestOtp, verifyOtp, fetchMe, logout }
})
