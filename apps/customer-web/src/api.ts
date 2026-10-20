// Constructs the shared ApiClient for the customer portal and exposes the typed
// endpoint groups. Tokens are stored under the `carlitos_customer` prefix so
// the three portals never share sessions.
import { ApiClient, localStorageTokenStore, authApi, publicApi, customerApi } from '@carlitos/api-client'

const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'

export const tokenStore = localStorageTokenStore('carlitos_customer')

export const api = new ApiClient({
  baseURL,
  store: tokenStore,
  onAuthError: () => {
    // Redirect to login on unrecoverable auth failure.
    if (typeof window !== 'undefined') window.location.assign('/login')
  },
})

export const auth = authApi(api)
export const pub = publicApi(api)
export const customer = customerApi(api)

export const MEDIA_BASE = import.meta.env.VITE_MEDIA_BASE_URL || 'http://localhost:8080/media'

// ── Global loading indicator ────────────────────────────────────────
// Every API request drives the small centered spinner (see RouteLoader.vue):
// it shows while any request is in flight and hides the instant the last one
// settles — real time, no fake delay. The loader store is imported lazily so
// this module doesn't depend on Pinia being initialised at import time.
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
