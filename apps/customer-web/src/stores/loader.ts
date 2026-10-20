import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Global loading indicator. A small centered spinner overlays the content
// whenever something is in flight — a route switch, a button action, or a
// data fetch. It reflects REAL time: it shows while the work is pending and
// hides the instant it settles (no fake/minimum duration).
//
// Usage:
//   const loader = useLoaderStore()
//   await loader.wrap(api.doSomething())   // spinner while the promise runs
// or manually:
//   loader.begin(); try { ... } finally { loader.end() }
export const useLoaderStore = defineStore('loader', () => {
  // Counter so overlapping tasks keep the spinner up until the last one ends.
  const pending = ref(0)
  const active = computed(() => pending.value > 0)

  function begin() { pending.value++ }
  function end() { if (pending.value > 0) pending.value-- }

  // Wrap a promise so the spinner stays exactly as long as it runs.
  async function wrap<T>(p: Promise<T>): Promise<T> {
    begin()
    try {
      return await p
    } finally {
      end()
    }
  }

  return { pending, active, begin, end, wrap }
})
