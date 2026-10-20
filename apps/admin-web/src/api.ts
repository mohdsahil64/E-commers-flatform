import { ApiClient, localStorageTokenStore, authApi, adminApi } from '@carlitos/api-client'

const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'

export const tokenStore = localStorageTokenStore('carlitos_admin')

export const api = new ApiClient({
  baseURL,
  store: tokenStore,
  onAuthError: () => {
    if (typeof window !== 'undefined') window.location.assign('/admin/login')
  },
})

export const auth = authApi(api)
export const admin = adminApi(api)
export const MEDIA_BASE = import.meta.env.VITE_MEDIA_BASE_URL || 'http://localhost:8080/media'

// ── Global loading indicator ────────────────────────────────────────
// Every API request drives the small centered spinner (see RouteLoader.vue):
// visible while any request is in flight, hidden the instant the last one
// settles. Loader store imported lazily so this module doesn't require Pinia
// to be initialised at import time.
import { useLoaderStore } from '@/stores/loader'
function loader() {
  try { return useLoaderStore() } catch { return null }
}
api.http.interceptors.request.use((config) => {
  loader()?.begin()
  return config
})
api.http.interceptors.response.use(
  (res) => { loader()?.end(); return res },
  (err) => { loader()?.end(); return Promise.reject(err) },
)
