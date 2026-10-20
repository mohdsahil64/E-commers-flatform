import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'
import type { ApiEnvelope, TokenPair } from '@carlitos/types'
import { TokenStore } from './tokens'

export interface ClientOptions {
  baseURL: string
  store: TokenStore
  /** Called when the session becomes invalid (refresh failed). */
  onAuthError?: () => void
}

/**
 * ApiClient wraps axios with:
 *  - automatic Bearer token attachment,
 *  - a single-flight refresh-on-401 that retries the original request,
 *  - envelope unwrapping (returns `data` from the standard response shape).
 */
export class ApiClient {
  readonly http: AxiosInstance
  private store: TokenStore
  private onAuthError?: () => void
  private refreshing: Promise<boolean> | null = null

  constructor(opts: ClientOptions) {
    this.store = opts.store
    this.onAuthError = opts.onAuthError
    this.http = axios.create({ baseURL: opts.baseURL, headers: { 'Content-Type': 'application/json' } })

    this.http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
      const token = this.store.getAccess()
      if (token) config.headers.Authorization = `Bearer ${token}`
      return config
    })

    this.http.interceptors.response.use(
      (res) => res,
      async (error: AxiosError) => {
        const original = error.config as (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined
        const status = error.response?.status
        if (status === 401 && original && !original._retried && this.store.getRefresh()) {
          original._retried = true
          const ok = await this.refresh()
          if (ok) {
            const token = this.store.getAccess()
            if (token) original.headers.Authorization = `Bearer ${token}`
            return this.http(original)
          }
          this.store.clear()
          this.onAuthError?.()
        }
        return Promise.reject(error)
      },
    )
  }

  /** refresh rotates tokens; concurrent callers share one in-flight request. */
  private async refresh(): Promise<boolean> {
    if (this.refreshing) return this.refreshing
    this.refreshing = (async () => {
      try {
        const refresh = this.store.getRefresh()
        if (!refresh) return false
        const res = await axios.post<ApiEnvelope<TokenPair>>(
          `${this.http.defaults.baseURL}/auth/refresh`,
          { refresh_token: refresh },
        )
        const pair = res.data.data
        if (!pair) return false
        this.store.set(pair.access_token, pair.refresh_token)
        return true
      } catch {
        return false
      } finally {
        this.refreshing = null
      }
    })()
    return this.refreshing
  }

  /** unwrap returns the `data` field from the standard envelope. */
  private unwrap<T>(env: ApiEnvelope<T>): T {
    return env.data as T
  }

  async get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
    const res = await this.http.get<ApiEnvelope<T>>(url, { params })
    return this.unwrap(res.data)
  }

  /** getWithMeta returns both data and pagination meta. */
  async getWithMeta<T>(url: string, params?: Record<string, unknown>): Promise<ApiEnvelope<T>> {
    const res = await this.http.get<ApiEnvelope<T>>(url, { params })
    return res.data
  }

  async post<T>(url: string, body?: unknown): Promise<T> {
    const res = await this.http.post<ApiEnvelope<T>>(url, body)
    return this.unwrap(res.data)
  }

  async put<T>(url: string, body?: unknown): Promise<T> {
    const res = await this.http.put<ApiEnvelope<T>>(url, body)
    return this.unwrap(res.data)
  }

  async patch<T>(url: string, body?: unknown): Promise<T> {
    const res = await this.http.patch<ApiEnvelope<T>>(url, body)
    return this.unwrap(res.data)
  }

  async del<T>(url: string): Promise<T> {
    const res = await this.http.delete<ApiEnvelope<T>>(url)
    return this.unwrap(res.data)
  }

  /** uploadFile posts multipart form-data to the uploads endpoint. */
  async uploadFile<T>(url: string, file: File): Promise<T> {
    const form = new FormData()
    form.append('file', file)
    const res = await this.http.post<ApiEnvelope<T>>(url, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return this.unwrap(res.data)
  }

  /** getBlob fetches a binary response (e.g. a PDF) with auth headers applied. */
  async getBlob(url: string, params?: Record<string, unknown>): Promise<Blob> {
    const res = await this.http.get(url, { params, responseType: 'blob' })
    return res.data as Blob
  }
}
