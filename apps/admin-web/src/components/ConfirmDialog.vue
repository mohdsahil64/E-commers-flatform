<script setup lang="ts">
import { ref } from 'vue'
import { AlertTriangle, X } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  title: string
  message?: string
  confirmLabel?: string
  variant?: 'danger' | 'primary'
  requireReason?: boolean
}>()

const emit = defineEmits<{
  (e: 'confirm', reason: string): void
  (e: 'cancel'): void
}>()

const reason = ref('')

function confirm() {
  emit('confirm', reason.value)
  reason.value = ''
}
</script>

<template>
  <transition name="dialog">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true">
      <div class="absolute inset-0 bg-black/40" @click="emit('cancel')" />
      <div class="relative w-full max-w-md rounded-xl bg-white p-6 shadow-lg">
        <div class="flex items-start gap-3">
          <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-full', variant === 'danger' ? 'bg-red-50' : 'bg-stone-100']">
            <AlertTriangle :size="20" :class="variant === 'danger' ? 'text-red-500' : 'text-stone-500'" />
          </div>
          <div class="flex-1">
            <h2 class="text-h4 text-ink">{{ title }}</h2>
            <p v-if="message" class="mt-1 text-sm text-stone-600">{{ message }}</p>
          </div>
          <button class="text-stone-400 hover:text-stone-600" @click="emit('cancel')" aria-label="Close">
            <X :size="18" />
          </button>
        </div>

        <div v-if="requireReason" class="mt-4">
          <label class="label">Reason</label>
          <textarea v-model="reason" class="textarea" rows="3" placeholder="Provide a reason…" />
        </div>

        <div class="mt-6 flex justify-end gap-3">
          <button class="btn-ghost" @click="emit('cancel')">Cancel</button>
          <button
            :class="variant === 'danger' ? 'btn-danger' : 'btn-primary'"
            :disabled="requireReason && !reason.trim()"
            @click="confirm"
          >
            {{ confirmLabel || 'Confirm' }}
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.dialog-enter-active, .dialog-leave-active { transition: opacity 200ms ease; }
.dialog-enter-from, .dialog-leave-to { opacity: 0; }
</style>
