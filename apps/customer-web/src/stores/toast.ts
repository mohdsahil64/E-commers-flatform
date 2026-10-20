import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastType = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  type: ToastType
  message: string
}

// A tiny global toast store. Components call push()/success()/error() and a
// single <Toaster> mounted in AppLayout renders the stack. Toasts auto-dismiss.
export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([])
  let seq = 0

  function push(message: string, type: ToastType = 'info', ttl = 3000) {
    const id = ++seq
    toasts.value.push({ id, type, message })
    if (ttl > 0) {
      setTimeout(() => dismiss(id), ttl)
    }
    return id
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  const success = (m: string, ttl = 3000) => push(m, 'success', ttl)
  const error = (m: string, ttl = 4000) => push(m, 'error', ttl)
  const info = (m: string, ttl = 3000) => push(m, 'info', ttl)

  return { toasts, push, dismiss, success, error, info }
})
