import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Global loading indicator — a small centered spinner overlays the content
// whenever something is in flight (route switch, button action, data fetch).
// Reflects REAL time: shows while work is pending, hides the instant it
// settles. No fake/minimum duration.
export const useLoaderStore = defineStore('loader', () => {
  const pending = ref(0)
  const active = computed(() => pending.value > 0)

  function begin() { pending.value++ }
  function end() { if (pending.value > 0) pending.value-- }

  async function wrap<T>(p: Promise<T>): Promise<T> {
    begin()
    try { return await p } finally { end() }
  }

  return { pending, active, begin, end, wrap }
})
