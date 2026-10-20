import { defineStore } from 'pinia'
import { ref } from 'vue'

// Small UI store for global overlays (cart drawer, etc.) so any component can
// open the cart panel without prop drilling or navigation.
export const useUiStore = defineStore('ui', () => {
  const cartOpen = ref(false)

  function openCart() { cartOpen.value = true }
  function closeCart() { cartOpen.value = false }
  function toggleCart() { cartOpen.value = !cartOpen.value }

  return { cartOpen, openCart, closeCart, toggleCart }
})
