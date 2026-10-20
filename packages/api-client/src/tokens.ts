// Token storage abstraction. Each portal uses a distinct storage key so the
// three apps (which may share localhost) never leak tokens into each other.

export interface TokenStore {
  getAccess(): string | null
  getRefresh(): string | null
  set(access: string, refresh: string): void
  clear(): void
}

/** localStorageTokenStore persists tokens under a per-portal key prefix. */
export function localStorageTokenStore(prefix: string): TokenStore {
  const accessKey = `${prefix}.access`
  const refreshKey = `${prefix}.refresh`
  return {
    getAccess: () => (typeof localStorage !== 'undefined' ? localStorage.getItem(accessKey) : null),
    getRefresh: () => (typeof localStorage !== 'undefined' ? localStorage.getItem(refreshKey) : null),
    set(access: string, refresh: string) {
      localStorage.setItem(accessKey, access)
      localStorage.setItem(refreshKey, refresh)
    },
    clear() {
      localStorage.removeItem(accessKey)
      localStorage.removeItem(refreshKey)
    },
  }
}
